UPDATE public.video_files_praktika
SET file_name = regexp_replace(file_name, '\.md$', '.txt')
WHERE file_name LIKE '%.md';