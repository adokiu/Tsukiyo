package handlers

import (
	"net/http"
	"strings"

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

	q := ParseListQuery(c)

	// 搜索
	filtered := disks
	if q.Search != "" {
		search := strings.ToLower(q.Search)
		filtered = make([]infrastructure.DiskInfo, 0, len(disks))
		for _, d := range disks {
			if strings.Contains(strings.ToLower(d.Device), search) ||
				strings.Contains(strings.ToLower(d.Model), search) ||
				strings.Contains(strings.ToLower(d.Serial), search) ||
				strings.Contains(strings.ToLower(d.Type), search) ||
				strings.Contains(strings.ToLower(d.Filesystem), search) {
				filtered = append(filtered, d)
			}
		}
	}

	// 筛选
	if v, ok := q.Filters["type"]; ok && v != "" {
		tmp := make([]infrastructure.DiskInfo, 0, len(filtered))
		for _, d := range filtered {
			if d.Type == v {
				tmp = append(tmp, d)
			}
		}
		filtered = tmp
	}
	if v, ok := q.Filters["is_mounted"]; ok && v != "" {
		want := v == "true"
		tmp := make([]infrastructure.DiskInfo, 0, len(filtered))
		for _, d := range filtered {
			if d.IsMounted == want {
				tmp = append(tmp, d)
			}
		}
		filtered = tmp
	}
	if v, ok := q.Filters["is_system"]; ok && v != "" {
		want := v == "true"
		tmp := make([]infrastructure.DiskInfo, 0, len(filtered))
		for _, d := range filtered {
			if d.IsSystem == want {
				tmp = append(tmp, d)
			}
		}
		filtered = tmp
	}

	total := len(filtered)
	offset := (q.Page - 1) * q.PerPage
	if offset >= total {
		filtered = []infrastructure.DiskInfo{}
	} else {
		end := offset + q.PerPage
		if end > total {
			end = total
		}
		filtered = filtered[offset:end]
	}

	c.JSON(http.StatusOK, ListResponse{
		Data:    filtered,
		Total:   int64(total),
		Page:    q.Page,
		PerPage: q.PerPage,
	})
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
	BroadcastDataRefresh("disks", nodeID.String())
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

	q := ParseListQuery(c)

	// 搜索
	filtered := storages
	if q.Search != "" {
		search := strings.ToLower(q.Search)
		tmp := make([]infrastructure.StoragePoolInfo, 0, len(filtered))
		for _, s := range filtered {
			if strings.Contains(strings.ToLower(s.Name), search) ||
				strings.Contains(strings.ToLower(s.Driver), search) ||
				strings.Contains(strings.ToLower(s.Source), search) {
				tmp = append(tmp, s)
			}
		}
		filtered = tmp
	}

	// 筛选
	if v, ok := q.Filters["driver"]; ok && v != "" {
		tmp := make([]infrastructure.StoragePoolInfo, 0, len(filtered))
		for _, s := range filtered {
			if s.Driver == v {
				tmp = append(tmp, s)
			}
		}
		filtered = tmp
	}

	total := len(filtered)
	offset := (q.Page - 1) * q.PerPage
	if offset >= total {
		filtered = []infrastructure.StoragePoolInfo{}
	} else {
		end := offset + q.PerPage
		if end > total {
			end = total
		}
		filtered = filtered[offset:end]
	}

	c.JSON(http.StatusOK, ListResponse{
		Data:    filtered,
		Total:   int64(total),
		Page:    q.Page,
		PerPage: q.PerPage,
	})
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
	BroadcastDataRefresh("storages", nodeID.String())
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
	BroadcastDataRefresh("storages", nodeID.String())
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
	BroadcastDataRefresh("disks", nodeID.String())
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
	BroadcastDataRefresh("disks", nodeID.String())
}
