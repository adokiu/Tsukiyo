package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// ListTasks 获取任务列表
func ListTasks(c *gin.Context) {
	var tasks []models.Task
	if err := db.DB.Order("created_at DESC").Limit(100).Find(&tasks).Error; err != nil {
		zap.L().Error("查询任务列表失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	result := make([]gin.H, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, gin.H{
			"id":           task.ID.String(),
			"type":         task.Type,
			"status":       task.Status,
			"node_id":      task.NodeID.String(),
			"instance_id":  task.InstanceID,
			"user_id":      task.UserID,
			"error":        task.Error,
			"retry_count":  task.RetryCount,
			"created_at":   task.CreatedAt,
			"completed_at": task.CompletedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  result,
		"total": len(result),
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
