package types

// ============================================================================
// 实例相关 Task Payload
// ============================================================================

// CreateInstancePayload 创建实例任务 payload
type CreateInstancePayload struct {
	TaskID           string                   `json:"task_id"`
	InstanceID       string                   `json:"instance_id"`
	Type             string                   `json:"type"`
	TemplateID       string                   `json:"template_id"`
	VCPU             float64                  `json:"vcpu"`
	MemoryMB         int                      `json:"memory_mb"`
	SwapMB           int                      `json:"swap_mb"`
	DiskMB           int                      `json:"disk_mb"`
	StoragePool      string                   `json:"storage_pool"`
	LoginMethod      string                   `json:"login_method"`
	SSHPassword      string                   `json:"ssh_password"`
	SSHPublicKey     string                   `json:"ssh_public_key"`
	ImageSource      string                   `json:"image_source"`
	IPv4Address      string                   `json:"ipv4_address"`
	IPv6Address      string                   `json:"ipv6_address"`
	NetworkDown      int                      `json:"network_down"`
	NetworkUp        int                      `json:"network_up"`
	IORead           int                      `json:"io_read"`
	IOWrite          int                      `json:"io_write"`
	DataDisks        []map[string]interface{} `json:"data_disks"`
	NATs             []map[string]interface{} `json:"nats"`
	PortMappings     []map[string]interface{} `json:"port_mappings"`
	MonthlyTraffic   int64                    `json:"monthly_traffic"`
	TrafficMode      string                   `json:"traffic_mode"`
	SnapshotLimit    int                      `json:"snapshot_limit"`
	AssignNAT        bool                     `json:"assign_nat"`
	PortMappingCount int                      `json:"port_mapping_count"`
	ExtraPorts       []int                    `json:"extra_ports"`
	BridgeID         string                   `json:"bridge_id,omitempty"`
	InternalIPv4     string                   `json:"internal_ipv4,omitempty"`
	GatewayV4        string                   `json:"gateway_v4,omitempty"`
	IPv4CIDR         string                   `json:"ipv4_cidr,omitempty"`
	BridgeName       string                   `json:"bridge_name,omitempty"`
	IPv4Filter       bool                     `json:"ipv4_filter,omitempty"`
	MACFilter        bool                     `json:"mac_filter,omitempty"`
	EgressV4Primary  string                   `json:"egress_v4_primary,omitempty"`
	ParentIface      string                   `json:"parent_iface,omitempty"`
	IPv4Mode         string                   `json:"ipv4_mode"`
	IPv6Mode         string                   `json:"ipv6_mode"`
	PortMappingLimit int                      `json:"port_mapping_limit"`
	GatewayV6        string                   `json:"gateway_v6,omitempty"`
	IPv6CIDR         string                   `json:"ipv6_cidr,omitempty"`
	EIPAssignments   []map[string]interface{} `json:"eip_assignments,omitempty"`
}

// ReinstallInstancePayload 重装实例任务 payload
type ReinstallInstancePayload struct {
	InstanceID      string                   `json:"instance_id"`
	Type            string                   `json:"type"`
	TemplateID      string                   `json:"template_id"`
	VCPU            float64                  `json:"vcpu"`
	MemoryMB        int                      `json:"memory_mb"`
	SwapMB          int                      `json:"swap_mb"`
	DiskMB          int                      `json:"disk_mb"`
	StoragePool     string                   `json:"storage_pool"`
	LoginMethod     string                   `json:"login_method"`
	SSHPassword     string                   `json:"ssh_password"`
	SSHPublicKey    string                   `json:"ssh_public_key"`
	ImageSource     string                   `json:"image_source"`
	NetworkDown     int                      `json:"network_down"`
	NetworkUp       int                      `json:"network_up"`
	IORead          int                      `json:"io_read"`
	IOWrite         int                      `json:"io_write"`
	DataDisks       []map[string]interface{} `json:"data_disks"`
	PortMappings    []map[string]interface{} `json:"port_mappings"`
	BridgeName      string                   `json:"bridge_name,omitempty"`
	InternalIPv4    string                   `json:"internal_ipv4,omitempty"`
	GatewayV4       string                   `json:"gateway_v4,omitempty"`
	IPv4CIDR        string                   `json:"ipv4_cidr,omitempty"`
	IPv4Filter      bool                     `json:"ipv4_filter,omitempty"`
	IPv4Mode        string                   `json:"ipv4_mode"`
	IPv6Mode        string                   `json:"ipv6_mode"`
	EIPAssignments  []map[string]interface{} `json:"eip_assignments,omitempty"`
	FormatDataDisks bool                     `json:"format_data_disks"`
	OldStatus       string                   `json:"old_status"`
}

// DeleteInstancePayload 删除实例任务 payload
type DeleteInstancePayload struct {
	InstanceID string `json:"instance_id"`
}

// StartInstancePayload 启动实例任务 payload
type StartInstancePayload struct {
	InstanceID string `json:"instance_id"`
}

// StopInstancePayload 停止实例任务 payload
type StopInstancePayload struct {
	InstanceID string `json:"instance_id"`
	Force      bool   `json:"force"`
}

// RestartInstancePayload 重启实例任务 payload
type RestartInstancePayload struct {
	InstanceID string `json:"instance_id"`
}

// ResizeInstancePayload 调整配置任务 payload
type ResizeInstancePayload struct {
	InstanceID string  `json:"instance_id"`
	VCPU       float64 `json:"vcpu"`
	MemoryMB   int     `json:"memory_mb"`
	SwapMB     int     `json:"swap_mb"`
}

// ResetPasswordPayload 重置密码任务 payload
type ResetPasswordPayload struct {
	InstanceID string `json:"instance_id"`
	Password   string `json:"password"`
}

// MigrateInstancePayload 迁移实例任务 payload
type MigrateInstancePayload struct {
	InstanceID string `json:"instance_id"`
	TargetNode string `json:"target_node"`
}

// LimitNetworkPayload 网络限速任务 payload
type LimitNetworkPayload struct {
	InstanceID  string `json:"instance_id"`
	NetworkDown int    `json:"network_down"`
	NetworkUp   int    `json:"network_up"`
}

// LimitIOPSPayload 磁盘 IOPS 限制任务 payload
type LimitIOPSPayload struct {
	InstanceID string `json:"instance_id"`
	IORead     int    `json:"io_read"`
	IOWrite    int    `json:"io_write"`
}

// ============================================================================
// 快照相关 Task Payload
// ============================================================================

// CreateSnapshotPayload 创建快照任务 payload
type CreateSnapshotPayload struct {
	InstanceID string `json:"instance_id"`
	Name       string `json:"name"`
	Stateful   bool   `json:"stateful"`
}

// RestoreSnapshotPayload 恢复快照任务 payload
type RestoreSnapshotPayload struct {
	InstanceID string `json:"instance_id"`
	Name       string `json:"name"`
}

// DeleteSnapshotPayload 删除快照任务 payload
type DeleteSnapshotPayload struct {
	InstanceID string `json:"instance_id"`
	Name       string `json:"name"`
}

// ============================================================================
// 镜像相关 Task Payload
// ============================================================================

// DownloadImagePayload 下载镜像任务 payload
type DownloadImagePayload struct {
	ImageKey   string `json:"image_key"`
	ImageType  string `json:"image_type"`
	Source     string `json:"source"`
	RemoteName string `json:"remote_name"`
	RemoteURL  string `json:"remote_url"`
}

// CheckImagePayload 检查镜像任务 payload
type CheckImagePayload struct {
	ImageKey string `json:"image_key"`
}

// DeleteImagePayload 删除镜像任务 payload
type DeleteImagePayload struct {
	ImageKey string `json:"image_key"`
}

// SyncImagesPayload 同步镜像列表任务 payload
type SyncImagesPayload struct {
	Images []map[string]interface{} `json:"images"`
}

// ============================================================================
// 网络相关 Task Payload
// ============================================================================

// ApplyNetworkPayload 应用网络配置任务 payload
type ApplyNetworkPayload struct {
	Action        string `json:"action"`
	InstanceID    string `json:"instance_id"`
	IPAddress     string `json:"ip_address"`
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
	HostIP        string `json:"host_ip"`
}

// BridgeNetworkPayload Bridge 网络任务 payload
type BridgeNetworkPayload struct {
	BridgeID    string `json:"bridge_id"`
	Action      string `json:"action"`
	BridgeName  string `json:"bridge_name"`
	IPv4Enabled bool   `json:"ipv4_enabled"`
	IPv4CIDR    string `json:"ipv4_cidr"`
	IPv4Gateway string `json:"ipv4_gateway"`
	IPv6Enabled bool   `json:"ipv6_enabled"`
	IPv6CIDR    string `json:"ipv6_cidr"`
	IPv6Gateway string `json:"ipv6_gateway"`
	DNSServers  []string `json:"dns_servers"`
}

// BindBridgeEgressPayload 绑定 Bridge 出口 EIP 任务 payload
type BindBridgeEgressPayload struct {
	BridgeName string `json:"bridge_name"`
	EgressCIDR string `json:"egress_cidr"`
	Interface  string `json:"interface"`
	IPVersion  string `json:"ip_version"`
}

// UnbindBridgeEgressPayload 解绑 Bridge 出口 EIP 任务 payload
type UnbindBridgeEgressPayload struct {
	BridgeName string `json:"bridge_name"`
	EgressCIDR string `json:"egress_cidr"`
	Interface  string `json:"interface"`
	IPVersion  string `json:"ip_version"`
}

// AssignEIPPayload 分配实例 EIP 任务 payload
type AssignEIPPayload struct {
	InstanceName     string `json:"instance_name"`
	InstanceIP       string `json:"instance_ip"`
	EIPCidr          string `json:"eip_cidr"`
	Interface        string `json:"interface"`
	IPVersion        string `json:"ip_version"`
	BridgeName       string `json:"bridge_name"`
	MappedInternalIP string `json:"mapped_internal_ip"`
	IPv4CIDR         string `json:"ipv4_cidr"`
	IPv6CIDR         string `json:"ipv6_cidr"`
	IPv4Gateway      string `json:"ipv4_gateway"`
	IPv6Gateway      string `json:"ipv6_gateway"`
	EIPGateway       string `json:"eip_gateway"`
}

// ReleaseEIPPayload 释放实例 EIP 任务 payload
type ReleaseEIPPayload struct {
	InstanceName     string `json:"instance_name"`
	InstanceIP       string `json:"instance_ip"`
	EIPCidr          string `json:"eip_cidr"`
	Interface        string `json:"interface"`
	IPVersion        string `json:"ip_version"`
	BridgeName       string `json:"bridge_name"`
	MappedInternalIP string `json:"mapped_internal_ip"`
}

// AddPortMappingPayload 添加端口映射任务 payload
type AddPortMappingPayload struct {
	InstanceID    string `json:"instance_id"`
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
	HostIP        string `json:"host_ip"`
	InternalIP    string `json:"internal_ip"`
}

// DeletePortMappingPayload 删除端口映射任务 payload
type DeletePortMappingPayload struct {
	InstanceID string `json:"instance_id"`
	HostPort   int    `json:"host_port"`
	Protocol   string `json:"protocol"`
}

// AddFirewallRulePayload 添加防火墙规则任务 payload
type AddFirewallRulePayload struct {
	Direction string `json:"direction"`
	Protocol  string `json:"protocol"`
	Source    string `json:"source"`
	Port      string `json:"port"`
	Action    string `json:"action"`
}

// DeleteFirewallRulePayload 删除防火墙规则任务 payload
type DeleteFirewallRulePayload struct {
	Direction string `json:"direction"`
	Protocol  string `json:"protocol"`
	Source    string `json:"source"`
	Port      string `json:"port"`
}

// ApplyFirewallPayload 应用防火墙规则任务 payload
type ApplyFirewallPayload struct {
	InstanceID string                   `json:"instance_id"`
	Rules      []map[string]interface{} `json:"rules"`
}

// ============================================================================
// 磁盘/存储相关 Task Payload
// ============================================================================

// AddDiskPayload 添加数据盘任务 payload
type AddDiskPayload struct {
	InstanceID  string `json:"instance_id"`
	DiskID      string `json:"disk_id"`
	DiskName    string `json:"disk_name"`
	SizeMB      int    `json:"size_mb"`
	StoragePool string `json:"storage_pool"`
	MountPoint  string `json:"mount_point"`
}

// DeleteDiskPayload 删除数据盘任务 payload
type DeleteDiskPayload struct {
	InstanceID string `json:"instance_id"`
	DiskName   string `json:"disk_name"`
}

// ResizeDiskPayload 调整数据盘大小任务 payload
type ResizeDiskPayload struct {
	InstanceID string `json:"instance_id"`
	DiskName   string `json:"disk_name"`
	SizeMB     int    `json:"size_mb"`
}

// FormatDiskPayload 格式化磁盘任务 payload
type FormatDiskPayload struct {
	TaskID string `json:"task_id"`
	Device string `json:"device"`
	Type   string `json:"type"`
}

// InitStoragePayload 初始化存储池任务 payload
type InitStoragePayload struct {
	TaskID string `json:"task_id"`
	Device string `json:"device"`
	Name   string `json:"name"`
	Type   string `json:"type"`
}

// CreatePartitionPayload 创建分区任务 payload
type CreatePartitionPayload struct {
	TaskID string `json:"task_id"`
	Device string `json:"device"`
}

// DeleteStoragePayload 删除存储池任务 payload
type DeleteStoragePayload struct {
	TaskID string `json:"task_id"`
	Name   string `json:"name"`
}

// AddIPPayload 添加 IP 到网卡任务 payload
type AddIPPayload struct {
	CIDR      string `json:"cidr"`
	Interface string `json:"interface"`
}
