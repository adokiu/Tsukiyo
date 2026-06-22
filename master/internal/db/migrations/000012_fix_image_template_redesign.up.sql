-- 修复迁移：000011 可能因 dirty force 跳过，此处幂等执行全部 DDL
-- 如果 fingerprint 列已存在则全部跳过（IF NOT EXISTS 保证幂等）

ALTER TABLE node_images ADD COLUMN IF NOT EXISTS fingerprint VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE node_images ADD COLUMN IF NOT EXISTS alias VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE node_images ADD COLUMN IF NOT EXISTS image_type VARCHAR(20) NOT NULL DEFAULT '';
ALTER TABLE node_images ADD COLUMN IF NOT EXISTS architecture VARCHAR(50) NOT NULL DEFAULT '';
ALTER TABLE node_images ADD COLUMN IF NOT EXISTS size_bytes BIGINT NOT NULL DEFAULT 0;
ALTER TABLE node_images ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
ALTER TABLE node_images ADD COLUMN IF NOT EXISTS upload_date VARCHAR(50) NOT NULL DEFAULT '';
ALTER TABLE node_images ADD COLUMN IF NOT EXISTS image_source VARCHAR(50) NOT NULL DEFAULT 'manual';

DROP INDEX IF EXISTS idx_node_image;

-- 清理重复数据
DELETE FROM node_images a USING node_images b
WHERE a.id < b.id AND a.node_id = b.node_id
  AND COALESCE(a.fingerprint, '') = COALESCE(b.fingerprint, '')
  AND COALESCE(a.image_type, '') = COALESCE(b.image_type, '');

-- 用旧 image_id 填充新字段（格式 alias|type|arch）
UPDATE node_images SET fingerprint = split_part(image_id, '|', 1) WHERE fingerprint = '' AND image_id IS NOT NULL AND image_id != '';
UPDATE node_images SET image_type = split_part(image_id, '|', 2) WHERE image_type = '' AND image_id IS NOT NULL AND image_id != '';
UPDATE node_images SET architecture = split_part(image_id, '|', 3) WHERE architecture = '' AND image_id IS NOT NULL AND image_id != '';

-- 再次清理
DELETE FROM node_images a USING node_images b
WHERE a.id < b.id AND a.node_id = b.node_id
  AND COALESCE(a.fingerprint, '') = COALESCE(b.fingerprint, '')
  AND COALESCE(a.image_type, '') = COALESCE(b.image_type, '');

CREATE INDEX IF NOT EXISTS idx_node_image ON node_images (node_id, fingerprint, image_type);
CREATE UNIQUE INDEX IF NOT EXISTS idx_node_image_unique ON node_images (node_id, fingerprint, image_type);

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

-- 补充 instances 表缺失的 IP 地址列
ALTER TABLE instances ADD COLUMN IF NOT EXISTS ipv4_address VARCHAR(64) DEFAULT '';
ALTER TABLE instances ADD COLUMN IF NOT EXISTS ipv6_address VARCHAR(128) DEFAULT '';
