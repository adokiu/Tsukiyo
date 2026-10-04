-- 补充 payment_channels 合并 payment_methods 后新增的字段
-- (000006 已执行过, golang-migrate 不会重新执行, 需要新迁移)

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

-- 删除 payment_methods 表 (不再需要)
DROP TABLE IF EXISTS payment_methods;
