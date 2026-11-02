-- Таблица для отслеживания прогресса индексации (чекпоинты)
-- Позволяет продолжить индексацию с последнего успешного видео после сбоя

CREATE TABLE IF NOT EXISTS public.indexing_jobs (
    id              SERIAL PRIMARY KEY,
    video_id        TEXT NOT NULL UNIQUE,
    status          TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'done', 'failed')),
    chunks_count    INTEGER NOT NULL DEFAULT 0,
    embedding_model TEXT NOT NULL DEFAULT 'unknown',
    error_message   TEXT,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Индекс для быстрого поиска необработанных задач
CREATE INDEX IF NOT EXISTS idx_indexing_jobs_status ON public.indexing_jobs(status);

-- Индекс для поиска по video_id
CREATE INDEX IF NOT EXISTS idx_indexing_jobs_video_id ON public.indexing_jobs(video_id);