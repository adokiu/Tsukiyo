-- 回滚：IOPS改回MB/s
ALTER TABLE instances RENAME COLUMN io_read_iops TO io_read_mbps;
ALTER TABLE instances RENAME COLUMN io_write_iops TO io_write_mbps;

ALTER TABLE instance_metrics DROP COLUMN IF EXISTS disk_read_iops;
ALTER TABLE instance_metrics DROP COLUMN IF EXISTS disk_write_iops;
