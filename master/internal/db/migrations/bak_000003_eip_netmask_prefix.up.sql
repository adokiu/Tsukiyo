-- 给 eip_pools 表添加 netmask_prefix 字段
ALTER TABLE eip_pools ADD COLUMN IF NOT EXISTS netmask_prefix INTEGER NOT NULL DEFAULT 0;

-- 给 eip_allocations 表添加 alias 和 mapped_internal_ip 字段
ALTER TABLE eip_allocations ADD COLUMN IF NOT EXISTS alias VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE eip_allocations ADD COLUMN IF NOT EXISTS mapped_internal_ip VARCHAR(64) NOT NULL DEFAULT '';
