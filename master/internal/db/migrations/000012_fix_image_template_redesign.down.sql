-- 回滚：删除迁移12新增的结构（与迁移11 down相同）
DROP TABLE IF EXISTS node_image_aliases;
DROP TABLE IF EXISTS node_image_categories;
DROP INDEX IF EXISTS idx_node_image_unique;
DROP INDEX IF EXISTS idx_node_image;
CREATE INDEX IF NOT EXISTS idx_node_image ON node_images (node_id, image_id);
