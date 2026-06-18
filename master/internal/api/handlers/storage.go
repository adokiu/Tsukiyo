package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"tsukiyo/master/internal/agent"
	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// agentMgr HTTP handler 使用的 Agent 管理器
var agentMgr *agent.Manager

// SetAgentManager 设置 Agent 管理器
func SetAgentManager(mgr *agent.Manager) {
	agentMgr = mgr
}

// DiskInfo 磁盘信息
type DiskInfo struct {
	Device     string `json:"device"`
	Size       int64  `json:"size"`
	Model      string `json:"model,omitempty"`
	Type       string `json:"type,omitempty"`
	Filesystem string `json:"filesystem,omitempty"`
	IsMounted  bool   `json:"is_mounted"`
	MountPoint string `json:"mount_point,omitempty"`
	IsSystem   bool   `json:"is_system"`
	IsInUse    bool   `json:"is_in_use"`
}

// ListNodeDisksRequest 获取节点磁盘列表请求
type ListNodeDisksRequest struct {
	NodeID string `uri:"id" binding:"required,uuid"`
}

// ListNodeDisks 获取节点磁盘列表
func ListNodeDisks(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}

	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	// 实时向 Agent 请求磁盘信息
	if agentMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Agent 管理器未初始化"})
		return
	}

	resp, err := agentMgr.SendRequest(nodeID, "get_disks", nil, 10*time.Second)
	if err != nil {
		zap.L().Warn("获取磁盘信息失败", zap.String("node_id", nodeID.String()), zap.Error(err))
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "获取磁盘信息失败: " + err.Error()})
		return
	}

	var disks []DiskInfo
	if err := json.Unmarshal(resp, &disks); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析磁盘数据失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": disks})
}

// FormatNodeDiskRequest 格式化磁盘请求
type FormatNodeDiskRequest struct {
	Device string `json:"device" binding:"required"`
	Type   string `json:"type" binding:"required,oneof=dir btrfs zfs lvm lvm-thin"`
}

// FormatNodeDisk 格式化节点磁盘
func FormatNodeDisk(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}

	var req FormatNodeDiskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
		return
	}

	if !node.IsHealthy() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "节点离线"})
		return
	}

	// 创建格式化任务
	userID, _ := c.Get("user_id")
	payload := map[string]interface{}{
		"device": req.Device,
		"type":   req.Type,
	}
	payloadBytes, _ := json.Marshal(payload)

	task := models.Task{
		ID:     uuid.New(),
		Type:   models.TaskTypeFormatDisk,
		NodeID: nodeID,
		UserID: userID.(uint),
		Status: models.TaskStatusPending,
		Payload: payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
		zap.L().Error("创建格式化任务失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建任务失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "格式化任务已创建",
		"task_id": task.ID.String(),
		"device":  req.Device,
		"type":    req.Type,
	})
}

// StoragePoolInfo 存储池信息
type StoragePoolInfo struct {
	Name      string `json:"name"`
	Driver    string `json:"driver"`
	Source    string `json:"source"`
	Total     int64  `json:"total"`
	Used      int64  `json:"used"`
	Available int64  `json:"available"`
	InUse     bool   `json:"in_use"`
}

// ListNodeStorages 获取节点存储池列表
func ListNodeStorages(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}

	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	// 实时向 Agent 请求存储池信息
	if agentMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Agent 管理器未初始化"})
		return
	}

	resp, err := agentMgr.SendRequest(nodeID, "get_storages", nil, 10*time.Second)
	if err != nil {
		zap.L().Warn("获取存储池信息失败", zap.String("node_id", nodeID.String()), zap.Error(err))
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "获取存储池信息失败: " + err.Error()})
		return
	}

	var storages []StoragePoolInfo
	if err := json.Unmarshal(resp, &storages); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析存储池数据失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": storages})
}

// InitNodeStorageRequest 初始化存储池请求
type InitNodeStorageRequest struct {
	Name   string `json:"name" binding:"required"`
	Driver string `json:"driver" binding:"required,oneof=dir btrfs zfs lvm"`
	Source string `json:"source" binding:"required"`
}

// InitNodeStorage 初始化节点存储池
func InitNodeStorage(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}

	var req InitNodeStorageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
		return
	}

	if !node.IsHealthy() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "节点离线"})
		return
	}

	userID, _ := c.Get("user_id")
	payload := map[string]interface{}{
		"name":   req.Name,
		"driver": req.Driver,
		"source": req.Source,
	}
	payloadBytes, _ := json.Marshal(payload)

	task := models.Task{
		ID:      uuid.New(),
		Type:    models.TaskTypeInitStorage,
		NodeID:  nodeID,
		UserID:  userID.(uint),
		Status:  models.TaskStatusPending,
		Payload: payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
		zap.L().Error("创建初始化存储池任务失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建任务失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "存储池初始化任务已创建",
		"task_id": task.ID.String(),
	})
}
