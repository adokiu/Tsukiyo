package infrastructure

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"strconv"
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

// =====================================================================
//  EIP 资源池管�?// =====================================================================

func (s *NetworkService) ListEIPPools(nodeID string, search string, filters map[string]string, page, perPage int) ([]models.EIPPool, int64, error) {
	query := db.DB.Model(&models.EIPPool{})
	if nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}

	// 搜索：匹配 cidr、interface、alias
	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("cidr ILIKE ? OR interface ILIKE ? OR alias ILIKE ?", searchPattern, searchPattern, searchPattern)
	}

	// 筛选
	if v, ok := filters["ip_version"]; ok && v != "" {
		query = query.Where("ip_version = ?", v)
	}
	if v, ok := filters["pool_type"]; ok && v != "" {
		query = query.Where("pool_type = ?", v)
	}
	if v, ok := filters["status"]; ok && v != "" {
		query = query.Where("status = ?", v)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var pools []models.EIPPool
	offset := (page - 1) * perPage
	if err := query.Order("created_at DESC").Limit(perPage).Offset(offset).Find(&pools).Error; err != nil {
		return nil, 0, err
	}

	// 填充使用统计
	for i := range pools {
		var usedCount int64
		db.DB.Model(&models.EIPAllocation{}).Where("pool_id = ? AND status = ?", pools[i].ID, models.EIPAllocationAssigned).Count(&usedCount)
		pools[i].UsedCount = usedCount

		_, ipNet, err := net.ParseCIDR(pools[i].CIDR)
		if err == nil {
			ones, bits := ipNet.Mask.Size()
			hostBits := bits - ones
			if hostBits >= 0 && hostBits < 63 {
				pools[i].TotalIPs = int64(1) << uint(hostBits)
				// IPv4 /32 时减去网络地址和广播地址
				if pools[i].IPVersion == "ipv4" && ones < 32 {
					pools[i].TotalIPs -= 2
				}
				// IPv6 总数过大时限制显示
				if pools[i].TotalIPs > 1000000000 {
					pools[i].TotalIPs = 1000000000
				}
			}
		}
	}

	return pools, total, nil
}

type CreateEIPPoolRequest struct {
	NodeID        uuid.UUID
	IPVersion     string
	CIDR          string
	Interface     string
	Gateway       string
	Alias         string
	PoolType      string
	NetmaskPrefix int
	UserID        uint
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

	// 校验别名 CIDR 的 IP 数量与池 CIDR 一致
	if req.Alias != "" {
		_, aliasNet, err := net.ParseCIDR(req.Alias)
		if err != nil {
			return nil, fmt.Errorf("别名 CIDR 格式无效")
		}
		aliasOnes, _ := aliasNet.Mask.Size()
		if aliasOnes != ones {
			return nil, fmt.Errorf("别名 CIDR 的 IP 数量必须与池 CIDR 一致（池 /%d，别名 /%d）", ones, aliasOnes)
		}
	}

	// 校验 CIDR 不与同节点同 IP 版本其他池重复
	var existingPools []models.EIPPool
	db.DB.Where("node_id = ? AND ip_version = ?", req.NodeID, req.IPVersion).Find(&existingPools)
	for _, p := range existingPools {
		_, existingNet, _ := net.ParseCIDR(p.CIDR)
		if existingNet == nil {
			continue
		}
		// 检查网段重叠
		if cidrOverlap(ipNet, existingNet) {
			return nil, service.ErrEIPPoolCIDROverlap
		}
	}

	poolType := models.EIPPoolTypeEIP
	if req.PoolType == "host" {
		poolType = models.EIPPoolTypeHost
	}

	alias := req.Alias

	pool := models.EIPPool{
		ID:            uuid.New(),
		NodeID:        req.NodeID,
		IPVersion:     req.IPVersion,
		CIDR:          req.CIDR,
		Interface:     req.Interface,
		Gateway:       req.Gateway,
		PrefixLen:     ones,
		NetmaskPrefix: req.NetmaskPrefix,
		Alias:         alias,
		PoolType:      poolType,
		Status:        models.EIPPoolStatusActive,
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

type UpdateEIPPoolRequest struct {
	Interface     string
	Gateway       string
	Alias         string
	NetmaskPrefix int
	PoolType      string
	Status        string
}

func (s *NetworkService) UpdateEIPPool(poolID uuid.UUID, req UpdateEIPPoolRequest) (*models.EIPPool, error) {
	var pool models.EIPPool
	if err := db.DB.Where("id = ?", poolID).First(&pool).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, service.ErrEIPPoolNotFound
		}
		return nil, err
	}

	// 校验别名 CIDR 的 IP 数量与池 CIDR 一致
	if req.Alias != "" {
		_, poolNet, err := net.ParseCIDR(pool.CIDR)
		if err != nil {
			return nil, service.ErrInvalidCIDR
		}
		poolOnes, _ := poolNet.Mask.Size()
		_, aliasNet, err := net.ParseCIDR(req.Alias)
		if err != nil {
			return nil, fmt.Errorf("别名 CIDR 格式无效")
		}
		aliasOnes, _ := aliasNet.Mask.Size()
		if aliasOnes != poolOnes {
			return nil, fmt.Errorf("别名 CIDR 的 IP 数量必须与池 CIDR 一致（池 /%d，别名 /%d）", poolOnes, aliasOnes)
		}
	}

	updates := map[string]interface{}{
		"interface":      req.Interface,
		"gateway":        req.Gateway,
		"alias":          req.Alias,
		"netmask_prefix": req.NetmaskPrefix,
	}
	if req.PoolType == "host" {
		updates["pool_type"] = models.EIPPoolTypeHost
	} else if req.PoolType == "eip" {
		updates["pool_type"] = models.EIPPoolTypeEIP
	}
	if req.Status == "active" {
		updates["status"] = models.EIPPoolStatusActive
	} else if req.Status == "disabled" {
		updates["status"] = models.EIPPoolStatusDisabled
	}

	if err := db.DB.Model(&pool).Updates(updates).Error; err != nil {
		return nil, err
	}

	// 如果别名变更，更新所有已分配记录的别名
	if req.Alias != pool.Alias {
		var allocs []models.EIPAllocation
		db.DB.Where("pool_id = ? AND status = ?", poolID, models.EIPAllocationAssigned).Find(&allocs)
		updatedPool := pool
		updatedPool.Alias = req.Alias
		for _, a := range allocs {
			newAlias := computeAliasIP(updatedPool, a.CIDR)
			db.DB.Model(&a).Update("alias", newAlias)
		}
	}

	if err := db.DB.Where("id = ?", poolID).First(&pool).Error; err != nil {
		return nil, err
	}
	return &pool, nil
}

// =====================================================================
//  EIP 分配管理
// =====================================================================

func (s *NetworkService) ListEIPAllocations(nodeID string, instanceID string, search string, filters map[string]string, page, perPage int) ([]models.EIPAllocation, int64, error) {
	query := db.DB.Model(&models.EIPAllocation{}).Where("status = ?", models.EIPAllocationAssigned)
	if nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}
	if instanceID != "" {
		query = query.Where("instance_id = ?", instanceID)
	}

	// 搜索：匹配 cidr
	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("cidr ILIKE ?", searchPattern)
	}

	// 筛选
	if v, ok := filters["ip_version"]; ok && v != "" {
		query = query.Where("ip_version = ?", v)
	}
	if v, ok := filters["usage"]; ok && v != "" {
		query = query.Where("usage = ?", v)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var allocs []models.EIPAllocation
	offset := (page - 1) * perPage
	if err := query.Order("allocated_at DESC").Limit(perPage).Offset(offset).Find(&allocs).Error; err != nil {
		return nil, 0, err
	}
	return allocs, total, nil
}

// CountAvailableEIP 统计节点可用 EIP 数量
// IPv4: 按 /32 从池 CIDR 中扫描可用数量
// IPv6: 按请求的前缀长度计算可分配的子段数
// prefixLen=0 时使用默认值：IPv4=32，IPv6=128
func (s *NetworkService) CountAvailableEIP(nodeID uuid.UUID, ipVersion string, prefixLen int) (int64, error) {
	if prefixLen <= 0 {
		if ipVersion == "ipv6" {
			prefixLen = 128
		} else {
			prefixLen = 32
		}
	}

	var pools []models.EIPPool
	if err := db.DB.Where("node_id = ? AND ip_version = ? AND status = ? AND pool_type = ?",
		nodeID, ipVersion, models.EIPPoolStatusActive, models.EIPPoolTypeEIP).Find(&pools).Error; err != nil {
		return 0, err
	}

	var total int64
	for _, pool := range pools {
		_, ipNet, err := net.ParseCIDR(pool.CIDR)
		if err != nil {
			continue
		}
		poolOnes, _ := ipNet.Mask.Size()

		// 请求的前缀长度不能短于池的前缀长度
		if prefixLen < poolOnes {
			continue
		}

		// 获取已分配的子段
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

		// 按请求的前缀长度扫描可用子段
		hostBits := ipVersionBits(ipVersion) - prefixLen
		poolHostBits := ipVersionBits(ipVersion) - poolOnes

		// 子段数量 = 池地址数 / 子段大小 = 2^(poolHostBits - hostBits)
		subnetCount := uint64(0)
		if poolHostBits >= hostBits {
			subnetCount = uint64(1) << uint(poolHostBits-hostBits)
		}

		// 子段数量过大时（超过 65536），用总数减去占用近似计算
		if subnetCount > 65536 {
			occupied := uint64(0)
			for _, a := range existingAllocs {
				_, aNet, err := net.ParseCIDR(a.CIDR)
				if err != nil {
					continue
				}
				aOnes, _ := aNet.Mask.Size()
				aHostBits := ipVersionBits(ipVersion) - aOnes
				if aHostBits <= hostBits {
					occupied += uint64(1) << uint(hostBits-aHostBits)
				}
			}
			// IPv4 排除网络地址和广播地址
			extraOccupied := uint64(0)
			if ipVersion == "ipv4" {
				if prefixLen == 32 {
					extraOccupied += 2
				} else if prefixLen < 32 {
					extraOccupied += 2
				}
			}
			// 排除网关（如果在池范围内）
			if pool.Gateway != "" {
				gwIP := net.ParseIP(pool.Gateway)
				if gwIP != nil && ipNet.Contains(gwIP) {
					extraOccupied++
				}
			}
			occupied += extraOccupied
			if occupied == 0 {
				total += int64(subnetCount)
			} else if subnetCount > occupied {
				avail := subnetCount - occupied
				if avail > uint64(1000000) {
					total += 1000000
				} else {
					total += int64(avail)
				}
			}
			continue
		}

		// 子段数量不大时精确扫描
		subnetSize := new(big.Int).Lsh(big.NewInt(1), uint(hostBits))
		poolStart, poolEnd := cidrToRange(ipNet)

		// 对齐起始地址到子段边界
		poolStart = new(big.Int).Div(poolStart, subnetSize)
		poolStart = new(big.Int).Mul(poolStart, subnetSize)

		// IPv4 时总是跳过网络地址和广播地址
		if ipVersion == "ipv4" {
			poolStart = new(big.Int).Add(poolStart, big.NewInt(1))
			poolEnd = new(big.Int).Sub(poolEnd, big.NewInt(1))
		}

		// 排除网关地址（仅在网关位于池范围内时）
		if pool.Gateway != "" {
			gwIP := net.ParseIP(pool.Gateway)
			if gwIP != nil && ipNet.Contains(gwIP) {
				gwVal := ipToBigInt(gwIP)
				usedRanges = append(usedRanges, ipRange{start: gwVal, end: gwVal})
			}
		}

		for cur := new(big.Int).Set(poolStart); new(big.Int).Sub(new(big.Int).Add(cur, subnetSize), big.NewInt(1)).Cmp(poolEnd) <= 0; cur = new(big.Int).Add(cur, subnetSize) {
			overlap := false
			curEnd := new(big.Int).Sub(new(big.Int).Add(cur, subnetSize), big.NewInt(1))
			for _, r := range usedRanges {
				if cur.Cmp(r.start) >= 0 && cur.Cmp(r.end) <= 0 {
					overlap = true
					break
				}
				if curEnd.Cmp(r.start) >= 0 && curEnd.Cmp(r.end) <= 0 {
					overlap = true
					break
				}
				if r.start.Cmp(cur) >= 0 && r.end.Cmp(curEnd) <= 0 {
					overlap = true
					break
				}
			}
			if !overlap {
				total++
			}
		}
	}

	return total, nil
}

// ipVersionBits 返回 IP 版本的位长度
func ipVersionBits(ipVersion string) int {
	if ipVersion == "ipv6" {
		return 128
	}
	return 32
}

func (s *NetworkService) AllocateEIP(nodeID uuid.UUID, ipVersion string, prefixLen int, specificIP string) (*models.EIPAllocation, error) {
	return s.AllocateEIPFromPool(nodeID, uuid.Nil, ipVersion, prefixLen, specificIP)
}

func (s *NetworkService) AllocateEIPFromPool(nodeID uuid.UUID, poolID uuid.UUID, ipVersion string, prefixLen int, specificIP string) (*models.EIPAllocation, error) {
	var pools []models.EIPPool
	query := db.DB.Where("node_id = ? AND ip_version = ? AND status = ? AND pool_type = ?",
		nodeID, ipVersion, models.EIPPoolStatusActive, models.EIPPoolTypeEIP)
	if poolID != uuid.Nil {
		query = query.Where("id = ?", poolID)
	}
	if err := query.Find(&pools).Error; err != nil {
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

// AllocateIPv6FromBridge 从 bridge 的 IPv6 CIDR 中分配子段给实例
// 逻辑与 tryAllocateFromPool 一致，但范围是 bridge 的 IPv6CIDR 而非 EIP 池
func (s *NetworkService) AllocateIPv6FromBridge(bridgeID uuid.UUID, prefixLen int, specificIP string) (*models.EIPAllocation, error) {
	var bridge models.Bridge
	if err := db.DB.Where("id = ?", bridgeID).First(&bridge).Error; err != nil {
		return nil, service.ErrBridgeNotFound
	}
	if !bridge.IPv6Enabled || bridge.IPv6CIDR == "" {
		return nil, service.ErrEIPNotAvailable
	}

	_, ipNet, err := net.ParseCIDR(bridge.IPv6CIDR)
	if err != nil {
		return nil, service.ErrInvalidCIDR
	}

	bridgeOnes, bridgeBits := ipNet.Mask.Size()
	if prefixLen == 0 {
		prefixLen = 128
	}
	if prefixLen < bridgeOnes {
		return nil, fmt.Errorf("请求的前缀长度小于 bridge IPv6 CIDR 前缀长度")
	}

	// 查询该 bridge 下所有已分配的 IPv6 实例 EIP
	var existingAllocs []models.EIPAllocation
	db.DB.Where("bridge_id = ? AND ip_version = ? AND status = ? AND usage = ?", bridgeID, "ipv6", models.EIPAllocationAssigned, models.EIPUsageInstanceEIP).Find(&existingAllocs)

	usedRanges := make([]ipRange, 0, len(existingAllocs))
	for _, a := range existingAllocs {
		_, aNet, err := net.ParseCIDR(a.CIDR)
		if err != nil {
			continue
		}
		start, end := cidrToRange(aNet)
		usedRanges = append(usedRanges, ipRange{start: start, end: end})
	}

	// 排除网关地址
	if bridge.IPv6Gateway != "" {
		gwIP := net.ParseIP(bridge.IPv6Gateway)
		if gwIP != nil && ipNet.Contains(gwIP) {
			gwVal := ipToBigInt(gwIP)
			usedRanges = append(usedRanges, ipRange{start: gwVal, end: gwVal})
		}
	}

	// 查询 bridge 关联的 EIP 池（用于记录 PoolID）
	var poolID uuid.UUID
	if bridge.IPv6EIPPoolID != nil {
		poolID = *bridge.IPv6EIPPoolID
	}
	var pool models.EIPPool
	if poolID != uuid.Nil {
		db.DB.Where("id = ?", poolID).First(&pool)
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
			if targetStart.Cmp(r.start) >= 0 && targetStart.Cmp(r.end) <= 0 {
				return nil, service.ErrEIPAlreadyAssigned
			}
			if targetEnd.Cmp(r.start) >= 0 && targetEnd.Cmp(r.end) <= 0 {
				return nil, service.ErrEIPAlreadyAssigned
			}
		}
		return s.createBridgeIPv6Allocation(pool, bridge, targetCIDR, prefixLen), nil
	}

	subnetSize := new(big.Int).Lsh(big.NewInt(1), uint(bridgeBits-prefixLen))
	poolStart, poolEnd := cidrToRange(ipNet)

	for cur := new(big.Int).Set(poolStart); new(big.Int).Sub(new(big.Int).Add(cur, subnetSize), big.NewInt(1)).Cmp(poolEnd) <= 0; cur = new(big.Int).Add(cur, subnetSize) {
		overlap := false
		curEnd := new(big.Int).Sub(new(big.Int).Add(cur, subnetSize), big.NewInt(1))
		for _, r := range usedRanges {
			if cur.Cmp(r.start) >= 0 && cur.Cmp(r.end) <= 0 {
				overlap = true
				break
			}
			if curEnd.Cmp(r.start) >= 0 && curEnd.Cmp(r.end) <= 0 {
				overlap = true
				break
			}
		}
		if !overlap {
			ip := rangeToIP(cur, len(ipNet.IP))
			cidr := fmt.Sprintf("%s/%d", ip.String(), prefixLen)
			return s.createBridgeIPv6Allocation(pool, bridge, cidr, prefixLen), nil
		}
	}

	return nil, service.ErrNoAvailableEIP
}

// createBridgeIPv6Allocation 创建从 bridge IPv6 子段分配的 EIPAllocation 记录
func (s *NetworkService) createBridgeIPv6Allocation(pool models.EIPPool, bridge models.Bridge, cidr string, prefixLen int) *models.EIPAllocation {
	alloc := models.EIPAllocation{
		ID:        uuid.New(),
		PoolID:    pool.ID,
		NodeID:    bridge.NodeID,
		CIDR:      cidr,
		PrefixLen: prefixLen,
		IPVersion: "ipv6",
		Usage:     models.EIPUsageInstanceEIP,
		BridgeID:  &bridge.ID,
		Status:    models.EIPAllocationAssigned,
	}
	db.DB.Create(&alloc)
	return &alloc
}

// ListAvailableIPv6FromBridge 列出 bridge IPv6 CIDR 中可用的子段（最多 maxCount 个）
func (s *NetworkService) ListAvailableIPv6FromBridge(bridgeID uuid.UUID, prefixLen int, maxCount int) ([]string, error) {
	var bridge models.Bridge
	if err := db.DB.Where("id = ?", bridgeID).First(&bridge).Error; err != nil {
		return nil, service.ErrBridgeNotFound
	}
	if !bridge.IPv6Enabled || bridge.IPv6CIDR == "" {
		return nil, service.ErrEIPNotAvailable
	}

	_, ipNet, err := net.ParseCIDR(bridge.IPv6CIDR)
	if err != nil {
		return nil, service.ErrInvalidCIDR
	}

	bridgeOnes, bridgeBits := ipNet.Mask.Size()
	if prefixLen <= 0 {
		prefixLen = 128
	}
	if prefixLen < bridgeOnes {
		return nil, fmt.Errorf("请求的前缀长度小于 bridge IPv6 CIDR 前缀长度")
	}

	var existingAllocs []models.EIPAllocation
	db.DB.Where("bridge_id = ? AND ip_version = ? AND status = ? AND usage = ?", bridgeID, "ipv6", models.EIPAllocationAssigned, models.EIPUsageInstanceEIP).Find(&existingAllocs)

	usedRanges := make([]ipRange, 0, len(existingAllocs))
	for _, a := range existingAllocs {
		_, aNet, err := net.ParseCIDR(a.CIDR)
		if err != nil {
			continue
		}
		start, end := cidrToRange(aNet)
		usedRanges = append(usedRanges, ipRange{start: start, end: end})
	}

	// 排除网关地址
	excludeRanges := make([]ipRange, 0)
	if bridge.IPv6Gateway != "" {
		gwIP := net.ParseIP(bridge.IPv6Gateway)
		if gwIP != nil && ipNet.Contains(gwIP) {
			gwVal := ipToBigInt(gwIP)
			excludeRanges = append(excludeRanges, ipRange{start: gwVal, end: gwVal})
		}
	}

	allExclude := append(usedRanges, excludeRanges...)

	subnetSize := new(big.Int).Lsh(big.NewInt(1), uint(bridgeBits-prefixLen))
	poolStart, poolEnd := cidrToRange(ipNet)

	// 对齐起始地址到子段边界
	poolStart = new(big.Int).Div(poolStart, subnetSize)
	poolStart = new(big.Int).Mul(poolStart, subnetSize)

	results := make([]string, 0, maxCount)
	for cur := new(big.Int).Set(poolStart); new(big.Int).Sub(new(big.Int).Add(cur, subnetSize), big.NewInt(1)).Cmp(poolEnd) <= 0 && len(results) < maxCount; cur = new(big.Int).Add(cur, subnetSize) {
		overlap := false
		curEnd := new(big.Int).Sub(new(big.Int).Add(cur, subnetSize), big.NewInt(1))
		for _, r := range allExclude {
			if cur.Cmp(r.start) >= 0 && cur.Cmp(r.end) <= 0 {
				overlap = true
				break
			}
			if curEnd.Cmp(r.start) >= 0 && curEnd.Cmp(r.end) <= 0 {
				overlap = true
				break
			}
			if r.start.Cmp(cur) >= 0 && r.end.Cmp(curEnd) <= 0 {
				overlap = true
				break
			}
		}
		if !overlap {
			ip := rangeToIP(cur, len(ipNet.IP))
			cidr := fmt.Sprintf("%s/%d", ip.String(), prefixLen)
			results = append(results, cidr)
		}
	}

	return results, nil
}

// ListAvailableEIPsFromPool 列出指定池中可用的 EIP 地址（最多 maxCount 个）
// 排除已分配的、网络地址、广播地址、网关地址
func (s *NetworkService) ListAvailableEIPsFromPool(poolID uuid.UUID, prefixLen int, maxCount int) ([]string, error) {
	var pool models.EIPPool
	if err := db.DB.Where("id = ?", poolID).First(&pool).Error; err != nil {
		return nil, service.ErrEIPPoolNotFound
	}

	_, ipNet, err := net.ParseCIDR(pool.CIDR)
	if err != nil {
		return nil, service.ErrInvalidCIDR
	}

	poolOnes, poolBits := ipNet.Mask.Size()
	if prefixLen <= 0 {
		if pool.IPVersion == "ipv4" {
			prefixLen = 32
		} else {
			prefixLen = 128
		}
	}
	if prefixLen < poolOnes {
		return nil, fmt.Errorf("请求的前缀长度小于资源池前缀长度")
	}

	// 获取已分配的子段
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

	// 构建排除范围：网络地址、广播地址、网关
	excludeRanges := make([]ipRange, 0)
	poolStart, poolEnd := cidrToRange(ipNet)
	ipLen := len(ipNet.IP)

	if pool.IPVersion == "ipv4" {
		// 网络地址
		excludeRanges = append(excludeRanges, ipRange{start: poolStart, end: poolStart})
		// 广播地址
		excludeRanges = append(excludeRanges, ipRange{start: poolEnd, end: poolEnd})
		// 网关地址（仅在网关位于池范围内时）
		if pool.Gateway != "" {
			gwIP := net.ParseIP(pool.Gateway)
			if gwIP != nil && ipNet.Contains(gwIP) {
				gwVal := ipToBigInt(gwIP)
				excludeRanges = append(excludeRanges, ipRange{start: gwVal, end: gwVal})
			}
		}
	}

	// 合并 usedRanges 和 excludeRanges
	allExclude := append(usedRanges, excludeRanges...)

	subnetSize := new(big.Int).Lsh(big.NewInt(1), uint(poolBits-prefixLen))

	// 对齐起始地址到子段边界
	poolStart = new(big.Int).Div(poolStart, subnetSize)
	poolStart = new(big.Int).Mul(poolStart, subnetSize)

	// IPv4 /32 时跳过网络地址和广播地址（已在 excludeRanges 中处理）
	if pool.IPVersion == "ipv4" && prefixLen == 32 {
		if poolStart.Sign() == 0 {
			poolStart = new(big.Int).Add(poolStart, big.NewInt(1))
		}
		poolEnd = new(big.Int).Sub(poolEnd, big.NewInt(1))
	}

	results := make([]string, 0, maxCount)
	for cur := new(big.Int).Set(poolStart); new(big.Int).Sub(new(big.Int).Add(cur, subnetSize), big.NewInt(1)).Cmp(poolEnd) <= 0 && len(results) < maxCount; cur = new(big.Int).Add(cur, subnetSize) {
		overlap := false
		curEnd := new(big.Int).Sub(new(big.Int).Add(cur, subnetSize), big.NewInt(1))
		for _, r := range allExclude {
			if cur.Cmp(r.start) >= 0 && cur.Cmp(r.end) <= 0 {
				overlap = true
				break
			}
			if curEnd.Cmp(r.start) >= 0 && curEnd.Cmp(r.end) <= 0 {
				overlap = true
				break
			}
			if r.start.Cmp(cur) >= 0 && r.end.Cmp(curEnd) <= 0 {
				overlap = true
				break
			}
		}
		if !overlap {
			ip := rangeToIP(cur, ipLen)
			cidr := fmt.Sprintf("%s/%d", ip.String(), prefixLen)
			results = append(results, cidr)
		}
	}

	return results, nil
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
			if targetStart.Cmp(r.start) >= 0 && targetStart.Cmp(r.end) <= 0 {
				return nil, service.ErrEIPAlreadyAssigned
			}
			if targetEnd.Cmp(r.start) >= 0 && targetEnd.Cmp(r.end) <= 0 {
				return nil, service.ErrEIPAlreadyAssigned
			}
		}
		return s.createEIPAllocation(pool, targetCIDR, prefixLen, ""), nil
	}

	subnetSize := new(big.Int).Lsh(big.NewInt(1), uint(poolBits-prefixLen))
	poolStart, poolEnd := cidrToRange(ipNet)

	// IPv4 时总是跳过网络地址和广播地址
	if pool.IPVersion == "ipv4" {
		poolStart = new(big.Int).Add(poolStart, big.NewInt(1))
		poolEnd = new(big.Int).Sub(poolEnd, big.NewInt(1))
	}

	// 排除网关地址（仅在网关位于池范围内时）
	if pool.Gateway != "" {
		gwIP := net.ParseIP(pool.Gateway)
		if gwIP != nil && ipNet.Contains(gwIP) {
			gwVal := ipToBigInt(gwIP)
			usedRanges = append(usedRanges, ipRange{start: gwVal, end: gwVal})
		}
	}

	for cur := new(big.Int).Set(poolStart); new(big.Int).Sub(new(big.Int).Add(cur, subnetSize), big.NewInt(1)).Cmp(poolEnd) <= 0; cur = new(big.Int).Add(cur, subnetSize) {
		overlap := false
		curEnd := new(big.Int).Sub(new(big.Int).Add(cur, subnetSize), big.NewInt(1))
		for _, r := range usedRanges {
			if cur.Cmp(r.start) >= 0 && cur.Cmp(r.end) <= 0 {
				overlap = true
				break
			}
			if curEnd.Cmp(r.start) >= 0 && curEnd.Cmp(r.end) <= 0 {
				overlap = true
				break
			}
		}
		if !overlap {
			ip := rangeToIP(cur, len(ipNet.IP))
			cidr := fmt.Sprintf("%s/%d", ip.String(), prefixLen)
			return s.createEIPAllocation(pool, cidr, prefixLen, ""), nil
		}
	}

	return nil, service.ErrNoAvailableEIP
}

type ipRange struct {
	start *big.Int
	end   *big.Int
}

func cidrToRange(ipNet *net.IPNet) (*big.Int, *big.Int) {
	start := new(big.Int).SetBytes(ipNet.IP)
	ones, bits := ipNet.Mask.Size()
	size := new(big.Int).Lsh(big.NewInt(1), uint(bits-ones))
	end := new(big.Int).Add(start, new(big.Int).Sub(size, big.NewInt(1)))
	return start, end
}

func rangeToIP(val *big.Int, size int) net.IP {
	b := val.Bytes()
	if len(b) < size {
		pad := make([]byte, size-len(b))
		b = append(pad, b...)
	}
	return net.IP(b)
}

func ipToBigInt(ip net.IP) *big.Int {
	ip = ip.To16()
	if ip == nil {
		return big.NewInt(0)
	}
	return new(big.Int).SetBytes(ip)
}

// computeAliasIP 根据池的 Alias CIDR 和分配的 CIDR 计算对应的公网别名 IP
// 例如池 CIDR 是 172.19.10.10/30，Alias 是 125.25.1.10/30
// 分配了 172.19.10.11/32，则别名为 125.25.1.11/32
func computeAliasIP(pool models.EIPPool, allocCIDR string) string {
	if pool.Alias == "" {
		return ""
	}
	_, poolNet, err := net.ParseCIDR(pool.CIDR)
	if err != nil {
		return ""
	}
	_, aliasNet, err := net.ParseCIDR(pool.Alias)
	if err != nil {
		return ""
	}
	// 解析分配的 CIDR
	allocIPStr := allocCIDR
	allocPrefixLen := 32
	if idx := strings.Index(allocCIDR, "/"); idx > 0 {
		allocIPStr = allocCIDR[:idx]
		allocPrefixLen, _ = strconv.Atoi(allocCIDR[idx+1:])
	}
	allocIP := net.ParseIP(allocIPStr)
	if allocIP == nil {
		return ""
	}
	// 计算分配 IP 相对池起始的偏移量
	poolStart := ipToBigInt(poolNet.IP)
	allocStart := ipToBigInt(allocIP)
	offset := new(big.Int).Sub(allocStart, poolStart)
	// 别名起始 + 偏移量
	aliasStart := ipToBigInt(aliasNet.IP)
	aliasVal := new(big.Int).Add(aliasStart, offset)
	aliasIP := rangeToIP(aliasVal, len(aliasNet.IP))
	return fmt.Sprintf("%s/%d", aliasIP.String(), allocPrefixLen)
}

func (s *NetworkService) createEIPAllocation(pool models.EIPPool, cidr string, prefixLen int, mappedInternalIP string) *models.EIPAllocation {
	// 根据池的 Alias CIDR 计算该 IP 对应的公网别名
	aliasIP := computeAliasIP(pool, cidr)

	alloc := models.EIPAllocation{
		ID:               uuid.New(),
		PoolID:           pool.ID,
		NodeID:           pool.NodeID,
		CIDR:             cidr,
		PrefixLen:        prefixLen,
		IPVersion:        pool.IPVersion,
		Alias:            aliasIP,
		Usage:            models.EIPUsageInstanceEIP,
		Status:           models.EIPAllocationAssigned,
		MappedInternalIP: mappedInternalIP,
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

func (s *NetworkService) AssignEIPToInstanceDBOnly(allocationID uuid.UUID, instanceID uuid.UUID) error {
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

	return nil
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
		payload := map[string]interface{}{
			"instance_name":      instance.IncusName,
			"instance_ip":        internalIP,
			"eip_cidr":           alloc.CIDR,
			"interface":          pool.Interface,
			"ip_version":         alloc.IPVersion,
			"bridge_name":        bridge.BridgeName,
			"mapped_internal_ip": alloc.MappedInternalIP,
			"ipv4_cidr":          bridge.IPv4CIDR,
			"ipv6_cidr":          bridge.IPv6CIDR,
			"ipv4_gateway":       bridge.IPv4Gateway,
			"ipv6_gateway":       bridge.IPv6Gateway,
		}
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
			payload := map[string]interface{}{
				"instance_name":      instance.IncusName,
				"instance_ip":        internalIP,
				"eip_cidr":           alloc.CIDR,
				"interface":          pool.Interface,
				"ip_version":         alloc.IPVersion,
				"bridge_name":        bridge.BridgeName,
				"mapped_internal_ip": alloc.MappedInternalIP,
			}
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

	if ipVersion != "ipv4" {
		return nil, service.ErrNoBridgeEgressIP
	}
	egressAllocID := bridge.NATEgressIPv4ID
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

	// count=0 表示不自动分配任何端口（包括 SSH）
	if count <= 0 {
		return nil, nil
	}

	// count=1 时只分配 SSH，count>1 时分配 SSH + 额外端口（总数不超过 count）
	sshPM, err := allocatePort(22, "tcp", "SSH")
	if err != nil {
		return nil, err
	}
	mappings = append(mappings, *sshPM)

	for _, port := range extraPorts {
		if port == 22 {
			continue
		}
		if len(mappings) >= count {
			break
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

func (s *NetworkService) AddPortMapping(instanceID uuid.UUID, containerPort int, hostPort int, protocol string, ipVersion string, description string) (*models.PortMapping, error) {
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

	if ipVersion != "ipv4" {
		return nil, service.ErrNoBridgeEgressIP
	}
	egressAllocID := bridge.NATEgressIPv4ID
	if egressAllocID == nil {
		return nil, service.ErrNoBridgeEgressIP
	}

	if hostPort > 0 {
		// 用户指定了外部端口，检查是否被占用
		var count int64
		db.DB.Model(&models.PortMapping{}).Where("bridge_id = ? AND host_port = ? AND protocol = ?", bridge.ID, hostPort, protocol).Count(&count)
		if count > 0 {
			return nil, fmt.Errorf("端口 %d 已被占用", hostPort)
		}
	} else {
		// 自动分配端口
		var err error
		hostPort, err = s.findAvailablePort(bridge.ID, bridge.PortRangeStart, bridge.PortRangeEnd, protocol)
		if err != nil {
			return nil, err
		}
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
		payload := map[string]interface{}{
			"instance_id":    instance.IncusName,
			"host_port":      hostPort,
			"container_port": containerPort,
			"protocol":       protocol,
			"host_ip":        egressAlloc.GetIP(),
			"internal_ip":    internalIP,
		}
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
		payload := map[string]interface{}{
			"direction": direction,
			"protocol":  protocol,
			"source":    sourceIP,
			"port":      port,
			"action":    action,
		}
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
		payload := map[string]interface{}{
			"direction": rule.Direction,
			"protocol":  rule.Protocol,
			"source":    rule.SourceIP,
			"port":      rule.Port,
		}
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
			// 检查 eip_allocations 表中 mapped_internal_ip 是否已占用
			var eipCount int64
			db.DB.Model(&models.EIPAllocation{}).Where("mapped_internal_ip = ? AND status = ?", ipStr, models.EIPAllocationAssigned).Count(&eipCount)
			if eipCount == 0 {
				return ipStr, nil
			}
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

	// 释放 EIP 分配（异步发送 release_eip 到 Agent，不阻塞删除流程）
	var allocs []models.EIPAllocation
	db.DB.Where("instance_id = ? AND status = ?", instanceID, models.EIPAllocationAssigned).Find(&allocs)
	for _, alloc := range allocs {
		allocCopy := alloc
		go func() {
			if err := s.ReleaseInstanceEIP(allocCopy.ID); err != nil {
				zap.L().Warn("异步释放 EIP 失败", zap.String("alloc_id", allocCopy.ID.String()), zap.Error(err))
			}
		}()
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
