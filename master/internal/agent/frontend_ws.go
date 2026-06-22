package agent

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// HandleFrontendWebSocket 处理前端 WebSocket 连接
func (m *Manager) HandleFrontendWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		zap.L().Error("前端 WebSocket 升级失败", zap.Error(err))
		return
	}

	fc := &FrontendConn{
		Conn:   conn,
		SendCh: make(chan []byte, 64),
	}

	m.frontendMu.Lock()
	m.frontendConns = append(m.frontendConns, fc)
	m.frontendMu.Unlock()

	// 连接断开时从列表中移除（sync.Once 确保只执行一次，防止双重 close 导致 panic）
	var removeOnce sync.Once
	removeConn := func() {
		removeOnce.Do(func() {
			m.frontendMu.Lock()
			defer m.frontendMu.Unlock()
			for i, c := range m.frontendConns {
				if c == fc {
					m.frontendConns = append(m.frontendConns[:i], m.frontendConns[i+1:]...)
					break
				}
			}
			close(fc.SendCh)
		})
	}

	// 启动写入 pump
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		defer removeConn()
		for {
			select {
			case msg, ok := <-fc.SendCh:
				if !ok {
					conn.WriteMessage(websocket.CloseMessage, []byte{})
					return
				}
				fc.mu.Lock()
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				err := conn.WriteMessage(websocket.TextMessage, msg)
				fc.mu.Unlock()
				if err != nil {
					return
				}
			case <-ticker.C:
				fc.mu.Lock()
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				err := conn.WriteMessage(websocket.PingMessage, nil)
				fc.mu.Unlock()
				if err != nil {
					return
				}
			}
		}
	}()

	// 读取循环（保持连接存活）
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}

	conn.Close()
	removeConn()
}

// BroadcastToFrontend 向所有前端 WebSocket 连接广播消息
func (m *Manager) BroadcastToFrontend(data interface{}) {
	payload, err := json.Marshal(data)
	if err != nil {
		return
	}
	m.frontendMu.RLock()
	defer m.frontendMu.RUnlock()
	for _, fc := range m.frontendConns {
		if fc != nil && fc.Conn != nil {
			fc.Conn.WriteMessage(websocket.TextMessage, payload)
		}
	}
}

// BroadcastInstanceMetrics 向前端广播实例实时监控指标
func (m *Manager) BroadcastInstanceMetrics(instanceID uuid.UUID, data map[string]interface{}) {
	msg, err := json.Marshal(map[string]interface{}{
		"type":        "instance_metrics",
		"instance_id": instanceID.String(),
		"data":        data,
	})
	if err != nil {
		return
	}

	m.frontendMu.Lock()
	defer m.frontendMu.Unlock()
	alive := make([]*FrontendConn, 0, len(m.frontendConns))
	for _, fc := range m.frontendConns {
		select {
		case fc.SendCh <- msg:
			alive = append(alive, fc)
		default:
			zap.L().Warn("发送缓冲区满，丢弃连接")
		}
	}
	m.frontendConns = alive
}
