package instance

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/service"
)

// SnapshotService 快照服务
type SnapshotService struct{}

// NewSnapshotService 创建快照服务
func NewSnapshotService() *SnapshotService {
	return &SnapshotService{}
}

// SnapshotInfo 快照信息
type SnapshotInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	SizeBytes   int64     `json:"size_bytes"`
	IsScheduled bool      `json:"is_scheduled"`
	CreatedAt   time.Time `json:"created_at"`
}

// ListSnapshots 获取快照列表
func (s *SnapshotService) ListSnapshots(instanceID uuid.UUID) ([]SnapshotInfo, error) {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, service.ErrInstanceNotFound
		}
		return nil, err
	}

	var snapshots []models.Snapshot
	if err := db.DB.Where("instance_id = ?", instanceID).Order("created_at DESC").Find(&snapshots).Error; err != nil {
		return nil, err
	}

	result := make([]SnapshotInfo, 0, len(snapshots))
	for _, snap := range snapshots {
		result = append(result, SnapshotInfo{
			ID:          snap.ID.String(),
			Name:        snap.Name,
			Description: snap.Description,
			SizeBytes:   snap.SizeBytes,
			IsScheduled: snap.IsScheduled,
			CreatedAt:   snap.CreatedAt,
		})
	}

	return result, nil
}

// CreateSnapshotRequest 创建快照请求
type CreateSnapshotRequest struct {
	Name        string `json:"name" binding:"required,max=64"`
	Description string `json:"description,omitempty"`
}

// CreateSnapshot 创建快照
func (s *SnapshotService) CreateSnapshot(instanceID uuid.UUID, req CreateSnapshotRequest, userID uint) (*models.Task, error) {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, service.ErrInstanceNotFound
		}
		return nil, err
	}

	// 检查快照配额
	var snapshotCount int64
	db.DB.Model(&models.Snapshot{}).Where("instance_id = ?", instanceID).Count(&snapshotCount)
	if int(snapshotCount) >= instance.SnapshotLimit {
		return nil, &service.ServiceError{Message: "快照数量已达上限"}
	}

	payload := map[string]interface{}{
		"instance_id": instance.IncusName,
		"name":        req.Name,
	}
	payloadBytes, _ := json.Marshal(payload)

	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeCreateSnapshot,
		NodeID:     instance.NodeID,
		InstanceID: &instance.ID,
		UserID:     userID,
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
		return nil, err
	}

	return &task, nil
}

// RestoreSnapshot 恢复快照
func (s *SnapshotService) RestoreSnapshot(instanceID uuid.UUID, snapshotName string, userID uint) (*models.Task, error) {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, service.ErrInstanceNotFound
		}
		return nil, err
	}

	var snapshot models.Snapshot
	if err := db.DB.Where("instance_id = ? AND name = ?", instanceID, snapshotName).First(&snapshot).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &service.ServiceError{Message: "快照不存在"}
		}
		return nil, err
	}

	payload := map[string]interface{}{
		"instance_id":   instance.IncusName,
		"snapshot_name": snapshot.Name,
	}
	payloadBytes, _ := json.Marshal(payload)

	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeRestoreSnapshot,
		NodeID:     instance.NodeID,
		InstanceID: &instance.ID,
		UserID:     userID,
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
		return nil, err
	}

	return &task, nil
}

// DeleteSnapshot 删除快照
func (s *SnapshotService) DeleteSnapshot(instanceID uuid.UUID, snapshotName string, userID uint) (*models.Task, error) {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, service.ErrInstanceNotFound
		}
		return nil, err
	}

	var snapshot models.Snapshot
	if err := db.DB.Where("instance_id = ? AND name = ?", instanceID, snapshotName).First(&snapshot).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &service.ServiceError{Message: "快照不存在"}
		}
		return nil, err
	}

	payload := map[string]interface{}{
		"instance_id":   instance.IncusName,
		"snapshot_name": snapshot.Name,
	}
	payloadBytes, _ := json.Marshal(payload)

	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeDeleteSnapshot,
		NodeID:     instance.NodeID,
		InstanceID: &instance.ID,
		UserID:     userID,
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
		return nil, err
	}

	return &task, nil
}
