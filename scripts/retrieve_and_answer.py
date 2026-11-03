#!/usr/bin/env python3
"""retrieve_and_answer.py — retrieval + rerank + промпт в 3 раздела.

Модели кешируются в памяти процесса (lru_cache).

Конвейер:
  1. e5-base bi-encoder: топ-N кандидатов по cosine similarity.
  2. Cross-encoder reranker: пересчёт пары (query, chunk).
  3. Фильтр по rerank_threshold.
  4. Дедуп по соседним chunk_index (одно видео).
  5. Диверсификация по каналам.
  6. Топ-N.
"""
import argparse
import os
from functools import lru_cache
from pathlib import Path
from typing import Dict, List

import psycopg2
import psycopg2.extras
import yaml
from sentence_transformers import SentenceTransformer, CrossEncoder

BASE_DIR = Path(__file__).resolve().parent.parent
CONFIG_PATH = BASE_DIR / "config" / "embedding_config.yaml"


def load_config() -> dict:
    if CONFIG_PATH.exists():
        with open(CONFIG_PATH, "r", encoding="utf-8") as f:
            return yaml.safe_load(f)
    return {}


def get_db_conn():
    db_url = os.environ.get(
        "DATABASE_URL",
        "postgres://peter:peter@localhost:5432/youtube_tracker?sslmode=disable",
    )
    return psycopg2.connect(db_url)


@lru_cache(maxsize=2)
def _get_embedder(model_name: str) -> SentenceTransformer:
    print(f"[cache] loading embedder {model_name}")
    return SentenceTransformer(model_name)


@lru_cache(maxsize=2)
def _get_reranker(model_name: str) -> CrossEncoder:
    print(f"[cache] loading reranker {model_name}")
    return CrossEncoder(model_name, max_length=512)


def embed_query(question: str, model_name: str) -> List[float]:
    model = _get_embedder(model_name)
    prefix = "query: " if "e5" in model_name.lower() else ""
    emb = model.encode([prefix + question], convert_to_numpy=True,
                       normalize_embeddings=True)[0]
    return emb.tolist()


def search_chunks(conn, question_embedding, config, limit_candidates: int = 40) -> List[dict]:
    rc = config.get("retrieval", {})
    where = []
    if rc.get("exclude_ads", True):
        where.append("is_ad = false")
    if rc.get("exclude_orphans", True):
        where.append("is_orphan = false")
    where_sql = " AND ".join(where) if where else "true"

    sql = f"""
        SELECT id, video_id, channel_name, video_url, chunk_index, chunk_text,
               lang, is_ad, is_orphan, created_at,
               1 - (embedding <=> %s::vector) AS similarity
        FROM public.knowledge_chunks
        WHERE {where_sql}
        ORDER BY embedding <=> %s::vector
        LIMIT %s
    """
    with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
        cur.execute(sql, [question_embedding, question_embedding, limit_candidates])
        return [dict(r) for r in cur.fetchall()]


def rerank(question: str, chunks: List[dict], config: dict) -> List[dict]:
    rc = config.get("retrieval", {})
    if not rc.get("use_reranker", True) or not chunks:
        for c in chunks:
            c["rerank_score"] = c["similarity"]
        return chunks

    model_name = rc.get("reranker_model", "BAAI/bge-reranker-v2-m3")
    threshold = rc.get("rerank_threshold", 0.005)
    model = _get_reranker(model_name)

    pairs = [(question, c["chunk_text"]) for c in chunks]
    scores = model.predict(pairs)
    for c, s in zip(chunks, scores):
        c["rerank_score"] = float(s)

    filtered = [c for c in chunks if c["rerank_score"] >= threshold]
    filtered.sort(key=lambda x: x["rerank_score"], reverse=True)
    return filtered


def dedup_neighbors(chunks: List[dict], window: int = 2) -> List[dict]:
    kept: List[dict] = []
    seen: set = set()
    for c in chunks:
        has_neighbor = any(
            sv == c["video_id"] and abs(si - c["chunk_index"]) <= window
            for sv, si in seen
        )
        if has_neighbor:
            continue
        kept.append(c)
        seen.add((c["video_id"], c["chunk_index"]))
    return kept


def diversify_by_channel(chunks: List[dict], max_per_channel: int = 3) -> List[dict]:
    by_ch: Dict[str, List[dict]] = {}
    for c in chunks:
        by_ch.setdefault(c["channel_name"], []).append(c)
    out: List[dict] = []
    for items in by_ch.values():
        out.extend(items[:max_per_channel])
    out.sort(key=lambda x: x.get("rerank_score", x.get("similarity", 0)), reverse=True)
    return out


def select_top_chunks(conn, question: str, question_embedding, config) -> List[dict]:
    rc = config.get("retrieval", {})
    top_k_cand = rc.get("top_k_candidates", 40)
    top_k_final = rc.get("top_k_final", 15)
    max_per_ch = rc.get("max_chunks_per_channel", 3)

    candidates = search_chunks(conn, question_embedding, config, top_k_cand)
    reranked = rerank(question, candidates, config)
    deduped = dedup_neighbors(reranked)
    diversified = diversify_by_channel(deduped, max_per_ch)
    return diversified[:top_k_final]


def generate_answer(question: str, chunks: List[dict], config: dict) -> str:
    ac = config.get("answer", {})
    require_cit = ac.get("require_citations", True)
    lang = ac.get("language", "ru")
    sections = ac.get("sections", [
        "Согласно вашей базе",
        "Что я думаю согласно моим базам",
        "Итоговый вывод — как действовать",
    ])
    parts = []
    for i, ch in enumerate(chunks, 1):
        cit = f"({ch['video_id']}#{ch['chunk_index']})" if require_cit else ""
        parts.append(f"[{i}] {ch['chunk_text']} {cit} [канал: {ch['channel_name']}]")
    context = "\n\n".join(parts)
    return f"""Ты — ассистент по трейдингу. Ответь на вопрос, опираясь на предоставленные фрагменты из базы знаний.

ВОПРОС: {question}

НАЙДЕННЫЕ ФРАГМЕНТЫ:
{context}

ТРЕБОВАНИЯ к ответу:
1. Раздел 1: "{sections[0]}" — обобщение найденных фрагментов с ссылками на источники (video_id#chunk_index). Без перенаправления «посмотрите видео сами».
2. Раздел 2: "{sections[1]}" — твои добавления, уточнения, нюансы, чётко отделены от раздела 1.
3. Раздел 3: "{sections[2]}" — готовая пошаговая инструкция, смесь первого и второго.
4. Каждое утверждение в разделе 1 должно иметь ссылку (video_id#chunk_index). Если не можете подкрепить ссылкой — не пишите утверждение.
5. Нет рекламных фрагментов.
6. Каналы диверсифицированы.

Ответ на языке: {lang}
"""


def main() -> None:
    parser = argparse.ArgumentParser(description="Поиск чанков и генерация промпта по вопросу")
    parser.add_argument("--question", required=True, help="Вопрос по трейдингу")
    args = parser.parse_args()

    config = load_config()
    emb_cfg = config.get("embedding", {})
    if emb_cfg.get("provider", "local") == "local":
        model_name = emb_cfg.get("local", {}).get("model_name", "intfloat/multilingual-e5-base")
    else:
        model_name = emb_cfg.get("jina", {}).get("model", "jina-embeddings-v3")

    print(f"[retrieve] Вопрос: {args.question}")
    print(f"[retrieve] Модель: {model_name}")

    q_emb = embed_query(args.question, model_name)
    conn = get_db_conn()
    try:
        chunks = select_top_chunks(conn, args.question, q_emb, config)
        print(f"[retrieve] Найдено чанков: {len(chunks)}")

        if not chunks:
            print("Релевантных чанков не найдено.")
            return

        for i, ch in enumerate(chunks, 1):
            rs = ch.get("rerank_score", 0.0)
            sim = ch.get("similarity", 0.0)
            print(f"  [{i}] rerank={rs:+.4f} sim={sim:.3f} ch={ch['channel_name']} "
                  f"vid={ch['video_id']}#{ch['chunk_index']} | {ch['chunk_text'][:80]}...")

        prompt = generate_answer(args.question, chunks, config)
        print("\n" + "=" * 60)
        print(prompt)
    finally:
        conn.close()


if __name__ == "__main__":
    main()