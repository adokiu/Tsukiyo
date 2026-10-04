package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ==================== 购物车 ====================

// CartItem 购物车项
type CartItem struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	ProductID uuid.UUID `gorm:"type:uuid;not null" json:"product_id"`
	Product   *Product  `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	// 配置快照（JSON）：包含 vcpu, memory_mb, disk_mb, data_disk_mb, payment_period, template_id, image_key, node_id, bridge_id, login_method, ssh_password, ssh_public_key, instance_name, trial 等
	Config         json.RawMessage `gorm:"type:jsonb;not null" json:"config"`
	UnitPriceCents int64           `gorm:"type:bigint;not null;default:0" json:"unit_price_cents"`
	Quantity       int             `gorm:"type:int;not null;default:1" json:"quantity"`
	CreatedAt      time.Time       `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt      time.Time       `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
}

func (CartItem) TableName() string {
	return "cart_items"
}

// ==================== 优惠码 ====================

// CouponType 优惠码类型
type CouponType string

const (
	CouponTypeFixed   CouponType = "fixed"   // 固定金额减免
	CouponTypePercent CouponType = "percent" // 百分比折扣
)

// CouponStatus 优惠码状态
type CouponStatus string

const (
	CouponStatusActive   CouponStatus = "active"
	CouponStatusDisabled CouponStatus = "disabled"
	CouponStatusExpired  CouponStatus = "expired"
)

// Coupon 优惠码
type Coupon struct {
	ID   uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Code string     `gorm:"type:varchar(64);uniqueIndex;not null" json:"code"`
	Name string     `gorm:"type:varchar(128);not null" json:"name"`
	Type CouponType `gorm:"type:varchar(16);not null" json:"type"`
	// 折扣值：fixed 时为分（如 500 = 减 5 元），percent 时为百分比（如 10 = 打 9 折）
	Value            int64          `gorm:"type:bigint;not null" json:"value"`
	MinAmountCents   int64          `gorm:"type:bigint;not null;default:0" json:"min_amount_cents"`
	MaxDiscountCents int64          `gorm:"type:bigint;not null;default:0" json:"max_discount_cents"`
	UsageLimit       int            `gorm:"type:int;not null;default:0" json:"usage_limit"`
	UsedCount        int            `gorm:"type:int;not null;default:0" json:"used_count"`
	ValidFrom        *time.Time     `gorm:"type:timestamptz" json:"valid_from,omitempty"`
	ValidTo          *time.Time     `gorm:"type:timestamptz" json:"valid_to,omitempty"`
	Status           CouponStatus   `gorm:"type:varchar(16);not null;default:'active'" json:"status"`
	CreatedAt        time.Time      `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt        time.Time      `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Coupon) TableName() string {
	return "coupons"
}

// ==================== 订单 ====================

// OrderStatus 订单状态
type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"   // 待支付
	OrderStatusPaid      OrderStatus = "paid"      // 已支付，创建实例中
	OrderStatusCompleted OrderStatus = "completed" // 已完成（实例已创建）
	OrderStatusCancelled OrderStatus = "cancelled" // 已取消
	OrderStatusFailed    OrderStatus = "failed"    // 支付失败或创建失败
	OrderStatusRefunded  OrderStatus = "refunded"  // 已退款
)

// Order 订单
type Order struct {
	ID       uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OrderNo  string    `gorm:"type:varchar(64);uniqueIndex;not null" json:"order_no"`
	UserID   uint      `gorm:"index;not null" json:"user_id"`
	Username string    `gorm:"type:varchar(64)" json:"username,omitempty"`
	// 原始总价（分）
	SubtotalCents int64 `gorm:"type:bigint;not null" json:"subtotal_cents"`
	// 优惠金额（分）
	DiscountCents int64 `gorm:"type:bigint;not null;default:0" json:"discount_cents"`
	// 实付金额（分）
	TotalCents       int64          `gorm:"type:bigint;not null" json:"total_cents"`
	CouponID         *uuid.UUID     `gorm:"type:uuid;index" json:"coupon_id,omitempty"`
	CouponCode       string         `gorm:"type:varchar(64)" json:"coupon_code,omitempty"`
	Status           OrderStatus    `gorm:"type:varchar(16);not null;default:'pending'" json:"status"`
	PaymentMethod    string         `gorm:"type:varchar(32)" json:"payment_method,omitempty"`
	PaymentChannelID *uint          `gorm:"index" json:"payment_channel_id,omitempty"`
	BillID           *uint          `gorm:"index" json:"bill_id,omitempty"`
	Description      string         `gorm:"type:text" json:"description,omitempty"`
	ExpiresAt        *time.Time     `gorm:"type:timestamptz" json:"expires_at,omitempty"`
	PaidAt           *time.Time     `gorm:"type:timestamptz" json:"paid_at,omitempty"`
	CreatedAt        time.Time      `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt        time.Time      `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联
	Items []OrderItem `gorm:"foreignKey:OrderID" json:"items,omitempty"`
}

func (Order) TableName() string {
	return "orders"
}

// OrderItem 订单项
type OrderItem struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OrderID   uuid.UUID `gorm:"type:uuid;index;not null" json:"order_id"`
	ProductID uuid.UUID `gorm:"type:uuid;not null" json:"product_id"`
	Product   *Product  `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	// 配置快照（JSON）
	Config         json.RawMessage `gorm:"type:jsonb;not null" json:"config"`
	UnitPriceCents int64           `gorm:"type:bigint;not null" json:"unit_price_cents"`
	Quantity       int             `gorm:"type:int;not null;default:1" json:"quantity"`
	SubtotalCents  int64           `gorm:"type:bigint;not null" json:"subtotal_cents"`
	// 创建实例后的关联
	InstanceID *uuid.UUID `gorm:"type:uuid;index" json:"instance_id,omitempty"`
	CreatedAt  time.Time  `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
}

func (OrderItem) TableName() string {
	return "order_items"
}

// CouponUsage 优惠码使用记录
type CouponUsage struct {
	ID            uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	CouponID      uuid.UUID  `gorm:"type:uuid;index;not null" json:"coupon_id"`
	UserID        uint       `gorm:"index;not null" json:"user_id"`
	OrderID       *uuid.UUID `gorm:"type:uuid;index" json:"order_id,omitempty"`
	DiscountCents int64      `gorm:"type:bigint;not null" json:"discount_cents"`
	CreatedAt     time.Time  `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
}

func (CouponUsage) TableName() string {
	return "coupon_usages"
}
