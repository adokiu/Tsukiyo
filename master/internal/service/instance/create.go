package instance

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

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
