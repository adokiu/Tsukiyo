package models

import (
	"time"

	"github.com/google/uuid"
)

// AuditLog 审计日志表
type AuditLog struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID    uint      `gorm:"index" json:"user_id,omitempty"`
	Username  string    `gorm:"type:varchar(64)" json:"username,omitempty"`
	Action    string    `gorm:"type:varchar(64);not null" json:"action"`
	Target    string    `gorm:"type:varchar(64)" json:"target,omitempty"`
	Detail    string    `gorm:"type:text" json:"detail,omitempty"`
	IPAddress string    `gorm:"type:inet" json:"ip_address,omitempty"`
	Success   bool      `gorm:"type:boolean;default:true" json:"success"`
	Error     string    `gorm:"type:text" json:"error,omitempty"`
	CreatedAt time.Time `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}
