-- 磁盘IO限制从MB/s改为IOPS
-- instances表：io_read_mbps -> io_read_iops, io_write_mbps -> io_write_iops
ALTER TABLE instances RENAME COLUMN io_read_mbps TO io_read_iops;
ALTER TABLE instances RENAME COLUMN io_write_mbps TO io_write_iops;

-- instance_metrics表：加磁盘读写IOPS列
ALTER TABLE instance_metrics ADD COLUMN IF NOT EXISTS disk_read_iops bigint DEFAULT 0;
ALTER TABLE instance_metrics ADD COLUMN IF NOT EXISTS disk_write_iops bigint DEFAULT 0;
