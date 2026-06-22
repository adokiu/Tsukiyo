package ws

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"tsukiyo/agent/internal/config"
	"tsukiyo/agent/internal/system"
)

// MessageType WebSocket 消息类型
type MessageType string

const (
	MsgTypeRegister         MessageType = "register"
	MsgTypeHeartbeat        MessageType = "heartbeat"
	MsgTypeInstanceStatus   MessageType = "instance_status"
	MsgTypeMetrics          MessageType = "metrics"
	MsgTypeTaskResult       MessageType = "task_result"
	MsgTypeResponse         MessageType = "response"
	MsgTypeImageProgress    MessageType = "image_progress"
	MsgTypeInstanceProgress MessageType = "instance_progress"
	MsgTypeSecurityAlert    MessageType = "security_alert"
	MsgTypeTaskLog          MessageType = "task_log"
)

// TaskHandler 任务处理回调
type TaskHandler func(taskID string, taskType string, payload json.RawMessage) (json.RawMessage, error)

// RequestHandler Master 同步请求处理回调
type RequestHandler func(reqType string, payload json.RawMessage) (json.RawMessage, error)

// ConfigHandler 配置下发回调
type ConfigHandler func(data map[string]interface{})

// ConsoleMessageHandler 控制台流式消息处理回调
type ConsoleMessageHandler func(msgType string, payload json.RawMessage)

// Client WebSocket 客户端
type Client struct {
	cfg            *config.Config
	conn           *websocket.Conn
	mu             sync.RWMutex
	connected      bool
	taskHandler    TaskHandler
	requestHandler RequestHandler
	configHandler  ConfigHandler
	consoleHandler ConsoleMessageHandler
	shutdown       chan struct{}
	reconnectCh    chan struct{}
	pendingReqs    map[string]chan []byte
	reqMu          sync.RWMutex
	hostname       string
	localAddress   string
}

// NewClient 创建 WebSocket 客户端
func NewClient(cfg *config.Config) *Client {
	hostname, _ := os.Hostname()
	return &Client{
		cfg:         cfg,
		shutdown:    make(chan struct{}),
		reconnectCh: make(chan struct{}, 1),
		pendingReqs: make(map[string]chan []byte),
		hostname:    hostname,
	}
}

// SetTaskHandler 设置任务处理器
func (c *Client) SetTaskHandler(h TaskHandler) {
	c.taskHandler = h
}

// SetRequestHandler 设置同步请求处理器
func (c *Client) SetRequestHandler(h RequestHandler) {
	c.requestHandler = h
}

// SetConfigHandler 设置配置下发处理器
func (c *Client) SetConfigHandler(h ConfigHandler) {
	c.configHandler = h
}

// SetConsoleHandler 设置控制台流式消息处理器
func (c *Client) SetConsoleHandler(h ConsoleMessageHandler) {
	c.consoleHandler = h
}

// SendConsoleMessage 发送控制台流式消息到 Master
func (c *Client) SendConsoleMessage(msgType string, payload interface{}) error {
	data, err := json.Marshal(map[string]interface{}{
		"type":    msgType,
		"payload": payload,
	})
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("WebSocket 未连接")
	}
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// Connect 连接到 Master
func (c *Client) Connect() error {
	headers := http.Header{}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	url := c.cfg.MasterWSURL()
	zap.L().Info("正在连接 Master", zap.String("url", url))

	conn, _, err := dialer.Dial(url, headers)
	if err != nil {
		return fmt.Errorf("连接 Master 失败: %w", err)
	}

	c.conn = conn
	c.connected = true

	// 发送注册消息
	if err := c.sendRegister(); err != nil {
		conn.Close()
		return fmt.Errorf("注册失败: %w", err)
	}

	zap.L().Info("WebSocket 连接成功")

	// 启动读写 goroutine
	go c.readLoop()
	go c.writeLoop()
	go c.heartbeatLoop()

	return nil
}

// sendRegister 发送注册消息（携带完整宿主机探测信息）
func (c *Client) sendRegister() error {
	// 获取 Incus 版本（如果 incus 命令可用）
	incusVersion := "unknown"
	if v, err := getIncusVersion(""); err == nil {
		incusVersion = v
	}

	// 探测宿主机信息
	hostInfo := system.Probe()

	// 获取公网出口 IP
	publicIPv4 := system.GetPublicIPv4()
	publicIPv6 := system.GetPublicIPv6()

	payload := map[string]interface{}{
		"token":         c.cfg.Token,
		"hostname":      c.hostname,
		"version":       "1.0.0",
		"incus_version": incusVersion,
		"total_cpu":     float64(hostInfo.CPU.Cores),
		"total_memory":  hostInfo.Memory.TotalMB,
		"total_disk":    getTotalDiskFromDisks(hostInfo.Disks),
		"public_ipv4":   publicIPv4,
		"public_ipv6":   publicIPv6,
		"system_info":   hostInfo,
	}

	return c.sendMessage(MsgTypeRegister, payload)
}

func getTotalDiskFromDisks(disks []system.DiskInfo) int64 {
	var total int64
	for _, d := range disks {
		total += int64(d.SizeBytes)
	}
	return total
}

// sendMessage 发送消息
func (c *Client) sendMessage(msgType MessageType, payload interface{}) error {
	data, err := json.Marshal(map[string]interface{}{
		"type":    string(msgType),
		"payload": payload,
	})
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("连接未建立")
	}
	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return err
	}
	return nil
}

// SendRequest 发送同步请求 (如 console 连接请求)
func (c *Client) SendRequest(reqType string, payload interface{}) ([]byte, error) {
	reqID := generateReqID()
	respCh := make(chan []byte, 1)

	c.reqMu.Lock()
	c.pendingReqs[reqID] = respCh
	c.reqMu.Unlock()

	defer func() {
		c.reqMu.Lock()
		delete(c.pendingReqs, reqID)
		c.reqMu.Unlock()
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

	data, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	if c.conn == nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("连接未建立")
	}
	err = c.conn.WriteMessage(websocket.TextMessage, data)
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}

	select {
	case resp := <-respCh:
		return resp, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("请求超时")
	}
}

// SendHeartbeat 发送心跳
func (c *Client) SendHeartbeat(cpuPercent float64, memUsed, memTotal, diskUsed, diskTotal, netIn, netOut, uptime int64, instances, running int, publicIPv4s, ipv6Prefixes []string, networkInterfaces json.RawMessage) error {
	payload := map[string]interface{}{
		"token":              c.cfg.Token,
		"cpu_percent":        cpuPercent,
		"mem_used":           memUsed,
		"mem_total":          memTotal,
		"disk_used":          diskUsed,
		"disk_total":         diskTotal,
		"net_in":             netIn,
		"net_out":            netOut,
		"uptime":             uptime,
		"instances":          instances,
		"running":            running,
		"timestamp":          time.Now().Unix(),
		"public_ipv4s":       publicIPv4s,
		"ipv6_prefixes":      ipv6Prefixes,
		"network_interfaces": networkInterfaces,
	}
	return c.sendMessage(MsgTypeHeartbeat, payload)
}

// SendInstanceStatus 发送实例状态
func (c *Client) SendInstanceStatus(statuses []InstanceStatusPayload) error {
	payload := map[string]interface{}{
		"token":     c.cfg.Token,
		"instances": statuses,
	}
	return c.sendMessage(MsgTypeInstanceStatus, payload)
}

// SendMetrics 发送监控指标
func (c *Client) SendMetrics(instances []InstanceMetricPayload) error {
	payload := map[string]interface{}{
		"token":     c.cfg.Token,
		"timestamp": time.Now().Unix(),
		"instances": instances,
	}
	return c.sendMessage(MsgTypeMetrics, payload)
}

// SendTaskResult 发送任务结果
func (c *Client) SendTaskResult(taskID string, success bool, result json.RawMessage, errMsg string) error {
	payload := map[string]interface{}{
		"token":   c.cfg.Token,
		"task_id": taskID,
		"success": success,
		"payload": result,
		"error":   errMsg,
	}
	return c.sendMessage(MsgTypeTaskResult, payload)
}

// ImageProgressPayload 镜像下载进度
type ImageProgressPayload struct {
	ImageID         string `json:"image_id"`
	Stage           string `json:"stage"`    // downloading / converting / done / error
	Progress        int    `json:"progress"` // 0-100
	DownloadedBytes int64  `json:"downloaded_bytes"`
	TotalBytes      int64  `json:"total_bytes"`
	SpeedBps        int64  `json:"speed_bps"` // 字节/秒
	Error           string `json:"error,omitempty"`
}

// SendImageProgress 发送镜像下载进度
func (c *Client) SendImageProgress(p ImageProgressPayload) error {
	payload := map[string]interface{}{
		"token":            c.cfg.Token,
		"image_id":         p.ImageID,
		"stage":            p.Stage,
		"progress":         p.Progress,
		"downloaded_bytes": p.DownloadedBytes,
		"total_bytes":      p.TotalBytes,
		"speed_bps":        p.SpeedBps,
		"error":            p.Error,
	}
	return c.sendMessage(MsgTypeImageProgress, payload)
}

// SendLocalImages 上报本地已有镜像完整信息列表
func (c *Client) SendLocalImages(images interface{}) error {
	payload := map[string]interface{}{
		"token":  c.cfg.Token,
		"images": images,
	}
	return c.sendMessage(MsgTypeImageProgress, payload)
}

// InstanceProgressPayload 实例创建进度
type InstanceProgressPayload struct {
	InstanceID string `json:"instance_id"`
	TaskID     string `json:"task_id"`
	Step       int    `json:"step"`     // 1=started 2=accepted 3=network 4=ssh 5=port_mapping 6=completed
	Progress   int    `json:"progress"` // 0-100
	Message    string `json:"message"`  // 中文描述
	Error      string `json:"error,omitempty"`
	Status     string `json:"status"` // running / success / error
}

// SendInstanceProgress 发送实例创建进度
func (c *Client) SendInstanceProgress(p InstanceProgressPayload) error {
	payload := map[string]interface{}{
		"token":       c.cfg.Token,
		"instance_id": p.InstanceID,
		"task_id":     p.TaskID,
		"step":        p.Step,
		"progress":    p.Progress,
		"message":     p.Message,
		"error":       p.Error,
		"status":      p.Status,
	}
	return c.sendMessage(MsgTypeInstanceProgress, payload)
}

// SendTaskLog 发送任务日志
func (c *Client) SendTaskLog(taskID string, level string, message string) error {
	payload := map[string]interface{}{
		"token":   c.cfg.Token,
		"task_id": taskID,
		"level":   level,
		"message": message,
	}
	return c.sendMessage(MsgTypeTaskLog, payload)
}

// SecurityAlertPayload 安全告警上报载荷
type SecurityAlertPayload struct {
	InstanceID string `json:"instance_id"`
	AlertType  string `json:"alert_type"`
	Severity   string `json:"severity"`
	SourceIP   string `json:"source_ip,omitempty"`
	DestPort   int    `json:"dest_port,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
	Details    string `json:"details"`
	RawData    string `json:"raw_data,omitempty"`
	DetectedAt int64  `json:"detected_at"`
}

// SendSecurityAlert 上报安全告警到 Master
func (c *Client) SendSecurityAlert(p SecurityAlertPayload) error {
	payload := map[string]interface{}{
		"token":       c.cfg.Token,
		"instance_id": p.InstanceID,
		"alert_type":  p.AlertType,
		"severity":    p.Severity,
		"source_ip":   p.SourceIP,
		"dest_port":   p.DestPort,
		"protocol":    p.Protocol,
		"details":     p.Details,
		"raw_data":    p.RawData,
		"detected_at": p.DetectedAt,
	}
	return c.sendMessage(MsgTypeSecurityAlert, payload)
}

// readLoop 读取消息循环
func (c *Client) readLoop() {
	defer func() {
		if r := recover(); r != nil {
			zap.L().Error("readLoop panic", zap.Any("recover", r))
		}
		c.triggerReconnect()
	}()

	for {
		select {
		case <-c.shutdown:
			return
		default:
		}

		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()
		if conn == nil {
			return
		}

		conn.SetReadDeadline(time.Now().Add(65 * time.Second))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(65 * time.Second))
			return nil
		})
		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				zap.L().Warn("WebSocket 读取异常", zap.Error(err))
			}
			return
		}

		var msg struct {
			Type    string          `json:"type"`
			ID      string          `json:"id,omitempty"`
			Payload json.RawMessage `json:"payload,omitempty"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			zap.L().Warn("解析消息失败", zap.Error(err))
			continue
		}

		// 处理响应消息
		if msg.Type == "response" && msg.ID != "" {
			c.reqMu.RLock()
			ch, exists := c.pendingReqs[msg.ID]
			c.reqMu.RUnlock()
			if exists {
				select {
				case ch <- msg.Payload:
				default:
				}
			}
			continue
		}

		// 处理认证失败
		if msg.Type == "auth_error" {
			var errMsg struct {
				Error string `json:"error"`
			}
			_ = json.Unmarshal(msg.Payload, &errMsg)
			zap.L().Fatal("Master 认证失败",
				zap.String("error", errMsg.Error),
				zap.String("提示", "请检查 config.yaml 中的 token 是否正确，或在 Master 前端重新创建节点获取新 token"))
			return
		}

		// 处理配置下发
		if msg.Type == "config" {
			var cfgData map[string]interface{}
			if err := json.Unmarshal(msg.Payload, &cfgData); err != nil {
				zap.L().Warn("解析配置消息失败", zap.Error(err))
				continue
			}
			c.cfg.UpdateFromMaster(cfgData)
			zap.L().Info("收到并应用 Master 下发的配置",
				zap.String("incus_socket", c.cfg.IncusSocketPath()),
				zap.String("network_interface", c.cfg.NetworkInterface()),
				zap.Bool("enable_nat", c.cfg.EnableNAT()))
			if c.configHandler != nil {
				go c.configHandler(cfgData)
			}
			continue
		}

		// 处理任务消息
		if msg.Type == "task" && c.taskHandler != nil {
			go c.handleTask(msg.Payload)
			continue
		}

		// 处理控制台流式消息
		if (msg.Type == "console_ssh_start" || msg.Type == "console_ssh_input" || msg.Type == "console_ssh_close" ||
			msg.Type == "console_vnc_start" || msg.Type == "console_vnc_input" || msg.Type == "console_vnc_close") && c.consoleHandler != nil {
			// console_vnc_start 和 console_ssh_start 同步处理, 确保 session 创建完成后再处理 input
			if msg.Type == "console_vnc_start" || msg.Type == "console_ssh_start" {
				c.consoleHandler(msg.Type, msg.Payload)
			} else {
				go c.consoleHandler(msg.Type, msg.Payload)
			}
			continue
		}

		// 处理 Master 同步请求消息（如 get_storages、get_instances 等）
		if msg.ID != "" && c.requestHandler != nil {
			go func(reqType string, reqID string, payload json.RawMessage) {
				respPayload, err := c.requestHandler(reqType, payload)
				resp := struct {
					Type    string          `json:"type"`
					ID      string          `json:"id"`
					Payload json.RawMessage `json:"payload,omitempty"`
					Error   string          `json:"error,omitempty"`
				}{
					Type: "response",
					ID:   reqID,
				}
				if err != nil {
					resp.Error = err.Error()
				} else {
					resp.Payload = respPayload
				}
				data, _ := json.Marshal(resp)
				c.mu.Lock()
				conn := c.conn
				if conn != nil {
					conn.WriteMessage(websocket.TextMessage, data)
				}
				c.mu.Unlock()
			}(msg.Type, msg.ID, msg.Payload)
		}
	}
}

// handleTask 处理任务 (带 panic 恢复)
func (c *Client) handleTask(payload json.RawMessage) {
	defer func() {
		if r := recover(); r != nil {
			zap.L().Error("任务处理 panic", zap.Any("recover", r))
		}
	}()

	var task struct {
		TaskID  string          `json:"task_id"`
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(payload, &task); err != nil {
		zap.L().Error("解析任务失败", zap.Error(err))
		return
	}

	zap.L().Info("收到任务",
		zap.String("task_id", task.TaskID),
		zap.String("type", task.Type))

	result, err := c.taskHandler(task.TaskID, task.Type, task.Payload)
	if err != nil {
		zap.L().Error("任务执行失败",
			zap.String("task_id", task.TaskID),
			zap.String("type", task.Type),
			zap.Error(err))
		if serr := c.SendTaskResult(task.TaskID, false, nil, err.Error()); serr != nil {
			zap.L().Error("发送任务失败结果失败", zap.Error(serr))
		}
		return
	}

	zap.L().Info("任务执行成功",
		zap.String("task_id", task.TaskID),
		zap.String("type", task.Type))
	if serr := c.SendTaskResult(task.TaskID, true, result, ""); serr != nil {
		zap.L().Error("发送任务成功结果失败", zap.Error(serr))
	}
}

// writeLoop 写入消息循环 (处理 ping)
func (c *Client) writeLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.shutdown:
			return
		case <-ticker.C:
			c.mu.Lock()
			if c.conn != nil {
				c.conn.WriteMessage(websocket.PingMessage, nil)
			}
			c.mu.Unlock()
		}
	}
}

// heartbeatLoop 心跳循环 (由外部调用更精确的数据)
func (c *Client) heartbeatLoop() {
	// 心跳由 monitor 模块主动调用 SendHeartbeat
	// 这里只负责 reconnect 逻辑
	for {
		select {
		case <-c.shutdown:
			return
		case <-c.reconnectCh:
			c.reconnect()
		}
	}
}

// triggerReconnect 触发重连
func (c *Client) triggerReconnect() {
	c.mu.Lock()
	c.connected = false
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.mu.Unlock()

	select {
	case c.reconnectCh <- struct{}{}:
	default:
	}
}

// reconnect 重连逻辑 (指数退避)
func (c *Client) reconnect() {
	zap.L().Info("开始重连 Master")
	for i := 0; i < 120; i++ {
		select {
		case <-c.shutdown:
			return
		default:
		}

		if err := c.Connect(); err == nil {
			zap.L().Info("重连成功")
			return
		} else {
			zap.L().Warn("重连失败", zap.Int("attempt", i+1), zap.Error(err))
		}

		// 指数退避: 1s, 2s, 4s, 8s, 16s, 30s, 30s...
		backoff := time.Duration(1<<uint(i)) * time.Second
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
		time.Sleep(backoff)
	}
	zap.L().Fatal("重连失败次数过多，退出 Agent")
}

// Shutdown 关闭连接
func (c *Client) Shutdown() {
	close(c.shutdown)
	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.mu.Unlock()
}

// IsConnected 检查连接状态
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// generateReqID 生成请求 ID
func generateReqID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

// InstanceStatusPayload 实例状态上报
type InstanceStatusPayload struct {
	InstanceID string  `json:"instance_id"`
	Status     string  `json:"status"`
	IPv4       string  `json:"ipv4,omitempty"`
	IPv6       string  `json:"ipv6,omitempty"`
	CPUPercent float64 `json:"cpu_percent,omitempty"`
	MemUsed    int64   `json:"mem_used,omitempty"`
	NetIn      int64   `json:"net_in,omitempty"`
	NetOut     int64   `json:"net_out,omitempty"`
}

// InstanceMetricPayload 实例监控指标上报
type InstanceMetricPayload struct {
	InstanceID    string  `json:"instance_id"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemUsed       int64   `json:"mem_used"`
	MemTotal      int64   `json:"mem_total"`
	DiskUsed      int64   `json:"disk_used"`
	DiskTotal     int64   `json:"disk_total"`
	DiskReadBps   int64   `json:"disk_read_bps"`
	DiskWriteBps  int64   `json:"disk_write_bps"`
	DiskReadIops  int64   `json:"disk_read_iops"`
	DiskWriteIops int64   `json:"disk_write_iops"`
	NetIn         int64   `json:"net_in"`
	NetOut        int64   `json:"net_out"`
	NetInTotal    int64   `json:"net_in_total"`
	NetOutTotal   int64   `json:"net_out_total"`
}
