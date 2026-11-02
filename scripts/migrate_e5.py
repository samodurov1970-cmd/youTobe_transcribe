#!/usr/bin/env python3
"""Выполняет sql/003_migrate_to_e5_768.sql через psycopg2."""
import os
from pathlib import Path
import psycopg2

BASE_DIR = Path(__file__).resolve().parent.parent
SQL_PATH = BASE_DIR / "sql" / "003_migrate_to_e5_768.sql"

DB_URL = os.environ.get(
    "DATABASE_URL",
    "postgres://peter:peter@localhost:5432/youtube_tracker?sslmode=disable",
)


def main() -> None:
    sql = SQL_PATH.read_text(encoding="utf-8")
    conn = psycopg2.connect(DB_URL)
    try:
        with conn.cursor() as cur:
            cur.execute(sql)
        conn.commit()
        print("[migrate_e5] OK")
        with conn.cursor() as cur:
            cur.execute("""
                SELECT column_name, udt_name
                FROM information_schema.columns
                WHERE table_schema='public' AND table_name='knowledge_chunks'
                  AND column_name='embedding'
            """)
            print("[migrate_e5] embedding:", cur.fetchone())
    finally:
        conn.close()


if __name__ == "__main__":
    main()