package instance

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/service"
)

var (
	ErrInstanceNotFound   = service.ErrInstanceNotFound
	ErrNodeNotFound       = service.ErrNodeNotFound
	ErrNodeOffline        = service.ErrNodeOffline
	ErrInvalidNodeID      = service.ErrInvalidNodeID
	ErrImageNotDownloaded = service.ErrImageNotDownloaded
	ErrInvalidVPCID       = service.ErrInvalidVPCID
	ErrVPCNotFound        = service.ErrVPCNotFound
	ErrUserNotFound       = service.ErrUserNotFound
	ErrInstanceNameExists = service.ErrInstanceNameExists
	ErrInvalidIPv4ID      = service.ErrInvalidIPv4ID
	ErrIPv4NotAvailable   = service.ErrIPv4NotAvailable
	ErrInvalidIPv6ID      = service.ErrInvalidIPv6ID
	ErrIPv6NotFound       = service.ErrIPv6NotFound
)

// InstanceService 实例服务
type InstanceService struct{}

// NewInstanceService 创建实例服务
func NewInstanceService() *InstanceService {
	return &InstanceService{}
}

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
	ImageKey           string            `json:"image_key,omitempty"`
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

	// 检查 VPC
	var vpc *models.VPCNetwork
	var internalIPv4 string
	if req.VPCID != "" {
		vpcID, err := uuid.Parse(req.VPCID)
		if err != nil {
			return nil, nil, ErrInvalidVPCID
		}
		var vpcNet models.VPCNetwork
		if err := db.DB.Where("id = ? AND node_id = ?", vpcID, nodeID).First(&vpcNet).Error; err != nil {
			return nil, nil, ErrVPCNotFound
		}
		vpc = &vpcNet

		ip, err := allocateIPFromVPC(vpc.ID, nodeID, vpc.IPv4CIDR, vpc.DefaultGatewayV4)
		if err != nil {
			return nil, nil, fmt.Errorf("VPC 内网 IP 分配失败: %w", err)
		}
		internalIPv4 = ip
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
			return nil, nil, ErrInvalidIPv4ID
		}
		var ip models.PublicIPPool
		if err := db.DB.Where("id = ? AND node_id = ? AND status = ?", ipv4ID, nodeID, models.IPStatusFree).First(&ip).Error; err != nil {
			return nil, nil, ErrIPv4NotAvailable
		}
		instance.PublicIPv4ID = &ipv4ID
		instance.IPv4Address = &ip.Address
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
			return nil, nil, ErrInvalidIPv6ID
		}
		var prefix models.IPv6Prefix
		if err := db.DB.Where("id = ? AND node_id = ?", ipv6ID, nodeID).First(&prefix).Error; err != nil {
			return nil, nil, ErrIPv6NotFound
		}
		instance.PublicIPv6PrefixID = &ipv6ID
		instance.IPv6Address = &prefix.Prefix
	}

	if err := db.DB.Create(&instance).Error; err != nil {
		zap.L().Error("创建实例失败", zap.Error(err))
		return nil, nil, err
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

	// 确定 NAT 分配策略
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

	// 创建任务
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
		zap.L().Error("创建实例任务失败", zap.Error(err))
	}

	return &instance, &task, nil
}

// allocateIPFromVPC 从 VPC CIDR 分配 IP
func allocateIPFromVPC(vpcID, nodeID uuid.UUID, cidr, gateway string) (string, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", err
	}

	// 获取已分配的 IP
	var allocated []string
	db.DB.Model(&models.Instance{}).Where("vpc_id = ? AND node_id = ? AND internal_ipv4 != ?", vpcID, nodeID, "").Pluck("internal_ipv4", &allocated)

	allocatedSet := make(map[string]struct{})
	for _, ip := range allocated {
		allocatedSet[ip] = struct{}{}
	}

	// 排除网关
	if gateway != "" {
		allocatedSet[gateway] = struct{}{}
	}

	// 从 .2 开始分配
	ip := ipNet.IP
	ip[len(ip)-1] = 2

	for ipNet.Contains(ip) {
		ipStr := ip.String()
		if _, exists := allocatedSet[ipStr]; !exists {
			return ipStr, nil
		}
		// 增加最后一个字节
		ip[len(ip)-1]++
	}

	return "", fmt.Errorf("VPC 内网 IP 已耗尽")
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
	db.DB.Model(&instance).Association("NATConfigs").Find(&instance.NATConfigs)
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

	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"instance_id": instance.IncusName,
		"old_status":  string(instance.Status),
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
		zap.L().Error("创建删除实例任务失败", zap.Error(err))
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

// ReinstallInstance 重装实例
func (s *InstanceService) ReinstallInstance(instanceID uuid.UUID, userID uint) (*models.Task, error) {
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
		Type:       models.TaskTypeReinstallInstance,
		NodeID:     instance.NodeID,
		InstanceID: &instance.ID,
		UserID:     userID,
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
		zap.L().Error("创建重装实例任务失败", zap.Error(err))
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

// ResetInstancePassword 重置密码
func (s *InstanceService) ResetInstancePassword(instanceID uuid.UUID, password string) error {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrInstanceNotFound
		}
		return err
	}

	// 更新数据库中的密码
	if err := db.DB.Model(&instance).Update("ssh_password", password).Error; err != nil {
		return err
	}

	return nil
}

// GetInstanceConsole 获取控制台 URL
func (s *InstanceService) GetInstanceConsole(instanceID uuid.UUID, consoleType string) (map[string]interface{}, error) {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}

	if consoleType == "" {
		consoleType = "vnc"
	}

	if consoleType != "vnc" && consoleType != "ssh" {
		return nil, fmt.Errorf("不支持的控制台类型")
	}

	// 生成一次性 Token
	consoleToken := uuid.New().String()

	// 控制台会话写入 Redis (5分钟有效)
	sessionKey := "console:" + consoleToken
	sessionData, _ := json.Marshal(map[string]interface{}{
		"instance_id": instance.ID.String(),
		"node_id":     instance.NodeID.String(),
		"type":        consoleType,
		"incus_name":  instance.IncusName,
	})
	db.RedisClient.Set(context.Background(), sessionKey, sessionData, 5*time.Minute)

	return map[string]interface{}{
		"type":        consoleType,
		"token":       consoleToken,
		"instance_id": instance.ID.String(),
		"incus_name":  instance.IncusName,
		"node_id":     instance.NodeID.String(),
		"expires_in":  300,
	}, nil
}
