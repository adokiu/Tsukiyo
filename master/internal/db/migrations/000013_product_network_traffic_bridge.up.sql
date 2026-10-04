-- 网络带宽单价拆分为上下行
ALTER TABLE products DROP COLUMN IF EXISTS network_unit_price_cents;
ALTER TABLE products ADD COLUMN IF NOT EXISTS network_down_unit_price_cents BIGINT NOT NULL DEFAULT 0;
ALTER TABLE products ADD COLUMN IF NOT EXISTS network_up_unit_price_cents BIGINT NOT NULL DEFAULT 0;

-- 流量改为最低/最高
ALTER TABLE products DROP COLUMN IF EXISTS traffic_limit_gb;
ALTER TABLE products ADD COLUMN IF NOT EXISTS traffic_min_gb INT NOT NULL DEFAULT 0;
ALTER TABLE products ADD COLUMN IF NOT EXISTS traffic_max_gb INT NOT NULL DEFAULT 0;

-- 网桥配置（按宿主机分别选择）
ALTER TABLE products ADD COLUMN IF NOT EXISTS node_bridges JSONB NOT NULL DEFAULT '{}';
