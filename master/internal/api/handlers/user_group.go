package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// UserGroupInfo 用户组信息
type UserGroupInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	IsBuiltin   bool      `json:"is_builtin"`
	UserCount   int64     `json:"user_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// ListUserGroups 获取用户组列表
func ListUserGroups(c *gin.Context) {
	var groups []models.UserGroup
	if err := db.DB.Order("is_builtin DESC, created_at DESC").Find(&groups).Error; err != nil {
		zap.L().Error("查询用户组失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	result := make([]UserGroupInfo, 0, len(groups))
	for _, g := range groups {
		var count int64
		db.DB.Model(&models.UserGroupMember{}).Where("group_id = ?", g.ID).Count(&count)

		result = append(result, UserGroupInfo{
			ID:          strconv.FormatUint(uint64(g.ID), 10),
			Name:        g.Name,
			Description: g.Description,
			IsBuiltin:   g.IsBuiltin,
			UserCount:   count,
			CreatedAt:   g.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  result,
		"total": len(result),
	})
}

// CreateUserGroupRequest 创建用户组请求
type CreateUserGroupRequest struct {
	Name        string   `json:"name" binding:"required,max=64"`
	Description string   `json:"description,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

// CreateUserGroup 创建用户组
func CreateUserGroup(c *gin.Context) {
	var req CreateUserGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 检查组名是否已存在
	var existing models.UserGroup
	if err := db.DB.Where("name = ?", req.Name).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "用户组名称已存在"})
		return
	}

	group := models.UserGroup{
		Name:        req.Name,
		Description: req.Description,
		IsBuiltin:   false,
	}

	if err := db.DB.Create(&group).Error; err != nil {
		zap.L().Error("创建用户组失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户组失败"})
		return
	}

	// 分配权限
	if len(req.Permissions) > 0 {
		for _, permID := range req.Permissions {
			gp := models.GroupPermission{
				GroupID:      group.ID,
				PermissionID: permID,
				Scope:        "all",
			}
			db.DB.Create(&gp)
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":          group.ID,
		"name":        group.Name,
		"description": group.Description,
	})
}

// UpdateUserGroupRequest 更新用户组请求
type UpdateUserGroupRequest struct {
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

// UpdateUserGroup 更新用户组
func UpdateUserGroup(c *gin.Context) {
	groupID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的组 ID"})
		return
	}
	gid := uint(groupID)

	var req UpdateUserGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	var group models.UserGroup
	if err := db.DB.Where("id = ?", gid).First(&group).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户组不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	if group.IsBuiltin {
		c.JSON(http.StatusForbidden, gin.H{"error": "内置用户组不允许修改"})
		return
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}

	if len(updates) > 0 {
		if err := db.DB.Model(&group).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
			return
		}
	}

	// 更新权限
	if req.Permissions != nil {
		// 删除旧权限
		db.DB.Where("group_id = ?", gid).Delete(&models.GroupPermission{})
		// 添加新权限
		for _, permID := range req.Permissions {
			gp := models.GroupPermission{
				GroupID:      gid,
				PermissionID: permID,
				Scope:        "all",
			}
			db.DB.Create(&gp)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// DeleteUserGroup 删除用户组
func DeleteUserGroup(c *gin.Context) {
	groupID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的组 ID"})
		return
	}
	gid := uint(groupID)

	var group models.UserGroup
	if err := db.DB.Where("id = ?", gid).First(&group).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户组不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	if group.IsBuiltin {
		c.JSON(http.StatusForbidden, gin.H{"error": "内置用户组不允许删除"})
		return
	}

	// 检查是否有用户属于该组
	var count int64
	db.DB.Model(&models.UserGroupMember{}).Where("group_id = ?", gid).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "该组下存在用户，无法删除"})
		return
	}

	// 删除组权限
	db.DB.Where("group_id = ?", gid).Delete(&models.GroupPermission{})
	// 删除组
	db.DB.Delete(&group)

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}
