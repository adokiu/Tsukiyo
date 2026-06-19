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

// ListNodes 获取节点列表
func (s *NodeService) ListNodes() ([]models.Node, error) {
	var nodes []models.Node
	if err := db.DB.Order("created_at DESC").Find(&nodes).Error; err != nil {
		zap.L().Error("查询节点列表失败", zap.Error(err))
		return nil, err
	}
	return nodes, nil
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
		"initialized": true,
		"status":      models.NodeStatusOnline,
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
		Networks []NetworkInfo `json:"networks"`
	}
	var parsed sysInfo
	if err := json.Unmarshal([]byte(node.SystemInfo), &parsed); err != nil {
		return []NetworkInfo{}, nil
	}

	// 过滤：排除 VPC bridge 和 loopback
	var filtered []NetworkInfo
	for _, n := range parsed.Networks {
		if n.Name == "lo" || strings.HasPrefix(n.Name, "vpc-") {
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
	Name   string   `json:"name"`
	Status string   `json:"status"`
	IPv4   []string `json:"ipv4"`
	IPv6   []string `json:"ipv6"`
	MAC    string   `json:"mac"`
}
