package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// ListAuditLogs 获取审计日志列表
func ListAuditLogs(c *gin.Context) {
	var logs []models.AuditLog
	if err := db.DB.Order("created_at DESC").Limit(100).Find(&logs).Error; err != nil {
		zap.L().Error("查询审计日志失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	result := make([]gin.H, 0, len(logs))
	for _, log := range logs {
		result = append(result, gin.H{
			"id":         log.ID.String(),
			"user_id":    log.UserID,
			"username":   log.Username,
			"action":     log.Action,
			"target":     log.Target,
			"detail":     log.Detail,
			"ip_address": log.IPAddress,
			"success":    log.Success,
			"created_at": log.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  result,
		"total": len(result),
	})
}
