package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"tsukiyo/master/internal/service"
	"tsukiyo/master/internal/service/user"
)

var authService *user.AuthService

// InitAuthService 初始化认证服务
func InitAuthService(svc *user.AuthService) {
	authService = svc
}

// LoginRequest 登录请求
type LoginRequest = user.LoginRequest

// LoginResponse 登录响应
type LoginResponse = user.LoginResponse

// RegisterRequest 注册请求
type RegisterRequest = user.RegisterRequest

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest = user.ChangePasswordRequest

// Login 用户登录
func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	resp, err := authService.Login(req, c.ClientIP())
	if err != nil {
		if serviceErr, ok := err.(*service.ServiceError); ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": serviceErr.Message})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "登录失败"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Logout 用户登出
func Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "登出成功"})
}

// Register 用户注册
func Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	user, err := authService.Register(req, c.ClientIP())
	if err != nil {
		if serviceErr, ok := err.(*service.ServiceError); ok {
			c.JSON(http.StatusConflict, gin.H{"error": serviceErr.Message})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "注册失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"email":    user.Email,
		"status":   user.Status,
	})
}

// ChangePassword 修改密码
func ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	userID := c.GetUint("user_id")
	if err := authService.ChangePassword(userID, req); err != nil {
		if err == service.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
			return
		}
		if serviceErr, ok := err.(*service.ServiceError); ok && serviceErr.Message == "原密码错误" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "原密码错误"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "修改密码失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码修改成功"})
}
