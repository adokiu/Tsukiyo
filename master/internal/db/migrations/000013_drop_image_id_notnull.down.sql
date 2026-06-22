-- 恢复 image_id NOT NULL 约束
ALTER TABLE node_images ALTER COLUMN image_id SET NOT NULL;
