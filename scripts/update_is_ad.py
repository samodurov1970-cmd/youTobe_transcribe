#!/usr/bin/env python3
"""update_is_ad.py — обновляет флаг is_ad без пересчёта эмбеддингов."""
import os
import re
import psycopg2


def is_ad_chunk(text: str) -> bool:
    if not text:
        return False
    low = text.lower()

    # 1. Жёсткие маркеры (сами по себе = реклама)
    hard = [
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
        # русские рекламные конструкции
        r"мастер[-\s]?класс",
        r"напиши(те)?\s+помощнику",
        r"обращайтесь\s+к\s+(нашим\s+)?менеджерам",
        r"кнопка\s+(вот\s+)?здесь\s+(находится\s+)?под\s+видео",
        r"подключайтесь\s+ребята",
        r"ссылка\s+в\s+описании.{0,60}(подписаться|подключиться|перейти)",
        r"выгодно\s+и\s+надежно",
        r"покупайте\s+золотые",
        r"покупайте\s+серебряные",
        r"инвестиционные\s+монеты",
        r"георгий\s+победоносец",
    ]
    for p in hard:
        if re.search(p, low):
            return True

    # 2. Конкретный спонсор + слово sponsor/thanks to
    brands = (
        r"\bola\s*prime\b|\bblack\s*bull\b|\btop\s*one\b|\btopone\b|"
        r"\bedge\s*finder\b|\bedgefinder\b|\bstock\s*box\b|\bstockbox\b"
    )
    if re.search(r"sponsor|спонсор", low) and re.search(brands, low):
        return True

    # 3. Слабый продукт + ЯВНЫЙ sale-контекст в окне 120 символов
    weak = r"\bedge\s*finder\b|\bedgefinder\b|\bstock\s*box\b|\bstockbox\b"
    sale = (r"black\s*friday|%\s*off|\bsale\b|\bdiscount\b|limited\s+time|"
            r"free\s+50\s*k|evaluation\s+account|\bvip\s+discord|"
            r"information\s+(is\s+)?(in|below)\s+(the\s+)?description")
    for m in re.finditer(weak, low):
        win = low[max(0, m.start()-120): m.end()+120]
        if re.search(sale, win):
            return True

    return False


def get_db_conn():
    db_url = os.environ.get(
        "DATABASE_URL",
        "postgres://peter:peter@localhost:5432/youtube_tracker?sslmode=disable",
    )
    return psycopg2.connect(db_url)


def main() -> None:
    conn = get_db_conn()
    try:
        with conn.cursor() as cur:
            cur.execute("SELECT id, chunk_text FROM public.knowledge_chunks")
            rows = cur.fetchall()
        print(f"[update_is_ad] Найдено чанков: {len(rows)}")
        true_count = 0
        with conn.cursor() as cur:
            for chunk_id, text in rows:
                new_ad = is_ad_chunk(text or "")
                if new_ad:
                    true_count += 1
                cur.execute(
                    "UPDATE public.knowledge_chunks SET is_ad = %s WHERE id = %s",
                    (new_ad, chunk_id),
                )
        conn.commit()
        print(f"[update_is_ad] is_ad=true: {true_count}")
        with conn.cursor() as cur:
            cur.execute("""
                SELECT channel_name, COUNT(*)
                FROM public.knowledge_chunks
                WHERE is_ad = true
                GROUP BY channel_name
                ORDER BY COUNT(*) DESC
            """)
            print("\n[update_is_ad] is_ad=true по каналам:")
            for ch, cnt in cur.fetchall():
                print(f"  {ch}: {cnt}")
    finally:
        conn.close()


if __name__ == "__main__":
    main()