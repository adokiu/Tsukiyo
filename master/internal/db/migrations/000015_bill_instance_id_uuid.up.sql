-- 将 bills.instance_id 从 bigint 改为 uuid
-- 先清除旧数据（旧 bigint 值无法对应 uuid）
UPDATE bills SET instance_id = NULL WHERE instance_id IS NOT NULL;

ALTER TABLE bills ALTER COLUMN instance_id DROP NOT NULL;
ALTER TABLE bills ALTER COLUMN instance_id TYPE uuid USING NULL;

-- 重建索引
DROP INDEX IF EXISTS idx_bills_instance_id;
CREATE INDEX IF NOT EXISTS idx_bills_instance_id ON bills(instance_id);
