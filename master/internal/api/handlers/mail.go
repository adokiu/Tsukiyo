package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	mailSvc "tsukiyo/master/internal/service/mail"
)

var mailService *mailSvc.MailService

// InitMailService 初始化邮件服务
func InitMailService(svc *mailSvc.MailService) {
	mailService = svc
}

// GetSMTPConfig 获取SMTP配置
func GetSMTPConfig(c *gin.Context) {
	var cfg models.SMTPConfig
	if err := db.DB.First(&cfg).Error; err != nil {
		// 配置不存在，返回空配置
		c.JSON(http.StatusOK, gin.H{
			"host":         "",
			"port":         587,
			"username":     "",
			"from_name":    "Tsukiyo",
			"from_email":   "",
			"encryption":   "starttls",
			"enabled":      false,
			"verify_email": false,
		})
		return
	}
	// 不返回密码
	cfg.Password = ""
	c.JSON(http.StatusOK, cfg)
}

// UpdateSMTPConfigRequest 更新SMTP配置请求
type UpdateSMTPConfigRequest struct {
	Host        string `json:"host" binding:"required"`
	Port        int    `json:"port" binding:"required"`
	Username    string `json:"username" binding:"required"`
	Password    string `json:"password"`
	FromName    string `json:"from_name"`
	FromEmail   string `json:"from_email" binding:"required"`
	Encryption  string `json:"encryption" binding:"required"`
	Enabled     bool   `json:"enabled"`
	VerifyEmail bool   `json:"verify_email"`
}

// UpdateSMTPConfig 更新SMTP配置
func UpdateSMTPConfig(c *gin.Context) {
	var req UpdateSMTPConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if req.Encryption != "none" && req.Encryption != "starttls" && req.Encryption != "ssl" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "加密方式无效，可选: none / starttls / ssl"})
		return
	}

	var cfg models.SMTPConfig
	isNew := false
	if err := db.DB.First(&cfg).Error; err != nil {
		isNew = true
		cfg = models.SMTPConfig{
			Host:        req.Host,
			Port:        req.Port,
			Username:    req.Username,
			FromName:    req.FromName,
			FromEmail:   req.FromEmail,
			Encryption:  req.Encryption,
			Enabled:     req.Enabled,
			VerifyEmail: req.VerifyEmail,
		}
	} else {
		cfg.Host = req.Host
		cfg.Port = req.Port
		cfg.Username = req.Username
		if req.FromName != "" {
			cfg.FromName = req.FromName
		}
		cfg.FromEmail = req.FromEmail
		cfg.Encryption = req.Encryption
		cfg.Enabled = req.Enabled
		cfg.VerifyEmail = req.VerifyEmail
	}

	// 密码非空时才更新
	if req.Password != "" {
		cfg.Password = req.Password
	}

	if isNew {
		if err := db.DB.Create(&cfg).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建SMTP配置失败"})
			return
		}
	} else {
		if err := db.DB.Save(&cfg).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新SMTP配置失败"})
			return
		}
	}

	cfg.Password = ""
	c.JSON(http.StatusOK, cfg)
}

// TestSMTPRequest 测试SMTP请求
type TestSMTPRequest struct {
	To string `json:"to" binding:"required,email"`
}

// TestSMTP 测试SMTP配置
func TestSMTP(c *gin.Context) {
	var req TestSMTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if mailService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "邮件服务未初始化"})
		return
	}

	if err := mailService.SendTestMail(req.To); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "发送测试邮件失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "测试邮件已发送"})
}
