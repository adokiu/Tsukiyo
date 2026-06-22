package agent

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"tsukiyo/master/internal/console"
)

// handleVNCData 处理 Agent 发来的 VNC 数据，转发到前端 WS
func (m *Manager) handleVNCData(msgType string, payload json.RawMessage) {
	var msg struct {
		SessionID string `json:"session_id"`
		Data      string `json:"data"`
		Error     string `json:"error"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		zap.L().Warn("解析 VNC 数据失败", zap.Error(err))
		return
	}

	m.consoleMu.RLock()
	conn, ok := m.consoleSessions[msg.SessionID]
	m.consoleMu.RUnlock()
	if !ok || conn == nil {
		zap.L().Warn("VNC 数据找不到前端 session", zap.String("session_id", msg.SessionID))
		return
	}

	if msgType == "console_vnc_error" {
		wsMsg, _ := json.Marshal(map[string]interface{}{
			"type":    "error",
			"message": msg.Error,
		})
		conn.WriteMessage(websocket.TextMessage, wsMsg)
		return
	}

	// VNC 数据是 base64 编码的二进制数据，解码后以二进制 WS 消息发送给前端
	decoded, err := base64.StdEncoding.DecodeString(msg.Data)
	if err != nil {
		zap.L().Warn("VNC 数据 base64 解码失败", zap.String("session_id", msg.SessionID), zap.Error(err))
		return
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, decoded); err != nil {
		zap.L().Warn("写入前端 VNC WebSocket 失败", zap.String("session_id", msg.SessionID), zap.Error(err))
	}
}

// HandleVNCWebSocket 处理前端 VNC WebSocket 连接
func (m *Manager) HandleVNCWebSocket(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少 token"})
		return
	}

	// 验证 token（不消费删除）。
	// SPICE 协议会为 main、display、cursor、inputs 等每个通道各发起一条
	// WebSocket 连接到同一个 URI（同一个 token），因此 token 必须在其 TTL 内
	// 支持多次连接，否则除 main 外的通道会全部 401，导致画面纯黑。
	// 安全性由 5 分钟 TTL + 每次打开控制台生成新 token 保证。
	session, err := console.ValidateConsoleToken(token)
	if err != nil {
		zap.L().Warn("VNC token 验证失败", zap.String("token", token), zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token 无效或已过期"})
		return
	}

	// 升级为 WebSocket（支持 binary 子协议）
	conn, err := vncUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		zap.L().Error("VNC WebSocket 升级失败", zap.Error(err))
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
		zap.L().Warn("VNC 请求节点离线", zap.String("node_id", nodeID.String()))
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

	// 向 Agent 发送 console_vnc_start
	if err := agentConn.Send(struct {
		Type    string      `json:"type"`
		Payload interface{} `json:"payload"`
	}{
		Type: "console_vnc_start",
		Payload: map[string]interface{}{
			"session_id": sessionID,
			"container":  session.IncusName,
		},
	}); err != nil {
		zap.L().Error("发送 console_vnc_start 到 Agent 失败", zap.String("session_id", sessionID), zap.Error(err))
		conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","message":"发送 VNC 请求失败"}`))
		return
	}

	// 读取前端输入（SPICE 协议数据），base64 编码后转发到 Agent
	for {
		msgType, msgBytes, err := conn.ReadMessage()
		if err != nil {
			break
		}

		if msgType == websocket.BinaryMessage || msgType == websocket.TextMessage {
			encoded := base64.StdEncoding.EncodeToString(msgBytes)
			agentConn.Send(struct {
				Type    string      `json:"type"`
				Payload interface{} `json:"payload"`
			}{
				Type: "console_vnc_input",
				Payload: map[string]interface{}{
					"session_id": sessionID,
					"data":       encoded,
				},
			})
		}
	}

	// 前端断开，通知 Agent 关闭 VNC 会话
	agentConn.Send(struct {
		Type    string      `json:"type"`
		Payload interface{} `json:"payload"`
	}{
		Type: "console_vnc_close",
		Payload: map[string]interface{}{
			"session_id": sessionID,
		},
	})
}
