package models

import "time"

// ImageType 镜像类型
type ImageType string

const (
	ImageTypeContainer ImageType = "container"
	ImageTypeVM        ImageType = "vm"
)

// ImageTemplate 镜像模板表
type ImageTemplate struct {
	ID          string    `gorm:"type:varchar(64);primary_key" json:"id"`
	Name        string    `gorm:"type:varchar(128);not null" json:"name"`
	Type        ImageType `gorm:"type:varchar(16);not null" json:"type"`
	Distro      string    `gorm:"type:varchar(32)" json:"distro,omitempty"`
	Release     string    `gorm:"type:varchar(32)" json:"release,omitempty"`
	Arch        string    `gorm:"type:varchar(16);default:'amd64'" json:"arch"`
	URL         string    `gorm:"type:varchar(512)" json:"url,omitempty"`
	Alias       string    `gorm:"type:varchar(128)" json:"alias,omitempty"` // Incus 镜像别名
	Description string    `gorm:"type:text" json:"description,omitempty"`
	Enabled     bool      `gorm:"type:boolean;default:true" json:"enabled"`
	Desktop     string    `gorm:"type:varchar(32)" json:"desktop,omitempty"`
	CreatedAt   time.Time `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt   time.Time `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
}

func (ImageTemplate) TableName() string {
	return "image_templates"
}

// NodeImage 节点已下载镜像关系表
type NodeImage struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	NodeID    string    `gorm:"type:uuid;not null;index:idx_node_image" json:"node_id"`
	ImageID   string    `gorm:"type:varchar(64);not null;index:idx_node_image" json:"image_id"`
	Status    string    `gorm:"type:varchar(16);default:'downloaded'" json:"status"` // downloaded / downloading / error
	UpdatedAt time.Time `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
}

func (NodeImage) TableName() string {
	return "node_images"
}
