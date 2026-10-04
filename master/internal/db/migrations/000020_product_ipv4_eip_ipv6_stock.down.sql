-- 删除 IPv4 EIP 多IP增配配置
ALTER TABLE products DROP COLUMN IF EXISTS ipv4_eip_min;
ALTER TABLE products DROP COLUMN IF EXISTS ipv4_eip_max;
ALTER TABLE products DROP COLUMN IF EXISTS ipv4_eip_custom_mode;
ALTER TABLE products DROP COLUMN IF EXISTS ipv4_eip_tiers;

-- 删除 IPv6 多前缀增配配置
ALTER TABLE products DROP COLUMN IF EXISTS ipv6_configs;

-- 删除库存配置
ALTER TABLE products DROP COLUMN IF EXISTS stock;

-- 恢复旧字段
ALTER TABLE products ADD COLUMN IF NOT EXISTS ipv4_eip_allow_custom BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE products ADD COLUMN IF NOT EXISTS ipv6_prefix_len INT NOT NULL DEFAULT 128;
