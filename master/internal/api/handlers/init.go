package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"tsukiyo/master/internal/auth"
	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// InitStatusResponse 初始化状态响应
type InitStatusResponse struct {
	Initialized bool   `json:"initialized"`
	SiteName    string `json:"site_name,omitempty"`
}

// InitSetupRequest 初始化设置请求
type InitSetupRequest struct {
	SiteName        string `json:"site_name" binding:"required,max=128"`
	SiteSubtitle    string `json:"site_subtitle,omitempty"`
	SiteDescription string `json:"site_description,omitempty"`
	SiteURL         string `json:"site_url,omitempty"`
	ContactEmail    string `json:"contact_email,omitempty"`
	IncusRemoteURL  string `json:"incus_remote_url,omitempty"`
	AdminUsername   string `json:"admin_username" binding:"required,min=3,max=32"`
	AdminEmail      string `json:"admin_email" binding:"required,email"`
	AdminPassword   string `json:"admin_password" binding:"required,min=6,max=128"`
}

// GetInitStatus 获取初始化状态
func GetInitStatus(c *gin.Context) {
	var count int64
	db.DB.Model(&models.User{}).Count(&count)

	if count > 0 {
		var site models.SiteConfig
		db.DB.First(&site)
		c.JSON(http.StatusOK, InitStatusResponse{
			Initialized: true,
			SiteName:    site.SiteName,
		})
		return
	}

	c.JSON(http.StatusOK, InitStatusResponse{
		Initialized: false,
	})
}

// InitSetup 执行初始化
func InitSetup(c *gin.Context) {
	var count int64
	db.DB.Model(&models.User{}).Count(&count)
	if count > 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "系统已初始化，无法重复设置"})
		return
	}

	var req InitSetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	tx := db.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 创建站点配置
	site := models.SiteConfig{
		SiteName:        req.SiteName,
		SiteSubtitle:    req.SiteSubtitle,
		SiteDescription: req.SiteDescription,
		SiteURL:         req.SiteURL,
		ContactEmail:    req.ContactEmail,
		IncusRemoteURL:  req.IncusRemoteURL,
		IsInitialized:   true,
	}
	if err := tx.Create(&site).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建站点配置失败"})
		return
	}

	// 创建管理员用户
	passwordHash, err := auth.HashPassword(req.AdminPassword)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码哈希失败"})
		return
	}

	admin := models.User{
		Username:     req.AdminUsername,
		Email:        req.AdminEmail,
		PasswordHash: passwordHash,
		Status:       models.UserStatusActive,
	}
	if err := tx.Create(&admin).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建管理员失败"})
		return
	}

	// 查找已有的 admin 权限组（由数据库迁移创建）
	var adminGroup models.UserGroup
	if err := tx.Where("name = ?", "admin").First(&adminGroup).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "admin 权限组不存在，请检查数据库迁移"})
		return
	}

	// 将管理员加入权限组（跳过已存在的）
	association := models.UserGroupMember{
		UserID:  admin.ID,
		GroupID: adminGroup.ID,
	}
	if err := tx.Where("user_id = ? AND group_id = ?", admin.ID, adminGroup.ID).FirstOrCreate(&association).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "关联权限组失败"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "事务提交失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "初始化成功",
		"site_name": site.SiteName,
		"admin_id":  admin.ID,
	})
}
