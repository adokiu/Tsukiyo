-- 回滚：恢复 nat_egress_ipv6_id，移除 ipv6_e_ip_pool_id
ALTER TABLE bridges DROP COLUMN IF EXISTS ipv6_e_ip_pool_id;
DROP INDEX IF EXISTS idx_bridges_ipv6_e_ip_pool_id;

ALTER TABLE bridges ADD COLUMN IF NOT EXISTS nat_egress_ipv6_id UUID;
CREATE INDEX IF NOT EXISTS idx_bridges_nat_egress_ipv6_id ON bridges (nat_egress_ipv6_id);
