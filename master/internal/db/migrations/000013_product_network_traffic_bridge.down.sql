-- 恢复网络带宽单价
ALTER TABLE products DROP COLUMN IF EXISTS network_down_unit_price_cents;
ALTER TABLE products DROP COLUMN IF EXISTS network_up_unit_price_cents;
ALTER TABLE products ADD COLUMN IF NOT EXISTS network_unit_price_cents BIGINT NOT NULL DEFAULT 0;

-- 恢复流量限制
ALTER TABLE products DROP COLUMN IF EXISTS traffic_min_gb;
ALTER TABLE products DROP COLUMN IF EXISTS traffic_max_gb;
ALTER TABLE products ADD COLUMN IF NOT EXISTS traffic_limit_gb INT NOT NULL DEFAULT 0;

-- 删除网桥配置
ALTER TABLE products DROP COLUMN IF EXISTS node_bridges;
