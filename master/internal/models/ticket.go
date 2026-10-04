package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TicketStatus 工单状态
type TicketStatus string

const (
	TicketStatusOpen       TicketStatus = "open"       // 待处理
	TicketStatusInProgress TicketStatus = "in_progress" // 处理中
	TicketStatusWaiting    TicketStatus = "waiting"    // 等待用户回复
	TicketStatusResolved   TicketStatus = "resolved"   // 已解决
	TicketStatusClosed     TicketStatus = "closed"     // 已关闭
)

// TicketType 工单类型
type TicketType string

const (
	TicketTypeFault      TicketType = "fault"      // 故障报告
	TicketTypeFinance    TicketType = "finance"    // 财务问题
	TicketTypeSales      TicketType = "sales"      // 售前咨询
	TicketTypeTech       TicketType = "tech"       // 技术支持
	TicketTypeComplaint  TicketType = "complaint"  // 投诉建议
	TicketTypeOther      TicketType = "other"      // 其他
)

// TicketPriority 工单优先级
type TicketPriority string

const (
	TicketPriorityLow    TicketPriority = "low"    // 低
	TicketPriorityNormal TicketPriority = "normal" // 普通
	TicketPriorityHigh   TicketPriority = "high"   // 高
	TicketPriorityUrgent TicketPriority = "urgent" // 紧急
)

// Ticket 工单
type Ticket struct {
	ID          uuid.UUID       `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TicketNo    string          `gorm:"type:varchar(32);uniqueIndex;not null" json:"ticket_no"`
	UserID      uint            `gorm:"index;not null" json:"user_id"`
	User        User            `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Subject     string          `gorm:"type:varchar(256);not null" json:"subject"`
	Content     string          `gorm:"type:text;not null" json:"content"`
	Type        TicketType      `gorm:"type:varchar(32);not null;default:'other'" json:"type"`
	Priority    TicketPriority  `gorm:"type:varchar(16);not null;default:'normal'" json:"priority"`
	Status      TicketStatus    `gorm:"type:varchar(16);not null;default:'open'" json:"status"`
	ProductID   *uuid.UUID      `gorm:"type:uuid;index" json:"product_id,omitempty"`
	Product     *Product        `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	InstanceID  *uuid.UUID      `gorm:"type:uuid;index" json:"instance_id,omitempty"`
	Instance    *Instance       `gorm:"foreignKey:InstanceID" json:"instance,omitempty"`
	AssigneeID  *uint           `gorm:"index" json:"assignee_id,omitempty"`
	Assignee    *User           `gorm:"foreignKey:AssigneeID" json:"assignee,omitempty"`
	Replies     []TicketReply   `gorm:"foreignKey:TicketID" json:"replies,omitempty"`
	ClosedAt    *time.Time      `gorm:"type:timestamptz" json:"closed_at,omitempty"`
	CreatedAt   time.Time       `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt   time.Time       `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
	DeletedAt   gorm.DeletedAt  `gorm:"index" json:"-"`
}

func (Ticket) TableName() string {
	return "tickets"
}

// TicketReply 工单回复
type TicketReply struct {
	ID        uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TicketID  uuid.UUID      `gorm:"type:uuid;index;not null" json:"ticket_id"`
	Ticket    Ticket         `gorm:"foreignKey:TicketID" json:"-"`
	UserID    uint           `gorm:"not null" json:"user_id"`
	User      User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Content   string         `gorm:"type:text;not null" json:"content"`
	IsStaff   bool           `gorm:"type:boolean;not null;default:false" json:"is_staff"`
	CreatedAt time.Time      `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt time.Time      `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (TicketReply) TableName() string {
	return "ticket_replies"
}
