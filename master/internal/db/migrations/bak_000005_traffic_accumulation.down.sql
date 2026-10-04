ALTER TABLE instances DROP COLUMN IF EXISTS last_net_in_total;
ALTER TABLE instances DROP COLUMN IF EXISTS last_net_out_total;
ALTER TABLE instances DROP COLUMN IF EXISTS monthly_traffic_in_bytes;
ALTER TABLE instances DROP COLUMN IF EXISTS monthly_traffic_out_bytes;
