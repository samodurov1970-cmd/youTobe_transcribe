from sentence_transformers import CrossEncoder
import os, psycopg2

print("Loading reranker...")
model = CrossEncoder("BAAI/bge-reranker-v2-m3", max_length=512)

conn = psycopg2.connect(os.environ.get(
    "DATABASE_URL",
    "postgres://peter:peter@localhost:5432/youtube_tracker?sslmode=disable"))
cur = conn.cursor()

# По одному релевантному и по одному мусорному чанку с разных видео
cur.execute("""
    (SELECT video_id, chunk_index, chunk_text FROM public.knowledge_chunks
     WHERE video_id = 'REcq3WiYSPA' AND chunk_index = 5)
    UNION ALL
    (SELECT video_id, chunk_index, chunk_text FROM public.knowledge_chunks
     WHERE video_id = 'sUOVMicefnw' AND chunk_index = 2)
    UNION ALL
    (SELECT video_id, chunk_index, chunk_text FROM public.knowledge_chunks
     WHERE video_id = 'EC7rV0hxljM' AND chunk_index = 13)
    UNION ALL
    (SELECT video_id, chunk_index, chunk_text FROM public.knowledge_chunks
     WHERE video_id = 'xQllll4fhvs' AND chunk_index = 5)
    UNION ALL
    (SELECT video_id, chunk_index, chunk_text FROM public.knowledge_chunks
     WHERE video_id = 'nIy3W-8ABhg' AND chunk_index = 8)
    UNION ALL
    (SELECT video_id, chunk_index, chunk_text FROM public.knowledge_chunks
     WHERE video_id = '2qs8-Ksu_iU' AND chunk_index = 5)
    UNION ALL
    (SELECT video_id, chunk_index, chunk_text FROM public.knowledge_chunks
     WHERE video_id = 'REcq3WiYSPA' AND chunk_index = 0)
    UNION ALL
    (SELECT video_id, chunk_index, chunk_text FROM public.knowledge_chunks
     WHERE video_id = 'UYmZJ48O4h0' AND chunk_index = 5)
""")
chunks = cur.fetchall()
cur.close(); conn.close()

query = "как правильно ставить стоп-лосс"
print(f"Query: {query}\n")
pairs = [(query, c[2]) for c in chunks]
scores = model.predict(pairs)
for (vid, idx, txt), s in zip(chunks, scores):
    print(f"  {s:+.4f} | {vid}#{idx} | {txt[:90]}...")