package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// removeConnection 移除连接
func (m *Manager) removeConnection(nodeID uuid.UUID) {
	m.mu.Lock()
	delete(m.connections, nodeID)
	m.mu.Unlock()

	// 更新节点状态为离线
	db.DB.Model(&models.Node{}).Where("id = ?", nodeID).Updates(map[string]interface{}{
		"status":         models.NodeStatusOffline,
		"last_heartbeat": time.Now(),
	})

	ctx := context.Background()
	nodeKey := fmt.Sprintf("agent:%s", nodeID)
	db.RedisClient.Del(ctx, nodeKey)

	zap.L().Info("Agent 断开连接", zap.String("node_id", nodeID.String()))

	// 处理该节点上运行中的任务：标记为失败
	now := time.Now()
	var runningTasks []models.Task
	db.DB.Where("node_id = ? AND status = ?", nodeID, models.TaskStatusRunning).Find(&runningTasks)
	for _, task := range runningTasks {
		errMsg := "Agent 断开连接，任务中断"
		updates := map[string]interface{}{
			"status":       models.TaskStatusFailed,
			"error":        errMsg,
			"completed_at": &now,
		}
		db.DB.Model(&task).Updates(updates)

		// 广播任务失败状态到前端
		m.broadcastTaskStatus(task.ID, task.Type, nodeID, models.TaskStatusFailed, errMsg)

		// 处理实例状态恢复
		if task.InstanceID != nil {
			var inst models.Instance
			if err := db.DB.Where("id = ?", *task.InstanceID).First(&inst).Error; err == nil {
				if inst.IsBusy() {
					switch task.Type {
					case models.TaskTypeCreateInstance:
						db.DB.Model(&inst).Update("status", models.InstanceStatusError)
					case models.TaskTypeDeleteInstance:
						// 删除中断，恢复到之前的状态（从 payload 读取 old_status）
						oldStatus := models.InstanceStatusError
						var payload struct {
							OldStatus string `json:"old_status"`
						}
						if err := json.Unmarshal(task.Payload, &payload); err == nil && payload.OldStatus != "" {
							oldStatus = models.InstanceStatus(payload.OldStatus)
						}
						db.DB.Model(&inst).Update("status", oldStatus)
					case models.TaskTypeStartInstance:
						db.DB.Model(&inst).Update("status", models.InstanceStatusStopped)
					case models.TaskTypeStopInstance:
						db.DB.Model(&inst).Update("status", models.InstanceStatusRunning)
					case models.TaskTypeRestartInstance:
						db.DB.Model(&inst).Update("status", models.InstanceStatusRunning)
					case models.TaskTypeReinstallInstance:
						oldStatus := models.InstanceStatusError
						var payload struct {
							OldStatus string `json:"old_status"`
						}
						if err := json.Unmarshal(task.Payload, &payload); err == nil && payload.OldStatus != "" {
							oldStatus = models.InstanceStatus(payload.OldStatus)
						}
						db.DB.Model(&inst).Update("status", oldStatus)
					case models.TaskTypeResizeInstance:
						oldStatus := models.InstanceStatusError
						var payload struct {
							OldStatus string `json:"old_status"`
						}
						if err := json.Unmarshal(task.Payload, &payload); err == nil && payload.OldStatus != "" {
							oldStatus = models.InstanceStatus(payload.OldStatus)
						}
						db.DB.Model(&inst).Update("status", oldStatus)
					case models.TaskTypeResizeDisk, models.TaskTypeLimitNetwork, models.TaskTypeLimitIOPS:
						oldStatus := models.InstanceStatusError
						var payload struct {
							OldStatus string `json:"old_status"`
						}
						if err := json.Unmarshal(task.Payload, &payload); err == nil && payload.OldStatus != "" {
							oldStatus = models.InstanceStatus(payload.OldStatus)
						}
						db.DB.Model(&inst).Update("status", oldStatus)
					default:
						db.DB.Model(&inst).Update("status", models.InstanceStatusError)
					}
					zap.L().Warn("Agent 断开，实例状态恢复",
						zap.String("instance_id", inst.ID.String()),
						zap.String("task_type", string(task.Type)))
				}
			}
		}
	}

	// 将该节点上所有非中间状态的实例标记为 offline
	// busy 状态的实例已经在上面通过任务恢复处理了
	var instances []models.Instance
	db.DB.Where("node_id = ? AND status NOT IN ?",
		nodeID, []models.InstanceStatus{
			models.InstanceStatusCreating, models.InstanceStatusStarting,
			models.InstanceStatusStopping, models.InstanceStatusRestarting,
			models.InstanceStatusReinstalling, models.InstanceStatusResizing,
			models.InstanceStatusDeleting, models.InstanceStatusOffline,
			models.InstanceStatusMissing, models.InstanceStatusBanned, models.InstanceStatusExpired,
		}).Find(&instances)
	for _, inst := range instances {
		db.DB.Model(&inst).Update("status", models.InstanceStatusOffline)
		zap.L().Info("Agent 离线，实例标记为 offline",
			zap.String("instance_id", inst.ID.String()),
			zap.String("incus_name", inst.IncusName))
	}

	// 广播节点离线到前端
	m.broadcastNodeOffline(nodeID)
}
