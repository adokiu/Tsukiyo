package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"tsukiyo/master/internal/security"
)

// ListSecurityAlerts 获取安全告警列表
func ListSecurityAlerts(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.ParseInt(limitStr, 10, 64)

	alerts, err := security.GetAlerts(limit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []security.Alert{}, "total": 0})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  alerts,
		"total": len(alerts),
	})
}

// GetSecuritySummary 获取安全汇总
func GetSecuritySummary(c *gin.Context) {
	c.JSON(http.StatusOK, security.GetSummary())
}
