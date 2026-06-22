package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// handleInstanceStatus 处理实例状态上报（批量）
func (m *Manager) handleInstanceStatus(nodeID uuid.UUID, payload json.RawMessage) {
	var msg struct {
		Instances []InstanceStatusPayload `json:"instances"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		return
	}

	reportedNames := make(map[string]bool, len(msg.Instances))
	ctx := context.Background()

	for _, st := range msg.Instances {
		reportedNames[st.InstanceID] = true

		var instance models.Instance
		if err := db.DB.Where("incus_name = ? AND node_id = ?", st.InstanceID, nodeID).First(&instance).Error; err != nil {
			continue
		}

		// 如果实例处于中间状态、封禁或过期，不覆盖状态
		// offline/missing 状态允许被 Agent 上报覆盖
		if (instance.IsBusy() && !instance.IsOffline() && instance.Status != models.InstanceStatusMissing) || instance.IsBanned() || instance.IsExpiredStatus() {
			updates := map[string]interface{}{}
			if st.IPv4 != "" {
				updates["ipv4_address"] = st.IPv4
			}
			if st.IPv6 != "" {
				updates["ipv6_address"] = st.IPv6
			}
			if len(updates) > 0 {
				db.DB.Model(&instance).Updates(updates)
			}
			continue
		}

		mappedStatus := mapIncusStatus(st.Status)
		updates := map[string]interface{}{
			"status": mappedStatus,
		}
		if st.IPv4 != "" {
			updates["ipv4_address"] = st.IPv4
		}
		if st.IPv6 != "" {
			updates["ipv6_address"] = st.IPv6
		}

		db.DB.Model(&instance).Updates(updates)

		statusKey := fmt.Sprintf("instance:%s:status", instance.ID)
		db.RedisClient.Set(ctx, statusKey, string(mappedStatus), 30*time.Second)
	}

	// 数据库中有但 agent 上报没有的实例，标记为 missing
	// 排除中间状态、封禁、过期、删除中、正在创建的实例
	var dbInstances []models.Instance
	db.DB.Where("node_id = ? AND status NOT IN ?",
		nodeID, []models.InstanceStatus{
			models.InstanceStatusCreating, models.InstanceStatusStarting,
			models.InstanceStatusStopping, models.InstanceStatusRestarting,
			models.InstanceStatusReinstalling, models.InstanceStatusResizing,
			models.InstanceStatusDeleting, models.InstanceStatusMissing,
			models.InstanceStatusBanned, models.InstanceStatusExpired,
		}).Find(&dbInstances)
	for _, inst := range dbInstances {
		if !reportedNames[inst.IncusName] {
			db.DB.Model(&inst).Update("status", models.InstanceStatusMissing)
			zap.L().Warn("实例在 Agent 上报列表中缺失，标记为 missing",
				zap.String("instance_id", inst.ID.String()),
				zap.String("incus_name", inst.IncusName))
		}
	}
}

// mapIncusStatus 将 Incus 状态映射到系统状态
func mapIncusStatus(incusStatus string) models.InstanceStatus {
	switch strings.ToLower(incusStatus) {
	case "running":
		return models.InstanceStatusRunning
	case "stopped":
		return models.InstanceStatusStopped
	case "frozen":
		return models.InstanceStatusStopped
	case "error":
		return models.InstanceStatusError
	default:
		return models.InstanceStatusError
	}
}
