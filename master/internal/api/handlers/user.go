package handlers

import (
	"net/http"
	"strconv"

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
	if v := c.Query("page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			page = p
		}
	}
	if v := c.Query("per_page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			pageSize = p
		}
	}
	search := c.Query("search")
	statusFilter := c.Query("filter_status")

	users, total, err := userService.ListUsers(page, pageSize, search, statusFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	// 批量查询用户组
	userIDs := make([]uint, 0, len(users))
	for _, u := range users {
		userIDs = append(userIDs, u.ID)
	}
	groupMap, _ := userService.GetUserGroups(userIDs)

	result := make([]gin.H, 0, len(users))
	for _, u := range users {
		groups := groupMap[u.ID]
		if groups == nil {
			groups = []string{}
		}
		result = append(result, gin.H{
			"id":               u.ID,
			"username":         u.Username,
			"email":            u.Email,
			"status":           u.Status,
			"balance_cents":    u.BalanceCents,
			"phone":            u.Phone,
			"qq":               u.QQ,
			"real_name_status": u.RealNameStatus,
			"real_name":        u.RealName,
			"id_card":          u.IDCard,
			"groups":           groups,
			"created_at":       u.CreatedAt,
			"last_login_at":    u.LastLoginAt,
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
		"id":               user.ID,
		"username":         user.Username,
		"email":            user.Email,
		"status":           user.Status,
		"balance_cents":    user.BalanceCents,
		"phone":            user.Phone,
		"qq":               user.QQ,
		"real_name_status": user.RealNameStatus,
		"real_name":        user.RealName,
		"id_card":          user.IDCard,
		"groups":           groups,
		"created_at":       user.CreatedAt,
		"updated_at":       user.UpdatedAt,
		"last_login_at":    user.LastLoginAt,
		"last_login_ip":    user.LastLoginIP,
	})
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	Email          string `json:"email"`
	Status         string `json:"status"`
	BalanceCents   *int64 `json:"balance_cents"`
	Password       string `json:"password"`
	Phone          string `json:"phone"`
	QQ             string `json:"qq"`
	RealNameStatus string `json:"real_name_status"`
	RealName       string `json:"real_name"`
	IDCard         string `json:"id_card"`
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

	if err := userService.UpdateUser(userID, req.Email, req.Status, req.BalanceCents, req.Password, req.Phone, req.QQ, req.RealNameStatus, req.RealName, req.IDCard); err != nil {
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
