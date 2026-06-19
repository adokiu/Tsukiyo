package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"tsukiyo/master/internal/service"
	"tsukiyo/master/internal/service/instance"
)

var taskService *instance.TaskService

// InitTaskService 初始化任务服务
func InitTaskService(svc *instance.TaskService) {
	taskService = svc
}

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
	nodeIDStr := c.Query("node_id")
	var nodeID uuid.UUID
	if nodeIDStr != "" {
		if parsed, err := uuid.Parse(nodeIDStr); err == nil {
			nodeID = parsed
		}
	}

	tasks, total, err := taskService.ListTasks(instance.ListTasksRequest{
		Page:    page,
		PerPage: perPage,
		Status:  status,
		NodeID:  nodeID,
	})
	if err != nil {
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

	task, err := taskService.GetTask(taskID)
	if err != nil {
		if serviceErr, ok := err.(*service.ServiceError); ok && serviceErr.Message == "任务不存在" {
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

	logs, err := taskService.GetTaskLogs(taskID)
	if err != nil {
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
