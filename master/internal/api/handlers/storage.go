package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"tsukiyo/master/internal/service"
	"tsukiyo/master/internal/service/infrastructure"
)

var storageService *infrastructure.StorageService

// InitStorageService 初始化存储服务
func InitStorageService(svc *infrastructure.StorageService) {
	storageService = svc
}

// DiskInfo 磁盘信息
type DiskInfo = infrastructure.DiskInfo

// FormatNodeDiskRequest 格式化磁盘请求
type FormatNodeDiskRequest = infrastructure.FormatNodeDiskRequest

// StoragePoolInfo 存储池信息
type StoragePoolInfo = infrastructure.StoragePoolInfo

// InitNodeStorageRequest 初始化存储池请求
type InitNodeStorageRequest = infrastructure.InitNodeStorageRequest

// CreatePartitionRequest 创建分区请求
type CreatePartitionRequest = infrastructure.CreatePartitionRequest

// DeletePartitionRequest 删除分区请求
type DeletePartitionRequest = infrastructure.DeletePartitionRequest

// ListNodeDisks 获取节点磁盘列表
func ListNodeDisks(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}

	disks, err := storageService.ListNodeDisks(nodeID)
	if err != nil {
		if err == service.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
			return
		}
		if serviceErr, ok := err.(*service.ServiceError); ok && serviceErr.Message == "Agent 管理器未初始化" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Agent 管理器未初始化"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "获取磁盘信息失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": disks})
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

	userID, _ := c.Get("user_id")
	task, err := storageService.FormatNodeDisk(nodeID, req, userID.(uint))
	if err != nil {
		if err == service.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
			return
		}
		if err == service.ErrNodeOffline {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "节点离线"})
			return
		}
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

// ListNodeStorages 获取节点存储池列表
func ListNodeStorages(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}

	storages, err := storageService.ListNodeStorages(nodeID)
	if err != nil {
		if err == service.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
			return
		}
		if serviceErr, ok := err.(*service.ServiceError); ok && serviceErr.Message == "Agent 管理器未初始化" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Agent 管理器未初始化"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "获取存储池信息失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": storages})
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

	userID, _ := c.Get("user_id")
	task, err := storageService.InitNodeStorage(nodeID, req, userID.(uint))
	if err != nil {
		if err == service.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
			return
		}
		if err == service.ErrNodeOffline {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "节点离线"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建任务失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "存储池初始化任务已创建",
		"task_id": task.ID.String(),
	})
}

// DeleteNodeStorage 删除节点存储池
func DeleteNodeStorage(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}

	poolName := c.Param("name")
	if poolName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少存储池名称"})
		return
	}

	userID, _ := c.Get("user_id")
	task, err := storageService.DeleteNodeStorage(nodeID, poolName, userID.(uint))
	if err != nil {
		if err == service.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
			return
		}
		if err == service.ErrNodeOffline {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "节点离线"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建任务失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "删除存储池任务已创建",
		"task_id": task.ID.String(),
		"name":    poolName,
	})
}

// ListNodeVolumes 获取存储池 Volume 列表
func ListNodeVolumes(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}

	poolName := c.Param("name")
	if poolName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少存储池名称"})
		return
	}

	volumes, err := storageService.ListNodeVolumes(nodeID, poolName)
	if err != nil {
		if err == service.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "获取卷列表失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": volumes})
}

// GetNodeStorageResources 获取存储池空间用量
func GetNodeStorageResources(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}

	poolName := c.Param("name")
	if poolName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少存储池名称"})
		return
	}

	resources, err := storageService.GetNodeStorageResources(nodeID, poolName)
	if err != nil {
		if err == service.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "获取存储资源失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": resources})
}

// CreatePartition 创建磁盘分区
func CreatePartition(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}

	var req CreatePartitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	userID, _ := c.Get("user_id")
	task, err := storageService.CreatePartition(nodeID, req, userID.(uint))
	if err != nil {
		if err == service.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
			return
		}
		if err == service.ErrNodeOffline {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "节点离线"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建任务失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "创建分区任务已创建",
		"task_id": task.ID.String(),
		"device":  req.Device,
		"size_gb": req.SizeGB,
	})
}

// DeletePartition 删除磁盘分区
func DeletePartition(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}

	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少设备路径"})
		return
	}

	req := DeletePartitionRequest{Device: device}
	userID, _ := c.Get("user_id")
	task, err := storageService.DeletePartition(nodeID, req, userID.(uint))
	if err != nil {
		if err == service.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
			return
		}
		if err == service.ErrNodeOffline {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "节点离线"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建任务失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "删除分区任务已创建",
		"task_id": task.ID.String(),
		"device":  device,
	})
}
