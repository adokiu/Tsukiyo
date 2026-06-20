package infrastructure

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"tsukiyo/master/internal/agent"
	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/service"
)

// StorageService 存储服务
type StorageService struct {
	agentMgr *agent.Manager
}

// NewStorageService 创建存储服务
func NewStorageService(agentMgr *agent.Manager) *StorageService {
	return &StorageService{agentMgr: agentMgr}
}

// PartitionInfo 分区信息
type PartitionInfo struct {
	Device     string `json:"device"`
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	Used       int64  `json:"used,omitempty"`
	Type       string `json:"type,omitempty"`
	Filesystem string `json:"filesystem,omitempty"`
	MountPoint string `json:"mount_point,omitempty"`
	IsSystem   bool   `json:"is_system"`
}

// DiskInfo 磁盘信息
type DiskInfo struct {
	Device     string          `json:"device"`
	Size       int64           `json:"size"`
	Used       int64           `json:"used,omitempty"`
	Model      string          `json:"model,omitempty"`
	Serial     string          `json:"serial,omitempty"`
	Type       string          `json:"type,omitempty"`
	Filesystem string          `json:"filesystem,omitempty"`
	IsMounted  bool            `json:"is_mounted"`
	MountPoint string          `json:"mount_point,omitempty"`
	IsSystem   bool            `json:"is_system"`
	IsInUse    bool            `json:"is_in_use"`
	Partitions []PartitionInfo `json:"partitions,omitempty"`
}

// ListNodeDisks 获取节点磁盘列表
func (s *StorageService) ListNodeDisks(nodeID uuid.UUID) ([]DiskInfo, error) {
	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, service.ErrNodeNotFound
		}
		return nil, err
	}

	if s.agentMgr == nil {
		return nil, &service.ServiceError{Message: "Agent 管理器未初始化"}
	}

	resp, err := s.agentMgr.SendRequest(nodeID, "get_disks", nil, 10*time.Second)
	if err != nil {
		return nil, err
	}

	var disks []DiskInfo
	if err := json.Unmarshal(resp, &disks); err != nil {
		return nil, err
	}

	return disks, nil
}

// FormatNodeDiskRequest 格式化磁盘请求
type FormatNodeDiskRequest struct {
	Device string `json:"device" binding:"required"`
	Type   string `json:"type" binding:"required,oneof=dir btrfs zfs lvm lvm-thin ext4"`
}

// FormatNodeDisk 格式化节点磁盘
func (s *StorageService) FormatNodeDisk(nodeID uuid.UUID, req FormatNodeDiskRequest, userID uint) (*models.Task, error) {
	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, service.ErrNodeNotFound
		}
		return nil, err
	}

	if !node.IsHealthy() {
		return nil, service.ErrNodeOffline
	}

	payload := map[string]interface{}{
		"device": req.Device,
		"type":   req.Type,
	}
	payloadBytes, _ := json.Marshal(payload)

	task := models.Task{
		ID:      uuid.New(),
		Type:    models.TaskTypeFormatDisk,
		NodeID:  nodeID,
		UserID:  userID,
		Status:  models.TaskStatusPending,
		Payload: payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
		return nil, err
	}

	return &task, nil
}

// StoragePoolInfo 存储池信息
type StoragePoolInfo struct {
	Name           string `json:"name"`
	Driver         string `json:"driver"`
	Source         string `json:"source"`
	Size           int64  `json:"size"`
	Used           int64  `json:"used"`
	InUse          bool   `json:"in_use"`
	QuotaSupported bool   `json:"quota_supported"`
}

// ListNodeStorages 获取节点存储池列表
func (s *StorageService) ListNodeStorages(nodeID uuid.UUID) ([]StoragePoolInfo, error) {
	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, service.ErrNodeNotFound
		}
		return nil, err
	}

	if s.agentMgr == nil {
		return nil, &service.ServiceError{Message: "Agent 管理器未初始化"}
	}

	resp, err := s.agentMgr.SendRequest(nodeID, "get_storages", nil, 10*time.Second)
	if err != nil {
		return nil, err
	}

	var storages []StoragePoolInfo
	if err := json.Unmarshal(resp, &storages); err != nil {
		return nil, err
	}

	return storages, nil
}

// InitNodeStorageRequest 初始化存储池请求
type InitNodeStorageRequest struct {
	Name         string `json:"name" binding:"required"`
	Driver       string `json:"driver" binding:"required,oneof=dir btrfs zfs lvm lvm-thin"`
	Source       string `json:"source"`
	Size         string `json:"size"`
	ThinpoolName string `json:"thinpool_name"`
	ZfsPoolName  string `json:"zfs_pool_name"`
}

// InitNodeStorage 初始化节点存储池
func (s *StorageService) InitNodeStorage(nodeID uuid.UUID, req InitNodeStorageRequest, userID uint) (*models.Task, error) {
	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, service.ErrNodeNotFound
		}
		return nil, err
	}

	if !node.IsHealthy() {
		return nil, service.ErrNodeOffline
	}

	payload := map[string]interface{}{
		"name":          req.Name,
		"driver":        req.Driver,
		"source":        req.Source,
		"size":          req.Size,
		"thinpool_name": req.ThinpoolName,
		"zfs_pool_name": req.ZfsPoolName,
	}
	payloadBytes, _ := json.Marshal(payload)

	task := models.Task{
		ID:      uuid.New(),
		Type:    models.TaskTypeInitStorage,
		NodeID:  nodeID,
		UserID:  userID,
		Status:  models.TaskStatusPending,
		Payload: payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
		return nil, err
	}

	return &task, nil
}

// CreatePartitionRequest 创建分区请求
type CreatePartitionRequest struct {
	Device string `json:"device" binding:"required"`
	SizeGB int    `json:"size_gb" binding:"required,gt=0"`
}

// CreatePartition 创建磁盘分区
func (s *StorageService) CreatePartition(nodeID uuid.UUID, req CreatePartitionRequest, userID uint) (*models.Task, error) {
	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, service.ErrNodeNotFound
		}
		return nil, err
	}

	if !node.IsHealthy() {
		return nil, service.ErrNodeOffline
	}

	payload := map[string]interface{}{
		"device":  req.Device,
		"size_gb": req.SizeGB,
	}
	payloadBytes, _ := json.Marshal(payload)

	task := models.Task{
		ID:      uuid.New(),
		Type:    models.TaskTypeCreatePartition,
		NodeID:  nodeID,
		UserID:  userID,
		Status:  models.TaskStatusPending,
		Payload: payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
		return nil, err
	}

	return &task, nil
}

// DeletePartitionRequest 删除分区请求
type DeletePartitionRequest struct {
	Device string `json:"device" binding:"required"`
}

// DeletePartition 删除磁盘分区
func (s *StorageService) DeletePartition(nodeID uuid.UUID, req DeletePartitionRequest, userID uint) (*models.Task, error) {
	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, service.ErrNodeNotFound
		}
		return nil, err
	}

	if !node.IsHealthy() {
		return nil, service.ErrNodeOffline
	}

	payload := map[string]interface{}{
		"device": req.Device,
	}
	payloadBytes, _ := json.Marshal(payload)

	task := models.Task{
		ID:      uuid.New(),
		Type:    models.TaskTypeDeletePartition,
		NodeID:  nodeID,
		UserID:  userID,
		Status:  models.TaskStatusPending,
		Payload: payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
		return nil, err
	}

	return &task, nil
}

// DeleteNodeStorage 删除节点存储池（异步任务）
func (s *StorageService) DeleteNodeStorage(nodeID uuid.UUID, poolName string, userID uint) (*models.Task, error) {
	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, service.ErrNodeNotFound
		}
		return nil, err
	}

	if !node.IsHealthy() {
		return nil, service.ErrNodeOffline
	}

	if s.agentMgr == nil {
		return nil, &service.ServiceError{Message: "Agent 管理器未初始化"}
	}

	payload := map[string]interface{}{
		"name": poolName,
	}
	payloadBytes, _ := json.Marshal(payload)

	task := models.Task{
		ID:      uuid.New(),
		Type:    models.TaskTypeDeleteStorage,
		NodeID:  nodeID,
		UserID:  userID,
		Status:  models.TaskStatusPending,
		Payload: payloadBytes,
	}

	if err := db.DB.Create(&task).Error; err != nil {
		return nil, err
	}

	return &task, nil
}

// VolumeInfo 卷信息
type VolumeInfo struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	ContentType string            `json:"content_type"`
	Config      map[string]string `json:"config"`
	UsedBy      []string          `json:"used_by"`
	Location    string            `json:"location"`
}

// ListNodeVolumes 获取存储池 Volume 列表
func (s *StorageService) ListNodeVolumes(nodeID uuid.UUID, poolName string) ([]VolumeInfo, error) {
	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, service.ErrNodeNotFound
		}
		return nil, err
	}

	if s.agentMgr == nil {
		return nil, &service.ServiceError{Message: "Agent 管理器未初始化"}
	}

	resp, err := s.agentMgr.SendRequest(nodeID, "get_volumes", map[string]string{
		"pool": poolName,
	}, 10*time.Second)
	if err != nil {
		return nil, err
	}

	var volumes []VolumeInfo
	if err := json.Unmarshal(resp, &volumes); err != nil {
		return nil, fmt.Errorf("解析卷列表失败: %w", err)
	}

	return volumes, nil
}

// StorageResources 存储池空间
type StorageResources struct {
	Space struct {
		Total uint64 `json:"total"`
		Used  uint64 `json:"used"`
	} `json:"space"`
	Inodes struct {
		Total uint64 `json:"total"`
		Used  uint64 `json:"used"`
	} `json:"inodes"`
}

// GetNodeStorageResources 获取存储池空间用量
func (s *StorageService) GetNodeStorageResources(nodeID uuid.UUID, poolName string) (*StorageResources, error) {
	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, service.ErrNodeNotFound
		}
		return nil, err
	}

	if s.agentMgr == nil {
		return nil, &service.ServiceError{Message: "Agent 管理器未初始化"}
	}

	resp, err := s.agentMgr.SendRequest(nodeID, "get_storage_resources", map[string]string{
		"name": poolName,
	}, 10*time.Second)
	if err != nil {
		return nil, err
	}

	var res StorageResources
	if err := json.Unmarshal(resp, &res); err != nil {
		return nil, fmt.Errorf("解析存储资源失败: %w", err)
	}

	return &res, nil
}
