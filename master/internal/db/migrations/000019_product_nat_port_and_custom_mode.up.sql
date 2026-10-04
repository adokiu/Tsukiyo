-- NAT 端口映射配置
ALTER TABLE products ADD COLUMN IF NOT EXISTS nat_port_min INT NOT NULL DEFAULT 0;
ALTER TABLE products ADD COLUMN IF NOT EXISTS nat_port_max INT NOT NULL DEFAULT 0;
ALTER TABLE products ADD COLUMN IF NOT EXISTS nat_port_custom_mode VARCHAR(16) NOT NULL DEFAULT 'unlimited';
ALTER TABLE products ADD COLUMN IF NOT EXISTS nat_port_tiers JSONB NOT NULL DEFAULT '[]';
ALTER TABLE products ADD COLUMN IF NOT EXISTS nat_port_unit_price_cents BIGINT NOT NULL DEFAULT 0;

-- 下行带宽 custom_mode 和 tiers
ALTER TABLE products ADD COLUMN IF NOT EXISTS network_down_custom_mode VARCHAR(16) NOT NULL DEFAULT 'unlimited';
ALTER TABLE products ADD COLUMN IF NOT EXISTS network_down_tiers JSONB NOT NULL DEFAULT '[]';

-- 上行带宽 custom_mode 和 tiers
ALTER TABLE products ADD COLUMN IF NOT EXISTS network_up_custom_mode VARCHAR(16) NOT NULL DEFAULT 'unlimited';
ALTER TABLE products ADD COLUMN IF NOT EXISTS network_up_tiers JSONB NOT NULL DEFAULT '[]';

-- 流量 custom_mode 和 tiers
ALTER TABLE products ADD COLUMN IF NOT EXISTS traffic_custom_mode VARCHAR(16) NOT NULL DEFAULT 'unlimited';
ALTER TABLE products ADD COLUMN IF NOT EXISTS traffic_tiers JSONB NOT NULL DEFAULT '[]';
