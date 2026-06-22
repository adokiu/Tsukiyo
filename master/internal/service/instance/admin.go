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
)

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
