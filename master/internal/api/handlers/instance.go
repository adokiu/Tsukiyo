package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

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
		if err == service.ErrInvalidBridgeID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的网桥 ID"})
			return
		}
		if err == service.ErrBridgeNotFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": "网桥不存在或不属于该节点"})
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
	_, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	// 分页参数
	page := 1
	perPage := 20
	if v := c.Query("page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			page = p
		}
	}
	if v := c.Query("per_page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			perPage = p
		}
	}

	// 过滤参数
	search := c.Query("search")
	typeFilter := c.Query("type")
	statusFilter := c.Query("filter_status")

	// 构建查询
	query := db.DB.Model(&models.Instance{})
	if search != "" {
		query = query.Where("name ILIKE ? OR incus_name ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if typeFilter != "" {
		query = query.Where("type = ?", typeFilter)
	}
	if statusFilter != "" {
		query = query.Where("status = ?", statusFilter)
	}

	var total int64
	query.Count(&total)

	offset := (page - 1) * perPage
	var instances []models.Instance
	if err := query.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&instances).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	// 预加载节点和用户信息
	nodeIDs := make(map[uuid.UUID]string)
	userIDs := make(map[uint]string)
	for _, inst := range instances {
		if _, ok := nodeIDs[inst.NodeID]; !ok {
			var node models.Node
			if err := db.DB.Where("id = ?", inst.NodeID).First(&node).Error; err == nil {
				nodeIDs[inst.NodeID] = node.Name
			}
		}
		if _, ok := userIDs[inst.UserID]; !ok {
			var u models.User
			if err := db.DB.Where("id = ?", inst.UserID).First(&u).Error; err == nil {
				userIDs[inst.UserID] = u.Username
			}
		}
	}

	// 预加载 EIP 信息
	type eipInfo struct {
		IP    string
		Alias string
	}
	ipv4EIPs := make(map[uuid.UUID]eipInfo)
	ipv6EIPs := make(map[uuid.UUID]eipInfo)
	for _, inst := range instances {
		if inst.IPv4EIPAllocationID != nil {
			if _, ok := ipv4EIPs[*inst.IPv4EIPAllocationID]; !ok {
				var alloc models.EIPAllocation
				if err := db.DB.Where("id = ?", *inst.IPv4EIPAllocationID).First(&alloc).Error; err == nil {
					ipv4EIPs[*inst.IPv4EIPAllocationID] = eipInfo{IP: stripCIDRMask(alloc.CIDR), Alias: alloc.Alias}
				}
			}
		}
		if inst.IPv6EIPAllocationID != nil {
			if _, ok := ipv6EIPs[*inst.IPv6EIPAllocationID]; !ok {
				var alloc models.EIPAllocation
				if err := db.DB.Where("id = ?", *inst.IPv6EIPAllocationID).First(&alloc).Error; err == nil {
					ipv6EIPs[*inst.IPv6EIPAllocationID] = eipInfo{IP: stripCIDRMask(alloc.CIDR), Alias: alloc.Alias}
				}
			}
		}
	}

	// 从 Redis 读取实时指标
	ctx := context.Background()
	result := make([]gin.H, 0, len(instances))
	for _, inst := range instances {
		item := gin.H{
			"id":                 inst.ID.String(),
			"name":               inst.Name,
			"type":               inst.Type,
			"status":             inst.Status,
			"node_id":            inst.NodeID.String(),
			"node_name":          nodeIDs[inst.NodeID],
			"user_id":            inst.UserID,
			"owner_name":         userIDs[inst.UserID],
			"incus_name":         inst.IncusName,
			"vcpu":               inst.VCPU,
			"memory_mb":          inst.MemoryMB,
			"swap_mb":            inst.SwapMB,
			"disk_mb":            inst.DiskMB,
			"storage_pool":       inst.StoragePool,
			"internal_ipv4":      inst.InternalIPv4,
			"internal_ipv6":      inst.InternalIPv6,
			"ssh_port":           inst.SSHPort,
			"login_method":       inst.LoginMethod,
			"network_down":       inst.NetworkDownMbps,
			"network_up":         inst.NetworkUpMbps,
			"io_read_iops":       inst.IOReadIops,
			"io_write_iops":      inst.IOWriteIops,
			"monthly_traffic":    inst.MonthlyTrafficGB,
			"traffic_used_gb":    inst.TrafficUsedGB,
			"traffic_mode":       inst.TrafficMode,
			"over_limit_action":  inst.OverLimitAction,
			"throttle_mbps":      inst.ThrottleMbps,
			"is_over_limit":      inst.IsOverLimit,
			"snapshot_limit":     inst.SnapshotLimit,
			"port_mapping_limit": inst.PortMappingLimit,
			"expires_at":         inst.ExpiresAt,
			"created_at":         inst.CreatedAt,
			"has_eip":            inst.IPv4EIPAllocationID != nil || inst.IPv6EIPAllocationID != nil,
		}
		if inst.BridgeID != nil {
			item["bridge_id"] = inst.BridgeID.String()
		}
		if inst.IPv4EIPAllocationID != nil {
			if eip, ok := ipv4EIPs[*inst.IPv4EIPAllocationID]; ok {
				item["ipv4_eip"] = eip.IP
				item["ipv4_eip_alias"] = eip.Alias
			}
		}
		if inst.IPv6EIPAllocationID != nil {
			if eip, ok := ipv6EIPs[*inst.IPv6EIPAllocationID]; ok {
				item["ipv6_eip"] = eip.IP
				item["ipv6_eip_alias"] = eip.Alias
			}
		}

		// 从 Redis 读取实时指标
		metricKey := fmt.Sprintf("instance:%s:metrics", inst.ID)
		if metricData, err := db.RedisClient.Get(ctx, metricKey).Result(); err == nil {
			var metrics map[string]interface{}
			if err := json.Unmarshal([]byte(metricData), &metrics); err == nil {
				item["metrics"] = metrics
			}
		}

		result = append(result, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  result,
		"total": total,
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

	resp := gin.H{
		"id":                 instance.ID.String(),
		"name":               instance.Name,
		"type":               instance.Type,
		"status":             instance.Status,
		"node_id":            instance.NodeID.String(),
		"user_id":            instance.UserID,
		"incus_name":         instance.IncusName,
		"template_id":        instance.TemplateID,
		"vcpu":               instance.VCPU,
		"memory_mb":          instance.MemoryMB,
		"swap_mb":            instance.SwapMB,
		"disk_mb":            instance.DiskMB,
		"storage_pool":       instance.StoragePool,
		"internal_ipv4":      instance.InternalIPv4,
		"internal_ipv6":      instance.InternalIPv6,
		"login_method":       instance.LoginMethod,
		"ssh_port":           instance.SSHPort,
		"ssh_password":       instance.SSHPassword,
		"ssh_public_key":     instance.SSHPublicKey,
		"network_down":       instance.NetworkDownMbps,
		"network_up":         instance.NetworkUpMbps,
		"io_read_iops":       instance.IOReadIops,
		"io_write_iops":      instance.IOWriteIops,
		"monthly_traffic":    instance.MonthlyTrafficGB,
		"traffic_used_gb":    instance.TrafficUsedGB,
		"traffic_mode":       instance.TrafficMode,
		"over_limit_action":  instance.OverLimitAction,
		"throttle_mbps":      instance.ThrottleMbps,
		"is_over_limit":      instance.IsOverLimit,
		"snapshot_limit":     instance.SnapshotLimit,
		"port_mapping_limit": instance.PortMappingLimit,
		"data_disks":         instance.DataDisks,
		"port_mappings":      instance.PortMappings,
		"expires_at":         instance.ExpiresAt,
		"created_at":         instance.CreatedAt,
		"ipv4_mode":          instance.IPv4Mode,
		"ipv6_mode":          instance.IPv6Mode,
	}
	if instance.IPv4EIPAllocationID != nil {
		resp["ipv4_eip_allocation_id"] = instance.IPv4EIPAllocationID.String()
	}
	if instance.IPv6EIPAllocationID != nil {
		resp["ipv6_eip_allocation_id"] = instance.IPv6EIPAllocationID.String()
	}

	// 加载节点名称
	var node models.Node
	if err := db.DB.Where("id = ?", instance.NodeID).First(&node).Error; err == nil {
		resp["node_name"] = node.Name
	}

	// 加载所有者信息
	var user models.User
	if err := db.DB.Where("id = ?", instance.UserID).First(&user).Error; err == nil {
		resp["owner_name"] = user.Username
		resp["owner_email"] = user.Email
	}

	// 加载 EIP 地址（去掉 CIDR 子网掩码后缀，只保留 IP）
	if instance.IPv4EIPAllocationID != nil {
		var alloc models.EIPAllocation
		if err := db.DB.Where("id = ?", *instance.IPv4EIPAllocationID).First(&alloc).Error; err == nil {
			resp["ipv4_eip"] = stripCIDRMask(alloc.CIDR)
			if alloc.Alias != "" {
				resp["ipv4_eip_alias"] = alloc.Alias
			}
		}
	}
	if instance.IPv6EIPAllocationID != nil {
		var alloc models.EIPAllocation
		if err := db.DB.Where("id = ?", *instance.IPv6EIPAllocationID).First(&alloc).Error; err == nil {
			resp["ipv6_eip"] = stripCIDRMask(alloc.CIDR)
			if alloc.Alias != "" {
				resp["ipv6_eip_alias"] = alloc.Alias
			}
		}
	}
	if instance.BridgeID != nil {
		resp["bridge_id"] = instance.BridgeID.String()
		var bridge models.Bridge
		if err := db.DB.Where("id = ?", *instance.BridgeID).First(&bridge).Error; err == nil {
			resp["bridge_name"] = bridge.Name
			resp["bridge_iface"] = bridge.BridgeName
			resp["bridge_cidr"] = bridge.IPv4CIDR
			resp["bridge_gateway"] = bridge.IPv4Gateway
		}
	}

	// 是否有公网 IP（有 EIP 的实例不需要端口映射）
	resp["has_eip"] = instance.IPv4EIPAllocationID != nil || instance.IPv6EIPAllocationID != nil

	c.JSON(http.StatusOK, resp)
}

// UpdateInstanceRequest 更新实例请求（使用 service 层的类型别名）
type UpdateInstanceRequest = instance.UpdateInstanceRequest

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

	if err := instanceService.UpdateInstance(instanceID, req); err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		if err == service.ErrInstanceBusy {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		if err == service.ErrInstanceBanned {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		if err == service.ErrInstanceExpired {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		if err == service.ErrDiskShrinkNotSupported {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err == service.ErrNoValidUpdateFields {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// SetInstanceStatus 管理员强制修改实例状态
func SetInstanceStatus(c *gin.Context) {
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	if err := instanceService.SetInstanceStatus(instanceID, models.InstanceStatus(req.Status)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "状态已更新"})
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

	var req instance.ReinstallInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	userID, _ := c.Get("user_id")
	task, err := instanceService.ReinstallInstance(instanceID, userID.(uint), req)
	if err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		if err == service.ErrInstanceBusy {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
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
	Password string `json:"password,omitempty"`
}

// ResetInstancePassword 重置密码（异步任务）
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

	userID, _ := c.Get("user_id")
	task, err := instanceService.ResetInstancePassword(instanceID, userID.(uint), req.Password)
	if err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		if err == service.ErrInstanceBusy {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		if err == service.ErrInstanceBanned {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		if err == service.ErrInstanceExpired {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建重置密码任务失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "重置密码任务已创建", "task_id": task.ID.String()})
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
		if err == service.ErrNodeNotConnected {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "节点离线"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取控制台失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetConsoleCredentials 通过控制台 token 换取实例登录密码
func GetConsoleCredentials(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 token"})
		return
	}

	result, err := instanceService.GetConsoleCredentialsByToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token 无效或已过期"})
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
	if err != nil || metrics == nil {
		c.JSON(http.StatusOK, gin.H{})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"cpu_usage":       metrics.CPU,
		"memory_usage":    metrics.MemUsed * 1024 * 1024,   // MB -> bytes
		"memory_total":    metrics.MemTotal * 1024 * 1024,  // MB -> bytes
		"disk_used":       metrics.DiskUsed * 1024 * 1024,  // MB -> bytes
		"disk_total":      metrics.DiskTotal * 1024 * 1024, // MB -> bytes
		"disk_read_bps":   metrics.DiskReadBps,
		"disk_write_bps":  metrics.DiskWriteBps,
		"disk_read_iops":  metrics.DiskReadIops,
		"disk_write_iops": metrics.DiskWriteIops,
		"network_rx":      metrics.NetIn,
		"network_tx":      metrics.NetOut,
		"timestamp":       metrics.Timestamp.Unix(),
	})
}

// GetInstanceMetricsHistory 获取实例历史监控数据
func GetInstanceMetricsHistory(c *gin.Context) {
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}

	period := c.DefaultQuery("period", "1h")
	var from, to time.Time
	to = time.Now()
	switch period {
	case "1m":
		from = to.Add(-1 * time.Minute)
	case "15m":
		from = to.Add(-15 * time.Minute)
	case "1h":
		from = to.Add(-1 * time.Hour)
	case "1d":
		from = to.Add(-24 * time.Hour)
	case "7d":
		from = to.Add(-7 * 24 * time.Hour)
	default:
		from = to.Add(-1 * time.Hour)
	}

	interval := "1m"
	switch period {
	case "1d":
		interval = "15m"
	case "7d":
		interval = "1h"
	}

	points, err := monitor.GetInstanceMetrics(instanceID, from, to, interval)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询监控数据失败"})
		return
	}

	// 转换单位：MB -> bytes，使前端可以用 formatBytes 统一显示
	result := make([]gin.H, 0, len(points))
	for _, p := range points {
		result = append(result, gin.H{
			"timestamp":       p.Timestamp,
			"cpu":             p.CPU,
			"cpu_max":         p.CPUMax,
			"cpu_min":         p.CPUMin,
			"mem_used":        p.MemUsed * 1024 * 1024,
			"mem_used_max":    p.MemUsedMax * 1024 * 1024,
			"mem_used_min":    p.MemUsedMin * 1024 * 1024,
			"mem_total":       p.MemTotal * 1024 * 1024,
			"disk_used":       p.DiskUsed * 1024 * 1024,
			"disk_used_max":   p.DiskUsedMax * 1024 * 1024,
			"disk_used_min":   p.DiskUsedMin * 1024 * 1024,
			"disk_total":      p.DiskTotal * 1024 * 1024,
			"disk_read_bps":   p.DiskReadBps,
			"disk_read_max":   p.DiskReadMax,
			"disk_write_bps":  p.DiskWriteBps,
			"disk_write_max":  p.DiskWriteMax,
			"disk_read_iops":  p.DiskReadIops,
			"disk_write_iops": p.DiskWriteIops,
			"net_in":          p.NetIn,
			"net_in_max":      p.NetInMax,
			"net_in_min":      p.NetInMin,
			"net_out":         p.NetOut,
			"net_out_max":     p.NetOutMax,
			"net_out_min":     p.NetOutMin,
			"net_in_total":    p.NetInTotal,
			"net_out_total":   p.NetOutTotal,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// BanInstance 封禁实例
func BanInstance(c *gin.Context) {
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}

	if err := instanceService.BanInstance(instanceID); err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		if err == service.ErrInstanceBusy {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "封禁失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "实例已封禁"})
}

// UnbanInstance 解封实例
func UnbanInstance(c *gin.Context) {
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}

	if err := instanceService.UnbanInstance(instanceID); err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解封失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "实例已解封"})
}

// RenewInstanceRequest 续期请求
type RenewInstanceRequest struct {
	ExpiresAt *string `json:"expires_at,omitempty"`
}

// RenewInstance 续期实例
func RenewInstance(c *gin.Context) {
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}

	var req RenewInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	var newExpiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的到期时间格式"})
			return
		}
		newExpiresAt = &t
	}

	if err := instanceService.RenewInstance(instanceID, newExpiresAt); err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "续期失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "实例已续期"})
}

// UpdateInstanceNetwork 修改实例网络配置
func UpdateInstanceNetwork(c *gin.Context) {
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}

	var req instance.UpdateInstanceNetworkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	task, err := instanceService.UpdateInstanceNetwork(instanceID, req)
	if err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		if err == service.ErrInstanceBusy {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		if err == service.ErrInstanceBanned {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		if err == service.ErrInstanceExpired {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建网络配置任务失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "网络配置任务已创建", "task_id": task.ID.String()})
}

// AddInstanceDisk 添加数据盘
func AddInstanceDisk(c *gin.Context) {
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}

	var req instance.DataDiskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	task, err := instanceService.AddDataDisk(instanceID, req)
	if err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		if err == service.ErrInstanceBusy {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		if err == service.ErrInstanceBanned {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		if err == service.ErrInstanceExpired {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		if err == service.ErrDiskNameExists {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建添加数据盘任务失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "添加数据盘任务已创建", "task_id": task.ID.String()})
}

// DeleteInstanceDisk 删除数据盘
func DeleteInstanceDisk(c *gin.Context) {
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}

	diskID, err := uuid.Parse(c.Param("disk_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的数据盘 ID"})
		return
	}

	task, err := instanceService.DeleteDataDisk(instanceID, diskID)
	if err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		if err == service.ErrInstanceBusy {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		if err == service.ErrInstanceBanned {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		if err == service.ErrInstanceExpired {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		if err == service.ErrDiskNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建删除数据盘任务失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除数据盘任务已创建", "task_id": task.ID.String()})
}

// ResizeInstanceDiskRequest 扩容数据盘请求
type ResizeInstanceDiskRequest struct {
	SizeMB int `json:"size_mb" binding:"required,min=1"`
}

// ResizeInstanceDisk 扩容数据盘
func ResizeInstanceDisk(c *gin.Context) {
	instanceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}

	diskID, err := uuid.Parse(c.Param("disk_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的数据盘 ID"})
		return
	}

	var req ResizeInstanceDiskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	task, err := instanceService.ResizeDataDisk(instanceID, diskID, req.SizeMB)
	if err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		if err == service.ErrInstanceBusy {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		if err == service.ErrInstanceBanned {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		if err == service.ErrInstanceExpired {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		if err == service.ErrDiskNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err == service.ErrDiskShrinkNotSupported {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建扩容数据盘任务失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "扩容数据盘任务已创建", "task_id": task.ID.String()})
}

// stripCIDRMask 去掉 CIDR 格式中的子网掩码后缀，只保留 IP 地址
// IPv4 /32 和 IPv6 /128 是单机地址，去掉前缀；其他前缀保留显示
// 例如 "172.18.10.8/32" -> "172.18.10.8"，"fd00::1/128" -> "fd00::1"
// "fd00::/64" -> "fd00::/64"（保留）
func stripCIDRMask(cidr string) string {
	if idx := strings.Index(cidr, "/"); idx != -1 {
		prefix := cidr[idx+1:]
		if prefix == "32" || prefix == "128" {
			return cidr[:idx]
		}
	}
	return cidr
}
