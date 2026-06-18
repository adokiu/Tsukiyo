package schedule

import (
	"context"
	"time"

	"go.uber.org/zap"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/monitor"
)

// Scheduler 定时任务调度器
type Scheduler struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// NewScheduler 创建定时任务调度器
func NewScheduler() *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{ctx: ctx, cancel: cancel}
}

// Start 启动所有定时任务
func (s *Scheduler) Start() {
	zap.L().Info("定时任务调度器启动")

	// 实例到期检查 (每5分钟)
	go s.runTicker(5*time.Minute, s.checkExpiredInstances)

	// 流量超额检测 (每10分钟)
	go s.runTicker(10*time.Minute, s.checkTrafficOverLimit)

	// 快照自动清理 (每小时)
	go s.runTicker(1*time.Hour, s.cleanupExpiredSnapshots)

	// 流量月度重置 (每天凌晨1点)
	go s.runDailyAt(1, 0, s.resetMonthlyTraffic)

	// 监控数据清理 (每天凌晨3点，保留30天)
	go s.runDailyAt(3, 0, s.cleanupMetrics)
}

// Stop 停止定时任务
func (s *Scheduler) Stop() {
	s.cancel()
	zap.L().Info("定时任务调度器停止")
}

// runTicker 通用定时器
func (s *Scheduler) runTicker(interval time.Duration, fn func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			fn()
		}
	}
}

// runDailyAt 每天在指定时间执行
func (s *Scheduler) runDailyAt(hour, minute int, fn func()) {
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}

		select {
		case <-s.ctx.Done():
			return
		case <-time.After(next.Sub(now)):
			fn()
		}
	}
}

// checkExpiredInstances 检查到期实例
func (s *Scheduler) checkExpiredInstances() {
	var instances []models.Instance
	if err := db.DB.Where("expires_at IS NOT NULL AND expires_at < ? AND status != ?",
		time.Now(), models.InstanceStatusExpired).Find(&instances).Error; err != nil {
		zap.L().Error("查询到期实例失败", zap.Error(err))
		return
	}

	for _, inst := range instances {
		zap.L().Info("实例已到期", zap.String("instance_id", inst.ID.String()))
		db.DB.Model(&inst).Updates(map[string]interface{}{
			"status":     models.InstanceStatusExpired,
			"expired_at": time.Now(),
		})
	}
}

// checkTrafficOverLimit 检查流量超额实例
func (s *Scheduler) checkTrafficOverLimit() {
	var instances []models.Instance
	if err := db.DB.Where("monthly_traffic_gb > 0 AND traffic_used_gb >= monthly_traffic_gb AND is_over_limit = ?",
		false).Find(&instances).Error; err != nil {
		zap.L().Error("查询流量超额实例失败", zap.Error(err))
		return
	}

	for _, inst := range instances {
		zap.L().Info("实例流量超额",
			zap.String("instance_id", inst.ID.String()),
			zap.Float64("used", inst.TrafficUsedGB),
			zap.Int64("limit", inst.MonthlyTrafficGB))

		updates := map[string]interface{}{
			"is_over_limit": true,
		}

		if inst.OverLimitAction == models.OverLimitActionShutdown {
			updates["status"] = models.InstanceStatusStopped
			// 下发停止任务
			task := models.Task{
				ID:         inst.ID,
				Type:       models.TaskTypeStopInstance,
				NodeID:     inst.NodeID,
				InstanceID: &inst.ID,
				UserID:     inst.UserID,
				Status:     models.TaskStatusPending,
			}
			db.DB.Create(&task)
		}

		db.DB.Model(&inst).Updates(updates)
	}
}

// cleanupExpiredSnapshots 清理超期快照
func (s *Scheduler) cleanupExpiredSnapshots() {
	// 删除超过快照上限的旧快照
	var instances []models.Instance
	db.DB.Find(&instances)

	for _, inst := range instances {
		var count int64
		db.DB.Model(&models.Snapshot{}).Where("instance_id = ?", inst.ID).Count(&count)

		if int(count) > inst.SnapshotLimit {
			var oldSnapshots []models.Snapshot
			db.DB.Where("instance_id = ? AND is_scheduled = ?", inst.ID, false).
				Order("created_at ASC").
				Limit(int(count) - inst.SnapshotLimit).
				Find(&oldSnapshots)

			for _, snap := range oldSnapshots {
				zap.L().Info("删除超期快照",
					zap.String("instance_id", inst.ID.String()),
					zap.String("snapshot", snap.Name))

				// 下发删除快照任务
				task := models.Task{
					ID:         snap.ID,
					Type:       models.TaskTypeDeleteSnapshot,
					NodeID:     inst.NodeID,
					InstanceID: &inst.ID,
					UserID:     inst.UserID,
					Status:     models.TaskStatusPending,
				}
				db.DB.Create(&task)

				// 删除数据库记录
				db.DB.Delete(&snap)
			}
		}
	}
}

// resetMonthlyTraffic 重置月度流量
func (s *Scheduler) resetMonthlyTraffic() {
	now := time.Now()
	currentMonth := now.Format("2006-01")

	// 查找流量重置月份不等于当前月的实例
	var instances []models.Instance
	if err := db.DB.Where("monthly_traffic_gb > 0 AND traffic_reset_date != ?", currentMonth).
		Find(&instances).Error; err != nil {
		zap.L().Error("查询流量重置实例失败", zap.Error(err))
		return
	}

	for _, inst := range instances {
		zap.L().Info("重置月度流量", zap.String("instance_id", inst.ID.String()))
		db.DB.Model(&inst).Updates(map[string]interface{}{
			"traffic_used_gb":  0,
			"traffic_reset_date": currentMonth,
			"is_over_limit":    false,
		})
	}
}

// cleanupMetrics 清理过期监控数据
func (s *Scheduler) cleanupMetrics() {
	monitor.CleanupOldMetrics()
}
