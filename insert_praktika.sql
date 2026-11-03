INSERT INTO public.video_files
    (video_id, channel_name, video_url, channel_topic,
     file_name, file_content, validation_status, is_in_obsidian, is_chunked)
VALUES
    (NULL,
     'Практика',
     NULL,
     'обучение_торговле_на_рынках',
     'Практика.md',
     pg_read_file('/tmp/praktika.md'),
     'pending',
     false,
     false);
