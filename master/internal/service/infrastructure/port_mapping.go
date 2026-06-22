package infrastructure

import (
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/service"
)

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
//  防火墙规则管理
// =====================================================================

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
			zap.L().Warn("Agent 添加防火墙规则失败", zap.Error(err))
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

// AllocateInternalIP 从网桥网段分配内网 IP
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
