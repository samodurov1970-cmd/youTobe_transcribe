-- Вставка результата тренировочной транскрибации в video_files_praktika.
-- Требует: /tmp/test_out.txt внутри контейнера postgres.

INSERT INTO public.video_files_praktika (
    video_id,
    channel_name,
    video_url,
    channel_topic,
    file_name,
    file_content
)
SELECT
    'pe4t_d9LZU0',
    'LP Research',
    'https://www.youtube.com/watch?v=pe4t_d9LZU0',
    'обучение_торговле_на_рынках',
    'pe4t_d9LZU0.md',
    pg_read_file('/tmp/test_out.txt')
WHERE NOT EXISTS (
    SELECT 1 FROM public.video_files_praktika WHERE video_id = 'pe4t_d9LZU0'
);