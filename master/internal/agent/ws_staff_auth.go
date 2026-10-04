package agent

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"tsukiyo/master/internal/api/middleware"
	"tsukiyo/master/internal/auth"
	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// authenticateStaffWebSocket 校验管理端 WebSocket（query: token 或 Authorization）。
func authenticateStaffWebSocket(c *gin.Context) (*models.User, bool) {
	tokenString := strings.TrimSpace(c.Query("token"))
	if tokenString == "" {
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			tokenString = strings.TrimSpace(authHeader[7:])
		}
	}
	if tokenString == "" {
		c.JSON(http.StatusOK, gin.H{"code": 401, "error": "缺少认证信息"})
		return nil, false
	}

	if auth.IsTokenRevoked(tokenString) {
		c.JSON(http.StatusOK, gin.H{"code": 401, "error": "Token 已吊销"})
		return nil, false
	}

	claims, err := auth.ParseToken(tokenString)
	if err != nil {
		zap.L().Warn("管理端 WS Token 解析失败", zap.Error(err))
		c.JSON(http.StatusOK, gin.H{"code": 401, "error": "无效的 Token"})
		return nil, false
	}

	var user models.User
	if err := db.DB.Where("id = ?", claims.UserID).First(&user).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 401, "error": "用户不存在"})
		return nil, false
	}
	if !user.IsActive() {
		c.JSON(http.StatusOK, gin.H{"code": 403, "error": "用户已被禁用"})
		return nil, false
	}
	if !middleware.UserHasAdminPanelAccess(user.ID) {
		c.JSON(http.StatusOK, gin.H{"code": 403, "error": "无权访问管理端实时通道"})
		return nil, false
	}

	return &user, true
}
