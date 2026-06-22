package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/service/instance"
)

// BatchCreateRequest 批量创建请求
type BatchCreateRequest struct {
	Count            int     `json:"count" binding:"required,min=1,max=50"`
	NamePrefix       string  `json:"name_prefix" binding:"required"`
	Type             string  `json:"type" binding:"required,oneof=container vm"`
	TemplateID       string  `json:"template_id" binding:"required"`
	NodeID           string  `json:"node_id" binding:"required,uuid"`
	AssignToUserID   uint    `json:"assign_to_user_id" binding:"required"`
	VCPU             float64 `json:"vcpu" binding:"required,min=0.1"`
	MemoryMB         int     `json:"memory_mb" binding:"required,min=64"`
	SwapMB           int     `json:"swap_mb,omitempty"`
	DiskMB           int     `json:"disk_mb" binding:"required,min=1"`
	StoragePool      string  `json:"storage_pool,omitempty"`
	LoginMethod      string  `json:"login_method,omitempty"`
	BridgeID         string  `json:"bridge_id,omitempty"`
	AssignEIPv4      bool    `json:"assign_eip_ipv4,omitempty"`
	AssignEIPv6      bool    `json:"assign_eip_ipv6,omitempty"`
	PortMappingCount int     `json:"port_mapping_count,omitempty"`
	NetworkDown      int     `json:"network_down_mbps,omitempty"`
	NetworkUp        int     `json:"network_up_mbps,omitempty"`
	IORead           int     `json:"io_read_iops,omitempty"`
	IOWrite          int     `json:"io_write_iops,omitempty"`
	MonthlyTraffic   int64   `json:"monthly_traffic_gb,omitempty"`
	SnapshotLimit    int     `json:"snapshot_limit,omitempty"`
}

// BatchCreate 批量创建实例
func BatchCreate(c *gin.Context) {
	var req BatchCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	nodeID, _ := uuid.Parse(req.NodeID)

	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
		return
	}

	if !node.IsHealthy() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "节点离线"})
		return
	}

	// 验证目标用户存在
	var targetUser models.User
	if err := db.DB.Where("id = ?", req.AssignToUserID).First(&targetUser).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "目标用户不存在"})
		return
	}

	created := make([]gin.H, 0, req.Count)
	failed := make([]gin.H, 0)

	for i := 0; i < req.Count; i++ {
		instanceID := uuid.New()
		incusName := "tsukiyo-" + instanceID.String()[:8]
		name := req.NamePrefix + "-" + incusName[8:]

		newInstance := models.Instance{
			ID:               instanceID,
			Name:             name,
			UserID:           req.AssignToUserID,
			NodeID:           nodeID,
			Type:             models.InstanceType(req.Type),
			Status:           models.InstanceStatusCreating,
			IncusName:        incusName,
			TemplateID:       req.TemplateID,
			VCPU:             req.VCPU,
			MemoryMB:         req.MemoryMB,
			SwapMB:           req.SwapMB,
			DiskMB:           req.DiskMB,
			StoragePool:      req.StoragePool,
			LoginMethod:      models.LoginMethod(req.LoginMethod),
			NetworkDownMbps:  req.NetworkDown,
			NetworkUpMbps:    req.NetworkUp,
			IOReadIops:       req.IORead,
			IOWriteIops:      req.IOWrite,
			MonthlyTrafficGB: req.MonthlyTraffic,
			SnapshotLimit:    req.SnapshotLimit,
		}
		if req.LoginMethod == "auto" {
			newInstance.SSHPassword = instance.GenerateRandomPassword(16)
		}

		if err := db.DB.Create(&newInstance).Error; err != nil {
			failed = append(failed, gin.H{"index": i, "error": err.Error()})
			continue
		}

		portMappingCount := req.PortMappingCount
		if portMappingCount < 1 {
			portMappingCount = 1
		}

		payload := map[string]interface{}{
			"instance_id":        newInstance.IncusName,
			"type":               newInstance.Type,
			"template_id":        newInstance.TemplateID,
			"vcpu":               newInstance.VCPU,
			"memory_mb":          newInstance.MemoryMB,
			"swap_mb":            newInstance.SwapMB,
			"disk_mb":            newInstance.DiskMB,
			"storage_pool":       newInstance.StoragePool,
			"login_method":       newInstance.LoginMethod,
			"ssh_password":       newInstance.SSHPassword,
			"network_down":       newInstance.NetworkDownMbps,
			"network_up":         newInstance.NetworkUpMbps,
			"io_read":            newInstance.IOReadIops,
			"io_write":           newInstance.IOWriteIops,
			"bridge_id":          req.BridgeID,
			"assign_eip_ipv4":    req.AssignEIPv4,
			"assign_eip_ipv6":    req.AssignEIPv6,
			"port_mapping_count": portMappingCount,
			"traffic_mode":       newInstance.TrafficMode,
			"monthly_traffic":    newInstance.MonthlyTrafficGB,
			"snapshot_limit":     newInstance.SnapshotLimit,
		}
		payloadBytes, _ := json.Marshal(payload)

		task := models.Task{
			ID:         uuid.New(),
			Type:       models.TaskTypeCreateInstance,
			NodeID:     nodeID,
			InstanceID: &newInstance.ID,
			UserID:     req.AssignToUserID,
			Status:     models.TaskStatusPending,
			Payload:    payloadBytes,
		}
		if err := db.DB.Create(&task).Error; err != nil {
			zap.L().Error("创建批量实例任务失败", zap.Error(err))
		}

		created = append(created, gin.H{
			"id":      newInstance.ID.String(),
			"name":    newInstance.Name,
			"task_id": task.ID.String(),
		})
	}

	c.JSON(http.StatusCreated, gin.H{
		"created":       created,
		"failed":        failed,
		"created_count": len(created),
		"failed_count":  len(failed),
	})
}

// BatchActionRequest 批量操作请求
type BatchActionRequest struct {
	InstanceIDs []string `json:"instance_ids" binding:"required,min=1,max=50"`
	Action      string   `json:"action" binding:"required,oneof=start stop restart delete"`
}

// BatchAction 批量操作实例
func BatchAction(c *gin.Context) {
	var req BatchActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	var taskType models.TaskType
	switch req.Action {
	case "start":
		taskType = models.TaskTypeStartInstance
	case "stop":
		taskType = models.TaskTypeStopInstance
	case "restart":
		taskType = models.TaskTypeRestartInstance
	case "delete":
		taskType = models.TaskTypeDeleteInstance
	}

	succeeded := make([]string, 0)
	failed := make([]gin.H, 0)

	for _, idStr := range req.InstanceIDs {
		instanceID, err := uuid.Parse(idStr)
		if err != nil {
			failed = append(failed, gin.H{"id": idStr, "error": "无效的实例 ID"})
			continue
		}

		var instance models.Instance
		if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
			failed = append(failed, gin.H{"id": idStr, "error": "实例不存在"})
			continue
		}

		payloadBytes, _ := json.Marshal(map[string]string{
			"instance_id": instance.IncusName,
		})
		task := models.Task{
			ID:         uuid.New(),
			Type:       taskType,
			NodeID:     instance.NodeID,
			InstanceID: &instance.ID,
			UserID:     uid,
			Status:     models.TaskStatusPending,
			Payload:    payloadBytes,
		}

		if err := db.DB.Create(&task).Error; err != nil {
			failed = append(failed, gin.H{"id": idStr, "error": "创建任务失败"})
			continue
		}

		if req.Action == "delete" {
			db.DB.Model(&instance).Update("status", models.InstanceStatusDeleting)
		}

		succeeded = append(succeeded, idStr)
	}

	c.JSON(http.StatusOK, gin.H{
		"action":        req.Action,
		"succeeded":     succeeded,
		"failed":        failed,
		"success_count": len(succeeded),
		"failed_count":  len(failed),
	})
}
