package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// handleMetrics 处理监控指标上报
func (m *Manager) handleMetrics(nodeID uuid.UUID, payload json.RawMessage) {
	var metrics MetricsPayload
	if err := json.Unmarshal(payload, &metrics); err != nil {
		return
	}

	now := time.Now()
	for _, im := range metrics.Instances {
		// Agent 上报的 instance_id 是 incus_name，不是 UUID
		var instance models.Instance
		if err := db.DB.Where("incus_name = ? AND node_id = ?", im.InstanceID, nodeID).
			First(&instance).Error; err != nil {
			continue
		}

		metric := models.InstanceMetric{
			InstanceID:    instance.ID,
			NodeID:        nodeID,
			Timestamp:     now,
			CPUPercent:    im.CPUPercent,
			MemUsed:       im.MemUsed,
			MemTotal:      im.MemTotal,
			DiskUsed:      im.DiskUsed,
			DiskTotal:     im.DiskTotal,
			DiskReadBps:   im.DiskReadBps,
			DiskWriteBps:  im.DiskWriteBps,
			DiskReadIops:  im.DiskReadIops,
			DiskWriteIops: im.DiskWriteIops,
			NetInBps:      im.NetIn,
			NetOutBps:     im.NetOut,
			NetInTotal:    im.NetInTotal,
			NetOutTotal:   im.NetOutTotal,
		}

		// 每秒原始数据写入 Redis sorted set，保留15分钟
		ctx := context.Background()
		rawKey := fmt.Sprintf("instance:%s:metrics_raw", instance.ID)
		metricJSON, _ := json.Marshal(metric)
		db.RedisClient.ZAdd(ctx, rawKey, redis.Z{
			Score:  float64(now.Unix()),
			Member: string(metricJSON),
		})
		// 设置过期时间15分钟
		db.RedisClient.Expire(ctx, rawKey, 15*time.Minute)
		// 清除15分钟前的旧数据
		cutoff := float64(now.Add(-15 * time.Minute).Unix())
		db.RedisClient.ZRemRangeByScore(ctx, rawKey, "-inf", fmt.Sprintf("%.0f", cutoff))

		// 流量累加逻辑（参考 YaoNet/Old 方案）
		// current < last → delta = current（计数器重置后当前值就是本次增量）
		// current >= last → delta = current - last
		var deltaIn, deltaOut int64
		if im.NetInTotal >= instance.LastNetInTotal {
			deltaIn = im.NetInTotal - instance.LastNetInTotal
		} else {
			deltaIn = im.NetInTotal
		}
		if im.NetOutTotal >= instance.LastNetOutTotal {
			deltaOut = im.NetOutTotal - instance.LastNetOutTotal
		} else {
			deltaOut = im.NetOutTotal
		}

		monthlyInBytes := instance.MonthlyTrafficInBytes + deltaIn
		monthlyOutBytes := instance.MonthlyTrafficOutBytes + deltaOut

		// 按 TrafficMode 计算已用流量（GB）
		var usedBytes int64
		switch instance.TrafficMode {
		case models.TrafficModeOutbound:
			usedBytes = monthlyOutBytes
		case models.TrafficModeInbound:
			usedBytes = monthlyInBytes
		case models.TrafficModeMax:
			if monthlyInBytes > monthlyOutBytes {
				usedBytes = monthlyInBytes
			} else {
				usedBytes = monthlyOutBytes
			}
		default: // total
			usedBytes = monthlyInBytes + monthlyOutBytes
		}
		usedGB := float64(usedBytes) / (1024 * 1024 * 1024)

		// 更新实例流量字段
		db.DB.Model(&instance).Updates(map[string]interface{}{
			"last_net_in_total":         im.NetInTotal,
			"last_net_out_total":        im.NetOutTotal,
			"monthly_traffic_in_bytes":  monthlyInBytes,
			"monthly_traffic_out_bytes": monthlyOutBytes,
			"traffic_used_gb":           math.Round(usedGB*100) / 100,
		})

		// 实时检测流量超额
		if instance.MonthlyTrafficGB > 0 && usedGB >= float64(instance.MonthlyTrafficGB) && !instance.IsOverLimit {
			m.handleTrafficOverLimit(&instance, usedGB)
		}

		// 缓存最新指标
		metricKey := fmt.Sprintf("instance:%s:metrics", instance.ID)
		metricData, _ := json.Marshal(map[string]interface{}{
			"cpu_percent":     im.CPUPercent,
			"mem_used":        im.MemUsed * 1024 * 1024,   // MB -> bytes
			"mem_total":       im.MemTotal * 1024 * 1024,  // MB -> bytes
			"disk_used":       im.DiskUsed * 1024 * 1024,  // MB -> bytes
			"disk_total":      im.DiskTotal * 1024 * 1024, // MB -> bytes
			"disk_read_bps":   im.DiskReadBps,
			"disk_write_bps":  im.DiskWriteBps,
			"disk_read_iops":  im.DiskReadIops,
			"disk_write_iops": im.DiskWriteIops,
			"net_in":          im.NetIn,
			"net_out":         im.NetOut,
			"net_in_total":    im.NetInTotal,
			"net_out_total":   im.NetOutTotal,
			"timestamp":       now.Unix(),
		})
		db.RedisClient.Set(ctx, metricKey, metricData, 10*time.Second)

		// 通过 WebSocket 推送实时监控指标给前端
		m.BroadcastInstanceMetrics(instance.ID, map[string]interface{}{
			"cpu_usage":       im.CPUPercent,
			"memory_usage":    im.MemUsed * 1024 * 1024,
			"memory_total":    im.MemTotal * 1024 * 1024,
			"disk_used":       im.DiskUsed * 1024 * 1024,
			"disk_total":      im.DiskTotal * 1024 * 1024,
			"disk_read_bps":   im.DiskReadBps,
			"disk_write_bps":  im.DiskWriteBps,
			"disk_read_iops":  im.DiskReadIops,
			"disk_write_iops": im.DiskWriteIops,
			"network_rx":      im.NetIn,
			"network_tx":      im.NetOut,
			"traffic_used_gb": math.Round(usedGB*100) / 100,
			"monthly_traffic": instance.MonthlyTrafficGB,
			"timestamp":       now.Unix(),
		})
	}
}

// handleTrafficOverLimit 处理流量超额
func (m *Manager) handleTrafficOverLimit(instance *models.Instance, usedGB float64) {
	zap.L().Info("实例流量超额",
		zap.String("instance_id", instance.ID.String()),
		zap.Float64("used", usedGB),
		zap.Int64("limit", instance.MonthlyTrafficGB),
		zap.String("action", string(instance.OverLimitAction)))

	updates := map[string]interface{}{
		"is_over_limit": true,
	}

	switch instance.OverLimitAction {
	case models.OverLimitActionShutdown:
		updates["status"] = models.InstanceStatusStopped
		db.DB.Model(instance).Updates(updates)
		// 下发停止任务
		task := models.Task{
			ID:         uuid.New(),
			Type:       models.TaskTypeStopInstance,
			NodeID:     instance.NodeID,
			InstanceID: &instance.ID,
			UserID:     instance.UserID,
			Status:     models.TaskStatusPending,
		}
		db.DB.Create(&task)

	case models.OverLimitActionThrottle:
		db.DB.Model(instance).Updates(updates)
		// 下发限速任务
		throttleMbps := instance.ThrottleMbps
		if throttleMbps <= 0 {
			throttleMbps = 1
		}
		payloadBytes, _ := json.Marshal(map[string]interface{}{
			"instance_id":  instance.IncusName,
			"network_down": throttleMbps,
			"network_up":   throttleMbps,
		})
		task := models.Task{
			ID:         uuid.New(),
			Type:       models.TaskTypeLimitNetwork,
			NodeID:     instance.NodeID,
			InstanceID: &instance.ID,
			UserID:     instance.UserID,
			Status:     models.TaskStatusPending,
			Payload:    payloadBytes,
		}
		db.DB.Create(&task)

	default:
		// 默认 shutdown
		updates["status"] = models.InstanceStatusStopped
		db.DB.Model(instance).Updates(updates)
		task := models.Task{
			ID:         uuid.New(),
			Type:       models.TaskTypeStopInstance,
			NodeID:     instance.NodeID,
			InstanceID: &instance.ID,
			UserID:     instance.UserID,
			Status:     models.TaskStatusPending,
		}
		db.DB.Create(&task)
	}
}
