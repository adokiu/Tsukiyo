package handlers

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/service/theme"
)

var themeService *theme.ThemeService

// InitThemeService 初始化主题服务
func InitThemeService(svc *theme.ThemeService) {
	themeService = svc
}

// GetPublicInfo 获取公开站点信息（无需认证）
// 返回站点名称、描述、主题设置等，供用户前台初始化使用
func GetPublicInfo(c *gin.Context) {
	var site models.SiteConfig
	if err := db.DB.First(&site).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"site_name":        "Tsukiyo",
			"site_subtitle":    "",
			"site_description": "",
			"theme":            "default",
			"theme_settings":   gin.H{},
			"enable_register":  false,
		})
		return
	}

	// 获取默认主题配置 JSON
	var defaultThemeJSON []byte
	if site.Theme == "default" {
		defaultThemeJSON, _ = DefaultThemeFS.ReadFile("defaultTheme/tsukiyo-theme.json")
	} else {
		// 非默认主题从文件系统读取
		import_path := filepath.Join(theme.ThemeDir, site.Theme, "tsukiyo-theme.json")
		defaultThemeJSON, _ = os.ReadFile(import_path)
	}

	// 获取主题配置（合并默认值）
	themeSettings, _ := themeService.GetThemeConfigWithDefaults(site.Theme, defaultThemeJSON)

	c.JSON(http.StatusOK, gin.H{
		"site_name":          site.SiteName,
		"site_subtitle":      site.SiteSubtitle,
		"site_description":   site.SiteDescription,
		"theme":              site.Theme,
		"theme_settings":     themeSettings,
		"enable_register":    site.AllowRegistration,
		"force_email_verify": site.ForceEmailVerify,
	})
}

// GetPublicProductCategories 公开获取活跃分类列表（无需认证）
func GetPublicProductCategories(c *gin.Context) {
	categories, err := commerceService.ListUserCategories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询分类失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": categories})
}

// GetPublicProducts 公开获取上架商品列表（无需认证）
func GetPublicProducts(c *gin.Context) {
	products, err := commerceService.ListUserProducts(c.Query("category_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询商品失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": products})
}

// GetPublicProduct 公开获取单个上架商品详情（无需认证）
func GetPublicProduct(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}
	product, err := commerceService.GetUserProduct(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "商品不存在"})
		return
	}
	c.JSON(http.StatusOK, product)
}
