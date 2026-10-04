-- node_images 表添加 status 列
ALTER TABLE node_images ADD COLUMN IF NOT EXISTS status VARCHAR(16) DEFAULT 'downloaded';
