-- 回滚: 移除新增字段
ALTER TABLE payment_channels DROP COLUMN IF EXISTS settlement_currency;
ALTER TABLE payment_channels DROP COLUMN IF EXISTS exchange_rate_markup;
