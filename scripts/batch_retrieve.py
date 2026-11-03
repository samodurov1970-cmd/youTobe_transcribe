#!/usr/bin/env python3
"""Пакетная проверка retrieval на нескольких вопросах с кешем моделей."""
import sys
import time
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import retrieve_and_answer as R


QUESTIONS = [
    "как правильно ставить стоп-лосс?",
    "что такое fair value gap?",
    "как определить тренд на старшем таймфрейме?",
    "как управлять риском в сделке?",
    "что такое ликвидность в трейдинге?",
]


def main() -> None:
    config = R.load_config()
    emb_cfg = config.get("embedding", {})
    model_name = emb_cfg.get("local", {}).get("model_name", "intfloat/multilingual-e5-base")
    conn = R.get_db_conn()
    t_total = time.time()
    try:
        for q in QUESTIONS:
            print(f"\n{'='*70}\nQ: {q}\n{'='*70}")
            t0 = time.time()
            q_emb = R.embed_query(q, model_name)
            chunks = R.select_top_chunks(conn, q, q_emb, config)
            print(f"Найдено: {len(chunks)}  |  time: {time.time()-t0:.1f}s")
            for i, ch in enumerate(chunks, 1):
                rs = ch.get("rerank_score", 0.0)
                print(f"  [{i}] rerank={rs:+.4f} ch={ch['channel_name']} "
                      f"vid={ch['video_id']}#{ch['chunk_index']} | {ch['chunk_text'][:70]}...")
    finally:
        conn.close()
    print(f"\nTotal: {time.time()-t_total:.1f}s")


if __name__ == "__main__":
    main()