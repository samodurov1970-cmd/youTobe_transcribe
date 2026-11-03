BEGIN;
INSERT INTO public.video_files_praktika (video_id, channel_name, video_url, channel_topic, file_name, file_content, validation_status, is_in_obsidian, is_chunked) VALUES ('noid_0', 'unknown', '', 'обучение_торговле_на_рынках', 'noid_0.txt', pg_read_file('/tmp/chunks/noid_0.txt'), 'pending', false, false) ON CONFLICT (video_id) DO NOTHING;
COMMIT;


