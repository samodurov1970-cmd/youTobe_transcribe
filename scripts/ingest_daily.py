#!/usr/bin/env python3
"""
ingest_daily.py — ежедневная инкрементальная индексация новых видео.

Обнаруживает новые записи в public.video_files со статусом published/validated,
которых ещё нет в public.indexing_jobs, и ставит их в очередь ('queued').
Обработку выполняет chunk_and_embed.py --mode full.

Зависимости:
  pip install psycopg2-binary pyyaml
"""

import argparse
import os
import sys
from datetime import datetime, timezone
from pathlib import Path

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8")
    sys.stderr.reconfigure(encoding="utf-8")

import psycopg2
import psycopg2.extras
import yaml

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


def find_unindexed_videos(
    conn: psycopg2.extensions.connection,
) -> list:
    """Находит опубликованные видео с транскриптом, которых ещё нет в очереди."""
    with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
        cur.execute(
            """
            SELECT vf.video_id, vf.channel_name, vf.video_url, vf.file_content,
                   vf.created_at
            FROM public.video_files vf
            WHERE vf.validation_status IN ('published', 'validated')
              AND vf.file_content IS NOT NULL
              AND vf.file_content != ''
              AND NOT EXISTS (
                  SELECT 1 FROM public.indexing_jobs ij
                  WHERE ij.video_id = vf.video_id
              )
            ORDER BY vf.created_at
            """
        )
        return [dict(row) for row in cur.fetchall()]


def enqueue_indexing_jobs(
    conn: psycopg2.extensions.connection,
    videos: list,
) -> int:
    """Добавляет новые видео в очередь индексации со статусом 'queued'."""
    now = datetime.now(timezone.utc).isoformat()
    count = 0
    with conn.cursor() as cur:
        for v in videos:
            cur.execute(
                """
                INSERT INTO public.indexing_jobs (video_id, status, chunks_count, embedding_model, updated_at)
                VALUES (%s, 'queued', 0, 'pending', %s)
                ON CONFLICT (video_id) DO NOTHING
                """,
                (v["video_id"], now),
            )
            count += cur.rowcount
    conn.commit()
    return count


def main() -> None:
    parser = argparse.ArgumentParser(description="Ежедневная инкрементальная индексация")
    parser.add_argument("--dry-run", action="store_true", help="Только показать, что будет сделано")
    args = parser.parse_args()

    config = load_config()

    conn = get_db_conn()
    try:
        videos = find_unindexed_videos(conn)
        print(f"[ingest_daily] Найдено новых видео для индексации: {len(videos)}", flush=True)

        if args.dry_run:
            for v in videos:
                print(f"  - {v['video_id']} — {v.get('channel_name', '?')}")
            return

        enqueued = enqueue_indexing_jobs(conn, videos)
        print(f"[ingest_daily] Добавлено в очередь: {enqueued}", flush=True)
    finally:
        conn.close()


if __name__ == "__main__":
    main()