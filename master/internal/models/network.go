package models

import (
	"time"

	"github.com/google/uuid"
)

// ========== 端口映射 ==========

type PortMapping struct {
	ID                 uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	InstanceID         uuid.UUID `gorm:"type:uuid;not null;index" json:"instance_id"`
	NodeID             uuid.UUID `gorm:"type:uuid;not null;index" json:"node_id"`
	BridgeID           uuid.UUID `gorm:"type:uuid;not null;index" json:"bridge_id"`
	IPVersion          string    `gorm:"type:varchar(8);not null;default:'ipv4'" json:"ip_version"`
	EgressAllocationID uuid.UUID `gorm:"type:uuid;not null;index" json:"egress_allocation_id"`
	ContainerPort      int       `gorm:"type:int;not null" json:"container_port"`
	HostPort           int       `gorm:"type:int;not null" json:"host_port"`
	Protocol           string    `gorm:"type:varchar(8);default:'tcp'" json:"protocol"`
	Description        string    `gorm:"type:varchar(255)" json:"description,omitempty"`
	CreatedAt          time.Time `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`

	Instance         Instance      `gorm:"foreignKey:InstanceID" json:"instance,omitempty"`
	Node             Node          `gorm:"foreignKey:NodeID" json:"node,omitempty"`
	Bridge           Bridge        `gorm:"foreignKey:BridgeID" json:"bridge,omitempty"`
	EgressAllocation EIPAllocation `gorm:"foreignKey:EgressAllocationID" json:"egress_allocation,omitempty"`
}

func (PortMapping) TableName() string {
	return "port_mappings"
}

// ========== 防火墙规则 ==========

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

	Instance Instance `gorm:"foreignKey:InstanceID" json:"instance,omitempty"`
	Node     Node     `gorm:"foreignKey:NodeID" json:"node,omitempty"`
}

func (FirewallRule) TableName() string {
	return "firewall_rules"
}
