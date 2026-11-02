import os, psycopg2
conn = psycopg2.connect(os.environ.get(
    "DATABASE_URL",
    "postgres://peter:peter@localhost:5432/youtube_tracker?sslmode=disable"))
cur = conn.cursor()
cur.execute("DELETE FROM public.indexing_jobs WHERE status='done'")
print("cleared:", cur.rowcount)
conn.commit()
cur.close(); conn.close()