#!/usr/bin/env python3
"""
chunk_and_embed.py — разбивка транскриптов на чанки и вычисление эмбеддингов.
Режимы:
  --mode pilot     Пилот на N видео
  --mode full      Полная индексация всех видео (перезаписывает)
"""
import argparse
import json
import os
import re
from datetime import datetime, timezone
from pathlib import Path
from typing import List, Optional

import numpy as np
import psycopg2
import psycopg2.extras
import yaml
from sentence_transformers import SentenceTransformer

BASE_DIR = Path(__file__).resolve().parent.parent
CONFIG_PATH = BASE_DIR / "config" / "embedding_config.yaml"

CHUNK_MAX_CHARS = 2000
CHUNK_OVERLAP_PCT = 0.15
SENTENCE_END_RE = re.compile(r"([.!?…])")
SOFT_SPLIT_RE = re.compile(r"([,;:]\s+)")


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


class EmbeddingProvider:
    def embed(self, texts: List[str]) -> np.ndarray:
        raise NotImplementedError

    @property
    def dimension(self) -> int:
        raise NotImplementedError


class LocalEmbeddingProvider(EmbeddingProvider):
    def __init__(self, model_name: str = "intfloat/multilingual-e5-base"):
        self.model = SentenceTransformer(model_name)
        self._dim = self.model.get_sentence_embedding_dimension()
        self._prefix = "passage: " if "e5" in model_name.lower() else ""

    def embed(self, texts: List[str]) -> np.ndarray:
        prefixed = [self._prefix + t for t in texts]
        return self.model.encode(prefixed, convert_to_numpy=True, normalize_embeddings=True)

    @property
    def dimension(self) -> int:
        return self._dim


class JinaEmbeddingProvider(EmbeddingProvider):
    def __init__(self, api_key: str, model: str = "jina-embeddings-v3",
                 base_url: str = "https://api.jina.ai/v1/embeddings"):
        import urllib.request
        self.api_key = api_key
        self.model = model
        self.base_url = base_url
        self._urllib = urllib.request
        self._dim = None

    def embed(self, texts: List[str]) -> np.ndarray:
        data = json.dumps({"input": texts, "model": self.model, "dimensions": 768}).encode("utf-8")
        req = self._urllib.Request(
            self.base_url, data=data,
            headers={"Content-Type": "application/json",
                     "Authorization": f"Bearer {self.api_key}"},
            method="POST",
        )
        with self._urllib.urlopen(req, timeout=60) as resp:
            result = json.loads(resp.read().decode("utf-8"))
        vecs = [item["embedding"] for item in result["data"]]
        arr = np.array(vecs, dtype=np.float32)
        if self._dim is None:
            self._dim = arr.shape[1]
        return arr

    @property
    def dimension(self) -> int:
        return self._dim


def create_embedding_provider(config: dict) -> EmbeddingProvider:
    emb_cfg = config.get("embedding", {})
    provider_name = emb_cfg.get("provider", "local")
    if provider_name == "local":
        local_cfg = emb_cfg.get("local", {})
        return LocalEmbeddingProvider(
            model_name=local_cfg.get("model_name", "intfloat/multilingual-e5-base")
        )
    elif provider_name == "jina":
        jina_cfg = emb_cfg.get("jina", {})
        return JinaEmbeddingProvider(
            api_key=jina_cfg.get("api_key", ""),
            model=jina_cfg.get("model", "jina-embeddings-v3"),
            base_url=jina_cfg.get("base_url", "https://api.jina.ai/v1/embeddings"),
        )
    raise ValueError(f"Unknown embedding provider: {provider_name}")


def split_into_sentences(text: str) -> List[str]:
    text = text.strip()
    if not text:
        return []
    parts = SENTENCE_END_RE.split(text)
    sentences: List[str] = []
    i = 0
    while i < len(parts):
        part = parts[i].strip()
        if not part:
            i += 1
            continue
        if i + 1 < len(parts) and parts[i + 1] in (".", "!", "?", "…"):
            part = part + parts[i + 1]
            i += 2
        else:
            i += 1
        if part:
            sentences.append(part)
    fixed: List[str] = []
    for sent in sentences:
        if len(sent) <= 1500:
            fixed.append(sent)
            continue
        sub_parts = SOFT_SPLIT_RE.split(sent)
        buf = ""
        for sp in sub_parts:
            buf += sp
            if len(buf) >= 800:
                fixed.append(buf.strip())
                buf = ""
        if buf.strip():
            fixed.append(buf.strip())
    return fixed


def chunk_text(text: str, max_chars: int = CHUNK_MAX_CHARS,
               overlap_pct: float = CHUNK_OVERLAP_PCT) -> List[str]:
    sentences = split_into_sentences(text)
    if not sentences:
        return []
    fixed_sentences: List[str] = []
    for sent in sentences:
        if len(sent) <= max_chars:
            fixed_sentences.append(sent)
        else:
            words = sent.split()
            buf: List[str] = []
            buf_len = 0
            for w in words:
                if buf_len + len(w) + 1 > max_chars and buf:
                    fixed_sentences.append(" ".join(buf))
                    buf = []
                    buf_len = 0
                buf.append(w)
                buf_len += len(w) + 1
            if buf:
                fixed_sentences.append(" ".join(buf))
    sentences = fixed_sentences

    chunks: List[str] = []
    current_chunk: List[str] = []
    current_len = 0
    overlap_budget = max_chars // 5
    for sent in sentences:
        sent_len = len(sent)
        if current_len + sent_len > max_chars and current_chunk:
            chunks.append(" ".join(current_chunk))
            tail: List[str] = []
            tail_len = 0
            for prev in reversed(current_chunk):
                if tail_len + len(prev) > overlap_budget:
                    break
                tail.insert(0, prev)
                tail_len += len(prev)
            current_chunk = tail
            current_len = tail_len
        current_chunk.append(sent)
        current_len += sent_len
    if current_chunk:
        chunks.append(" ".join(current_chunk))
    return chunks


def detect_lang(text: str) -> str:
    cyrillic = sum(1 for c in text if "\u0400" <= c <= "\u04FF")
    total = len(text)
    if total == 0:
        return "ru"
    return "ru" if cyrillic / total > 0.3 else "en"


def is_ad_chunk(text: str) -> bool:
    if not text:
        return False
    low = text.lower()
    hard = [
        r"промокод", r"promo\s*code", r"use\s+(the\s+)?code",
        r"реферальная\s+ссылка", r"партнёрская\s+ссылка",
        r"переходи(те)?\s+по\s+ссылке",
        r"этот\s+видео\s+спонсор", r"спонсор\s+выпуска",
        r"thanks?\s+(you\s+)?to\s+\w+(\s+\w+)?\s+for\s+sponsoring",
        r"sponsoring\s+today'?s\s+video", r"sales?\s+pitch",
        r"scan\s+the\s+qr\s+code", r"мастер[-\s]?класс",
        r"напиши(те)?\s+помощнику",
        r"обращайтесь\s+к\s+(нашим\s+)?менеджерам",
        r"кнопка\s+(вот\s+)?здесь\s+(находится\s+)?под\s+видео",
        r"подключайтесь\s+ребята",
        r"выгодно\s+и\s+надежно",
        r"покупайте\s+золотые", r"покупайте\s+серебряные",
        r"инвестиционные\s+монеты", r"георгий\s+победоносец",
    ]
    for p in hard:
        if re.search(p, low):
            return True
    brands = (r"\bola\s*prime\b|\bblack\s*bull\b|\btop\s*one\b|\btopone\b|"
              r"\bedge\s*finder\b|\bedgefinder\b|\bstock\s*box\b|\bstockbox\b")
    if re.search(r"sponsor|спонсор", low) and re.search(brands, low):
        return True
    weak = r"\bedge\s*finder\b|\bedgefinder\b|\bstock\s*box\b|\bstockbox\b"
    sale = (r"black\s*friday|%\s*off|\bsale\b|\bdiscount\b|limited\s+time|"
            r"free\s+\d+\s*k|evaluation\s+account|\bvip\s+discord|"
            r"information\s+(can\s+be\s+)?found\s+below|"
            r"you\s+get\s+to\s+see\s+every\s+trade")
    for m in re.finditer(weak, low):
        win = low[max(0, m.start()-300): m.end()+300]
        if re.search(sale, win):
            return True
    return False


def video_exists_in_videos_table(conn, video_id: str) -> bool:
    with conn.cursor() as cur:
        cur.execute("SELECT 1 FROM public.videos WHERE video_id = %s", (video_id,))
        return cur.fetchone() is not None


def get_videos_for_indexing(conn, mode: str, limit: Optional[int] = None) -> List[dict]:
    with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
        if mode == "pilot":
            cur.execute(
                """
                SELECT vf.video_id, vf.channel_name, vf.video_url, vf.file_content
                FROM public.video_files vf
                JOIN public.videos v ON v.video_id = vf.video_id
                WHERE vf.validation_status IN ('published', 'validated')
                  AND vf.file_content IS NOT NULL
                  AND vf.file_content != ''
                ORDER BY vf.created_at
                LIMIT %s
                """,
                (limit or 14,),
            )
        elif mode == "full":
            cur.execute(
                """
                SELECT vf.video_id, vf.channel_name, vf.video_url, vf.file_content
                FROM public.video_files vf
                WHERE vf.validation_status IN ('published', 'validated')
                  AND vf.file_content IS NOT NULL
                  AND vf.file_content != ''
                ORDER BY vf.created_at
                """
            )
        else:
            raise ValueError(f"Unknown mode: {mode}")
        return [dict(row) for row in cur.fetchall()]


def insert_chunks(conn, provider, video_id, channel_name, video_url, transcript_text) -> int:
    chunks = chunk_text(transcript_text)
    if not chunks:
        return 0
    is_orphan = not video_exists_in_videos_table(conn, video_id)
    embeddings = provider.embed(chunks)
    now = datetime.now(timezone.utc).isoformat()
    rows = []
    for i, (chunk, emb) in enumerate(zip(chunks, embeddings)):
        lang = detect_lang(chunk)
        rows.append((video_id, channel_name, video_url, i, chunk,
                     emb.tolist(), lang, is_ad_chunk(chunk), is_orphan, now))
    with conn.cursor() as cur:
        cur.execute("DELETE FROM public.knowledge_chunks WHERE video_id = %s", (video_id,))
        psycopg2.extras.execute_values(
            cur,
            """
            INSERT INTO public.knowledge_chunks
                (video_id, channel_name, video_url, chunk_index, chunk_text,
                 embedding, lang, is_ad, is_orphan, created_at)
            VALUES %s
            """,
            rows,
            template="(%s, %s, %s, %s, %s, %s::vector, %s, %s, %s, %s)",
        )
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO public.indexing_jobs (video_id, status, chunks_count, embedding_model, updated_at)
            VALUES (%s, 'done', %s, %s, %s)
            ON CONFLICT (video_id) DO UPDATE
                SET status = 'done', chunks_count = %s, embedding_model = %s, updated_at = %s
            """,
            (video_id, len(rows), provider.__class__.__name__, now,
             len(rows), provider.__class__.__name__, now),
        )
    conn.commit()
    return len(rows)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--mode", choices=["pilot", "full"], default="pilot")
    parser.add_argument("--limit", type=int, default=14)
    args = parser.parse_args()

    config = load_config()
    print(f"[chunk_and_embed] Режим: {args.mode}")
    provider = create_embedding_provider(config)
    print(f"[chunk_and_embed] Провайдер: {provider.__class__.__name__}, размерность: {provider.dimension}")

    conn = get_db_conn()
    try:
        videos = get_videos_for_indexing(conn, args.mode, args.limit)
        print(f"[chunk_and_embed] Найдено видео для индексации: {len(videos)}")
        total_chunks = 0
        for idx, video in enumerate(videos, 1):
            vid = video["video_id"]
            print(f"  [{idx}/{len(videos)}] {vid} — {video.get('channel_name', '?')}")
            try:
                n = insert_chunks(conn, provider, vid,
                                  video.get("channel_name", ""),
                                  video.get("video_url", ""),
                                  video.get("file_content", ""))
                total_chunks += n
                print(f"    → чанков: {n}")
            except Exception as e:
                print(f"    → ОШИБКА: {e}")
                with conn.cursor() as cur:
                    cur.execute(
                        """
                        INSERT INTO public.indexing_jobs (video_id, status, error_message, updated_at)
                        VALUES (%s, 'failed', %s, %s)
                        ON CONFLICT (video_id) DO UPDATE
                            SET status = 'failed', error_message = %s, updated_at = %s
                        """,
                        (vid, str(e), datetime.now(timezone.utc).isoformat(),
                         str(e), datetime.now(timezone.utc).isoformat()),
                    )
                conn.commit()
        print(f"\n[chunk_and_embed] Готово. Всего чанков: {total_chunks}")
    finally:
        conn.close()


if __name__ == "__main__":
    main()