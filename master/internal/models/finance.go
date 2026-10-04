package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PaymentChannelStatus 支付渠道状态
type PaymentChannelStatus string

const (
	PaymentChannelStatusEnabled  PaymentChannelStatus = "enabled"
	PaymentChannelStatusDisabled PaymentChannelStatus = "disabled"
)

// PaymentChannel 支付渠道 (单个渠道就是一个支付方式)
type PaymentChannel struct {
	ID                 uint                 `gorm:"primaryKey;autoIncrement" json:"id"`
	Name               string               `gorm:"type:varchar(64);not null" json:"name"`
	Type               string               `gorm:"type:varchar(32);not null" json:"type"`
	Config             string               `gorm:"type:text" json:"config,omitempty"`
	Icon               string               `gorm:"type:varchar(256)" json:"icon,omitempty"`
	Description        string               `gorm:"type:text" json:"description,omitempty"`
	FeeBearer          FeeBearer            `gorm:"type:varchar(16);default:'user'" json:"fee_bearer"`
	FeeType            FeeType              `gorm:"type:varchar(32);default:'fixed'" json:"fee_type"`
	FeeFixedCents      int64                `gorm:"type:bigint;default:0" json:"fee_fixed_cents"`
	FeePercent         float64              `gorm:"type:decimal(8,4);default:0" json:"fee_percent"`
	MinAmount          int64                `gorm:"type:bigint;default:0" json:"min_amount"`
	MaxAmount          int64                `gorm:"type:bigint;default:0" json:"max_amount"`
	SettlementCurrency string               `gorm:"type:varchar(8);default:'CNY'" json:"settlement_currency"`
	ExchangeRateMarkup float64              `gorm:"type:decimal(5,2);default:0" json:"exchange_rate_markup"`
	Status             PaymentChannelStatus `gorm:"type:varchar(16);default:'enabled'" json:"status"`
	SortOrder          int                  `gorm:"type:int;default:0" json:"sort_order"`
	CreatedAt          time.Time            `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt          time.Time            `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
	DeletedAt          gorm.DeletedAt       `gorm:"index" json:"-"`
}

func (PaymentChannel) TableName() string {
	return "payment_channels"
}

// FeeBearer 手续费承担方
type FeeBearer string

const (
	FeeBearerUser     FeeBearer = "user"
	FeeBearerMerchant FeeBearer = "merchant"
)

// FeeType 手续费计算类型
type FeeType string

const (
	FeeTypeFixed            FeeType = "fixed"
	FeeTypePercent          FeeType = "percent"
	FeeTypeFixedPlusPercent FeeType = "fixed_plus_percent"
)

// BillType 账单类型
type BillType string

const (
	BillTypeRecharge     BillType = "recharge"
	BillTypeConsume      BillType = "consume"
	BillTypeRefund       BillType = "refund"
	BillTypeAdjustment   BillType = "adjustment"
	BillTypeSubscription BillType = "subscription"
)

// BillStatus 账单状态
type BillStatus string

const (
	BillStatusPending   BillStatus = "pending"
	BillStatusPaid      BillStatus = "paid"
	BillStatusFailed    BillStatus = "failed"
	BillStatusRefunded  BillStatus = "refunded"
	BillStatusCancelled BillStatus = "cancelled"
)

// Bill 账单
type Bill struct {
	ID                 uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	BillNo             string     `gorm:"type:varchar(64);uniqueIndex;not null" json:"bill_no"`
	UserID             uint       `gorm:"index;not null" json:"user_id"`
	Username           string     `gorm:"type:varchar(64)" json:"username,omitempty"`
	Type               BillType   `gorm:"type:varchar(32);not null" json:"type"`
	Status             BillStatus `gorm:"type:varchar(16);default:'pending'" json:"status"`
	AmountCents        int64      `gorm:"type:bigint;not null" json:"amount_cents"`
	BalanceBefore      int64      `gorm:"type:bigint;default:0" json:"balance_before"`
	BalanceAfter       int64      `gorm:"type:bigint;default:0" json:"balance_after"`
	PaymentChannelID   *uint      `gorm:"index" json:"payment_channel_id,omitempty"`
	PaymentChannelName string     `gorm:"type:varchar(64)" json:"payment_channel_name,omitempty"`
	TransactionNo      string     `gorm:"type:varchar(128)" json:"transaction_no,omitempty"`
	Description        string     `gorm:"type:text" json:"description,omitempty"`
	InstanceID         *uuid.UUID `gorm:"type:uuid;index" json:"instance_id,omitempty"`
	ExpiresAt          *time.Time `gorm:"type:timestamptz" json:"expires_at,omitempty"`
	CreatedAt          time.Time  `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt          time.Time  `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
}

func (Bill) TableName() string {
	return "bills"
}

// WalletTransaction 钱包流水
type WalletTransaction struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      uint      `gorm:"index;not null" json:"user_id"`
	AmountCents int64     `gorm:"type:bigint;not null" json:"amount_cents"`
	Type        string    `gorm:"type:varchar(32);not null" json:"type"`
	BillID      *uint     `gorm:"index" json:"bill_id,omitempty"`
	Description string    `gorm:"type:text" json:"description,omitempty"`
	CreatedAt   time.Time `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
}

func (WalletTransaction) TableName() string {
	return "wallet_transactions"
}
