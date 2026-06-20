package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/geoip"
	"tsukiyo/master/internal/models"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  8192,
	WriteBufferSize: 8192,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// ImageProgressPayload 镜像下载进度上报
type ImageProgressPayload struct {
	Token           string `json:"token"`
	ImageID         string `json:"image_id"`
	Stage           string `json:"stage"`    // downloading / done / error / canceled
	Progress        int    `json:"progress"` // 0-100
	DownloadedBytes int64  `json:"downloaded_bytes"`
	TotalBytes      int64  `json:"total_bytes"`
	SpeedBps        int64  `json:"speed_bps"`
	Error           string `json:"error,omitempty"`
}

// NodeImageStatus 单个节点上某个镜像的下载状态
type NodeImageStatus struct {
	ImageID         string    `json:"image_id"`
	Stage           string    `json:"stage"`
	Progress        int       `json:"progress"`
	DownloadedBytes int64     `json:"downloaded_bytes"`
	TotalBytes      int64     `json:"total_bytes"`
	SpeedBps        int64     `json:"speed_bps"`
	Error           string    `json:"error,omitempty"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// FrontendConn 前端 WebSocket 连接
type FrontendConn struct {
	Conn   *websocket.Conn
	SendCh chan []byte
	mu     sync.Mutex
}

// Manager Agent 连接管理器
type Manager struct {
	connections     map[uuid.UUID]*Connection
	mu              sync.RWMutex
	taskCh          chan *TaskMessage
	nodeStatusCh    chan NodeStatusUpdate
	pendingRequests map[string]chan agentResponse
	reqMu           sync.RWMutex
	OnTaskResult    func(taskID uuid.UUID, result json.RawMessage, errMsg string)
	imageProgress   map[uuid.UUID]map[string]*NodeImageStatus // nodeID -> imageID -> status
	imageMu         sync.RWMutex
	frontendConns   []*FrontendConn
	frontendMu      sync.RWMutex
}

// Connection Agent 连接
type Connection struct {
	NodeID   uuid.UUID
	Conn     *websocket.Conn
	SendCh   chan []byte
	LastPing time.Time
	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
}

// TaskMessage 下发给 Agent 的任务消息
type TaskMessage struct {
	NodeID  uuid.UUID       `json:"node_id"`
	TaskID  uuid.UUID       `json:"task_id"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// NodeStatusUpdate 节点状态更新
type NodeStatusUpdate struct {
	NodeID     uuid.UUID `json:"node_id"`
	Status     string    `json:"status"`
	CPUPercent float64   `json:"cpu_percent"`
	MemUsed    int64     `json:"mem_used"`
	MemTotal   int64     `json:"mem_total"`
	DiskUsed   int64     `json:"disk_used"`
	DiskTotal  int64     `json:"disk_total"`
	NetIn      int64     `json:"net_in"`
	NetOut     int64     `json:"net_out"`
	Instances  int       `json:"instances"`
	Running    int       `json:"running"`
	Timestamp  int64     `json:"timestamp"`
}

// RegisterPayload Agent 注册消息
type RegisterPayload struct {
	Token        string          `json:"token"`
	Hostname     string          `json:"hostname"`
	Version      string          `json:"version"`
	IncusVersion string          `json:"incus_version"`
	TotalCPU     float64         `json:"total_cpu"`
	TotalMemory  int64           `json:"total_memory"`
	TotalDisk    int64           `json:"total_disk"`
	PublicIPv4   string          `json:"public_ipv4"`
	PublicIPv6   string          `json:"public_ipv6"`
	SystemInfo   json.RawMessage `json:"system_info"`
}

// HeartbeatPayload Agent 心跳消息
type HeartbeatPayload struct {
	Token             string          `json:"token"`
	CPUPercent        float64         `json:"cpu_percent"`
	MemUsed           int64           `json:"mem_used"`
	MemTotal          int64           `json:"mem_total"`
	DiskUsed          int64           `json:"disk_used"`
	DiskTotal         int64           `json:"disk_total"`
	NetIn             int64           `json:"net_in"`
	NetOut            int64           `json:"net_out"`
	Uptime            int64           `json:"uptime"`
	Instances         int             `json:"instances"`
	Running           int             `json:"running"`
	Timestamp         int64           `json:"timestamp"`
	PublicIPv4s       []string        `json:"public_ipv4s"`
	IPv6Prefixes      []string        `json:"ipv6_prefixes"`
	NetworkInterfaces json.RawMessage `json:"network_interfaces"`
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

// InstanceProgressPayload 实例创建进度上报
type InstanceProgressPayload struct {
	Token      string `json:"token"`
	InstanceID string `json:"instance_id"`
	TaskID     string `json:"task_id"`
	Step       int    `json:"step"`     // 1=started 2=accepted 3=network 4=ssh 5=port_mapping 6=completed
	Progress   int    `json:"progress"` // 0-100
	Message    string `json:"message"`  // 中文描述
	Error      string `json:"error,omitempty"`
	Status     string `json:"status"` // running / success / error
}

// MetricsPayload 监控指标上报
type MetricsPayload struct {
	Token     string           `json:"token"`
	Timestamp int64            `json:"timestamp"`
	Instances []InstanceMetric `json:"instances"`
}

// InstanceMetric 单个实例监控指标
type InstanceMetric struct {
	InstanceID string  `json:"instance_id"`
	CPUPercent float64 `json:"cpu_percent"`
	MemUsed    int64   `json:"mem_used"`
	MemTotal   int64   `json:"mem_total"`
	DiskRead   int64   `json:"disk_read"`
	DiskWrite  int64   `json:"disk_write"`
	NetIn      int64   `json:"net_in"`
	NetOut     int64   `json:"net_out"`
}

// NewManager 创建 Agent 管理器
func NewManager() *Manager {
	return &Manager{
		connections:     make(map[uuid.UUID]*Connection),
		taskCh:          make(chan *TaskMessage, 1000),
		nodeStatusCh:    make(chan NodeStatusUpdate, 1000),
		pendingRequests: make(map[string]chan agentResponse),
		imageProgress:   make(map[uuid.UUID]map[string]*NodeImageStatus),
		frontendConns:   make([]*FrontendConn, 0),
	}
}

// BroadcastImageProgress 向前端广播镜像下载进度
func (m *Manager) BroadcastImageProgress(nodeID uuid.UUID, payload ImageProgressPayload) {
	data, err := json.Marshal(map[string]interface{}{
		"type":    "image_progress",
		"node_id": nodeID.String(),
		"payload": payload,
	})
	if err != nil {
		return
	}

	m.frontendMu.Lock()
	defer m.frontendMu.Unlock()
	alive := make([]*FrontendConn, 0, len(m.frontendConns))
	for _, fc := range m.frontendConns {
		select {
		case fc.SendCh <- data:
			alive = append(alive, fc)
		default:
			// 发送缓冲区满，丢弃该连接
			zap.L().Warn("发送缓冲区满，丢弃连接")
		}
	}
	m.frontendConns = alive
}

// BroadcastInstanceProgress 向前端广播实例创建进度
func (m *Manager) BroadcastInstanceProgress(nodeID uuid.UUID, payload InstanceProgressPayload) {
	data, err := json.Marshal(map[string]interface{}{
		"type":    "instance_progress",
		"node_id": nodeID.String(),
		"payload": payload,
	})
	if err != nil {
		return
	}

	m.frontendMu.Lock()
	defer m.frontendMu.Unlock()
	zap.L().Info("广播实例进度", zap.String("node_id", nodeID.String()), zap.String("instance_id", payload.InstanceID), zap.Int("前端连接数", len(m.frontendConns)))
	alive := make([]*FrontendConn, 0, len(m.frontendConns))
	for _, fc := range m.frontendConns {
		select {
		case fc.SendCh <- data:
			alive = append(alive, fc)
		default:
			zap.L().Warn("发送缓冲区满，丢弃连接")
		}
	}
	m.frontendConns = alive
}

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

// handleSecurityAlert 处理 Agent 上报的安全告警
func (m *Manager) handleSecurityAlert(nodeID uuid.UUID, payload json.RawMessage) {
	var alert struct {
		Token      string `json:"token"`
		InstanceID string `json:"instance_id"`
		AlertType  string `json:"alert_type"`
		Severity   string `json:"severity"`
		SourceIP   string `json:"source_ip"`
		DestPort   int    `json:"dest_port"`
		Protocol   string `json:"protocol"`
		Details    string `json:"details"`
		RawData    string `json:"raw_data"`
		DetectedAt int64  `json:"detected_at"`
	}
	if err := json.Unmarshal(payload, &alert); err != nil {
		zap.L().Error("解析安全告警失败", zap.Error(err))
		return
	}

	detectedAt := time.Unix(alert.DetectedAt, 0)
	if alert.DetectedAt == 0 {
		detectedAt = time.Now()
	}

	dbAlert := models.SecurityAlert{
		ID:         uuid.New(),
		NodeID:     nodeID,
		InstanceID: alert.InstanceID,
		AlertType:  alert.AlertType,
		Severity:   models.AlertSeverity(alert.Severity),
		Status:     models.AlertStatusOpen,
		SourceIP:   alert.SourceIP,
		DestPort:   alert.DestPort,
		Protocol:   alert.Protocol,
		Details:    alert.Details,
		RawData:    alert.RawData,
		DetectedAt: detectedAt,
	}

	if err := db.DB.Create(&dbAlert).Error; err != nil {
		zap.L().Error("持久化安全告警失败",
			zap.String("node_id", nodeID.String()),
			zap.String("alert_type", alert.AlertType),
			zap.Error(err))
		return
	}

	zap.L().Warn("收到安全告警",
		zap.String("node_id", nodeID.String()),
		zap.String("alert_type", alert.AlertType),
		zap.String("severity", alert.Severity),
		zap.String("source_ip", alert.SourceIP),
		zap.String("details", alert.Details))

	if alert.AlertType == "mining" || alert.AlertType == "smtp_abuse" {
		dbAlert.AutoAction = "auto_stop_instance"
		db.DB.Model(&dbAlert).Update("auto_action", dbAlert.AutoAction)

		if alert.InstanceID != "" {
			var instance models.Instance
			if err := db.DB.Where("incus_name = ? AND node_id = ?", alert.InstanceID, nodeID).First(&instance).Error; err == nil {
				zap.L().Warn("自动处置：因安全告警暂停实例",
					zap.String("instance_id", instance.ID.String()),
					zap.String("alert_type", alert.AlertType))

				payloadBytes, _ := json.Marshal(map[string]interface{}{
					"instance_id": instance.IncusName,
					"force":       true,
				})
				stopTask := models.Task{
					ID:         uuid.New(),
					Type:       models.TaskTypeStopInstance,
					NodeID:     nodeID,
					InstanceID: &instance.ID,
					UserID:     0,
					Status:     models.TaskStatusPending,
					Payload:    payloadBytes,
				}
				db.DB.Create(&stopTask)
			}
		}
	}

	m.BroadcastToFrontend(map[string]interface{}{
		"type": "security_alert",
		"data": dbAlert,
	})
}

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

// HandleWebSocket 处理 Agent WebSocket 连接
func (m *Manager) HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		zap.L().Error("WebSocket 升级失败", zap.Error(err))
		return
	}
	defer conn.Close()

	// 等待注册消息 (30秒超时，system_info 数据量大)
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	_, msgBytes, err := conn.ReadMessage()
	if err != nil {
		zap.L().Error("读取注册消息失败", zap.Error(err))
		return
	}
	conn.SetReadDeadline(time.Time{})

	var regMsg struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(msgBytes, &regMsg); err != nil {
		zap.L().Error("解析注册消息失败", zap.Error(err))
		return
	}

	if regMsg.Type != "register" {
		zap.L().Warn("收到非注册消息", zap.String("type", regMsg.Type))
		return
	}

	var payload RegisterPayload
	if err := json.Unmarshal(regMsg.Payload, &payload); err != nil {
		zap.L().Error("解析注册 payload 失败", zap.Error(err))
		return
	}

	// 通过 Token 查找节点
	var node models.Node
	if err := db.DB.Where("token = ?", payload.Token).First(&node).Error; err != nil {
		zap.L().Error("节点认证失败", zap.String("token", payload.Token), zap.Error(err))
		// 发送认证错误消息给 Agent，避免 Agent 无限重连
		_ = conn.WriteJSON(map[string]interface{}{
			"type":  "auth_error",
			"error": "invalid token",
		})
		return
	}
	nodeID := node.ID

	ctx, cancel := context.WithCancel(context.Background())
	ac := &Connection{
		NodeID:   nodeID,
		Conn:     conn,
		SendCh:   make(chan []byte, 256),
		LastPing: time.Now(),
		ctx:      ctx,
		cancel:   cancel,
	}

	m.mu.Lock()
	if oldConn, exists := m.connections[nodeID]; exists {
		oldConn.Close()
	}
	m.connections[nodeID] = ac
	m.mu.Unlock()

	// 更新节点状态及上报的宿主机信息
	// IP 地址: 优先使用 agent 上报的公网 IP，fallback 用 WebSocket 连接出口 IP（参考 komari）
	clientIP := c.ClientIP()
	updates := map[string]interface{}{
		"status":         models.NodeStatusOnline,
		"hostname":       payload.Hostname,
		"incus_version":  payload.IncusVersion,
		"total_cpu":      payload.TotalCPU,
		"total_memory":   payload.TotalMemory,
		"total_disk":     payload.TotalDisk,
		"last_seen_at":   time.Now(),
		"last_heartbeat": time.Now(),
	}
	if payload.PublicIPv4 != "" {
		updates["ip_address"] = payload.PublicIPv4
	} else if clientIP != "" {
		ip := net.ParseIP(clientIP)
		if ip != nil && ip.To4() != nil {
			updates["ip_address"] = clientIP
		}
	}
	if payload.PublicIPv6 != "" {
		updates["ipv6_address"] = payload.PublicIPv6
	} else if clientIP != "" {
		ip := net.ParseIP(clientIP)
		if ip != nil && ip.To4() == nil {
			updates["ipv6_address"] = clientIP
		}
	}
	// 通过 IP 查询国家码（异步，不阻塞注册流程）
	lookupIP := payload.PublicIPv4
	if lookupIP == "" {
		lookupIP = clientIP
	}
	if lookupIP != "" {
		go func(ip, nodeID string) {
			code := geoip.LookupCountryCode(ip)
			if code != "" {
				db.DB.Model(&models.Node{}).Where("id = ?", nodeID).Update("country_code", code)
			}
		}(lookupIP, nodeID.String())
	}
	if len(payload.SystemInfo) > 0 {
		updates["system_info"] = string(payload.SystemInfo)
	}
	db.DB.Model(&node).Updates(updates)

	// Agent 连接后直接下发已有配置
	go func() {
		// 等待连接稳定后下发
		time.Sleep(1 * time.Second)
		cfg := map[string]interface{}{
			"incus_socket_path":    node.IncusSocketPath,
			"metrics_interval":     node.MetricsInterval,
			"heartbeat_interval":   node.HeartbeatInterval,
			"network_interface":    node.NetworkInterface,
			"enable_nat":           node.EnableNAT,
			"enable_firewall":      node.EnableFirewall,
			"enable_security_scan": node.EnableSecurityScan,
			"scan_interval":        node.ScanInterval,
			"console_bind_addr":    node.ConsoleBindAddr,
			"agent_url":            node.AgentURL,
			"image_remote_url":     node.ImageRemoteURL,
			"storage_pool_type":    node.StoragePoolType,
			"storage_pool_source":  node.StoragePoolSource,
		}

		// 查询该节点所有网桥配置并下发
		var bridges []models.Bridge
		if err := db.DB.Where("node_id = ?", nodeID).Find(&bridges).Error; err == nil && len(bridges) > 0 {
			bridgeConfigs := make([]map[string]interface{}, 0, len(bridges))
			for _, b := range bridges {
				var dnsServers []string
				json.Unmarshal(b.DNSServers, &dnsServers)

				bridgeConfigs = append(bridgeConfigs, map[string]interface{}{
					"id":               b.ID.String(),
					"name":             b.Name,
					"bridge_name":      b.BridgeName,
					"ipv4_enabled":     b.IPv4Enabled,
					"ipv4_cidr":        b.IPv4CIDR,
					"ipv4_gateway":     b.IPv4Gateway,
					"ipv6_enabled":     b.IPv6Enabled,
					"ipv6_cidr":        b.IPv6CIDR,
					"ipv6_gateway":     b.IPv6Gateway,
					"dns_servers":      dnsServers,
					"port_range_start": b.PortRangeStart,
					"port_range_end":   b.PortRangeEnd,
					"status":           string(b.Status),
					"nat_egress_ipv4":  getEIPAllocCIDR(b.NATEgressIPv4ID),
					"nat_egress_ipv6":  getEIPAllocCIDR(b.NATEgressIPv6ID),
				})
			}
			cfg["bridges"] = bridgeConfigs
			zap.L().Info("下发网桥配置到 Agent", zap.String("node_id", nodeID.String()), zap.Int("count", len(bridgeConfigs)))
		}

		// 查询该节点所有实例的端口映射并下发（Agent 重启后恢复 proxy 设备）
		var portMappings []models.PortMapping
		if err := db.DB.Where("node_id = ?", nodeID).Find(&portMappings).Error; err == nil && len(portMappings) > 0 {
			// 批量加载出口 EIP 分配
			allocIDs := make([]uuid.UUID, 0, len(portMappings))
			for _, pm := range portMappings {
				allocIDs = append(allocIDs, pm.EgressAllocationID)
			}
			var allocs []models.EIPAllocation
			db.DB.Where("id IN ?", allocIDs).Find(&allocs)
			allocMap := make(map[uuid.UUID]string, len(allocs))
			for _, a := range allocs {
				allocMap[a.ID] = a.GetIP()
			}

			pmConfigs := make([]map[string]interface{}, 0, len(portMappings))
			for _, pm := range portMappings {
				var inst models.Instance
				incusName := ""
				internalIP := ""
				if db.DB.Where("id = ?", pm.InstanceID).First(&inst).Error == nil {
					incusName = inst.IncusName
					if pm.IPVersion == "ipv6" {
						internalIP = inst.InternalIPv6
					} else {
						internalIP = inst.InternalIPv4
					}
				}
				pmConfigs = append(pmConfigs, map[string]interface{}{
					"id":             pm.ID.String(),
					"instance_id":    pm.InstanceID.String(),
					"incus_name":     incusName,
					"internal_ip":    internalIP,
					"host_port":      pm.HostPort,
					"container_port": pm.ContainerPort,
					"protocol":       pm.Protocol,
					"ip_version":     pm.IPVersion,
					"host_ip":        allocMap[pm.EgressAllocationID],
					"description":    pm.Description,
				})
			}
			cfg["port_mappings"] = pmConfigs
			zap.L().Info("下发端口映射到 Agent", zap.String("node_id", nodeID.String()), zap.Int("count", len(pmConfigs)))
		}

		if err := m.SendConfig(nodeID, cfg); err != nil {
			zap.L().Warn("下发已有配置失败", zap.String("node_id", nodeID.String()), zap.Error(err))
		}
	}()

	// 写入 Redis 缓存
	nodeKey := fmt.Sprintf("agent:%s", nodeID)
	db.RedisClient.Set(ctx, nodeKey, "online", 60*time.Second)

	zap.L().Info("Agent 连接成功",
		zap.String("node_id", nodeID.String()),
		zap.String("hostname", payload.Hostname),
	)

	// 启动读写 goroutine
	go ac.writePump()
	ac.readPump(m)
}

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

// SendConfig 向指定节点下发配置
func (m *Manager) SendConfig(nodeID uuid.UUID, cfg map[string]interface{}) error {
	m.mu.RLock()
	conn, exists := m.connections[nodeID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("节点 %s 未连接", nodeID)
	}

	msg := struct {
		Type    string                 `json:"type"`
		Payload map[string]interface{} `json:"payload"`
	}{
		Type:    "config",
		Payload: cfg,
	}

	return conn.Send(msg)
}

// removeConnection 移除连接
func (m *Manager) removeConnection(nodeID uuid.UUID) {
	m.mu.Lock()
	delete(m.connections, nodeID)
	m.mu.Unlock()

	// 更新节点状态为离线
	db.DB.Model(&models.Node{}).Where("id = ?", nodeID).Updates(map[string]interface{}{
		"status":         models.NodeStatusOffline,
		"last_heartbeat": time.Now(),
	})

	ctx := context.Background()
	nodeKey := fmt.Sprintf("agent:%s", nodeID)
	db.RedisClient.Del(ctx, nodeKey)

	zap.L().Info("Agent 断开连接", zap.String("node_id", nodeID.String()))

	// 广播节点离线到前端
	m.broadcastNodeOffline(nodeID)
}

// handleHeartbeat 处理心跳
func (m *Manager) handleHeartbeat(nodeID uuid.UUID, payload json.RawMessage) {
	var hb HeartbeatPayload
	if err := json.Unmarshal(payload, &hb); err != nil {
		zap.L().Warn("解析心跳失败", zap.String("node_id", nodeID.String()), zap.Error(err))
		return
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":         models.NodeStatusOnline,
		"used_cpu":       hb.CPUPercent,
		"used_memory":    hb.MemUsed,
		"used_disk":      hb.DiskUsed,
		"net_in":         hb.NetIn,
		"net_out":        hb.NetOut,
		"uptime":         hb.Uptime,
		"instance_count": hb.Instances,
		"running_count":  hb.Running,
		"last_heartbeat": now,
	}

	db.DB.Model(&models.Node{}).Where("id = ?", nodeID).Updates(updates)

	ctx := context.Background()
	nodeKey := fmt.Sprintf("agent:%s", nodeID)
	db.RedisClient.Set(ctx, nodeKey, "online", 60*time.Second)

	// 缓存节点资源
	resourceKey := fmt.Sprintf("node:%s:resources", nodeID)
	resourceData, _ := json.Marshal(map[string]interface{}{
		"cpu_percent": hb.CPUPercent,
		"mem_used":    hb.MemUsed,
		"mem_total":   hb.MemTotal,
		"disk_used":   hb.DiskUsed,
		"disk_total":  hb.DiskTotal,
		"net_in":      hb.NetIn,
		"net_out":     hb.NetOut,
		"uptime":      hb.Uptime,
		"instances":   hb.Instances,
		"running":     hb.Running,
		"timestamp":   now.Unix(),
	})
	db.RedisClient.Set(ctx, resourceKey, resourceData, 15*time.Second)

	// 更新 system_info 中的网卡信息，并检测 host EIP 池失效
	if len(hb.NetworkInterfaces) > 0 {
		db.DB.Model(&models.Node{}).Where("id = ?", nodeID).UpdateColumn("system_info", gorm.Expr("jsonb_set(COALESCE(system_info, '{}'::jsonb), '{network_interfaces}', ?::jsonb)", string(hb.NetworkInterfaces)))
		m.checkHostEIPPoolExpired(nodeID, hb.NetworkInterfaces)
	}

	// 广播心跳数据到前端 WebSocket
	m.broadcastNodeHeartbeat(nodeID, hb, now)
}

// checkHostEIPPoolExpired 检测 host 类型 EIP 池的 IP 是否已不在网卡上，不在则标记为 inactive
func (m *Manager) checkHostEIPPoolExpired(nodeID uuid.UUID, networkInterfaces json.RawMessage) {
	type ipProbe struct {
		Address string `json:"address"`
	}
	type netInfo struct {
		Name string    `json:"name"`
		IPv4 []ipProbe `json:"ipv4"`
		IPv6 []ipProbe `json:"ipv6"`
	}
	var nics []netInfo
	if err := json.Unmarshal(networkInterfaces, &nics); err != nil {
		return
	}

	// 构建当前所有网卡上的 IP 集合
	currentIPs := map[string]bool{}
	for _, nic := range nics {
		for _, ip := range nic.IPv4 {
			currentIPs[ip.Address] = true
		}
		for _, ip := range nic.IPv6 {
			currentIPs[ip.Address] = true
		}
	}

	// 查询该节点所有 active 的 host 类型 EIP 池
	var pools []models.EIPPool
	db.DB.Where("node_id = ? AND pool_type = ? AND status = ?", nodeID, models.EIPPoolTypeHost, models.EIPPoolStatusActive).Find(&pools)

	for i := range pools {
		pool := &pools[i]
		// 从 CIDR 中提取 IP
		poolIP := pool.CIDR
		if idx := strings.Index(pool.CIDR, "/"); idx > 0 {
			poolIP = pool.CIDR[:idx]
		}
		if !currentIPs[poolIP] {
			// IP 已不在网卡上，标记为 inactive
			db.DB.Model(pool).Update("status", "inactive")
			zap.L().Warn("host EIP 池 IP 已失效，标记为 inactive", zap.String("pool_id", pool.ID.String()), zap.String("old_ip", poolIP), zap.String("interface", pool.Interface))
		}
	}
}

// broadcastNodeHeartbeat 向前端广播节点心跳数据
func (m *Manager) broadcastNodeHeartbeat(nodeID uuid.UUID, hb HeartbeatPayload, now time.Time) {
	data, err := json.Marshal(map[string]interface{}{
		"type":    "node_heartbeat",
		"node_id": nodeID.String(),
		"payload": map[string]interface{}{
			"status":         "online",
			"is_online":      true,
			"used_cpu":       hb.CPUPercent,
			"used_memory":    hb.MemUsed,
			"mem_total":      hb.MemTotal,
			"used_disk":      hb.DiskUsed,
			"disk_total":     hb.DiskTotal,
			"net_in":         hb.NetIn,
			"net_out":        hb.NetOut,
			"uptime":         hb.Uptime,
			"instance_count": hb.Instances,
			"running_count":  hb.Running,
			"last_heartbeat": now,
		},
	})
	if err != nil {
		return
	}

	m.frontendMu.RLock()
	defer m.frontendMu.RUnlock()
	for _, fc := range m.frontendConns {
		select {
		case fc.SendCh <- data:
		default:
		}
	}
}

// broadcastNodeOffline 向前端广播节点离线
func (m *Manager) broadcastNodeOffline(nodeID uuid.UUID) {
	data, err := json.Marshal(map[string]interface{}{
		"type":    "node_heartbeat",
		"node_id": nodeID.String(),
		"payload": map[string]interface{}{
			"status":         "offline",
			"is_online":      false,
			"last_heartbeat": time.Now(),
		},
	})
	if err != nil {
		return
	}

	m.frontendMu.RLock()
	defer m.frontendMu.RUnlock()
	for _, fc := range m.frontendConns {
		select {
		case fc.SendCh <- data:
		default:
		}
	}
}

// handleInstanceStatus 处理实例状态上报
func (m *Manager) handleInstanceStatus(nodeID uuid.UUID, payload json.RawMessage) {
	var status InstanceStatusPayload
	if err := json.Unmarshal(payload, &status); err != nil {
		return
	}

	// Agent 上报的 instance_id 是 incus_name（如 tsukiyo-xxx），不是 UUID
	var instance models.Instance
	if err := db.DB.Where("incus_name = ? AND node_id = ?", status.InstanceID, nodeID).First(&instance).Error; err != nil {
		return
	}

	// 映射 Incus 状态到系统状态
	mappedStatus := mapIncusStatus(status.Status)
	updates := map[string]interface{}{
		"status": mappedStatus,
	}
	if status.IPv4 != "" {
		updates["ipv4_address"] = status.IPv4
	}
	if status.IPv6 != "" {
		updates["ipv6_address"] = status.IPv6
	}

	db.DB.Model(&instance).Updates(updates)

	// 更新缓存
	ctx := context.Background()
	statusKey := fmt.Sprintf("instance:%s:status", instance.ID)
	db.RedisClient.Set(ctx, statusKey, string(mappedStatus), 30*time.Second)
}

// mapIncusStatus 将 Incus 状态映射到系统状态
func mapIncusStatus(incusStatus string) models.InstanceStatus {
	switch strings.ToLower(incusStatus) {
	case "running":
		return models.InstanceStatusRunning
	case "stopped":
		return models.InstanceStatusStopped
	case "frozen":
		return models.InstanceStatusStopped
	case "error":
		return models.InstanceStatusError
	default:
		return models.InstanceStatusError
	}
}

// handleMetrics 处理监控指标上报
func (m *Manager) handleMetrics(nodeID uuid.UUID, payload json.RawMessage) {
	var metrics MetricsPayload
	if err := json.Unmarshal(payload, &metrics); err != nil {
		return
	}

	now := time.Now()
	for _, im := range metrics.Instances {
		// Agent 上报的 instance_id 是 incus_name，不是 UUID
		var instance models.Instance
		if err := db.DB.Where("incus_name = ? AND node_id = ?", im.InstanceID, nodeID).
			Select("id").First(&instance).Error; err != nil {
			continue
		}

		metric := models.InstanceMetric{
			InstanceID:   instance.ID,
			NodeID:       nodeID,
			Timestamp:    now,
			CPUPercent:   im.CPUPercent,
			MemUsed:      im.MemUsed,
			MemTotal:     im.MemTotal,
			DiskReadBps:  im.DiskRead,
			DiskWriteBps: im.DiskWrite,
			NetInBps:     im.NetIn,
			NetOutBps:    im.NetOut,
		}

		db.DB.Create(&metric)

		// 缓存最新指标
		ctx := context.Background()
		metricKey := fmt.Sprintf("instance:%s:metrics", instance.ID)
		metricData, _ := json.Marshal(map[string]interface{}{
			"cpu_percent": im.CPUPercent,
			"mem_used":    im.MemUsed,
			"mem_total":   im.MemTotal,
			"disk_read":   im.DiskRead,
			"disk_write":  im.DiskWrite,
			"net_in":      im.NetIn,
			"net_out":     im.NetOut,
			"timestamp":   now.Unix(),
		})
		db.RedisClient.Set(ctx, metricKey, metricData, 10*time.Second)
	}
}

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

// StartHeartbeatChecker 启动心跳检查器
func (m *Manager) StartHeartbeatChecker() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			m.mu.RLock()
			for nodeID, conn := range m.connections {
				if time.Since(conn.LastPing) > 60*time.Second {
					zap.L().Warn("Agent 心跳超时", zap.String("node_id", nodeID.String()))
					conn.Close()
				}
			}
			m.mu.RUnlock()
		}
	}()
}

// handleImageProgress 处理 Agent 上报的镜像下载进度或本地镜像列表
func (m *Manager) handleImageProgress(nodeID uuid.UUID, payload json.RawMessage) {
	// 先尝试解析为镜像列表上报（Agent 启动时 / 定期同步）
	var imageList struct {
		Images []string `json:"images"`
	}
	if err := json.Unmarshal(payload, &imageList); err == nil && imageList.Images != nil {
		m.handleImageListSync(nodeID, imageList.Images)
		return
	}

	// 单条下载进度
	var p ImageProgressPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		zap.L().Warn("解析镜像进度失败", zap.String("node_id", nodeID.String()), zap.Error(err))
		return
	}

	now := time.Now()

	m.imageMu.Lock()
	nodeMap, ok := m.imageProgress[nodeID]
	if !ok {
		nodeMap = make(map[string]*NodeImageStatus)
		m.imageProgress[nodeID] = nodeMap
	}
	oldStatus, existed := nodeMap[p.ImageID]
	nodeMap[p.ImageID] = &NodeImageStatus{
		ImageID:         p.ImageID,
		Stage:           p.Stage,
		Progress:        p.Progress,
		DownloadedBytes: p.DownloadedBytes,
		TotalBytes:      p.TotalBytes,
		SpeedBps:        p.SpeedBps,
		Error:           p.Error,
		UpdatedAt:       now,
	}
	m.imageMu.Unlock()

	// 限制广播频率：每 0.5 秒最多一次（或状态变化时立即广播）
	shouldBroadcast := true
	if existed && oldStatus.Stage == p.Stage && p.Stage == "downloading" {
		if now.Sub(oldStatus.UpdatedAt) < 500*time.Millisecond {
			shouldBroadcast = false
		}
	}
	if shouldBroadcast {
		m.BroadcastImageProgress(nodeID, p)
	}

	// 下载完成 → 持久化到 NodeImage 表
	if p.Stage == "done" {
		nodeStr := nodeID.String()
		var ni models.NodeImage
		if err := db.DB.Where("node_id = ? AND image_id = ?", nodeStr, p.ImageID).First(&ni).Error; err != nil {
			db.DB.Create(&models.NodeImage{
				NodeID:  nodeStr,
				ImageID: p.ImageID,
				Status:  "downloaded",
			})
		} else {
			db.DB.Model(&ni).Updates(map[string]interface{}{"status": "downloaded", "updated_at": now})
		}
		// 下载完成清除 Redis 中的中间进度
		ctx := context.Background()
		progressKey := fmt.Sprintf("image_progress:%s:%s", nodeStr, p.ImageID)
		db.RedisClient.Del(ctx, progressKey)
	} else if p.Stage == "downloading" {
		// 中间进度写入 Redis，防止 Master 重启丢失
		nodeStr := nodeID.String()
		ctx := context.Background()
		progressKey := fmt.Sprintf("image_progress:%s:%s", nodeStr, p.ImageID)
		progressData, _ := json.Marshal(map[string]interface{}{
			"stage":            p.Stage,
			"progress":         p.Progress,
			"downloaded_bytes": p.DownloadedBytes,
			"total_bytes":      p.TotalBytes,
			"speed_bps":        p.SpeedBps,
			"updated_at":       now.Unix(),
		})
		db.RedisClient.Set(ctx, progressKey, progressData, 10*time.Minute)
	}
}

// handleInstanceProgress 处理 Agent 上报的实例创建进度
func (m *Manager) handleInstanceProgress(nodeID uuid.UUID, payload json.RawMessage) {
	var p InstanceProgressPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		zap.L().Warn("解析实例进度失败", zap.String("node_id", nodeID.String()), zap.Error(err))
		return
	}

	// 直接广播给前端
	m.BroadcastInstanceProgress(nodeID, p)
}

// handleImageListSync 处理 Agent 全量镜像列表上报，执行增量同步 + 清理已删除
// agent 上报的是 image_key 列表 (alias|type|arch)，与 DB 中格式一致，直接精确匹配
func (m *Manager) handleImageListSync(nodeID uuid.UUID, imageKeys []string) {
	now := time.Now()
	nodeStr := nodeID.String()

	zap.L().Info("Agent 上报镜像列表", zap.String("node_id", nodeStr), zap.Int("count", len(imageKeys)))

	// 构建上报集合
	reported := make(map[string]struct{}, len(imageKeys))
	for _, key := range imageKeys {
		reported[key] = struct{}{}
	}

	// Upsert 上报的镜像
	for _, key := range imageKeys {
		var ni models.NodeImage
		if err := db.DB.Where("node_id = ? AND image_id = ?", nodeStr, key).First(&ni).Error; err != nil {
			db.DB.Create(&models.NodeImage{NodeID: nodeStr, ImageID: key, Status: "downloaded"})
		} else if ni.Status != "downloaded" {
			db.DB.Model(&ni).Updates(map[string]interface{}{"status": "downloaded", "updated_at": now})
		}
	}

	// 清理 DB 中该节点已不存在的镜像
	var existing []models.NodeImage
	db.DB.Where("node_id = ?", nodeStr).Find(&existing)
	for _, ni := range existing {
		if _, ok := reported[ni.ImageID]; !ok {
			zap.L().Info("删除节点镜像记录", zap.String("node_id", nodeStr), zap.String("image_id", ni.ImageID))
			db.DB.Where("node_id = ? AND image_id = ?", nodeStr, ni.ImageID).Delete(&models.NodeImage{})
			m.BroadcastImageProgress(nodeID, ImageProgressPayload{
				ImageID: ni.ImageID, Stage: "deleted", Progress: 0,
			})
		}
	}

	// 同步更新内存缓存
	m.imageMu.Lock()
	nodeMap := make(map[string]*NodeImageStatus, len(imageKeys))
	for _, key := range imageKeys {
		nodeMap[key] = &NodeImageStatus{
			ImageID: key, Stage: "done", Progress: 100, UpdatedAt: now,
		}
	}
	// 保留正在下载中的条目
	if oldMap, ok := m.imageProgress[nodeID]; ok {
		for k, v := range oldMap {
			if v.Stage == "downloading" {
				nodeMap[k] = v
			}
		}
	}
	m.imageProgress[nodeID] = nodeMap
	m.imageMu.Unlock()

	zap.L().Info("节点镜像列表已同步",
		zap.String("node_id", nodeStr),
		zap.Int("reported", len(imageKeys)),
		zap.Int("cleaned", len(existing)-len(imageKeys)))

	// 广播镜像列表更新给前端
	m.BroadcastImageProgress(nodeID, ImageProgressPayload{
		ImageID:  "",
		Stage:    "sync",
		Progress: 100,
	})
}

// GetImageProgress 获取指定节点的所有镜像下载状态
func (m *Manager) GetImageProgress(nodeID uuid.UUID) map[string]*NodeImageStatus {
	m.imageMu.RLock()
	nodeMap, ok := m.imageProgress[nodeID]
	m.imageMu.RUnlock()

	if !ok || len(nodeMap) == 0 {
		// 内存无数据，从数据库加载
		var nodeImages []models.NodeImage
		if err := db.DB.Where("node_id = ?", nodeID.String()).Find(&nodeImages).Error; err == nil && len(nodeImages) > 0 {
			m.imageMu.Lock()
			nodeMap = make(map[string]*NodeImageStatus)
			m.imageProgress[nodeID] = nodeMap
			for _, ni := range nodeImages {
				nodeMap[ni.ImageID] = &NodeImageStatus{
					ImageID:   ni.ImageID,
					Stage:     "done",
					Progress:  100,
					UpdatedAt: ni.UpdatedAt,
				}
			}
			m.imageMu.Unlock()
		} else {
			return nil
		}
	}

	// 返回副本
	m.imageMu.RLock()
	defer m.imageMu.RUnlock()
	result := make(map[string]*NodeImageStatus, len(nodeMap))
	for k, v := range nodeMap {
		cp := *v
		result[k] = &cp
	}
	return result
}

// GetSingleImageProgress 获取指定节点上指定镜像的下载状态
func (m *Manager) GetSingleImageProgress(nodeID uuid.UUID, imageID string) *NodeImageStatus {
	m.imageMu.RLock()
	defer m.imageMu.RUnlock()

	if nodeMap, ok := m.imageProgress[nodeID]; ok {
		if s, ok := nodeMap[imageID]; ok {
			cp := *s
			return &cp
		}
	}
	return nil
}

// getEIPAllocCIDR 查询 EIP 分配记录的 CIDR
func getEIPAllocCIDR(allocID *uuid.UUID) string {
	if allocID == nil {
		return ""
	}
	var alloc models.EIPAllocation
	if err := db.DB.Where("id = ?", *allocID).First(&alloc).Error; err != nil {
		return ""
	}
	return alloc.CIDR
}
