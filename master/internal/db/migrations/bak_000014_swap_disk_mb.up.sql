-- 添加 swap_mb 列
ALTER TABLE instances ADD COLUMN IF NOT EXISTS swap_mb integer NOT NULL DEFAULT 0;

-- 将 disk_gb 转为 disk_mb（1GB = 1024MB）
ALTER TABLE instances ADD COLUMN IF NOT EXISTS disk_mb integer NOT NULL DEFAULT 10240;
UPDATE instances SET disk_mb = disk_gb * 1024;
ALTER TABLE instances DROP COLUMN IF EXISTS disk_gb;

-- 将 data_disks.size_gb 转为 size_mb
ALTER TABLE data_disks ADD COLUMN IF NOT EXISTS size_mb integer NOT NULL DEFAULT 10240;
UPDATE data_disks SET size_mb = size_gb * 1024;
ALTER TABLE data_disks DROP COLUMN IF EXISTS size_gb;
