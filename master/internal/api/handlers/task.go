package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// ListTasks 获取任务列表
func ListTasks(c *gin.Context) {
	page := 1
	perPage := 20
	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if pp := c.Query("per_page"); pp != "" {
		if parsed, err := strconv.Atoi(pp); err == nil && parsed > 0 && parsed <= 100 {
			perPage = parsed
		}
	}

	status := c.Query("status")
	nodeID := c.Query("node_id")

	query := db.DB.Model(&models.Task{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if nodeID != "" {
		if parsed, err := uuid.Parse(nodeID); err == nil {
			query = query.Where("node_id = ?", parsed)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		zap.L().Error("查询任务总数失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	var tasks []models.Task
	offset := (page - 1) * perPage
	if err := query.Preload("Node").Preload("Instance").Order("created_at DESC").Limit(perPage).Offset(offset).Find(&tasks).Error; err != nil {
		zap.L().Error("查询任务列表失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	result := make([]gin.H, 0, len(tasks))
	for _, task := range tasks {
		nodeName := ""
		if task.Node.ID != uuid.Nil {
			nodeName = task.Node.Name
		}
		instanceName := ""
		if task.Instance != nil {
			instanceName = task.Instance.Name
		}

		result = append(result, gin.H{
			"id":            task.ID.String(),
			"type":          task.Type,
			"status":        task.Status,
			"node_id":       task.NodeID.String(),
			"node_name":     nodeName,
			"instance_id":   task.InstanceID,
			"instance_name": instanceName,
			"user_id":       task.UserID,
			"error":         task.Error,
			"retry_count":   task.RetryCount,
			"created_at":    task.CreatedAt,
			"started_at":    task.StartedAt,
			"completed_at":  task.CompletedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":     result,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

// GetTask 获取任务详情
func GetTask(c *gin.Context) {
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的任务 ID"})
		return
	}

	var task models.Task
	if err := db.DB.Where("id = ?", taskID).First(&task).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":           task.ID.String(),
		"type":         task.Type,
		"status":       task.Status,
		"node_id":      task.NodeID.String(),
		"instance_id":  task.InstanceID,
		"user_id":      task.UserID,
		"payload":      task.Payload,
		"result":       task.Result,
		"error":        task.Error,
		"retry_count":  task.RetryCount,
		"max_retries":  task.MaxRetries,
		"created_at":   task.CreatedAt,
		"started_at":   task.StartedAt,
		"completed_at": task.CompletedAt,
	})
}

// GetTaskLogs 获取任务日志
func GetTaskLogs(c *gin.Context) {
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的任务 ID"})
		return
	}

	var logs []models.TaskLog
	if err := db.DB.Where("task_id = ?", taskID).Order("created_at ASC").Find(&logs).Error; err != nil {
		zap.L().Error("查询任务日志失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	result := make([]gin.H, 0, len(logs))
	for _, log := range logs {
		result = append(result, gin.H{
			"id":         log.ID.String(),
			"task_id":    log.TaskID.String(),
			"level":      log.Level,
			"message":    log.Message,
			"created_at": log.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  result,
		"total": len(result),
	})
}
