-- 流量累计字段
ALTER TABLE instances ADD COLUMN IF NOT EXISTS last_net_in_total BIGINT DEFAULT 0;
ALTER TABLE instances ADD COLUMN IF NOT EXISTS last_net_out_total BIGINT DEFAULT 0;
ALTER TABLE instances ADD COLUMN IF NOT EXISTS monthly_traffic_in_bytes BIGINT DEFAULT 0;
ALTER TABLE instances ADD COLUMN IF NOT EXISTS monthly_traffic_out_bytes BIGINT DEFAULT 0;
