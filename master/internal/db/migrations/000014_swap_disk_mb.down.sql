-- 恢复 disk_gb 列
ALTER TABLE instances ADD COLUMN IF NOT EXISTS disk_gb integer NOT NULL DEFAULT 10;
UPDATE instances SET disk_gb = disk_mb / 1024;
ALTER TABLE instances DROP COLUMN IF EXISTS disk_mb;

-- 恢复 data_disks.size_gb 列
ALTER TABLE data_disks ADD COLUMN IF NOT EXISTS size_gb integer NOT NULL DEFAULT 10;
UPDATE data_disks SET size_gb = size_mb / 1024;
ALTER TABLE data_disks DROP COLUMN IF EXISTS size_mb;

-- 删除 swap_mb 列
ALTER TABLE instances DROP COLUMN IF EXISTS swap_mb;
