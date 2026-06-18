package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"tsukiyo/master/internal/auth"
	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      UserInfo  `json:"user"`
}

// UserInfo 用户信息
type UserInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Status   string `json:"status"`
}

// Login 用户登录
func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	var user models.User
	if err := db.DB.Where("username = ? OR email = ?", req.Username, req.Username).First(&user).Error; err != nil {
		zap.L().Warn("登录失败: 用户不存在", zap.String("username", req.Username))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	if !user.IsActive() {
		c.JSON(http.StatusForbidden, gin.H{"error": "用户已被禁用"})
		return
	}

	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		zap.L().Warn("登录失败: 密码错误", zap.String("username", req.Username))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	// 生成 JWT
	token, err := auth.GenerateToken(user.ID, user.Username)
	if err != nil {
		zap.L().Error("生成 Token 失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "登录失败"})
		return
	}

	// 更新最后登录时间
	now := time.Now()
	db.DB.Model(&user).Updates(map[string]interface{}{
		"last_login_at": now,
		"last_login_ip": c.ClientIP(),
	})

	// 写审计日志
	auditLog := models.AuditLog{
		UserID:    user.ID,
		Username:  user.Username,
		Action:    "user:login",
		Target:    "user",
		Detail:    "用户登录",
		IPAddress: c.ClientIP(),
		Success:   true,
	}
	db.DB.Create(&auditLog)

	zap.L().Info("用户登录成功", zap.Uint("user_id", user.ID), zap.String("username", user.Username))

	c.JSON(http.StatusOK, LoginResponse{
		Token:     token,
		ExpiresAt: now.Add(24 * time.Hour),
		User: UserInfo{
			ID:       fmt.Sprintf("%d", user.ID),
			Username: user.Username,
			Email:    user.Email,
			Status:   string(user.Status),
		},
	})
}

// Logout 用户登出
func Logout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusOK, gin.H{"message": "登出成功"})
		return
	}

	// 解析 Token 获取过期时间
	// 简单处理：直接返回成功，Token 自然过期即可
	// 生产环境应加入黑名单
	c.JSON(http.StatusOK, gin.H{"message": "登出成功"})
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=128"`
}

// Register 用户注册 (仅管理员可用)
func Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 检查用户名是否已存在
	var existingUser models.User
	if err := db.DB.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "用户名已存在"})
		return
	}

	// 检查邮箱是否已存在
	if err := db.DB.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "邮箱已存在"})
		return
	}

	// 哈希密码
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		zap.L().Error("密码哈希失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "注册失败"})
		return
	}

	// 创建用户
	user := models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: passwordHash,
		Status:       models.UserStatusActive,
	}

	if err := db.DB.Create(&user).Error; err != nil {
		zap.L().Error("创建用户失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "注册失败"})
		return
	}

	// 分配默认 user 组
	var userGroup models.UserGroup
	if err := db.DB.Where("name = ?", "user").First(&userGroup).Error; err == nil {
		member := models.UserGroupMember{
			UserID:     user.ID,
			GroupID:    userGroup.ID,
			AssignedBy: 0,
		}
		db.DB.Create(&member)
	}

	// 写审计日志
	auditLog := models.AuditLog{
		UserID:    user.ID,
		Username:  user.Username,
		Action:    "user:create",
		Target:    "user",
		Detail:    "用户注册",
		IPAddress: c.ClientIP(),
		Success:   true,
	}
	db.DB.Create(&auditLog)

	zap.L().Info("用户注册成功", zap.Uint("user_id", user.ID), zap.String("username", user.Username))

	c.JSON(http.StatusCreated, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"email":    user.Email,
		"status":   user.Status,
	})
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=128"`
}

// ChangePassword 修改密码
func ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	userID := c.GetUint("user_id")

	var user models.User
	if err := db.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	if !auth.CheckPassword(req.OldPassword, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "原密码错误"})
		return
	}

	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "修改密码失败"})
		return
	}

	db.DB.Model(&user).Update("password_hash", newHash)

	c.JSON(http.StatusOK, gin.H{"message": "密码修改成功"})
}
