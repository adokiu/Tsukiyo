-- Tsukiyo Master 初始数据库迁移
-- 创建于 2026-06-18

-- 用户表
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(64) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    status VARCHAR(16) DEFAULT 'active' NOT NULL,
    last_login_at TIMESTAMPTZ,
    last_login_ip VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- 用户组表
CREATE TABLE IF NOT EXISTS user_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(64) UNIQUE NOT NULL,
    description TEXT,
    is_builtin BOOLEAN DEFAULT FALSE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 用户与组关联表
CREATE TABLE IF NOT EXISTS user_group_members (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id UUID NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    assigned_by UUID,
    PRIMARY KEY (user_id, group_id)
);

-- 权限定义表
CREATE TABLE IF NOT EXISTS permissions (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    resource VARCHAR(32) NOT NULL,
    action VARCHAR(32) NOT NULL,
    description TEXT
);

-- 组权限关联表
CREATE TABLE IF NOT EXISTS group_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id UUID NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
    permission_id VARCHAR(64) NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    scope VARCHAR(16) DEFAULT 'all' NOT NULL,
    scope_target UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (group_id, permission_id, scope_target)
);

-- 节点表
CREATE TABLE IF NOT EXISTS nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(64) NOT NULL,
    token VARCHAR(255) NOT NULL,
    hostname VARCHAR(255),
    ip_address INET,
    status VARCHAR(16) DEFAULT 'offline' NOT NULL,
    incus_version VARCHAR(32),
    total_cpu FLOAT DEFAULT 0,
    total_memory BIGINT DEFAULT 0,
    total_disk BIGINT DEFAULT 0,
    used_cpu FLOAT DEFAULT 0,
    used_memory BIGINT DEFAULT 0,
    used_disk BIGINT DEFAULT 0,
    instance_count INT DEFAULT 0,
    running_count INT DEFAULT 0,
    last_seen_at TIMESTAMPTZ,
    last_heartbeat TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 实例表
CREATE TABLE IF NOT EXISTS instances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(64) NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id),
    node_id UUID NOT NULL REFERENCES nodes(id),
    type VARCHAR(16) NOT NULL,
    status VARCHAR(16) DEFAULT 'creating' NOT NULL,
    incus_name VARCHAR(64) NOT NULL,
    template_id VARCHAR(64),
    vcpu FLOAT DEFAULT 1,
    memory_mb INT DEFAULT 512,
    disk_gb INT DEFAULT 10,
    storage_pool VARCHAR(64) DEFAULT 'default',
    login_method VARCHAR(16) DEFAULT 'auto',
    network_down_mbps INT DEFAULT 0,
    network_up_mbps INT DEFAULT 0,
    io_read_mbps INT DEFAULT 0,
    io_write_mbps INT DEFAULT 0,
    ipv4_address INET,
    ipv6_address INET,
    ssh_port INT,
    ssh_password VARCHAR(255),
    ssh_public_key TEXT,
    mac_address VARCHAR(32),
    traffic_mode VARCHAR(16) DEFAULT 'total',
    traffic_in_gb BIGINT DEFAULT 0,
    traffic_out_gb BIGINT DEFAULT 0,
    monthly_traffic_gb BIGINT DEFAULT 0,
    traffic_used_gb FLOAT DEFAULT 0,
    traffic_reset_date VARCHAR(7) DEFAULT '',
    over_limit_action VARCHAR(16) DEFAULT 'shutdown',
    is_over_limit BOOLEAN DEFAULT FALSE,
    snapshot_limit INT DEFAULT 5,
    port_mapping_limit INT DEFAULT 2,
    expires_at TIMESTAMPTZ,
    expired_at TIMESTAMPTZ,
    vnc_port INT,
    public_ipv4_id UUID,
    public_ipv6_prefix_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- 数据磁盘表
CREATE TABLE IF NOT EXISTS data_disks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    node_id UUID NOT NULL REFERENCES nodes(id),
    name VARCHAR(64) NOT NULL,
    size_gb INT NOT NULL,
    storage_pool VARCHAR(64) DEFAULT 'default',
    mount_point VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- NAT 配置表
CREATE TABLE IF NOT EXISTS nat_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    node_id UUID NOT NULL REFERENCES nodes(id),
    internal_ip INET NOT NULL,
    external_ip INET,
    internal_port INT,
    external_port INT,
    protocol VARCHAR(8) DEFAULT 'tcp',
    description VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 公网 IP 池表
CREATE TABLE IF NOT EXISTS public_ip_pools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id UUID NOT NULL REFERENCES nodes(id),
    address INET NOT NULL,
    gateway INET,
    prefix_len INT DEFAULT 32,
    interface VARCHAR(32),
    status VARCHAR(16) DEFAULT 'free' NOT NULL,
    instance_id UUID,
    assigned_at TIMESTAMPTZ,
    UNIQUE (address, node_id)
);

-- IPv6 前缀表
CREATE TABLE IF NOT EXISTS ipv6_prefixes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id UUID NOT NULL REFERENCES nodes(id),
    prefix CIDR NOT NULL,
    prefix_len INT,
    interface VARCHAR(32),
    gateway INET,
    status VARCHAR(16) DEFAULT 'active' NOT NULL
);

-- 端口映射表
CREATE TABLE IF NOT EXISTS port_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    node_id UUID NOT NULL REFERENCES nodes(id),
    container_port INT NOT NULL,
    host_port INT NOT NULL,
    protocol VARCHAR(8) DEFAULT 'tcp',
    host_ip INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 防火墙规则表
CREATE TABLE IF NOT EXISTS firewall_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    node_id UUID NOT NULL REFERENCES nodes(id),
    network VARCHAR(16) DEFAULT 'ipv4',
    direction VARCHAR(8) NOT NULL,
    protocol VARCHAR(16) DEFAULT 'all',
    port VARCHAR(32),
    source_ip INET,
    action VARCHAR(16) NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    priority INT DEFAULT 100,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 镜像模板表
CREATE TABLE IF NOT EXISTS image_templates (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    type VARCHAR(16) NOT NULL,
    distro VARCHAR(32),
    release VARCHAR(32),
    arch VARCHAR(16) DEFAULT 'amd64',
    url VARCHAR(512),
    description TEXT,
    enabled BOOLEAN DEFAULT TRUE,
    desktop VARCHAR(32)
);

-- 审计日志表
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    username VARCHAR(64),
    action VARCHAR(64) NOT NULL,
    target VARCHAR(64),
    detail TEXT,
    ip_address INET,
    success BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 任务队列表
CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(32) NOT NULL,
    node_id UUID NOT NULL REFERENCES nodes(id),
    instance_id UUID REFERENCES instances(id),
    user_id UUID NOT NULL REFERENCES users(id),
    status VARCHAR(16) DEFAULT 'pending' NOT NULL,
    payload JSONB,
    result JSONB,
    error TEXT,
    retry_count INT DEFAULT 0,
    max_retries INT DEFAULT 3,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);

-- 任务日志表
CREATE TABLE IF NOT EXISTS task_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    level VARCHAR(16) NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 快照表
CREATE TABLE IF NOT EXISTS snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    name VARCHAR(64) NOT NULL,
    description TEXT,
    size_bytes BIGINT DEFAULT 0,
    is_scheduled BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (instance_id, name)
);

-- 监控指标表 (时序数据)
CREATE TABLE IF NOT EXISTS instance_metrics (
    id BIGSERIAL PRIMARY KEY,
    instance_id UUID NOT NULL,
    node_id UUID NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    cpu_percent FLOAT DEFAULT 0,
    mem_used BIGINT DEFAULT 0,
    mem_total BIGINT DEFAULT 0,
    disk_used BIGINT DEFAULT 0,
    disk_total BIGINT DEFAULT 0,
    disk_read_bps BIGINT DEFAULT 0,
    disk_write_bps BIGINT DEFAULT 0,
    net_in_bps BIGINT DEFAULT 0,
    net_out_bps BIGINT DEFAULT 0,
    net_in_total BIGINT DEFAULT 0,
    net_out_total BIGINT DEFAULT 0
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_instances_user_id ON instances(user_id);
CREATE INDEX IF NOT EXISTS idx_instances_node_id ON instances(node_id);
CREATE INDEX IF NOT EXISTS idx_instances_status ON instances(status);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_node_id ON tasks(node_id);
CREATE INDEX IF NOT EXISTS idx_task_logs_task_id ON task_logs(task_id);
CREATE INDEX IF NOT EXISTS idx_task_logs_created ON task_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created ON audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_instance_metrics_instance_time ON instance_metrics(instance_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_instance_metrics_node_time ON instance_metrics(node_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_public_ip_pools_status ON public_ip_pools(status);
CREATE INDEX IF NOT EXISTS idx_public_ip_pools_node ON public_ip_pools(node_id);
CREATE INDEX IF NOT EXISTS idx_data_disks_instance ON data_disks(instance_id);
CREATE INDEX IF NOT EXISTS idx_nat_configs_instance ON nat_configs(instance_id);

-- 更新时间触发器
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_instances_updated_at BEFORE UPDATE ON instances
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_firewall_rules_updated_at BEFORE UPDATE ON firewall_rules
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
