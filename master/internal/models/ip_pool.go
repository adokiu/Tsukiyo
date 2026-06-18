package models

import (
	"fmt"
	"net"
	"sync"

	"github.com/google/uuid"
)

// IPPool IPv4 地址池，管理一个 CIDR 段内所有地址的分配状态
type IPPool struct {
	mu        sync.RWMutex
	CIDR      string            // CIDR 段，如 "10.10.1.0/24"
	Gateway   string            // 网关地址
	Reserved  map[string]bool   // 预留地址（网关、广播等）
	Allocated map[string]string // 已分配地址 -> 实例ID/用途
}

// NewIPPool 从 CIDR 创建地址池
func NewIPPool(cidr, gateway string) (*IPPool, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR %s: %w", cidr, err)
	}

	pool := &IPPool{
		CIDR:      cidr,
		Gateway:   gateway,
		Reserved:  make(map[string]bool),
		Allocated: make(map[string]string),
	}

	// 预留网络地址、广播地址、网关地址
	pool.reserveNetworkAddresses(ipNet, gateway)
	return pool, nil
}

// reserveNetworkAddresses 预留不可分配的地址
func (p *IPPool) reserveNetworkAddresses(ipNet *net.IPNet, gateway string) {
	// 网络地址和广播地址
	ip := ipNet.IP.Mask(ipNet.Mask)
	p.Reserved[ip.String()] = true
	broadcast := make(net.IP, len(ip))
	copy(broadcast, ip)
	for i := range broadcast {
		broadcast[i] |= ^ipNet.Mask[i]
	}
	p.Reserved[broadcast.String()] = true

	// 网关地址
	if gateway != "" {
		p.Reserved[gateway] = true
	}
}

// Allocate 分配一个可用 IP，返回分配到的 IP 或错误
func (p *IPPool) Allocate(ownerID string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, ipNet, err := net.ParseCIDR(p.CIDR)
	if err != nil {
		return "", err
	}

	// 遍历整个网段找第一个可用 IP
	ip := ipNet.IP.Mask(ipNet.Mask)
	for ipNet.Contains(ip) {
		ipStr := ip.String()
		if !p.Reserved[ipStr] && p.Allocated[ipStr] == "" {
			p.Allocated[ipStr] = ownerID
			return ipStr, nil
		}
		incrementIP(ip)
	}

	return "", fmt.Errorf("no available IP in pool %s", p.CIDR)
}

// AllocateSpecific 分配指定 IP
func (p *IPPool) AllocateSpecific(ipStr, ownerID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, ipNet, err := net.ParseCIDR(p.CIDR)
	if err != nil {
		return err
	}

	ip := net.ParseIP(ipStr)
	if ip == nil || !ipNet.Contains(ip) {
		return fmt.Errorf("IP %s not in CIDR %s", ipStr, p.CIDR)
	}

	if p.Reserved[ipStr] {
		return fmt.Errorf("IP %s is reserved", ipStr)
	}
	if p.Allocated[ipStr] != "" {
		return fmt.Errorf("IP %s already allocated to %s", ipStr, p.Allocated[ipStr])
	}

	p.Allocated[ipStr] = ownerID
	return nil
}

// Release 释放一个已分配的 IP
func (p *IPPool) Release(ipStr string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Allocated[ipStr] == "" {
		return false
	}
	delete(p.Allocated, ipStr)
	return true
}

// IsAvailable 检查 IP 是否可用
func (p *IPPool) IsAvailable(ipStr string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return !p.Reserved[ipStr] && p.Allocated[ipStr] == ""
}

// incrementIP 将 IP 地址加 1
func incrementIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
}

// IPPoolEntry 数据库持久化的 IP 池条目
type IPPoolEntry struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PoolType  string    `gorm:"type:varchar(16);not null" json:"pool_type"` // vpc_internal / public_v4 / public_v6
	OwnerID   uuid.UUID `gorm:"type:uuid;index" json:"owner_id"`            // VPC ID 或 Node ID
	Address   string    `gorm:"type:inet;not null;index" json:"address"`
	Gateway   string    `gorm:"type:varchar(32)" json:"gateway,omitempty"`
	PrefixLen int       `gorm:"type:int;not null" json:"prefix_len"`
	Status    string    `gorm:"type:varchar(16);default:'free'" json:"status"` // free / allocated / reserved
	BoundTo   uuid.UUID `gorm:"type:uuid;index" json:"bound_to,omitempty"`     // 绑定的实例ID
	Interface string    `gorm:"type:varchar(32)" json:"interface,omitempty"`
	AliasIP   string    `gorm:"type:varchar(32)" json:"alias_ip,omitempty"`
	CreatedAt int64     `gorm:"autoCreateTime" json:"created_at"`
}

func (IPPoolEntry) TableName() string {
	return "ip_pool_entries"
}

// VPCNetwork VPC 网络表
type VPCNetwork struct {
	ID               uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	NodeID           uuid.UUID `gorm:"type:uuid;not null;index" json:"node_id"`
	Name             string    `gorm:"type:varchar(64);not null" json:"name"`
	IPv4CIDR         string    `gorm:"type:varchar(32);not null" json:"ipv4_cidr"`
	IPv6ULACIDR      string    `gorm:"type:varchar(64)" json:"ipv6_ula_cidr,omitempty"`
	IPv6GUACIDR      string    `gorm:"type:varchar(64)" json:"ipv6_gua_cidr,omitempty"`
	DefaultGatewayV4 string    `gorm:"type:varchar(32)" json:"default_gateway_v4,omitempty"`
	DefaultGatewayV6 string    `gorm:"type:varchar(64)" json:"default_gateway_v6,omitempty"`
	EgressV4Primary  string    `gorm:"type:varchar(32)" json:"egress_v4_primary,omitempty"`
	EgressV4Extra    string    `gorm:"type:text" json:"egress_v4_extra,omitempty"` // JSON 数组，独立公网 IP 地址池
	PortRangeStart   int       `gorm:"type:int;default:10000" json:"port_range_start"`
	PortRangeEnd     int       `gorm:"type:int;default:65535" json:"port_range_end"`
	AddressAliases   []byte    `gorm:"type:jsonb" json:"address_aliases,omitempty"`
	ParentIface      string    `gorm:"type:varchar(32)" json:"parent_iface,omitempty"`
	Status           string    `gorm:"type:varchar(16);default:'active'" json:"status"`
	BridgeName       string    `gorm:"type:varchar(64)" json:"bridge_name,omitempty"`
	SNATEnabled      bool      `gorm:"type:boolean;default:true" json:"snat_enabled"`
	IPv4Filter       bool      `gorm:"type:boolean;default:true" json:"ipv4_filter"`
	IPv6Filter       bool      `gorm:"type:boolean;default:true" json:"ipv6_filter"`
	MACFilter        bool      `gorm:"type:boolean;default:true" json:"mac_filter"`
	CreatedAt        int64     `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        int64     `gorm:"autoUpdateTime" json:"updated_at"`
}

func (VPCNetwork) TableName() string {
	return "vpc_networks"
}

// GetBridgeName 返回 Incus bridge 名称
func (v *VPCNetwork) GetBridgeName() string {
	if v.BridgeName != "" {
		return v.BridgeName
	}
	// 默认使用 vpc-<前8位uuid>
	return fmt.Sprintf("vpc-%s", v.ID.String()[:8])
}
