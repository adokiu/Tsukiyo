package monitor

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// MetricPoint 指标数据点
type MetricPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	CPU         float64   `json:"cpu"`
	MemUsed     int64     `json:"mem_used"`
	MemTotal    int64     `json:"mem_total"`
	DiskRead    int64     `json:"disk_read"`
	DiskWrite   int64     `json:"disk_write"`
	NetIn       int64     `json:"net_in"`
	NetOut      int64     `json:"net_out"`
	NetInTotal  int64     `json:"net_in_total"`
	NetOutTotal int64     `json:"net_out_total"`
}

// GetInstanceMetrics 获取实例监控数据
func GetInstanceMetrics(instanceID uuid.UUID, from, to time.Time, interval string) ([]MetricPoint, error) {
	query := db.DB.Where("instance_id = ? AND timestamp >= ? AND timestamp <= ?",
		instanceID, from, to).Order("timestamp ASC")

	// 根据 interval 进行降采样
	var rawMetrics []models.InstanceMetric
	if err := query.Find(&rawMetrics).Error; err != nil {
		return nil, fmt.Errorf("查询监控数据失败: %w", err)
	}

	if len(rawMetrics) == 0 {
		return []MetricPoint{}, nil
	}

	// 根据间隔降采样
	points := downsample(rawMetrics, interval)
	return points, nil
}

// GetInstanceLatestMetrics 获取实例最新监控指标
func GetInstanceLatestMetrics(instanceID uuid.UUID) (*MetricPoint, error) {
	var metric models.InstanceMetric
	if err := db.DB.Where("instance_id = ?", instanceID).
		Order("timestamp DESC").First(&metric).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &MetricPoint{
		Timestamp:   metric.Timestamp,
		CPU:         metric.CPUPercent,
		MemUsed:     metric.MemUsed,
		MemTotal:    metric.MemTotal,
		DiskRead:    metric.DiskReadBps,
		DiskWrite:   metric.DiskWriteBps,
		NetIn:       metric.NetInBps,
		NetOut:      metric.NetOutBps,
		NetInTotal:  metric.NetInTotal,
		NetOutTotal: metric.NetOutTotal,
	}, nil
}

// GetNodeMetricsSummary 获取节点监控汇总
func GetNodeMetricsSummary(nodeID uuid.UUID) (map[string]interface{}, error) {
	var instances []models.Instance
	if err := db.DB.Where("node_id = ?", nodeID).Find(&instances).Error; err != nil {
		return nil, err
	}

	summary := make(map[string]interface{})
	summary["instance_count"] = len(instances)

	var totalCPU, totalMem, totalDisk int64
	for _, inst := range instances {
		latest, _ := GetInstanceLatestMetrics(inst.ID)
		if latest != nil {
			totalCPU += int64(latest.CPU)
			totalMem += latest.MemUsed
		}
	}

	summary["total_cpu_percent"] = totalCPU
	summary["total_mem_used_mb"] = totalMem
	summary["total_disk_used_gb"] = totalDisk

	return summary, nil
}

// downsample 降采样
func downsample(metrics []models.InstanceMetric, interval string) []MetricPoint {
	if len(metrics) == 0 {
		return []MetricPoint{}
	}

	// 根据 interval 确定聚合窗口
	var window time.Duration
	switch interval {
	case "1m":
		window = time.Minute
	case "5m":
		window = 5 * time.Minute
	case "15m":
		window = 15 * time.Minute
	case "1h":
		window = time.Hour
	default:
		window = time.Minute
	}

	points := make([]MetricPoint, 0)
	var current *MetricPoint
	var count int

	for _, m := range metrics {
		if current == nil || m.Timestamp.Sub(current.Timestamp) >= window {
			if current != nil {
				current.CPU /= float64(count)
				current.MemUsed /= int64(count)
				points = append(points, *current)
			}
			current = &MetricPoint{
				Timestamp:   m.Timestamp,
				CPU:         m.CPUPercent,
				MemUsed:     m.MemUsed,
				MemTotal:    m.MemTotal,
				DiskRead:    m.DiskReadBps,
				DiskWrite:   m.DiskWriteBps,
				NetIn:       m.NetInBps,
				NetOut:      m.NetOutBps,
				NetInTotal:  m.NetInTotal,
				NetOutTotal: m.NetOutTotal,
			}
			count = 1
		} else {
			current.CPU += m.CPUPercent
			current.MemUsed += m.MemUsed
			current.DiskRead += m.DiskReadBps
			current.DiskWrite += m.DiskWriteBps
			current.NetIn += m.NetInBps
			current.NetOut += m.NetOutBps
			count++
		}
	}

	if current != nil {
		current.CPU /= float64(count)
		current.MemUsed /= int64(count)
		points = append(points, *current)
	}

	return points
}

// AggregateNodeResources 聚合节点资源使用情况
func AggregateNodeResources() ([]map[string]interface{}, error) {
	var nodes []models.Node
	if err := db.DB.Find(&nodes).Error; err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, map[string]interface{}{
			"id":             node.ID.String(),
			"name":           node.Name,
			"status":         node.Status,
			"cpu_percent":    node.UsedCPU,
			"mem_percent":    float64(node.UsedMemory) / float64(node.TotalMemory) * 100,
			"disk_percent":   float64(node.UsedDisk) / float64(node.TotalDisk) * 100,
			"instance_count": node.InstanceCount,
			"running_count":  node.RunningCount,
		})
	}

	return result, nil
}

// CleanupOldMetrics 清理过期监控数据 (保留30天)
func CleanupOldMetrics() {
	cutoff := time.Now().AddDate(0, 0, -30)
	result := db.DB.Where("timestamp < ?", cutoff).Delete(&models.InstanceMetric{})
	if result.Error != nil {
		zap.L().Error("清理旧监控数据失败", zap.Error(result.Error))
	} else {
		zap.L().Info("清理旧监控数据", zap.Int64("rows", result.RowsAffected))
	}
}
