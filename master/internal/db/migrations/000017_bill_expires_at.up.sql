-- 账单表添加过期时间字段
ALTER TABLE bills ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;
