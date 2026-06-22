package models

import (
	"time"

	"github.com/google/uuid"
)

// NodeImage 节点已安装镜像缓存表（agent上报，master仅缓存）
type NodeImage struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	NodeID       string    `gorm:"type:uuid;not null;index:idx_node_image" json:"node_id"`
	Fingerprint  string    `gorm:"type:varchar(64);not null;index:idx_node_image" json:"fingerprint"`
	Alias        string    `gorm:"type:varchar(255);not null" json:"alias"`
	ImageType    string    `gorm:"type:varchar(20);not null" json:"image_type"`
	Architecture string    `gorm:"type:varchar(50);not null;default:''" json:"architecture"`
	SizeBytes    int64     `gorm:"not null;default:0" json:"size_bytes"`
	Description  string    `gorm:"type:text;not null;default:''" json:"description"`
	UploadDate   string    `gorm:"type:varchar(50);not null;default:''" json:"upload_date"`
	ImageSource  string    `gorm:"type:varchar(50);not null;default:'manual'" json:"image_source"`
	Status       string    `gorm:"type:varchar(16);default:'downloaded'" json:"status"`
	UpdatedAt    time.Time `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
}

func (NodeImage) TableName() string {
	return "node_images"
}

// NodeImageCategory 节点镜像分类表（按节点+类型独立维护）
type NodeImageCategory struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	NodeID    string    `gorm:"type:uuid;not null;index:idx_node_image_category" json:"node_id"`
	Name      string    `gorm:"type:varchar(100);not null" json:"name"`
	ImageType string    `gorm:"type:varchar(20);not null" json:"image_type"`
	SortOrder int       `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt time.Time `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
}

func (NodeImageCategory) TableName() string {
	return "node_image_categories"
}

// NodeImageAlias 镜像别名映射表（用户自定义分类和显示名）
type NodeImageAlias struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	NodeID      string     `gorm:"type:uuid;not null;index:idx_node_image_alias" json:"node_id"`
	Fingerprint string     `gorm:"type:varchar(64);not null;index:idx_node_image_alias" json:"fingerprint"`
	ImageType   string     `gorm:"type:varchar(20);not null" json:"image_type"`
	CategoryID  *uuid.UUID `gorm:"type:uuid" json:"category_id"`
	DisplayName string     `gorm:"type:varchar(200);not null;default:''" json:"display_name"`
	InstallSSH  bool       `gorm:"not null;default:false" json:"install_ssh"`
	CreatedAt   time.Time  `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
}

func (NodeImageAlias) TableName() string {
	return "node_image_aliases"
}
