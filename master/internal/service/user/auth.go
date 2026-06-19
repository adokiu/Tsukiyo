package user

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"tsukiyo/master/internal/auth"
	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/service"
)

var (
	ErrUserNotFound = service.ErrUserNotFound
)

// AuthService 认证服务
type AuthService struct{}

// NewAuthService 创建认证服务
func NewAuthService() *AuthService {
	return &AuthService{}
}

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
func (s *AuthService) Login(req LoginRequest, clientIP string) (*LoginResponse, error) {
	var user models.User
	if err := db.DB.Where("username = ? OR email = ?", req.Username, req.Username).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &service.ServiceError{Message: "用户名或密码错误"}
		}
		return nil, err
	}

	if !user.IsActive() {
		return nil, &service.ServiceError{Message: "用户已被禁用"}
	}

	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		return nil, &service.ServiceError{Message: "用户名或密码错误"}
	}

	token, err := auth.GenerateToken(user.ID, user.Username)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	db.DB.Model(&user).Updates(map[string]interface{}{
		"last_login_at": now,
		"last_login_ip": clientIP,
	})

	auditLog := models.AuditLog{
		UserID:    user.ID,
		Username:  user.Username,
		Action:    "user:login",
		Target:    "user",
		Detail:    "用户登录",
		IPAddress: clientIP,
		Success:   true,
	}
	db.DB.Create(&auditLog)

	return &LoginResponse{
		Token:     token,
		ExpiresAt: now.Add(24 * time.Hour),
		User: UserInfo{
			ID:       fmt.Sprintf("%d", user.ID),
			Username: user.Username,
			Email:    user.Email,
			Status:   string(user.Status),
		},
	}, nil
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=128"`
}

// Register 用户注册
func (s *AuthService) Register(req RegisterRequest, clientIP string) (*models.User, error) {
	var existingUser models.User
	if err := db.DB.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		return nil, &service.ServiceError{Message: "用户名已存在"}
	}

	if err := db.DB.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		return nil, &service.ServiceError{Message: "邮箱已存在"}
	}

	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	user := models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: passwordHash,
		Status:       models.UserStatusActive,
	}

	if err := db.DB.Create(&user).Error; err != nil {
		return nil, err
	}

	var userGroup models.UserGroup
	if err := db.DB.Where("name = ?", "user").First(&userGroup).Error; err == nil {
		member := models.UserGroupMember{
			UserID:     user.ID,
			GroupID:    userGroup.ID,
			AssignedBy: 0,
		}
		db.DB.Create(&member)
	}

	auditLog := models.AuditLog{
		UserID:    user.ID,
		Username:  user.Username,
		Action:    "user:create",
		Target:    "user",
		Detail:    "用户注册",
		IPAddress: clientIP,
		Success:   true,
	}
	db.DB.Create(&auditLog)

	return &user, nil
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=128"`
}

// ChangePassword 修改密码
func (s *AuthService) ChangePassword(userID uint, req ChangePasswordRequest) error {
	var user models.User
	if err := db.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrUserNotFound
		}
		return err
	}

	if !auth.CheckPassword(req.OldPassword, user.PasswordHash) {
		return &service.ServiceError{Message: "原密码错误"}
	}

	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		return err
	}

	return db.DB.Model(&user).Update("password_hash", newHash).Error
}
