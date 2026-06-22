-- 重构 node_images 表：agent 上报完整镜像信息
ALTER TABLE node_images ADD COLUMN IF NOT EXISTS fingerprint VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE node_images ADD COLUMN IF NOT EXISTS alias VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE node_images ADD COLUMN IF NOT EXISTS image_type VARCHAR(20) NOT NULL DEFAULT '';
ALTER TABLE node_images ADD COLUMN IF NOT EXISTS architecture VARCHAR(50) NOT NULL DEFAULT '';
ALTER TABLE node_images ADD COLUMN IF NOT EXISTS size_bytes BIGINT NOT NULL DEFAULT 0;
ALTER TABLE node_images ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
ALTER TABLE node_images ADD COLUMN IF NOT EXISTS upload_date VARCHAR(50) NOT NULL DEFAULT '';
ALTER TABLE node_images ADD COLUMN IF NOT EXISTS image_source VARCHAR(50) NOT NULL DEFAULT 'manual';

-- 更新索引：旧的 (node_id, image_id) 改为 (node_id, fingerprint, image_type)
DROP INDEX IF EXISTS idx_node_image;

-- 清理重复数据：旧数据中 fingerprint 和 image_type 均为空字符串，按 node_id 去重保留最新一条
DELETE FROM node_images a USING node_images b
WHERE a.id < b.id AND a.node_id = b.node_id
  AND COALESCE(a.fingerprint, '') = COALESCE(b.fingerprint, '')
  AND COALESCE(a.image_type, '') = COALESCE(b.image_type, '');

-- 对于旧数据中 fingerprint 为空的记录，尝试用 alias 填充 fingerprint（旧 image_id 格式为 alias|type|arch）
UPDATE node_images SET fingerprint = split_part(image_id, '|', 1) WHERE fingerprint = '' AND image_id IS NOT NULL AND image_id != '';
UPDATE node_images SET image_type = split_part(image_id, '|', 2) WHERE image_type = '' AND image_id IS NOT NULL AND image_id != '';
UPDATE node_images SET architecture = split_part(image_id, '|', 3) WHERE architecture = '' AND image_id IS NOT NULL AND image_id != '';

-- 再次清理可能因填充产生的重复
DELETE FROM node_images a USING node_images b
WHERE a.id < b.id AND a.node_id = b.node_id
  AND COALESCE(a.fingerprint, '') = COALESCE(b.fingerprint, '')
  AND COALESCE(a.image_type, '') = COALESCE(b.image_type, '');

CREATE INDEX IF NOT EXISTS idx_node_image ON node_images (node_id, fingerprint, image_type);

-- 添加唯一约束：同一节点同一指纹同一类型只能有一条记录
CREATE UNIQUE INDEX IF NOT EXISTS idx_node_image_unique ON node_images (node_id, fingerprint, image_type);

-- 节点镜像分类表
CREATE TABLE IF NOT EXISTS node_image_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id UUID NOT NULL,
    name VARCHAR(100) NOT NULL,
    image_type VARCHAR(20) NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_node_image_category ON node_image_categories (node_id, image_type);
CREATE UNIQUE INDEX IF NOT EXISTS idx_node_image_category_unique ON node_image_categories (node_id, name, image_type);

-- 镜像别名映射表
CREATE TABLE IF NOT EXISTS node_image_aliases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id UUID NOT NULL,
    fingerprint VARCHAR(64) NOT NULL,
    image_type VARCHAR(20) NOT NULL,
    category_id UUID REFERENCES node_image_categories(id) ON DELETE SET NULL,
    display_name VARCHAR(200) NOT NULL DEFAULT '',
    install_ssh BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_node_image_alias ON node_image_aliases (node_id, fingerprint, image_type);
CREATE UNIQUE INDEX IF NOT EXISTS idx_node_image_alias_unique ON node_image_aliases (node_id, fingerprint, image_type);
