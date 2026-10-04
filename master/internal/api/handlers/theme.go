package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// UploadTheme 上传主题 ZIP 包
func UploadTheme(c *gin.Context) {
	data, err := io.ReadAll(c.Request.Body)
	if err != nil || len(data) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要上传的主题文件"})
		return
	}

	tempFile := filepath.Join(os.TempDir(), "uploaded_theme.zip")
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败: " + err.Error()})
		return
	}
	defer os.Remove(tempFile)

	themeInfo, err := themeService.ExtractAndValidateTheme(tempFile)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "主题上传成功", "theme": themeInfo})
}

// ListThemes 列出所有已安装主题
func ListThemes(c *gin.Context) {
	themes, err := themeService.ListThemes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取主题目录失败: " + err.Error()})
		return
	}

	// 添加默认主题
	defaultThemeJSON, err := DefaultThemeFS.ReadFile("defaultTheme/tsukiyo-theme.json")
	if err == nil {
		var dt models.Theme
		if err := json.Unmarshal(defaultThemeJSON, &dt); err == nil {
			themes = append([]models.Theme{dt}, themes...)
		}
	}

	c.JSON(http.StatusOK, themes)
}

// DeleteTheme 删除主题
func DeleteTheme(c *gin.Context) {
	var req struct {
		Short string `json:"short" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	if err := themeService.DeleteTheme(req.Short); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "主题删除成功"})
}

// SetTheme 设置当前主题
func SetTheme(c *gin.Context) {
	themeName := c.Query("theme")
	if themeName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "主题名称不能为空"})
		return
	}

	if themeName != "default" {
		themeConfigPath := filepath.Join("./data/theme", themeName, "tsukiyo-theme.json")
		if _, err := os.Stat(themeConfigPath); os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "主题不存在"})
			return
		}
	}

	if err := themeService.SetTheme(themeName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新主题设置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "主题设置成功", "theme": themeName})
}

// UpdateThemeSettings 更新主题配置
func UpdateThemeSettings(c *gin.Context) {
	themeName := c.Query("theme")
	if themeName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "主题名称不能为空"})
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	if err := themeService.UpdateThemeSettings(themeName, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新主题配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "主题配置更新成功"})
}

// GetThemeSettings 获取主题配置
func GetThemeSettings(c *gin.Context) {
	themeName := c.Query("theme")
	if themeName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "主题名称不能为空"})
		return
	}

	settings, err := themeService.GetThemeSettings(themeName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取主题配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, settings)
}

// ImportTheme 从 URL 导入主题
func ImportTheme(c *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	// 下载主题 ZIP
	themeData, err := downloadThemeFromURL(req.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "下载主题失败: " + err.Error()})
		return
	}

	tempFile := filepath.Join(os.TempDir(), "import_theme.zip")
	if err := os.WriteFile(tempFile, themeData, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败: " + err.Error()})
		return
	}
	defer os.Remove(tempFile)

	// preview 模式：仅解析并返回主题信息
	preview := c.Query("preview")
	if preview == "true" {
		themeInfo, err := themeService.PeekThemeFromZip(tempFile)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		exists := false
		themeDir := filepath.Join("./data/theme", themeInfo.Short)
		if _, err := os.Stat(themeDir); err == nil {
			exists = true
		}

		c.JSON(http.StatusOK, gin.H{
			"theme":  themeInfo,
			"exists": exists,
		})
		return
	}

	// 安装模式
	themeInfo, err := themeService.PeekThemeFromZip(tempFile)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	overwritten := false
	themeDir := filepath.Join("./data/theme", themeInfo.Short)
	if _, err := os.Stat(themeDir); err == nil {
		overwritten = true
	}

	installedTheme, err := themeService.ExtractAndValidateTheme(tempFile)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	msg := "主题导入成功"
	if overwritten {
		msg = "主题导入成功（已覆盖同名主题）"
	}

	c.JSON(http.StatusOK, gin.H{"message": msg, "theme": installedTheme})
}

// downloadThemeFromURL 从 URL 下载主题 ZIP
func downloadThemeFromURL(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("下载失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("下载的文件为空")
	}

	return data, nil
}

// GetAdminEntryPath 获取管理员安全入口路径（仅管理员可调用）
func GetAdminEntryPath(c *gin.Context) {
	var site models.SiteConfig
	if err := db.DB.First(&site).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取站点配置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"entry_path": site.AdminEntryPath,
	})
}

// RegenerateAdminEntryPath 重新生成管理员安全入口路径
func RegenerateAdminEntryPath(c *gin.Context) {
	var site models.SiteConfig
	if err := db.DB.First(&site).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取站点配置失败"})
		return
	}

	newPath := generateRandomEntryPath()
	if err := db.DB.Model(&site).Update("admin_entry_path", newPath).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新安全入口失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "安全入口已更新",
		"entry_path": newPath,
	})
}

// generateRandomEntryPath 生成随机 8 字符 hex 安全入口路径
func generateRandomEntryPath() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%08x", os.Getpid())
	}
	return hex.EncodeToString(b)
}

// ValidateAdminEntryPath 验证管理员安全入口路径
// 中间件：检查请求路径是否匹配安全入口路径
func ValidateAdminEntryPath() gin.HandlerFunc {
	return func(c *gin.Context) {
		entryPath := c.Param("entry_path")
		if entryPath == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not Found"})
			c.Abort()
			return
		}

		var site models.SiteConfig
		if err := db.DB.First(&site).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not Found"})
			c.Abort()
			return
		}

		if site.AdminEntryPath == "" || entryPath != site.AdminEntryPath {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not Found"})
			c.Abort()
			return
		}

		c.Set("admin_entry_valid", true)
		c.Next()
	}
}
