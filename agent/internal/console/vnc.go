package console

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// VNCSession 表示一个 SPICE 通道（对应一条到 Incus operation data fd 的 WebSocket）
type VNCSession struct {
	id        string
	instance  string
	incusWS   *websocket.Conn
	writeMu   sync.Mutex // 串行化对 incusWS 的写入, gorilla/websocket 禁止并发写
	sendFunc  func(data []byte)
	closeOnce sync.Once
}

// sharedOperation 表示一个 instance 共享的 Incus VGA operation。
// SPICE 协议会为 main、display、cursor、inputs 等每个通道各发起一条连接，
// 这些通道必须复用同一个 operation 的 data secret（与 LXD-UI 实现一致），
// 否则每个通道各自创建 operation 会因 force 互相踢掉，导致画面纯黑。
type sharedOperation struct {
	opPath     string
	dataSecret string
	controlWS  *websocket.Conn
	refCount   int
}

// VNCHandler VNC 控制台处理器
type VNCHandler struct {
	sessions   sync.Map // sessionID -> *VNCSession
	socketPath string
	opMu       sync.Mutex
	ops        map[string]*sharedOperation // instance -> 共享 operation
}

// NewVNCHandler 创建 VNC 控制台处理器
func NewVNCHandler(socketPath string) *VNCHandler {
	return &VNCHandler{
		socketPath: socketPath,
		ops:        make(map[string]*sharedOperation),
	}
}

// newHTTPClient 创建经由 Incus Unix socket 通信的 HTTP 客户端
func (h *VNCHandler) newHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Dial: func(_, _ string) (net.Conn, error) {
				return net.Dial("unix", h.socketPath)
			},
		},
		Timeout: 30 * time.Second,
	}
}

// newDialer 创建经由 Incus Unix socket 拨号的 WebSocket Dialer
func (h *VNCHandler) newDialer() *websocket.Dialer {
	return &websocket.Dialer{
		NetDial: func(_, _ string) (net.Conn, error) {
			return net.Dial("unix", h.socketPath)
		},
		HandshakeTimeout: 10 * time.Second,
		Subprotocols:     []string{"binary"},
	}
}

// createOperation 调用 Incus console API 创建 VGA operation，返回 operation 路径与 secret。
// force: true 仅在首次为该 instance 创建 operation 时执行一次，用于清理上一次遗留的
// SPICE 连接；同一会话内的后续通道复用该 operation，不再调用本函数，因此不会互相踢掉。
func (h *VNCHandler) createOperation(instance string) (opPath, dataSecret, controlSecret string, err error) {
	body := map[string]interface{}{"type": "vga", "force": true}
	bodyBytes, _ := json.Marshal(body)

	url := "http://unix/1.0/instances/" + instance + "/console"
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", "", "", fmt.Errorf("创建 VNC 控制台请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.newHTTPClient().Do(req)
	if err != nil {
		return "", "", "", fmt.Errorf("请求 Incus VNC 控制台失败: %w", err)
	}
	respData, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var incusResp struct {
		Type       string          `json:"type"`
		Operation  string          `json:"operation"`
		StatusCode int             `json:"status_code"`
		Metadata   json.RawMessage `json:"metadata"`
		Error      string          `json:"error"`
	}
	if err := json.Unmarshal(respData, &incusResp); err != nil {
		return "", "", "", fmt.Errorf("解析 Incus 响应失败: %w, body: %s", err, string(respData))
	}
	if incusResp.StatusCode >= 400 {
		return "", "", "", fmt.Errorf("Incus 错误 %d: %s", incusResp.StatusCode, incusResp.Error)
	}
	if incusResp.Operation == "" {
		return "", "", "", fmt.Errorf("Incus 未返回 operation, body: %s", string(respData))
	}

	var opMeta struct {
		Metadata struct {
			Fds map[string]string `json:"fds"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(incusResp.Metadata, &opMeta); err != nil {
		return "", "", "", fmt.Errorf("解析 operation metadata 失败: %w", err)
	}

	dataSecret = opMeta.Metadata.Fds["0"]
	if dataSecret == "" {
		return "", "", "", fmt.Errorf("无法获取 VNC data secret, metadata: %s", string(incusResp.Metadata))
	}
	controlSecret = opMeta.Metadata.Fds["control"]
	opPath = incusResp.Operation

	return opPath, dataSecret, controlSecret, nil
}

// acquireOperation 获取或创建 instance 的共享 VGA operation，并对引用计数加一。
func (h *VNCHandler) acquireOperation(instance string) (string, string, error) {
	h.opMu.Lock()
	defer h.opMu.Unlock()

	if op, ok := h.ops[instance]; ok {
		op.refCount++
		return op.opPath, op.dataSecret, nil
	}

	opPath, dataSecret, controlSecret, err := h.createOperation(instance)
	if err != nil {
		return "", "", err
	}

	// 连接 control fd（每个 operation 仅一条，Incus 要求 control fd 被连接）
	var controlWS *websocket.Conn
	if controlSecret != "" {
		controlURL := "ws://unix" + opPath + "/websocket?secret=" + controlSecret
		controlWS, _, err = h.newDialer().Dial(controlURL, nil)
		if err != nil {
			zap.L().Warn("连接 Incus VNC control WebSocket 失败", zap.String("instance", instance), zap.Error(err))
		}
	}

	h.ops[instance] = &sharedOperation{
		opPath:     opPath,
		dataSecret: dataSecret,
		controlWS:  controlWS,
		refCount:   1,
	}
	return opPath, dataSecret, nil
}

// releaseOperation 释放一个通道对 operation 的引用，归零时关闭 control 并清理缓存。
func (h *VNCHandler) releaseOperation(instance string) {
	h.opMu.Lock()
	defer h.opMu.Unlock()

	op, ok := h.ops[instance]
	if !ok {
		return
	}
	op.refCount--
	if op.refCount > 0 {
		return
	}
	if op.controlWS != nil {
		op.controlWS.Close()
	}
	delete(h.ops, instance)
	zap.L().Debug("VGA operation 已释放", zap.String("instance", instance))
}

// StartVNC 为一个 SPICE 通道建立到共享 Incus operation 的 data 连接并双向转发。
// 同一 instance 的所有通道复用同一个 operation 的 data secret，各自 dial 一条连接，
// 全部代理到同一个 qemu.spice socket，由 QEMU 按 connection-id 关联为同一 SPICE 会话。
func (h *VNCHandler) StartVNC(sessionID, instance string, sendFunc func(data []byte)) error {
	opPath, dataSecret, err := h.acquireOperation(instance)
	if err != nil {
		return err
	}

	wsURL := "ws://unix" + opPath + "/websocket?secret=" + dataSecret
	wsConn, _, err := h.newDialer().Dial(wsURL, nil)
	if err != nil {
		h.releaseOperation(instance)
		return fmt.Errorf("连接 Incus VNC data WebSocket 失败: %w", err)
	}

	sess := &VNCSession{
		id:       sessionID,
		instance: instance,
		incusWS:  wsConn,
		sendFunc: sendFunc,
	}
	h.sessions.Store(sessionID, sess)

	// 转发 Incus WS -> Master（base64 编码）
	go func() {
		defer h.RemoveVNCSession(sessionID)
		for {
			_, data, err := wsConn.ReadMessage()
			if err != nil {
				if !isCloseErr(err) {
					zap.L().Debug("VNC Incus WS 读取结束", zap.String("session_id", sessionID), zap.Error(err))
				}
				return
			}
			encoded := base64.StdEncoding.EncodeToString(data)
			sess.sendFunc([]byte(encoded))
		}
	}()

	return nil
}

// WriteVNCInput 向 VNC 会话写入数据（base64 解码后写入 Incus WS）
func (h *VNCHandler) WriteVNCInput(sessionID string, data []byte) error {
	val, ok := h.sessions.Load(sessionID)
	if !ok {
		zap.L().Warn("WriteVNCInput 找不到 session", zap.String("session_id", sessionID))
		return nil
	}
	sess := val.(*VNCSession)

	// data 是 base64 编码的
	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return fmt.Errorf("base64 解码 VNC 输入失败: %w", err)
	}

	// console_vnc_input 在 Agent 端以独立 goroutine 并发处理, 多条输入会并发写入
	// 同一个 incusWS, 必须加锁串行化, 否则触发 gorilla/websocket 的并发写 panic。
	sess.writeMu.Lock()
	defer sess.writeMu.Unlock()
	return sess.incusWS.WriteMessage(websocket.BinaryMessage, decoded)
}

// RemoveVNCSession 关闭一个通道连接并释放对共享 operation 的引用
func (h *VNCHandler) RemoveVNCSession(sessionID string) {
	val, ok := h.sessions.LoadAndDelete(sessionID)
	if !ok {
		return
	}
	sess := val.(*VNCSession)
	sess.closeOnce.Do(func() {
		if sess.incusWS != nil {
			sess.incusWS.Close()
		}
		h.releaseOperation(sess.instance)
	})
}

// isCloseErr 判断是否为正常的连接关闭错误
func isCloseErr(err error) bool {
	return websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway)
}
