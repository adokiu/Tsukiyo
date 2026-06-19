package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"tsukiyo/master/internal/service"
	"tsukiyo/master/internal/service/instance"
)

var snapshotService *instance.SnapshotService

// InitSnapshotService 初始化快照服务
func InitSnapshotService(svc *instance.SnapshotService) {
	snapshotService = svc
}

// CreateSnapshotRequest 创建快照请求
type CreateSnapshotRequest = instance.CreateSnapshotRequest

// SnapshotInfo 快照信息
type SnapshotInfo = instance.SnapshotInfo

// ListSnapshots 获取快照列表
func ListSnapshots(c *gin.Context) {
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}

	snapshots, err := snapshotService.ListSnapshots(instanceID)
	if err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  snapshots,
		"total": len(snapshots),
	})
}

// CreateSnapshot 创建快照
func CreateSnapshot(c *gin.Context) {
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}

	var req CreateSnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	userID, _ := c.Get("user_id")
	task, err := snapshotService.CreateSnapshot(instanceID, req, userID.(uint))
	if err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		if serviceErr, ok := err.(*service.ServiceError); ok && serviceErr.Message == "快照数量已达上限" {
			c.JSON(http.StatusForbidden, gin.H{"error": "快照数量已达上限"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建任务失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "快照创建任务已下发",
		"task_id": task.ID.String(),
	})
}

// RestoreSnapshot 恢复快照
func RestoreSnapshot(c *gin.Context) {
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}

	snapshotName := c.Param("name")
	if snapshotName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "快照名称不能为空"})
		return
	}

	userID, _ := c.Get("user_id")
	task, err := snapshotService.RestoreSnapshot(instanceID, snapshotName, userID.(uint))
	if err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		if serviceErr, ok := err.(*service.ServiceError); ok && serviceErr.Message == "快照不存在" {
			c.JSON(http.StatusNotFound, gin.H{"error": "快照不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建任务失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "恢复快照任务已下发",
		"task_id": task.ID.String(),
	})
}

// DeleteSnapshot 删除快照
func DeleteSnapshot(c *gin.Context) {
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}

	snapshotName := c.Param("name")
	if snapshotName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "快照名称不能为空"})
		return
	}

	userID, _ := c.Get("user_id")
	task, err := snapshotService.DeleteSnapshot(instanceID, snapshotName, userID.(uint))
	if err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		if serviceErr, ok := err.(*service.ServiceError); ok && serviceErr.Message == "快照不存在" {
			c.JSON(http.StatusNotFound, gin.H{"error": "快照不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建任务失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "删除快照任务已下发",
		"task_id": task.ID.String(),
	})
}
