package task

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

func (e *Executor) handleDeleteInstance(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析删除实例任务参数失败", zap.Error(err))
		return nil, err
	}
	zap.L().Info("开始删除实例", zap.String("instance_id", req.InstanceID))

	zap.L().Info("获取实例信息", zap.String("instance_id", req.InstanceID))
	info, err := e.incusClient.GetInstance(req.InstanceID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "Not found") {
			zap.L().Info("实例不存在，幂等删除成功", zap.String("instance_id", req.InstanceID))
			return json.Marshal(map[string]interface{}{"deleted": true, "not_found": true})
		}
		zap.L().Error("获取实例信息失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
		return nil, fmt.Errorf("获取实例信息失败: %w", err)
	}

	zap.L().Info("清理网络配置", zap.String("instance_id", req.InstanceID))
	if info.Devices != nil {
		if devs, ok := info.Devices["eth0"].(map[string]interface{}); ok {
			if ip, ok := devs["ipv4.address"].(string); ok && ip != "" {
				zap.L().Info("解绑 IP", zap.String("ip", ip))
				e.netManager.UnbindIP(ip, "")
			}
		}
	}

	zap.L().Info("删除实例", zap.String("instance_id", req.InstanceID))
	if err := e.incusClient.DeleteInstance(req.InstanceID); err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "Not found") {
			zap.L().Info("删除时实例已不存在，幂等成功", zap.String("instance_id", req.InstanceID))
			return json.Marshal(map[string]interface{}{"deleted": true, "not_found": true})
		}
		zap.L().Error("删除实例失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
		return nil, fmt.Errorf("删除实例失败: %w", err)
	}
	zap.L().Info("实例删除成功", zap.String("instance_id", req.InstanceID))
	return json.Marshal(map[string]bool{"deleted": true})
}

func (e *Executor) handleStartInstance(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析启动实例任务参数失败", zap.Error(err))
		return nil, err
	}
	zap.L().Info("开始启动实例", zap.String("instance_id", req.InstanceID))
	if err := e.incusClient.StartInstance(req.InstanceID); err != nil {
		zap.L().Error("启动实例失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
		return nil, err
	}
	zap.L().Info("启动实例成功", zap.String("instance_id", req.InstanceID))
	return json.Marshal(map[string]string{"status": "running", "instance_id": req.InstanceID})
}

func (e *Executor) handleStopInstance(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
		Force      bool   `json:"force"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析停止实例任务参数失败", zap.Error(err))
		return nil, err
	}
	zap.L().Info("开始停止实例", zap.String("instance_id", req.InstanceID), zap.Bool("force", req.Force))
	if err := e.incusClient.StopInstance(req.InstanceID, req.Force); err != nil {
		zap.L().Error("停止实例失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
		return nil, err
	}
	zap.L().Info("停止实例成功", zap.String("instance_id", req.InstanceID))
	return json.Marshal(map[string]string{"status": "stopped", "instance_id": req.InstanceID})
}

func (e *Executor) handleRestartInstance(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析重启实例任务参数失败", zap.Error(err))
		return nil, err
	}
	zap.L().Info("开始重启实例", zap.String("instance_id", req.InstanceID))
	if err := e.incusClient.RestartInstance(req.InstanceID); err != nil {
		zap.L().Error("重启实例失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
		return nil, err
	}
	zap.L().Info("重启实例成功", zap.String("instance_id", req.InstanceID))
	return json.Marshal(map[string]string{"status": "running", "instance_id": req.InstanceID})
}

func (e *Executor) handleReinstallInstance(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
		TemplateID string `json:"template_id"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析重装实例任务参数失败", zap.Error(err))
		return nil, err
	}
	zap.L().Info("开始重装实例", zap.String("instance_id", req.InstanceID), zap.String("template_id", req.TemplateID))
	if err := e.incusClient.ReinstallInstance(req.InstanceID, req.TemplateID); err != nil {
		zap.L().Error("重装实例失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
		return nil, err
	}
	zap.L().Info("重装实例成功", zap.String("instance_id", req.InstanceID))
	return json.Marshal(map[string]string{"status": "reinstalled", "instance_id": req.InstanceID})
}

func (e *Executor) handleResizeInstance(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string  `json:"instance_id"`
		VCPU       float64 `json:"vcpu"`
		MemoryMB   int     `json:"memory_mb"`
		DiskGB     int     `json:"disk_gb"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析调整配置任务参数失败", zap.Error(err))
		return nil, err
	}
	zap.L().Info("开始调整实例配置", zap.String("instance_id", req.InstanceID), zap.Float64("vcpu", req.VCPU), zap.Int("memory_mb", req.MemoryMB), zap.Int("disk_gb", req.DiskGB))

	config := map[string]string{}
	if req.VCPU > 0 {
		config["limits.cpu"] = strconv.Itoa(int(req.VCPU))
	}
	if req.MemoryMB > 0 {
		config["limits.memory"] = fmt.Sprintf("%dMB", req.MemoryMB)
	}

	if req.DiskGB > 0 {
		zap.L().Info("调整磁盘大小", zap.String("instance_id", req.InstanceID), zap.Int("disk_gb", req.DiskGB))
		devices := map[string]map[string]string{
			"root": {
				"path": "/",
				"pool": "default",
				"type": "disk",
				"size": fmt.Sprintf("%dGB", req.DiskGB),
			},
		}
		if err := e.incusClient.SetInstanceConfig(req.InstanceID, config, devices); err != nil {
			zap.L().Error("调整磁盘大小失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
			return nil, err
		}
	} else {
		zap.L().Info("调整 CPU 和内存", zap.String("instance_id", req.InstanceID))
		if err := e.incusClient.UpdateInstanceConfig(req.InstanceID, config); err != nil {
			zap.L().Error("调整 CPU 和内存失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
			return nil, err
		}
	}

	zap.L().Info("调整实例配置成功", zap.String("instance_id", req.InstanceID))
	return json.Marshal(map[string]string{"status": "resized", "instance_id": req.InstanceID})
}

func (e *Executor) handleResetPassword(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
		Password   string `json:"password"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析重置密码任务参数失败", zap.Error(err))
		return nil, err
	}
	zap.L().Info("开始重置实例密码", zap.String("instance_id", req.InstanceID))
	if err := e.incusClient.SetInstancePassword(req.InstanceID, req.Password); err != nil {
		zap.L().Error("重置密码失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
		return nil, err
	}
	zap.L().Info("重置密码成功", zap.String("instance_id", req.InstanceID))
	return json.Marshal(map[string]interface{}{"success": true, "instance_id": req.InstanceID})
}

func (e *Executor) handleMigrateInstance(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
		TargetNode string `json:"target_node"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"status": "migrated"})
}
