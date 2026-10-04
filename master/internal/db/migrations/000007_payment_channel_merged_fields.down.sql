-- 回滚: 移除合并的字段
ALTER TABLE payment_channels DROP COLUMN IF EXISTS icon;
ALTER TABLE payment_channels DROP COLUMN IF EXISTS description;
ALTER TABLE payment_channels DROP COLUMN IF EXISTS fee_bearer;
ALTER TABLE payment_channels DROP COLUMN IF EXISTS fee_type;
ALTER TABLE payment_channels DROP COLUMN IF EXISTS fee_fixed_cents;
ALTER TABLE payment_channels DROP COLUMN IF EXISTS fee_percent;
