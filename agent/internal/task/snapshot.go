package task

import (
	"encoding/json"

	"go.uber.org/zap"
)

func (e *Executor) handleCreateSnapshot(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
		Name       string `json:"name"`
		Stateful   bool   `json:"stateful"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析创建快照任务参数失败", zap.Error(err))
		return nil, err
	}
	zap.L().Info("开始创建快照", zap.String("instance_id", req.InstanceID), zap.String("name", req.Name), zap.Bool("stateful", req.Stateful))
	if err := e.incusClient.CreateSnapshot(req.InstanceID, req.Name, req.Stateful); err != nil {
		zap.L().Error("创建快照失败", zap.String("instance_id", req.InstanceID), zap.String("name", req.Name), zap.Error(err))
		return nil, err
	}
	zap.L().Info("创建快照成功", zap.String("instance_id", req.InstanceID), zap.String("name", req.Name))
	return json.Marshal(map[string]string{"snapshot": req.Name, "instance_id": req.InstanceID})
}

func (e *Executor) handleRestoreSnapshot(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
		Name       string `json:"name"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析恢复快照任务参数失败", zap.Error(err))
		return nil, err
	}
	zap.L().Info("开始恢复快照", zap.String("instance_id", req.InstanceID), zap.String("name", req.Name))
	if err := e.incusClient.RestoreSnapshot(req.InstanceID, req.Name); err != nil {
		zap.L().Error("恢复快照失败", zap.String("instance_id", req.InstanceID), zap.String("name", req.Name), zap.Error(err))
		return nil, err
	}
	zap.L().Info("恢复快照成功", zap.String("instance_id", req.InstanceID), zap.String("name", req.Name))
	return json.Marshal(map[string]string{"status": "restored", "instance_id": req.InstanceID, "name": req.Name})
}

func (e *Executor) handleDeleteSnapshot(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
		Name       string `json:"name"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析删除快照任务参数失败", zap.Error(err))
		return nil, err
	}
	zap.L().Info("开始删除快照", zap.String("instance_id", req.InstanceID), zap.String("name", req.Name))
	if err := e.incusClient.DeleteSnapshot(req.InstanceID, req.Name); err != nil {
		zap.L().Error("删除快照失败", zap.String("instance_id", req.InstanceID), zap.String("name", req.Name), zap.Error(err))
		return nil, err
	}
	zap.L().Info("删除快照成功", zap.String("instance_id", req.InstanceID), zap.String("name", req.Name))
	return json.Marshal(map[string]interface{}{"deleted": true, "instance_id": req.InstanceID, "name": req.Name})
}
