package models

import "time"

// ImageCache 镜像缓存表，缓存从 streams API 获取的镜像列表
type ImageCache struct {
	ID         int       `gorm:"primaryKey;autoIncrement" json:"id"`
	SourceURL  string    `gorm:"type:varchar(512);not null" json:"source_url"`
	ImageKey   string    `gorm:"type:varchar(255);not null" json:"image_key"`
	Alias      string    `gorm:"type:varchar(255);not null" json:"alias"`
	Name       string    `gorm:"type:varchar(255);not null;default:''" json:"name"`
	Type       string    `gorm:"type:varchar(50);not null" json:"type"`
	Distro     string    `gorm:"type:varchar(100);not null;default:''" json:"distro"`
	Release    string    `gorm:"type:varchar(100);not null;default:''" json:"release"`
	Arch       string    `gorm:"type:varchar(50);not null" json:"arch"`
	Description string   `gorm:"type:text;not null;default:''" json:"description"`
	TotalBytes int64     `gorm:"not null;default:0" json:"total_bytes"`
	CreatedAt  time.Time `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt  time.Time `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
}

func (ImageCache) TableName() string {
	return "image_cache"
}
