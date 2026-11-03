#!/usr/bin/env python3
import sys
import os
import psycopg2
from sentence_transformers import SentenceTransformer

if len(sys.argv) < 2:
    print("usage: python search_test.py \"вопрос\"")
    sys.exit(1)

question = sys.argv[1]
print(f"Вопрос: {question}\n")

print("Генерация вектора (e5-base)...")
model = SentenceTransformer("intfloat/multilingual-e5-base")
emb = model.encode(["query: " + question], normalize_embeddings=True)[0].tolist()

db_url = os.environ.get("DATABASE_URL", "postgres://peter:peter@localhost:5432/youtube_tracker?sslmode=disable")
conn = psycopg2.connect(db_url)
cur = conn.cursor()

sql = """
    SELECT video_id, chunk_index, channel_name, LEFT(chunk_text, 100) AS preview, 
           1 - (embedding <=> %s::vector) AS similarity
    FROM public.knowledge_chunks 
    WHERE is_ad = false AND is_orphan = false
    ORDER BY embedding <=> %s::vector 
    LIMIT 10;
"""
cur.execute(sql, (emb, emb))
rows = cur.fetchall()

print(f"\n{'similarity':<12} | {'video_id':<12} | {'chunk':<6} | {'channel':<25} | preview")
print("-" * 130)
for vid, cidx, ch, preview, sim in rows:
    print(f"{sim:<12.4f} | {vid:<12} | {cidx:<6} | {ch[:25]:<25} | {preview[:60]}...")

cur.close()
conn.close()