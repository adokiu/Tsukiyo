package infrastructure

import (
	"encoding/json"
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

// DiskInfo 磁盘信息
type DiskInfo struct {
	Device     string `json:"device"`
	Size       int64  `json:"size"`
	Model      string `json:"model,omitempty"`
	Type       string `json:"type,omitempty"`
	Filesystem string `json:"filesystem,omitempty"`
	IsMounted  bool   `json:"is_mounted"`
	MountPoint string `json:"mount_point,omitempty"`
	IsSystem   bool   `json:"is_system"`
	IsInUse    bool   `json:"is_in_use"`
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
	Type   string `json:"type" binding:"required,oneof=dir btrfs zfs lvm lvm-thin"`
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
	Name      string `json:"name"`
	Driver    string `json:"driver"`
	Source    string `json:"source"`
	Total     int64  `json:"total"`
	Used      int64  `json:"used"`
	Available int64  `json:"available"`
	InUse     bool   `json:"in_use"`
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
	Name   string `json:"name" binding:"required"`
	Driver string `json:"driver" binding:"required,oneof=dir btrfs zfs lvm"`
	Source string `json:"source" binding:"required"`
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
		"name":   req.Name,
		"driver": req.Driver,
		"source": req.Source,
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
