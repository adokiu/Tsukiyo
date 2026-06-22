package agent

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// readPump 读取 Agent 消息
func (c *Connection) readPump(m *Manager) {
	defer func() {
		c.Close()
		m.removeConnection(c.NodeID)
	}()

	c.Conn.SetReadLimit(65536)
	c.Conn.SetPongHandler(func(string) error {
		c.LastPing = time.Now()
		c.Conn.SetReadDeadline(time.Now().Add(65 * time.Second))
		return nil
	})

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		c.Conn.SetReadDeadline(time.Now().Add(65 * time.Second))
		_, msgBytes, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				zap.L().Warn("Agent 连接异常断开", zap.String("node_id", c.NodeID.String()), zap.Error(err))
			}
			return
		}

		c.LastPing = time.Now()

		var msg struct {
			Type    string          `json:"type"`
			ID      string          `json:"id,omitempty"`
			Payload json.RawMessage `json:"payload,omitempty"`
			Error   string          `json:"error,omitempty"`
		}
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			zap.L().Warn("解析 Agent 消息失败", zap.String("node_id", c.NodeID.String()), zap.Error(err))
			continue
		}

		switch msg.Type {
		case "heartbeat":
			m.handleHeartbeat(c.NodeID, msg.Payload)
		case "instance_status":
			m.handleInstanceStatus(c.NodeID, msg.Payload)
		case "metrics":
			m.handleMetrics(c.NodeID, msg.Payload)
		case "task_result":
			m.handleTaskResult(c.NodeID, msg.Payload)
		case "response":
			m.handleResponse(msg.ID, msg.Payload, msg.Error)
		case "image_progress":
			m.handleImageProgress(c.NodeID, msg.Payload)
		case "instance_progress":
			m.handleInstanceProgress(c.NodeID, msg.Payload)
		case "task_log":
			m.handleTaskLog(c.NodeID, msg.Payload)
		case "security_alert":
			m.handleSecurityAlert(c.NodeID, msg.Payload)
		case "verify_console_token":
			m.handleVerifyConsoleToken(c, msg.ID, msg.Payload)
		case "console_data", "console_error":
			m.handleConsoleData(msg.Type, msg.Payload)
		case "console_vnc_data", "console_vnc_error":
			m.handleVNCData(msg.Type, msg.Payload)
		default:
			zap.L().Debug("收到未处理的 Agent 消息", zap.String("type", msg.Type), zap.String("node_id", c.NodeID.String()))
		}
	}
}

// writePump 写入 Agent 消息
func (c *Connection) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case message, ok := <-c.SendCh:
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.mu.Lock()
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			err := c.Conn.WriteMessage(websocket.TextMessage, message)
			c.mu.Unlock()
			if err != nil {
				return
			}

		case <-ticker.C:
			c.mu.Lock()
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			err := c.Conn.WriteMessage(websocket.PingMessage, nil)
			c.mu.Unlock()
			if err != nil {
				return
			}

		case <-c.ctx.Done():
			return
		}
	}
}

// Close 关闭连接
func (c *Connection) Close() {
	c.cancel()
	c.mu.Lock()
	if c.Conn != nil {
		c.Conn.Close()
	}
	c.mu.Unlock()
}

// CloseAll 关闭所有 Agent 和前端 WebSocket 连接
func (m *Manager) CloseAll() {
	m.mu.Lock()
	for _, conn := range m.connections {
		conn.Close()
	}
	m.connections = make(map[uuid.UUID]*Connection)
	m.mu.Unlock()

	m.frontendMu.Lock()
	for _, fc := range m.frontendConns {
		if fc.Conn != nil {
			fc.Conn.Close()
		}
	}
	m.frontendConns = nil
	m.frontendMu.Unlock()
}

// Send 发送消息给 Agent
func (c *Connection) Send(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case c.SendCh <- data:
		return nil
	case <-c.ctx.Done():
		return fmt.Errorf("连接已关闭")
	case <-time.After(5 * time.Second):
		return fmt.Errorf("发送超时")
	}
}
