-- 用户表
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(64) NOT NULL,
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    status VARCHAR(16) DEFAULT 'active',
    last_login_at TIMESTAMPTZ,
    last_login_ip VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users (username) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users (email) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users (deleted_at);

-- 用户组表
CREATE TABLE IF NOT EXISTS user_groups (
    id SERIAL PRIMARY KEY,
    name VARCHAR(64) NOT NULL,
    description TEXT,
    is_builtin BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_groups_name ON user_groups (name);

-- 用户组成员关联表
CREATE TABLE IF NOT EXISTS user_group_members (
    user_id INTEGER NOT NULL,
    group_id INTEGER NOT NULL,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    assigned_by INTEGER,
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
    group_id INTEGER NOT NULL,
    permission_id VARCHAR(64) NOT NULL,
    scope VARCHAR(16) DEFAULT 'all',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, permission_id)
);

-- 节点表
CREATE TABLE IF NOT EXISTS nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(64) NOT NULL,
    token VARCHAR(255) NOT NULL,
    hostname VARCHAR(255),
    ip_address VARCHAR(64),
    ipv6_address VARCHAR(128),
    country_code VARCHAR(8),
    status VARCHAR(16) DEFAULT 'offline',
    incus_version VARCHAR(128),
    total_cpu FLOAT DEFAULT 0,
    total_memory BIGINT DEFAULT 0,
    total_disk BIGINT DEFAULT 0,
    used_cpu FLOAT DEFAULT 0,
    used_memory BIGINT DEFAULT 0,
    used_disk BIGINT DEFAULT 0,
    net_in BIGINT DEFAULT 0,
    net_out BIGINT DEFAULT 0,
    uptime BIGINT DEFAULT 0,
    instance_count INTEGER DEFAULT 0,
    running_count INTEGER DEFAULT 0,
    last_seen_at TIMESTAMPTZ,
    last_heartbeat TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    system_info JSONB DEFAULT '{}',
    incus_socket_path VARCHAR(255) DEFAULT '/var/lib/incus/unix.socket',
    metrics_interval INTEGER DEFAULT 1,
    heartbeat_interval INTEGER DEFAULT 1,
    network_interface VARCHAR(64),
    enable_nat BOOLEAN DEFAULT true,
    enable_firewall BOOLEAN DEFAULT true,
    enable_security_scan BOOLEAN DEFAULT true,
    scan_interval INTEGER DEFAULT 300,
    console_bind_addr VARCHAR(64) DEFAULT '0.0.0.0:9090',
    agent_url VARCHAR(255),
    image_remote_url VARCHAR(512),
    default_storage_pool VARCHAR(64) DEFAULT 'default',
    storage_pool_type VARCHAR(16) DEFAULT 'dir',
    storage_pool_source VARCHAR(255),
    storage_pool_created BOOLEAN DEFAULT false
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_nodes_token ON nodes (token) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_nodes_deleted_at ON nodes (deleted_at);

-- 实例表
CREATE TABLE IF NOT EXISTS instances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(64) NOT NULL,
    user_id INTEGER NOT NULL,
    node_id UUID NOT NULL,
    type VARCHAR(16) NOT NULL,
    status VARCHAR(16) DEFAULT 'creating',
    incus_name VARCHAR(64) NOT NULL,
    template_id VARCHAR(64),
    bridge_id UUID,
    internal_ipv4 INET,
    internal_ipv6 VARCHAR(64) DEFAULT '',
    vcpu FLOAT DEFAULT 1,
    memory_mb INTEGER DEFAULT 512,
    disk_gb INTEGER DEFAULT 10,
    network_down_mbps INTEGER DEFAULT 0,
    network_up_mbps INTEGER DEFAULT 0,
    io_read_mbps INTEGER DEFAULT 0,
    io_write_mbps INTEGER DEFAULT 0,
    ipv4_mode VARCHAR(8) NOT NULL DEFAULT 'nat',
    ipv6_mode VARCHAR(8) NOT NULL DEFAULT 'none',
    ipv4_eip_allocation_id UUID,
    ipv6_eip_allocation_id UUID,
    ssh_port INTEGER,
    ssh_password VARCHAR(255),
    ssh_public_key TEXT,
    mac_address VARCHAR(32),
    storage_pool VARCHAR(64) DEFAULT 'default',
    login_method VARCHAR(16) DEFAULT 'auto',
    traffic_mode VARCHAR(16) DEFAULT 'total',
    traffic_in_gb BIGINT DEFAULT 0,
    traffic_out_gb BIGINT DEFAULT 0,
    monthly_traffic_gb BIGINT DEFAULT 0,
    traffic_used_gb FLOAT DEFAULT 0,
    traffic_reset_date VARCHAR(7) DEFAULT '',
    over_limit_action VARCHAR(16) DEFAULT 'shutdown',
    is_over_limit BOOLEAN DEFAULT false,
    snapshot_limit INTEGER DEFAULT 5,
    port_mapping_limit INTEGER DEFAULT 2,
    expires_at TIMESTAMPTZ,
    expired_at TIMESTAMPTZ,
    vnc_port INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_instances_user_id ON instances (user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_instances_node_id ON instances (node_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_instances_bridge_id ON instances (bridge_id);
CREATE INDEX IF NOT EXISTS idx_instances_ipv4_eip_allocation_id ON instances (ipv4_eip_allocation_id);
CREATE INDEX IF NOT EXISTS idx_instances_ipv6_eip_allocation_id ON instances (ipv6_eip_allocation_id);
CREATE INDEX IF NOT EXISTS idx_instances_deleted_at ON instances (deleted_at);

-- 快照表
CREATE TABLE IF NOT EXISTS snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL,
    name VARCHAR(64) NOT NULL,
    description TEXT,
    size_bytes BIGINT DEFAULT 0,
    is_scheduled BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_snapshots_instance_id ON snapshots (instance_id);

-- 数据磁盘表
CREATE TABLE IF NOT EXISTS data_disks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL,
    node_id UUID NOT NULL,
    name VARCHAR(64) NOT NULL,
    size_gb INTEGER NOT NULL,
    storage_pool VARCHAR(64) DEFAULT 'default',
    mount_point VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_data_disks_instance_id ON data_disks (instance_id);
CREATE INDEX IF NOT EXISTS idx_data_disks_node_id ON data_disks (node_id);

-- 实例监控指标表
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
CREATE INDEX IF NOT EXISTS idx_instance_metrics_instance_id ON instance_metrics (instance_id);
CREATE INDEX IF NOT EXISTS idx_instance_metrics_node_id ON instance_metrics (node_id);
CREATE INDEX IF NOT EXISTS idx_instance_metrics_timestamp ON instance_metrics (timestamp);

-- 端口映射表
CREATE TABLE IF NOT EXISTS port_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL,
    node_id UUID NOT NULL,
    bridge_id UUID NOT NULL,
    ip_version VARCHAR(8) NOT NULL DEFAULT 'ipv4',
    egress_allocation_id UUID NOT NULL,
    container_port INTEGER NOT NULL,
    host_port INTEGER NOT NULL,
    protocol VARCHAR(8) DEFAULT 'tcp',
    description VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_port_mappings_instance_id ON port_mappings (instance_id);
CREATE INDEX IF NOT EXISTS idx_port_mappings_node_id ON port_mappings (node_id);
CREATE INDEX IF NOT EXISTS idx_port_mappings_bridge_id ON port_mappings (bridge_id);
CREATE INDEX IF NOT EXISTS idx_port_mappings_egress_allocation_id ON port_mappings (egress_allocation_id);

-- 防火墙规则表
CREATE TABLE IF NOT EXISTS firewall_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL,
    node_id UUID NOT NULL,
    network VARCHAR(8) DEFAULT 'ipv4',
    direction VARCHAR(8) NOT NULL,
    protocol VARCHAR(8) DEFAULT 'all',
    port VARCHAR(64),
    source_ip INET,
    action VARCHAR(8) NOT NULL,
    description VARCHAR(255),
    enabled BOOLEAN DEFAULT true,
    priority INTEGER DEFAULT 100,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_firewall_rules_instance_id ON firewall_rules (instance_id);
CREATE INDEX IF NOT EXISTS idx_firewall_rules_node_id ON firewall_rules (node_id);

-- 网桥网络表
CREATE TABLE IF NOT EXISTS bridges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id UUID NOT NULL,
    name VARCHAR(64) NOT NULL,
    bridge_name VARCHAR(64) NOT NULL,
    ipv4_enabled BOOLEAN NOT NULL DEFAULT true,
    ipv4_cidr VARCHAR(32) NOT NULL DEFAULT '',
    ipv4_gateway VARCHAR(32) NOT NULL DEFAULT '',
    ipv6_enabled BOOLEAN NOT NULL DEFAULT false,
    ipv6_cidr VARCHAR(64) NOT NULL DEFAULT '',
    ipv6_gateway VARCHAR(64) NOT NULL DEFAULT '',
    dns_servers JSONB NOT NULL DEFAULT '[]',
    nat_egress_ipv4_id UUID,
    nat_egress_ipv6_id UUID,
    port_range_start INTEGER NOT NULL DEFAULT 20000,
    port_range_end INTEGER NOT NULL DEFAULT 65535,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_bridges_node_id ON bridges (node_id);
CREATE INDEX IF NOT EXISTS idx_bridges_nat_egress_ipv4_id ON bridges (nat_egress_ipv4_id);
CREATE INDEX IF NOT EXISTS idx_bridges_nat_egress_ipv6_id ON bridges (nat_egress_ipv6_id);

-- EIP 资源池表
CREATE TABLE IF NOT EXISTS eip_pools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id UUID NOT NULL,
    ip_version VARCHAR(8) NOT NULL,
    cidr VARCHAR(64) NOT NULL,
    interface VARCHAR(32) NOT NULL DEFAULT '',
    gateway VARCHAR(64) NOT NULL DEFAULT '',
    prefix_len INTEGER NOT NULL,
    alias VARCHAR(128) NOT NULL DEFAULT '',
    pool_type VARCHAR(8) NOT NULL DEFAULT 'eip',
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_eip_pools_node_id ON eip_pools (node_id);

-- EIP 分配记录表
CREATE TABLE IF NOT EXISTS eip_allocations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pool_id UUID NOT NULL,
    node_id UUID NOT NULL,
    cidr VARCHAR(64) NOT NULL,
    prefix_len INTEGER NOT NULL,
    ip_version VARCHAR(8) NOT NULL,
    usage VARCHAR(20) NOT NULL,
    bridge_id UUID,
    instance_id UUID,
    status VARCHAR(16) NOT NULL DEFAULT 'assigned',
    allocated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    released_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_eip_allocations_pool_id ON eip_allocations (pool_id);
CREATE INDEX IF NOT EXISTS idx_eip_allocations_node_id ON eip_allocations (node_id);
CREATE INDEX IF NOT EXISTS idx_eip_allocations_bridge_id ON eip_allocations (bridge_id);
CREATE INDEX IF NOT EXISTS idx_eip_allocations_instance_id ON eip_allocations (instance_id);

-- 任务队列表
CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(32) NOT NULL,
    node_id UUID NOT NULL,
    instance_id UUID,
    user_id INTEGER NOT NULL,
    status VARCHAR(16) DEFAULT 'pending',
    payload JSONB,
    result JSONB,
    error TEXT,
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 3,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_tasks_node_id ON tasks (node_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_instance_id ON tasks (instance_id);
CREATE INDEX IF NOT EXISTS idx_tasks_user_id ON tasks (user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_deleted_at ON tasks (deleted_at);

-- 任务日志表
CREATE TABLE IF NOT EXISTS task_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL,
    level VARCHAR(16) NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_task_logs_task_id ON task_logs (task_id);

-- 审计日志表
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id INTEGER,
    username VARCHAR(64),
    action VARCHAR(64) NOT NULL,
    target VARCHAR(64),
    detail TEXT,
    ip_address INET,
    success BOOLEAN DEFAULT true,
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs (user_id);

-- 站点配置表
CREATE TABLE IF NOT EXISTS site_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_name VARCHAR(128) NOT NULL DEFAULT 'Tsukiyo',
    site_subtitle VARCHAR(255),
    site_description TEXT,
    site_url VARCHAR(255),
    contact_email VARCHAR(255),
    incus_remote_url VARCHAR(512) DEFAULT 'images:',
    is_initialized BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 节点镜像关系表
CREATE TABLE IF NOT EXISTS node_images (
    id SERIAL PRIMARY KEY,
    node_id UUID NOT NULL,
    image_id VARCHAR(255) NOT NULL,
    status VARCHAR(16) DEFAULT 'downloaded',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_node_image ON node_images (node_id, image_id);

-- 镜像缓存表
CREATE TABLE IF NOT EXISTS image_cache (
    id SERIAL PRIMARY KEY,
    source_url VARCHAR(512) NOT NULL,
    image_key VARCHAR(255) NOT NULL,
    alias VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL DEFAULT '',
    type VARCHAR(50) NOT NULL,
    distro VARCHAR(100) NOT NULL DEFAULT '',
    release VARCHAR(100) NOT NULL DEFAULT '',
    arch VARCHAR(50) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    total_bytes BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 安全告警表
CREATE TABLE IF NOT EXISTS security_alerts (
    id UUID PRIMARY KEY,
    node_id UUID NOT NULL,
    instance_id VARCHAR(64),
    alert_type VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'open',
    source_ip VARCHAR(45),
    dest_port INTEGER,
    protocol VARCHAR(10),
    details TEXT NOT NULL,
    raw_data TEXT,
    auto_action VARCHAR(50),
    resolved_by INTEGER,
    resolved_at TIMESTAMPTZ,
    detected_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_security_alerts_node_id ON security_alerts (node_id);
CREATE INDEX IF NOT EXISTS idx_security_alerts_instance_id ON security_alerts (instance_id);
CREATE INDEX IF NOT EXISTS idx_security_alerts_alert_type ON security_alerts (alert_type);
CREATE INDEX IF NOT EXISTS idx_security_alerts_severity ON security_alerts (severity);
CREATE INDEX IF NOT EXISTS idx_security_alerts_status ON security_alerts (status);
CREATE INDEX IF NOT EXISTS idx_security_alerts_detected_at ON security_alerts (detected_at);
CREATE INDEX IF NOT EXISTS idx_security_alerts_deleted_at ON security_alerts (deleted_at);
