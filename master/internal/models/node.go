package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NodeStatus 节点状态
type NodeStatus string

const (
	NodeStatusOnline      NodeStatus = "online"
	NodeStatusOffline     NodeStatus = "offline"
	NodeStatusMaintenance NodeStatus = "maintenance"
	NodeStatusPending     NodeStatus = "pending" // 等待初始化配置
)

// Node 节点表
type Node struct {
	ID            uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name          string         `gorm:"type:varchar(64);not null" json:"name"`
	Token         string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"-"`
	Hostname      string         `gorm:"type:varchar(255)" json:"hostname,omitempty"`
	IPAddress     string         `gorm:"type:varchar(64)" json:"ip_address,omitempty"`
	IPv6Address   string         `gorm:"type:varchar(128)" json:"ipv6_address,omitempty"`
	CountryCode   string         `gorm:"type:varchar(8)" json:"country_code,omitempty"`
	Status        NodeStatus     `gorm:"type:varchar(16);default:'offline'" json:"status"`
	IncusVersion  string         `gorm:"type:varchar(128)" json:"incus_version,omitempty"`
	TotalCPU      float64        `gorm:"type:float;default:0" json:"total_cpu"`
	TotalMemory   int64          `gorm:"type:bigint;default:0" json:"total_memory"`
	TotalDisk     int64          `gorm:"type:bigint;default:0" json:"total_disk"`
	UsedCPU       float64        `gorm:"type:float;default:0" json:"used_cpu"`
	UsedMemory    int64          `gorm:"type:bigint;default:0" json:"used_memory"`
	UsedDisk      int64          `gorm:"type:bigint;default:0" json:"used_disk"`
	NetIn         int64          `gorm:"type:bigint;default:0" json:"net_in"`
	NetOut        int64          `gorm:"type:bigint;default:0" json:"net_out"`
	Uptime        int64          `gorm:"type:bigint;default:0" json:"uptime"`
	InstanceCount int            `gorm:"type:int;default:0" json:"instance_count"`
	RunningCount  int            `gorm:"type:int;default:0" json:"running_count"`
	LastSeenAt    *time.Time     `gorm:"type:timestamptz" json:"last_seen_at,omitempty"`
	LastHeartbeat *time.Time     `gorm:"type:timestamptz" json:"last_heartbeat,omitempty"`
	CreatedAt     time.Time      `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt     time.Time      `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	// Agent 上报的宿主机探测信息（JSON）
	SystemInfo string `gorm:"type:jsonb;default:'{}'" json:"system_info,omitempty"`

	// Agent 配置（由 Master 前端配置后入库并下发）
	IncusSocketPath    string `gorm:"type:varchar(255);default:'/var/lib/incus/unix.socket'" json:"incus_socket_path,omitempty"`
	MetricsInterval    int    `gorm:"type:int;default:1" json:"metrics_interval,omitempty"`   // 秒
	HeartbeatInterval  int    `gorm:"type:int;default:1" json:"heartbeat_interval,omitempty"` // 秒
	NetworkInterface   string `gorm:"type:varchar(64)" json:"network_interface,omitempty"`
	EnableNAT          bool   `gorm:"type:boolean;default:true" json:"enable_nat,omitempty"`
	EnableFirewall     bool   `gorm:"type:boolean;default:true" json:"enable_firewall,omitempty"`
	EnableSecurityScan bool   `gorm:"type:boolean;default:true" json:"enable_security_scan,omitempty"`
	ScanInterval       int    `gorm:"type:int;default:300" json:"scan_interval,omitempty"` // 秒
	ConsoleBindAddr    string `gorm:"type:varchar(64);default:'0.0.0.0:9090'" json:"console_bind_addr,omitempty"`
	AgentURL           string `gorm:"type:varchar(255)" json:"agent_url,omitempty"`
	ImageRemoteURL     string `gorm:"type:varchar(512)" json:"image_remote_url,omitempty"`

	// 存储池配置
	DefaultStoragePool string `gorm:"type:varchar(64);default:'default'" json:"default_storage_pool,omitempty"`
	StoragePoolType    string `gorm:"type:varchar(16);default:'dir'" json:"storage_pool_type,omitempty"` // dir/zfs/btrfs/lvm
	StoragePoolSource  string `gorm:"type:varchar(255)" json:"storage_pool_source,omitempty"`            // dir=目录路径，其他=设备路径
	StoragePoolCreated bool   `gorm:"type:boolean;default:false" json:"storage_pool_created,omitempty"`

	// 关联
	Instances []Instance `gorm:"foreignKey:NodeID" json:"instances,omitempty"`
}

func (Node) TableName() string {
	return "nodes"
}

// IsOnline 检查节点是否在线
func (n *Node) IsOnline() bool {
	return n.Status == NodeStatusOnline
}

// IsHealthy 检查节点是否健康 (60秒内有心跳)
func (n *Node) IsHealthy() bool {
	if n.LastHeartbeat == nil {
		return false
	}
	return time.Since(*n.LastHeartbeat) < 60*time.Second
}

// BeforeCreate 创建前钩子
func (n *Node) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}
