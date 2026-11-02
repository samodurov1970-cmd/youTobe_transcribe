#!/usr/bin/env python3
"""update_is_ad.py — обновляет флаг is_ad без пересчёта эмбеддингов."""
import os
import re
import psycopg2
from psycopg2.extras import execute_batch


HARD = [
    r"промокод",
    r"promo\s*code",
    r"use\s+(the\s+)?code",
    r"реферальная\s+ссылка",
    r"партнёрская\s+ссылка",
    r"переходи(те)?\s+по\s+ссылке",
    r"этот\s+видео\s+спонсор",
    r"спонсор\s+выпуска",
    r"thanks?\s+(you\s+)?to\s+\w+(\s+\w+)?\s+for\s+sponsoring",
    r"sponsoring\s+today'?s\s+video",
    r"sales?\s+pitch",
    r"scan\s+the\s+qr\s+code",
    r"мастер[-\s]?класс",
    r"напиши(те)?\s+помощнику",
    r"обращайтесь\s+к\s+(нашим\s+)?менеджерам",
    r"кнопка\s+(вот\s+)?здесь\s+(находится\s+)?под\s+видео",
    r"подключайтесь\s+ребята",
    r"выгодно\s+и\s+надежно",
    r"покупайте\s+золотые",
    r"покупайте\s+серебряные",
    r"инвестиционные\s+монеты",
    r"георгий\s+победоносец",
]

BRANDS = re.compile(
    r"\bola\s*prime\b|\bblack\s*bull\b|\btop\s*one\b|\btopone\b|"
    r"\bedge\s*finder\b|\bedgefinder\b|\bstock\s*box\b|\bstockbox\b"
)

WEAK = re.compile(r"\bedge\s*finder\b|\bedgefinder\b|\bstock\s*box\b|\bstockbox\b")

SALE = re.compile(
    r"black\s*friday|%\s*off|\bsale\b|\bdiscount\b|limited\s+time|"
    r"free\s+\d+\s*k|evaluation\s+account|\bvip\s+discord|"
    r"information\s+(can\s+be\s+)?found\s+below|"
    r"you\s+get\s+to\s+see\s+every\s+trade|"
    r"scan\s+the\s+qr\s+code"
)


def is_ad_chunk(text: str) -> bool:
    if not text:
        return False
    low = text.lower()

    for p in HARD:
        if re.search(p, low):
            return True

    if re.search(r"sponsor|спонсор", low) and BRANDS.search(low):
        return True

    for m in WEAK.finditer(low):
        win = low[max(0, m.start() - 300): m.end() + 300]
        if SALE.search(win):
            return True

    return False


def main() -> None:
    db_url = os.environ.get(
        "DATABASE_URL",
        "postgres://peter:peter@localhost:5432/youtube_tracker?sslmode=disable",
    )
    conn = psycopg2.connect(db_url)
    try:
        with conn.cursor() as cur:
            cur.execute("SELECT id, chunk_text FROM public.knowledge_chunks")
            rows = cur.fetchall()

        decisions = [(is_ad_chunk(t or ""), cid) for cid, t in rows]
        with conn.cursor() as cur:
            execute_batch(
                cur,
                "UPDATE public.knowledge_chunks SET is_ad = %s WHERE id = %s",
                decisions,
            )
        conn.commit()

        true_cnt = sum(1 for d, _ in decisions if d)
        print(f"[update_is_ad] Всего чанков: {len(decisions)}, is_ad=true: {true_cnt}")

        with conn.cursor() as cur:
            cur.execute("""
                SELECT channel_name, COUNT(*) FILTER (WHERE is_ad), COUNT(*)
                FROM public.knowledge_chunks
                GROUP BY channel_name
                HAVING COUNT(*) FILTER (WHERE is_ad) > 0
                ORDER BY 2 DESC
            """)
            print("\n[update_is_ad] is_ad / total по каналам:")
            for ch, ad, tot in cur.fetchall():
                print(f"  {ch}: {ad}/{tot}")
    finally:
        conn.close()


if __name__ == "__main__":
    main()