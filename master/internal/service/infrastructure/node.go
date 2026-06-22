package infrastructure

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"tsukiyo/master/internal/agent"
	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/service"
)

// NodeService 节点服务
type NodeService struct {
	agentMgr *agent.Manager
}

// NewNodeService 创建节点服务
func NewNodeService(agentMgr *agent.Manager) *NodeService {
	return &NodeService{agentMgr: agentMgr}
}

// CreateNode 创建节点
func (s *NodeService) CreateNode(name string) (*models.Node, error) {
	token := uuid.New().String() + uuid.New().String()

	node := models.Node{
		ID:     uuid.New(),
		Name:   name,
		Token:  token,
		Status: models.NodeStatusOffline,
	}

	if err := db.DB.Create(&node).Error; err != nil {
		zap.L().Error("创建节点失败", zap.Error(err))
		return nil, err
	}

	return &node, nil
}

// ListNodesRequest 获取节点列表请求
type ListNodesRequest struct {
	Page    int
	PerPage int
	Search  string
	Filters map[string]string
}

// ListNodes 获取节点列表（支持分页/搜索/筛选）
func (s *NodeService) ListNodes(req ListNodesRequest) ([]models.Node, int64, error) {
	query := db.DB.Model(&models.Node{})

	// 搜索：匹配 name、hostname、ip_address
	if req.Search != "" {
		search := "%" + req.Search + "%"
		query = query.Where("name ILIKE ? OR hostname ILIKE ? OR ip_address ILIKE ?", search, search, search)
	}

	// 筛选
	if v, ok := req.Filters["status"]; ok && v != "" {
		query = query.Where("status = ?", v)
	}
	if v, ok := req.Filters["is_online"]; ok && v != "" {
		if v == "true" {
			query = query.Where("last_heartbeat > ?", time.Now().Add(-1*time.Minute))
		} else if v == "false" {
			query = query.Where("last_heartbeat <= ? OR last_heartbeat IS NULL", time.Now().Add(-1*time.Minute))
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		zap.L().Error("查询节点总数失败", zap.Error(err))
		return nil, 0, err
	}

	var nodes []models.Node
	offset := (req.Page - 1) * req.PerPage
	if err := query.Order("created_at DESC").Limit(req.PerPage).Offset(offset).Find(&nodes).Error; err != nil {
		zap.L().Error("查询节点列表失败", zap.Error(err))
		return nil, 0, err
	}
	return nodes, total, nil
}

// GetNode 获取节点详情
func (s *NodeService) GetNode(nodeID uuid.UUID) (*models.Node, error) {
	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, service.ErrNodeNotFound
		}
		return nil, err
	}
	return &node, nil
}

// UpdateNodeConfig 更新节点配置
func (s *NodeService) UpdateNodeConfig(nodeID uuid.UUID, req map[string]interface{}) error {
	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return service.ErrNodeNotFound
		}
		return err
	}

	updates := map[string]interface{}{
		"status": models.NodeStatusOnline,
	}
	for k, v := range req {
		updates[k] = v
	}

	if err := db.DB.Model(&node).Updates(updates).Error; err != nil {
		zap.L().Error("更新节点配置失败", zap.Error(err))
		return err
	}

	// 下发配置给 Agent
	if s.agentMgr != nil {
		if err := s.agentMgr.SendConfig(nodeID, req); err != nil {
			zap.L().Warn("下发配置给 Agent 失败", zap.String("node_id", nodeID.String()), zap.Error(err))
		}
	}

	return nil
}

// DeleteNode 删除节点
func (s *NodeService) DeleteNode(nodeID uuid.UUID) error {
	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return service.ErrNodeNotFound
		}
		return err
	}

	// 检查节点下是否有实例
	var count int64
	db.DB.Model(&models.Instance{}).Where("node_id = ?", nodeID).Count(&count)
	if count > 0 {
		return service.ErrNodeHasInstances
	}

	if err := db.DB.Delete(&node).Error; err != nil {
		return err
	}

	return nil
}

// GetNodeNetworks 获取节点网卡列表
func (s *NodeService) GetNodeNetworks(nodeID uuid.UUID) ([]NetworkInfo, error) {
	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, service.ErrNodeNotFound
		}
		return nil, err
	}

	type sysInfo struct {
		Networks []NetworkInfo `json:"network_interfaces"`
	}
	var parsed sysInfo
	if err := json.Unmarshal([]byte(node.SystemInfo), &parsed); err != nil {
		return []NetworkInfo{}, nil
	}

	// 过滤：排除网桥和 loopback
	filtered := make([]NetworkInfo, 0, len(parsed.Networks))
	for _, n := range parsed.Networks {
		if n.Name == "lo" || strings.HasPrefix(n.Name, "br-") {
			continue
		}
		filtered = append(filtered, n)
	}

	return filtered, nil
}

// IsNodeOnline 检查节点是否在线
func (s *NodeService) IsNodeOnline(node *models.Node) bool {
	if node.Status != models.NodeStatusOnline {
		return false
	}
	if node.LastHeartbeat != nil {
		return time.Since(*node.LastHeartbeat) < 60*time.Second
	}
	return false
}

// NetworkInfo 网卡信息
type NetworkInfo struct {
	Name      string    `json:"name"`
	MAC       string    `json:"mac"`
	State     string    `json:"state"`
	SpeedMbps int       `json:"speed_mbps"`
	Driver    string    `json:"driver"`
	Model     string    `json:"model"`
	IPv4      []IPProbe `json:"ipv4"`
	IPv6      []IPProbe `json:"ipv6"`
}

// IPProbe IP 地址信息
type IPProbe struct {
	Interface string `json:"interface"`
	Address   string `json:"address"`
	PrefixLen int    `json:"prefix_len"`
	Scope     string `json:"scope"`
	Gateway   string `json:"gateway,omitempty"`
}
