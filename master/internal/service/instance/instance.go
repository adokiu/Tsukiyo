package instance

import (
	"crypto/rand"
	"math/big"
	"time"

	"tsukiyo/master/internal/agent"
	"tsukiyo/master/internal/service"
	"tsukiyo/master/internal/service/infrastructure"
)

var (
	ErrInstanceNotFound       = service.ErrInstanceNotFound
	ErrNodeNotFound           = service.ErrNodeNotFound
	ErrNodeOffline            = service.ErrNodeOffline
	ErrInvalidNodeID          = service.ErrInvalidNodeID
	ErrImageNotDownloaded     = service.ErrImageNotDownloaded
	ErrInvalidBridgeID        = service.ErrInvalidBridgeID
	ErrBridgeNotFound         = service.ErrBridgeNotFound
	ErrUserNotFound           = service.ErrUserNotFound
	ErrInstanceNameExists     = service.ErrInstanceNameExists
	ErrInstanceNoBridge       = service.ErrInstanceNoBridge
	ErrNoBridgeEgressIP       = service.ErrNoBridgeEgressIP
	ErrInstanceBusy           = service.ErrInstanceBusy
	ErrInstanceBanned         = service.ErrInstanceBanned
	ErrInstanceExpired        = service.ErrInstanceExpired
	ErrDiskNotFound           = service.ErrDiskNotFound
	ErrDiskShrinkNotSupported = service.ErrDiskShrinkNotSupported
	ErrInvalidResizeConfig    = service.ErrInvalidResizeConfig
	ErrDiskNameExists         = service.ErrDiskNameExists
	ErrVMResizeRequiresStop   = service.ErrVMResizeRequiresStop
)

// InstanceService 实例服务
type InstanceService struct {
	networkSvc *infrastructure.NetworkService
	agentMgr   *agent.Manager
}

// NewInstanceService 创建实例服务
func NewInstanceService(networkSvc *infrastructure.NetworkService, agentMgr *agent.Manager) *InstanceService {
	return &InstanceService{networkSvc: networkSvc, agentMgr: agentMgr}
}

// isHex 判断字符串是否为十六进制
func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// GenerateRandomPassword 生成随机密码（crypto/rand 真随机，大小写字母+数字）
func GenerateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

// DataDiskRequest 数据磁盘请求
type DataDiskRequest struct {
	Name        string `json:"name" binding:"required"`
	SizeMB      int    `json:"size_mb" binding:"required,min=1"`
	StoragePool string `json:"storage_pool,omitempty"`
	MountPoint  string `json:"mount_point,omitempty"`
}

// CreateInstanceRequest 创建实例请求
type CreateInstanceRequest struct {
	Name             string            `json:"name" binding:"required"`
	Type             string            `json:"type" binding:"required,oneof=container vm"`
	TemplateID       string            `json:"template_id" binding:"required"`
	ImageKey         string            `json:"image_key,omitempty"`
	NodeID           string            `json:"node_id" binding:"required"`
	BridgeID         string            `json:"bridge_id,omitempty"`
	AssignToUserID   uint              `json:"assign_to_user_id" binding:"required"`
	LoginMethod      string            `json:"login_method" binding:"required,oneof=auto password sshkey"`
	VCPU             float64           `json:"vcpu" binding:"required,min=0.1"`
	MemoryMB         int               `json:"memory_mb" binding:"required,min=64"`
	SwapMB           int               `json:"swap_mb,omitempty"`
	DiskMB           int               `json:"disk_mb" binding:"required,min=1"`
	StoragePool      string            `json:"storage_pool,omitempty"`
	DataDisks        []DataDiskRequest `json:"data_disks,omitempty"`
	AssignEIPv4      bool              `json:"assign_eip_ipv4,omitempty"`
	AssignEIPv6      bool              `json:"assign_eip_ipv6,omitempty"`
	EIPv4Count       int               `json:"eip_ipv4_count,omitempty"`
	EIPv6Count       int               `json:"eip_ipv6_count,omitempty"`
	EIPv6PrefixLen   int               `json:"eip_ipv6_prefix_len,omitempty"`
	EIPv4SpecificIP  string            `json:"eip_ipv4_specific_ip,omitempty"`
	EIPv6SpecificIP  string            `json:"eip_ipv6_specific_ip,omitempty"`
	EIPv4PoolID      string            `json:"eip_ipv4_pool_id,omitempty"`
	EIPv6PoolID      string            `json:"eip_ipv6_pool_id,omitempty"`
	PortMappingCount int               `json:"port_mapping_count,omitempty"`
	ExtraPorts       []int             `json:"extra_ports,omitempty"`
	NetworkDownMbps  int               `json:"network_down_mbps,omitempty"`
	NetworkUpMbps    int               `json:"network_up_mbps,omitempty"`
	IOReadIops       int               `json:"io_read_iops,omitempty"`
	IOWriteIops      int               `json:"io_write_iops,omitempty"`
	SSHPassword      string            `json:"ssh_password,omitempty"`
	SSHPublicKey     string            `json:"ssh_public_key,omitempty"`
	MonthlyTrafficGB int64             `json:"monthly_traffic_gb,omitempty"`
	TrafficMode      string            `json:"traffic_mode,omitempty"`
	OverLimitAction  string            `json:"over_limit_action,omitempty"`
	ThrottleMbps     int               `json:"throttle_mbps,omitempty"`
	SnapshotLimit    int               `json:"snapshot_limit,omitempty"`
	ExpiresAt        *time.Time        `json:"expires_at,omitempty"`
}

// UpdateInstanceRequest 更新实例请求
type UpdateInstanceRequest struct {
	Name             *string  `json:"name,omitempty"`
	VCPU             *float64 `json:"vcpu,omitempty"`
	MemoryMB         *int     `json:"memory_mb,omitempty"`
	DiskMB           *int     `json:"disk_mb,omitempty"`
	SwapMB           *int     `json:"swap_mb,omitempty"`
	NetworkDownMbps  *int     `json:"network_down_mbps,omitempty"`
	NetworkUpMbps    *int     `json:"network_up_mbps,omitempty"`
	IOReadIops       *int     `json:"io_read_iops,omitempty"`
	IOWriteIops      *int     `json:"io_write_iops,omitempty"`
	ExpiresAt        *string  `json:"expires_at,omitempty"`
	MonthlyTrafficGB *int64   `json:"monthly_traffic_gb,omitempty"`
	TrafficMode      *string  `json:"traffic_mode,omitempty"`
	OverLimitAction  *string  `json:"over_limit_action,omitempty"`
	ThrottleMbps     *int     `json:"throttle_mbps,omitempty"`
	SnapshotLimit    *int     `json:"snapshot_limit,omitempty"`
	PortMappingLimit *int     `json:"port_mapping_limit,omitempty"`
}

// ReinstallInstanceRequest 重装请求参数
type ReinstallInstanceRequest struct {
	TemplateID      string `json:"template_id"`
	Password        string `json:"password"`
	SSHKey          string `json:"ssh_key"`
	LoginMethod     string `json:"login_method"`
	FormatDataDisks bool   `json:"format_data_disks"`
}

// UpdateInstanceNetworkRequest 修改网络配置请求
type UpdateInstanceNetworkRequest struct {
	BridgeID        *string `json:"bridge_id,omitempty"`
	IPv4Mode        *string `json:"ipv4_mode,omitempty"`
	IPv6Mode        *string `json:"ipv6_mode,omitempty"`
	NetworkDownMbps *int    `json:"network_down_mbps,omitempty"`
	NetworkUpMbps   *int    `json:"network_up_mbps,omitempty"`
}
