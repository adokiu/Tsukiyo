package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"tsukiyo/master/internal/service"
	"tsukiyo/master/internal/service/user"
)

var userService *user.UserService

// InitUserService 初始化用户服务
func InitUserService(svc *user.UserService) {
	userService = svc
}

// ListUsers 获取用户列表
func ListUsers(c *gin.Context) {
	page := 1
	pageSize := 20

	users, total, err := userService.ListUsers(page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	result := make([]gin.H, 0, len(users))
	for _, user := range users {
		result = append(result, gin.H{
			"id":         user.ID,
			"username":   user.Username,
			"email":      user.Email,
			"status":     user.Status,
			"created_at": user.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      result,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetUser 获取用户详情
func GetUser(c *gin.Context) {
	userID, err := user.ParseUserID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户 ID"})
		return
	}

	user, groups, err := userService.GetUser(userID)
	if err != nil {
		if err == service.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":         user.ID,
		"username":   user.Username,
		"email":      user.Email,
		"status":     user.Status,
		"groups":     groups,
		"created_at": user.CreatedAt,
		"updated_at": user.UpdatedAt,
	})
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	Email  string `json:"email"`
	Status string `json:"status"`
}

// UpdateUser 更新用户
func UpdateUser(c *gin.Context) {
	userID, err := user.ParseUserID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户 ID"})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if err := userService.UpdateUser(userID, req.Email, req.Status); err != nil {
		if err == service.ErrNoValidUpdateFields {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无有效更新字段"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// DeleteUser 删除用户
func DeleteUser(c *gin.Context) {
	userID, err := user.ParseUserID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户 ID"})
		return
	}

	if err := userService.DeleteUser(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}
