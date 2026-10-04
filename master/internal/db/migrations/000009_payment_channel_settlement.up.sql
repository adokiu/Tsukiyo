-- 补充 payment_channels 缺失的结算相关字段
ALTER TABLE payment_channels ADD COLUMN IF NOT EXISTS settlement_currency VARCHAR(8) DEFAULT 'CNY';
ALTER TABLE payment_channels ADD COLUMN IF NOT EXISTS exchange_rate_markup DECIMAL(5,2) DEFAULT 0;
