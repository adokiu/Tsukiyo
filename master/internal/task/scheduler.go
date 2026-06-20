package task

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"tsukiyo/master/internal/agent"
	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/service/infrastructure"
)

// Scheduler 任务调度器
type Scheduler struct {
	mgr        *agent.Manager
	networkSvc *infrastructure.NetworkService
	interval   time.Duration
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewScheduler 创建任务调度器
func NewScheduler(mgr *agent.Manager, networkSvc *infrastructure.NetworkService) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		mgr:        mgr,
		networkSvc: networkSvc,
		interval:   2 * time.Second,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start 启动调度循环
func (s *Scheduler) Start() {
	zap.L().Info("任务调度器启动")
	go s.loop()
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.cancel()
	zap.L().Info("任务调度器停止")
}

// loop 调度主循环
func (s *Scheduler) loop() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.dispatchPendingTasks(); err != nil {
				zap.L().Error("调度任务失败", zap.Error(err))
			}
		}
	}
}

// dispatchPendingTasks 分发 pending 任务
func (s *Scheduler) dispatchPendingTasks() error {
	var tasks []models.Task
	if err := db.DB.Where("status = ?", models.TaskStatusPending).
		Order("created_at ASC").
		Limit(50).
		Find(&tasks).Error; err != nil {
		return fmt.Errorf("查询 pending 任务失败: %w", err)
	}

	for _, task := range tasks {
		if err := s.processTask(&task); err != nil {
			zap.L().Warn("处理任务失败",
				zap.String("task_id", task.ID.String()),
				zap.String("task_type", string(task.Type)),
				zap.Error(err))
			s.markTaskFailed(&task, err.Error())
		}
	}

	return nil
}

// processTask 处理单个任务
func (s *Scheduler) processTask(task *models.Task) error {
	// 检查节点是否在线
	if !s.mgr.IsNodeConnected(task.NodeID) {
		return fmt.Errorf("节点 %s 未连接", task.NodeID)
	}

	// 对 create_instance 任务做幂等性检查：如果实例已经 running/stopped，直接标记完成
	if task.Type == models.TaskTypeCreateInstance && task.InstanceID != nil {
		var instance models.Instance
		if err := db.DB.Where("id = ?", *task.InstanceID).First(&instance).Error; err == nil {
			if instance.Status == models.InstanceStatusRunning || instance.Status == models.InstanceStatusStopped {
				zap.L().Info("实例已存在，跳过创建任务",
					zap.String("task_id", task.ID.String()),
					zap.String("instance_id", task.InstanceID.String()),
					zap.String("status", string(instance.Status)))
				now := time.Now()
				db.DB.Model(task).Updates(map[string]interface{}{
					"status":       models.TaskStatusCompleted,
					"completed_at": &now,
					"result":       `{"skipped":true,"reason":"instance_already_exists"}`,
				})
				return nil
			}
		}
	}

	// 更新任务状态为 running
	now := time.Now()
	if err := db.DB.Model(task).Updates(map[string]interface{}{
		"status":     models.TaskStatusRunning,
		"started_at": &now,
	}).Error; err != nil {
		return fmt.Errorf("更新任务状态失败: %w", err)
	}

	// 构建 Agent 任务消息
	msg := agent.TaskMessage{
		NodeID:  task.NodeID,
		TaskID:  task.ID,
		Type:    string(task.Type),
		Payload: task.Payload,
	}

	// 发送任务到 Agent
	if err := s.mgr.SendTask(msg); err != nil {
		// 发送失败，恢复 pending 状态等待重试
		db.DB.Model(task).Update("status", models.TaskStatusPending)
		return fmt.Errorf("发送任务到 Agent 失败: %w", err)
	}

	zap.L().Info("任务已下发",
		zap.String("task_id", task.ID.String()),
		zap.String("type", string(task.Type)),
		zap.String("node_id", task.NodeID.String()))

	return nil
}

// markTaskFailed 标记任务失败
func (s *Scheduler) markTaskFailed(task *models.Task, errMsg string) {
	now := time.Now()
	updates := map[string]interface{}{
		"error":        errMsg,
		"retry_count":  gorm.Expr("retry_count + 1"),
		"completed_at": &now,
	}

	// 有副作用的操作失败后不再重试，直接标记为 failed
	nonRetryableTypes := map[models.TaskType]bool{
		models.TaskTypeCreateInstance:    true,
		models.TaskTypeDeleteInstance:    true,
		models.TaskTypeReinstallInstance: true,
		models.TaskTypeResizeInstance:    true,
		models.TaskTypeCreatePartition:   true,
		models.TaskTypeDeletePartition:   true,
		models.TaskTypeFormatDisk:        true,
		models.TaskTypeInitStorage:       true,
	}
	if nonRetryableTypes[task.Type] || task.RetryCount+1 >= task.MaxRetries {
		updates["status"] = models.TaskStatusFailed
	} else {
		updates["status"] = models.TaskStatusPending
	}

	db.DB.Model(task).Updates(updates)
}

// HandleTaskResult 处理 Agent 返回的任务结果 (由 agent manager 调用)
func (s *Scheduler) HandleTaskResult(taskID uuid.UUID, result json.RawMessage, errMsg string) {
	var task models.Task
	if err := db.DB.Where("id = ?", taskID).First(&task).Error; err != nil {
		zap.L().Warn("找不到任务", zap.String("task_id", taskID.String()))
		return
	}

	now := time.Now()
	updates := map[string]interface{}{
		"completed_at": &now,
	}

	if errMsg != "" {
		updates["status"] = models.TaskStatusFailed
		updates["error"] = errMsg
		updates["retry_count"] = gorm.Expr("retry_count + 1")
		// create_instance 有副作用，失败后不重试
		nonRetryableTypes := map[models.TaskType]bool{
			models.TaskTypeCreateInstance:    true,
			models.TaskTypeDeleteInstance:    true,
			models.TaskTypeReinstallInstance: true,
			models.TaskTypeResizeInstance:    true,
			models.TaskTypeCreatePartition:   true,
			models.TaskTypeDeletePartition:   true,
			models.TaskTypeFormatDisk:        true,
			models.TaskTypeInitStorage:       true,
			models.TaskTypeDeleteStorage:     true,
		}
		if !nonRetryableTypes[task.Type] && task.RetryCount+1 < task.MaxRetries {
			updates["status"] = models.TaskStatusPending
			updates["completed_at"] = nil
		}
	} else {
		updates["status"] = models.TaskStatusCompleted
		updates["result"] = string(result)
	}

	if err := db.DB.Model(&task).Updates(updates).Error; err != nil {
		zap.L().Error("更新任务结果失败", zap.String("task_id", taskID.String()), zap.Error(err))
	}

	// 根据任务类型执行后续操作
	s.handlePostTask(&task, errMsg == "")
}

// handlePostTask 任务完成后的后续处理
func (s *Scheduler) handlePostTask(task *models.Task, success bool) {
	switch task.Type {
	case models.TaskTypeCreateInstance:
		if task.InstanceID == nil {
			return
		}
		if success {
			instanceUpdates := map[string]interface{}{
				"status": models.InstanceStatusRunning,
			}
			// 解析 Agent 回传的 instance_ip（端口映射已由 Master 提前分配，无需 Agent 回传）
			if len(task.Result) > 0 {
				var result struct {
					InstanceIP string `json:"instance_ip"`
				}
				if err := json.Unmarshal(task.Result, &result); err == nil {
					if result.InstanceIP != "" {
						instanceUpdates["internal_ipv4"] = &result.InstanceIP
					}
				}
			}
			db.DB.Model(&models.Instance{}).Where("id = ?", *task.InstanceID).Updates(instanceUpdates)
		} else {
			db.DB.Model(&models.Instance{}).Where("id = ?", *task.InstanceID).Updates(map[string]interface{}{
				"status": models.InstanceStatusError,
			})
		}
	case models.TaskTypeDeleteInstance:
		if task.InstanceID != nil {
			if success {
				// 释放所有网络资源（EIP、端口映射、防火墙规则）
				s.networkSvc.ReleaseInstanceNetworkResources(*task.InstanceID)
				db.DB.Delete(&models.Instance{}, *task.InstanceID)
				db.DB.Where("instance_id = ?", *task.InstanceID).Delete(&models.DataDisk{})
			} else {
				// 删除失败：恢复实例状态，不删除记录
				oldStatus := models.InstanceStatusError
				var payload struct {
					OldStatus string `json:"old_status"`
				}
				if err := json.Unmarshal(task.Payload, &payload); err == nil && payload.OldStatus != "" {
					oldStatus = models.InstanceStatus(payload.OldStatus)
				}
				db.DB.Model(&models.Instance{}).Where("id = ?", *task.InstanceID).
					Update("status", oldStatus)
				zap.L().Warn("实例删除失败，恢复状态", zap.String("task_id", task.ID.String()), zap.String("old_status", string(oldStatus)))
			}
		}
	case models.TaskTypeReinstallInstance:
		if task.InstanceID != nil {
			if success {
				updates := map[string]interface{}{
					"status": models.InstanceStatusRunning,
				}
				var payload struct {
					TemplateID  string `json:"template_id"`
					Password    string `json:"password"`
					LoginMethod string `json:"login_method"`
				}
				if err := json.Unmarshal(task.Payload, &payload); err == nil {
					if payload.TemplateID != "" {
						updates["template_id"] = payload.TemplateID
					}
					if payload.Password != "" {
						updates["ssh_password"] = payload.Password
					}
					if payload.LoginMethod != "" {
						updates["login_method"] = payload.LoginMethod
					}
				}
				db.DB.Model(&models.Instance{}).Where("id = ?", *task.InstanceID).Updates(updates)
			} else {
				oldStatus := models.InstanceStatusError
				var payload struct {
					OldStatus string `json:"old_status"`
				}
				if err := json.Unmarshal(task.Payload, &payload); err == nil && payload.OldStatus != "" {
					oldStatus = models.InstanceStatus(payload.OldStatus)
				}
				db.DB.Model(&models.Instance{}).Where("id = ?", *task.InstanceID).
					Update("status", oldStatus)
			}
		}
	case models.TaskTypeStartInstance:
		if task.InstanceID != nil {
			if success {
				db.DB.Model(&models.Instance{}).Where("id = ?", *task.InstanceID).
					Update("status", models.InstanceStatusRunning)
			}
		}
	case models.TaskTypeStopInstance:
		if task.InstanceID != nil {
			if success {
				db.DB.Model(&models.Instance{}).Where("id = ?", *task.InstanceID).
					Update("status", models.InstanceStatusStopped)
			}
		}
	case models.TaskTypeRestartInstance:
		if task.InstanceID != nil {
			if success {
				db.DB.Model(&models.Instance{}).Where("id = ?", *task.InstanceID).
					Update("status", models.InstanceStatusRunning)
			}
		}
	case models.TaskTypeDeleteStorage:
		if success {
			var payload struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(task.Payload, &payload); err == nil && payload.Name != "" {
				db.DB.Model(&models.Node{}).Where("id = ? AND default_storage_pool = ?", task.NodeID, payload.Name).
					Update("default_storage_pool", "")
			}
		}
	}
}
