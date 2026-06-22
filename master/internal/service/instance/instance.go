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

// isHex 判断字符串是否为十六进制
func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

var (
	ErrInstanceNotFound       = service.ErrInstanceNotFound
	ErrNodeNotFound           = service.ErrNodeNotFound
	ErrNodeOffline            = service.ErrNodeOffline
	ErrInvalidNodeID          = service.ErrInvalidNodeID
	ErrImageNotDownloaded     = service.ErrImageNotDownloaded
	ErrInvalidBridgeID        = service.ErrInvalidBridgeID
	ErrBridgeNotFound         = service.ErrBridgeNotFound
	ErrUserNotFound           = service.ErrUserNotFound
	ErrInstanceNameExists     = service.ErrInstanceNameExists
	ErrInstanceNoBridge       = service.ErrInstanceNoBridge
	ErrNoBridgeEgressIP       = service.ErrNoBridgeEgressIP
	ErrInstanceBusy           = service.ErrInstanceBusy
	ErrInstanceBanned         = service.ErrInstanceBanned
	ErrInstanceExpired        = service.ErrInstanceExpired
	ErrDiskNotFound           = service.ErrDiskNotFound
	ErrDiskShrinkNotSupported = service.ErrDiskShrinkNotSupported
	ErrInvalidResizeConfig    = service.ErrInvalidResizeConfig
	ErrDiskNameExists         = service.ErrDiskNameExists
	ErrVMResizeRequiresStop   = service.ErrVMResizeRequiresStop
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
	SizeMB      int    `json:"size_mb" binding:"required,min=1"`
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
	SwapMB           int               `json:"swap_mb,omitempty"`
	DiskMB           int               `json:"disk_mb" binding:"required,min=1"`
	StoragePool      string            `json:"storage_pool,omitempty"`
	DataDisks        []DataDiskRequest `json:"data_disks,omitempty"`
	AssignEIPv4      bool              `json:"assign_eip_ipv4,omitempty"`
	AssignEIPv6      bool              `json:"assign_eip_ipv6,omitempty"`
	EIPv4Count       int               `json:"eip_ipv4_count,omitempty"`
	EIPv6Count       int               `json:"eip_ipv6_count,omitempty"`
	EIPv6PrefixLen   int               `json:"eip_ipv6_prefix_len,omitempty"`
	EIPv4SpecificIP  string            `json:"eip_ipv4_specific_ip,omitempty"`
	EIPv6SpecificIP  string            `json:"eip_ipv6_specific_ip,omitempty"`
	EIPv4PoolID      string            `json:"eip_ipv4_pool_id,omitempty"`
	EIPv6PoolID      string            `json:"eip_ipv6_pool_id,omitempty"`
	PortMappingCount int               `json:"port_mapping_count,omitempty"`
	ExtraPorts       []int             `json:"extra_ports,omitempty"`
	NetworkDownMbps  int               `json:"network_down_mbps,omitempty"`
	NetworkUpMbps    int               `json:"network_up_mbps,omitempty"`
	IOReadIops       int               `json:"io_read_iops,omitempty"`
	IOWriteIops      int               `json:"io_write_iops,omitempty"`
	SSHPassword      string            `json:"ssh_password,omitempty"`
	SSHPublicKey     string            `json:"ssh_public_key,omitempty"`
	MonthlyTrafficGB int64             `json:"monthly_traffic_gb,omitempty"`
	TrafficMode      string            `json:"traffic_mode,omitempty"`
	OverLimitAction  string            `json:"over_limit_action,omitempty"`
	ThrottleMbps     int               `json:"throttle_mbps,omitempty"`
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
	// 解析 image_key: alias|type|arch 或 fingerprint|type
	imageParts := strings.SplitN(imageIDForCheck, "|", 3)
	imageAlias := ""
	imageType := ""
	if len(imageParts) > 0 {
		imageAlias = imageParts[0]
	}
	if len(imageParts) > 1 {
		imageType = imageParts[1]
	}
	// 镜像类型映射：前端用 vm/container，Incus 数据库存 virtual-machine/container
	incusImageType := imageType
	if incusImageType == "vm" {
		incusImageType = "virtual-machine"
	}
	// 判断是 fingerprint（64位十六进制）还是 alias
	isFingerprint := len(imageAlias) == 64 && isHex(imageAlias)
	var nodeImage models.NodeImage
	if isFingerprint {
		if err := db.DB.Where("node_id = ? AND fingerprint = ? AND image_type = ? AND status = ?", nodeID, imageAlias, incusImageType, "downloaded").First(&nodeImage).Error; err != nil {
			return nil, nil, ErrImageNotDownloaded
		}
	} else {
		if err := db.DB.Where("node_id = ? AND alias = ? AND image_type = ? AND status = ?", nodeID, imageAlias, incusImageType, "downloaded").First(&nodeImage).Error; err != nil {
			// 回退：按 alias 前缀匹配（同时匹配 image_type）
			if err2 := db.DB.Where("node_id = ? AND alias LIKE ? AND image_type = ? AND status = ?", nodeID, imageAlias+"%", incusImageType, "downloaded").First(&nodeImage).Error; err2 != nil {
				return nil, nil, ErrImageNotDownloaded
			}
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

	// 磁盘大小统一用 MB
	diskMB := req.DiskMB
	if diskMB <= 0 {
		diskMB = 10240 // 默认 10GB
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
		SwapMB:           req.SwapMB,
		DiskMB:           diskMB,
		StoragePool:      req.StoragePool,
		LoginMethod:      loginMethod,
		SSHPassword:      sshPassword,
		SSHPublicKey:     req.SSHPublicKey,
		NetworkDownMbps:  req.NetworkDownMbps,
		NetworkUpMbps:    req.NetworkUpMbps,
		IOReadIops:       req.IOReadIops,
		IOWriteIops:      req.IOWriteIops,
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
	if req.OverLimitAction != "" {
		instance.OverLimitAction = models.OverLimitAction(req.OverLimitAction)
	}
	if req.ThrottleMbps > 0 {
		instance.ThrottleMbps = req.ThrottleMbps
	}
	if req.ExpiresAt != nil {
		instance.ExpiresAt = req.ExpiresAt
	}

	// 分配 EIP（仅分配 EIP 资源，关联在实例创建后执行）
	var eipv4AllocIDs []uuid.UUID
	var eipv6AllocIDs []uuid.UUID
	if req.AssignEIPv4 && bridge != nil {
		v4Count := req.EIPv4Count
		if v4Count <= 0 {
			v4Count = 1
		}
		v4PoolID := uuid.Nil
		if req.EIPv4PoolID != "" {
			v4PoolID, _ = uuid.Parse(req.EIPv4PoolID)
		}
		for i := 0; i < v4Count; i++ {
			specificIP := ""
			if i == 0 && req.EIPv4SpecificIP != "" {
				specificIP = req.EIPv4SpecificIP
			}
			alloc, err := s.networkSvc.AllocateEIPFromPool(nodeID, v4PoolID, "ipv4", 32, specificIP)
			if err == nil {
				// 为每个 EIP 分配一个额外的内网 IP，用于 policy routing
				mappedIP, mapErr := s.networkSvc.AllocateInternalIP(bridge.ID, nodeID, bridge.IPv4CIDR, bridge.IPv4Gateway, "ipv4")
				if mapErr == nil {
					alloc.MappedInternalIP = mappedIP
					db.DB.Model(&alloc).Update("mapped_internal_ip", mappedIP)
				}
				eipv4AllocIDs = append(eipv4AllocIDs, alloc.ID)
				if instance.IPv4EIPAllocationID == nil {
					instance.IPv4EIPAllocationID = &alloc.ID
					instance.IPv4Mode = "eip"
				}
			} else {
				zap.L().Warn("分配 IPv4 EIP 失败（非致命）", zap.Int("index", i), zap.Error(err))
				break
			}
		}
	}
	if req.AssignEIPv6 && bridge != nil {
		v6Count := req.EIPv6Count
		if v6Count <= 0 {
			v6Count = 1
		}
		v6Prefix := req.EIPv6PrefixLen
		if v6Prefix <= 0 {
			v6Prefix = 128
		}
		for i := 0; i < v6Count; i++ {
			specificIP := ""
			if i == 0 && req.EIPv6SpecificIP != "" {
				specificIP = req.EIPv6SpecificIP
			}
			alloc, err := s.networkSvc.AllocateIPv6FromBridge(bridge.ID, v6Prefix, specificIP)
			if err == nil {
				eipv6AllocIDs = append(eipv6AllocIDs, alloc.ID)
				if instance.IPv6EIPAllocationID == nil {
					instance.IPv6EIPAllocationID = &alloc.ID
					instance.IPv6Mode = "eip"
				}
			} else {
				zap.L().Warn("分配 IPv6 EIP 失败（非致命）", zap.Int("index", i), zap.Error(err))
				break
			}
		}
	}

	if err := db.DB.Create(&instance).Error; err != nil {
		zap.L().Error("创建实例失败", zap.Error(err))
		return nil, nil, err
	}

	// 实例创建后，仅关联 EIP 分配记录到 DB（Agent 侧 EIP 配置在 create_instance 任务完成后触发）
	for _, allocID := range eipv4AllocIDs {
		if err := s.networkSvc.AssignEIPToInstanceDBOnly(allocID, instanceID); err != nil {
			zap.L().Warn("关联 IPv4 EIP 到实例失败", zap.String("alloc_id", allocID.String()), zap.Error(err))
		}
	}
	for _, allocID := range eipv6AllocIDs {
		if err := s.networkSvc.AssignEIPToInstanceDBOnly(allocID, instanceID); err != nil {
			zap.L().Warn("关联 IPv6 EIP 到实例失败", zap.String("alloc_id", allocID.String()), zap.Error(err))
		}
	}

	// 创建数据磁盘
	for _, dd := range req.DataDisks {
		disk := models.DataDisk{
			ID:          uuid.New(),
			InstanceID:  instanceID,
			NodeID:      nodeID,
			Name:        dd.Name,
			SizeMB:      dd.SizeMB,
			StoragePool: dd.StoragePool,
			MountPoint:  dd.MountPoint,
		}
		if disk.StoragePool == "" {
			disk.StoragePool = instance.StoragePool
		}
		db.DB.Create(&disk)
	}

	// Master 集中分配端口映射（仅 NAT 模式实例需要）
	// EIP 模式实例有公网 IP，不需要 NAT 端口映射
	// 端口配额为 0 时不自动分配 SSH
	var assignedPortMappings []map[string]interface{}

	isIPv4EIP := instance.IPv4Mode == "eip"
	isIPv6EIP := instance.IPv6Mode == "eip"

	if bridge != nil && !isIPv4EIP && !isIPv6EIP && portMappingLimit > 0 {
		ipVersion := "ipv4"
		if bridge.IPv4Enabled {
			ipVersion = "ipv4"
		} else if bridge.IPv6Enabled {
			ipVersion = "ipv6"
		}
		mappings, err := s.networkSvc.AllocatePortMappingsForInstance(
			instanceID, bridge.ID, nodeID, portMappingLimit, req.ExtraPorts, ipVersion,
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
	} else if isIPv4EIP || isIPv6EIP {
		zap.L().Info("EIP 模式实例，跳过 NAT 端口映射", zap.String("instance_id", instanceID.String()))
	} else if portMappingLimit <= 0 {
		zap.L().Info("端口映射配额为 0，跳过端口映射", zap.String("instance_id", instanceID.String()))
	}

	// 获取实际镜像源：节点配置优先，为空时回退到站点配置
	imageSource := node.ImageRemoteURL
	if imageSource == "" {
		var siteConfig models.SiteConfig
		if err := db.DB.First(&siteConfig).Error; err == nil {
			imageSource = siteConfig.IncusRemoteURL
		}
	}

	taskPayload := map[string]interface{}{
		"instance_id":        instance.IncusName,
		"type":               instance.Type,
		"template_id":        instance.TemplateID,
		"vcpu":               instance.VCPU,
		"memory_mb":          instance.MemoryMB,
		"swap_mb":            instance.SwapMB,
		"disk_mb":            instance.DiskMB,
		"storage_pool":       instance.StoragePool,
		"login_method":       instance.LoginMethod,
		"ssh_password":       instance.SSHPassword,
		"ssh_public_key":     instance.SSHPublicKey,
		"network_down":       instance.NetworkDownMbps,
		"network_up":         instance.NetworkUpMbps,
		"io_read":            instance.IOReadIops,
		"io_write":           instance.IOWriteIops,
		"data_disks":         req.DataDisks,
		"traffic_mode":       instance.TrafficMode,
		"monthly_traffic":    instance.MonthlyTrafficGB,
		"snapshot_limit":     instance.SnapshotLimit,
		"port_mappings":      assignedPortMappings,
		"image_source":       imageSource,
		"ipv4_mode":          instance.IPv4Mode,
		"ipv6_mode":          instance.IPv6Mode,
		"port_mapping_limit": portMappingLimit,
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

	// 添加 EIP 分配信息，Agent 在实例创建成功后自行配置 EIP
	var eipAssignments []map[string]interface{}
	for _, allocID := range eipv4AllocIDs {
		var alloc models.EIPAllocation
		if err := db.DB.Where("id = ?", allocID).First(&alloc).Error; err != nil {
			continue
		}
		var pool models.EIPPool
		db.DB.Where("id = ?", alloc.PoolID).First(&pool)
		eipAssignments = append(eipAssignments, map[string]interface{}{
			"eip_cidr":           alloc.CIDR,
			"ip_version":         alloc.IPVersion,
			"interface":          pool.Interface,
			"mapped_internal_ip": alloc.MappedInternalIP,
			"eip_gateway":        pool.Gateway,
		})
	}
	for _, allocID := range eipv6AllocIDs {
		var alloc models.EIPAllocation
		if err := db.DB.Where("id = ?", allocID).First(&alloc).Error; err != nil {
			continue
		}
		var pool models.EIPPool
		db.DB.Where("id = ?", alloc.PoolID).First(&pool)
		eipAssignments = append(eipAssignments, map[string]interface{}{
			"eip_cidr":           alloc.CIDR,
			"ip_version":         alloc.IPVersion,
			"interface":          pool.Interface,
			"mapped_internal_ip": alloc.MappedInternalIP,
			"eip_gateway":        bridge.IPv6Gateway,
		})
	}
	if len(eipAssignments) > 0 {
		taskPayload["eip_assignments"] = eipAssignments
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

// UpdateInstanceRequest 更新实例请求
type UpdateInstanceRequest struct {
	Name             *string  `json:"name,omitempty"`
	VCPU             *float64 `json:"vcpu,omitempty"`
	MemoryMB         *int     `json:"memory_mb,omitempty"`
	DiskMB           *int     `json:"disk_mb,omitempty"`
	SwapMB           *int     `json:"swap_mb,omitempty"`
	NetworkDownMbps  *int     `json:"network_down_mbps,omitempty"`
	NetworkUpMbps    *int     `json:"network_up_mbps,omitempty"`
	IOReadIops       *int     `json:"io_read_iops,omitempty"`
	IOWriteIops      *int     `json:"io_write_iops,omitempty"`
	ExpiresAt        *string  `json:"expires_at,omitempty"`
	MonthlyTrafficGB *int64   `json:"monthly_traffic_gb,omitempty"`
	TrafficMode      *string  `json:"traffic_mode,omitempty"`
	OverLimitAction  *string  `json:"over_limit_action,omitempty"`
	ThrottleMbps     *int     `json:"throttle_mbps,omitempty"`
	SnapshotLimit    *int     `json:"snapshot_limit,omitempty"`
	PortMappingLimit *int     `json:"port_mapping_limit,omitempty"`
}

// UpdateInstance 更新实例（元数据仅更新DB，资源配置更新DB+下发Agent）
func (s *InstanceService) UpdateInstance(instanceID uuid.UUID, req UpdateInstanceRequest) error {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrInstanceNotFound
		}
		return err
	}

	// 分离元数据更新和资源配置更新
	metaUpdates := make(map[string]interface{})
	var resourceChanged bool
	var newVCPU float64
	var newMemoryMB, newDiskMB, newSwapMB, newNetworkDown, newNetworkUp, newIORead, newIOWrite int

	if req.Name != nil && *req.Name != "" {
		metaUpdates["name"] = *req.Name
	}
	if req.ExpiresAt != nil {
		metaUpdates["expires_at"] = *req.ExpiresAt
	}
	if req.MonthlyTrafficGB != nil {
		metaUpdates["monthly_traffic_gb"] = *req.MonthlyTrafficGB
	}
	if req.TrafficMode != nil && *req.TrafficMode != "" {
		metaUpdates["traffic_mode"] = *req.TrafficMode
	}
	if req.OverLimitAction != nil && *req.OverLimitAction != "" {
		metaUpdates["over_limit_action"] = *req.OverLimitAction
	}
	if req.ThrottleMbps != nil {
		metaUpdates["throttle_mbps"] = *req.ThrottleMbps
	}
	if req.SnapshotLimit != nil {
		metaUpdates["snapshot_limit"] = *req.SnapshotLimit
	}
	if req.PortMappingLimit != nil {
		metaUpdates["port_mapping_limit"] = *req.PortMappingLimit
	}

	// 资源配置字段（仅当值实际变化时才标记 resourceChanged）
	if req.VCPU != nil {
		metaUpdates["vcpu"] = *req.VCPU
		if *req.VCPU != instance.VCPU {
			newVCPU = *req.VCPU
			resourceChanged = true
		}
	}
	if req.MemoryMB != nil {
		metaUpdates["memory_mb"] = *req.MemoryMB
		if *req.MemoryMB != instance.MemoryMB {
			newMemoryMB = *req.MemoryMB
			resourceChanged = true
		}
	}
	if req.DiskMB != nil {
		if *req.DiskMB < instance.DiskMB {
			return ErrDiskShrinkNotSupported
		}
		metaUpdates["disk_mb"] = *req.DiskMB
		if *req.DiskMB != instance.DiskMB {
			newDiskMB = *req.DiskMB
			resourceChanged = true
		}
	}
	if req.SwapMB != nil {
		metaUpdates["swap_mb"] = *req.SwapMB
		if *req.SwapMB != instance.SwapMB {
			newSwapMB = *req.SwapMB
			resourceChanged = true
		}
	}
	if req.NetworkDownMbps != nil {
		metaUpdates["network_down_mbps"] = *req.NetworkDownMbps
		if *req.NetworkDownMbps != instance.NetworkDownMbps {
			newNetworkDown = *req.NetworkDownMbps
			resourceChanged = true
		}
	}
	if req.NetworkUpMbps != nil {
		metaUpdates["network_up_mbps"] = *req.NetworkUpMbps
		if *req.NetworkUpMbps != instance.NetworkUpMbps {
			newNetworkUp = *req.NetworkUpMbps
			resourceChanged = true
		}
	}
	if req.IOReadIops != nil {
		metaUpdates["io_read_iops"] = *req.IOReadIops
		if *req.IOReadIops != instance.IOReadIops {
			newIORead = *req.IOReadIops
			resourceChanged = true
		}
	}
	if req.IOWriteIops != nil {
		metaUpdates["io_write_iops"] = *req.IOWriteIops
		if *req.IOWriteIops != instance.IOWriteIops {
			newIOWrite = *req.IOWriteIops
			resourceChanged = true
		}
	}

	if len(metaUpdates) == 0 {
		return service.ErrNoValidUpdateFields
	}

	// 资源配置变更需要前置状态检查
	if resourceChanged {
		if instance.IsBanned() {
			return ErrInstanceBanned
		}
		if instance.IsExpiredStatus() {
			return ErrInstanceExpired
		}
		if instance.Status != models.InstanceStatusRunning && instance.Status != models.InstanceStatusStopped {
			return ErrInstanceBusy
		}
		// VM 运行时调整内存需要先关机
		if instance.Type == models.InstanceTypeVM && instance.Status == models.InstanceStatusRunning && newMemoryMB > 0 {
			return ErrVMResizeRequiresStop
		}
	}

	// 更新 DB
	if err := db.DB.Model(&instance).Updates(metaUpdates).Error; err != nil {
		return err
	}

	// 如果资源配置有变更，按类型分别创建独立 task
	if resourceChanged {
		oldStatus := string(instance.Status)
		db.DB.Model(&instance).Update("status", models.InstanceStatusResizing)

		var tasks []models.Task

		// CPU/内存/Swap 变更 -> resize_instance
		if newVCPU > 0 || newMemoryMB > 0 || newSwapMB > 0 {
			payload := map[string]interface{}{
				"instance_id": instance.IncusName,
				"old_status":  oldStatus,
			}
			if newVCPU > 0 {
				payload["vcpu"] = newVCPU
			}
			if newMemoryMB > 0 {
				payload["memory_mb"] = newMemoryMB
			}
			if newSwapMB > 0 {
				payload["swap_mb"] = newSwapMB
			}
			payloadBytes, _ := json.Marshal(payload)
			tasks = append(tasks, models.Task{
				ID:         uuid.New(),
				Type:       models.TaskTypeResizeInstance,
				NodeID:     instance.NodeID,
				InstanceID: &instance.ID,
				UserID:     instance.UserID,
				Status:     models.TaskStatusPending,
				Payload:    payloadBytes,
			})
		}

		// 磁盘扩容 -> resize_disk (root 盘)
		if newDiskMB > 0 {
			payloadBytes, _ := json.Marshal(map[string]interface{}{
				"instance_id": instance.IncusName,
				"disk_name":   "root",
				"size_mb":     newDiskMB,
				"old_status":  oldStatus,
			})
			tasks = append(tasks, models.Task{
				ID:         uuid.New(),
				Type:       models.TaskTypeResizeDisk,
				NodeID:     instance.NodeID,
				InstanceID: &instance.ID,
				UserID:     instance.UserID,
				Status:     models.TaskStatusPending,
				Payload:    payloadBytes,
			})
		}

		// 网络限速变更 -> limit_network
		if newNetworkDown > 0 || newNetworkUp > 0 {
			payload := map[string]interface{}{
				"instance_id": instance.IncusName,
				"old_status":  oldStatus,
			}
			if newNetworkDown > 0 {
				payload["network_down"] = newNetworkDown
			}
			if newNetworkUp > 0 {
				payload["network_up"] = newNetworkUp
			}
			payloadBytes, _ := json.Marshal(payload)
			tasks = append(tasks, models.Task{
				ID:         uuid.New(),
				Type:       models.TaskTypeLimitNetwork,
				NodeID:     instance.NodeID,
				InstanceID: &instance.ID,
				UserID:     instance.UserID,
				Status:     models.TaskStatusPending,
				Payload:    payloadBytes,
			})
		}

		// 磁盘 IOPS 变更 -> limit_iops
		if newIORead > 0 || newIOWrite > 0 {
			payload := map[string]interface{}{
				"instance_id": instance.IncusName,
				"old_status":  oldStatus,
			}
			if newIORead > 0 {
				payload["io_read"] = newIORead
			}
			if newIOWrite > 0 {
				payload["io_write"] = newIOWrite
			}
			payloadBytes, _ := json.Marshal(payload)
			tasks = append(tasks, models.Task{
				ID:         uuid.New(),
				Type:       models.TaskTypeLimitIOPS,
				NodeID:     instance.NodeID,
				InstanceID: &instance.ID,
				UserID:     instance.UserID,
				Status:     models.TaskStatusPending,
				Payload:    payloadBytes,
			})
		}

		// 批量创建任务
		if len(tasks) > 0 {
			if err := db.DB.Create(&tasks).Error; err != nil {
				db.DB.Model(&instance).Update("status", oldStatus)
				return fmt.Errorf("创建升降配任务失败: %w", err)
			}
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

	// Agent 离线时强制删除：直接释放网络资源、删除DB记录、创建已完成的task
	if instance.Status == models.InstanceStatusOffline {
		s.networkSvc.ReleaseInstanceNetworkResources(instance.ID)
		db.DB.Where("instance_id = ?", instance.ID).Delete(&models.DataDisk{})
		db.DB.Delete(&models.Instance{}, instance.ID)

		now := time.Now()
		task := models.Task{
			ID:          uuid.New(),
			Type:        models.TaskTypeDeleteInstance,
			NodeID:      instance.NodeID,
			InstanceID:  &instance.ID,
			UserID:      userID,
			Status:      models.TaskStatusCompleted,
			Payload:     []byte(`{"force_deleted":true,"reason":"agent_offline"}`),
			Result:      []byte(`{"force_deleted":true,"reason":"agent_offline"}`),
			CompletedAt: &now,
		}
		db.DB.Create(&task)

		// 广播实例删除通知到前端
		s.agentMgr.BroadcastToFrontend(map[string]interface{}{
			"type":        "instance_status",
			"instance_id": instance.ID.String(),
			"status":      "deleted",
			"timestamp":   now.Unix(),
		})

		zap.L().Info("Agent 离线，强制删除实例",
			zap.String("instance_id", instance.ID.String()),
			zap.String("incus_name", instance.IncusName))
		return &task, nil
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

	if instance.IsBusy() {
		return nil, ErrInstanceBusy
	}
	if instance.IsBanned() {
		return nil, ErrInstanceBanned
	}
	if instance.IsExpiredStatus() {
		return nil, ErrInstanceExpired
	}
	if instance.IsOverLimit && instance.OverLimitAction == models.OverLimitActionShutdown {
		return nil, fmt.Errorf("实例流量已超限，无法开机")
	}
	if instance.Status != models.InstanceStatusStopped && instance.Status != models.InstanceStatusError {
		return nil, ErrInstanceBusy
	}

	// 设置中间状态
	if err := db.DB.Model(&instance).Update("status", models.InstanceStatusStarting).Error; err != nil {
		return nil, fmt.Errorf("更新实例状态为 starting 失败: %w", err)
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
		db.DB.Model(&instance).Update("status", models.InstanceStatusStopped)
		return nil, fmt.Errorf("创建启动实例任务失败: %w", err)
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

	if instance.IsBusy() {
		return nil, ErrInstanceBusy
	}
	if instance.IsBanned() {
		return nil, ErrInstanceBanned
	}
	if instance.IsExpiredStatus() {
		return nil, ErrInstanceExpired
	}
	if instance.Status != models.InstanceStatusRunning {
		return nil, ErrInstanceBusy
	}

	// 设置中间状态
	if err := db.DB.Model(&instance).Update("status", models.InstanceStatusStopping).Error; err != nil {
		return nil, fmt.Errorf("更新实例状态为 stopping 失败: %w", err)
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
		db.DB.Model(&instance).Update("status", models.InstanceStatusRunning)
		return nil, fmt.Errorf("创建停止实例任务失败: %w", err)
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

	if instance.IsBusy() {
		return nil, ErrInstanceBusy
	}
	if instance.IsBanned() {
		return nil, ErrInstanceBanned
	}
	if instance.IsExpiredStatus() {
		return nil, ErrInstanceExpired
	}
	if instance.Status != models.InstanceStatusRunning {
		return nil, ErrInstanceBusy
	}

	// 设置中间状态
	if err := db.DB.Model(&instance).Update("status", models.InstanceStatusRestarting).Error; err != nil {
		return nil, fmt.Errorf("更新实例状态为 restarting 失败: %w", err)
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
		db.DB.Model(&instance).Update("status", models.InstanceStatusRunning)
		return nil, fmt.Errorf("创建重启实例任务失败: %w", err)
	}

	return &task, nil
}

// ReinstallInstanceRequest 重装请求参数
type ReinstallInstanceRequest struct {
	TemplateID      string `json:"template_id"`
	Password        string `json:"password"`
	SSHKey          string `json:"ssh_key"`
	LoginMethod     string `json:"login_method"`
	FormatDataDisks bool   `json:"format_data_disks"`
}

// ReinstallInstance 重装实例（删除后重新创建，保留流量用量等不变）
func (s *InstanceService) ReinstallInstance(instanceID uuid.UUID, userID uint, req ReinstallInstanceRequest) (*models.Task, error) {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}

	if instance.IsBusy() {
		return nil, ErrInstanceBusy
	}
	if instance.IsBanned() {
		return nil, ErrInstanceBanned
	}
	if instance.IsExpiredStatus() {
		return nil, ErrInstanceExpired
	}
	if instance.Status != models.InstanceStatusRunning && instance.Status != models.InstanceStatusStopped {
		return nil, ErrInstanceBusy
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

	// 查询节点
	var node models.Node
	if err := db.DB.Where("id = ?", instance.NodeID).First(&node).Error; err != nil {
		db.DB.Model(&instance).Update("status", oldStatus)
		return nil, ErrNodeNotFound
	}

	// 查询网桥
	var bridge *models.Bridge
	if instance.BridgeID != nil {
		var b models.Bridge
		if err := db.DB.Where("id = ? AND node_id = ?", *instance.BridgeID, instance.NodeID).First(&b).Error; err == nil {
			bridge = &b
		}
	}

	// 获取实际镜像源
	imageSource := node.ImageRemoteURL
	if imageSource == "" {
		var siteConfig models.SiteConfig
		if err := db.DB.First(&siteConfig).Error; err == nil {
			imageSource = siteConfig.IncusRemoteURL
		}
	}

	// 查询数据盘
	var dataDisks []models.DataDisk
	db.DB.Where("instance_id = ?", instanceID).Find(&dataDisks)
	dataDiskPayloads := make([]map[string]interface{}, 0, len(dataDisks))
	for _, dd := range dataDisks {
		dataDiskPayloads = append(dataDiskPayloads, map[string]interface{}{
			"name":         dd.Name,
			"size_mb":      dd.SizeMB,
			"storage_pool": dd.StoragePool,
			"mount_point":  dd.MountPoint,
		})
	}

	// 查询 EIP 分配
	var eipAssignments []map[string]interface{}
	if instance.IPv4EIPAllocationID != nil {
		var alloc models.EIPAllocation
		if err := db.DB.Where("id = ?", *instance.IPv4EIPAllocationID).First(&alloc).Error; err == nil {
			var pool models.EIPPool
			db.DB.Where("id = ?", alloc.PoolID).First(&pool)
			eipAssignments = append(eipAssignments, map[string]interface{}{
				"eip_cidr":           alloc.CIDR,
				"ip_version":         alloc.IPVersion,
				"interface":          pool.Interface,
				"mapped_internal_ip": alloc.MappedInternalIP,
				"eip_gateway":        pool.Gateway,
			})
		}
	}
	if instance.IPv6EIPAllocationID != nil {
		var alloc models.EIPAllocation
		if err := db.DB.Where("id = ?", *instance.IPv6EIPAllocationID).First(&alloc).Error; err == nil {
			var pool models.EIPPool
			db.DB.Where("id = ?", alloc.PoolID).First(&pool)
			eipAssignments = append(eipAssignments, map[string]interface{}{
				"eip_cidr":           alloc.CIDR,
				"ip_version":         alloc.IPVersion,
				"interface":          pool.Interface,
				"mapped_internal_ip": alloc.MappedInternalIP,
				"eip_gateway":        pool.Gateway,
			})
		}
	}

	// 查询端口映射
	var portMappings []models.PortMapping
	db.DB.Where("instance_id = ?", instanceID).Find(&portMappings)
	portMappingPayloads := make([]map[string]interface{}, 0, len(portMappings))
	for _, pm := range portMappings {
		var egressAlloc models.EIPAllocation
		hostIP := ""
		if err := db.DB.Where("id = ?", pm.EgressAllocationID).First(&egressAlloc).Error; err == nil {
			hostIP = egressAlloc.GetIP()
		}
		portMappingPayloads = append(portMappingPayloads, map[string]interface{}{
			"host_port":      pm.HostPort,
			"container_port": pm.ContainerPort,
			"protocol":       pm.Protocol,
			"host_ip":        hostIP,
			"description":    pm.Description,
		})
	}

	// 构建和创建实例相同的完整 payload
	taskPayload := map[string]interface{}{
		"instance_id":        instance.IncusName,
		"type":               instance.Type,
		"template_id":        templateID,
		"vcpu":               instance.VCPU,
		"memory_mb":          instance.MemoryMB,
		"swap_mb":            instance.SwapMB,
		"disk_mb":            instance.DiskMB,
		"storage_pool":       instance.StoragePool,
		"login_method":       loginMethod,
		"ssh_password":       password,
		"ssh_public_key":     instance.SSHPublicKey,
		"network_down":       instance.NetworkDownMbps,
		"network_up":         instance.NetworkUpMbps,
		"io_read":            instance.IOReadIops,
		"io_write":           instance.IOWriteIops,
		"data_disks":         dataDiskPayloads,
		"traffic_mode":       instance.TrafficMode,
		"monthly_traffic":    instance.MonthlyTrafficGB,
		"snapshot_limit":     instance.SnapshotLimit,
		"port_mappings":      portMappingPayloads,
		"image_source":       imageSource,
		"ipv4_mode":          instance.IPv4Mode,
		"ipv6_mode":          instance.IPv6Mode,
		"port_mapping_limit": instance.PortMappingLimit,
		"format_data_disks":  req.FormatDataDisks,
		"old_status":         oldStatus,
	}
	if bridge != nil {
		taskPayload["bridge_id"] = bridge.ID.String()
		taskPayload["bridge_name"] = bridge.BridgeName
		taskPayload["internal_ipv4"] = instance.InternalIPv4
		taskPayload["internal_ipv6"] = instance.InternalIPv6
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
	if len(eipAssignments) > 0 {
		taskPayload["eip_assignments"] = eipAssignments
	}

	payloadBytes, _ := json.Marshal(taskPayload)

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

// ResizeInstance 升降配（已由 UpdateInstance 内部处理，保留接口兼容）
func (s *InstanceService) ResizeInstance(instanceID uuid.UUID, userID uint) (*models.Task, error) {
	return nil, ErrInvalidResizeConfig
}

// ResetInstancePassword 重置密码（改为异步任务）
func (s *InstanceService) ResetInstancePassword(instanceID uuid.UUID, userID uint, password string) (*models.Task, error) {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}

	if instance.IsBusy() {
		return nil, ErrInstanceBusy
	}
	if instance.IsBanned() {
		return nil, ErrInstanceBanned
	}
	if instance.IsExpiredStatus() {
		return nil, ErrInstanceExpired
	}
	if instance.Status != models.InstanceStatusRunning && instance.Status != models.InstanceStatusStopped {
		return nil, ErrInstanceBusy
	}

	if password == "" {
		password = GenerateRandomPassword(16)
	}

	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"instance_id": instance.IncusName,
		"password":    password,
	})

	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeResetPassword,
		NodeID:     instance.NodeID,
		InstanceID: &instance.ID,
		UserID:     userID,
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
		return nil, fmt.Errorf("创建重置密码任务失败: %w", err)
	}

	// 先更新 DB 中的密码（Agent 执行时会使用）
	db.DB.Model(&instance).Update("ssh_password", password)

	return &task, nil
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

	return map[string]interface{}{
		"type":        consoleType,
		"token":       token,
		"instance_id": instance.ID.String(),
		"incus_name":  instance.IncusName,
		"node_id":     instance.NodeID.String(),
		"expires_in":  30,
	}, nil
}

// GetConsoleCredentialsByToken 通过控制台 token 换取实例登录密码（供 VNC 控制台“粘贴密码”使用）。
// token 是已签发的 5 分钟控制台凭证, 持有者本就拥有该实例的完全控制权(可连接 VNC),
// 因此可凭 token 换取密码。密码不进入 URL, 避免出现在浏览器历史与日志中。
func (s *InstanceService) GetConsoleCredentialsByToken(token string) (map[string]interface{}, error) {
	session, err := console.ValidateConsoleToken(token)
	if err != nil {
		return nil, fmt.Errorf("token 无效或已过期: %w", err)
	}

	instanceID, err := uuid.Parse(session.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("无效的实例 ID: %w", err)
	}

	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}

	return map[string]interface{}{
		"password": instance.SSHPassword,
	}, nil
}

// BanInstance 封禁实例
func (s *InstanceService) BanInstance(instanceID uuid.UUID) error {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrInstanceNotFound
		}
		return err
	}

	if instance.Status == models.InstanceStatusBanned {
		return nil
	}

	if instance.IsBusy() {
		return ErrInstanceBusy
	}

	// 如果实例正在运行，创建停止任务
	if instance.Status == models.InstanceStatusRunning {
		payloadBytes, _ := json.Marshal(map[string]interface{}{
			"instance_id": instance.IncusName,
		})
		task := models.Task{
			ID:         uuid.New(),
			Type:       models.TaskTypeStopInstance,
			NodeID:     instance.NodeID,
			InstanceID: &instance.ID,
			UserID:     instance.UserID,
			Status:     models.TaskStatusPending,
			Payload:    payloadBytes,
		}
		db.DB.Create(&task)
	}

	// 设置封禁状态
	if err := db.DB.Model(&instance).Update("status", models.InstanceStatusBanned).Error; err != nil {
		return fmt.Errorf("更新实例状态为 banned 失败: %w", err)
	}

	return nil
}

// UnbanInstance 解封实例
func (s *InstanceService) UnbanInstance(instanceID uuid.UUID) error {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrInstanceNotFound
		}
		return err
	}

	if instance.Status != models.InstanceStatusBanned {
		return fmt.Errorf("实例未被封禁")
	}

	if err := db.DB.Model(&instance).Update("status", models.InstanceStatusStopped).Error; err != nil {
		return fmt.Errorf("更新实例状态为 stopped 失败: %w", err)
	}

	return nil
}

// SetInstanceStatus 管理员强制修改实例状态（仅允许非中间状态）
func (s *InstanceService) SetInstanceStatus(instanceID uuid.UUID, status models.InstanceStatus) error {
	allowedStatuses := map[models.InstanceStatus]bool{
		models.InstanceStatusRunning: true,
		models.InstanceStatusStopped: true,
		models.InstanceStatusError:   true,
		models.InstanceStatusExpired: true,
		models.InstanceStatusBanned:  true,
		models.InstanceStatusOffline: true,
	}
	if !allowedStatuses[status] {
		return fmt.Errorf("不允许设置为中间状态: %s", status)
	}

	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrInstanceNotFound
		}
		return err
	}

	if err := db.DB.Model(&instance).Update("status", status).Error; err != nil {
		return fmt.Errorf("更新实例状态失败: %w", err)
	}

	zap.L().Info("管理员强制修改实例状态",
		zap.String("instance_id", instanceID.String()),
		zap.String("old_status", string(instance.Status)),
		zap.String("new_status", string(status)))
	return nil
}

// RenewInstance 续期实例
func (s *InstanceService) RenewInstance(instanceID uuid.UUID, newExpiresAt *time.Time) error {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrInstanceNotFound
		}
		return err
	}

	if instance.Status != models.InstanceStatusExpired && instance.Status != models.InstanceStatusBanned {
		return fmt.Errorf("实例未处于过期或封禁状态")
	}

	updates := map[string]interface{}{
		"status":     models.InstanceStatusStopped,
		"expired_at": nil,
	}
	if newExpiresAt != nil {
		updates["expires_at"] = *newExpiresAt
	}

	if err := db.DB.Model(&instance).Updates(updates).Error; err != nil {
		return fmt.Errorf("续期实例失败: %w", err)
	}

	return nil
}

// UpdateInstanceNetworkRequest 修改网络配置请求
type UpdateInstanceNetworkRequest struct {
	BridgeID        *string `json:"bridge_id,omitempty"`
	IPv4Mode        *string `json:"ipv4_mode,omitempty"`
	IPv6Mode        *string `json:"ipv6_mode,omitempty"`
	NetworkDownMbps *int    `json:"network_down_mbps,omitempty"`
	NetworkUpMbps   *int    `json:"network_up_mbps,omitempty"`
}

// UpdateInstanceNetwork 修改实例网络配置
func (s *InstanceService) UpdateInstanceNetwork(instanceID uuid.UUID, req UpdateInstanceNetworkRequest) (*models.Task, error) {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}

	if instance.IsBusy() {
		return nil, ErrInstanceBusy
	}
	if instance.IsBanned() {
		return nil, ErrInstanceBanned
	}
	if instance.IsExpiredStatus() {
		return nil, ErrInstanceExpired
	}

	// 带宽限制热更新允许 running，Bridge/IP 模式切换需要 stopped
	needStop := false
	if req.BridgeID != nil || req.IPv4Mode != nil || req.IPv6Mode != nil {
		if instance.Status != models.InstanceStatusStopped {
			return nil, ErrInstanceBusy
		}
		needStop = true
	}
	if req.NetworkDownMbps != nil || req.NetworkUpMbps != nil {
		if instance.Status != models.InstanceStatusRunning && instance.Status != models.InstanceStatusStopped {
			return nil, ErrInstanceBusy
		}
	}

	updates := make(map[string]interface{})
	if req.NetworkDownMbps != nil {
		updates["network_down_mbps"] = *req.NetworkDownMbps
	}
	if req.NetworkUpMbps != nil {
		updates["network_up_mbps"] = *req.NetworkUpMbps
	}
	if req.IPv4Mode != nil {
		updates["ipv4_mode"] = *req.IPv4Mode
	}
	if req.IPv6Mode != nil {
		updates["ipv6_mode"] = *req.IPv6Mode
	}
	if req.BridgeID != nil {
		bridgeID, err := uuid.Parse(*req.BridgeID)
		if err != nil {
			return nil, ErrInvalidBridgeID
		}
		updates["bridge_id"] = bridgeID
	}

	if len(updates) > 0 {
		if err := db.DB.Model(&instance).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"instance_id":  instance.IncusName,
		"need_stop":    needStop,
		"network_down": instance.NetworkDownMbps,
		"network_up":   instance.NetworkUpMbps,
	})

	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeApplyNetwork,
		NodeID:     instance.NodeID,
		InstanceID: &instance.ID,
		UserID:     instance.UserID,
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
		return nil, fmt.Errorf("创建网络配置任务失败: %w", err)
	}

	return &task, nil
}

// AddDataDisk 添加数据盘
func (s *InstanceService) AddDataDisk(instanceID uuid.UUID, req DataDiskRequest) (*models.Task, error) {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}

	if instance.IsBusy() {
		return nil, ErrInstanceBusy
	}
	if instance.IsBanned() {
		return nil, ErrInstanceBanned
	}
	if instance.IsExpiredStatus() {
		return nil, ErrInstanceExpired
	}
	if instance.Status != models.InstanceStatusRunning && instance.Status != models.InstanceStatusStopped {
		return nil, ErrInstanceBusy
	}

	// 检查同名数据盘
	var count int64
	db.DB.Model(&models.DataDisk{}).Where("instance_id = ? AND name = ?", instanceID, req.Name).Count(&count)
	if count > 0 {
		return nil, ErrDiskNameExists
	}

	storagePool := req.StoragePool
	if storagePool == "" {
		storagePool = instance.StoragePool
	}

	disk := models.DataDisk{
		ID:          uuid.New(),
		InstanceID:  instanceID,
		NodeID:      instance.NodeID,
		Name:        req.Name,
		SizeMB:      req.SizeMB,
		StoragePool: storagePool,
		MountPoint:  req.MountPoint,
		Status:      "attaching",
	}

	if err := db.DB.Create(&disk).Error; err != nil {
		return nil, fmt.Errorf("创建数据盘记录失败: %w", err)
	}

	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"instance_id":  instance.IncusName,
		"disk_name":    disk.Name,
		"size_mb":      disk.SizeMB,
		"storage_pool": disk.StoragePool,
		"mount_point":  disk.MountPoint,
		"disk_id":      disk.ID.String(),
	})

	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeAddDisk,
		NodeID:     instance.NodeID,
		InstanceID: &instance.ID,
		UserID:     instance.UserID,
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
		db.DB.Delete(&disk)
		return nil, fmt.Errorf("创建添加数据盘任务失败: %w", err)
	}

	return &task, nil
}

// DeleteDataDisk 删除数据盘
func (s *InstanceService) DeleteDataDisk(instanceID uuid.UUID, diskID uuid.UUID) (*models.Task, error) {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}

	if instance.IsBusy() {
		return nil, ErrInstanceBusy
	}
	if instance.IsBanned() {
		return nil, ErrInstanceBanned
	}
	if instance.IsExpiredStatus() {
		return nil, ErrInstanceExpired
	}
	if instance.Status != models.InstanceStatusStopped {
		return nil, ErrInstanceBusy
	}

	var disk models.DataDisk
	if err := db.DB.Where("id = ? AND instance_id = ?", diskID, instanceID).First(&disk).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrDiskNotFound
		}
		return nil, err
	}

	// 更新数据盘状态为 detaching
	db.DB.Model(&disk).Update("status", "detaching")

	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"instance_id":  instance.IncusName,
		"disk_name":    disk.Name,
		"storage_pool": disk.StoragePool,
		"disk_id":      disk.ID.String(),
	})

	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeDeleteDisk,
		NodeID:     instance.NodeID,
		InstanceID: &instance.ID,
		UserID:     instance.UserID,
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
		db.DB.Model(&disk).Update("status", "attached")
		return nil, fmt.Errorf("创建删除数据盘任务失败: %w", err)
	}

	return &task, nil
}

// ResizeDataDisk 扩容数据盘
func (s *InstanceService) ResizeDataDisk(instanceID uuid.UUID, diskID uuid.UUID, newSizeMB int) (*models.Task, error) {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}

	if instance.IsBusy() {
		return nil, ErrInstanceBusy
	}
	if instance.IsBanned() {
		return nil, ErrInstanceBanned
	}
	if instance.IsExpiredStatus() {
		return nil, ErrInstanceExpired
	}
	if instance.Status != models.InstanceStatusRunning && instance.Status != models.InstanceStatusStopped {
		return nil, ErrInstanceBusy
	}

	var disk models.DataDisk
	if err := db.DB.Where("id = ? AND instance_id = ?", diskID, instanceID).First(&disk).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrDiskNotFound
		}
		return nil, err
	}

	if newSizeMB <= disk.SizeMB {
		return nil, ErrDiskShrinkNotSupported
	}

	// 更新 DB 中的大小
	db.DB.Model(&disk).Update("size_mb", newSizeMB)

	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"instance_id":  instance.IncusName,
		"disk_name":    disk.Name,
		"size_mb":      newSizeMB,
		"storage_pool": disk.StoragePool,
		"disk_id":      disk.ID.String(),
	})

	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeResizeDisk,
		NodeID:     instance.NodeID,
		InstanceID: &instance.ID,
		UserID:     instance.UserID,
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
		db.DB.Model(&disk).Update("size_mb", disk.SizeMB)
		return nil, fmt.Errorf("创建扩容数据盘任务失败: %w", err)
	}

	return &task, nil
}
