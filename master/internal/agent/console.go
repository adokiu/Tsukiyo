package agent

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"tsukiyo/master/internal/console"
	"tsukiyo/master/internal/db"
)

// handleVerifyConsoleToken 处理 Agent 发来的 Token 验证请求
func (m *Manager) handleVerifyConsoleToken(c *Connection, msgID string, payload json.RawMessage) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		m.sendResponseToAgent(c, msgID, map[string]interface{}{"valid": false, "error": "invalid request"})
		return
	}

	sessionKey := "console_token:" + req.Token
	result, err := db.RedisClient.Get(context.Background(), sessionKey).Result()
	valid := err == nil && result != ""

	m.sendResponseToAgent(c, msgID, map[string]interface{}{"valid": valid})
}

// handleConsoleData 处理 Agent 发来的控制台数据，转发到前端 WS
func (m *Manager) handleConsoleData(msgType string, payload json.RawMessage) {
	var msg struct {
		SessionID string `json:"session_id"`
		Stream    string `json:"stream"`
		Data      string `json:"data"`
		Error     string `json:"error"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		zap.L().Warn("解析控制台数据失败", zap.Error(err))
		return
	}

	m.consoleMu.RLock()
	conn, ok := m.consoleSessions[msg.SessionID]
	m.consoleMu.RUnlock()
	if !ok || conn == nil {
		return
	}

	// 转发到前端 WS
	var wsMsg []byte
	if msgType == "console_error" {
		wsMsg, _ = json.Marshal(map[string]interface{}{
			"type":    "error",
			"message": msg.Error,
		})
	} else if msg.Stream == "exit" {
		wsMsg, _ = json.Marshal(map[string]interface{}{
			"type": "exit",
		})
	} else {
		wsMsg, _ = json.Marshal(map[string]interface{}{
			"type":   "data",
			"stream": msg.Stream,
			"data":   msg.Data,
		})
	}
	conn.WriteMessage(websocket.TextMessage, wsMsg)
}

// HandleConsoleWebSocket 处理前端控制台 WebSocket 连接
func (m *Manager) HandleConsoleWebSocket(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少 token"})
		return
	}

	// 验证并消费 token（一次性，30秒有效）
	session, err := console.ConsumeConsoleToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token 无效或已过期"})
		return
	}

	// 升级为 WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		zap.L().Error("控制台 WebSocket 升级失败", zap.Error(err))
		return
	}
	defer conn.Close()

	nodeID, err := uuid.Parse(session.NodeID)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","message":"无效的节点ID"}`))
		return
	}

	// 检查 Agent 是否在线
	m.mu.RLock()
	agentConn, exists := m.connections[nodeID]
	m.mu.RUnlock()
	if !exists {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","message":"节点离线"}`))
		return
	}

	sessionID := uuid.New().String()

	// 注册前端 WS 连接
	m.consoleMu.Lock()
	m.consoleSessions[sessionID] = conn
	m.consoleMu.Unlock()
	defer func() {
		m.consoleMu.Lock()
		delete(m.consoleSessions, sessionID)
		m.consoleMu.Unlock()
	}()

	// 向 Agent 发送 console_ssh_start
	if err := agentConn.Send(struct {
		Type    string      `json:"type"`
		Payload interface{} `json:"payload"`
	}{
		Type: "console_ssh_start",
		Payload: map[string]interface{}{
			"session_id": sessionID,
			"container":  session.IncusName,
		},
	}); err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","message":"发送控制台请求失败"}`))
		return
	}

	// 读取前端输入，转发到 Agent
	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var input struct {
			Type string `json:"type"`
			Data string `json:"data"`
			Cols int    `json:"cols"`
			Rows int    `json:"rows"`
		}
		if err := json.Unmarshal(msgBytes, &input); err != nil {
			// 直接当作原始输入
			input.Type = "input"
			input.Data = string(msgBytes)
		}

		if input.Type == "input" {
			agentConn.Send(struct {
				Type    string      `json:"type"`
				Payload interface{} `json:"payload"`
			}{
				Type: "console_ssh_input",
				Payload: map[string]interface{}{
					"session_id": sessionID,
					"data":       input.Data,
				},
			})
		} else if input.Type == "resize" {
			agentConn.Send(struct {
				Type    string      `json:"type"`
				Payload interface{} `json:"payload"`
			}{
				Type: "console_ssh_resize",
				Payload: map[string]interface{}{
					"session_id": sessionID,
					"cols":       input.Cols,
					"rows":       input.Rows,
				},
			})
		}
	}

	// 前端断开，通知 Agent 关闭会话
	agentConn.Send(struct {
		Type    string      `json:"type"`
		Payload interface{} `json:"payload"`
	}{
		Type: "console_ssh_close",
		Payload: map[string]interface{}{
			"session_id": sessionID,
		},
	})
}

// sendResponseToAgent 向 Agent 连接发送 response 消息
func (m *Manager) sendResponseToAgent(c *Connection, msgID string, data interface{}) {
	respPayload, _ := json.Marshal(data)
	msg, _ := json.Marshal(map[string]interface{}{
		"type":    "response",
		"id":      msgID,
		"payload": json.RawMessage(respPayload),
	})

	select {
	case c.SendCh <- msg:
	default:
		zap.L().Warn("Agent 发送队列已满，丢弃 response", zap.String("msg_id", msgID))
	}
}
