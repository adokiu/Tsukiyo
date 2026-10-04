package models

import (
	"time"

	"github.com/google/uuid"
)

// SMTPConfig SMTP推送配置表
type SMTPConfig struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Host        string    `gorm:"type:varchar(255);not null" json:"host"`
	Port        int       `gorm:"type:int;not null;default:587" json:"port"`
	Username    string    `gorm:"type:varchar(255);not null" json:"username"`
	Password    string    `gorm:"type:varchar(255);not null" json:"password,omitempty"`
	FromName    string    `gorm:"type:varchar(128);not null;default:'Tsukiyo'" json:"from_name"`
	FromEmail   string    `gorm:"type:varchar(255);not null" json:"from_email"`
	Encryption  string    `gorm:"type:varchar(16);not null;default:'starttls'" json:"encryption"` // none / starttls / ssl
	Enabled     bool      `gorm:"type:boolean;not null;default:false" json:"enabled"`
	VerifyEmail bool      `gorm:"type:boolean;not null;default:false" json:"verify_email"` // 是否启用注册邮箱验证
	CreatedAt   time.Time `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt   time.Time `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
}

func (SMTPConfig) TableName() string {
	return "smtp_configs"
}

// EmailVerification 邮箱验证码记录表
type EmailVerification struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Email     string    `gorm:"type:varchar(255);not null;index" json:"email"`
	Code      string    `gorm:"type:varchar(6);not null;index" json:"-"`
	Used      bool      `gorm:"type:boolean;not null;default:false" json:"used"`
	ExpiresAt time.Time `gorm:"type:timestamptz;not null" json:"expires_at"`
	CreatedAt time.Time `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
}

func (EmailVerification) TableName() string {
	return "email_verifications"
}
