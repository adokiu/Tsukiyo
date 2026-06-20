package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// BridgeStatus 网桥状态
type BridgeStatus string

const (
	BridgeStatusActive   BridgeStatus = "active"
	BridgeStatusDisabled BridgeStatus = "disabled"
)

// Bridge 网桥网络表（替代 VPCNetwork）
type Bridge struct {
	ID              uuid.UUID       `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	NodeID          uuid.UUID       `gorm:"type:uuid;not null;index" json:"node_id"`
	Name            string          `gorm:"type:varchar(64);not null" json:"name"`
	BridgeName      string          `gorm:"type:varchar(64);not null" json:"bridge_name"`
	IPv4Enabled     bool            `gorm:"type:boolean;not null;default:true" json:"ipv4_enabled"`
	IPv4CIDR        string          `gorm:"column:ipv4_cidr;type:varchar(32);not null;default:''" json:"ipv4_cidr"`
	IPv4Gateway     string          `gorm:"type:varchar(32);not null;default:''" json:"ipv4_gateway"`
	IPv6Enabled     bool            `gorm:"type:boolean;not null;default:false" json:"ipv6_enabled"`
	IPv6CIDR        string          `gorm:"column:ipv6_cidr;type:varchar(64);not null;default:''" json:"ipv6_cidr"`
	IPv6Gateway     string          `gorm:"type:varchar(64);not null;default:''" json:"ipv6_gateway"`
	DNSServers      json.RawMessage `gorm:"type:jsonb;not null;default:'[]'" json:"dns_servers"`
	NATEgressIPv4ID *uuid.UUID      `gorm:"type:uuid;index" json:"nat_egress_ipv4_id,omitempty"`
	NATEgressIPv6ID *uuid.UUID      `gorm:"type:uuid;index" json:"nat_egress_ipv6_id,omitempty"`
	PortRangeStart  int             `gorm:"type:int;not null;default:20000" json:"port_range_start"`
	PortRangeEnd    int             `gorm:"type:int;not null;default:65535" json:"port_range_end"`
	Status          BridgeStatus    `gorm:"type:varchar(16);not null;default:'active'" json:"status"`
	CreatedAt       time.Time       `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt       time.Time       `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`

	// 关联
	Node          Node           `gorm:"foreignKey:NodeID" json:"node,omitempty"`
	NATEgressIPv4 *EIPAllocation `gorm:"foreignKey:NATEgressIPv4ID" json:"nat_egress_ipv4,omitempty"`
	NATEgressIPv6 *EIPAllocation `gorm:"foreignKey:NATEgressIPv6ID" json:"nat_egress_ipv6,omitempty"`
}

func (Bridge) TableName() string {
	return "bridges"
}

// GetBridgeName 返回 Incus bridge 名称
func (b *Bridge) GetBridgeName() string {
	if b.BridgeName != "" {
		return b.BridgeName
	}
	return "br-" + b.ID.String()[:8]
}
