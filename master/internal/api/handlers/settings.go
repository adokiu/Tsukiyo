package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// GetSiteConfig 获取站点配置
func GetSiteConfig(c *gin.Context) {
	var site models.SiteConfig
	if err := db.DB.First(&site).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "站点配置不存在"})
		return
	}

	c.JSON(http.StatusOK, site)
}

// UpdateSiteConfig 更新站点配置
type UpdateSiteConfigRequest struct {
	SiteName        string `json:"site_name,omitempty" binding:"max=128"`
	SiteSubtitle    string `json:"site_subtitle,omitempty"`
	SiteDescription string `json:"site_description,omitempty"`
	SiteURL         string `json:"site_url,omitempty"`
	ContactEmail    string `json:"contact_email,omitempty"`
	IncusRemoteURL  string `json:"incus_remote_url,omitempty"`
}

func UpdateSiteConfig(c *gin.Context) {
	var req UpdateSiteConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	var site models.SiteConfig
	if err := db.DB.First(&site).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "站点配置不存在"})
		return
	}

	// 更新字段
	if req.SiteName != "" {
		site.SiteName = req.SiteName
	}
	if req.SiteSubtitle != "" {
		site.SiteSubtitle = req.SiteSubtitle
	}
	if req.SiteDescription != "" {
		site.SiteDescription = req.SiteDescription
	}
	if req.SiteURL != "" {
		site.SiteURL = req.SiteURL
	}
	if req.ContactEmail != "" {
		site.ContactEmail = req.ContactEmail
	}
	if req.IncusRemoteURL != "" {
		site.IncusRemoteURL = req.IncusRemoteURL
	}

	if err := db.DB.Save(&site).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新站点配置失败"})
		return
	}

	c.JSON(http.StatusOK, site)
}
