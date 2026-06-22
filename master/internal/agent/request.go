package agent

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// agentResponse Agent 响应
type agentResponse struct {
	payload []byte
	errMsg  string
}

// SendRequest 向指定 Agent 发送同步请求并等待响应
func (m *Manager) SendRequest(nodeID uuid.UUID, reqType string, payload interface{}, timeout time.Duration) ([]byte, error) {
	m.mu.RLock()
	conn, exists := m.connections[nodeID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("节点 %s 未连接", nodeID)
	}

	reqID := uuid.New().String()
	respCh := make(chan agentResponse, 1)

	m.reqMu.Lock()
	m.pendingRequests[reqID] = respCh
	m.reqMu.Unlock()

	defer func() {
		m.reqMu.Lock()
		delete(m.pendingRequests, reqID)
		m.reqMu.Unlock()
	}()

	msg := struct {
		Type    string      `json:"type"`
		ID      string      `json:"id"`
		Payload interface{} `json:"payload,omitempty"`
	}{
		Type:    reqType,
		ID:      reqID,
		Payload: payload,
	}

	if err := conn.Send(msg); err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}

	select {
	case resp := <-respCh:
		if resp.errMsg != "" {
			return nil, fmt.Errorf("%s", resp.errMsg)
		}
		return resp.payload, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("请求超时 (%s)", timeout)
	case <-conn.ctx.Done():
		return nil, fmt.Errorf("连接已关闭")
	}
}

// handleResponse 处理 Agent 响应
func (m *Manager) handleResponse(msgID string, payload json.RawMessage, errMsg string) {
	m.reqMu.RLock()
	ch, exists := m.pendingRequests[msgID]
	m.reqMu.RUnlock()

	if exists {
		select {
		case ch <- agentResponse{payload: []byte(payload), errMsg: errMsg}:
		default:
		}
	}
}
