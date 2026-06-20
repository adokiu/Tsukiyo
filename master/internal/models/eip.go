package models

import (
	"time"

	"github.com/google/uuid"
)

// EIPPoolType 资源池类型
type EIPPoolType string

const (
	EIPPoolTypeHost EIPPoolType = "host"
	EIPPoolTypeEIP  EIPPoolType = "eip"
)

// EIPPoolStatus 资源池状态
type EIPPoolStatus string

const (
	EIPPoolStatusActive   EIPPoolStatus = "active"
	EIPPoolStatusDisabled EIPPoolStatus = "disabled"
)

// EIPPool EIP 资源池
type EIPPool struct {
	ID        uuid.UUID     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	NodeID    uuid.UUID     `gorm:"type:uuid;not null;index" json:"node_id"`
	IPVersion string        `gorm:"type:varchar(8);not null" json:"ip_version"`
	CIDR      string        `gorm:"column:cidr;type:varchar(64);not null" json:"cidr"`
	Interface string        `gorm:"type:varchar(32);not null;default:''" json:"interface"`
	Gateway   string        `gorm:"type:varchar(64);not null;default:''" json:"gateway"`
	PrefixLen int           `gorm:"type:int;not null" json:"prefix_len"`
	Alias     string        `gorm:"type:varchar(128);not null;default:''" json:"alias"`
	PoolType  EIPPoolType   `gorm:"type:varchar(8);not null;default:'eip'" json:"pool_type"`
	Status    EIPPoolStatus `gorm:"type:varchar(16);not null;default:'active'" json:"status"`
	CreatedAt time.Time     `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt time.Time     `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`

	// 关联
	Node        Node            `gorm:"foreignKey:NodeID" json:"node,omitempty"`
	Allocations []EIPAllocation `gorm:"foreignKey:PoolID" json:"allocations,omitempty"`
}

func (EIPPool) TableName() string {
	return "eip_pools"
}

// EIPAllocationUsage 分配用途
type EIPAllocationUsage string

const (
	EIPUsageBridgeNATEgress EIPAllocationUsage = "bridge_nat_egress"
	EIPUsageInstanceEIP     EIPAllocationUsage = "instance_eip"
)

// EIPAllocationStatus 分配状态
type EIPAllocationStatus string

const (
	EIPAllocationAssigned EIPAllocationStatus = "assigned"
	EIPAllocationReleased EIPAllocationStatus = "released"
)

// EIPAllocation EIP 分配记录
type EIPAllocation struct {
	ID          uuid.UUID           `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PoolID      uuid.UUID           `gorm:"type:uuid;not null;index" json:"pool_id"`
	NodeID      uuid.UUID           `gorm:"type:uuid;not null;index" json:"node_id"`
	CIDR        string              `gorm:"column:cidr;type:varchar(64);not null" json:"cidr"`
	PrefixLen   int                 `gorm:"type:int;not null" json:"prefix_len"`
	IPVersion   string              `gorm:"type:varchar(8);not null" json:"ip_version"`
	Usage       EIPAllocationUsage  `gorm:"type:varchar(20);not null" json:"usage"`
	BridgeID    *uuid.UUID          `gorm:"type:uuid;index" json:"bridge_id,omitempty"`
	InstanceID  *uuid.UUID          `gorm:"type:uuid;index" json:"instance_id,omitempty"`
	Status      EIPAllocationStatus `gorm:"type:varchar(16);not null;default:'assigned'" json:"status"`
	AllocatedAt time.Time           `gorm:"type:timestamptz;not null;default:now()" json:"allocated_at"`
	ReleasedAt  *time.Time          `gorm:"type:timestamptz" json:"released_at,omitempty"`

	// 关联
	Pool     EIPPool   `gorm:"foreignKey:PoolID" json:"pool,omitempty"`
	Node     Node      `gorm:"foreignKey:NodeID" json:"node,omitempty"`
	Bridge   *Bridge   `gorm:"foreignKey:BridgeID" json:"bridge,omitempty"`
	Instance *Instance `gorm:"foreignKey:InstanceID" json:"instance,omitempty"`
}

func (EIPAllocation) TableName() string {
	return "eip_allocations"
}

// GetIP 返回去掉掩码的 IP 地址
func (a *EIPAllocation) GetIP() string {
	cidr := a.CIDR
	for i := 0; i < len(cidr); i++ {
		if cidr[i] == '/' {
			return cidr[:i]
		}
	}
	return cidr
}
