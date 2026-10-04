package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"tsukiyo/master/internal/api/middleware"
	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/monitor"
)

// UserMiddleware 用户所有权校验中间件
// 确保用户只能访问自己的实例
func UserMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		if userID == 0 {
			c.JSON(http.StatusOK, gin.H{"code": 401, "error": "未认证"})
			c.Abort()
			return
		}
		c.Set("owner_user_id", userID)
		c.Next()
	}
}

// GetUserDashboard 用户仪表盘
func GetUserDashboard(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var totalInstances int64
	var runningInstances int64
	var stoppedInstances int64

	db.DB.Model(&models.Instance{}).Where("user_id = ?", userID).Count(&totalInstances)
	db.DB.Model(&models.Instance{}).Where("user_id = ? AND status = ?", userID, models.InstanceStatusRunning).Count(&runningInstances)
	db.DB.Model(&models.Instance{}).Where("user_id = ? AND status = ?", userID, models.InstanceStatusStopped).Count(&stoppedInstances)

	// 最近实例
	var recentInstances []models.Instance
	db.DB.Where("user_id = ?", userID).Order("created_at DESC").Limit(5).Find(&recentInstances)

	c.JSON(http.StatusOK, gin.H{
		"total_instances":   totalInstances,
		"running_instances": runningInstances,
		"stopped_instances": stoppedInstances,
		"recent_instances":  recentInstances,
	})
}

// ListUserInstances 获取当前用户的实例列表
func ListUserInstances(c *gin.Context) {
	userID := middleware.GetUserID(c)

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

	search := c.Query("search")
	typeFilter := c.Query("type")
	statusFilter := c.Query("filter_status")

	query := db.DB.Model(&models.Instance{}).Where("user_id = ?", userID)
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

	// 状态分类统计（不受分页影响，受搜索和类型过滤影响）
	baseQuery := db.DB.Model(&models.Instance{}).Where("user_id = ?", userID)
	if search != "" {
		baseQuery = baseQuery.Where("name ILIKE ? OR incus_name ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if typeFilter != "" {
		baseQuery = baseQuery.Where("type = ?", typeFilter)
	}
	var allCount, runningCount, stoppedCount, creatingCount, errorCount int64
	baseQuery.Count(&allCount)
	baseQuery.Where("status = ?", models.InstanceStatusRunning).Count(&runningCount)
	baseQuery.Where("status = ?", models.InstanceStatusStopped).Count(&stoppedCount)
	baseQuery.Where("status = ?", models.InstanceStatusCreating).Count(&creatingCount)
	baseQuery.Where("status = ?", models.InstanceStatusError).Count(&errorCount)

	offset := (page - 1) * perPage
	var instances []models.Instance
	if err := query.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&instances).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
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
					ipv4EIPs[*inst.IPv4EIPAllocationID] = eipInfo{IP: alloc.GetIP(), Alias: alloc.Alias}
				}
			}
		}
		if inst.IPv6EIPAllocationID != nil {
			if _, ok := ipv6EIPs[*inst.IPv6EIPAllocationID]; !ok {
				var alloc models.EIPAllocation
				if err := db.DB.Where("id = ?", *inst.IPv6EIPAllocationID).First(&alloc).Error; err == nil {
					ipv6EIPs[*inst.IPv6EIPAllocationID] = eipInfo{IP: alloc.GetIP(), Alias: alloc.Alias}
				}
			}
		}
	}

	// 预加载 Bridge NAT 出口 IP 信息
	bridgeIDs := make(map[uuid.UUID]*models.Bridge)
	for _, inst := range instances {
		if inst.BridgeID != nil {
			if _, ok := bridgeIDs[*inst.BridgeID]; !ok {
				var bridge models.Bridge
				if err := db.DB.Where("id = ?", *inst.BridgeID).First(&bridge).Error; err == nil {
					bridgeIDs[*inst.BridgeID] = &bridge
				}
			}
		}
	}
	// 查询 Bridge 关联的 NAT 出口 EIP
	bridgeNATIPs := make(map[uuid.UUID]string)
	for bridgeID, bridge := range bridgeIDs {
		if bridge.NATEgressIPv4ID != nil {
			var natEIP models.EIPAllocation
			if err := db.DB.Where("id = ?", *bridge.NATEgressIPv4ID).First(&natEIP).Error; err == nil {
				bridgeNATIPs[bridgeID] = natEIP.GetIP()
			}
		}
	}

	type instanceMetrics struct {
		CPUUsage    float64 `json:"cpu_usage"`
		MemoryUsed  int64   `json:"memory_used"`
		MemoryTotal int64   `json:"memory_total"`
		DiskUsed    int64   `json:"disk_used"`
		DiskTotal   int64   `json:"disk_total"`
		NetInBps    int64   `json:"net_in_bps"`
		NetOutBps   int64   `json:"net_out_bps"`
		NetInTotal  int64   `json:"net_in_total"`
		NetOutTotal int64   `json:"net_out_total"`
	}

	type instanceWithExtra struct {
		models.Instance
		IPv4Addr string           `json:"ipv4_addr"`
		IPv6Addr string           `json:"ipv6_addr"`
		Metrics  *instanceMetrics `json:"metrics,omitempty"`
	}

	// 批量查询每个实例的最新监控指标
	metricsMap := make(map[uuid.UUID]*instanceMetrics)
	for _, inst := range instances {
		if inst.Status != models.InstanceStatusRunning {
			continue
		}
		mp, err := monitor.GetInstanceLatestMetrics(inst.ID)
		if err != nil || mp == nil {
			continue
		}
		metricsMap[inst.ID] = &instanceMetrics{
			CPUUsage:    mp.CPU,
			MemoryUsed:  (mp.MemUsed + (int64(inst.MemoryMB) - mp.MemTotal)) * 1024 * 1024,
			MemoryTotal: int64(inst.MemoryMB) * 1024 * 1024,
			DiskUsed:    (mp.DiskUsed + (int64(inst.DiskMB) - mp.DiskTotal)) * 1024 * 1024,
			DiskTotal:   int64(inst.DiskMB) * 1024 * 1024,
			NetInBps:    mp.NetIn,
			NetOutBps:   mp.NetOut,
			NetInTotal:  mp.NetInTotal,
			NetOutTotal: mp.NetOutTotal,
		}
	}

	var result []instanceWithExtra
	for _, inst := range instances {
		extra := instanceWithExtra{
			Instance: inst,
		}

		// IPv4 显示逻辑：NAT 出口 IP > EIP 别名 > EIP 本身
		if inst.IPv4Mode == "nat" && inst.BridgeID != nil {
			if natIP, ok := bridgeNATIPs[*inst.BridgeID]; ok {
				extra.IPv4Addr = natIP
			}
		}
		if extra.IPv4Addr == "" && inst.IPv4EIPAllocationID != nil {
			if info, ok := ipv4EIPs[*inst.IPv4EIPAllocationID]; ok {
				if info.Alias != "" {
					extra.IPv4Addr = info.Alias
				} else {
					extra.IPv4Addr = info.IP
				}
			}
		}
		// IPv6 显示逻辑：EIP 别名 > EIP 本身
		if inst.IPv6EIPAllocationID != nil {
			if info, ok := ipv6EIPs[*inst.IPv6EIPAllocationID]; ok {
				if info.Alias != "" {
					extra.IPv6Addr = info.Alias
				} else {
					extra.IPv6Addr = info.IP
				}
			}
		}

		if m, ok := metricsMap[inst.ID]; ok {
			extra.Metrics = m
		}
		result = append(result, extra)
	}

	c.JSON(http.StatusOK, gin.H{
		"data":     result,
		"total":    total,
		"page":     page,
		"per_page": perPage,
		"counts": gin.H{
			"all":      allCount,
			"running":  runningCount,
			"stopped":  stoppedCount,
			"creating": creatingCount,
			"error":    errorCount,
		},
	})
}

// BatchUserInstanceMetrics 批量获取用户实例的最新监控指标
func BatchUserInstanceMetrics(c *gin.Context) {
	userID := middleware.GetUserID(c)

	// 查询用户所有运行中的实例
	var instances []models.Instance
	if err := db.DB.Where("user_id = ? AND status = ?", userID, models.InstanceStatusRunning).Find(&instances).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	type metricsItem struct {
		InstanceID  uuid.UUID `json:"instance_id"`
		CPUUsage    float64   `json:"cpu_usage"`
		MemoryUsed  int64     `json:"memory_used"`
		MemoryTotal int64     `json:"memory_total"`
		DiskUsed    int64     `json:"disk_used"`
		DiskTotal   int64     `json:"disk_total"`
		NetInBps    int64     `json:"net_in_bps"`
		NetOutBps   int64     `json:"net_out_bps"`
		NetInTotal  int64     `json:"net_in_total"`
		NetOutTotal int64     `json:"net_out_total"`
	}

	result := make([]metricsItem, 0, len(instances))
	for _, inst := range instances {
		mp, err := monitor.GetInstanceLatestMetrics(inst.ID)
		if err != nil || mp == nil {
			result = append(result, metricsItem{InstanceID: inst.ID})
			continue
		}
		result = append(result, metricsItem{
			InstanceID:  inst.ID,
			CPUUsage:    mp.CPU,
			MemoryUsed:  (mp.MemUsed + (int64(inst.MemoryMB) - mp.MemTotal)) * 1024 * 1024,
			MemoryTotal: int64(inst.MemoryMB) * 1024 * 1024,
			DiskUsed:    (mp.DiskUsed + (int64(inst.DiskMB) - mp.DiskTotal)) * 1024 * 1024,
			DiskTotal:   int64(inst.DiskMB) * 1024 * 1024,
			NetInBps:    mp.NetIn,
			NetOutBps:   mp.NetOut,
			NetInTotal:  mp.NetInTotal,
			NetOutTotal: mp.NetOutTotal,
		})
	}

	c.JSON(http.StatusOK, gin.H{"items": result})
}

// GetUserInstance 获取用户实例详情
func GetUserInstance(c *gin.Context) {
	userID := middleware.GetUserID(c)
	instanceID := c.Param("id")

	var inst models.Instance
	if err := db.DB.Where("id = ? AND user_id = ?", instanceID, userID).First(&inst).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}

	// 加载关联数据
	db.DB.Model(&inst).Association("DataDisks").Find(&inst.DataDisks)
	db.DB.Model(&inst).Association("PortMappings").Find(&inst.PortMappings)

	resp := gin.H{
		"id":                 inst.ID.String(),
		"name":               inst.Name,
		"type":               inst.Type,
		"status":             inst.Status,
		"node_id":            inst.NodeID.String(),
		"user_id":            inst.UserID,
		"incus_name":         inst.IncusName,
		"template_id":        inst.TemplateID,
		"vcpu":               inst.VCPU,
		"memory_mb":          inst.MemoryMB,
		"swap_mb":            inst.SwapMB,
		"disk_mb":            inst.DiskMB,
		"storage_pool":       inst.StoragePool,
		"internal_ipv4":      inst.InternalIPv4,
		"internal_ipv6":      inst.InternalIPv6,
		"login_method":       inst.LoginMethod,
		"ssh_port":           inst.SSHPort,
		"ssh_password":       inst.SSHPassword,
		"ssh_public_key":     inst.SSHPublicKey,
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
		"data_disks":         inst.DataDisks,
		"port_mappings":      inst.PortMappings,
		"expires_at":         inst.ExpiresAt,
		"created_at":         inst.CreatedAt,
		"ipv4_mode":          inst.IPv4Mode,
		"ipv6_mode":          inst.IPv6Mode,
	}

	// 加载节点名称
	var node models.Node
	if err := db.DB.Where("id = ?", inst.NodeID).First(&node).Error; err == nil {
		resp["node_name"] = node.Name
	}

	// 加载 EIP 地址
	if inst.IPv4EIPAllocationID != nil {
		var alloc models.EIPAllocation
		if err := db.DB.Where("id = ?", *inst.IPv4EIPAllocationID).First(&alloc).Error; err == nil {
			resp["ipv4_eip"] = stripCIDRMask(alloc.CIDR)
			if alloc.Alias != "" {
				resp["ipv4_eip_alias"] = alloc.Alias
			}
		}
	}
	if inst.IPv6EIPAllocationID != nil {
		var alloc models.EIPAllocation
		if err := db.DB.Where("id = ?", *inst.IPv6EIPAllocationID).First(&alloc).Error; err == nil {
			resp["ipv6_eip"] = stripCIDRMask(alloc.CIDR)
			if alloc.Alias != "" {
				resp["ipv6_eip_alias"] = alloc.Alias
			}
		}
	}

	// 加载 Bridge 信息
	if inst.BridgeID != nil {
		resp["bridge_id"] = inst.BridgeID.String()
		var bridge models.Bridge
		if err := db.DB.Where("id = ?", *inst.BridgeID).First(&bridge).Error; err == nil {
			resp["bridge_name"] = bridge.Name
			resp["bridge_iface"] = bridge.BridgeName
			resp["bridge_cidr"] = bridge.IPv4CIDR
			resp["bridge_gateway"] = bridge.IPv4Gateway
		}
	}

	resp["has_eip"] = inst.IPv4EIPAllocationID != nil || inst.IPv6EIPAllocationID != nil

	c.JSON(http.StatusOK, resp)
}

// StartUserInstance 启动用户实例
func StartUserInstance(c *gin.Context) {
	userID := middleware.GetUserID(c)
	instanceID := c.Param("id")

	var inst models.Instance
	if err := db.DB.Where("id = ? AND user_id = ?", instanceID, userID).First(&inst).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}

	task, err := instanceService.StartInstance(inst.ID, userID)
	if err != nil {
		HandleServiceErrorWithFallback(c, err, http.StatusInternalServerError, "启动实例失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "启动指令已发送", "task_id": task.ID})
}

// StopUserInstance 停止用户实例
func StopUserInstance(c *gin.Context) {
	userID := middleware.GetUserID(c)
	instanceID := c.Param("id")

	var inst models.Instance
	if err := db.DB.Where("id = ? AND user_id = ?", instanceID, userID).First(&inst).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}

	task, err := instanceService.StopInstance(inst.ID, userID)
	if err != nil {
		HandleServiceErrorWithFallback(c, err, http.StatusInternalServerError, "停止实例失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "停止指令已发送", "task_id": task.ID})
}

// RestartUserInstance 重启用户实例
func RestartUserInstance(c *gin.Context) {
	userID := middleware.GetUserID(c)
	instanceID := c.Param("id")

	var inst models.Instance
	if err := db.DB.Where("id = ? AND user_id = ?", instanceID, userID).First(&inst).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}

	task, err := instanceService.RestartInstance(inst.ID, userID)
	if err != nil {
		HandleServiceErrorWithFallback(c, err, http.StatusInternalServerError, "重启实例失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "重启指令已发送", "task_id": task.ID})
}

// GetUserInstanceConsole 获取用户实例控制台凭据
func GetUserInstanceConsole(c *gin.Context) {
	userID := middleware.GetUserID(c)
	instanceID := c.Param("id")

	var inst models.Instance
	if err := db.DB.Where("id = ? AND user_id = ?", instanceID, userID).First(&inst).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}

	// 复用现有 GetInstanceConsole 逻辑
	c.Set("instance_id", inst.ID)
	// 直接调用现有 handler
	GetInstanceConsole(c)
}

// ListUserSnapshots 获取用户实例的快照列表
func ListUserSnapshots(c *gin.Context) {
	userID := middleware.GetUserID(c)
	instanceID := c.Param("id")

	var inst models.Instance
	if err := db.DB.Where("id = ? AND user_id = ?", instanceID, userID).First(&inst).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}

	// 复用现有 handler
	ListSnapshots(c)
}

// CreateUserSnapshot 创建用户实例快照
func CreateUserSnapshot(c *gin.Context) {
	userID := middleware.GetUserID(c)
	instanceID := c.Param("id")

	var inst models.Instance
	if err := db.DB.Where("id = ? AND user_id = ?", instanceID, userID).First(&inst).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}

	CreateSnapshot(c)
}

// GetUserInstanceMetrics 获取用户实例监控数据
func GetUserInstanceMetrics(c *gin.Context) {
	userID := middleware.GetUserID(c)
	instanceID := c.Param("id")

	var inst models.Instance
	if err := db.DB.Where("id = ? AND user_id = ?", instanceID, userID).First(&inst).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}

	GetInstanceMetrics(c)
}

// GetUserInstanceMetricsHistory 获取用户实例监控历史数据
func GetUserInstanceMetricsHistory(c *gin.Context) {
	userID := middleware.GetUserID(c)
	instanceID := c.Param("id")

	var inst models.Instance
	if err := db.DB.Where("id = ? AND user_id = ?", instanceID, userID).First(&inst).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}

	GetInstanceMetricsHistory(c)
}

// ResetUserInstancePassword 重置用户实例 SSH 密码
func ResetUserInstancePassword(c *gin.Context) {
	userID := middleware.GetUserID(c)
	instanceID := c.Param("id")

	var inst models.Instance
	if err := db.DB.Where("id = ? AND user_id = ?", instanceID, userID).First(&inst).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}

	ResetInstancePassword(c)
}

// ReinstallUserInstance 重装用户实例
func ReinstallUserInstance(c *gin.Context) {
	userID := middleware.GetUserID(c)
	instanceID := c.Param("id")

	var inst models.Instance
	if err := db.DB.Where("id = ? AND user_id = ?", instanceID, userID).First(&inst).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}

	ReinstallInstance(c)
}

// DeleteUserInstance 删除用户实例
func DeleteUserInstance(c *gin.Context) {
	userID := middleware.GetUserID(c)
	instanceID := c.Param("id")

	var inst models.Instance
	if err := db.DB.Where("id = ? AND user_id = ?", instanceID, userID).First(&inst).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}

	DeleteInstance(c)
}

// RestoreUserSnapshot 恢复用户实例快照
func RestoreUserSnapshot(c *gin.Context) {
	userID := middleware.GetUserID(c)
	instanceID := c.Param("id")

	var inst models.Instance
	if err := db.DB.Where("id = ? AND user_id = ?", instanceID, userID).First(&inst).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}

	RestoreSnapshot(c)
}

// DeleteUserSnapshot 删除用户实例快照
func DeleteUserSnapshot(c *gin.Context) {
	userID := middleware.GetUserID(c)
	instanceID := c.Param("id")

	var inst models.Instance
	if err := db.DB.Where("id = ? AND user_id = ?", instanceID, userID).First(&inst).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}

	DeleteSnapshot(c)
}

// GetUserInstanceVNC 获取用户实例 VNC 控制台凭据
func GetUserInstanceVNC(c *gin.Context) {
	userID := middleware.GetUserID(c)
	instanceID := c.Param("id")

	var inst models.Instance
	if err := db.DB.Where("id = ? AND user_id = ?", instanceID, userID).First(&inst).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}

	GetInstanceConsole(c)
}

// GetCurrentUser 获取当前用户信息
func GetCurrentUser(c *gin.Context) {
	user := middleware.GetUser(c)
	c.JSON(http.StatusOK, gin.H{
		"id":            user.ID,
		"username":      user.Username,
		"email":         user.Email,
		"status":        string(user.Status),
		"balance_cents": user.BalanceCents,
	})
}
