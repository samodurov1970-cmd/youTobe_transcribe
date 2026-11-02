-- Миграция knowledge_chunks: vector(384) → vector(768) для multilingual-e5-base
-- Сохраняет структуру, индексы и данные (данные будут перезаписаны chunk_and_embed.py --mode full)

BEGIN;

-- 1. Дропаем старый HNSW-индекс (если есть), чтобы не мешал
DROP INDEX IF EXISTS public.idx_knowledge_chunks_embedding;

-- 2. Добавляем новую колонку с 768 измерениями
ALTER TABLE public.knowledge_chunks
    ADD COLUMN embedding_new vector(768);

-- 3. Дропаем старую колонку
ALTER TABLE public.knowledge_chunks
    DROP COLUMN embedding;

-- 4. Переименовываем
ALTER TABLE public.knowledge_chunks
    RENAME COLUMN embedding_new TO embedding;

-- 5. Пересоздаём HNSW-индекс
CREATE INDEX idx_knowledge_chunks_embedding
    ON public.knowledge_chunks
    USING hnsw (embedding vector_cosine_ops);

COMMIT;