package models

import "time"

// NodeImage 节点已下载镜像关系表
type NodeImage struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	NodeID    string    `gorm:"type:uuid;not null;index:idx_node_image" json:"node_id"`
	ImageID   string    `gorm:"type:varchar(255);not null;index:idx_node_image" json:"image_id"`
	Status    string    `gorm:"type:varchar(16);default:'downloaded'" json:"status"` // downloaded / downloading / error
	UpdatedAt time.Time `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
}

func (NodeImage) TableName() string {
	return "node_images"
}
