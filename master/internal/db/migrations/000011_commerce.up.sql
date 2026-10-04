-- 商品分类表
CREATE TABLE IF NOT EXISTS product_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_id UUID REFERENCES product_categories(id) ON DELETE SET NULL,
    name VARCHAR(128) NOT NULL,
    sort INT NOT NULL DEFAULT 0,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_product_categories_parent_id ON product_categories(parent_id);
CREATE INDEX IF NOT EXISTS idx_product_categories_deleted_at ON product_categories(deleted_at);

-- 商品表
CREATE TABLE IF NOT EXISTS products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id UUID REFERENCES product_categories(id) ON DELETE SET NULL,
    name VARCHAR(256) NOT NULL,
    type VARCHAR(16) NOT NULL DEFAULT 'container',
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    sort INT NOT NULL DEFAULT 0,

    -- 承载宿主机
    node_ids JSONB NOT NULL DEFAULT '[]',

    -- CPU 配置
    vcpu INT NOT NULL DEFAULT 1,
    vcpu_allow_custom BOOLEAN NOT NULL DEFAULT false,
    vcpu_unit_price_cents BIGINT NOT NULL DEFAULT 0,

    -- 内存配置
    memory_mb INT NOT NULL DEFAULT 1024,
    memory_allow_custom BOOLEAN NOT NULL DEFAULT false,
    memory_unit_price_cents BIGINT NOT NULL DEFAULT 0,

    -- 系统盘配置
    disk_mb INT NOT NULL DEFAULT 10240,
    disk_allow_custom BOOLEAN NOT NULL DEFAULT false,
    disk_unit_price_cents BIGINT NOT NULL DEFAULT 0,
    disk_storage_pools JSONB NOT NULL DEFAULT '{}',

    -- 数据盘配置
    data_disk_allow BOOLEAN NOT NULL DEFAULT true,
    data_disk_default_mb INT NOT NULL DEFAULT 0,
    data_disk_allow_custom BOOLEAN NOT NULL DEFAULT false,
    data_disk_unit_price_cents BIGINT NOT NULL DEFAULT 0,
    data_disk_storage_pools JSONB NOT NULL DEFAULT '{}',

    -- 网络带宽配置
    network_down_mbps INT NOT NULL DEFAULT 0,
    network_up_mbps INT NOT NULL DEFAULT 0,
    network_allow_custom BOOLEAN NOT NULL DEFAULT false,
    network_unit_price_cents BIGINT NOT NULL DEFAULT 0,

    -- 流量配置
    traffic_limit_gb INT NOT NULL DEFAULT 0,
    traffic_calc_mode VARCHAR(16) NOT NULL DEFAULT 'both',
    traffic_unit_price_cents BIGINT NOT NULL DEFAULT 0,

    -- IPv4 配置
    ipv4_mode VARCHAR(16) NOT NULL DEFAULT 'nat',
    ipv4_eip_pools JSONB NOT NULL DEFAULT '{}',
    ipv4_eip_allow_custom BOOLEAN NOT NULL DEFAULT false,
    ipv4_eip_unit_price_cents BIGINT NOT NULL DEFAULT 0,
    ipv4_eip_allow_change BOOLEAN NOT NULL DEFAULT false,
    ipv4_eip_change_price_cents BIGINT NOT NULL DEFAULT 0,

    -- IPv6 配置
    ipv6_enabled BOOLEAN NOT NULL DEFAULT false,
    ipv6_prefix_len INT NOT NULL DEFAULT 128,
    ipv6_eip_pools JSONB NOT NULL DEFAULT '{}',

    -- 价格配置
    base_price_cents BIGINT NOT NULL DEFAULT 0,
    min_payment_period VARCHAR(16) NOT NULL DEFAULT 'monthly',
    trial_enabled BOOLEAN NOT NULL DEFAULT false,
    trial_hours INT NOT NULL DEFAULT 0,
    trial_price_cents BIGINT NOT NULL DEFAULT 0,
    quarterly_discount DECIMAL(4,2) NOT NULL DEFAULT 1.00,
    half_yearly_discount DECIMAL(4,2) NOT NULL DEFAULT 1.00,
    yearly_discount DECIMAL(4,2) NOT NULL DEFAULT 1.00,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_products_category_id ON products(category_id);
CREATE INDEX IF NOT EXISTS idx_products_deleted_at ON products(deleted_at);
CREATE INDEX IF NOT EXISTS idx_products_status ON products(status);
