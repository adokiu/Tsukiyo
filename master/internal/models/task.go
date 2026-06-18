package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCanceled  TaskStatus = "canceled"
)

// TaskType 任务类型
type TaskType string

const (
	TaskTypeCreateInstance    TaskType = "create_instance"
	TaskTypeDeleteInstance    TaskType = "delete_instance"
	TaskTypeStartInstance     TaskType = "start_instance"
	TaskTypeStopInstance      TaskType = "stop_instance"
	TaskTypeRestartInstance   TaskType = "restart_instance"
	TaskTypeReinstallInstance TaskType = "reinstall_instance"
	TaskTypeResizeInstance    TaskType = "resize_instance"
	TaskTypeCreateSnapshot    TaskType = "create_snapshot"
	TaskTypeRestoreSnapshot   TaskType = "restore_snapshot"
	TaskTypeDeleteSnapshot    TaskType = "delete_snapshot"
	TaskTypeDownloadImage     TaskType = "download_image"
	TaskTypeApplyNetwork      TaskType = "apply_network"
	TaskTypeApplyFirewall     TaskType = "apply_firewall"
	TaskTypeFormatDisk        TaskType = "format_disk"
	TaskTypeInitStorage       TaskType = "init_storage"
	TaskTypeMigrateInstance   TaskType = "migrate_instance"
	TaskTypeVPCNetwork        TaskType = "vpc_network"
)

// Task 任务队列表
type Task struct {
	ID          uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Type        TaskType       `gorm:"type:varchar(32);not null" json:"type"`
	NodeID      uuid.UUID      `gorm:"type:uuid;not null;index" json:"node_id"`
	InstanceID  *uuid.UUID     `gorm:"type:uuid;index" json:"instance_id,omitempty"`
	UserID      uint           `gorm:"not null;index" json:"user_id"`
	Status      TaskStatus     `gorm:"type:varchar(16);default:'pending'" json:"status"`
	Payload     []byte         `gorm:"type:jsonb" json:"payload,omitempty"`
	Result      []byte         `gorm:"type:jsonb" json:"result,omitempty"`
	Error       string         `gorm:"type:text" json:"error,omitempty"`
	RetryCount  int            `gorm:"type:int;default:0" json:"retry_count"`
	MaxRetries  int            `gorm:"type:int;default:3" json:"max_retries"`
	StartedAt   *time.Time     `gorm:"type:timestamptz" json:"started_at,omitempty"`
	CompletedAt *time.Time     `gorm:"type:timestamptz" json:"completed_at,omitempty"`
	CreatedAt   time.Time      `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联
	Node     Node      `gorm:"foreignKey:NodeID" json:"node,omitempty"`
	Instance *Instance `gorm:"foreignKey:InstanceID" json:"instance,omitempty"`
}

func (Task) TableName() string {
	return "tasks"
}

// IsPending 检查任务是否待处理
func (t *Task) IsPending() bool {
	return t.Status == TaskStatusPending
}

// CanRetry 检查任务是否可以重试
func (t *Task) CanRetry() bool {
	return t.RetryCount < t.MaxRetries
}
