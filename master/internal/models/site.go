package models

import (
	"time"

	"github.com/google/uuid"
)

// SiteConfig 站点配置表
type SiteConfig struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	SiteName        string    `gorm:"type:varchar(128);not null;default:'Tsukiyo'" json:"site_name"`
	SiteSubtitle    string    `gorm:"type:varchar(255)" json:"site_subtitle,omitempty"`
	SiteDescription string    `gorm:"type:text" json:"site_description,omitempty"`
	SiteURL         string    `gorm:"type:varchar(255)" json:"site_url,omitempty"`
	ContactEmail    string    `gorm:"type:varchar(255)" json:"contact_email,omitempty"`
	IncusRemoteURL  string    `gorm:"type:varchar(512);default:'images:'" json:"incus_remote_url,omitempty"`
	IsInitialized   bool      `gorm:"type:boolean;not null;default:false" json:"is_initialized"`
	CreatedAt       time.Time `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt       time.Time `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
}

func (SiteConfig) TableName() string {
	return "site_configs"
}
