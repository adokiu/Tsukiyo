-- NAT 端口映射配置
ALTER TABLE products DROP COLUMN IF EXISTS nat_port_min;
ALTER TABLE products DROP COLUMN IF EXISTS nat_port_max;
ALTER TABLE products DROP COLUMN IF EXISTS nat_port_custom_mode;
ALTER TABLE products DROP COLUMN IF EXISTS nat_port_tiers;
ALTER TABLE products DROP COLUMN IF EXISTS nat_port_unit_price_cents;

-- 下行带宽 custom_mode 和 tiers
ALTER TABLE products DROP COLUMN IF EXISTS network_down_custom_mode;
ALTER TABLE products DROP COLUMN IF EXISTS network_down_tiers;

-- 上行带宽 custom_mode 和 tiers
ALTER TABLE products DROP COLUMN IF EXISTS network_up_custom_mode;
ALTER TABLE products DROP COLUMN IF EXISTS network_up_tiers;

-- 流量 custom_mode 和 tiers
ALTER TABLE products DROP COLUMN IF EXISTS traffic_custom_mode;
ALTER TABLE products DROP COLUMN IF EXISTS traffic_tiers;
