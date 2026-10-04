package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProductCategoryStatus 商品分类状态
type ProductCategoryStatus string

const (
	ProductCategoryStatusActive   ProductCategoryStatus = "active"
	ProductCategoryStatusDisabled ProductCategoryStatus = "disabled"
)

// ProductCategory 商品分类表（两级分类）
type ProductCategory struct {
	ID          uuid.UUID             `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ParentID    *uuid.UUID            `gorm:"type:uuid;index" json:"parent_id,omitempty"`
	Name        string                `gorm:"type:varchar(128);not null" json:"name"`
	Sort        int                   `gorm:"type:int;not null;default:0" json:"sort"`
	Status      ProductCategoryStatus `gorm:"type:varchar(16);not null;default:'active'" json:"status"`
	Description string                `gorm:"type:text" json:"description,omitempty"`
	CreatedAt   time.Time             `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt   time.Time             `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
	DeletedAt   gorm.DeletedAt        `gorm:"index" json:"-"`

	// 关联
	Parent   *ProductCategory  `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	Children []ProductCategory `gorm:"foreignKey:ParentID" json:"children,omitempty"`
	Products []Product         `gorm:"foreignKey:CategoryID" json:"products,omitempty"`
}

func (ProductCategory) TableName() string {
	return "product_categories"
}

// ProductType 商品类型
type ProductType string

const (
	ProductTypeContainer ProductType = "container"
	ProductTypeVM        ProductType = "vm"
)

// IPv4Mode 商品 IPv4 模式
type ProductIPv4Mode string

const (
	ProductIPv4ModeNAT ProductIPv4Mode = "nat"
	ProductIPv4ModeEIP ProductIPv4Mode = "eip"
)

// TrafficCalcMode 流量计算方式
type TrafficCalcMode string

const (
	TrafficCalcBoth     TrafficCalcMode = "both"
	TrafficCalcInbound  TrafficCalcMode = "inbound"
	TrafficCalcOutbound TrafficCalcMode = "outbound"
	TrafficCalcMax      TrafficCalcMode = "max"
)

// ProductStatus 商品状态
type ProductStatus string

const (
	ProductStatusActive   ProductStatus = "active"
	ProductStatusDisabled ProductStatus = "disabled"
)

// PaymentPeriod 付款周期
type PaymentPeriod string

const (
	PaymentPeriodMonthly    PaymentPeriod = "monthly"
	PaymentPeriodQuarterly  PaymentPeriod = "quarterly"
	PaymentPeriodHalfYearly PaymentPeriod = "half_yearly"
	PaymentPeriodYearly     PaymentPeriod = "yearly"
)

// CustomMode 增配模式
type CustomMode string

const (
	CustomModeUnlimited CustomMode = "unlimited"
	CustomModeTiers     CustomMode = "tiers"
)

// MemoryUnit 内存单价单位
type MemoryUnit string

const (
	MemoryUnitMB MemoryUnit = "mb"
	MemoryUnitGB MemoryUnit = "gb"
)

// Product 商品表
type Product struct {
	ID         uuid.UUID     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	CategoryID *uuid.UUID    `gorm:"type:uuid;index" json:"category_id,omitempty"`
	Name       string        `gorm:"type:varchar(256);not null" json:"name"`
	Type       ProductType   `gorm:"type:varchar(16);not null;default:'container'" json:"type"`
	Status     ProductStatus `gorm:"type:varchar(16);not null;default:'active'" json:"status"`
	Sort       int           `gorm:"type:int;not null;default:0" json:"sort"`
	Stock      int           `gorm:"type:int;not null;default:0" json:"stock"`

	// 承载宿主机（JSON 数组，可选多个，实例开在哪个宿主机动态分配）
	NodeIDs json.RawMessage `gorm:"type:jsonb;not null;default:'[]'" json:"node_ids"`

	// CPU 配置
	VCPUMin            int             `gorm:"column:vcpu_min;type:int;not null;default:1" json:"vcpu_min"`
	VCPUMax            int             `gorm:"column:vcpu_max;type:int;not null;default:1" json:"vcpu_max"`
	VCPUCustomMode     CustomMode      `gorm:"column:vcpu_custom_mode;type:varchar(16);not null;default:'unlimited'" json:"vcpu_custom_mode"`
	VCPUTiers          json.RawMessage `gorm:"column:vcpu_tiers;type:jsonb;not null;default:'[]'" json:"vcpu_tiers"`
	VCPUUnitPriceCents int64           `gorm:"column:vcpu_unit_price_cents;type:bigint;not null;default:0" json:"vcpu_unit_price_cents"`

	// 内存配置
	MemoryMinMB          int             `gorm:"type:int;not null;default:1024" json:"memory_min_mb"`
	MemoryMaxMB          int             `gorm:"type:int;not null;default:1024" json:"memory_max_mb"`
	MemoryCustomMode     CustomMode      `gorm:"type:varchar(16);not null;default:'unlimited'" json:"memory_custom_mode"`
	MemoryTiers          json.RawMessage `gorm:"type:jsonb;not null;default:'[]'" json:"memory_tiers"`
	MemoryUnitPriceCents int64           `gorm:"type:bigint;not null;default:0" json:"memory_unit_price_cents"`
	MemoryUnit           MemoryUnit      `gorm:"type:varchar(4);not null;default:'mb'" json:"memory_unit"`

	// 系统盘配置
	DiskMinMB          int             `gorm:"type:int;not null;default:10240" json:"disk_min_mb"`
	DiskMaxMB          int             `gorm:"type:int;not null;default:10240" json:"disk_max_mb"`
	DiskCustomMode     CustomMode      `gorm:"type:varchar(16);not null;default:'unlimited'" json:"disk_custom_mode"`
	DiskTiers          json.RawMessage `gorm:"type:jsonb;not null;default:'[]'" json:"disk_tiers"`
	DiskUnitPriceCents int64           `gorm:"type:bigint;not null;default:0" json:"disk_unit_price_cents"`
	// 系统盘存储池（按宿主机分别选择，JSON: {"nodeId": "poolName"}）
	DiskStoragePools json.RawMessage `gorm:"type:jsonb;not null;default:'{}'" json:"disk_storage_pools"`

	// 数据盘配置
	DataDiskAllow          bool            `gorm:"type:boolean;not null;default:true" json:"data_disk_allow"`
	DataDiskMinMB          int             `gorm:"type:int;not null;default:0" json:"data_disk_min_mb"`
	DataDiskMaxMB          int             `gorm:"type:int;not null;default:0" json:"data_disk_max_mb"`
	DataDiskCustomMode     CustomMode      `gorm:"type:varchar(16);not null;default:'unlimited'" json:"data_disk_custom_mode"`
	DataDiskTiers          json.RawMessage `gorm:"type:jsonb;not null;default:'[]'" json:"data_disk_tiers"`
	DataDiskUnitPriceCents int64           `gorm:"type:bigint;not null;default:0" json:"data_disk_unit_price_cents"`
	// 数据盘存储池（按宿主机分别选择，JSON: {"nodeId": "poolName"}）
	DataDiskStoragePools json.RawMessage `gorm:"type:jsonb;not null;default:'{}'" json:"data_disk_storage_pools"`

	// 网络带宽配置
	NetworkDownMinMbps        int             `gorm:"type:int;not null;default:0" json:"network_down_min_mbps"`
	NetworkDownMaxMbps        int             `gorm:"type:int;not null;default:0" json:"network_down_max_mbps"`
	NetworkDownCustomMode     CustomMode      `gorm:"column:network_down_custom_mode;type:varchar(16);not null;default:'unlimited'" json:"network_down_custom_mode"`
	NetworkDownTiers          json.RawMessage `gorm:"column:network_down_tiers;type:jsonb;not null;default:'[]'" json:"network_down_tiers"`
	NetworkUpMinMbps          int             `gorm:"type:int;not null;default:0" json:"network_up_min_mbps"`
	NetworkUpMaxMbps          int             `gorm:"type:int;not null;default:0" json:"network_up_max_mbps"`
	NetworkUpCustomMode       CustomMode      `gorm:"column:network_up_custom_mode;type:varchar(16);not null;default:'unlimited'" json:"network_up_custom_mode"`
	NetworkUpTiers            json.RawMessage `gorm:"column:network_up_tiers;type:jsonb;not null;default:'[]'" json:"network_up_tiers"`
	NetworkDownUnitPriceCents int64           `gorm:"type:bigint;not null;default:0" json:"network_down_unit_price_cents"`
	NetworkUpUnitPriceCents   int64           `gorm:"type:bigint;not null;default:0" json:"network_up_unit_price_cents"`

	// 流量配置
	TrafficMinGB          int             `gorm:"type:int;not null;default:0" json:"traffic_min_gb"`
	TrafficMaxGB          int             `gorm:"type:int;not null;default:0" json:"traffic_max_gb"`
	TrafficCalcMode       TrafficCalcMode `gorm:"type:varchar(16);not null;default:'both'" json:"traffic_calc_mode"`
	TrafficCustomMode     CustomMode      `gorm:"column:traffic_custom_mode;type:varchar(16);not null;default:'unlimited'" json:"traffic_custom_mode"`
	TrafficTiers          json.RawMessage `gorm:"column:traffic_tiers;type:jsonb;not null;default:'[]'" json:"traffic_tiers"`
	TrafficUnitPriceCents int64           `gorm:"type:bigint;not null;default:0" json:"traffic_unit_price_cents"`

	// 网桥配置（按宿主机分别选择，JSON: {"nodeId": "bridgeId"}）
	NodeBridges json.RawMessage `gorm:"type:jsonb;not null;default:'{}'" json:"node_bridges"`

	// IPv4 配置
	IPv4Mode ProductIPv4Mode `gorm:"column:ipv4_mode;type:varchar(16);not null;default:'nat'" json:"ipv4_mode"`
	// 独立 IPv4 地址池（按宿主机分别选择，JSON: {"nodeId": ["poolId1", "poolId2"]}）
	IPv4EIPPools            json.RawMessage `gorm:"column:ipv4_eip_pools;type:jsonb;not null;default:'{}'" json:"ipv4_eip_pools"`
	IPv4EIPMin              int             `gorm:"column:ipv4_eip_min;type:int;not null;default:0" json:"ipv4_eip_min"`
	IPv4EIPMax              int             `gorm:"column:ipv4_eip_max;type:int;not null;default:0" json:"ipv4_eip_max"`
	IPv4EIPCustomMode       CustomMode      `gorm:"column:ipv4_eip_custom_mode;type:varchar(16);not null;default:'unlimited'" json:"ipv4_eip_custom_mode"`
	IPv4EIPTiers            json.RawMessage `gorm:"column:ipv4_eip_tiers;type:jsonb;not null;default:'[]'" json:"ipv4_eip_tiers"`
	IPv4EIPUnitPriceCents   int64           `gorm:"column:ipv4_eip_unit_price_cents;type:bigint;not null;default:0" json:"ipv4_eip_unit_price_cents"`
	IPv4EIPAllowChange      bool            `gorm:"column:ipv4_eip_allow_change;type:boolean;not null;default:false" json:"ipv4_eip_allow_change"`
	IPv4EIPChangePriceCents int64           `gorm:"column:ipv4_eip_change_price_cents;type:bigint;not null;default:0" json:"ipv4_eip_change_price_cents"`

	// NAT 端口映射配置（ipv4_mode 为 nat 时生效）
	NATPortMin            int             `gorm:"column:nat_port_min;type:int;not null;default:0" json:"nat_port_min"`
	NATPortMax            int             `gorm:"column:nat_port_max;type:int;not null;default:0" json:"nat_port_max"`
	NATPortCustomMode     CustomMode      `gorm:"column:nat_port_custom_mode;type:varchar(16);not null;default:'unlimited'" json:"nat_port_custom_mode"`
	NATPortTiers          json.RawMessage `gorm:"column:nat_port_tiers;type:jsonb;not null;default:'[]'" json:"nat_port_tiers"`
	NATPortUnitPriceCents int64           `gorm:"column:nat_port_unit_price_cents;type:bigint;not null;default:0" json:"nat_port_unit_price_cents"`

	// IPv6 配置（多前缀增配，JSON数组: [{"prefix_len":128,"min":1,"max":10,"custom_mode":"unlimited","tiers":[],"unit_price_cents":500}, ...]）
	IPv6Enabled  bool            `gorm:"column:ipv6_enabled;type:boolean;not null;default:false" json:"ipv6_enabled"`
	IPv6Configs  json.RawMessage `gorm:"column:ipv6_configs;type:jsonb;not null;default:'[]'" json:"ipv6_configs"`
	IPv6EIPPools json.RawMessage `gorm:"column:ipv6_eip_pools;type:jsonb;not null;default:'{}'" json:"ipv6_eip_pools"`

	// 价格配置
	BasePriceCents     int64         `gorm:"type:bigint;not null;default:0" json:"base_price_cents"`
	MinPaymentPeriod   PaymentPeriod `gorm:"type:varchar(16);not null;default:'monthly'" json:"min_payment_period"`
	TrialEnabled       bool          `gorm:"type:boolean;not null;default:false" json:"trial_enabled"`
	TrialHours         int           `gorm:"type:int;not null;default:0" json:"trial_hours"`
	TrialPriceCents    int64         `gorm:"type:bigint;not null;default:0" json:"trial_price_cents"`
	QuarterlyDiscount  float64       `gorm:"type:decimal(4,2);not null;default:1.00" json:"quarterly_discount"`
	HalfYearlyDiscount float64       `gorm:"type:decimal(4,2);not null;default:1.00" json:"half_yearly_discount"`
	YearlyDiscount     float64       `gorm:"type:decimal(4,2);not null;default:1.00" json:"yearly_discount"`

	CreatedAt time.Time      `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt time.Time      `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联
	Category *ProductCategory `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
}

func (Product) TableName() string {
	return "products"
}
