package task

import (
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
)

// handleAddDisk 添加数据盘
func (e *Executor) handleAddDisk(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID  string `json:"instance_id"`
		DiskID      string `json:"disk_id"`
		DiskName    string `json:"disk_name"`
		SizeMB      int    `json:"size_mb"`
		StoragePool string `json:"storage_pool"`
		MountPoint  string `json:"mount_point"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析添加数据盘任务参数失败", zap.Error(err))
		return nil, err
	}

	zap.L().Info("开始添加数据盘",
		zap.String("instance_id", req.InstanceID),
		zap.String("disk_name", req.DiskName),
		zap.Int("size_mb", req.SizeMB),
		zap.String("pool", req.StoragePool),
		zap.String("mount_point", req.MountPoint))

	pool := req.StoragePool
	if pool == "" {
		pool = "default"
	}

	deviceConfig := map[string]string{
		"type": "disk",
		"pool": pool,
		"path": req.MountPoint,
		"size": fmt.Sprintf("%dMB", req.SizeMB),
	}

	if err := e.incusClient.AddDevice(req.InstanceID, req.DiskName, deviceConfig); err != nil {
		zap.L().Error("添加数据盘失败",
			zap.String("instance_id", req.InstanceID),
			zap.String("disk_name", req.DiskName),
			zap.Error(err))
		return nil, err
	}

	zap.L().Info("添加数据盘成功",
		zap.String("instance_id", req.InstanceID),
		zap.String("disk_name", req.DiskName))

	return json.Marshal(map[string]interface{}{
		"success":   true,
		"disk_id":   req.DiskID,
		"disk_name": req.DiskName,
	})
}

// handleDeleteDisk 删除数据盘
func (e *Executor) handleDeleteDisk(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
		DiskID     string `json:"disk_id"`
		DiskName   string `json:"disk_name"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析删除数据盘任务参数失败", zap.Error(err))
		return nil, err
	}

	zap.L().Info("开始删除数据盘",
		zap.String("instance_id", req.InstanceID),
		zap.String("disk_name", req.DiskName))

	if err := e.incusClient.RemoveDevice(req.InstanceID, req.DiskName); err != nil {
		zap.L().Error("删除数据盘失败",
			zap.String("instance_id", req.InstanceID),
			zap.String("disk_name", req.DiskName),
			zap.Error(err))
		return nil, err
	}

	zap.L().Info("删除数据盘成功",
		zap.String("instance_id", req.InstanceID),
		zap.String("disk_name", req.DiskName))

	return json.Marshal(map[string]interface{}{
		"success":   true,
		"disk_id":   req.DiskID,
		"disk_name": req.DiskName,
	})
}

// handleResizeDisk 扩容数据盘
func (e *Executor) handleResizeDisk(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
		DiskID     string `json:"disk_id"`
		DiskName   string `json:"disk_name"`
		SizeMB     int    `json:"size_mb"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析扩容数据盘任务参数失败", zap.Error(err))
		return nil, err
	}

	zap.L().Info("开始扩容数据盘",
		zap.String("instance_id", req.InstanceID),
		zap.String("disk_name", req.DiskName),
		zap.Int("size_mb", req.SizeMB))

	// 获取当前所有设备，修改目标磁盘的 size 后全量更新
	devices, err := e.incusClient.GetInstanceDevices(req.InstanceID)
	if err != nil {
		zap.L().Error("获取实例设备列表失败",
			zap.String("instance_id", req.InstanceID),
			zap.Error(err))
		return nil, err
	}

	disk, exists := devices[req.DiskName]
	if !exists {
		return nil, fmt.Errorf("数据盘 %s 不存在于实例 %s", req.DiskName, req.InstanceID)
	}

	disk["size"] = fmt.Sprintf("%dMB", req.SizeMB)
	devices[req.DiskName] = disk

	if err := e.incusClient.UpdateDevice(req.InstanceID, devices); err != nil {
		zap.L().Error("扩容数据盘失败",
			zap.String("instance_id", req.InstanceID),
			zap.String("disk_name", req.DiskName),
			zap.Error(err))
		return nil, err
	}

	zap.L().Info("扩容数据盘成功",
		zap.String("instance_id", req.InstanceID),
		zap.String("disk_name", req.DiskName),
		zap.Int("size_mb", req.SizeMB))

	return json.Marshal(map[string]interface{}{
		"success":   true,
		"disk_id":   req.DiskID,
		"disk_name": req.DiskName,
		"size_mb":   req.SizeMB,
	})
}
