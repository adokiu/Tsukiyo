-- 旧 image_id 列不再使用，改为可空
ALTER TABLE node_images ALTER COLUMN image_id DROP NOT NULL;
