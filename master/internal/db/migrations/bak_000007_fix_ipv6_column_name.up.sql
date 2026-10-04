-- 确保 GORM 期望的列名存在（兼容 000006 可能创建了错误列名的情况）
ALTER TABLE bridges ADD COLUMN IF NOT EXISTS ipv6_e_ip_pool_id UUID;

-- 如果旧列名 ipv6_eip_pool_id 存在，迁移数据后删除
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'bridges' AND column_name = 'ipv6_eip_pool_id') THEN
        UPDATE bridges SET ipv6_e_ip_pool_id = ipv6_eip_pool_id WHERE ipv6_e_ip_pool_id IS NULL AND ipv6_eip_pool_id IS NOT NULL;
        ALTER TABLE bridges DROP COLUMN ipv6_eip_pool_id;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_bridges_ipv6_e_ip_pool_id ON bridges (ipv6_e_ip_pool_id);
