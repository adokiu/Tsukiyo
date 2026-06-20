package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// ListSecurityAlerts 获取安全告警列表（从 PostgreSQL 查询）
func ListSecurityAlerts(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if offset < 0 {
		offset = 0
	}

	query := db.DB.Model(&models.SecurityAlert{}).Order("detected_at DESC")

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if severity := c.Query("severity"); severity != "" {
		query = query.Where("severity = ?", severity)
	}
	if alertType := c.Query("type"); alertType != "" {
		query = query.Where("alert_type = ?", alertType)
	}
	if nodeID := c.Query("node_id"); nodeID != "" {
		if _, err := uuid.Parse(nodeID); err == nil {
			query = query.Where("node_id = ?", nodeID)
		}
	}
	if instanceID := c.Query("instance_id"); instanceID != "" {
		query = query.Where("instance_id = ?", instanceID)
	}

	var total int64
	query.Count(&total)

	var alerts []models.SecurityAlert
	if err := query.Offset(offset).Limit(limit).Find(&alerts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询安全告警失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   alerts,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetSecuritySummary 获取安全告警汇总统计
func GetSecuritySummary(c *gin.Context) {
	type countResult struct {
		Key   string `json:"key"`
		Count int64  `json:"count"`
	}

	var totalOpen int64
	db.DB.Model(&models.SecurityAlert{}).Where("status = ?", models.AlertStatusOpen).Count(&totalOpen)

	var totalCritical int64
	db.DB.Model(&models.SecurityAlert{}).
		Where("status = ? AND severity = ?", models.AlertStatusOpen, models.AlertSeverityCritical).
		Count(&totalCritical)

	var totalWarning int64
	db.DB.Model(&models.SecurityAlert{}).
		Where("status = ? AND severity = ?", models.AlertStatusOpen, models.AlertSeverityWarning).
		Count(&totalWarning)

	var last24h int64
	db.DB.Model(&models.SecurityAlert{}).
		Where("detected_at > ?", time.Now().Add(-24*time.Hour)).
		Count(&last24h)

	var byType []countResult
	db.DB.Model(&models.SecurityAlert{}).
		Select("alert_type as key, count(*) as count").
		Where("status = ?", models.AlertStatusOpen).
		Group("alert_type").
		Find(&byType)

	typeMap := make(map[string]int64)
	for _, t := range byType {
		typeMap[t.Key] = t.Count
	}

	c.JSON(http.StatusOK, gin.H{
		"open_alerts":     totalOpen,
		"critical_alerts": totalCritical,
		"warning_alerts":  totalWarning,
		"last_24h":        last24h,
		"by_type":         typeMap,
	})
}

// ResolveSecurityAlert 解决安全告警
func ResolveSecurityAlert(c *gin.Context) {
	alertID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的告警 ID"})
		return
	}

	var alert models.SecurityAlert
	if err := db.DB.First(&alert, "id = ?", alertID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "告警不存在"})
		return
	}

	if alert.Status != models.AlertStatusOpen {
		c.JSON(http.StatusConflict, gin.H{"error": "告警已处理"})
		return
	}

	now := time.Now()
	userID := c.GetUint("user_id")
	if err := db.DB.Model(&alert).Updates(map[string]interface{}{
		"status":      models.AlertStatusResolved,
		"resolved_by": userID,
		"resolved_at": &now,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新告警状态失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "告警已标记为已解决"})
}

// IgnoreSecurityAlert 忽略安全告警
func IgnoreSecurityAlert(c *gin.Context) {
	alertID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的告警 ID"})
		return
	}

	var alert models.SecurityAlert
	if err := db.DB.First(&alert, "id = ?", alertID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "告警不存在"})
		return
	}

	if alert.Status != models.AlertStatusOpen {
		c.JSON(http.StatusConflict, gin.H{"error": "告警已处理"})
		return
	}

	if err := db.DB.Model(&alert).Update("status", models.AlertStatusIgnored).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新告警状态失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "告警已忽略"})
}
