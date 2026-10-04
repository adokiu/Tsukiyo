-- users 表补充 balance_cents 字段 (000007 执行时遗漏)
ALTER TABLE users ADD COLUMN IF NOT EXISTS balance_cents BIGINT DEFAULT 0;
