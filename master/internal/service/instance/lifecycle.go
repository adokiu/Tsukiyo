package instance

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/service"
)

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
