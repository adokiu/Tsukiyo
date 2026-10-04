-- 回滚 DataDisk 扩展
ALTER TABLE data_disks DROP COLUMN IF EXISTS status;
ALTER TABLE data_disks DROP COLUMN IF EXISTS updated_at;

-- 回滚 SiteConfig
ALTER TABLE site_configs DROP COLUMN IF EXISTS auto_release_days;

-- 回滚实例 expired_at
ALTER TABLE instances DROP COLUMN IF EXISTS expired_at;
