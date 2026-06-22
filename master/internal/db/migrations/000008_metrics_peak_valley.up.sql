-- 添加监控指标峰值/谷值字段（降采样后保留峰值谷值）
ALTER TABLE instance_metrics ADD COLUMN IF NOT EXISTS cpu_max float DEFAULT 0;
ALTER TABLE instance_metrics ADD COLUMN IF NOT EXISTS cpu_min float DEFAULT 0;
ALTER TABLE instance_metrics ADD COLUMN IF NOT EXISTS mem_used_max bigint DEFAULT 0;
ALTER TABLE instance_metrics ADD COLUMN IF NOT EXISTS mem_used_min bigint DEFAULT 0;
ALTER TABLE instance_metrics ADD COLUMN IF NOT EXISTS disk_used_max bigint DEFAULT 0;
ALTER TABLE instance_metrics ADD COLUMN IF NOT EXISTS disk_used_min bigint DEFAULT 0;
ALTER TABLE instance_metrics ADD COLUMN IF NOT EXISTS disk_read_max bigint DEFAULT 0;
ALTER TABLE instance_metrics ADD COLUMN IF NOT EXISTS disk_write_max bigint DEFAULT 0;
ALTER TABLE instance_metrics ADD COLUMN IF NOT EXISTS net_in_max bigint DEFAULT 0;
ALTER TABLE instance_metrics ADD COLUMN IF NOT EXISTS net_in_min bigint DEFAULT 0;
ALTER TABLE instance_metrics ADD COLUMN IF NOT EXISTS net_out_max bigint DEFAULT 0;
ALTER TABLE instance_metrics ADD COLUMN IF NOT EXISTS net_out_min bigint DEFAULT 0;
ALTER TABLE instance_metrics ADD COLUMN IF NOT EXISTS sample_count int DEFAULT 1;

-- 为历史数据查询优化（按实例ID+时间范围查询）
CREATE INDEX IF NOT EXISTS idx_instance_metrics_instance_time ON instance_metrics (instance_id, timestamp);
