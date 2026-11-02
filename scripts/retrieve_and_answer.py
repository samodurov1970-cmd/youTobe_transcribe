#!/usr/bin/env python3
"""
retrieve_and_answer.py — ретрив-конвейер: поиск чанков + генерация промпта в 3 раздела.

Зависимости:
  pip install sentence-transformers psycopg2-binary numpy pyyaml

Возвращает промпт для LLM, не вызывает LLM.
"""

import argparse
import json
import os
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

import numpy as np
import psycopg2
import psycopg2.extras
import yaml
from sentence_transformers import SentenceTransformer

BASE_DIR = Path(__file__).resolve().parent.parent
CONFIG_PATH = BASE_DIR / "config" / "embedding_config.yaml"


def load_config() -> dict:
    if CONFIG_PATH.exists():
        with open(CONFIG_PATH, "r", encoding="utf-8") as f:
            return yaml.safe_load(f)
    return {}


def get_db_conn() -> psycopg2.extensions.connection:
    db_url = os.environ.get(
        "DATABASE_URL",
        "postgres://peter:peter@localhost:5432/youtube_tracker?sslmode=disable",
    )
    return psycopg2.connect(db_url)


def embed_texts(texts: List[str], model_name: str) -> np.ndarray:
    model = SentenceTransformer(model_name)
    return model.encode(texts, convert_to_numpy=True, normalize_embeddings=True)


def search_chunks(
    conn: psycopg2.extensions.connection,
    question_embedding: List[float],
    config: dict,
    limit_candidates: int = 40,
) -> List[dict]:
    """Векторный поиск чанков с фильтрами."""
    retrieval_cfg = config.get("retrieval", {})
    exclude_ads = retrieval_cfg.get("exclude_ads", True)
    exclude_orphans = retrieval_cfg.get("exclude_orphans", True)

    where_conditions: List[str] = []
    params: List[Any] = []

    if exclude_ads:
        where_conditions.append("is_ad = false")
    if exclude_orphans:
        where_conditions.append("is_orphan = false")

    where_sql = " AND ".join(where_conditions) if where_conditions else "true"

    sql = f"""
        SELECT
            id, video_id, channel_name, video_url, chunk_index, chunk_text,
            lang, is_ad, is_orphan, created_at,
            embedding::text AS embedding_text,
            1 - (embedding <=> %s::vector) AS similarity
        FROM public.knowledge_chunks
        WHERE {where_sql}
        ORDER BY embedding <=> %s::vector
        LIMIT %s
    """
    params = [question_embedding, question_embedding, limit_candidates]

    with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
        cur.execute(sql, params)
        rows = cur.fetchall()

    # Подсчет отфильтрованных сирот (если включен фильтр)
    if exclude_orphans:
        with conn.cursor() as cur2:
            cur2.execute(
                "SELECT COUNT(*) FROM public.knowledge_chunks WHERE is_orphan = true"
            )
            orphans_count = cur2.fetchone()[0]
        if orphans_count > 0:
            print(f"[search_chunks] Отфильтровано сирот: {orphans_count}")

    return [dict(row) for row in rows]


def diversify_by_channel(chunks: List[dict], max_per_channel: int = 3) -> List[dict]:
    """Диверсификация: не больше max_per_channel чанков с одного канала."""
    by_channel: Dict[str, List[dict]] = {}
    for ch in chunks:
        by_channel.setdefault(ch["channel_name"], []).append(ch)

    result: List[dict] = []
    for channel_name, items in by_channel.items():
        result.extend(items[:max_per_channel])

    result.sort(key=lambda x: x["similarity"], reverse=True)
    return result


def deduplicate_by_embedding(chunks: List[dict], threshold: float = 0.85) -> List[dict]:
    """Дедупликация по косинусному сходству эмбеддингов + по соседним чанкам одного видео."""
    if len(chunks) <= 1:
        return chunks

    import ast
    embeddings: Dict[int, np.ndarray] = {}
    for i, ch in enumerate(chunks):
        emb_text = ch.get("embedding_text")
        if emb_text:
            try:
                embeddings[i] = np.array(ast.literal_eval(emb_text), dtype=np.float32)
            except (ValueError, SyntaxError):
                pass

    result: List[Tuple[int, dict]] = []
    seen_video_chunks: set = set()

    for i, ch in enumerate(chunks):
        # 1. Дедуп по соседним чанкам одного видео (chunk_index ± 2)
        key = (ch["video_id"], ch["chunk_index"])
        is_neighbor = False
        for seen_vid, seen_idx in seen_video_chunks:
            if seen_vid == ch["video_id"] and abs(seen_idx - ch["chunk_index"]) <= 2:
                is_neighbor = True
                break
        if is_neighbor:
            continue

        # 2. Дедуп по косинусу
        is_dup = False
        for kept_idx, _ in result:
            if i in embeddings and kept_idx in embeddings:
                dot = float(np.dot(embeddings[i], embeddings[kept_idx]))
                norm_a = float(np.linalg.norm(embeddings[i]))
                norm_b = float(np.linalg.norm(embeddings[kept_idx]))
                if norm_a > 0 and norm_b > 0:
                    sim = dot / (norm_a * norm_b)
                    if sim > threshold:
                        is_dup = True
                        break
        if not is_dup:
            result.append((i, ch))
            seen_video_chunks.add(key)

    return [ch for _, ch in result]


def select_top_chunks(
    conn: psycopg2.extensions.connection,
    question_embedding: List[float],
    config: dict,
) -> List[dict]:
    """Полный конвейер отбора чанков: поиск → диверсификация → дедупликация → топ."""
    retrieval_cfg = config.get("retrieval", {})
    top_k_candidates = retrieval_cfg.get("top_k_candidates", 40)
    top_k_final = retrieval_cfg.get("top_k_final", 15)
    max_per_channel = retrieval_cfg.get("max_chunks_per_channel", 3)
    dedup_threshold = retrieval_cfg.get("dedup_threshold", 0.85)
    sim_threshold = retrieval_cfg.get("similarity_threshold", 0.45)

    candidates = search_chunks(conn, question_embedding, config, limit_candidates=top_k_candidates)
    candidates = [c for c in candidates if c["similarity"] >= sim_threshold]
    diversified = diversify_by_channel(candidates, max_per_channel=max_per_channel)
    deduped = deduplicate_by_embedding(diversified, threshold=dedup_threshold)
    return deduped[:top_k_final]


def generate_answer(
    question: str,
    chunks: List[dict],
    config: dict,
) -> str:
    """Генерирует промпт для LLM в 3 раздела. Возвращает промпт, не вызывает LLM."""
    answer_cfg = config.get("answer", {})
    require_citations = answer_cfg.get("require_citations", True)
    language = answer_cfg.get("language", "ru")

    sections = answer_cfg.get(
        "sections",
        ["Согласно вашей базе", "Что я думаю согласно моим базам", "Итоговый вывод — как действовать"],
    )

    context_parts: List[str] = []
    for i, ch in enumerate(chunks, 1):
        citation = f"({ch['video_id']}#{ch['chunk_index']})" if require_citations else ""
        context_parts.append(
            f"[{i}] {ch['chunk_text']} {citation} [канал: {ch['channel_name']}]"
        )

    context = "\n\n".join(context_parts)

    prompt = f"""Ты — ассистент по трейдингу. Ответь на вопрос, опираясь на предоставленные фрагменты из базы знаний.

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

Ответ на языке: {language}
"""

    return prompt


def main() -> None:
    parser = argparse.ArgumentParser(description="Поиск чанков и генерация промпта по вопросу")
    parser.add_argument("--question", required=True, help="Вопрос по трейдингу")
    args = parser.parse_args()

    config = load_config()

    # Модель берется из того же конфига, что и при индексации
    emb_cfg = config.get("embedding", {})
    provider_name = emb_cfg.get("provider", "local")
    if provider_name == "local":
        model_name = emb_cfg.get("local", {}).get("model_name", "paraphrase-multilingual-MiniLM-L12-v2")
    elif provider_name == "jina":
        model_name = emb_cfg.get("jina", {}).get("model", "jina-embeddings-v3")
    else:
        model_name = "paraphrase-multilingual-MiniLM-L12-v2"

    print(f"[retrieve_and_answer] Вопрос: {args.question}")
    print(f"[retrieve_and_answer] Модель эмбеддингов: {model_name}")

    question_emb = embed_texts([args.question], model_name)[0].tolist()

    conn = get_db_conn()
    try:
        chunks = select_top_chunks(conn, question_emb, config)
        print(f"[retrieve_and_answer] Найдено чанков: {len(chunks)}")

        if not chunks:
            print("Релевантных чанков не найдено.")
            return

        for i, ch in enumerate(chunks, 1):
            print(f"  [{i}] sim={ch['similarity']:.3f} ch={ch['channel_name']} "
                  f"vid={ch['video_id']}#{ch['chunk_index']} | {ch['chunk_text'][:80]}...")

        prompt = generate_answer(args.question, chunks, config)
        print("\n" + "=" * 60)
        print("ПРОМПТ ДЛЯ LLM:")
        print("=" * 60)
        print(prompt)
    finally:
        conn.close()


if __name__ == "__main__":
    main()