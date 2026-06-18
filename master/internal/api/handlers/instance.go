package handlers

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/monitor"
)

// DataDiskRequest 数据磁盘请求
type DataDiskRequest struct {
	Name        string `json:"name" binding:"required"`
	SizeGB      int    `json:"size_gb" binding:"required,min=1"`
	StoragePool string `json:"storage_pool,omitempty"`
	MountPoint  string `json:"mount_point,omitempty"`
}

// NATRequest NAT 请求
type NATRequest struct {
	InternalIP   string `json:"internal_ip" binding:"required,ip"`
	ExternalIP   string `json:"external_ip,omitempty"`
	InternalPort int    `json:"internal_port,omitempty"`
	ExternalPort int    `json:"external_port,omitempty"`
	Protocol     string `json:"protocol,omitempty"`
	Description  string `json:"description,omitempty"`
}

// CreateInstanceRequest 创建实例请求
type CreateInstanceRequest struct {
	Name               string            `json:"name" binding:"required"`
	Type               string            `json:"type" binding:"required,oneof=container vm"`
	TemplateID         string            `json:"template_id" binding:"required"`
	NodeID             string            `json:"node_id" binding:"required"`
	VPCID              string            `json:"vpc_id,omitempty"`
	AssignToUserID     uint              `json:"assign_to_user_id" binding:"required"`
	LoginMethod        string            `json:"login_method" binding:"required,oneof=auto password sshkey"`
	VCPU               float64           `json:"vcpu" binding:"required,min=0.1"`
	MemoryMB           int               `json:"memory_mb" binding:"required,min=64"`
	DiskGB             int               `json:"disk_gb" binding:"required,min=1"`
	StoragePool        string            `json:"storage_pool,omitempty"`
	DataDisks          []DataDiskRequest `json:"data_disks,omitempty"`
	PublicIPv4ID       string            `json:"public_ipv4_id,omitempty"`
	PublicIPv6PrefixID string            `json:"public_ipv6_prefix_id,omitempty"`
	NATs               []NATRequest      `json:"nats,omitempty"`
	AssignNAT          *bool             `json:"assign_nat,omitempty"`
	PortMappingCount   int               `json:"port_mapping_count,omitempty"`
	ExtraPorts         []int             `json:"extra_ports,omitempty"`
	AssignIPv4         bool              `json:"assign_ipv4,omitempty"`
	IPv4Count          int               `json:"ipv4_count,omitempty"`
	AssignIPv6         bool              `json:"assign_ipv6,omitempty"`
	IPv6Count          int               `json:"ipv6_count,omitempty"`
	NetworkDownMbps    int               `json:"network_down_mbps,omitempty"`
	NetworkUpMbps      int               `json:"network_up_mbps,omitempty"`
	IOReadMBps         int               `json:"io_read_mbps,omitempty"`
	IOWriteMBps        int               `json:"io_write_mbps,omitempty"`
	SSHPassword        string            `json:"ssh_password,omitempty"`
	SSHPublicKey       string            `json:"ssh_public_key,omitempty"`
	MonthlyTrafficGB   int64             `json:"monthly_traffic_gb,omitempty"`
	TrafficMode        string            `json:"traffic_mode,omitempty"`
	SnapshotLimit      int               `json:"snapshot_limit,omitempty"`
	ExpiresAt          *time.Time        `json:"expires_at,omitempty"`
}

// CreateInstance 创建实例
func CreateInstance(c *gin.Context) {
	var req CreateInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	nodeID, err := uuid.Parse(req.NodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}

	// 检查节点是否存在且在线
	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
		return
	}

	if !node.IsHealthy() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "节点离线"})
		return
	}

	// 检查镜像是否已在节点下载（必须先下载才能创建）
	var nodeImage models.NodeImage
	if err := db.DB.Where("node_id = ? AND image_id = ? AND status = ?", nodeID, req.TemplateID, "downloaded").First(&nodeImage).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "镜像未下载，请先下载镜像"})
		return
	}

	// 检查 VPC 是否存在且属于该节点
	var vpc *models.VPCNetwork
	var internalIPv4 string
	if req.VPCID != "" {
		vpcID, err := uuid.Parse(req.VPCID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 VPC ID"})
			return
		}
		var vpcNet models.VPCNetwork
		if err := db.DB.Where("id = ? AND node_id = ?", vpcID, nodeID).First(&vpcNet).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "VPC 不存在或不属于该节点"})
			return
		}
		vpc = &vpcNet

		// 从 VPC 内网 CIDR 分配一个可用 IP（排除网关地址）
		ip, err := allocateIPFromVPC(vpc.ID, vpc.IPv4CIDR, vpc.DefaultGatewayV4)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "VPC 内网 IP 分配失败: " + err.Error()})
			return
		}
		internalIPv4 = ip
	}

	// 验证目标用户存在
	var targetUser models.User
	if err := db.DB.Where("id = ?", req.AssignToUserID).First(&targetUser).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "目标用户不存在"})
		return
	}

	// 检查是否已有同名实例（避免重复创建）
	var existingInstance models.Instance
	if err := db.DB.Where("name = ? AND node_id = ?", req.Name, nodeID).First(&existingInstance).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "该节点上已存在同名实例"})
		return
	}

	// 生成 Incus 内部名称
	instanceID := uuid.New()
	incusName := fmt.Sprintf("tsukiyo-%s", instanceID.String()[:8])

	// 处理登录方式
	loginMethod := models.LoginMethod(req.LoginMethod)
	sshPassword := req.SSHPassword
	if loginMethod == models.LoginMethodAuto {
		sshPassword = generateRandomPassword(16)
	}

	instance := models.Instance{
		ID:               instanceID,
		Name:             req.Name,
		UserID:           req.AssignToUserID,
		NodeID:           nodeID,
		Type:             models.InstanceType(req.Type),
		Status:           models.InstanceStatusCreating,
		IncusName:        incusName,
		TemplateID:       req.TemplateID,
		InternalIPv4:     internalIPv4,
		VCPU:             req.VCPU,
		MemoryMB:         req.MemoryMB,
		DiskGB:           req.DiskGB,
		StoragePool:      req.StoragePool,
		LoginMethod:      loginMethod,
		SSHPassword:      sshPassword,
		SSHPublicKey:     req.SSHPublicKey,
		NetworkDownMbps:  req.NetworkDownMbps,
		NetworkUpMbps:    req.NetworkUpMbps,
		IOReadMBps:       req.IOReadMBps,
		IOWriteMBps:      req.IOWriteMBps,
		MonthlyTrafficGB: req.MonthlyTrafficGB,
		SnapshotLimit:    req.SnapshotLimit,
	}
	if vpc != nil {
		instance.VPCID = &vpc.ID
	}

	if req.TrafficMode != "" {
		instance.TrafficMode = models.TrafficMode(req.TrafficMode)
	}
	if req.ExpiresAt != nil {
		instance.ExpiresAt = req.ExpiresAt
	}

	// 处理公网 IPv4
	if req.PublicIPv4ID != "" {
		ipv4ID, err := uuid.Parse(req.PublicIPv4ID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的公网 IPv4 ID"})
			return
		}
		var ip models.PublicIPPool
		if err := db.DB.Where("id = ? AND node_id = ? AND status = ?", ipv4ID, nodeID, models.IPStatusFree).First(&ip).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "公网 IPv4 不可用或已被分配"})
			return
		}
		instance.PublicIPv4ID = &ipv4ID
		instance.IPv4Address = &ip.Address
		// 标记 IP 为已分配
		now := time.Now()
		db.DB.Model(&ip).Updates(map[string]interface{}{
			"status":      models.IPStatusAssigned,
			"instance_id": instanceID,
			"assigned_at": &now,
		})
	}

	// 处理公网 IPv6 前缀
	if req.PublicIPv6PrefixID != "" {
		ipv6ID, err := uuid.Parse(req.PublicIPv6PrefixID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 IPv6 前缀 ID"})
			return
		}
		var prefix models.IPv6Prefix
		if err := db.DB.Where("id = ? AND node_id = ?", ipv6ID, nodeID).First(&prefix).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "IPv6 前缀不存在"})
			return
		}
		instance.PublicIPv6PrefixID = &ipv6ID
		instance.IPv6Address = &prefix.Prefix
	}

	if err := db.DB.Create(&instance).Error; err != nil {
		zap.L().Error("创建实例失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建实例失败"})
		return
	}

	// 创建数据磁盘
	for _, dd := range req.DataDisks {
		disk := models.DataDisk{
			ID:          uuid.New(),
			InstanceID:  instanceID,
			NodeID:      nodeID,
			Name:        dd.Name,
			SizeGB:      dd.SizeGB,
			StoragePool: dd.StoragePool,
			MountPoint:  dd.MountPoint,
		}
		if disk.StoragePool == "" {
			disk.StoragePool = instance.StoragePool
		}
		db.DB.Create(&disk)
	}

	// 创建 NAT 配置
	for _, nat := range req.NATs {
		natCfg := models.NATConfig{
			ID:           uuid.New(),
			InstanceID:   instanceID,
			NodeID:       nodeID,
			InternalIP:   nat.InternalIP,
			ExternalIP:   nat.ExternalIP,
			InternalPort: nat.InternalPort,
			ExternalPort: nat.ExternalPort,
			Protocol:     nat.Protocol,
			Description:  nat.Description,
		}
		if natCfg.Protocol == "" {
			natCfg.Protocol = "tcp"
		}
		db.DB.Create(&natCfg)
	}

	// 确定 NAT 分配策略：默认开启 NAT（与 CLICD 一致）
	wantsNAT := true
	if req.AssignNAT != nil {
		wantsNAT = *req.AssignNAT
	}
	portMappingCount := req.PortMappingCount
	if wantsNAT && portMappingCount < 1 {
		portMappingCount = 2
	}
	if !wantsNAT {
		portMappingCount = 0
	}

	// 创建任务下发给 Agent
	taskPayload := map[string]interface{}{
		"instance_id":        instance.IncusName,
		"type":               instance.Type,
		"template_id":        instance.TemplateID,
		"vcpu":               instance.VCPU,
		"memory_mb":          instance.MemoryMB,
		"disk_gb":            instance.DiskGB,
		"storage_pool":       instance.StoragePool,
		"login_method":       instance.LoginMethod,
		"ssh_password":       instance.SSHPassword,
		"ssh_public_key":     instance.SSHPublicKey,
		"network_down":       instance.NetworkDownMbps,
		"network_up":         instance.NetworkUpMbps,
		"io_read":            instance.IOReadMBps,
		"io_write":           instance.IOWriteMBps,
		"data_disks":         req.DataDisks,
		"nats":               req.NATs,
		"assign_nat":         wantsNAT,
		"port_mapping_count": portMappingCount,
		"traffic_mode":       instance.TrafficMode,
		"monthly_traffic":    instance.MonthlyTrafficGB,
		"snapshot_limit":     instance.SnapshotLimit,
	}
	if vpc != nil {
		taskPayload["vpc_id"] = vpc.ID.String()
		taskPayload["internal_ipv4"] = internalIPv4
		taskPayload["gateway_v4"] = vpc.DefaultGatewayV4
		taskPayload["ipv4_cidr"] = vpc.IPv4CIDR
		taskPayload["bridge_name"] = vpc.GetBridgeName()
		taskPayload["ipv4_filter"] = vpc.IPv4Filter
		taskPayload["mac_filter"] = vpc.MACFilter
		egressV4 := vpc.EgressV4Primary
		if idx := strings.Index(egressV4, "/"); idx > 0 {
			egressV4 = egressV4[:idx]
		}
		taskPayload["egress_v4_primary"] = egressV4
		taskPayload["parent_iface"] = vpc.ParentIface
	}

	payloadBytes, _ := json.Marshal(taskPayload)
	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeCreateInstance,
		NodeID:     nodeID,
		InstanceID: &instance.ID,
		UserID:     req.AssignToUserID,
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
		zap.L().Error("创建任务失败", zap.Error(err))
	}

	ipv4 := ""
	if instance.IPv4Address != nil {
		ipv4 = *instance.IPv4Address
	}
	ipv6 := ""
	if instance.IPv6Address != nil {
		ipv6 = *instance.IPv6Address
	}
	respData := gin.H{
		"id":            instance.ID.String(),
		"name":          instance.Name,
		"incus_name":    instance.IncusName,
		"status":        instance.Status,
		"node_id":       instance.NodeID.String(),
		"internal_ipv4": instance.InternalIPv4,
		"ipv4_address":  ipv4,
		"ipv6_address":  ipv6,
		"ssh_password":  instance.SSHPassword,
		"task_id":       task.ID.String(),
	}
	if vpc != nil {
		respData["vpc_id"] = vpc.ID.String()
		respData["vpc_name"] = vpc.Name
	}
	c.JSON(http.StatusCreated, respData)
}

// ListInstances 获取实例列表
func ListInstances(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	var instances []models.Instance
	if err := db.DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&instances).Error; err != nil {
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

	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	// 加载关联数据
	db.DB.Model(&instance).Association("DataDisks").Find(&instance.DataDisks)
	db.DB.Model(&instance).Association("NATConfigs").Find(&instance.NATConfigs)
	db.DB.Model(&instance).Association("PortMappings").Find(&instance.PortMappings)

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
		// 加载 VPC 详情
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

	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.VCPU > 0 {
		updates["vcpu"] = req.VCPU
	}
	if req.MemoryMB > 0 {
		updates["memory_mb"] = req.MemoryMB
	}
	if req.DiskGB > 0 {
		updates["disk_gb"] = req.DiskGB
	}
	if req.NetworkDownMbps > 0 {
		updates["network_down_mbps"] = req.NetworkDownMbps
	}
	if req.NetworkUpMbps > 0 {
		updates["network_up_mbps"] = req.NetworkUpMbps
	}
	if req.ExpiresAt != nil {
		updates["expires_at"] = *req.ExpiresAt
	}

	if len(updates) > 0 {
		if err := db.DB.Model(&instance).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
			return
		}
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

	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}

	// 创建删除任务（保存原状态以便失败后恢复）
	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"instance_id": instance.IncusName,
		"old_status":  string(instance.Status),
	})
	userID, _ := c.Get("user_id")
	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeDeleteInstance,
		NodeID:     instance.NodeID,
		InstanceID: &instance.ID,
		UserID:     userID.(uint),
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}
	db.DB.Create(&task)

	// 更新实例状态
	db.DB.Model(&instance).Update("status", models.InstanceStatusDeleting)

	c.JSON(http.StatusOK, gin.H{
		"message": "删除任务已创建",
		"task_id": task.ID.String(),
	})
}

// StartInstance 启动实例
func StartInstance(c *gin.Context) {
	sendInstanceTask(c, models.TaskTypeStartInstance)
}

// StopInstance 停止实例
func StopInstance(c *gin.Context) {
	sendInstanceTask(c, models.TaskTypeStopInstance)
}

// RestartInstance 重启实例
func RestartInstance(c *gin.Context) {
	sendInstanceTask(c, models.TaskTypeRestartInstance)
}

// ReinstallInstance 重装实例
func ReinstallInstance(c *gin.Context) {
	sendInstanceTask(c, models.TaskTypeReinstallInstance)
}

// ResizeInstance 升降配
func ResizeInstance(c *gin.Context) {
	sendInstanceTask(c, models.TaskTypeResizeInstance)
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

	// 更新数据库中的密码
	if err := db.DB.Model(&instance).Update("ssh_password", req.Password).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新密码失败"})
		return
	}

	// 下发密码重置任务
	payload := map[string]interface{}{
		"instance_id": instance.IncusName,
		"password":    req.Password,
	}
	payloadBytes, _ := json.Marshal(payload)

	userID, _ := c.Get("user_id")
	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeResizeInstance,
		NodeID:     instance.NodeID,
		InstanceID: &instance.ID,
		UserID:     userID.(uint),
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}
	db.DB.Create(&task)

	c.JSON(http.StatusOK, gin.H{
		"message": "密码重置任务已下发",
		"task_id": task.ID.String(),
	})
}

// GetInstanceConsole 获取控制台 URL
func GetInstanceConsole(c *gin.Context) {
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

	consoleType := c.Query("type")
	if consoleType == "" {
		consoleType = "vnc"
	}

	if consoleType != "vnc" && consoleType != "ssh" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的控制台类型"})
		return
	}

	// 生成一次性 Token
	consoleToken := uuid.New().String()

	// 控制台会话写入 Redis (5分钟有效)
	ctx := c.Request.Context()
	sessionKey := "console:" + consoleToken
	sessionData, _ := json.Marshal(map[string]interface{}{
		"instance_id": instance.ID.String(),
		"node_id":     instance.NodeID.String(),
		"type":        consoleType,
		"incus_name":  instance.IncusName,
	})
	db.RedisClient.Set(ctx, sessionKey, sessionData, 5*time.Minute)

	c.JSON(http.StatusOK, gin.H{
		"type":        consoleType,
		"token":       consoleToken,
		"instance_id": instance.ID.String(),
		"incus_name":  instance.IncusName,
		"node_id":     instance.NodeID.String(),
		"expires_in":  300,
	})
}

// sendInstanceTask 发送实例操作任务
func sendInstanceTask(c *gin.Context, taskType models.TaskType) {
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

	// 创建任务
	payloadBytes, _ := json.Marshal(map[string]string{
		"instance_id": instance.IncusName,
	})
	userID, _ := c.Get("user_id")
	task := models.Task{
		ID:         uuid.New(),
		Type:       taskType,
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
		"message": "任务已创建",
		"task_id": task.ID.String(),
	})
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

// allocateIPFromVPC 从 VPC 的内网 CIDR 中分配一个可用 IP
// 使用数据库级别的并发安全：INSERT IPPoolEntry 行，状态为 allocated
// gateway: VPC 的网关地址，必须排除
func allocateIPFromVPC(vpcID uuid.UUID, cidr string, gateway string) (string, error) {
	// 解析 CIDR
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", fmt.Errorf("invalid CIDR %s: %w", cidr, err)
	}

	// 获取该 VPC 下已分配的所有 IP
	var allocated []models.IPPoolEntry
	if err := db.DB.Where("owner_id = ? AND pool_type = ? AND status = ?", vpcID, "vpc_internal", "allocated").
		Select("address").Find(&allocated).Error; err != nil {
		return "", fmt.Errorf("查询已分配 IP 失败: %w", err)
	}

	used := make(map[string]bool)
	for _, entry := range allocated {
		used[entry.Address] = true
	}
	// 排除网关地址（无论网关是什么值）
	if gateway != "" {
		used[gateway] = true
	}

	// 遍历网段找第一个可用 IP（排除网络地址、广播地址、网关、已分配）
	ip := ipNet.IP.Mask(ipNet.Mask)
	for ipNet.Contains(ip) {
		ipStr := ip.String()
		// 跳过网络地址、广播地址、已分配（含网关）
		if !used[ipStr] && isAssignableIP(ip, ipNet) {
			// 尝试在数据库中标记为已分配
			entry := models.IPPoolEntry{
				PoolType:  "vpc_internal",
				OwnerID:   vpcID,
				Address:   ipStr,
				PrefixLen: getPrefixLen(ipNet),
				Status:    "allocated",
			}
			if err := db.DB.Create(&entry).Error; err == nil {
				return ipStr, nil
			}
			// 创建失败（可能是并发冲突），继续尝试下一个
		}
		incrementIP(ip)
	}

	return "", fmt.Errorf("VPC %s 内无可用 IP", cidr)
}

// isAssignableIP 判断 IP 是否可分配（排除网络地址、广播地址）
func isAssignableIP(ip net.IP, ipNet *net.IPNet) bool {
	// 网络地址
	network := ip.Mask(ipNet.Mask)
	if ip.Equal(network) {
		return false
	}
	// 广播地址
	broadcast := make(net.IP, len(network))
	copy(broadcast, network)
	for i := range broadcast {
		broadcast[i] |= ^ipNet.Mask[i]
	}
	if ip.Equal(broadcast) {
		return false
	}
	return true
}

// getPrefixLen 获取前缀长度
func getPrefixLen(ipNet *net.IPNet) int {
	ones, _ := ipNet.Mask.Size()
	return ones
}

// incrementIP 将 IP 地址加 1
func incrementIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
}

// generateRandomPassword 生成随机密码
func generateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}
