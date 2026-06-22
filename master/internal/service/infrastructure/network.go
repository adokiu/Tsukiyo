package infrastructure

import (
	"encoding/json"
	"fmt"
	"net"
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

func (s *NetworkService) ListBridges(nodeID string, search string, filters map[string]string, page, perPage int) ([]models.Bridge, int64, error) {
	query := db.DB.Model(&models.Bridge{})
	if nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}

	// 搜索：匹配 name、bridge_name、ipv4_cidr、ipv6_cidr
	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("name ILIKE ? OR bridge_name ILIKE ? OR ipv4_cidr ILIKE ? OR ipv6_cidr ILIKE ?", searchPattern, searchPattern, searchPattern, searchPattern)
	}

	// 筛选
	if v, ok := filters["status"]; ok && v != "" {
		query = query.Where("status = ?", v)
	}
	if v, ok := filters["ipv4_enabled"]; ok && v != "" {
		query = query.Where("ipv4_enabled = ?", v == "true")
	}
	if v, ok := filters["ipv6_enabled"]; ok && v != "" {
		query = query.Where("ipv6_enabled = ?", v == "true")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var bridges []models.Bridge
	offset := (page - 1) * perPage
	if err := query.Order("created_at DESC").Limit(perPage).Offset(offset).Find(&bridges).Error; err != nil {
		return nil, 0, err
	}

	// 填充计算字段
	for i := range bridges {
		b := &bridges[i]

		// NAT 出口 IP 地址
		if b.NATEgressIPv4ID != nil {
			var alloc models.EIPAllocation
			if err := db.DB.Where("id = ?", *b.NATEgressIPv4ID).First(&alloc).Error; err == nil {
				b.NATEgressIPv4Addr = alloc.GetIP()
			}
		}

		// 端口总数
		b.PortTotal = b.PortRangeEnd - b.PortRangeStart + 1

		// 已用端口数（从 port_mappings 表统计）
		var portUsed int64
		db.DB.Model(&models.PortMapping{}).Where("bridge_id = ?", b.ID).Count(&portUsed)
		b.PortUsed = int(portUsed)

		// 实例数（非 deleting 状态的实例）
		var instanceCount int64
		db.DB.Model(&models.Instance{}).Where("bridge_id = ? AND status != ?", b.ID, models.InstanceStatusDeleting).Count(&instanceCount)
		b.InstanceCount = int(instanceCount)
	}

	return bridges, total, nil
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
	NodeID                uuid.UUID
	Name                  string
	IPv4Enabled           bool
	IPv4CIDR              string
	IPv4Gateway           string
	IPv6Enabled           bool
	IPv6EIPPoolID         *uuid.UUID
	IPv6PrefixLen         int
	IPv6SpecificIP        string
	DNSServers            []string
	PortRangeStart        int
	PortRangeEnd          int
	NATEgressV4PoolID     *uuid.UUID
	NATEgressV4SpecificIP string
	UserID                uint
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
		if req.IPv6EIPPoolID == nil {
			return nil, nil, fmt.Errorf("IPv6 启用时必须选择 EIP 池")
		}
		if req.IPv6PrefixLen <= 0 {
			req.IPv6PrefixLen = 64
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

	bridgeID := uuid.New()

	gatewayV4 := req.IPv4Gateway
	if gatewayV4 == "" && req.IPv4Enabled {
		ip, ipNet, _ := net.ParseCIDR(req.IPv4CIDR)
		ip = ip.Mask(ipNet.Mask)
		ip[len(ip)-1] = 1
		gatewayV4 = ip.String()
	}
	// IPv6 CIDR 和网关从 EIP 池分配子段后计算
	var ipv6CIDR string
	var gatewayV6 string
	var ipv6AllocID *uuid.UUID
	if req.IPv6Enabled {
		var pool models.EIPPool
		if err := db.DB.Where("id = ? AND status = ?", *req.IPv6EIPPoolID, models.EIPPoolStatusActive).First(&pool).Error; err != nil {
			return nil, nil, service.ErrEIPPoolNotFound
		}
		if pool.IPVersion != "ipv6" {
			return nil, nil, service.ErrEIPNotAvailable
		}
		alloc, err := s.tryAllocateFromPool(pool, req.IPv6PrefixLen, req.IPv6SpecificIP)
		if err != nil || alloc == nil {
			return nil, nil, fmt.Errorf("从 IPv6 EIP 池分配子段失败: %w", err)
		}
		ipv6AllocID = &alloc.ID
		ipv6CIDR = alloc.CIDR
		// 网关取子段第一个地址
		ip, ipNet, _ := net.ParseCIDR(ipv6CIDR)
		ip = ip.Mask(ipNet.Mask)
		if len(ip) == 16 {
			ip[15] = 1
		}
		gatewayV6 = ip.String()
		// 校验不与同节点其他 Bridge IPv6 CIDR 重叠
		_, newV6Net, _ := net.ParseCIDR(ipv6CIDR)
		for _, b := range existingBridges {
			if !b.IPv6Enabled {
				continue
			}
			_, existingNet, _ := net.ParseCIDR(b.IPv6CIDR)
			if existingNet != nil && cidrOverlap(newV6Net, existingNet) {
				now := time.Now()
				db.DB.Model(&alloc).Updates(map[string]interface{}{"status": models.EIPAllocationReleased, "released_at": &now})
				return nil, nil, service.ErrBridgeCIDROverlap
			}
		}
		// 标记为 bridge IPv6 子段用途
		alloc.Usage = models.EIPUsageBridgeIPv6Subnet
		alloc.BridgeID = &bridgeID
		db.DB.Model(&alloc).Updates(map[string]interface{}{
			"usage":     models.EIPUsageBridgeIPv6Subnet,
			"bridge_id": bridgeID,
		})
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

	bridgeName := fmt.Sprintf("br-%s", bridgeID.String()[:8])

	// 先通过 Agent 同步创建 Incus bridge，成功后才入库
	if s.agentMgr == nil || !s.agentMgr.IsNodeConnected(req.NodeID) {
		// 释放 IPv6 分配
		if ipv6AllocID != nil {
			now := time.Now()
			db.DB.Model(&models.EIPAllocation{}).Where("id = ?", *ipv6AllocID).Updates(map[string]interface{}{"status": models.EIPAllocationReleased, "released_at": &now})
		}
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
		"ipv6_cidr":    ipv6CIDR,
		"ipv6_gateway": gatewayV6,
		"dns_servers":  req.DNSServers,
	}

	_, err := s.agentMgr.SendRequest(req.NodeID, "bridge_network", taskPayload, 30*time.Second)
	if err != nil {
		// 释放 IPv6 分配
		if ipv6AllocID != nil {
			now := time.Now()
			db.DB.Model(&models.EIPAllocation{}).Where("id = ?", *ipv6AllocID).Updates(map[string]interface{}{"status": models.EIPAllocationReleased, "released_at": &now})
		}
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
		IPv6CIDR:       ipv6CIDR,
		IPv6Gateway:    gatewayV6,
		IPv6EIPPoolID:  req.IPv6EIPPoolID,
		DNSServers:     dnsJSON,
		PortRangeStart: portStart,
		PortRangeEnd:   portEnd,
		Status:         models.BridgeStatusActive,
	}

	if err := db.DB.Create(&bridge).Error; err != nil {
		zap.L().Error("创建网桥失败", zap.Error(err))
		// 释放 IPv6 分配
		if ipv6AllocID != nil {
			now := time.Now()
			db.DB.Model(&models.EIPAllocation{}).Where("id = ?", *ipv6AllocID).Updates(map[string]interface{}{"status": models.EIPAllocationReleased, "released_at": &now})
		}
		return nil, nil, err
	}

	// 分配并绑定 NAT 出口 EIP
	if req.NATEgressV4PoolID != nil && req.IPv4Enabled {
		alloc, err := s.allocateBridgeEgressFromPool(*req.NATEgressV4PoolID, bridge.ID, "ipv4", req.NATEgressV4SpecificIP)
		if err != nil {
			zap.L().Warn("分配 IPv4 NAT 出口 EIP 失败（非致命）", zap.Error(err))
		} else {
			db.DB.Model(&bridge).Update("nat_egress_ipv4_id", alloc.ID)
		}
	}
	if req.IPv6Enabled && req.IPv6EIPPoolID != nil {
		db.DB.Model(&bridge).Update("ipv6_eip_pool_id", *req.IPv6EIPPoolID)
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
func (s *NetworkService) allocateBridgeEgressFromPool(poolID uuid.UUID, bridgeID uuid.UUID, ipVersion string, specificIP string) (*models.EIPAllocation, error) {
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

	alloc, err := s.tryAllocateFromPool(pool, prefixLen, specificIP)
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

func (s *NetworkService) BindBridgeEgress(bridgeID uuid.UUID, poolID uuid.UUID, ipVersion string, specificIP string, userID uint) error {
	var bridge models.Bridge
	if err := db.DB.Where("id = ?", bridgeID).First(&bridge).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return service.ErrBridgeNotFound
		}
		return err
	}

	// 如果已绑定，先记录旧 allocation ID 用于换绑端口映射
	var oldAllocID *uuid.UUID
	if ipVersion != "ipv4" {
		return service.ErrEIPNotAvailable
	}
	oldAllocID = bridge.NATEgressIPv4ID

	// 换绑时先保存旧端口映射数据（解绑会删除记录）
	var savedPortMappings []models.PortMapping
	if oldAllocID != nil {
		db.DB.Where("egress_allocation_id = ?", *oldAllocID).Find(&savedPortMappings)
		s.UnbindBridgeEgress(bridgeID, ipVersion, userID)
	}

	// 从池中分配新 EIP
	alloc, err := s.allocateBridgeEgressFromPool(poolID, bridgeID, ipVersion, specificIP)
	if err != nil {
		return err
	}

	// 更新 bridge 的 nat_egress 字段
	field := "nat_egress_ipv4_id"
	if err := db.DB.Model(&bridge).Update(field, alloc.ID).Error; err != nil {
		return err
	}

	// 换绑：把保存的端口映射重新绑定到新 EIP，通知 Agent 重建
	if len(savedPortMappings) > 0 {
		s.rebindPortMappingsWithData(bridgeID, savedPortMappings, alloc)
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

	if ipVersion != "ipv4" {
		return nil
	}
	field := "nat_egress_ipv4_id"

	var allocID *uuid.UUID
	allocID = bridge.NATEgressIPv4ID
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

		// 清理使用该 egress allocation 的端口映射
		var portMappings []models.PortMapping
		db.DB.Where("egress_allocation_id = ?", alloc.ID).Find(&portMappings)
		for _, pm := range portMappings {
			var instance models.Instance
			db.DB.Where("id = ?", pm.InstanceID).First(&instance)
			if s.agentMgr != nil && s.agentMgr.IsNodeConnected(instance.NodeID) {
				payload := map[string]interface{}{
					"instance_id": instance.IncusName,
					"host_port":   pm.HostPort,
					"protocol":    pm.Protocol,
				}
				_, err := s.agentMgr.SendRequest(instance.NodeID, "del_port_mapping", payload, 15*time.Second)
				if err != nil {
					zap.L().Warn("Agent 删除端口映射失败", zap.Error(err))
				}
			}
		}
		db.DB.Where("egress_allocation_id = ?", alloc.ID).Delete(&models.PortMapping{})

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

// rebindPortMappingsWithData 换绑时用保存的端口映射数据重建到新 EIP
func (s *NetworkService) rebindPortMappingsWithData(bridgeID uuid.UUID, savedPortMappings []models.PortMapping, newAlloc *models.EIPAllocation) {
	for _, pm := range savedPortMappings {
		var instance models.Instance
		db.DB.Where("id = ?", pm.InstanceID).First(&instance)

		// 先通知 Agent 删除旧端口映射
		if s.agentMgr != nil && s.agentMgr.IsNodeConnected(instance.NodeID) {
			delPayload := map[string]interface{}{
				"instance_id": instance.IncusName,
				"host_port":   pm.HostPort,
				"protocol":    pm.Protocol,
			}
			_, err := s.agentMgr.SendRequest(instance.NodeID, "del_port_mapping", delPayload, 15*time.Second)
			if err != nil {
				zap.L().Warn("Agent 删除旧端口映射失败", zap.Error(err))
			}
		}

		// 重新创建端口映射记录，关联到新 EIP
		newPM := models.PortMapping{
			ID:                 uuid.New(),
			InstanceID:         pm.InstanceID,
			NodeID:             pm.NodeID,
			BridgeID:           pm.BridgeID,
			IPVersion:          pm.IPVersion,
			EgressAllocationID: newAlloc.ID,
			ContainerPort:      pm.ContainerPort,
			HostPort:           pm.HostPort,
			Protocol:           pm.Protocol,
			Description:        pm.Description,
		}
		if err := db.DB.Create(&newPM).Error; err != nil {
			zap.L().Error("重建端口映射记录失败", zap.Error(err))
			continue
		}

		// 通知 Agent 用新 EIP 重建端口映射
		if s.agentMgr != nil && s.agentMgr.IsNodeConnected(instance.NodeID) {
			internalIP := instance.InternalIPv4
			if pm.IPVersion == "ipv6" {
				internalIP = instance.InternalIPv6
			}
			addPayload := map[string]interface{}{
				"instance_id":    instance.IncusName,
				"host_port":      pm.HostPort,
				"container_port": pm.ContainerPort,
				"protocol":       pm.Protocol,
				"host_ip":        newAlloc.GetIP(),
				"internal_ip":    internalIP,
			}
			_, err := s.agentMgr.SendRequest(instance.NodeID, "add_port_mapping", addPayload, 15*time.Second)
			if err != nil {
				zap.L().Warn("Agent 重建端口映射失败", zap.Error(err))
			}
		}
	}
}
