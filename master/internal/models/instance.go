package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// InstanceType 实例类型
type InstanceType string

const (
	InstanceTypeContainer InstanceType = "container"
	InstanceTypeVM        InstanceType = "vm"
)

// InstanceStatus 实例状态
type InstanceStatus string

const (
	InstanceStatusCreating     InstanceStatus = "creating"
	InstanceStatusRunning      InstanceStatus = "running"
	InstanceStatusStopped      InstanceStatus = "stopped"
	InstanceStatusRestarting   InstanceStatus = "restarting"
	InstanceStatusDeleting     InstanceStatus = "deleting"
	InstanceStatusError        InstanceStatus = "error"
	InstanceStatusReinstalling InstanceStatus = "reinstalling"
	InstanceStatusExpired      InstanceStatus = "expired"
)

// TrafficMode 流量计算模式
type TrafficMode string

const (
	TrafficModeTotal    TrafficMode = "total"
	TrafficModeOutbound TrafficMode = "outbound"
	TrafficModeInbound  TrafficMode = "inbound"
	TrafficModeMax      TrafficMode = "max"
)

// OverLimitAction 超额处理策略
type OverLimitAction string

const (
	OverLimitActionShutdown OverLimitAction = "shutdown"
	OverLimitActionThrottle OverLimitAction = "throttle"
)

// LoginMethod 登录方式
type LoginMethod string

const (
	LoginMethodAuto     LoginMethod = "auto"
	LoginMethodPassword LoginMethod = "password"
	LoginMethodSSHKey   LoginMethod = "sshkey"
)

// Instance 实例表
type Instance struct {
	ID                  uuid.UUID       `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name                string          `gorm:"type:varchar(64);not null" json:"name"`
	UserID              uint            `gorm:"not null;index" json:"user_id"`
	NodeID              uuid.UUID       `gorm:"type:uuid;not null;index" json:"node_id"`
	Type                InstanceType    `gorm:"type:varchar(16);not null" json:"type"`
	Status              InstanceStatus  `gorm:"type:varchar(16);default:'creating'" json:"status"`
	IncusName           string          `gorm:"type:varchar(64);not null" json:"incus_name"`
	TemplateID          string          `gorm:"type:varchar(64)" json:"template_id,omitempty"`
	BridgeID            *uuid.UUID      `gorm:"type:uuid;index" json:"bridge_id,omitempty"`
	InternalIPv4        string          `gorm:"type:inet" json:"internal_ipv4,omitempty"`
	InternalIPv6        string          `gorm:"type:varchar(64);default:''" json:"internal_ipv6,omitempty"`
	VCPU                float64         `gorm:"column:vcpu;type:float;default:1" json:"vcpu"`
	MemoryMB            int             `gorm:"type:int;default:512" json:"memory_mb"`
	DiskGB              int             `gorm:"type:int;default:10" json:"disk_gb"`
	NetworkDownMbps     int             `gorm:"type:int;default:0" json:"network_down_mbps"`
	NetworkUpMbps       int             `gorm:"type:int;default:0" json:"network_up_mbps"`
	IOReadMBps          int             `gorm:"column:io_read_mbps;type:int;default:0" json:"io_read_mbps"`
	IOWriteMBps         int             `gorm:"column:io_write_mbps;type:int;default:0" json:"io_write_mbps"`
	IPv4Mode            string          `gorm:"type:varchar(8);not null;default:'nat'" json:"ipv4_mode"`
	IPv6Mode            string          `gorm:"type:varchar(8);not null;default:'none'" json:"ipv6_mode"`
	IPv4EIPAllocationID *uuid.UUID      `gorm:"column:ipv4_eip_allocation_id;type:uuid;index" json:"ipv4_eip_allocation_id,omitempty"`
	IPv6EIPAllocationID *uuid.UUID      `gorm:"column:ipv6_eip_allocation_id;type:uuid;index" json:"ipv6_eip_allocation_id,omitempty"`
	SSHPort             int             `gorm:"type:int" json:"ssh_port,omitempty"`
	SSHPassword         string          `gorm:"type:varchar(255)" json:"-"`
	SSHPublicKey        string          `gorm:"type:text" json:"-"`
	MACAddress          string          `gorm:"type:varchar(32)" json:"mac_address,omitempty"`
	StoragePool         string          `gorm:"type:varchar(64);default:'default'" json:"storage_pool"`
	LoginMethod         LoginMethod     `gorm:"type:varchar(16);default:'auto'" json:"login_method"`
	TrafficMode         TrafficMode     `gorm:"type:varchar(16);default:'total'" json:"traffic_mode"`
	TrafficInGB         int64           `gorm:"type:bigint;default:0" json:"traffic_in_gb"`
	TrafficOutGB        int64           `gorm:"type:bigint;default:0" json:"traffic_out_gb"`
	MonthlyTrafficGB    int64           `gorm:"type:bigint;default:0" json:"monthly_traffic_gb"`
	TrafficUsedGB       float64         `gorm:"type:float;default:0" json:"traffic_used_gb"`
	TrafficResetDate    string          `gorm:"type:varchar(7);default:''" json:"traffic_reset_date"`
	OverLimitAction     OverLimitAction `gorm:"type:varchar(16);default:'shutdown'" json:"over_limit_action"`
	IsOverLimit         bool            `gorm:"type:boolean;default:false" json:"is_over_limit"`
	SnapshotLimit       int             `gorm:"type:int;default:5" json:"snapshot_limit"`
	PortMappingLimit    int             `gorm:"type:int;default:2" json:"port_mapping_limit"`
	ExpiresAt           *time.Time      `gorm:"type:timestamptz" json:"expires_at,omitempty"`
	ExpiredAt           *time.Time      `gorm:"type:timestamptz" json:"expired_at,omitempty"`
	VNCPort             int             `gorm:"type:int" json:"vnc_port,omitempty"`
	CreatedAt           time.Time       `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt           time.Time       `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
	DeletedAt           gorm.DeletedAt  `gorm:"index" json:"-"`

	// 关联
	User          User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Node          Node           `gorm:"foreignKey:NodeID" json:"node,omitempty"`
	PortMappings  []PortMapping  `gorm:"foreignKey:InstanceID" json:"port_mappings,omitempty"`
	FirewallRules []FirewallRule `gorm:"foreignKey:InstanceID" json:"firewall_rules,omitempty"`
	Snapshots     []Snapshot     `gorm:"foreignKey:InstanceID" json:"snapshots,omitempty"`
	DataDisks     []DataDisk     `gorm:"foreignKey:InstanceID" json:"data_disks,omitempty"`
	Bridge        *Bridge        `gorm:"foreignKey:BridgeID" json:"bridge,omitempty"`
	IPv4EIP       *EIPAllocation `gorm:"foreignKey:IPv4EIPAllocationID" json:"ipv4_eip,omitempty"`
	IPv6EIP       *EIPAllocation `gorm:"foreignKey:IPv6EIPAllocationID" json:"ipv6_eip,omitempty"`
}

func (Instance) TableName() string {
	return "instances"
}

// IsRunning 检查实例是否运行中
func (i *Instance) IsRunning() bool {
	return i.Status == InstanceStatusRunning
}

// IsExpired 检查实例是否已到期
func (i *Instance) IsExpired() bool {
	if i.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*i.ExpiresAt)
}

// BeforeCreate 创建前钩子
func (i *Instance) BeforeCreate(tx *gorm.DB) error {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	return nil
}

// Snapshot 实例快照表
type Snapshot struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	InstanceID  uuid.UUID `gorm:"type:uuid;not null;index" json:"instance_id"`
	Name        string    `gorm:"type:varchar(64);not null" json:"name"`
	Description string    `gorm:"type:text" json:"description,omitempty"`
	SizeBytes   int64     `gorm:"type:bigint;default:0" json:"size_bytes"`
	IsScheduled bool      `gorm:"type:boolean;default:false" json:"is_scheduled"`
	CreatedAt   time.Time `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
}

func (Snapshot) TableName() string {
	return "snapshots"
}

// DataDisk 数据磁盘表
type DataDisk struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	InstanceID  uuid.UUID `gorm:"type:uuid;not null;index" json:"instance_id"`
	NodeID      uuid.UUID `gorm:"type:uuid;not null;index" json:"node_id"`
	Name        string    `gorm:"type:varchar(64);not null" json:"name"`
	SizeGB      int       `gorm:"type:int;not null" json:"size_gb"`
	StoragePool string    `gorm:"type:varchar(64);default:'default'" json:"storage_pool"`
	MountPoint  string    `gorm:"type:varchar(255)" json:"mount_point,omitempty"`
	CreatedAt   time.Time `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
}

func (DataDisk) TableName() string {
	return "data_disks"
}

// InstanceMetric 监控指标表 (时序数据，按时间分区)
type InstanceMetric struct {
	ID           int64     `gorm:"type:bigserial;primary_key" json:"-"`
	InstanceID   uuid.UUID `gorm:"type:uuid;not null;index" json:"instance_id"`
	NodeID       uuid.UUID `gorm:"type:uuid;not null;index" json:"node_id"`
	Timestamp    time.Time `gorm:"type:timestamptz;not null;index" json:"timestamp"`
	CPUPercent   float64   `gorm:"type:float;default:0" json:"cpu_percent"`
	MemUsed      int64     `gorm:"type:bigint;default:0" json:"mem_used"`
	MemTotal     int64     `gorm:"type:bigint;default:0" json:"mem_total"`
	DiskUsed     int64     `gorm:"type:bigint;default:0" json:"disk_used"`
	DiskTotal    int64     `gorm:"type:bigint;default:0" json:"disk_total"`
	DiskReadBps  int64     `gorm:"type:bigint;default:0" json:"disk_read_bps"`
	DiskWriteBps int64     `gorm:"type:bigint;default:0" json:"disk_write_bps"`
	NetInBps     int64     `gorm:"type:bigint;default:0" json:"net_in_bps"`
	NetOutBps    int64     `gorm:"type:bigint;default:0" json:"net_out_bps"`
	NetInTotal   int64     `gorm:"type:bigint;default:0" json:"net_in_total"`
	NetOutTotal  int64     `gorm:"type:bigint;default:0" json:"net_out_total"`
}

func (InstanceMetric) TableName() string {
	return "instance_metrics"
}
