package api

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

const (
	themeDataDir   = "./data/theme"
	themeDistDir   = "dist"
	themeIndexFile = "index.html"
	adminDistDir   = "admin-dist"
)

// SetupThemeStaticRoutes 注册主题静态文件路由和 SPA 回退
// 路由：
//
//	/themes/:id/*path  -> 按主题 ID 从磁盘或 embed.FS 读取文件
//	NoRoute 回退       -> 非管理员路径返回当前主题的 dist/index.html
func SetupThemeStaticRoutes(r *gin.Engine, defaultThemeFS embed.FS) {
	// 获取默认主题子文件系统
	defaultThemeSubFS, err := fs.Sub(defaultThemeFS, "defaultTheme")
	if err != nil {
		zap.L().Fatal("初始化默认主题文件系统失败", zap.Error(err))
	}

	// 获取当前主题
	getCurrentTheme := func() string {
		var site models.SiteConfig
		if err := db.DB.First(&site).Error; err != nil {
			return "default"
		}
		if site.Theme == "" {
			return "default"
		}
		return site.Theme
	}

	// 获取文件内容
	// themeID: 主题短名称
	// relativePath: 相对于主题根目录的路径
	// 返回: content, contentType, exists
	getFileContent := func(themeID string, relativePath string) ([]byte, string, bool) {
		cleanPath := strings.TrimPrefix(relativePath, "/")
		cleanPath = filepath.Clean(cleanPath)

		// 非默认主题从磁盘读取
		if themeID != "default" {
			if strings.Contains(themeID, "..") || strings.Contains(themeID, "/") || strings.Contains(themeID, "\\") {
				return nil, "", false
			}

			themeBasePath := filepath.Join(themeDataDir, themeID)
			if !isSafePath(themeBasePath, cleanPath) {
				return nil, "", false
			}

			localPath := filepath.Join(themeBasePath, cleanPath)
			if info, err := os.Stat(localPath); err == nil && !info.IsDir() {
				content, err := os.ReadFile(localPath)
				if err == nil {
					return content, mime.TypeByExtension(filepath.Ext(localPath)), true
				}
			}
			// 本地文件不存在，回退到默认主题
		}

		// 从 embed.FS 读取默认主题
		embedPath := filepath.ToSlash(cleanPath)
		if strings.Contains(embedPath, "..") {
			return nil, "", false
		}

		if content, err := fs.ReadFile(defaultThemeSubFS, embedPath); err == nil {
			return content, mime.TypeByExtension(filepath.Ext(embedPath)), true
		}

		return nil, "", false
	}

	// 主题静态资源路由
	r.GET("/themes/:id/*path", func(c *gin.Context) {
		themeID := c.Param("id")
		filePath := c.Param("path")

		content, mimeType, exists := getFileContent(themeID, filePath)
		if exists {
			c.Data(http.StatusOK, mimeType, content)
			return
		}
		c.Status(http.StatusNotFound)
	})

	// SPA 回退：非 API、非 WebSocket、非管理员路径 -> 返回当前主题的 index.html
	r.NoRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
			return
		}

		reqPath := c.Request.URL.Path

		// API 和 WebSocket 路径返回 JSON 404
		if strings.HasPrefix(reqPath, "/api/") || strings.HasPrefix(reqPath, "/ws/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
			return
		}

		// 管理员路径：未登录返回 404
		// 管理员前端由独立静态服务处理，不在这里
		if strings.HasPrefix(reqPath, "/admin") {
			c.Status(http.StatusNotFound)
			return
		}

		// /console 和 /vnc 是独立页面，由管理员 embed.FS 处理
		if strings.HasPrefix(reqPath, "/console") || strings.HasPrefix(reqPath, "/vnc") {
			// 这些页面需要认证，由前端路由处理
			// 回退到管理员 dist
			content, mimeType, exists := getFileContent("default", path.Join(adminDistDir, reqPath))
			if exists {
				c.Data(http.StatusOK, mimeType, content)
				return
			}
			// 回退到 admin index.html
			content, _, exists = getFileContent("default", path.Join(adminDistDir, themeIndexFile))
			if exists {
				c.Data(http.StatusOK, "text/html; charset=utf-8", content)
				return
			}
			c.Status(http.StatusNotFound)
			return
		}

		// 用户前台 SPA 回退
		currentTheme := getCurrentTheme()

		// 1. 尝试从主题 dist 目录读取静态资源
		distPath := path.Join(themeDistDir, reqPath)
		content, mimeType, exists := getFileContent(currentTheme, distPath)
		if exists {
			c.Data(http.StatusOK, mimeType, content)
			return
		}

		// 2. 回退到 index.html
		targetFile := path.Join(themeDistDir, themeIndexFile)
		content, _, exists = getFileContent(currentTheme, targetFile)
		if exists {
			c.Data(http.StatusOK, "text/html; charset=utf-8", content)
			return
		}

		c.Status(http.StatusNotFound)
	})
}

// isSafePath 检查路径是否在基础目录内，防止路径遍历
func isSafePath(basePath, targetPath string) bool {
	fullPath := filepath.Join(basePath, targetPath)
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(fullPath)
	if err != nil {
		return false
	}
	return strings.HasPrefix(absTarget, absBase)
}
