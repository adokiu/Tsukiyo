package models

import (
	"time"

	"gorm.io/gorm"
)

// UserStatus 用户状态
type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusSuspended UserStatus = "suspended"
	UserStatusDeleted   UserStatus = "deleted"
)

// User 用户表
type User struct {
	ID           uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	Username     string         `gorm:"type:varchar(64);uniqueIndex;not null" json:"username"`
	Email        string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	PasswordHash string         `gorm:"type:varchar(255);not null" json:"-"`
	Status       UserStatus     `gorm:"type:varchar(16);default:'active'" json:"status"`
	LastLoginAt  *time.Time     `gorm:"type:timestamptz" json:"last_login_at,omitempty"`
	LastLoginIP  string         `gorm:"type:varchar(64)" json:"last_login_ip,omitempty"`
	CreatedAt    time.Time      `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}

// IsActive 检查用户是否激活
func (u *User) IsActive() bool {
	return u.Status == UserStatusActive
}


// UserGroup 用户组表
type UserGroup struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"type:varchar(64);uniqueIndex;not null" json:"name"`
	Description string    `gorm:"type:text" json:"description,omitempty"`
	IsBuiltin   bool      `gorm:"type:boolean;default:false" json:"is_builtin"`
	CreatedAt   time.Time `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
	UpdatedAt   time.Time `gorm:"type:timestamptz;not null;default:now()" json:"updated_at"`
}

func (UserGroup) TableName() string {
	return "user_groups"
}

// UserGroupMember 用户与组关联表
type UserGroupMember struct {
	UserID     uint      `gorm:"primaryKey" json:"user_id"`
	GroupID    uint      `gorm:"primaryKey" json:"group_id"`
	AssignedAt time.Time `gorm:"type:timestamptz;not null;default:now()" json:"assigned_at"`
	AssignedBy uint      `json:"assigned_by,omitempty"`
}

func (UserGroupMember) TableName() string {
	return "user_group_members"
}

// Permission 权限定义表
type Permission struct {
	ID          string `gorm:"type:varchar(64);primary_key" json:"id"`
	Name        string `gorm:"type:varchar(128);not null" json:"name"`
	Resource    string `gorm:"type:varchar(32);not null" json:"resource"`
	Action      string `gorm:"type:varchar(32);not null" json:"action"`
	Description string `gorm:"type:text" json:"description,omitempty"`
}

func (Permission) TableName() string {
	return "permissions"
}

// GroupPermission 组权限关联表
type GroupPermission struct {
	GroupID      uint   `gorm:"primaryKey" json:"group_id"`
	PermissionID string `gorm:"type:varchar(64);primaryKey" json:"permission_id"`
	Scope        string `gorm:"type:varchar(16);default:'all'" json:"scope"`
	CreatedAt    time.Time `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`
}

func (GroupPermission) TableName() string {
	return "group_permissions"
}
