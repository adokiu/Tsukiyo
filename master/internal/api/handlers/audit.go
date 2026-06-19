package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"tsukiyo/master/internal/service/system"
)

var auditService *system.AuditService

// InitAuditService 初始化审计服务
func InitAuditService(svc *system.AuditService) {
	auditService = svc
}

// AuditLogInfo 审计日志信息
type AuditLogInfo = system.AuditLogInfo

// ListAuditLogs 获取审计日志列表
func ListAuditLogs(c *gin.Context) {
	logs, err := auditService.ListAuditLogs(100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  logs,
		"total": len(logs),
	})
}
