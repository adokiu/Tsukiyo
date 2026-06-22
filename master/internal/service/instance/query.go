package instance

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/service"
)

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
