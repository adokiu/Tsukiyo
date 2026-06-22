package agent

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// handleSecurityAlert 处理 Agent 上报的安全告警
func (m *Manager) handleSecurityAlert(nodeID uuid.UUID, payload json.RawMessage) {
	var alert struct {
		Token      string `json:"token"`
		InstanceID string `json:"instance_id"`
		AlertType  string `json:"alert_type"`
		Severity   string `json:"severity"`
		SourceIP   string `json:"source_ip"`
		DestPort   int    `json:"dest_port"`
		Protocol   string `json:"protocol"`
		Details    string `json:"details"`
		RawData    string `json:"raw_data"`
		DetectedAt int64  `json:"detected_at"`
	}
	if err := json.Unmarshal(payload, &alert); err != nil {
		zap.L().Error("解析安全告警失败", zap.Error(err))
		return
	}

	detectedAt := time.Unix(alert.DetectedAt, 0)
	if alert.DetectedAt == 0 {
		detectedAt = time.Now()
	}

	dbAlert := models.SecurityAlert{
		ID:         uuid.New(),
		NodeID:     nodeID,
		InstanceID: alert.InstanceID,
		AlertType:  alert.AlertType,
		Severity:   models.AlertSeverity(alert.Severity),
		Status:     models.AlertStatusOpen,
		SourceIP:   alert.SourceIP,
		DestPort:   alert.DestPort,
		Protocol:   alert.Protocol,
		Details:    alert.Details,
		RawData:    alert.RawData,
		DetectedAt: detectedAt,
	}

	if err := db.DB.Create(&dbAlert).Error; err != nil {
		zap.L().Error("持久化安全告警失败",
			zap.String("node_id", nodeID.String()),
			zap.String("alert_type", alert.AlertType),
			zap.Error(err))
		return
	}

	zap.L().Warn("收到安全告警",
		zap.String("node_id", nodeID.String()),
		zap.String("alert_type", alert.AlertType),
		zap.String("severity", alert.Severity),
		zap.String("source_ip", alert.SourceIP),
		zap.String("details", alert.Details))

	if alert.AlertType == "mining" || alert.AlertType == "smtp_abuse" {
		dbAlert.AutoAction = "auto_stop_instance"
		db.DB.Model(&dbAlert).Update("auto_action", dbAlert.AutoAction)

		if alert.InstanceID != "" {
			var instance models.Instance
			if err := db.DB.Where("incus_name = ? AND node_id = ?", alert.InstanceID, nodeID).First(&instance).Error; err == nil {
				zap.L().Warn("自动处置：因安全告警暂停实例",
					zap.String("instance_id", instance.ID.String()),
					zap.String("alert_type", alert.AlertType))

				payloadBytes, _ := json.Marshal(map[string]interface{}{
					"instance_id": instance.IncusName,
					"force":       true,
				})
				stopTask := models.Task{
					ID:         uuid.New(),
					Type:       models.TaskTypeStopInstance,
					NodeID:     nodeID,
					InstanceID: &instance.ID,
					UserID:     0,
					Status:     models.TaskStatusPending,
					Payload:    payloadBytes,
				}
				db.DB.Create(&stopTask)
			}
		}
	}

	m.BroadcastToFrontend(map[string]interface{}{
		"type": "security_alert",
		"data": dbAlert,
	})
}
