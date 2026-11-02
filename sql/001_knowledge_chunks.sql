-- Таблица для хранения чанков транскрипций с векторными эмбеддингами
-- Размерность вектора фиксируется один раз при создании (например, 768 для multilingual-e5-base)
-- При смене модели эмбеддингов таблицу нужно пересоздавать и пересчитывать все векторы

CREATE TABLE IF NOT EXISTS public.knowledge_chunks (
    id              SERIAL PRIMARY KEY,
    video_id        TEXT NOT NULL,
    channel_name    TEXT NOT NULL,
    video_url       TEXT NOT NULL,
    chunk_index     INTEGER NOT NULL,
    chunk_text      TEXT NOT NULL,
    embedding       vector(384) NOT NULL,
    lang            TEXT NOT NULL CHECK (lang IN ('ru', 'en')),
    is_ad           BOOLEAN NOT NULL DEFAULT FALSE,
    is_orphan       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Индекс для быстрого поиска по видео
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_video_id ON public.knowledge_chunks(video_id);

-- Индекс для диверсификации по каналу
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_channel ON public.knowledge_chunks(channel_name);

-- Индекс для фильтрации по языку и рекламе
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_lang_ad ON public.knowledge_chunks(lang, is_ad);

-- Индекс для поиска сирот
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_orphan ON public.knowledge_chunks(is_orphan);

-- Полнотекстовый индекс для точного поиска тикеров, стратегий, брокеров
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_text_ru ON public.knowledge_chunks
USING gin (to_tsvector('russian', chunk_text));

CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_text_en ON public.knowledge_chunks
USING gin (to_tsvector('english', chunk_text));