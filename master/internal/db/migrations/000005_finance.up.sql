-- 财务系统表结构

-- 支付渠道
CREATE TABLE IF NOT EXISTS payment_channels (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(64) NOT NULL,
    type VARCHAR(32) NOT NULL,
    config TEXT DEFAULT '',
    min_amount BIGINT DEFAULT 0,
    max_amount BIGINT DEFAULT 0,
    settlement_currency VARCHAR(8) DEFAULT 'CNY',
    exchange_rate_markup DECIMAL(5,2) DEFAULT 0,
    status VARCHAR(16) DEFAULT 'enabled',
    sort_order INT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_payment_channels_status ON payment_channels(status);
CREATE INDEX IF NOT EXISTS idx_payment_channels_deleted_at ON payment_channels(deleted_at);

-- 支付方式 (渠道下的子方式, 如信用卡/借记卡/钱包等)
CREATE TABLE IF NOT EXISTS payment_methods (
    id BIGSERIAL PRIMARY KEY,
    channel_id BIGINT NOT NULL,
    name VARCHAR(64) NOT NULL,
    type VARCHAR(32) NOT NULL,
    icon VARCHAR(256) DEFAULT '',
    description TEXT DEFAULT '',
    fee_bearer VARCHAR(16) DEFAULT 'user',
    fee_type VARCHAR(32) DEFAULT 'fixed',
    fee_fixed_cents BIGINT DEFAULT 0,
    fee_percent DECIMAL(8,4) DEFAULT 0,
    min_amount BIGINT DEFAULT 0,
    max_amount BIGINT DEFAULT 0,
    status VARCHAR(16) DEFAULT 'enabled',
    sort_order INT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_payment_methods_channel_id ON payment_methods(channel_id);
CREATE INDEX IF NOT EXISTS idx_payment_methods_status ON payment_methods(status);
CREATE INDEX IF NOT EXISTS idx_payment_methods_sort_order ON payment_methods(sort_order);

-- 账单
CREATE TABLE IF NOT EXISTS bills (
    id BIGSERIAL PRIMARY KEY,
    bill_no VARCHAR(64) NOT NULL,
    user_id BIGINT NOT NULL,
    username VARCHAR(64) DEFAULT '',
    type VARCHAR(32) NOT NULL,
    status VARCHAR(16) DEFAULT 'pending',
    amount_cents BIGINT NOT NULL,
    balance_before BIGINT DEFAULT 0,
    balance_after BIGINT DEFAULT 0,
    payment_channel_id BIGINT,
    payment_channel_name VARCHAR(64) DEFAULT '',
    transaction_no VARCHAR(128) DEFAULT '',
    description TEXT DEFAULT '',
    instance_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bills_bill_no ON bills(bill_no);
CREATE INDEX IF NOT EXISTS idx_bills_user_id ON bills(user_id);
CREATE INDEX IF NOT EXISTS idx_bills_status ON bills(status);
CREATE INDEX IF NOT EXISTS idx_bills_type ON bills(type);
CREATE INDEX IF NOT EXISTS idx_bills_created_at ON bills(created_at);
CREATE INDEX IF NOT EXISTS idx_bills_instance_id ON bills(instance_id);

-- 钱包流水
CREATE TABLE IF NOT EXISTS wallet_transactions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    amount_cents BIGINT NOT NULL,
    type VARCHAR(32) NOT NULL,
    bill_id BIGINT,
    description TEXT DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_wallet_transactions_user_id ON wallet_transactions(user_id);
CREATE INDEX IF NOT EXISTS idx_wallet_transactions_type ON wallet_transactions(type);
CREATE INDEX IF NOT EXISTS idx_wallet_transactions_bill_id ON wallet_transactions(bill_id);
CREATE INDEX IF NOT EXISTS idx_wallet_transactions_created_at ON wallet_transactions(created_at);
