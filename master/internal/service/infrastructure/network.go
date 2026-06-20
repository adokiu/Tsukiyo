package infrastructure

import (
	"encoding/json"
	"fmt"
	"net"
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

type NetworkService struct {
	agentMgr *agent.Manager
}

func NewNetworkService(agentMgr *agent.Manager) *NetworkService {
	return &NetworkService{agentMgr: agentMgr}
}

// =====================================================================
//  Bridge CRUD
// =====================================================================

func (s *NetworkService) ListBridges(nodeID string) ([]models.Bridge, error) {
	query := db.DB.Order("created_at DESC")
	if nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}
	var bridges []models.Bridge
	if err := query.Find(&bridges).Error; err != nil {
		return nil, err
	}
	return bridges, nil
}

func (s *NetworkService) GetBridge(bridgeID uuid.UUID) (*models.Bridge, error) {
	var bridge models.Bridge
	if err := db.DB.Where("id = ?", bridgeID).First(&bridge).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, service.ErrBridgeNotFound
		}
		return nil, err
	}
	return &bridge, nil
}

type CreateBridgeRequest struct {
	NodeID            uuid.UUID
	Name              string
	IPv4Enabled       bool
	IPv4CIDR          string
	IPv4Gateway       string
	IPv6Enabled       bool
	IPv6CIDR          string
	IPv6Gateway       string
	DNSServers        []string
	PortRangeStart    int
	PortRangeEnd      int
	NATEgressV4PoolID *uuid.UUID
	NATEgressV6PoolID *uuid.UUID
	UserID            uint
}

func (s *NetworkService) CreateBridge(req CreateBridgeRequest) (*models.Bridge, *models.Task, error) {
	var node models.Node
	if err := db.DB.Where("id = ?", req.NodeID).First(&node).Error; err != nil {
		return nil, nil, service.ErrNodeNotFound
	}

	if req.IPv4Enabled {
		if _, _, err := net.ParseCIDR(req.IPv4CIDR); err != nil {
			return nil, nil, service.ErrInvalidCIDR
		}
	}
	if req.IPv6Enabled {
		if _, _, err := net.ParseCIDR(req.IPv6CIDR); err != nil {
			return nil, nil, service.ErrInvalidCIDR
		}
	}

	// 校验 CIDR 不与同节点其他 Bridge 重叠
	var existingBridges []models.Bridge
	db.DB.Where("node_id = ?", req.NodeID).Find(&existingBridges)
	if req.IPv4Enabled {
		_, newV4Net, _ := net.ParseCIDR(req.IPv4CIDR)
		for _, b := range existingBridges {
			if !b.IPv4Enabled {
				continue
			}
			_, existingNet, _ := net.ParseCIDR(b.IPv4CIDR)
			if existingNet != nil && cidrOverlap(newV4Net, existingNet) {
				return nil, nil, service.ErrBridgeCIDROverlap
			}
		}
	}
	if req.IPv6Enabled {
		_, newV6Net, _ := net.ParseCIDR(req.IPv6CIDR)
		for _, b := range existingBridges {
			if !b.IPv6Enabled {
				continue
			}
			_, existingNet, _ := net.ParseCIDR(b.IPv6CIDR)
			if existingNet != nil && cidrOverlap(newV6Net, existingNet) {
				return nil, nil, service.ErrBridgeCIDROverlap
			}
		}
	}

	gatewayV4 := req.IPv4Gateway
	if gatewayV4 == "" && req.IPv4Enabled {
		ip, ipNet, _ := net.ParseCIDR(req.IPv4CIDR)
		ip = ip.Mask(ipNet.Mask)
		ip[len(ip)-1] = 1
		gatewayV4 = ip.String()
	}
	gatewayV6 := req.IPv6Gateway
	if gatewayV6 == "" && req.IPv6Enabled {
		ip, ipNet, _ := net.ParseCIDR(req.IPv6CIDR)
		ip = ip.Mask(ipNet.Mask)
		if len(ip) == 16 {
			ip[15] = 1
		}
		gatewayV6 = ip.String()
	}

	portStart := req.PortRangeStart
	if portStart <= 0 {
		portStart = 20000
	}
	portEnd := req.PortRangeEnd
	if portEnd <= 0 {
		portEnd = 65535
	}

	dnsJSON, _ := json.Marshal(req.DNSServers)
	if len(dnsJSON) == 0 {
		dnsJSON = []byte("[]")
	}

	bridgeID := uuid.New()
	bridgeName := fmt.Sprintf("br-%s", bridgeID.String()[:8])

	// 先通过 Agent 同步创建 Incus bridge，成功后才入库
	if s.agentMgr == nil || !s.agentMgr.IsNodeConnected(req.NodeID) {
		return nil, nil, service.ErrNodeNotConnected
	}

	taskPayload := map[string]interface{}{
		"bridge_id":    bridgeID.String(),
		"action":       "create",
		"bridge_name":  bridgeName,
		"ipv4_enabled": req.IPv4Enabled,
		"ipv4_cidr":    req.IPv4CIDR,
		"ipv4_gateway": gatewayV4,
		"ipv6_enabled": req.IPv6Enabled,
		"ipv6_cidr":    req.IPv6CIDR,
		"ipv6_gateway": gatewayV6,
		"dns_servers":  req.DNSServers,
	}

	_, err := s.agentMgr.SendRequest(req.NodeID, "bridge_network", taskPayload, 30*time.Second)
	if err != nil {
		return nil, nil, fmt.Errorf("Agent 创建 Bridge 失败: %w", err)
	}

	bridge := models.Bridge{
		ID:             bridgeID,
		NodeID:         req.NodeID,
		Name:           req.Name,
		BridgeName:     bridgeName,
		IPv4Enabled:    req.IPv4Enabled,
		IPv4CIDR:       req.IPv4CIDR,
		IPv4Gateway:    gatewayV4,
		IPv6Enabled:    req.IPv6Enabled,
		IPv6CIDR:       req.IPv6CIDR,
		IPv6Gateway:    gatewayV6,
		DNSServers:     dnsJSON,
		PortRangeStart: portStart,
		PortRangeEnd:   portEnd,
		Status:         models.BridgeStatusActive,
	}

	if err := db.DB.Create(&bridge).Error; err != nil {
		zap.L().Error("创建网桥失败", zap.Error(err))
		return nil, nil, err
	}

	// 分配并绑定 NAT 出口 EIP
	if req.NATEgressV4PoolID != nil && req.IPv4Enabled {
		alloc, err := s.allocateBridgeEgressFromPool(*req.NATEgressV4PoolID, bridge.ID, "ipv4")
		if err != nil {
			zap.L().Warn("分配 IPv4 NAT 出口 EIP 失败（非致命）", zap.Error(err))
		} else {
			db.DB.Model(&bridge).Update("nat_egress_ipv4_id", alloc.ID)
		}
	}
	if req.NATEgressV6PoolID != nil && req.IPv6Enabled {
		alloc, err := s.allocateBridgeEgressFromPool(*req.NATEgressV6PoolID, bridge.ID, "ipv6")
		if err != nil {
			zap.L().Warn("分配 IPv6 NAT 出口 EIP 失败（非致命）", zap.Error(err))
		} else {
			db.DB.Model(&bridge).Update("nat_egress_ipv6_id", alloc.ID)
		}
	}

	return &bridge, nil, nil
}

type UpdateBridgeRequest struct {
	Name           *string
	IPv4Enabled    *bool
	IPv4CIDR       *string
	IPv4Gateway    *string
	IPv6Enabled    *bool
	IPv6CIDR       *string
	IPv6Gateway    *string
	DNSServers     *[]string
	PortRangeStart *int
	PortRangeEnd   *int
	Status         *string
	UserID         uint
}

func (s *NetworkService) UpdateBridge(bridgeID uuid.UUID, req UpdateBridgeRequest) error {
	var bridge models.Bridge
	if err := db.DB.Where("id = ?", bridgeID).First(&bridge).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return service.ErrBridgeNotFound
		}
		return err
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.IPv4Enabled != nil {
		updates["ipv4_enabled"] = *req.IPv4Enabled
	}
	if req.IPv4CIDR != nil {
		if *req.IPv4CIDR != bridge.IPv4CIDR {
			var count int64
			db.DB.Model(&models.Instance{}).Where("bridge_id = ?", bridgeID).Count(&count)
			if count > 0 {
				return service.ErrBridgeHasInstances
			}
		}
		updates["ipv4_cidr"] = *req.IPv4CIDR
	}
	if req.IPv4Gateway != nil {
		updates["ipv4_gateway"] = *req.IPv4Gateway
	}
	if req.IPv6Enabled != nil {
		updates["ipv6_enabled"] = *req.IPv6Enabled
	}
	if req.IPv6CIDR != nil {
		if *req.IPv6CIDR != bridge.IPv6CIDR {
			var count int64
			db.DB.Model(&models.Instance{}).Where("bridge_id = ?", bridgeID).Count(&count)
			if count > 0 {
				return service.ErrBridgeHasInstances
			}
		}
		updates["ipv6_cidr"] = *req.IPv6CIDR
	}
	if req.IPv6Gateway != nil {
		updates["ipv6_gateway"] = *req.IPv6Gateway
	}
	if req.DNSServers != nil {
		dnsJSON, _ := json.Marshal(*req.DNSServers)
		updates["dns_servers"] = dnsJSON
	}
	if req.PortRangeStart != nil {
		updates["port_range_start"] = *req.PortRangeStart
	}
	if req.PortRangeEnd != nil {
		updates["port_range_end"] = *req.PortRangeEnd
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	// 先同步调用 Agent 更新 Incus bridge
	if s.agentMgr != nil && s.agentMgr.IsNodeConnected(bridge.NodeID) {
		var updatedBridge models.Bridge
		db.DB.Where("id = ?", bridgeID).First(&updatedBridge)
		// 应用更新到临时对象用于构造 payload
		if v, ok := updates["name"]; ok {
			updatedBridge.Name = v.(string)
		}
		if v, ok := updates["ipv4_enabled"]; ok {
			updatedBridge.IPv4Enabled = v.(bool)
		}
		if v, ok := updates["ipv4_cidr"]; ok {
			updatedBridge.IPv4CIDR = v.(string)
		}
		if v, ok := updates["ipv4_gateway"]; ok {
			updatedBridge.IPv4Gateway = v.(string)
		}
		if v, ok := updates["ipv6_enabled"]; ok {
			updatedBridge.IPv6Enabled = v.(bool)
		}
		if v, ok := updates["ipv6_cidr"]; ok {
			updatedBridge.IPv6CIDR = v.(string)
		}
		if v, ok := updates["ipv6_gateway"]; ok {
			updatedBridge.IPv6Gateway = v.(string)
		}

		var dnsServers []string
		json.Unmarshal(updatedBridge.DNSServers, &dnsServers)
		if v, ok := updates["dns_servers"]; ok {
			json.Unmarshal(v.([]byte), &dnsServers)
		}

		taskPayload := map[string]interface{}{
			"bridge_id":    updatedBridge.ID.String(),
			"action":       "update",
			"bridge_name":  updatedBridge.BridgeName,
			"ipv4_enabled": updatedBridge.IPv4Enabled,
			"ipv4_cidr":    updatedBridge.IPv4CIDR,
			"ipv4_gateway": updatedBridge.IPv4Gateway,
			"ipv6_enabled": updatedBridge.IPv6Enabled,
			"ipv6_cidr":    updatedBridge.IPv6CIDR,
			"ipv6_gateway": updatedBridge.IPv6Gateway,
			"dns_servers":  dnsServers,
		}
		_, err := s.agentMgr.SendRequest(bridge.NodeID, "bridge_network", taskPayload, 30*time.Second)
		if err != nil {
			return fmt.Errorf("Agent 更新 Bridge 失败: %w", err)
		}
	}

	if len(updates) > 0 {
		if err := db.DB.Model(&bridge).Updates(updates).Error; err != nil {
			return err
		}
	}

	return nil
}

func (s *NetworkService) DeleteBridge(bridgeID uuid.UUID, userID uint) error {
	var bridge models.Bridge
	if err := db.DB.Where("id = ?", bridgeID).First(&bridge).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return service.ErrBridgeNotFound
		}
		return err
	}

	var count int64
	db.DB.Model(&models.Instance{}).Where("bridge_id = ?", bridgeID).Count(&count)
	if count > 0 {
		return service.ErrBridgeHasInstances
	}

	// 同步调用 Agent 删除 Incus bridge
	if s.agentMgr != nil && s.agentMgr.IsNodeConnected(bridge.NodeID) {
		taskPayload := map[string]interface{}{
			"bridge_id":   bridge.ID.String(),
			"action":      "delete",
			"bridge_name": bridge.BridgeName,
		}
		_, err := s.agentMgr.SendRequest(bridge.NodeID, "bridge_network", taskPayload, 30*time.Second)
		if err != nil {
			return fmt.Errorf("Agent 删除 Bridge 失败: %w", err)
		}
	}

	if err := db.DB.Delete(&bridge).Error; err != nil {
		return err
	}

	return nil
}

// allocateBridgeEgressFromPool 从指定 EIP 池中分配一个 EIP 作为网桥 NAT 出口
func (s *NetworkService) allocateBridgeEgressFromPool(poolID uuid.UUID, bridgeID uuid.UUID, ipVersion string) (*models.EIPAllocation, error) {
	var pool models.EIPPool
	if err := db.DB.Where("id = ? AND status = ?", poolID, models.EIPPoolStatusActive).First(&pool).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, service.ErrEIPPoolNotFound
		}
		return nil, err
	}

	if pool.IPVersion != ipVersion {
		return nil, service.ErrEIPNotAvailable
	}

	prefixLen := 32
	if ipVersion == "ipv6" {
		prefixLen = 128
	}

	alloc, err := s.tryAllocateFromPool(pool, prefixLen, "")
	if err != nil || alloc == nil {
		return nil, fmt.Errorf("从资源池分配失败: %w", err)
	}

	// 设置 usage 为 bridge_nat_egress 并关联网桥
	alloc.Usage = models.EIPUsageBridgeNATEgress
	alloc.BridgeID = &bridgeID
	if err := db.DB.Model(&alloc).Updates(map[string]interface{}{
		"usage":     models.EIPUsageBridgeNATEgress,
		"bridge_id": bridgeID,
	}).Error; err != nil {
		return nil, err
	}

	// 通知 Agent 绑定出口 IP
	var bridge models.Bridge
	db.DB.Where("id = ?", bridgeID).First(&bridge)

	if s.agentMgr != nil && s.agentMgr.IsNodeConnected(bridge.NodeID) {
		payload := map[string]interface{}{
			"bridge_name": bridge.BridgeName,
			"egress_cidr": alloc.CIDR,
			"interface":   pool.Interface,
			"ip_version":  ipVersion,
		}
		_, err := s.agentMgr.SendRequest(bridge.NodeID, "bind_bridge_egress", payload, 15*time.Second)
		if err != nil {
			zap.L().Warn("Agent 绑定网桥出口 IP 失败", zap.Error(err))
		}
	}

	return alloc, nil
}

// =====================================================================
//  Bridge NAT 出口 IP 绑定
// =====================================================================

func (s *NetworkService) BindBridgeEgress(bridgeID uuid.UUID, poolID uuid.UUID, ipVersion string, userID uint) error {
	var bridge models.Bridge
	if err := db.DB.Where("id = ?", bridgeID).First(&bridge).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return service.ErrBridgeNotFound
		}
		return err
	}

	// 如果已绑定，先解绑旧的
	var oldAllocID *uuid.UUID
	if ipVersion == "ipv4" {
		oldAllocID = bridge.NATEgressIPv4ID
	} else {
		oldAllocID = bridge.NATEgressIPv6ID
	}
	if oldAllocID != nil {
		s.UnbindBridgeEgress(bridgeID, ipVersion, userID)
	}

	// 从池中分配新 EIP
	alloc, err := s.allocateBridgeEgressFromPool(poolID, bridgeID, ipVersion)
	if err != nil {
		return err
	}

	// 更新 bridge 的 nat_egress 字段
	field := "nat_egress_ipv4_id"
	if ipVersion == "ipv6" {
		field = "nat_egress_ipv6_id"
	}
	if err := db.DB.Model(&bridge).Update(field, alloc.ID).Error; err != nil {
		return err
	}

	return nil
}

func (s *NetworkService) UnbindBridgeEgress(bridgeID uuid.UUID, ipVersion string, userID uint) error {
	var bridge models.Bridge
	if err := db.DB.Where("id = ?", bridgeID).First(&bridge).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return service.ErrBridgeNotFound
		}
		return err
	}

	field := "nat_egress_ipv4_id"
	if ipVersion == "ipv6" {
		field = "nat_egress_ipv6_id"
	}

	var allocID *uuid.UUID
	if ipVersion == "ipv4" {
		allocID = bridge.NATEgressIPv4ID
	} else {
		allocID = bridge.NATEgressIPv6ID
	}
	if allocID == nil {
		return nil
	}

	// 查询 allocation 和 pool 信息
	var alloc models.EIPAllocation
	if err := db.DB.Where("id = ?", *allocID).First(&alloc).Error; err != nil {
		zap.L().Warn("查询 EIP allocation 失败", zap.String("alloc_id", allocID.String()), zap.Error(err))
	} else {
		var pool models.EIPPool
		db.DB.Where("id = ?", alloc.PoolID).First(&pool)

		// 通知 Agent 解绑出口 IP
		if s.agentMgr != nil && s.agentMgr.IsNodeConnected(bridge.NodeID) {
			payload := map[string]interface{}{
				"bridge_name": bridge.BridgeName,
				"egress_cidr": alloc.CIDR,
				"interface":   pool.Interface,
				"ip_version":  ipVersion,
			}
			_, err := s.agentMgr.SendRequest(bridge.NodeID, "unbind_bridge_egress", payload, 15*time.Second)
			if err != nil {
				zap.L().Warn("Agent 解绑网桥出口 IP 失败", zap.Error(err))
			}
		}

		// 释放 EIP allocation
		now := time.Now()
		db.DB.Model(&alloc).Updates(map[string]interface{}{
			"status":      models.EIPAllocationReleased,
			"released_at": &now,
			"bridge_id":   nil,
		})
	}

	return db.DB.Model(&bridge).Update(field, nil).Error
}

// =====================================================================
//  EIP 资源池管�?// =====================================================================

func (s *NetworkService) ListEIPPools(nodeID string) ([]models.EIPPool, error) {
	query := db.DB.Order("created_at DESC")
	if nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}
	var pools []models.EIPPool
	if err := query.Find(&pools).Error; err != nil {
		return nil, err
	}

	// 查询已被 bridge_nat_egress 占用的池 ID，从列表中排除
	var usedPoolIDs []uuid.UUID
	db.DB.Model(&models.EIPAllocation{}).
		Where("usage = ? AND status = ?", models.EIPUsageBridgeNATEgress, models.EIPAllocationAssigned).
		Pluck("pool_id", &usedPoolIDs)
	usedSet := make(map[uuid.UUID]bool, len(usedPoolIDs))
	for _, id := range usedPoolIDs {
		usedSet[id] = true
	}
	filtered := pools[:0]
	for _, p := range pools {
		if !usedSet[p.ID] {
			filtered = append(filtered, p)
		}
	}
	return filtered, nil
}

type CreateEIPPoolRequest struct {
	NodeID    uuid.UUID
	IPVersion string
	CIDR      string
	Interface string
	Gateway   string
	Alias     string
	PoolType  string
	UserID    uint
}

func (s *NetworkService) CreateEIPPool(req CreateEIPPoolRequest) (*models.EIPPool, error) {
	var node models.Node
	if err := db.DB.Where("id = ?", req.NodeID).First(&node).Error; err != nil {
		return nil, service.ErrNodeNotFound
	}

	_, ipNet, err := net.ParseCIDR(req.CIDR)
	if err != nil {
		return nil, service.ErrInvalidCIDR
	}
	ones, _ := ipNet.Mask.Size()

	// 校验 CIDR 不与同节点同 IP 版本其他池重复
	var existingPools []models.EIPPool
	db.DB.Where("node_id = ? AND ip_version = ?", req.NodeID, req.IPVersion).Find(&existingPools)
	for _, p := range existingPools {
		_, existingNet, _ := net.ParseCIDR(p.CIDR)
		if existingNet == nil {
			continue
		}
		if req.IPVersion == "ipv4" {
			// IPv4：比较原始 IP 地址是否相同
			reqIPStr := req.CIDR
			if idx := strings.Index(req.CIDR, "/"); idx > 0 {
				reqIPStr = req.CIDR[:idx]
			}
			existingIPStr := p.CIDR
			if idx := strings.Index(p.CIDR, "/"); idx > 0 {
				existingIPStr = p.CIDR[:idx]
			}
			reqIP := net.ParseIP(reqIPStr)
			existingIP := net.ParseIP(existingIPStr)
			if reqIP != nil && existingIP != nil && reqIP.Equal(existingIP) {
				return nil, service.ErrEIPPoolCIDROverlap
			}
		} else {
			// IPv6：检查网段重叠
			if cidrOverlap(ipNet, existingNet) {
				return nil, service.ErrEIPPoolCIDROverlap
			}
		}
	}

	poolType := models.EIPPoolTypeEIP
	if req.PoolType == "host" {
		poolType = models.EIPPoolTypeHost
	}

	alias := req.Alias
	if alias == "" {
		alias = req.CIDR
	}

	pool := models.EIPPool{
		ID:        uuid.New(),
		NodeID:    req.NodeID,
		IPVersion: req.IPVersion,
		CIDR:      req.CIDR,
		Interface: req.Interface,
		Gateway:   req.Gateway,
		PrefixLen: ones,
		Alias:     alias,
		PoolType:  poolType,
		Status:    models.EIPPoolStatusActive,
	}

	if err := db.DB.Create(&pool).Error; err != nil {
		return nil, err
	}

	// 通过 Agent 在宿主机网卡上添加 IP（如果网卡上没有该 IP）
	if s.agentMgr != nil && s.agentMgr.IsNodeConnected(req.NodeID) && req.Interface != "" {
		_, err := s.agentMgr.SendRequest(req.NodeID, "add_ip", map[string]interface{}{
			"cidr":      req.CIDR,
			"interface": req.Interface,
		}, 15*time.Second)
		if err != nil {
			zap.L().Error("通过 Agent 添加 IP 到网卡失败",
				zap.String("cidr", req.CIDR),
				zap.String("interface", req.Interface),
				zap.Error(err))
			return nil, fmt.Errorf("Agent 添加 IP 失败: %w", err)
		}
	}

	return &pool, nil
}

func (s *NetworkService) DeleteEIPPool(poolID uuid.UUID) error {
	var count int64
	db.DB.Model(&models.EIPAllocation{}).Where("pool_id = ? AND status = ?", poolID, models.EIPAllocationAssigned).Count(&count)
	if count > 0 {
		return service.ErrEIPPoolHasAllocations
	}
	return db.DB.Where("id = ?", poolID).Delete(&models.EIPPool{}).Error
}

// =====================================================================
//  EIP 分配管理
// =====================================================================

func (s *NetworkService) ListEIPAllocations(nodeID string, instanceID string) ([]models.EIPAllocation, error) {
	query := db.DB.Where("status = ?", models.EIPAllocationAssigned).Order("allocated_at DESC")
	if nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}
	if instanceID != "" {
		query = query.Where("instance_id = ?", instanceID)
	}
	var allocs []models.EIPAllocation
	if err := query.Find(&allocs).Error; err != nil {
		return nil, err
	}
	return allocs, nil
}

func (s *NetworkService) AllocateEIP(nodeID uuid.UUID, ipVersion string, prefixLen int, specificIP string) (*models.EIPAllocation, error) {
	var pools []models.EIPPool
	if err := db.DB.Where("node_id = ? AND ip_version = ? AND status = ? AND pool_type = ?",
		nodeID, ipVersion, models.EIPPoolStatusActive, models.EIPPoolTypeEIP).Find(&pools).Error; err != nil {
		return nil, err
	}

	if len(pools) == 0 {
		return nil, service.ErrNoAvailableEIP
	}

	for _, pool := range pools {
		alloc, err := s.tryAllocateFromPool(pool, prefixLen, specificIP)
		if err == nil && alloc != nil {
			return alloc, nil
		}
	}

	return nil, service.ErrNoAvailableEIP
}

func (s *NetworkService) tryAllocateFromPool(pool models.EIPPool, prefixLen int, specificIP string) (*models.EIPAllocation, error) {
	_, ipNet, err := net.ParseCIDR(pool.CIDR)
	if err != nil {
		return nil, err
	}

	poolOnes, poolBits := ipNet.Mask.Size()
	if prefixLen == 0 {
		if pool.IPVersion == "ipv4" {
			prefixLen = 32
		} else {
			prefixLen = 128
		}
	}
	if prefixLen < poolOnes {
		return nil, fmt.Errorf("请求的前缀长度小于资源池前缀长度")
	}

	var existingAllocs []models.EIPAllocation
	db.DB.Where("pool_id = ? AND status = ?", pool.ID, models.EIPAllocationAssigned).Find(&existingAllocs)

	usedRanges := make([]ipRange, 0, len(existingAllocs))
	for _, a := range existingAllocs {
		_, aNet, err := net.ParseCIDR(a.CIDR)
		if err != nil {
			continue
		}
		start, end := cidrToRange(aNet)
		usedRanges = append(usedRanges, ipRange{start: start, end: end})
	}

	if specificIP != "" {
		targetCIDR := specificIP
		if !strings.Contains(specificIP, "/") {
			targetCIDR = fmt.Sprintf("%s/%d", specificIP, prefixLen)
		}
		_, targetNet, err := net.ParseCIDR(targetCIDR)
		if err != nil {
			return nil, err
		}
		if !ipNet.Contains(targetNet.IP) {
			return nil, service.ErrEIPNotAvailable
		}
		targetStart, targetEnd := cidrToRange(targetNet)
		for _, r := range usedRanges {
			if targetStart >= r.start && targetStart <= r.end {
				return nil, service.ErrEIPAlreadyAssigned
			}
			if targetEnd >= r.start && targetEnd <= r.end {
				return nil, service.ErrEIPAlreadyAssigned
			}
		}
		return s.createEIPAllocation(pool, targetCIDR, prefixLen), nil
	}

	subnetSize := uint64(1) << uint(poolBits-prefixLen)
	poolStart, poolEnd := cidrToRange(ipNet)

	// IPv4 池为单个 IP，直接提取 CIDR 中的 IP 本身
	if pool.IPVersion == "ipv4" {
		// pool.CIDR 格式为 "IP/prefix"，提取 IP 部分
		hostIPStr := pool.CIDR
		if idx := strings.Index(hostIPStr, "/"); idx > 0 {
			hostIPStr = hostIPStr[:idx]
		}
		hostCIDR := fmt.Sprintf("%s/%d", hostIPStr, prefixLen)
		// 解析实际 IP 用于冲突检查
		hostIP := net.ParseIP(hostIPStr)
		if hostIP == nil {
			return nil, fmt.Errorf("无效的 IP: %s", hostIPStr)
		}
		hostStart := ipToUint64(hostIP)
		// 检查是否已被分配
		for _, r := range usedRanges {
			if hostStart >= r.start && hostStart <= r.end {
				return nil, service.ErrNoAvailableEIP
			}
		}
		return s.createEIPAllocation(pool, hostCIDR, prefixLen), nil
	}

	// IPv4 /32 分配时跳过网络地址和广播地址
	if poolBits == 32 && prefixLen == 32 {
		if poolStart == 0 {
			poolStart++
		}
		poolEnd--
	}

	for cur := poolStart; cur+subnetSize-1 <= poolEnd; cur += subnetSize {
		overlap := false
		for _, r := range usedRanges {
			if cur >= r.start && cur <= r.end {
				overlap = true
				break
			}
			if cur+subnetSize-1 >= r.start && cur+subnetSize-1 <= r.end {
				overlap = true
				break
			}
		}
		if !overlap {
			ip := rangeToIP(cur, len(ipNet.IP))
			cidr := fmt.Sprintf("%s/%d", ip.String(), prefixLen)
			return s.createEIPAllocation(pool, cidr, prefixLen), nil
		}
	}

	return nil, service.ErrNoAvailableEIP
}

type ipRange struct {
	start uint64
	end   uint64
}

func cidrToRange(ipNet *net.IPNet) (uint64, uint64) {
	start := ipToUint64(ipNet.IP)
	ones, bits := ipNet.Mask.Size()
	end := start + (uint64(1) << uint(bits-ones)) - 1
	return start, end
}

func rangeToIP(val uint64, size int) net.IP {
	if size == 4 {
		return net.IPv4(byte(val>>24), byte(val>>16), byte(val>>8), byte(val))
	}
	ip := make(net.IP, 16)
	for i := 15; i >= 0; i-- {
		ip[i] = byte(val)
		val >>= 8
	}
	return ip
}

func ipToUint64(ip net.IP) uint64 {
	ip = ip.To16()
	if ip == nil {
		return 0
	}
	var val uint64
	for i := 8; i < 16; i++ {
		val = (val << 8) | uint64(ip[i])
	}
	return val
}

func (s *NetworkService) createEIPAllocation(pool models.EIPPool, cidr string, prefixLen int) *models.EIPAllocation {
	alloc := models.EIPAllocation{
		ID:        uuid.New(),
		PoolID:    pool.ID,
		NodeID:    pool.NodeID,
		CIDR:      cidr,
		PrefixLen: prefixLen,
		IPVersion: pool.IPVersion,
		Usage:     models.EIPUsageInstanceEIP,
		Status:    models.EIPAllocationAssigned,
	}
	if err := db.DB.Create(&alloc).Error; err != nil {
		zap.L().Error("创建 EIP 分配记录失败", zap.Error(err))
		return nil
	}
	return &alloc
}

func (s *NetworkService) ReleaseEIP(allocationID uuid.UUID) error {
	var alloc models.EIPAllocation
	if err := db.DB.Where("id = ?", allocationID).First(&alloc).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return service.ErrEIPAllocationNotFound
		}
		return err
	}

	if alloc.Status == models.EIPAllocationReleased {
		return nil
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":      models.EIPAllocationReleased,
		"released_at": &now,
		"bridge_id":   nil,
		"instance_id": nil,
	}

	return db.DB.Model(&alloc).Updates(updates).Error
}

func (s *NetworkService) AssignEIPToInstance(allocationID uuid.UUID, instanceID uuid.UUID) error {
	var alloc models.EIPAllocation
	if err := db.DB.Where("id = ? AND status = ?", allocationID, models.EIPAllocationAssigned).First(&alloc).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return service.ErrEIPAllocationNotFound
		}
		return err
	}

	if alloc.Usage != models.EIPUsageInstanceEIP {
		return service.ErrEIPNotAvailable
	}

	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		return service.ErrInstanceNotFound
	}

	if instance.BridgeID == nil {
		return service.ErrInstanceNoBridge
	}

	field := "ipv4_eip_allocation_id"
	if alloc.IPVersion == "ipv6" {
		field = "ipv6_eip_allocation_id"
	}

	if err := db.DB.Model(&alloc).Updates(map[string]interface{}{
		"instance_id": instanceID,
	}).Error; err != nil {
		return err
	}

	if err := db.DB.Model(&instance).Update(field, allocationID).Error; err != nil {
		return err
	}

	var bridge models.Bridge
	db.DB.Where("id = ?", *instance.BridgeID).First(&bridge)
	var pool models.EIPPool
	db.DB.Where("id = ?", alloc.PoolID).First(&pool)

	if s.agentMgr != nil && s.agentMgr.IsNodeConnected(instance.NodeID) {
		internalIP := instance.InternalIPv4
		if alloc.IPVersion == "ipv6" {
			internalIP = instance.InternalIPv6
		}
		payload, _ := json.Marshal(map[string]interface{}{
			"instance_name": instance.IncusName,
			"instance_ip":   internalIP,
			"eip_cidr":      alloc.CIDR,
			"interface":     pool.Interface,
			"ip_version":    alloc.IPVersion,
			"bridge_name":   bridge.BridgeName,
		})
		_, err := s.agentMgr.SendRequest(instance.NodeID, "assign_eip", payload, 30*time.Second)
		if err != nil {
			zap.L().Warn("Agent 分配 EIP 失败", zap.Error(err))
		}
	}

	return nil
}

func (s *NetworkService) ReleaseInstanceEIP(allocationID uuid.UUID) error {
	var alloc models.EIPAllocation
	if err := db.DB.Where("id = ?", allocationID).First(&alloc).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return service.ErrEIPAllocationNotFound
		}
		return err
	}

	if alloc.InstanceID != nil {
		var instance models.Instance
		db.DB.Where("id = ?", *alloc.InstanceID).First(&instance)

		var bridge models.Bridge
		if instance.BridgeID != nil {
			db.DB.Where("id = ?", *instance.BridgeID).First(&bridge)
		}
		var pool models.EIPPool
		db.DB.Where("id = ?", alloc.PoolID).First(&pool)

		if s.agentMgr != nil && s.agentMgr.IsNodeConnected(instance.NodeID) {
			internalIP := instance.InternalIPv4
			if alloc.IPVersion == "ipv6" {
				internalIP = instance.InternalIPv6
			}
			payload, _ := json.Marshal(map[string]interface{}{
				"instance_name": instance.IncusName,
				"instance_ip":   internalIP,
				"eip_cidr":      alloc.CIDR,
				"interface":     pool.Interface,
				"ip_version":    alloc.IPVersion,
				"bridge_name":   bridge.BridgeName,
			})
			_, err := s.agentMgr.SendRequest(instance.NodeID, "release_eip", payload, 30*time.Second)
			if err != nil {
				zap.L().Warn("Agent 释放 EIP 失败", zap.Error(err))
			}
		}

		field := "ipv4_eip_allocation_id"
		if alloc.IPVersion == "ipv6" {
			field = "ipv6_eip_allocation_id"
		}
		db.DB.Model(&models.Instance{}).Where("id = ?", *alloc.InstanceID).Update(field, nil)
	}

	return s.ReleaseEIP(allocationID)
}

// =====================================================================
//  端口映射管理
// =====================================================================

func (s *NetworkService) ListPortMappings(instanceID string) ([]models.PortMapping, error) {
	query := db.DB.Order("created_at DESC")
	if instanceID != "" {
		query = query.Where("instance_id = ?", instanceID)
	}
	var pms []models.PortMapping
	if err := query.Find(&pms).Error; err != nil {
		return nil, err
	}
	return pms, nil
}

func (s *NetworkService) AllocatePortMappingsForInstance(instanceID uuid.UUID, bridgeID uuid.UUID, nodeID uuid.UUID, count int, extraPorts []int, ipVersion string) ([]models.PortMapping, error) {
	var bridge models.Bridge
	if err := db.DB.Where("id = ?", bridgeID).First(&bridge).Error; err != nil {
		return nil, service.ErrBridgeNotFound
	}

	var egressAllocID *uuid.UUID
	if ipVersion == "ipv4" {
		egressAllocID = bridge.NATEgressIPv4ID
	} else {
		egressAllocID = bridge.NATEgressIPv6ID
	}
	if egressAllocID == nil {
		return nil, service.ErrNoBridgeEgressIP
	}

	var mappings []models.PortMapping

	allocatePort := func(containerPort int, protocol string, desc string) (*models.PortMapping, error) {
		hostPort, err := s.findAvailablePort(bridge.ID, bridge.PortRangeStart, bridge.PortRangeEnd, protocol)
		if err != nil {
			return nil, err
		}
		pm := models.PortMapping{
			ID:                 uuid.New(),
			InstanceID:         instanceID,
			NodeID:             nodeID,
			BridgeID:           bridgeID,
			IPVersion:          ipVersion,
			EgressAllocationID: *egressAllocID,
			ContainerPort:      containerPort,
			HostPort:           hostPort,
			Protocol:           protocol,
			Description:        desc,
		}
		if err := db.DB.Create(&pm).Error; err != nil {
			return nil, err
		}
		return &pm, nil
	}

	sshPM, err := allocatePort(22, "tcp", "SSH")
	if err != nil {
		return nil, err
	}
	mappings = append(mappings, *sshPM)

	for _, port := range extraPorts {
		if port == 22 {
			continue
		}
		pm, err := allocatePort(port, "tcp", "")
		if err != nil {
			break
		}
		mappings = append(mappings, *pm)
	}

	return mappings, nil
}

func (s *NetworkService) findAvailablePort(bridgeID uuid.UUID, start, end int, protocol string) (int, error) {
	var usedPorts []int
	db.DB.Model(&models.PortMapping{}).Where("bridge_id = ? AND protocol = ?", bridgeID, protocol).Pluck("host_port", &usedPorts)

	used := make(map[int]bool, len(usedPorts))
	for _, p := range usedPorts {
		used[p] = true
	}

	for port := start; port <= end; port++ {
		if !used[port] {
			return port, nil
		}
	}

	return 0, service.ErrNoAvailablePorts
}

func (s *NetworkService) AddPortMapping(instanceID uuid.UUID, containerPort int, protocol string, ipVersion string, description string) (*models.PortMapping, error) {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		return nil, service.ErrInstanceNotFound
	}
	if instance.BridgeID == nil {
		return nil, service.ErrInstanceNoBridge
	}

	// 检查端口映射配额
	var currentCount int64
	db.DB.Model(&models.PortMapping{}).Where("instance_id = ?", instanceID).Count(&currentCount)
	if instance.PortMappingLimit > 0 && int(currentCount) >= instance.PortMappingLimit {
		return nil, service.ErrPortMappingLimitReached
	}

	var bridge models.Bridge
	if err := db.DB.Where("id = ?", *instance.BridgeID).First(&bridge).Error; err != nil {
		return nil, service.ErrBridgeNotFound
	}

	egressAllocID := bridge.NATEgressIPv4ID
	if ipVersion == "ipv6" {
		egressAllocID = bridge.NATEgressIPv6ID
	}
	if egressAllocID == nil {
		return nil, service.ErrNoBridgeEgressIP
	}

	hostPort, err := s.findAvailablePort(bridge.ID, bridge.PortRangeStart, bridge.PortRangeEnd, protocol)
	if err != nil {
		return nil, err
	}

	pm := models.PortMapping{
		ID:                 uuid.New(),
		InstanceID:         instanceID,
		NodeID:             instance.NodeID,
		BridgeID:           *instance.BridgeID,
		IPVersion:          ipVersion,
		EgressAllocationID: *egressAllocID,
		ContainerPort:      containerPort,
		HostPort:           hostPort,
		Protocol:           protocol,
		Description:        description,
	}
	if err := db.DB.Create(&pm).Error; err != nil {
		return nil, err
	}

	var egressAlloc models.EIPAllocation
	db.DB.Where("id = ?", *egressAllocID).First(&egressAlloc)

	if s.agentMgr != nil && s.agentMgr.IsNodeConnected(instance.NodeID) {
		internalIP := instance.InternalIPv4
		if ipVersion == "ipv6" {
			internalIP = instance.InternalIPv6
		}
		payload, _ := json.Marshal(map[string]interface{}{
			"instance_id":    instance.IncusName,
			"host_port":      hostPort,
			"container_port": containerPort,
			"protocol":       protocol,
			"host_ip":        egressAlloc.GetIP(),
			"internal_ip":    internalIP,
		})
		_, err := s.agentMgr.SendRequest(instance.NodeID, "add_port_mapping", payload, 15*time.Second)
		if err != nil {
			zap.L().Warn("Agent 添加端口映射失败", zap.Error(err))
		}
	}

	return &pm, nil
}

func (s *NetworkService) DeletePortMapping(pmID uuid.UUID) error {
	var pm models.PortMapping
	if err := db.DB.Where("id = ?", pmID).First(&pm).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return service.ErrPortMappingNotFound
		}
		return err
	}

	var instance models.Instance
	db.DB.Where("id = ?", pm.InstanceID).First(&instance)

	if s.agentMgr != nil && s.agentMgr.IsNodeConnected(instance.NodeID) {
		payload, _ := json.Marshal(map[string]interface{}{
			"instance_id": instance.IncusName,
			"host_port":   pm.HostPort,
			"protocol":    pm.Protocol,
		})
		_, err := s.agentMgr.SendRequest(instance.NodeID, "del_port_mapping", payload, 15*time.Second)
		if err != nil {
			zap.L().Warn("Agent 删除端口映射失败", zap.Error(err))
		}
	}

	return db.DB.Delete(&pm).Error
}

// =====================================================================
//  防火墙规则管�?// =====================================================================

func (s *NetworkService) ListFirewallRules(instanceID string) ([]models.FirewallRule, error) {
	query := db.DB.Order("priority ASC, created_at DESC")
	if instanceID != "" {
		query = query.Where("instance_id = ?", instanceID)
	}
	var rules []models.FirewallRule
	if err := query.Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

func (s *NetworkService) AddFirewallRule(instanceID uuid.UUID, network, direction, protocol, port, sourceIP, action, description string, priority int) (*models.FirewallRule, error) {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		return nil, service.ErrInstanceNotFound
	}

	rule := models.FirewallRule{
		ID:          uuid.New(),
		InstanceID:  instanceID,
		NodeID:      instance.NodeID,
		Network:     network,
		Direction:   direction,
		Protocol:    protocol,
		Port:        port,
		SourceIP:    sourceIP,
		Action:      action,
		Description: description,
		Enabled:     true,
		Priority:    priority,
	}
	if err := db.DB.Create(&rule).Error; err != nil {
		return nil, err
	}

	if s.agentMgr != nil && s.agentMgr.IsNodeConnected(instance.NodeID) {
		payload, _ := json.Marshal(map[string]interface{}{
			"direction": direction,
			"protocol":  protocol,
			"source":    sourceIP,
			"port":      port,
			"action":    action,
		})
		_, err := s.agentMgr.SendRequest(instance.NodeID, "add_firewall_rule", payload, 15*time.Second)
		if err != nil {
			zap.L().Warn("Agent 添加防火墙规则失败?", zap.Error(err))
		}
	}

	return &rule, nil
}

func (s *NetworkService) DeleteFirewallRule(ruleID uuid.UUID) error {
	var rule models.FirewallRule
	if err := db.DB.Where("id = ?", ruleID).First(&rule).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return service.ErrFirewallRuleNotFound
		}
		return err
	}

	var instance models.Instance
	db.DB.Where("id = ?", rule.InstanceID).First(&instance)

	if s.agentMgr != nil && s.agentMgr.IsNodeConnected(instance.NodeID) {
		payload, _ := json.Marshal(map[string]interface{}{
			"direction": rule.Direction,
			"protocol":  rule.Protocol,
			"source":    rule.SourceIP,
			"port":      rule.Port,
		})
		_, err := s.agentMgr.SendRequest(instance.NodeID, "del_firewall_rule", payload, 15*time.Second)
		if err != nil {
			zap.L().Warn("Agent 删除防火墙规则失败", zap.Error(err))
		}
	}

	return db.DB.Delete(&rule).Error
}

// =====================================================================
//  工具方法
// =====================================================================

// AllocateInternalIP 从网桥网段分配内�?IP
func (s *NetworkService) AllocateInternalIP(bridgeID uuid.UUID, nodeID uuid.UUID, cidr string, gateway string, ipVersion string) (string, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", err
	}

	ip := ipNet.IP
	ip[len(ip)-1] = 2

	for i := 0; i < 254; i++ {
		if !ipNet.Contains(ip) {
			return "", fmt.Errorf("网桥内网 IP 已耗尽")
		}

		ipStr := ip.String()
		if ipStr == gateway {
			ip[len(ip)-1]++
			continue
		}

		var count int64
		field := "internal_ipv4"
		if ipVersion == "ipv6" {
			field = "internal_ipv6"
		}
		db.DB.Model(&models.Instance{}).Where("bridge_id = ? AND "+field+" = ?", bridgeID, ipStr).Count(&count)
		if count == 0 {
			return ipStr, nil
		}

		ip[len(ip)-1]++
	}

	return "", fmt.Errorf("网桥内网 IP 已耗尽")
}

// ReleaseInstanceNetworkResources 释放实例的所有网络资源（EIP、端口映射、防火墙规则）
func (s *NetworkService) ReleaseInstanceNetworkResources(instanceID uuid.UUID) error {
	// 释放端口映射
	db.DB.Where("instance_id = ?", instanceID).Delete(&models.PortMapping{})

	// 释放防火墙规则
	db.DB.Where("instance_id = ?", instanceID).Delete(&models.FirewallRule{})

	// 释放 EIP 分配
	var allocs []models.EIPAllocation
	db.DB.Where("instance_id = ? AND status = ?", instanceID, models.EIPAllocationAssigned).Find(&allocs)
	for _, alloc := range allocs {
		s.ReleaseInstanceEIP(alloc.ID)
	}

	return nil
}

// cidrOverlap 判断两个 CIDR 网段是否重叠
func cidrOverlap(a, b *net.IPNet) bool {
	if a == nil || b == nil {
		return false
	}
	return a.Contains(b.IP) || b.Contains(a.IP)
}
