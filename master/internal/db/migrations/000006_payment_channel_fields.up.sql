-- 补充 payment_channels 新增字段 (合并原 payment_methods 的字段)

-- settlement_currency: 结算货币
ALTER TABLE payment_channels ADD COLUMN IF NOT EXISTS settlement_currency VARCHAR(8) DEFAULT 'CNY';

-- exchange_rate_markup: 汇率加价百分比
ALTER TABLE payment_channels ADD COLUMN IF NOT EXISTS exchange_rate_markup DECIMAL(5,2) DEFAULT 0;

-- icon: 图标URL
ALTER TABLE payment_channels ADD COLUMN IF NOT EXISTS icon VARCHAR(256) DEFAULT '';

-- description: 描述
ALTER TABLE payment_channels ADD COLUMN IF NOT EXISTS description TEXT DEFAULT '';

-- fee_bearer: 手续费承担方
ALTER TABLE payment_channels ADD COLUMN IF NOT EXISTS fee_bearer VARCHAR(16) DEFAULT 'user';

-- fee_type: 手续费计算类型
ALTER TABLE payment_channels ADD COLUMN IF NOT EXISTS fee_type VARCHAR(32) DEFAULT 'fixed';

-- fee_fixed_cents: 固定手续费 (分)
ALTER TABLE payment_channels ADD COLUMN IF NOT EXISTS fee_fixed_cents BIGINT DEFAULT 0;

-- fee_percent: 百分比手续费
ALTER TABLE payment_channels ADD COLUMN IF NOT EXISTS fee_percent DECIMAL(8,4) DEFAULT 0;

-- 移除旧字段 fee_rate (如果存在)
ALTER TABLE payment_channels DROP COLUMN IF EXISTS fee_rate;

-- 删除 payment_methods 表 (不再需要)
DROP TABLE IF EXISTS payment_methods;

