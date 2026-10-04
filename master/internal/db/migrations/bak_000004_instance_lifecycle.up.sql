-- DataDisk 扩展：添加 status 和 updated_at 字段
ALTER TABLE data_disks ADD COLUMN IF NOT EXISTS status VARCHAR(16) NOT NULL DEFAULT 'attached';
ALTER TABLE data_disks ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- SiteConfig 添加 auto_release_days 字段
ALTER TABLE site_configs ADD COLUMN IF NOT EXISTS auto_release_days INTEGER NOT NULL DEFAULT 7;

-- 实例添加 expired_at 字段（记录过期时间点，区别于 expires_at 过期策略时间）
ALTER TABLE instances ADD COLUMN IF NOT EXISTS expired_at TIMESTAMPTZ;
