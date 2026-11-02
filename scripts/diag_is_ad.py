import os, re, psycopg2
from collections import Counter

conn = psycopg2.connect(os.environ.get(
    "DATABASE_URL",
    "postgres://peter:peter@localhost:5432/youtube_tracker?sslmode=disable"))
cur = conn.cursor()
cur.execute("SELECT id, chunk_text FROM knowledge_chunks WHERE channel_name='TraderNick' AND is_ad=true")

pats = {
    'promokod': r'промокод',
    'promo_code': r'promo\s*code',
    'use_code': r'use\s+(the\s+)?code',
    'ref_link': r'реферальная\s+ссылка',
    'partner_link': r'партнёрская\s+ссылка',
    'link_desc': r'ссылка\s+в\s+описании',
    'perehod': r'переходи(те)?\s+по\s+ссылке',
    'sponsor_marker': r'(этот\s+видео\s+спонсор|спонсор\s+выпуска)',
    'vip': r'vip\s+(signals|discord)',
    'eval_acc': r'evaluation\s+account',
    'free50k': r'free\s+50k',
    'sponsor+ctx': r'(sponsor|спонсор).{0,200}(\bola\b|\bprime\b|black\s*bull|top\s*one|topone|edge\s*finder|edgefinder|stock\s*box|stockbox)',
    'weak+sale': r'(edge\s*finder|edgefinder|stock\s*box|stockbox).{0,200}(\bsale\b|\bdiscount\b|\bpromo\b|\bcode\b|\bbuy\b|\bpurchase\b|sign\s*up|get\s+access)',
}

c = Counter()
for cid, txt in cur.fetchall():
    low = (txt or "").lower()
    hits = [k for k, p in pats.items() if re.search(p, low, re.S)]
    for h in hits:
        c[h] += 1
    if not hits:
        print("NO-MATCH", cid, repr((txt or "")[:300]))
print(dict(c))