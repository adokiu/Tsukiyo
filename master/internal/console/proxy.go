package console

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"tsukiyo/master/internal/agent"
	"tsukiyo/master/internal/db"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ConsoleSession 控制台会话信息
type ConsoleSession struct {
	InstanceID string `json:"instance_id"`
	NodeID     string `json:"node_id"`
	Type       string `json:"type"`
	IncusName  string `json:"incus_name"`
}

// HandleWebSSH WebSSH 代理
func HandleWebSSH(agentMgr *agent.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 Token"})
			return
		}

		session, err := getConsoleSession(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的 Token 或已过期"})
			return
		}

		nodeID, _ := uuid.Parse(session.NodeID)
		if !agentMgr.IsNodeConnected(nodeID) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "节点离线"})
			return
		}

		// 升级 WebSocket
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			zap.L().Error("WebSocket 升级失败", zap.Error(err))
			return
		}
		defer ws.Close()

		// 通过 Agent 建立 SSH 连接隧道
		payload := map[string]interface{}{
			"action":       "console",
			"type":         "ssh",
			"instance_id":  session.IncusName,
		}

		// 向 Agent 发送请求，建立双向转发
		resp, err := agentMgr.SendRequest(nodeID, "console", payload, 5*time.Second)
		if err != nil {
			zap.L().Error("Agent 控制台请求失败", zap.Error(err))
			ws.Close()
			return
		}

		var consoleResp struct {
			Port int `json:"port"`
		}
		if err := json.Unmarshal(resp, &consoleResp); err != nil {
			zap.L().Error("解析控制台响应失败", zap.Error(err))
			ws.Close()
			return
		}

		// 这里应该建立与 Agent 的独立 TCP/WebSocket 隧道
		// 简化处理：直接返回连接信息，由前端直接与 Agent 通信
		ws.WriteJSON(gin.H{
			"status":  "connected",
			"node_id": session.NodeID,
			"port":    consoleResp.Port,
		})

		// 保持连接存活
		for {
			_, _, err := ws.ReadMessage()
			if err != nil {
				break
			}
		}
	}
}

// HandleWebVNC WebVNC 代理
func HandleWebVNC(agentMgr *agent.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 Token"})
			return
		}

		session, err := getConsoleSession(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的 Token 或已过期"})
			return
		}

		nodeID, _ := uuid.Parse(session.NodeID)
		if !agentMgr.IsNodeConnected(nodeID) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "节点离线"})
			return
		}

		payload := map[string]interface{}{
			"action":       "console",
			"type":         "vnc",
			"instance_id":  session.IncusName,
		}

		resp, err := agentMgr.SendRequest(nodeID, "console", payload, 5*time.Second)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Agent 控制台请求失败"})
			return
		}

		var consoleResp struct {
			Port    int    `json:"port"`
			Token   string `json:"token"`
		}
		if err := json.Unmarshal(resp, &consoleResp); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "解析控制台响应失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"type":    "vnc",
			"node_id": session.NodeID,
			"port":    consoleResp.Port,
			"token":   consoleResp.Token,
		})
	}
}

// getConsoleSession 从 Redis 获取控制台会话
func getConsoleSession(token string) (*ConsoleSession, error) {
	ctx := context.Background()
	key := "console:" + token
	data, err := db.RedisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var session ConsoleSession
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, err
	}

	return &session, nil
}

// GenerateConsoleToken 生成控制台 Token
func GenerateConsoleToken(session ConsoleSession) (string, error) {
	token := uuid.New().String()
	ctx := context.Background()
	key := "console:" + token
	data, _ := json.Marshal(session)
	if err := db.RedisClient.Set(ctx, key, data, 5*time.Minute).Err(); err != nil {
		return "", err
	}
	return token, nil
}
