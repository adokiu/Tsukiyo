package agent

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// handleTaskResult 处理任务结果
func (m *Manager) handleTaskResult(nodeID uuid.UUID, payload json.RawMessage) {
	var result struct {
		TaskID  string          `json:"task_id"`
		Success bool            `json:"success"`
		Payload json.RawMessage `json:"payload,omitempty"`
		Error   string          `json:"error,omitempty"`
	}
	if err := json.Unmarshal(payload, &result); err != nil {
		zap.L().Error("解析任务结果失败", zap.Error(err))
		return
	}

	taskID, err := uuid.Parse(result.TaskID)
	if err != nil {
		zap.L().Error("解析任务 ID 失败", zap.Error(err))
		return
	}

	zap.L().Info("收到任务结果",
		zap.String("task_id", result.TaskID),
		zap.String("node_id", nodeID.String()),
		zap.Bool("success", result.Success))

	// 查询任务信息
	var task models.Task
	if err := db.DB.Where("id = ? AND node_id = ?", taskID, nodeID).First(&task).Error; err != nil {
		zap.L().Error("查询任务失败", zap.String("task_id", result.TaskID), zap.Error(err))
		return
	}

	// 记录任务日志
	logLevel := "info"
	if !result.Success {
		logLevel = "error"
	}
	logMessage := fmt.Sprintf("任务执行%s", map[bool]string{true: "成功", false: "失败"}[result.Success])
	if !result.Success && result.Error != "" {
		logMessage += fmt.Sprintf(": %s", result.Error)
	}

	taskLog := models.TaskLog{
		TaskID:  taskID,
		Level:   logLevel,
		Message: logMessage,
	}
	if err := db.DB.Create(&taskLog).Error; err != nil {
		zap.L().Error("创建任务日志失败", zap.Error(err))
	}

	// 如果有外部回调，优先交给外部处理 (task scheduler)
	if m.OnTaskResult != nil {
		errMsg := ""
		if !result.Success {
			errMsg = result.Error
		}
		m.OnTaskResult(taskID, result.Payload, errMsg)

		// 广播任务状态到前端（OnTaskResult 路径也需要广播）
		var finalStatus models.TaskStatus
		if result.Success {
			finalStatus = models.TaskStatusCompleted
		} else {
			finalStatus = models.TaskStatusFailed
		}
		m.broadcastTaskStatus(taskID, task.Type, nodeID, finalStatus, result.Error)
		return
	}

	// 默认处理：直接更新数据库
	updates := map[string]interface{}{
		"status":       models.TaskStatusCompleted,
		"result":       result.Payload,
		"completed_at": time.Now(),
	}
	if !result.Success {
		updates["status"] = models.TaskStatusFailed
		updates["error"] = result.Error
	}

	if err := db.DB.Model(&models.Task{}).Where("id = ? AND node_id = ?", taskID, nodeID).Updates(updates).Error; err != nil {
		zap.L().Error("更新任务状态失败", zap.String("task_id", result.TaskID), zap.Error(err))
		return
	}

	// WebSocket 推送任务状态更新
	m.broadcastTaskStatus(taskID, task.Type, nodeID, updates["status"].(models.TaskStatus), result.Error)
}

// broadcastTaskStatus 向前端广播任务状态更新
func (m *Manager) broadcastTaskStatus(taskID uuid.UUID, taskType models.TaskType, nodeID uuid.UUID, status models.TaskStatus, errMsg string) {
	m.frontendMu.RLock()
	defer m.frontendMu.RUnlock()

	payload := map[string]interface{}{
		"type":      "task_status",
		"task_id":   taskID.String(),
		"task_type": string(taskType),
		"node_id":   nodeID.String(),
		"status":    string(status),
		"error":     errMsg,
		"timestamp": time.Now().Unix(),
	}

	payloadBytes, _ := json.Marshal(payload)
	for _, conn := range m.frontendConns {
		if conn != nil && conn.Conn != nil {
			conn.Conn.WriteMessage(websocket.TextMessage, payloadBytes)
		}
	}
}

// handleTaskLog 处理 Agent 上报的任务日志
func (m *Manager) handleTaskLog(nodeID uuid.UUID, payload json.RawMessage) {
	var log struct {
		TaskID  string `json:"task_id"`
		Level   string `json:"level"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(payload, &log); err != nil {
		zap.L().Error("解析任务日志失败", zap.Error(err))
		return
	}

	taskID, err := uuid.Parse(log.TaskID)
	if err != nil {
		zap.L().Error("解析任务 ID 失败", zap.String("task_id", log.TaskID), zap.Error(err))
		return
	}

	taskLog := models.TaskLog{
		TaskID:  taskID,
		Level:   log.Level,
		Message: log.Message,
	}
	if err := db.DB.Create(&taskLog).Error; err != nil {
		zap.L().Error("创建任务日志失败", zap.Error(err))
		return
	}

	// WebSocket 推送任务日志给前端
	m.broadcastTaskLog(taskID, log.Level, log.Message, taskLog.CreatedAt)
}

// broadcastTaskLog 向前端广播任务日志
func (m *Manager) broadcastTaskLog(taskID uuid.UUID, level string, message string, createdAt time.Time) {
	m.frontendMu.RLock()
	defer m.frontendMu.RUnlock()

	payload := map[string]interface{}{
		"type":      "task_log",
		"task_id":   taskID.String(),
		"level":     level,
		"message":   message,
		"timestamp": createdAt.Unix(),
	}

	payloadBytes, _ := json.Marshal(payload)
	for _, conn := range m.frontendConns {
		if conn != nil && conn.Conn != nil {
			conn.Conn.WriteMessage(websocket.TextMessage, payloadBytes)
		}
	}
}

// SendTask 发送任务给指定 Agent (接受 TaskMessage)
func (m *Manager) SendTask(msg TaskMessage) error {
	m.mu.RLock()
	conn, exists := m.connections[msg.NodeID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("节点 %s 未连接", msg.NodeID)
	}

	// 记录任务开始日志
	taskLog := models.TaskLog{
		TaskID:  msg.TaskID,
		Level:   "info",
		Message: fmt.Sprintf("任务已下发到节点 %s", msg.NodeID.String()),
	}
	if err := db.DB.Create(&taskLog).Error; err != nil {
		zap.L().Error("创建任务日志失败", zap.Error(err))
	}

	// 更新任务状态为 running
	now := time.Now()
	if err := db.DB.Model(&models.Task{}).Where("id = ?", msg.TaskID).Updates(map[string]interface{}{
		"status":     models.TaskStatusRunning,
		"started_at": now,
	}).Error; err != nil {
		zap.L().Error("更新任务状态失败", zap.String("task_id", msg.TaskID.String()), zap.Error(err))
	} else {
		// WebSocket 推送任务状态更新
		var task models.Task
		if err := db.DB.Where("id = ?", msg.TaskID).First(&task).Error; err == nil {
			m.broadcastTaskStatus(msg.TaskID, task.Type, msg.NodeID, models.TaskStatusRunning, "")
		}
	}

	// Agent readLoop 期望外层格式为 {type: "task", payload: <TaskMessage>}
	wrapper := struct {
		Type    string      `json:"type"`
		Payload TaskMessage `json:"payload"`
	}{
		Type:    "task",
		Payload: msg,
	}

	msgData, err := json.Marshal(wrapper)
	if err != nil {
		return err
	}

	select {
	case conn.SendCh <- msgData:
		return nil
	case <-conn.ctx.Done():
		return fmt.Errorf("连接已关闭")
	case <-time.After(5 * time.Second):
		return fmt.Errorf("发送超时")
	}
}

// IsNodeConnected 检查节点是否在线
func (m *Manager) IsNodeConnected(nodeID uuid.UUID) bool {
	return m.IsAgentConnected(nodeID)
}

// IsAgentConnected 检查 Agent 是否在线
func (m *Manager) IsAgentConnected(nodeID uuid.UUID) bool {
	m.mu.RLock()
	_, exists := m.connections[nodeID]
	m.mu.RUnlock()
	return exists
}

// GetConnectedNodes 获取所有在线节点
func (m *Manager) GetConnectedNodes() []uuid.UUID {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]uuid.UUID, 0, len(m.connections))
	for nodeID := range m.connections {
		nodes = append(nodes, nodeID)
	}
	return nodes
}
