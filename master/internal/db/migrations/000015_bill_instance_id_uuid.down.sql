-- 将 bills.instance_id 从 uuid 改回 bigint
ALTER TABLE bills ALTER COLUMN instance_id TYPE bigint USING 0;

DROP INDEX IF EXISTS idx_bills_instance_id;
CREATE INDEX IF NOT EXISTS idx_bills_instance_id ON bills(instance_id);
