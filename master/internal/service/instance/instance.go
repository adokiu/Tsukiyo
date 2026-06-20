package instance

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"tsukiyo/master/internal/agent"
	"tsukiyo/master/internal/console"
	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/service"
	"tsukiyo/master/internal/service/infrastructure"
)

var (
	ErrInstanceNotFound   = service.ErrInstanceNotFound
	ErrNodeNotFound       = service.ErrNodeNotFound
	ErrNodeOffline        = service.ErrNodeOffline
	ErrInvalidNodeID      = service.ErrInvalidNodeID
	ErrImageNotDownloaded = service.ErrImageNotDownloaded
	ErrInvalidBridgeID    = service.ErrInvalidBridgeID
	ErrBridgeNotFound     = service.ErrBridgeNotFound
	ErrUserNotFound       = service.ErrUserNotFound
	ErrInstanceNameExists = service.ErrInstanceNameExists
	ErrInstanceNoBridge   = service.ErrInstanceNoBridge
	ErrNoBridgeEgressIP   = service.ErrNoBridgeEgressIP
)

// InstanceService 实例服务
type InstanceService struct {
	networkSvc *infrastructure.NetworkService
	agentMgr   *agent.Manager
}

// NewInstanceService 创建实例服务
func NewInstanceService(networkSvc *infrastructure.NetworkService, agentMgr *agent.Manager) *InstanceService {
	return &InstanceService{networkSvc: networkSvc, agentMgr: agentMgr}
}

// DataDiskRequest 数据磁盘请求
type DataDiskRequest struct {
	Name        string `json:"name" binding:"required"`
	SizeGB      int    `json:"size_gb" binding:"required,min=1"`
	StoragePool string `json:"storage_pool,omitempty"`
	MountPoint  string `json:"mount_point,omitempty"`
}

// CreateInstanceRequest 创建实例请求
type CreateInstanceRequest struct {
	Name             string            `json:"name" binding:"required"`
	Type             string            `json:"type" binding:"required,oneof=container vm"`
	TemplateID       string            `json:"template_id" binding:"required"`
	ImageKey         string            `json:"image_key,omitempty"`
	NodeID           string            `json:"node_id" binding:"required"`
	BridgeID         string            `json:"bridge_id,omitempty"`
	AssignToUserID   uint              `json:"assign_to_user_id" binding:"required"`
	LoginMethod      string            `json:"login_method" binding:"required,oneof=auto password sshkey"`
	VCPU             float64           `json:"vcpu" binding:"required,min=0.1"`
	MemoryMB         int               `json:"memory_mb" binding:"required,min=64"`
	DiskGB           int               `json:"disk_gb" binding:"required,min=1"`
	StoragePool      string            `json:"storage_pool,omitempty"`
	DataDisks        []DataDiskRequest `json:"data_disks,omitempty"`
	AssignEIPv4      bool              `json:"assign_eip_ipv4,omitempty"`
	AssignEIPv6      bool              `json:"assign_eip_ipv6,omitempty"`
	PortMappingCount int               `json:"port_mapping_count,omitempty"`
	ExtraPorts       []int             `json:"extra_ports,omitempty"`
	NetworkDownMbps  int               `json:"network_down_mbps,omitempty"`
	NetworkUpMbps    int               `json:"network_up_mbps,omitempty"`
	IOReadMBps       int               `json:"io_read_mbps,omitempty"`
	IOWriteMBps      int               `json:"io_write_mbps,omitempty"`
	SSHPassword      string            `json:"ssh_password,omitempty"`
	SSHPublicKey     string            `json:"ssh_public_key,omitempty"`
	MonthlyTrafficGB int64             `json:"monthly_traffic_gb,omitempty"`
	TrafficMode      string            `json:"traffic_mode,omitempty"`
	SnapshotLimit    int               `json:"snapshot_limit,omitempty"`
	ExpiresAt        *time.Time        `json:"expires_at,omitempty"`
}

// CreateInstance 创建实例
func (s *InstanceService) CreateInstance(req CreateInstanceRequest) (*models.Instance, *models.Task, error) {
	nodeID, err := uuid.Parse(req.NodeID)
	if err != nil {
		return nil, nil, ErrInvalidNodeID
	}

	// 检查节点是否存在且在线
	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		return nil, nil, ErrNodeNotFound
	}

	if !node.IsHealthy() {
		return nil, nil, ErrNodeOffline
	}

	// 检查镜像是否已下载
	imageIDForCheck := req.ImageKey
	if imageIDForCheck == "" {
		imageIDForCheck = req.TemplateID
	}
	var nodeImage models.NodeImage
	if err := db.DB.Where("node_id = ? AND image_id = ? AND status = ?", nodeID, imageIDForCheck, "downloaded").First(&nodeImage).Error; err != nil {
		if err2 := db.DB.Where("node_id = ? AND image_id LIKE ? AND status = ?", nodeID, req.TemplateID+"|%", "downloaded").First(&nodeImage).Error; err2 != nil {
			return nil, nil, ErrImageNotDownloaded
		}
	}

	// 检查网桥
	var bridge *models.Bridge
	var internalIPv4 string
	var internalIPv6 string
	if req.BridgeID != "" {
		bridgeID, err := uuid.Parse(req.BridgeID)
		if err != nil {
			return nil, nil, ErrInvalidBridgeID
		}
		var bridgeNet models.Bridge
		if err := db.DB.Where("id = ? AND node_id = ?", bridgeID, nodeID).First(&bridgeNet).Error; err != nil {
			return nil, nil, ErrBridgeNotFound
		}
		bridge = &bridgeNet

		if bridge.IPv4Enabled {
			ip, err := s.networkSvc.AllocateInternalIP(bridge.ID, nodeID, bridge.IPv4CIDR, bridge.IPv4Gateway, "ipv4")
			if err != nil {
				return nil, nil, fmt.Errorf("网桥内网 IPv4 分配失败: %w", err)
			}
			internalIPv4 = ip
		}
		if bridge.IPv6Enabled {
			ip, err := s.networkSvc.AllocateInternalIP(bridge.ID, nodeID, bridge.IPv6CIDR, bridge.IPv6Gateway, "ipv6")
			if err == nil {
				internalIPv6 = ip
			}
		}
	}

	// 验证目标用户存在
	var targetUser models.User
	if err := db.DB.Where("id = ?", req.AssignToUserID).First(&targetUser).Error; err != nil {
		return nil, nil, ErrUserNotFound
	}

	// 检查同名实例
	var existingInstance models.Instance
	if err := db.DB.Where("name = ? AND node_id = ?", req.Name, nodeID).First(&existingInstance).Error; err == nil {
		return nil, nil, ErrInstanceNameExists
	}

	// 生成 Incus 内部名称
	instanceID := uuid.New()
	incusName := fmt.Sprintf("tsukiyo-%s", instanceID.String()[:8])

	// 处理登录方式
	loginMethod := models.LoginMethod(req.LoginMethod)
	sshPassword := req.SSHPassword
	if loginMethod == models.LoginMethodAuto {
		sshPassword = GenerateRandomPassword(16)
	}

	portMappingLimit := req.PortMappingCount
	if portMappingLimit <= 0 {
		portMappingLimit = 2
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
		InternalIPv6:     internalIPv6,
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
		PortMappingLimit: portMappingLimit,
	}
	if bridge != nil {
		instance.BridgeID = &bridge.ID
	}

	if req.TrafficMode != "" {
		instance.TrafficMode = models.TrafficMode(req.TrafficMode)
	}
	if req.ExpiresAt != nil {
		instance.ExpiresAt = req.ExpiresAt
	}

	// 分配 EIP
	if req.AssignEIPv4 && bridge != nil {
		alloc, err := s.networkSvc.AllocateEIP(nodeID, "ipv4", 32, "")
		if err == nil {
			instance.IPv4EIPAllocationID = &alloc.ID
			instance.IPv4Mode = "eip"
		} else {
			zap.L().Warn("分配 IPv4 EIP 失败（非致命）", zap.Error(err))
		}
	}
	if req.AssignEIPv6 && bridge != nil {
		alloc, err := s.networkSvc.AllocateEIP(nodeID, "ipv6", 128, "")
		if err == nil {
			instance.IPv6EIPAllocationID = &alloc.ID
			instance.IPv6Mode = "eip"
		} else {
			zap.L().Warn("分配 IPv6 EIP 失败（非致命）", zap.Error(err))
		}
	}

	if err := db.DB.Create(&instance).Error; err != nil {
		zap.L().Error("创建实例失败", zap.Error(err))
		return nil, nil, err
	}

	// 关联 EIP 分配记录
	if instance.IPv4EIPAllocationID != nil {
		s.networkSvc.AssignEIPToInstance(*instance.IPv4EIPAllocationID, instanceID)
	}
	if instance.IPv6EIPAllocationID != nil {
		s.networkSvc.AssignEIPToInstance(*instance.IPv6EIPAllocationID, instanceID)
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

	// Master 集中分配端口映射（创建实例时只自动分配 SSH）
	// 由 Master 分配端口，写入数据库后把完整映射列表下发给 Agent
	var assignedPortMappings []map[string]interface{}

	if bridge != nil {
		ipVersion := "ipv4"
		if bridge.IPv4Enabled {
			ipVersion = "ipv4"
		} else if bridge.IPv6Enabled {
			ipVersion = "ipv6"
		}
		mappings, err := s.networkSvc.AllocatePortMappingsForInstance(
			instanceID, bridge.ID, nodeID, 0, req.ExtraPorts, ipVersion,
		)
		if err != nil {
			zap.L().Warn("分配端口映射失败（非致命）", zap.Error(err))
		} else {
			for _, pm := range mappings {
				var egressAlloc models.EIPAllocation
				if err := db.DB.Where("id = ?", pm.EgressAllocationID).First(&egressAlloc).Error; err != nil {
					zap.L().Error("查询 NAT 出口 EIP 分配记录失败", zap.String("egress_alloc_id", pm.EgressAllocationID.String()), zap.Error(err))
					return nil, nil, fmt.Errorf("查询 NAT 出口 EIP 分配记录失败: %w", err)
				}
				hostIP := egressAlloc.GetIP()
				if hostIP == "" {
					return nil, nil, fmt.Errorf("NAT 出口 EIP 分配记录 CIDR 为空, alloc_id=%s", egressAlloc.ID.String())
				}
				assignedPortMappings = append(assignedPortMappings, map[string]interface{}{
					"host_port":      pm.HostPort,
					"container_port": pm.ContainerPort,
					"protocol":       pm.Protocol,
					"host_ip":        hostIP,
					"description":    pm.Description,
				})
			}
		}
	}

	taskPayload := map[string]interface{}{
		"instance_id":     instance.IncusName,
		"type":            instance.Type,
		"template_id":     instance.TemplateID,
		"vcpu":            instance.VCPU,
		"memory_mb":       instance.MemoryMB,
		"disk_gb":         instance.DiskGB,
		"storage_pool":    instance.StoragePool,
		"login_method":    instance.LoginMethod,
		"ssh_password":    instance.SSHPassword,
		"ssh_public_key":  instance.SSHPublicKey,
		"network_down":    instance.NetworkDownMbps,
		"network_up":      instance.NetworkUpMbps,
		"io_read":         instance.IOReadMBps,
		"io_write":        instance.IOWriteMBps,
		"data_disks":      req.DataDisks,
		"traffic_mode":    instance.TrafficMode,
		"monthly_traffic": instance.MonthlyTrafficGB,
		"snapshot_limit":  instance.SnapshotLimit,
		"port_mappings":   assignedPortMappings,
		"image_source":    node.ImageRemoteURL,
	}
	if bridge != nil {
		taskPayload["bridge_id"] = bridge.ID.String()
		taskPayload["bridge_name"] = bridge.BridgeName
		taskPayload["internal_ipv4"] = internalIPv4
		taskPayload["internal_ipv6"] = internalIPv6
		taskPayload["gateway_v4"] = bridge.IPv4Gateway
		taskPayload["gateway_v6"] = bridge.IPv6Gateway
		taskPayload["ipv4_cidr"] = bridge.IPv4CIDR
		taskPayload["ipv6_cidr"] = bridge.IPv6CIDR
		taskPayload["ipv4_enabled"] = bridge.IPv4Enabled
		taskPayload["ipv6_enabled"] = bridge.IPv6Enabled
		var dnsServers []string
		json.Unmarshal(bridge.DNSServers, &dnsServers)
		taskPayload["dns_servers"] = dnsServers
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
		zap.L().Error("创建实例任务失败", zap.Error(err))
	}

	return &instance, &task, nil
}

// GenerateRandomPassword 生成随机密码（crypto/rand 真随机，大小写字母+数字）
func GenerateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

// ListInstances 获取实例列表
func (s *InstanceService) ListInstances(userID uint) ([]models.Instance, error) {
	var instances []models.Instance
	if err := db.DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&instances).Error; err != nil {
		return nil, err
	}
	return instances, nil
}

// GetInstance 获取实例详情
func (s *InstanceService) GetInstance(instanceID uuid.UUID) (*models.Instance, error) {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}

	// 加载关联数据
	db.DB.Model(&instance).Association("DataDisks").Find(&instance.DataDisks)
	db.DB.Model(&instance).Association("PortMappings").Find(&instance.PortMappings)

	return &instance, nil
}

// UpdateInstance 更新实例
func (s *InstanceService) UpdateInstance(instanceID uuid.UUID, name string, vcpu float64, memoryMB, diskGB, networkDownMbps, networkUpMbps int, expiresAt *string) error {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrInstanceNotFound
		}
		return err
	}

	updates := make(map[string]interface{})
	if name != "" {
		updates["name"] = name
	}
	if vcpu > 0 {
		updates["vcpu"] = vcpu
	}
	if memoryMB > 0 {
		updates["memory_mb"] = memoryMB
	}
	if diskGB > 0 {
		updates["disk_gb"] = diskGB
	}
	if networkDownMbps > 0 {
		updates["network_down_mbps"] = networkDownMbps
	}
	if networkUpMbps > 0 {
		updates["network_up_mbps"] = networkUpMbps
	}
	if expiresAt != nil {
		updates["expires_at"] = *expiresAt
	}

	if len(updates) > 0 {
		if err := db.DB.Model(&instance).Updates(updates).Error; err != nil {
			return err
		}
	}

	return nil
}

// DeleteInstance 删除实例
func (s *InstanceService) DeleteInstance(instanceID uuid.UUID, userID uint) (*models.Task, error) {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}

	if instance.Status == models.InstanceStatusDeleting {
		return nil, service.ErrInstanceBusy
	}

	oldStatus := string(instance.Status)

	if err := db.DB.Model(&instance).Update("status", models.InstanceStatusDeleting).Error; err != nil {
		return nil, fmt.Errorf("更新实例状态为 deleting 失败: %w", err)
	}

	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"instance_id": instance.IncusName,
		"old_status":  oldStatus,
	})

	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeDeleteInstance,
		NodeID:     instance.NodeID,
		InstanceID: &instance.ID,
		UserID:     userID,
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
		db.DB.Model(&instance).Update("status", oldStatus)
		return nil, fmt.Errorf("创建删除实例任务失败: %w", err)
	}

	return &task, nil
}

// StartInstance 启动实例
func (s *InstanceService) StartInstance(instanceID uuid.UUID, userID uint) (*models.Task, error) {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}

	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"instance_id": instance.IncusName,
	})

	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeStartInstance,
		NodeID:     instance.NodeID,
		InstanceID: &instance.ID,
		UserID:     userID,
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
		zap.L().Error("创建启动实例任务失败", zap.Error(err))
	}

	return &task, nil
}

// StopInstance 停止实例
func (s *InstanceService) StopInstance(instanceID uuid.UUID, userID uint) (*models.Task, error) {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}

	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"instance_id": instance.IncusName,
	})

	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeStopInstance,
		NodeID:     instance.NodeID,
		InstanceID: &instance.ID,
		UserID:     userID,
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
		zap.L().Error("创建停止实例任务失败", zap.Error(err))
	}

	return &task, nil
}

// RestartInstance 重启实例
func (s *InstanceService) RestartInstance(instanceID uuid.UUID, userID uint) (*models.Task, error) {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}

	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"instance_id": instance.IncusName,
	})

	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeRestartInstance,
		NodeID:     instance.NodeID,
		InstanceID: &instance.ID,
		UserID:     userID,
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
		zap.L().Error("创建重启实例任务失败", zap.Error(err))
	}

	return &task, nil
}

// ReinstallInstanceRequest 重装请求参数
type ReinstallInstanceRequest struct {
	TemplateID  string `json:"template_id"`
	Password    string `json:"password"`
	SSHKey      string `json:"ssh_key"`
	LoginMethod string `json:"login_method"`
}

// ReinstallInstance 重装实例
func (s *InstanceService) ReinstallInstance(instanceID uuid.UUID, userID uint, req ReinstallInstanceRequest) (*models.Task, error) {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}

	if instance.Status == models.InstanceStatusReinstalling || instance.Status == models.InstanceStatusDeleting {
		return nil, service.ErrInstanceBusy
	}

	templateID := req.TemplateID
	if templateID == "" {
		templateID = instance.TemplateID
	}

	password := req.Password
	if password == "" {
		password = instance.SSHPassword
	}

	loginMethod := req.LoginMethod
	if loginMethod == "" {
		loginMethod = string(instance.LoginMethod)
	}

	oldStatus := string(instance.Status)

	if err := db.DB.Model(&instance).Update("status", models.InstanceStatusReinstalling).Error; err != nil {
		return nil, fmt.Errorf("更新实例状态为 reinstalling 失败: %w", err)
	}

	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"instance_id":  instance.IncusName,
		"template_id":  templateID,
		"password":     password,
		"ssh_key":      req.SSHKey,
		"login_method": loginMethod,
		"old_status":   oldStatus,
	})

	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeReinstallInstance,
		NodeID:     instance.NodeID,
		InstanceID: &instance.ID,
		UserID:     userID,
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
		db.DB.Model(&instance).Update("status", oldStatus)
		return nil, fmt.Errorf("创建重装实例任务失败: %w", err)
	}

	return &task, nil
}

// ResizeInstance 升降配
func (s *InstanceService) ResizeInstance(instanceID uuid.UUID, userID uint) (*models.Task, error) {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}

	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"instance_id": instance.IncusName,
	})

	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeResizeInstance,
		NodeID:     instance.NodeID,
		InstanceID: &instance.ID,
		UserID:     userID,
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
		zap.L().Error("创建升降配任务失败", zap.Error(err))
	}

	return &task, nil
}

// ResetInstancePassword 重置密码（同步调用 Agent，成功后才更新 DB）
func (s *InstanceService) ResetInstancePassword(instanceID uuid.UUID, password string) error {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrInstanceNotFound
		}
		return err
	}

	if s.agentMgr == nil {
		return service.ErrAgentManagerNotInitialized
	}

	if !s.agentMgr.IsNodeConnected(instance.NodeID) {
		return service.ErrNodeNotConnected
	}

	_, err := s.agentMgr.SendRequest(instance.NodeID, "reset_password", map[string]string{
		"instance_id": instance.IncusName,
		"password":    password,
	}, 15*time.Second)
	if err != nil {
		if strings.Contains(err.Error(), "超时") {
			return service.ErrOperationTimeout
		}
		return fmt.Errorf("Agent 重置密码失败: %w", err)
	}

	if err := db.DB.Model(&instance).Update("ssh_password", password).Error; err != nil {
		zap.L().Error("密码已在 Agent 重置但 DB 更新失败",
			zap.String("instance_id", instanceID.String()),
			zap.Error(err))
		return fmt.Errorf("Agent 已重置密码但数据库更新失败: %w", err)
	}

	return nil
}

// GetInstanceConsole 获取控制台直连信息（返回 Agent 地址 + Token，前端直连 Agent）
func (s *InstanceService) GetInstanceConsole(instanceID uuid.UUID, consoleType string) (map[string]interface{}, error) {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}

	if consoleType == "" {
		consoleType = "ssh"
	}
	if consoleType != "vnc" && consoleType != "ssh" {
		return nil, fmt.Errorf("不支持的控制台类型: %s", consoleType)
	}

	var node models.Node
	if err := db.DB.Where("id = ?", instance.NodeID).First(&node).Error; err != nil {
		return nil, fmt.Errorf("查询节点信息失败: %w", err)
	}

	if !node.IsOnline() {
		return nil, service.ErrNodeNotConnected
	}

	session := console.ConsoleSession{
		InstanceID: instance.ID.String(),
		NodeID:     instance.NodeID.String(),
		Type:       consoleType,
		IncusName:  instance.IncusName,
	}
	token, err := console.GenerateConsoleToken(session)
	if err != nil {
		return nil, fmt.Errorf("生成控制台 Token 失败: %w", err)
	}

	agentAddr := node.IPAddress
	if agentAddr == "" {
		agentAddr = node.Hostname
	}

	var agentURL string
	if node.AgentURL != "" {
		base := strings.TrimRight(node.AgentURL, "/")
		scheme := "ws"
		if strings.HasPrefix(base, "https://") {
			scheme = "wss"
			base = strings.TrimPrefix(base, "https://")
		} else if strings.HasPrefix(base, "http://") {
			base = strings.TrimPrefix(base, "http://")
		}
		agentURL = fmt.Sprintf("%s://%s/console/%s?token=%s&container=%s",
			scheme, base, consoleType, token, instance.IncusName)
	} else {
		agentURL = fmt.Sprintf("ws://%s:9090/console/%s?token=%s&container=%s",
			agentAddr, consoleType, token, instance.IncusName)
	}

	return map[string]interface{}{
		"type":        consoleType,
		"token":       token,
		"agent_url":   agentURL,
		"agent_addr":  fmt.Sprintf("%s:9090", agentAddr),
		"instance_id": instance.ID.String(),
		"incus_name":  instance.IncusName,
		"node_id":     instance.NodeID.String(),
		"expires_in":  300,
	}, nil
}
