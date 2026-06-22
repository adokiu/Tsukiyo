package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  8192,
	WriteBufferSize: 8192,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var vncUpgrader = websocket.Upgrader{
	ReadBufferSize:  8192,
	WriteBufferSize: 8192,
	Subprotocols:    []string{"binary"},
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
	consoleSessions map[string]*websocket.Conn // sessionID -> 前端 WS 连接
	consoleMu       sync.RWMutex
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

// NewManager 创建 Agent 管理器
func NewManager() *Manager {
	return &Manager{
		connections:     make(map[uuid.UUID]*Connection),
		taskCh:          make(chan *TaskMessage, 1000),
		nodeStatusCh:    make(chan NodeStatusUpdate, 1000),
		pendingRequests: make(map[string]chan agentResponse),
		imageProgress:   make(map[uuid.UUID]map[string]*NodeImageStatus),
		frontendConns:   make([]*FrontendConn, 0),
		consoleSessions: make(map[string]*websocket.Conn),
	}
}
