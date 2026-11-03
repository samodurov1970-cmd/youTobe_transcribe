#!/usr/bin/env python3

# mkdir logs -Force
# python scripts/run_pipeline.py

"""
run_pipeline.py — полный цикл инкрементальной индексации:
  1. Синхронизация is_chunked=true там, где чанки уже есть.
  2. Возврат в очередь done-джоб без чанков (retry).
  3. enqueue новых видео (ingest_daily).
  4. chunk_and_embed --mode full.

Идемпотентно. Запускать по cron/вручную.
"""
import os
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8")
    sys.stderr.reconfigure(encoding="utf-8")

import psycopg2

BASE_DIR = Path(__file__).resolve().parent.parent
DB_URL = os.environ.get(
    "DATABASE_URL",
    "postgres://peter:peter@localhost:5432/youtube_tracker?sslmode=disable",
)


def step1_sync_flag(conn) -> int:
    with conn.cursor() as cur:
        cur.execute(
            """
            UPDATE video_files vf
            SET is_chunked = true
            WHERE vf.is_chunked = false
              AND EXISTS (
                  SELECT 1 FROM knowledge_chunks kc WHERE kc.video_id = vf.video_id
              )
            """
        )
        n = cur.rowcount
    conn.commit()
    return n


def step2_requeue_empty(conn) -> int:
    with conn.cursor() as cur:
        cur.execute(
            """
            UPDATE indexing_jobs ij
            SET status = 'queued',
                error_message = 'retry: no chunks',
                updated_at = %s
            WHERE ij.status = 'done'
              AND NOT EXISTS (
                  SELECT 1 FROM knowledge_chunks kc WHERE kc.video_id = ij.video_id
              )
            """,
            (datetime.now(timezone.utc).isoformat(),),
        )
        n = cur.rowcount
    conn.commit()
    return n


def run_stream(cmd: list) -> int:
    """Запускает команду с выводом в реальном времени, без буферизации."""
    print(f"[pipeline] $ {' '.join(cmd)}", flush=True)
    proc = subprocess.Popen(
        cmd,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        encoding="utf-8",
        errors="replace",
        bufsize=1,
    )
    assert proc.stdout is not None
    for line in proc.stdout:
        sys.stdout.write(line)
        sys.stdout.flush()
    proc.wait()
    return proc.returncode


def report(conn) -> None:
    with conn.cursor() as cur:
        print("\n[report] indexing_jobs:", flush=True)
        cur.execute("SELECT status, COUNT(*) FROM indexing_jobs GROUP BY status ORDER BY 2 DESC")
        for s, c in cur.fetchall():
            print(f"  {s}: {c}")
        print("[report] video_files.is_chunked:")
        cur.execute("SELECT is_chunked, COUNT(*) FROM video_files GROUP BY is_chunked")
        for f, c in cur.fetchall():
            print(f"  {f}: {c}")
        cur.execute("SELECT COUNT(*), COUNT(DISTINCT video_id) FROM knowledge_chunks")
        ch, vd = cur.fetchone()
        print(f"[report] knowledge_chunks: {ch} чанков / {vd} видео")


def main() -> None:
    conn = psycopg2.connect(DB_URL)
    try:
        n1 = step1_sync_flag(conn)
        print(f"[pipeline] step1 sync is_chunked=true: {n1}", flush=True)
        n2 = step2_requeue_empty(conn)
        print(f"[pipeline] step2 requeue empty done: {n2}", flush=True)
        print("[pipeline] step3 ingest_daily:", flush=True)
        run_stream([sys.executable, str(BASE_DIR / "scripts" / "ingest_daily.py")])
        print("[pipeline] step4 chunk_and_embed --mode full:", flush=True)
        run_stream([sys.executable, str(BASE_DIR / "scripts" / "chunk_and_embed.py"), "--mode", "full"])
        report(conn)
    finally:
        conn.close()


if __name__ == "__main__":
    main()