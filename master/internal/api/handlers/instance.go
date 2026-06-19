package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/monitor"
	"tsukiyo/master/internal/service"
	"tsukiyo/master/internal/service/instance"
)

var instanceService *instance.InstanceService

// InitInstanceService 初始化实例服务
func InitInstanceService(svc *instance.InstanceService) {
	instanceService = svc
}

// DataDiskRequest 数据磁盘请求
type DataDiskRequest = instance.DataDiskRequest

// NATRequest NAT 请求
type NATRequest = instance.NATRequest

// CreateInstanceRequest 创建实例请求
type CreateInstanceRequest = instance.CreateInstanceRequest

// CreateInstance 创建实例
func CreateInstance(c *gin.Context) {
	var req CreateInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	instance, task, err := instanceService.CreateInstance(req)
	if err != nil {
		if err == service.ErrInvalidNodeID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
			return
		}
		if err == service.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
			return
		}
		if err == service.ErrNodeOffline {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "节点离线"})
			return
		}
		if err == service.ErrImageNotDownloaded {
			c.JSON(http.StatusBadRequest, gin.H{"error": "镜像未下载，请先下载镜像"})
			return
		}
		if err == service.ErrInvalidVPCID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 VPC ID"})
			return
		}
		if err == service.ErrVPCNotFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": "VPC 不存在或不属于该节点"})
			return
		}
		if err == service.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "目标用户不存在"})
			return
		}
		if err == service.ErrInstanceNameExists {
			c.JSON(http.StatusConflict, gin.H{"error": "该节点上已存在同名实例"})
			return
		}
		if err == service.ErrInvalidIPv4ID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的公网 IPv4 ID"})
			return
		}
		if err == service.ErrIPv4NotAvailable {
			c.JSON(http.StatusBadRequest, gin.H{"error": "公网 IPv4 不可用或已被分配"})
			return
		}
		if err == service.ErrInvalidIPv6ID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 IPv6 前缀 ID"})
			return
		}
		if err == service.ErrIPv6NotFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": "IPv6 前缀不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建实例失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":         instance.ID.String(),
		"name":       instance.Name,
		"incus_name": instance.IncusName,
		"status":     instance.Status,
		"task_id":    task.ID.String(),
	})
}

// ListInstances 获取实例列表
func ListInstances(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	instances, err := instanceService.ListInstances(userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	result := make([]gin.H, 0, len(instances))
	for _, inst := range instances {
		ipv4 := ""
		if inst.IPv4Address != nil {
			ipv4 = *inst.IPv4Address
		}
		ipv6 := ""
		if inst.IPv6Address != nil {
			ipv6 = *inst.IPv6Address
		}
		item := gin.H{
			"id":              inst.ID.String(),
			"name":            inst.Name,
			"type":            inst.Type,
			"status":          inst.Status,
			"node_id":         inst.NodeID.String(),
			"incus_name":      inst.IncusName,
			"vcpu":            inst.VCPU,
			"memory_mb":       inst.MemoryMB,
			"disk_gb":         inst.DiskGB,
			"storage_pool":    inst.StoragePool,
			"internal_ipv4":   inst.InternalIPv4,
			"ipv4_address":    ipv4,
			"ipv6_address":    ipv6,
			"ssh_port":        inst.SSHPort,
			"login_method":    inst.LoginMethod,
			"network_down":    inst.NetworkDownMbps,
			"network_up":      inst.NetworkUpMbps,
			"io_read":         inst.IOReadMBps,
			"io_write":        inst.IOWriteMBps,
			"monthly_traffic": inst.MonthlyTrafficGB,
			"traffic_mode":    inst.TrafficMode,
			"snapshot_limit":  inst.SnapshotLimit,
			"expires_at":      inst.ExpiresAt,
			"created_at":      inst.CreatedAt,
		}
		if inst.VPCID != nil {
			item["vpc_id"] = inst.VPCID.String()
		}
		result = append(result, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  result,
		"total": len(result),
	})
}

// GetInstance 获取实例详情
func GetInstance(c *gin.Context) {
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}

	instance, err := instanceService.GetInstance(instanceID)
	if err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	ipv4 := ""
	if instance.IPv4Address != nil {
		ipv4 = *instance.IPv4Address
	}
	ipv6 := ""
	if instance.IPv6Address != nil {
		ipv6 = *instance.IPv6Address
	}

	resp := gin.H{
		"id":              instance.ID.String(),
		"name":            instance.Name,
		"type":            instance.Type,
		"status":          instance.Status,
		"node_id":         instance.NodeID.String(),
		"user_id":         instance.UserID,
		"incus_name":      instance.IncusName,
		"template_id":     instance.TemplateID,
		"vcpu":            instance.VCPU,
		"memory_mb":       instance.MemoryMB,
		"disk_gb":         instance.DiskGB,
		"storage_pool":    instance.StoragePool,
		"internal_ipv4":   instance.InternalIPv4,
		"login_method":    instance.LoginMethod,
		"ipv4_address":    ipv4,
		"ipv6_address":    ipv6,
		"ssh_port":        instance.SSHPort,
		"ssh_password":    instance.SSHPassword,
		"ssh_public_key":  instance.SSHPublicKey,
		"network_down":    instance.NetworkDownMbps,
		"network_up":      instance.NetworkUpMbps,
		"io_read":         instance.IOReadMBps,
		"io_write":        instance.IOWriteMBps,
		"monthly_traffic": instance.MonthlyTrafficGB,
		"traffic_mode":    instance.TrafficMode,
		"snapshot_limit":  instance.SnapshotLimit,
		"data_disks":      instance.DataDisks,
		"nat_configs":     instance.NATConfigs,
		"port_mappings":   instance.PortMappings,
		"expires_at":      instance.ExpiresAt,
		"created_at":      instance.CreatedAt,
	}
	if instance.VPCID != nil {
		resp["vpc_id"] = instance.VPCID.String()
		var vpc models.VPCNetwork
		if err := db.DB.Where("id = ?", *instance.VPCID).First(&vpc).Error; err == nil {
			resp["vpc_name"] = vpc.Name
			resp["vpc_bridge"] = vpc.GetBridgeName()
			resp["vpc_cidr"] = vpc.IPv4CIDR
			resp["vpc_gateway"] = vpc.DefaultGatewayV4
		}
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateInstanceRequest 更新实例请求
type UpdateInstanceRequest struct {
	Name            string  `json:"name,omitempty"`
	VCPU            float64 `json:"vcpu,omitempty"`
	MemoryMB        int     `json:"memory_mb,omitempty"`
	DiskGB          int     `json:"disk_gb,omitempty"`
	NetworkDownMbps int     `json:"network_down_mbps,omitempty"`
	NetworkUpMbps   int     `json:"network_up_mbps,omitempty"`
	ExpiresAt       *string `json:"expires_at,omitempty"`
}

// UpdateInstance 更新实例
func UpdateInstance(c *gin.Context) {
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}

	var req UpdateInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if err := instanceService.UpdateInstance(instanceID, req.Name, req.VCPU, req.MemoryMB, req.DiskGB, req.NetworkDownMbps, req.NetworkUpMbps, req.ExpiresAt); err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// DeleteInstance 删除实例
func DeleteInstance(c *gin.Context) {
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}

	userID, _ := c.Get("user_id")
	task, err := instanceService.DeleteInstance(instanceID, userID.(uint))
	if err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "删除任务已创建",
		"task_id": task.ID.String(),
	})
}

// StartInstance 启动实例
func StartInstance(c *gin.Context) {
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}

	userID, _ := c.Get("user_id")
	task, err := instanceService.StartInstance(instanceID, userID.(uint))
	if err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建任务失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "启动任务已创建", "task_id": task.ID.String()})
}

// StopInstance 停止实例
func StopInstance(c *gin.Context) {
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}

	userID, _ := c.Get("user_id")
	task, err := instanceService.StopInstance(instanceID, userID.(uint))
	if err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建任务失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "停止任务已创建", "task_id": task.ID.String()})
}

// RestartInstance 重启实例
func RestartInstance(c *gin.Context) {
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}

	userID, _ := c.Get("user_id")
	task, err := instanceService.RestartInstance(instanceID, userID.(uint))
	if err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建任务失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "重启任务已创建", "task_id": task.ID.String()})
}

// ReinstallInstance 重装实例
func ReinstallInstance(c *gin.Context) {
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}

	userID, _ := c.Get("user_id")
	task, err := instanceService.ReinstallInstance(instanceID, userID.(uint))
	if err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建任务失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "重装任务已创建", "task_id": task.ID.String()})
}

// ResizeInstance 升降配
func ResizeInstance(c *gin.Context) {
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}

	userID, _ := c.Get("user_id")
	task, err := instanceService.ResizeInstance(instanceID, userID.(uint))
	if err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建任务失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "升降配任务已创建", "task_id": task.ID.String()})
}

// ResetInstancePasswordRequest 重置密码请求
type ResetInstancePasswordRequest struct {
	Password string `json:"password" binding:"required,min=8"`
}

// ResetInstancePassword 重置密码
func ResetInstancePassword(c *gin.Context) {
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}

	var req ResetInstancePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}

	if err := instanceService.ResetInstancePassword(instanceID, req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新密码失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码重置成功"})
}

// GetInstanceConsole 获取控制台 URL
func GetInstanceConsole(c *gin.Context) {
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}

	consoleType := c.Query("type")
	result, err := instanceService.GetInstanceConsole(instanceID, consoleType)
	if err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取控制台失败"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetInstanceMetrics 获取实例最新监控指标
func GetInstanceMetrics(c *gin.Context) {
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}

	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}

	metrics, err := monitor.GetInstanceLatestMetrics(instanceID)
	if err != nil {
		// 没有监控数据，返回空值
		c.JSON(http.StatusOK, gin.H{})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"cpu_usage":    metrics.CPU,
		"memory_usage": metrics.MemUsed,
		"memory_total": metrics.MemTotal,
		"disk_read":    metrics.DiskRead,
		"disk_write":   metrics.DiskWrite,
		"network_rx":   metrics.NetIn,
		"network_tx":   metrics.NetOut,
	})
}
