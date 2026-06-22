-- 删除镜像别名映射表
DROP TABLE IF EXISTS node_image_aliases CASCADE;

-- 删除节点镜像分类表
DROP TABLE IF EXISTS node_image_categories CASCADE;

-- 恢复 node_images 表结构
DROP INDEX IF EXISTS idx_node_image_unique;
DROP INDEX IF EXISTS idx_node_image;
CREATE INDEX IF NOT EXISTS idx_node_image ON node_images (node_id, image_id);

ALTER TABLE node_images DROP COLUMN IF EXISTS image_source;
ALTER TABLE node_images DROP COLUMN IF EXISTS upload_date;
ALTER TABLE node_images DROP COLUMN IF EXISTS description;
ALTER TABLE node_images DROP COLUMN IF EXISTS size_bytes;
ALTER TABLE node_images DROP COLUMN IF EXISTS architecture;
ALTER TABLE node_images DROP COLUMN IF EXISTS image_type;
ALTER TABLE node_images DROP COLUMN IF EXISTS alias;
ALTER TABLE node_images DROP COLUMN IF EXISTS fingerprint;
