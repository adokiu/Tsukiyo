package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// CreateSnapshotRequest 创建快照请求
type CreateSnapshotRequest struct {
	Name        string `json:"name" binding:"required,max=64"`
	Description string `json:"description,omitempty"`
}

// SnapshotInfo 快照信息
type SnapshotInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	SizeBytes   int64     `json:"size_bytes"`
	IsScheduled bool      `json:"is_scheduled"`
	CreatedAt   time.Time `json:"created_at"`
}

// ListSnapshots 获取快照列表
func ListSnapshots(c *gin.Context) {
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}

	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	var snapshots []models.Snapshot
	if err := db.DB.Where("instance_id = ?", instanceID).Order("created_at DESC").Find(&snapshots).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	result := make([]SnapshotInfo, 0, len(snapshots))
	for _, snap := range snapshots {
		result = append(result, SnapshotInfo{
			ID:          snap.ID.String(),
			Name:        snap.Name,
			Description: snap.Description,
			SizeBytes:   snap.SizeBytes,
			IsScheduled: snap.IsScheduled,
			CreatedAt:   snap.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  result,
		"total": len(result),
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

	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}

	// 检查快照配额
	var snapshotCount int64
	db.DB.Model(&models.Snapshot{}).Where("instance_id = ?", instanceID).Count(&snapshotCount)
	if int(snapshotCount) >= instance.SnapshotLimit {
		c.JSON(http.StatusForbidden, gin.H{"error": "快照数量已达上限"})
		return
	}

	userID, _ := c.Get("user_id")
	payload := map[string]interface{}{
		"instance_id": instance.IncusName,
		"name":        req.Name,
	}
	payloadBytes, _ := json.Marshal(payload)

	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeCreateSnapshot,
		NodeID:     instance.NodeID,
		InstanceID: &instance.ID,
		UserID:     userID.(uint),
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
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

	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}

	var snapshot models.Snapshot
	if err := db.DB.Where("instance_id = ? AND name = ?", instanceID, snapshotName).First(&snapshot).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "快照不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	userID, _ := c.Get("user_id")
	payload := map[string]interface{}{
		"instance_id":   instance.IncusName,
		"snapshot_name": snapshot.Name,
	}
	payloadBytes, _ := json.Marshal(payload)

	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeRestoreSnapshot,
		NodeID:     instance.NodeID,
		InstanceID: &instance.ID,
		UserID:     userID.(uint),
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
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

	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}

	var snapshot models.Snapshot
	if err := db.DB.Where("instance_id = ? AND name = ?", instanceID, snapshotName).First(&snapshot).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "快照不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	userID, _ := c.Get("user_id")
	payload := map[string]interface{}{
		"instance_id":   instance.IncusName,
		"snapshot_name": snapshot.Name,
	}
	payloadBytes, _ := json.Marshal(payload)

	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeDeleteSnapshot,
		NodeID:     instance.NodeID,
		InstanceID: &instance.ID,
		UserID:     userID.(uint),
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建任务失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "删除快照任务已下发",
		"task_id": task.ID.String(),
	})
}
