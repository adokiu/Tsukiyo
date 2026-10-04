package user

import (
	"crypto/rand"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"tsukiyo/master/internal/auth"
	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/service"
	mail "tsukiyo/master/internal/service/mail"
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
	ID           string `json:"id"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	Status       string `json:"status"`
	BalanceCents int64  `json:"balance_cents"`
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
		if user.Status == models.UserStatusPending {
			return nil, &service.ServiceError{Message: "邮箱未验证，请先验证邮箱"}
		}
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
			ID:           fmt.Sprintf("%d", user.ID),
			Username:     user.Username,
			Email:        user.Email,
			Status:       string(user.Status),
			BalanceCents: user.BalanceCents,
		},
	}, nil
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username     string `json:"username" binding:"required,min=3,max=64"`
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password" binding:"required,min=8,max=128"`
	Code         string `json:"code,omitempty"`
	Phone        string `json:"phone"`
	QQ           string `json:"qq"`
	BalanceCents *int64 `json:"balance_cents"`
}

// Register 用户注册
func (s *AuthService) Register(req RegisterRequest, clientIP string) (*models.User, error) {
	// 检查是否允许注册
	var site models.SiteConfig
	if err := db.DB.First(&site).Error; err == nil {
		if !site.AllowRegistration {
			return nil, &service.ServiceError{Message: "管理员已关闭注册"}
		}
	}

	var existingUser models.User
	if err := db.DB.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		return nil, &service.ServiceError{Message: "用户名已存在"}
	}

	if err := db.DB.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		return nil, &service.ServiceError{Message: "邮箱已存在"}
	}

	// 判断是否需要邮箱验证码：站点配置强制邮箱验证 或 SMTP配置启用了邮箱验证
	var smtpCfg models.SMTPConfig
	verifyEmailEnabled := false
	if err := db.DB.First(&smtpCfg).Error; err == nil && smtpCfg.Enabled && smtpCfg.VerifyEmail {
		verifyEmailEnabled = true
	}
	if site.ForceEmailVerify {
		verifyEmailEnabled = true
	}

	if verifyEmailEnabled {
		var verification models.EmailVerification
		if err := db.DB.Where("email = ? AND code = ? AND used = false", req.Email, req.Code).First(&verification).Error; err != nil {
			return nil, &service.ServiceError{Message: "验证码错误"}
		}
		if verification.ExpiresAt.Before(time.Now()) {
			return nil, &service.ServiceError{Message: "验证码已过期，请重新获取"}
		}
		// 标记验证码已使用
		db.DB.Model(&verification).Update("used", true)
	}

	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	user := models.User{
		Username:      req.Username,
		Email:         req.Email,
		PasswordHash:  passwordHash,
		Status:        models.UserStatusActive,
		EmailVerified: verifyEmailEnabled,
		Phone:         req.Phone,
		QQ:            req.QQ,
	}
	if req.BalanceCents != nil {
		user.BalanceCents = *req.BalanceCents
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

// SendRegisterCode 发送注册验证码
func (s *AuthService) SendRegisterCode(email string) error {
	// 检查SMTP是否启用
	var smtpCfg models.SMTPConfig
	if err := db.DB.First(&smtpCfg).Error; err != nil {
		return &service.ServiceError{Message: "邮件服务未配置"}
	}
	if !smtpCfg.Enabled {
		return &service.ServiceError{Message: "邮件服务未启用"}
	}

	// 检查是否启用了邮箱验证（SMTP配置或站点配置）
	var site models.SiteConfig
	if err := db.DB.First(&site).Error; err == nil {
		if !smtpCfg.VerifyEmail && !site.ForceEmailVerify {
			return &service.ServiceError{Message: "邮箱验证未启用"}
		}
	} else if !smtpCfg.VerifyEmail {
		return &service.ServiceError{Message: "邮箱验证未启用"}
	}

	// 检查邮箱是否已注册
	var existingUser models.User
	if err := db.DB.Where("email = ?", email).First(&existingUser).Error; err == nil {
		return &service.ServiceError{Message: "邮箱已注册"}
	}

	// 检查发送频率限制（60秒内不能重复发送）
	var lastVerification models.EmailVerification
	if err := db.DB.Where("email = ? AND created_at > ?", email, time.Now().Add(-60*time.Second)).First(&lastVerification).Error; err == nil {
		return &service.ServiceError{Message: "验证码发送过于频繁，请稍后再试"}
	}

	// 生成6位验证码
	code := generateVerifyCode()
	verification := models.EmailVerification{
		Email:     email,
		Code:      code,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	if err := db.DB.Create(&verification).Error; err != nil {
		return fmt.Errorf("创建验证码失败: %w", err)
	}

	// 获取站点名称
	siteName := "Tsukiyo"
	if site.SiteName != "" {
		siteName = site.SiteName
	}

	// 发送验证码邮件
	mailSvc := mail.NewMailServiceWithConfig(&smtpCfg)
	subject := "注册验证码 - " + siteName
	body := fmt.Sprintf(`
<div style="max-width:600px;margin:0 auto;font-family:sans-serif;">
  <h2>注册验证码</h2>
  <p>您好，您的注册验证码为：</p>
  <p style="font-size:32px;font-weight:bold;letter-spacing:8px;color:#2563eb;">%s</p>
  <p>验证码有效期为10分钟，请尽快使用。</p>
  <p>如果您没有注册账户，请忽略此邮件。</p>
</div>
`, code)

	if err := mailSvc.SendMailDirect([]string{email}, subject, body); err != nil {
		zap.L().Error("发送验证码邮件失败", zap.String("email", email), zap.Error(err))
		return &service.ServiceError{Message: "发送验证码失败，请稍后重试"}
	}

	zap.L().Info("注册验证码已发送", zap.String("email", email))
	return nil
}

// generateVerifyCode 生成6位数字验证码
func generateVerifyCode() string {
	b := make([]byte, 3)
	rand.Read(b)
	num := int(b[0])<<16 | int(b[1])<<8 | int(b[2])
	return fmt.Sprintf("%06d", num%1000000)
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
