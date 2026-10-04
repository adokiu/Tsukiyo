-- 移除 IPv6 NAT 出口字段，新增 IPv6 EIP 池关联
ALTER TABLE bridges DROP COLUMN IF EXISTS nat_egress_ipv6_id;
DROP INDEX IF EXISTS idx_bridges_nat_egress_ipv6_id;

-- 新增 IPv6 EIP 池 ID 字段（GORM 将 IPv6EIPPoolID 转为 ipv6_e_ip_pool_id）
ALTER TABLE bridges ADD COLUMN IF NOT EXISTS ipv6_e_ip_pool_id UUID;
CREATE INDEX IF NOT EXISTS idx_bridges_ipv6_e_ip_pool_id ON bridges (ipv6_e_ip_pool_id);
