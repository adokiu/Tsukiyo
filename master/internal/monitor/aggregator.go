package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// MetricPoint 指标数据点
type MetricPoint struct {
	Timestamp     time.Time `json:"timestamp"`
	CPU           float64   `json:"cpu"`
	CPUMax        float64   `json:"cpu_max"`
	CPUMin        float64   `json:"cpu_min"`
	MemUsed       int64     `json:"mem_used"`
	MemUsedMax    int64     `json:"mem_used_max"`
	MemUsedMin    int64     `json:"mem_used_min"`
	MemTotal      int64     `json:"mem_total"`
	DiskUsed      int64     `json:"disk_used"`
	DiskUsedMax   int64     `json:"disk_used_max"`
	DiskUsedMin   int64     `json:"disk_used_min"`
	DiskTotal     int64     `json:"disk_total"`
	DiskReadBps   int64     `json:"disk_read_bps"`
	DiskReadMax   int64     `json:"disk_read_max"`
	DiskWriteBps  int64     `json:"disk_write_bps"`
	DiskWriteMax  int64     `json:"disk_write_max"`
	DiskReadIops  int64     `json:"disk_read_iops"`
	DiskWriteIops int64     `json:"disk_write_iops"`
	NetIn         int64     `json:"net_in"`
	NetInMax      int64     `json:"net_in_max"`
	NetInMin      int64     `json:"net_in_min"`
	NetOut        int64     `json:"net_out"`
	NetOutMax     int64     `json:"net_out_max"`
	NetOutMin     int64     `json:"net_out_min"`
	NetInTotal    int64     `json:"net_in_total"`
	NetOutTotal   int64     `json:"net_out_total"`
}

// GetInstanceMetrics 获取实例监控数据
// 15分钟内从 Redis 读原始数据，超过15分钟从 DB 读降采样数据
func GetInstanceMetrics(instanceID uuid.UUID, from, to time.Time, interval string) ([]MetricPoint, error) {
	now := time.Now()
	fifteenMinAgo := now.Add(-15 * time.Minute)

	var points []MetricPoint

	// 从 Redis 读取15分钟内的原始数据
	if to.After(fifteenMinAgo) {
		redisFrom := from
		if redisFrom.Before(fifteenMinAgo) {
			redisFrom = fifteenMinAgo
		}
		redisPoints, err := readMetricsFromRedis(instanceID, redisFrom, to)
		if err != nil {
			zap.L().Warn("从Redis读取监控数据失败", zap.Error(err))
		} else {
			points = append(points, redisPoints...)
		}
	}

	// 从 DB 读取15分钟前的降采样数据
	if from.Before(fifteenMinAgo) {
		dbTo := to
		if dbTo.After(fifteenMinAgo) {
			dbTo = fifteenMinAgo
		}
		dbPoints, err := readMetricsFromDB(instanceID, from, dbTo, interval)
		if err != nil {
			return nil, fmt.Errorf("查询监控数据失败: %w", err)
		}
		// DB 数据在前（时间更早），Redis 数据在后
		points = append(dbPoints, points...)
	}

	if len(points) == 0 {
		return []MetricPoint{}, nil
	}

	return points, nil
}

// readMetricsFromRedis 从 Redis sorted set 读取原始监控数据
func readMetricsFromRedis(instanceID uuid.UUID, from, to time.Time) ([]MetricPoint, error) {
	ctx := context.Background()
	rawKey := fmt.Sprintf("instance:%s:metrics_raw", instanceID)

	results, err := db.RedisClient.ZRangeByScore(ctx, rawKey, &redis.ZRangeBy{
		Min: fmt.Sprintf("%d", from.Unix()),
		Max: fmt.Sprintf("%d", to.Unix()),
	}).Result()
	if err != nil {
		return nil, err
	}

	points := make([]MetricPoint, 0, len(results))
	for _, s := range results {
		var m models.InstanceMetric
		if err := json.Unmarshal([]byte(s), &m); err != nil {
			continue
		}
		points = append(points, metricToPoint(m))
	}
	return points, nil
}

// readMetricsFromDB 从数据库读取降采样后的监控数据
func readMetricsFromDB(instanceID uuid.UUID, from, to time.Time, interval string) ([]MetricPoint, error) {
	var rawMetrics []models.InstanceMetric
	if err := db.DB.Where("instance_id = ? AND timestamp >= ? AND timestamp <= ?",
		instanceID, from, to).Order("timestamp ASC").Find(&rawMetrics).Error; err != nil {
		return nil, err
	}

	points := make([]MetricPoint, 0, len(rawMetrics))
	for _, m := range rawMetrics {
		points = append(points, metricToPoint(m))
	}
	return points, nil
}

// metricToPoint 将 InstanceMetric 转换为 MetricPoint
func metricToPoint(m models.InstanceMetric) MetricPoint {
	return MetricPoint{
		Timestamp:     m.Timestamp,
		CPU:           m.CPUPercent,
		CPUMax:        m.CPUMax,
		CPUMin:        m.CPUMin,
		MemUsed:       m.MemUsed,
		MemUsedMax:    m.MemUsedMax,
		MemUsedMin:    m.MemUsedMin,
		MemTotal:      m.MemTotal,
		DiskUsed:      m.DiskUsed,
		DiskUsedMax:   m.DiskUsedMax,
		DiskUsedMin:   m.DiskUsedMin,
		DiskTotal:     m.DiskTotal,
		DiskReadBps:   m.DiskReadBps,
		DiskReadMax:   m.DiskReadMax,
		DiskWriteBps:  m.DiskWriteBps,
		DiskWriteMax:  m.DiskWriteMax,
		DiskReadIops:  m.DiskReadIops,
		DiskWriteIops: m.DiskWriteIops,
		NetIn:         m.NetInBps,
		NetInMax:      m.NetInMax,
		NetInMin:      m.NetInMin,
		NetOut:        m.NetOutBps,
		NetOutMax:     m.NetOutMax,
		NetOutMin:     m.NetOutMin,
		NetInTotal:    m.NetInTotal,
		NetOutTotal:   m.NetOutTotal,
	}
}

// GetInstanceLatestMetrics 获取实例最新监控指标（从 Redis 读取）
func GetInstanceLatestMetrics(instanceID uuid.UUID) (*MetricPoint, error) {
	ctx := context.Background()
	rawKey := fmt.Sprintf("instance:%s:metrics_raw", instanceID)

	// 从 Redis sorted set 取最后一条
	results, err := db.RedisClient.ZRevRange(ctx, rawKey, 0, 0).Result()
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		// Redis 没有，回退到 DB
		var metric models.InstanceMetric
		if err := db.DB.Where("instance_id = ?", instanceID).
			Order("timestamp DESC").First(&metric).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, nil
			}
			return nil, err
		}
		p := metricToPoint(metric)
		return &p, nil
	}

	var m models.InstanceMetric
	if err := json.Unmarshal([]byte(results[0]), &m); err != nil {
		return nil, err
	}
	p := metricToPoint(m)
	return &p, nil
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

// DownsampleMetrics 降采样任务：把 Redis 中超过15分钟的数据按1分钟窗口聚合后写入 DB
// 保留均值、峰值、谷值，聚合后从 Redis 删除原始数据
func DownsampleMetrics() {
	ctx := context.Background()

	// 获取所有有原始数据的实例
	// 遍历所有实例的 metrics_raw key
	keys, err := db.RedisClient.Keys(ctx, "instance:*:metrics_raw").Result()
	if err != nil {
		zap.L().Error("扫描 Redis metrics_raw key 失败", zap.Error(err))
		return
	}

	now := time.Now()
	cutoff := now.Add(-15 * time.Minute)

	for _, key := range keys {
		// 从 key 解析 instance_id: instance:{id}:metrics_raw
		parts := splitKey(key)
		if len(parts) < 3 {
			continue
		}
		instanceID, err := uuid.Parse(parts[1])
		if err != nil {
			continue
		}

		// 取出15分钟前的所有数据
		results, err := db.RedisClient.ZRangeByScore(ctx, key, &redis.ZRangeBy{
			Min: "-inf",
			Max: fmt.Sprintf("%d", cutoff.Unix()),
		}).Result()
		if err != nil || len(results) == 0 {
			continue
		}

		// 按1分钟窗口聚合
		buckets := make(map[int64][]models.InstanceMetric)
		for _, s := range results {
			var m models.InstanceMetric
			if err := json.Unmarshal([]byte(s), &m); err != nil {
				continue
			}
			// 按分钟对齐
			bucketKey := m.Timestamp.Unix() / 60 * 60
			buckets[bucketKey] = append(buckets[bucketKey], m)
		}

		// 聚合每个窗口
		for bucketTime, samples := range buckets {
			agg := aggregateSamples(samples)
			agg.InstanceID = instanceID
			agg.Timestamp = time.Unix(bucketTime, 0)
			agg.SampleCount = len(samples)
			if err := db.DB.Create(&agg).Error; err != nil {
				zap.L().Error("写入降采样数据失败", zap.Error(err))
			}
		}

		// 从 Redis 删除已聚合的数据
		db.RedisClient.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", cutoff.Unix()))
	}

	zap.L().Info("降采样任务完成", zap.Int("keys", len(keys)))
}

// aggregateSamples 聚合一组样本：计算均值、峰值、谷值
func aggregateSamples(samples []models.InstanceMetric) models.InstanceMetric {
	if len(samples) == 0 {
		return models.InstanceMetric{}
	}

	var cpuSum float64
	var memSum, diskSum, diskReadSum, diskWriteSum, diskReadIopsSum, diskWriteIopsSum, netInSum, netOutSum int64
	var memTotal, diskTotal, netInTotal, netOutTotal int64

	cpuMax := samples[0].CPUPercent
	cpuMin := samples[0].CPUPercent
	memMax := samples[0].MemUsed
	memMin := samples[0].MemUsed
	diskMax := samples[0].DiskUsed
	diskMin := samples[0].DiskUsed
	diskReadMax := samples[0].DiskReadBps
	diskWriteMax := samples[0].DiskWriteBps
	netInMax := samples[0].NetInBps
	netInMin := samples[0].NetInBps
	netOutMax := samples[0].NetOutBps
	netOutMin := samples[0].NetOutBps

	for _, s := range samples {
		cpuSum += s.CPUPercent
		memSum += s.MemUsed
		diskSum += s.DiskUsed
		diskReadSum += s.DiskReadBps
		diskWriteSum += s.DiskWriteBps
		diskReadIopsSum += s.DiskReadIops
		diskWriteIopsSum += s.DiskWriteIops
		netInSum += s.NetInBps
		netOutSum += s.NetOutBps
		memTotal = s.MemTotal
		diskTotal = s.DiskTotal
		netInTotal = s.NetInTotal
		netOutTotal = s.NetOutTotal

		if s.CPUPercent > cpuMax {
			cpuMax = s.CPUPercent
		}
		if s.CPUPercent < cpuMin {
			cpuMin = s.CPUPercent
		}
		if s.MemUsed > memMax {
			memMax = s.MemUsed
		}
		if s.MemUsed < memMin {
			memMin = s.MemUsed
		}
		if s.DiskUsed > diskMax {
			diskMax = s.DiskUsed
		}
		if s.DiskUsed < diskMin {
			diskMin = s.DiskUsed
		}
		if s.DiskReadBps > diskReadMax {
			diskReadMax = s.DiskReadBps
		}
		if s.DiskWriteBps > diskWriteMax {
			diskWriteMax = s.DiskWriteBps
		}
		if s.NetInBps > netInMax {
			netInMax = s.NetInBps
		}
		if s.NetInBps < netInMin {
			netInMin = s.NetInBps
		}
		if s.NetOutBps > netOutMax {
			netOutMax = s.NetOutBps
		}
		if s.NetOutBps < netOutMin {
			netOutMin = s.NetOutBps
		}
	}

	n := int64(len(samples))
	return models.InstanceMetric{
		NodeID:        samples[0].NodeID,
		CPUPercent:    cpuSum / float64(n),
		CPUMax:        cpuMax,
		CPUMin:        cpuMin,
		MemUsed:       memSum / n,
		MemUsedMax:    memMax,
		MemUsedMin:    memMin,
		MemTotal:      memTotal,
		DiskUsed:      diskSum / n,
		DiskUsedMax:   diskMax,
		DiskUsedMin:   diskMin,
		DiskTotal:     diskTotal,
		DiskReadBps:   diskReadSum / n,
		DiskReadMax:   diskReadMax,
		DiskWriteBps:  diskWriteSum / n,
		DiskWriteMax:  diskWriteMax,
		DiskReadIops:  diskReadIopsSum / n,
		DiskWriteIops: diskWriteIopsSum / n,
		NetInBps:      netInSum / n,
		NetInMax:      netInMax,
		NetInMin:      netInMin,
		NetOutBps:     netOutSum / n,
		NetOutMax:     netOutMax,
		NetOutMin:     netOutMin,
		NetInTotal:    netInTotal,
		NetOutTotal:   netOutTotal,
	}
}

// splitKey 分割 Redis key，返回各部分
func splitKey(key string) []string {
	var parts []string
	current := ""
	for _, c := range key {
		if c == ':' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	parts = append(parts, current)
	return parts
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
