package models

import (
	"time"

	"github.com/google/uuid"
)

// IPStatus IP 状态
type IPStatus string

const (
	IPStatusFree     IPStatus = "free"
	IPStatusAssigned IPStatus = "assigned"
	IPStatusReserved IPStatus = "reserved"
)

// PublicIPPool 公网 IP 池表
type PublicIPPool struct {
	ID         uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	NodeID     uuid.UUID  `gorm:"type:uuid;not null;index" json:"node_id"`
	Address    string     `gorm:"type:inet;not null;index" json:"address"`
	Gateway    string     `gorm:"type:inet" json:"gateway,omitempty"`
	PrefixLen  int        `gorm:"type:int;default:32" json:"prefix_len"`
	Interface  string     `gorm:"type:varchar(32)" json:"interface,omitempty"`
	Status     IPStatus   `gorm:"type:varchar(16);default:'free'" json:"status"`
	InstanceID uuid.UUID  `gorm:"type:uuid;index" json:"instance_id,omitempty"`
	AssignedAt *time.Time `gorm:"type:timestamptz" json:"assigned_at,omitempty"`
	CreatedAt  time.Time  `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`

	// 关联
	Node     Node     `gorm:"foreignKey:NodeID" json:"node,omitempty"`
	Instance Instance `gorm:"foreignKey:InstanceID" json:"instance,omitempty"`
}

func (PublicIPPool) TableName() string {
	return "public_ip_pools"
}

// IsFree 检查 IP 是否空闲
func (p *PublicIPPool) IsFree() bool {
	return p.Status == IPStatusFree
}

// IPv6Prefix IPv6 前缀表
type IPv6Prefix struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	NodeID    uuid.UUID `gorm:"type:uuid;not null;index" json:"node_id"`
	Prefix    string    `gorm:"type:cidr;not null" json:"prefix"`
	PrefixLen int       `gorm:"type:int;not null" json:"prefix_len"`
	Interface string    `gorm:"type:varchar(32)" json:"interface,omitempty"`
	Gateway   string    `gorm:"type:inet" json:"gateway,omitempty"`
	Status    string    `gorm:"type:varchar(16);default:'active'" json:"status"`
	CreatedAt time.Time `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`

	// 关联
	Node Node `gorm:"foreignKey:NodeID" json:"node,omitempty"`
}

func (IPv6Prefix) TableName() string {
	return "ipv6_prefixes"
}

// PortMapping 端口映射表
type PortMapping struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	InstanceID    uuid.UUID `gorm:"type:uuid;not null;index" json:"instance_id"`
	NodeID        uuid.UUID `gorm:"type:uuid;not null;index" json:"node_id"`
	ContainerPort int       `gorm:"type:int;not null" json:"container_port"`
	HostPort      int       `gorm:"type:int;not null;index" json:"host_port"`
	Protocol      string    `gorm:"type:varchar(8);default:'tcp'" json:"protocol"`
	HostIP        string    `gorm:"type:varchar(64)" json:"host_ip,omitempty"`
	Description   string    `gorm:"type:varchar(255)" json:"description,omitempty"`
	CreatedAt     time.Time `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`

	// 关联
	Instance Instance `gorm:"foreignKey:InstanceID" json:"instance,omitempty"`
	Node     Node     `gorm:"foreignKey:NodeID" json:"node,omitempty"`
}

func (PortMapping) TableName() string {
	return "port_mappings"
}

// FirewallRule 防火墙规则表
type FirewallRule struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	InstanceID  uuid.UUID `gorm:"type:uuid;not null;index" json:"instance_id"`
	NodeID      uuid.UUID `gorm:"type:uuid;not null;index" json:"node_id"`
	Network     string    `gorm:"type:varchar(8);default:'ipv4'" json:"network"`
	Direction   string    `gorm:"type:varchar(8);not null" json:"direction"`
	Protocol    string    `gorm:"type:varchar(8);default:'all'" json:"protocol"`
	Port        string    `gorm:"type:varchar(64)" json:"port,omitempty"`
	SourceIP    string    `gorm:"type:inet" json:"source_ip,omitempty"`
	Action      string    `gorm:"type:varchar(8);not null" json:"action"`
	Description string    `gorm:"type:varchar(255)" json:"description,omitempty"`
	Enabled     bool      `gorm:"type:boolean;default:true" json:"enabled"`
	Priority    int       `gorm:"type:int;default:100" json:"priority"`
	CreatedAt   time.Time `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt   time.Time `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`

	// 关联
	Instance Instance `gorm:"foreignKey:InstanceID" json:"instance,omitempty"`
	Node     Node     `gorm:"foreignKey:NodeID" json:"node,omitempty"`
}

func (FirewallRule) TableName() string {
	return "firewall_rules"
}
