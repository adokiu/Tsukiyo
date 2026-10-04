package infrastructure

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// dynamicPoolNICInfo 心跳上报的网卡信息（仅取需要的字段）
type dynamicPoolNICInfo struct {
	Name string `json:"name"`
	IPv4 []struct {
		Address   string `json:"address"`
		PrefixLen int    `json:"prefix_len"`
		Gateway   string `json:"gateway,omitempty"`
	} `json:"ipv4"`
	IPv6 []struct {
		Address   string `json:"address"`
		PrefixLen int    `json:"prefix_len"`
		Gateway   string `json:"gateway,omitempty"`
	} `json:"ipv6"`
}

// CheckDynamicBindingPools 检测动态绑定池的 IP 是否变化，变化则自动换绑
// 由心跳处理调用
func (s *NetworkService) CheckDynamicBindingPools(nodeID uuid.UUID, networkInterfaces json.RawMessage) {
	if len(networkInterfaces) == 0 {
		return
	}

	var nics []dynamicPoolNICInfo
	if err := json.Unmarshal(networkInterfaces, &nics); err != nil {
		zap.L().Warn("[DynamicBinding] 解析网卡信息失败", zap.Error(err))
		return
	}

	// 查询该节点所有开启动态绑定的 active 池
	var pools []models.EIPPool
	if err := db.DB.Where("node_id = ? AND dynamic_binding = true AND status = ?",
		nodeID, models.EIPPoolStatusActive).Find(&pools).Error; err != nil {
		zap.L().Error("[DynamicBinding] 查询动态绑定池失败", zap.Error(err))
		return
	}

	for i := range pools {
		pool := &pools[i]
		poolIP := extractIP(pool.CIDR)

		// 在网卡上查找该 IP
		found := false
		for _, nic := range nics {
			if nic.Name != pool.Interface {
				continue
			}
			ipList := nic.IPv4
			if pool.IPVersion == "ipv6" {
				ipList = nic.IPv6
			}
			for _, ip := range ipList {
				if ip.Address == poolIP {
					found = true
					break
				}
			}
		}

		if found {
			continue
		}

		// IP 已不在网卡上，查找新 IP
		newIP, newPrefixLen, newGateway := findNewIPOnInterface(nics, pool.Interface, pool.IPVersion, poolIP)
		if newIP == "" {
			zap.L().Warn("[DynamicBinding] 池绑定的 IP 已失效且网卡上无新 IP 可用",
				zap.String("pool_id", pool.ID.String()),
				zap.String("old_ip", poolIP),
				zap.String("interface", pool.Interface),
				zap.String("ip_version", pool.IPVersion))
			continue
		}

		zap.L().Info("[DynamicBinding] 检测到 IP 变化，开始自动换绑",
			zap.String("pool_id", pool.ID.String()),
			zap.String("old_ip", poolIP),
			zap.String("new_ip", newIP),
			zap.String("interface", pool.Interface),
			zap.String("ip_version", pool.IPVersion))

		if err := s.rebindDynamicPool(pool, newIP, newPrefixLen, newGateway); err != nil {
			zap.L().Error("[DynamicBinding] 自动换绑失败",
				zap.String("pool_id", pool.ID.String()),
				zap.Error(err))
		}
	}
}

// rebindDynamicPool 执行动态池 IP 换绑：更新池 CIDR/Gateway，重算所有分配，下发到 Agent
func (s *NetworkService) rebindDynamicPool(pool *models.EIPPool, newIP string, newPrefixLen int, newGateway string) error {
	oldCIDR := pool.CIDR
	_, oldNet, _ := net.ParseCIDR(oldCIDR)
	oldOnes := 0
	if oldNet != nil {
		oldOnes, _ = oldNet.Mask.Size()
	}

	// 构造新 CIDR，保持原前缀长度
	newCIDR := fmt.Sprintf("%s/%d", newIP, oldOnes)
	if pool.IPVersion == "ipv6" && newPrefixLen > 0 {
		// IPv6 动态池：使用网卡上报的前缀长度
		newCIDR = fmt.Sprintf("%s/%d", newIP, newPrefixLen)
	}

	// 更新池的 CIDR 和 Gateway
	updates := map[string]interface{}{
		"cidr":    newCIDR,
		"gateway": newGateway,
	}
	// 更新 PrefixLen
	_, newNet, err := net.ParseCIDR(newCIDR)
	if err != nil {
		return fmt.Errorf("新 CIDR 格式无效: %w", err)
	}
	newOnes, _ := newNet.Mask.Size()
	updates["prefix_len"] = newOnes

	// 如果有 Alias，按偏移重新计算 Alias CIDR
	if pool.Alias != "" {
		_, aliasNet, err := net.ParseCIDR(pool.Alias)
		if err == nil {
			aliasOnes, _ := aliasNet.Mask.Size()
			// 别名起始地址 = 旧别名起始 + (新池起始 - 旧池起始)
			// 但动态池 IP 变化时，别名也应该跟着变
			// 动态池场景：别名就是新 IP 本身（因为动态 IP 通常只有一个地址）
			// 保持别名前缀长度不变，用新 IP 作为别名起始
			newAliasCIDR := fmt.Sprintf("%s/%d", newIP, aliasOnes)
			updates["alias"] = newAliasCIDR
		}
	}

	if err := db.DB.Model(pool).Updates(updates).Error; err != nil {
		return fmt.Errorf("更新池 CIDR 失败: %w", err)
	}

	// 重新读取更新后的池
	var updatedPool models.EIPPool
	if err := db.DB.Where("id = ?", pool.ID).First(&updatedPool).Error; err != nil {
		return fmt.Errorf("重新读取池失败: %w", err)
	}

	// 先处理 bridge IPv6 CIDR 更新（如果池是 IPv6 且被 bridge 引用）
	// 必须在通知实例 EIP 变更之前执行，确保 EIP 实例拿到新的 bridge gateway
	if pool.IPVersion == "ipv6" {
		s.updateBridgeIPv6CIDR(pool, oldCIDR, newCIDR)
	}

	// 查询该池所有已分配的记录
	var allocs []models.EIPAllocation
	if err := db.DB.Where("pool_id = ? AND status = ?", pool.ID, models.EIPAllocationAssigned).Find(&allocs).Error; err != nil {
		return fmt.Errorf("查询池分配记录失败: %w", err)
	}

	// 按偏移量重新计算每个分配的 CIDR
	for i := range allocs {
		alloc := &allocs[i]
		// bridge_ipv6_subnet 类型已在 updateBridgeIPv6CIDR 中更新，跳过避免二次偏移
		if alloc.Usage == models.EIPUsageBridgeIPv6Subnet {
			continue
		}
		newAllocCIDR := recomputeAllocCIDR(oldCIDR, newCIDR, alloc.CIDR)
		if newAllocCIDR == "" {
			zap.L().Warn("[DynamicBinding] 无法重算分配 CIDR，跳过",
				zap.String("alloc_id", alloc.ID.String()),
				zap.String("old_alloc_cidr", alloc.CIDR))
			continue
		}

		// 更新分配记录的 CIDR
		dbUpdates := map[string]interface{}{
			"cidr": newAllocCIDR,
		}

		// 重新计算 Alias
		if updatedPool.Alias != "" {
			newAlias := computeAliasIP(updatedPool, newAllocCIDR)
			if newAlias != "" {
				dbUpdates["alias"] = newAlias
			}
		}

		// 更新 PrefixLen
		if _, newAllocNet, err := net.ParseCIDR(newAllocCIDR); err == nil {
			allocOnes, _ := newAllocNet.Mask.Size()
			dbUpdates["prefix_len"] = allocOnes
		}

		if err := db.DB.Model(alloc).Updates(dbUpdates).Error; err != nil {
			zap.L().Error("[DynamicBinding] 更新分配记录 CIDR 失败",
				zap.String("alloc_id", alloc.ID.String()),
				zap.Error(err))
			continue
		}

		// 根据 usage 类型下发到 Agent
		s.notifyAgentAllocChange(&updatedPool, alloc, newAllocCIDR)
	}

	// 通知 Agent 在网卡上添加新 IP
	if s.agentMgr != nil && s.agentMgr.IsNodeConnected(pool.NodeID) && pool.Interface != "" {
		_, err := s.agentMgr.SendRequest(pool.NodeID, "add_ip", map[string]interface{}{
			"cidr":      newCIDR,
			"interface": pool.Interface,
		}, 15*time.Second)
		if err != nil {
			zap.L().Warn("[DynamicBinding] Agent 添加新 IP 到网卡失败",
				zap.String("cidr", newCIDR),
				zap.String("interface", pool.Interface),
				zap.Error(err))
		}
	}

	zap.L().Info("[DynamicBinding] 自动换绑完成",
		zap.String("pool_id", pool.ID.String()),
		zap.String("old_cidr", oldCIDR),
		zap.String("new_cidr", newCIDR),
		zap.Int("alloc_count", len(allocs)))

	return nil
}

// notifyAgentAllocChange 根据分配类型通知 Agent 更新
func (s *NetworkService) notifyAgentAllocChange(pool *models.EIPPool, alloc *models.EIPAllocation, newAllocCIDR string) {
	if s.agentMgr == nil || !s.agentMgr.IsNodeConnected(pool.NodeID) {
		return
	}

	switch alloc.Usage {
	case models.EIPUsageBridgeNATEgress:
		// 网桥 NAT 出口 IP 变更
		if alloc.BridgeID != nil {
			var bridge models.Bridge
			if db.DB.Where("id = ?", *alloc.BridgeID).First(&bridge).Error != nil {
				return
			}
			// 先解绑旧出口 IP
			s.agentMgr.SendRequest(pool.NodeID, "unbind_bridge_egress", map[string]interface{}{
				"bridge_name": bridge.BridgeName,
				"egress_cidr": alloc.CIDR,
				"interface":   pool.Interface,
				"ip_version":  alloc.IPVersion,
			}, 15*time.Second)

			// 绑定新出口 IP
			s.agentMgr.SendRequest(pool.NodeID, "bind_bridge_egress", map[string]interface{}{
				"bridge_name": bridge.BridgeName,
				"egress_cidr": newAllocCIDR,
				"interface":   pool.Interface,
				"ip_version":  alloc.IPVersion,
			}, 15*time.Second)

			// 换绑端口映射
			s.rebindPortMappingsForDynamicBridge(bridge.ID, alloc.ID, newAllocCIDR)
		}

	case models.EIPUsageInstanceEIP:
		// 实例 EIP 变更：先释放旧 EIP，再分配新 EIP
		if alloc.InstanceID != nil {
			var instance models.Instance
			if db.DB.Where("id = ?", *alloc.InstanceID).First(&instance).Error != nil {
				return
			}
			var bridge models.Bridge
			if instance.BridgeID != nil {
				db.DB.Where("id = ?", *instance.BridgeID).First(&bridge)
			}

			internalIP := instance.InternalIPv4
			if alloc.IPVersion == "ipv6" {
				internalIP = instance.InternalIPv6
			}

			// 释放旧 EIP（Agent 清理旧规则）
			releasePayload := map[string]interface{}{
				"instance_name":      instance.IncusName,
				"instance_ip":        internalIP,
				"eip_cidr":           alloc.CIDR,
				"interface":          pool.Interface,
				"ip_version":         alloc.IPVersion,
				"bridge_name":        bridge.BridgeName,
				"mapped_internal_ip": alloc.MappedInternalIP,
			}
			s.agentMgr.SendRequest(pool.NodeID, "release_eip", releasePayload, 30*time.Second)

			// 分配新 EIP（Agent 建立新规则）
			assignPayload := map[string]interface{}{
				"instance_name":      instance.IncusName,
				"instance_ip":        internalIP,
				"eip_cidr":           newAllocCIDR,
				"interface":          pool.Interface,
				"ip_version":         alloc.IPVersion,
				"bridge_name":        bridge.BridgeName,
				"mapped_internal_ip": alloc.MappedInternalIP,
				"ipv4_cidr":          bridge.IPv4CIDR,
				"ipv6_cidr":          bridge.IPv6CIDR,
				"ipv4_gateway":       bridge.IPv4Gateway,
				"ipv6_gateway":       bridge.IPv6Gateway,
				"eip_gateway":        pool.Gateway,
			}
			s.agentMgr.SendRequest(pool.NodeID, "assign_eip", assignPayload, 30*time.Second)
		}

	case models.EIPUsageBridgeIPv6Subnet:
		// bridge IPv6 子段变更，由 updateBridgeIPv6CIDR 统一处理
	}
}

// rebindPortMappingsForDynamicBridge 动态换绑时重建端口映射
func (s *NetworkService) rebindPortMappingsForDynamicBridge(bridgeID uuid.UUID, egressAllocID uuid.UUID, newEgressCIDR string) {
	var portMappings []models.PortMapping
	db.DB.Where("egress_allocation_id = ?", egressAllocID).Find(&portMappings)

	newEgressIP := extractIP(newEgressCIDR)

	for _, pm := range portMappings {
		var instance models.Instance
		if db.DB.Where("id = ?", pm.InstanceID).First(&instance).Error != nil {
			continue
		}

		// 删除旧端口映射
		if s.agentMgr != nil && s.agentMgr.IsNodeConnected(instance.NodeID) {
			delPayload := map[string]interface{}{
				"instance_id": instance.IncusName,
				"host_port":   pm.HostPort,
				"protocol":    pm.Protocol,
			}
			s.agentMgr.SendRequest(instance.NodeID, "del_port_mapping", delPayload, 15*time.Second)
		}

		// 用新 EIP 重建端口映射
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
				"host_ip":        newEgressIP,
				"internal_ip":    internalIP,
			}
			s.agentMgr.SendRequest(instance.NodeID, "add_port_mapping", addPayload, 15*time.Second)
		}
	}
}

// updateBridgeIPv6CIDR 更新引用该池的 bridge 的 IPv6 CIDR
// NAT 实例：更新 Incus bridge ipv6.address，容器通过 DHCPv6 自动获取新 IP
// EIP 实例：由 notifyAgentAllocChange 单独处理 release+assign
func (s *NetworkService) updateBridgeIPv6CIDR(pool *models.EIPPool, oldPoolCIDR, newPoolCIDR string) {
	// 查找引用该池的 bridge（通过 IPv6EIPPoolID）
	var bridges []models.Bridge
	if err := db.DB.Where("ipv6_eip_pool_id = ? AND ipv6_enabled = true", pool.ID).Find(&bridges).Error; err != nil {
		zap.L().Error("[DynamicBinding] 查询引用池的 bridge 失败", zap.Error(err))
		return
	}

	for i := range bridges {
		bridge := &bridges[i]
		if bridge.IPv6CIDR == "" {
			continue
		}

		// 重新计算 bridge IPv6 CIDR：保持偏移量不变
		newBridgeCIDR := recomputeAllocCIDR(oldPoolCIDR, newPoolCIDR, bridge.IPv6CIDR)
		if newBridgeCIDR == "" {
			zap.L().Warn("[DynamicBinding] 无法重算 bridge IPv6 CIDR，跳过",
				zap.String("bridge_id", bridge.ID.String()),
				zap.String("old_bridge_cidr", bridge.IPv6CIDR))
			continue
		}

		// 重新计算 IPv6 Gateway（保持偏移）
		newGateway := ""
		if bridge.IPv6Gateway != "" {
			gwRecomputed := recomputeAllocCIDR(oldPoolCIDR, newPoolCIDR, bridge.IPv6Gateway+"/128")
			if gwRecomputed != "" {
				newGateway = extractIP(gwRecomputed)
			}
		}
		// 如果没算出来，从新 CIDR 推导网关（取第一个地址）
		if newGateway == "" {
			_, newNet, err := net.ParseCIDR(newBridgeCIDR)
			if err == nil {
				gwIP := newNet.IP
				if len(gwIP) == 16 {
					gwIP[15] = 1
				} else if len(gwIP) == 4 {
					gwIP[3] = 1
				}
				newGateway = gwIP.String()
			}
		}

		// 更新 bridge IPv6 CIDR 和 Gateway
		bridgeUpdates := map[string]interface{}{
			"ipv6_cidr":    newBridgeCIDR,
			"ipv6_gateway": newGateway,
		}

		if err := db.DB.Model(bridge).Updates(bridgeUpdates).Error; err != nil {
			zap.L().Error("[DynamicBinding] 更新 bridge IPv6 CIDR 失败",
				zap.String("bridge_id", bridge.ID.String()),
				zap.Error(err))
			continue
		}

		// 更新 bridge IPv6 子段的 EIPAllocation 记录
		var bridgeV6Alloc models.EIPAllocation
		if err := db.DB.Where("bridge_id = ? AND usage = ? AND status = ?",
			bridge.ID, models.EIPUsageBridgeIPv6Subnet, models.EIPAllocationAssigned).First(&bridgeV6Alloc).Error; err == nil {
			allocUpdates := map[string]interface{}{
				"cidr": newBridgeCIDR,
			}
			if _, newAllocNet, err := net.ParseCIDR(newBridgeCIDR); err == nil {
				allocOnes, _ := newAllocNet.Mask.Size()
				allocUpdates["prefix_len"] = allocOnes
			}
			db.DB.Model(&bridgeV6Alloc).Updates(allocUpdates)
		}

		// 通知 Agent 更新 Incus bridge 的 ipv6.address（DHCPv6 范围随之变更）
		if s.agentMgr != nil && s.agentMgr.IsNodeConnected(bridge.NodeID) {
			payload := map[string]interface{}{
				"bridge_name":  bridge.BridgeName,
				"ipv6_cidr":    newBridgeCIDR,
				"ipv6_gateway": newGateway,
				"action":       "update_ipv6",
			}
			if _, err := s.agentMgr.SendRequest(bridge.NodeID, "bridge_network", payload, 15*time.Second); err != nil {
				zap.L().Warn("[DynamicBinding] Agent 更新 bridge IPv6 配置失败",
					zap.String("bridge_id", bridge.ID.String()),
					zap.Error(err))
			}
		}

		// 更新该 bridge 下所有 NAT 实例的 InternalIPv6 DB 记录
		// NAT 实例的 IPv6 由 Incus DHCPv6 管理，bridge ipv6.address 变更后容器自动获取新 IP
		// DB 记录按偏移量重算，保持一致性
		var natInstances []models.Instance
		db.DB.Where("bridge_id = ? AND ipv6_mode = ?", bridge.ID, "nat").Find(&natInstances)
		for j := range natInstances {
			inst := &natInstances[j]
			if inst.InternalIPv6 == "" {
				continue
			}
			// 按偏移量重算实例 InternalIPv6
			newInternalIPv6 := recomputeAllocCIDR(bridge.IPv6CIDR, newBridgeCIDR, inst.InternalIPv6+"/128")
			if newInternalIPv6 != "" {
				newInternalIP := extractIP(newInternalIPv6)
				db.DB.Model(inst).Update("internal_ipv6", newInternalIP)
			}
		}

		// 通知 Agent 重启 NAT 实例的网络接口，触发 DHCPv6 续约获取新 IP
		if s.agentMgr != nil && s.agentMgr.IsNodeConnected(bridge.NodeID) && len(natInstances) > 0 {
			var instanceNames []string
			for j := range natInstances {
				instanceNames = append(instanceNames, natInstances[j].IncusName)
			}
			renewPayload := map[string]interface{}{
				"bridge_name":    bridge.BridgeName,
				"ipv6_cidr":      newBridgeCIDR,
				"ipv6_gateway":   newGateway,
				"instance_names": instanceNames,
			}
			s.agentMgr.SendRequest(bridge.NodeID, "renew_ipv6_dhcp", renewPayload, 30*time.Second)
		}

		zap.L().Info("[DynamicBinding] bridge IPv6 CIDR 已更新",
			zap.String("bridge_id", bridge.ID.String()),
			zap.String("old_cidr", bridge.IPv6CIDR),
			zap.String("new_cidr", newBridgeCIDR),
			zap.Int("nat_instances", len(natInstances)))
	}
}

// recomputeAllocCIDR 根据旧池 CIDR 和新池 CIDR，按偏移量重新计算分配的 CIDR
// 例如旧池 192.168.1.0/24，新池 10.0.0.0/24，分配 192.168.1.5/32 -> 10.0.0.5/32
func recomputeAllocCIDR(oldPoolCIDR, newPoolCIDR, allocCIDR string) string {
	_, oldPoolNet, err := net.ParseCIDR(oldPoolCIDR)
	if err != nil {
		return ""
	}
	_, newPoolNet, err := net.ParseCIDR(newPoolCIDR)
	if err != nil {
		return ""
	}

	// 解析分配 CIDR
	allocIPStr := allocCIDR
	allocPrefixLen := 32
	if idx := strings.Index(allocCIDR, "/"); idx > 0 {
		allocIPStr = allocCIDR[:idx]
		allocPrefixLen, _ = parseInt(allocCIDR[idx+1:])
	}

	allocIP := net.ParseIP(allocIPStr)
	if allocIP == nil {
		return ""
	}

	// 计算偏移量
	oldPoolStart := ipToBigInt(oldPoolNet.IP)
	allocStart := ipToBigInt(allocIP)
	offset := new(big.Int).Sub(allocStart, oldPoolStart)
	if offset.Sign() < 0 {
		return ""
	}

	// 新池起始 + 偏移量
	newPoolStart := ipToBigInt(newPoolNet.IP)
	newAllocStart := new(big.Int).Add(newPoolStart, offset)
	newAllocIP := rangeToIP(newAllocStart, len(newPoolNet.IP))

	return fmt.Sprintf("%s/%d", newAllocIP.String(), allocPrefixLen)
}

// findNewIPOnInterface 在网卡上查找新的可用 IP（排除旧 IP）
func findNewIPOnInterface(nics []dynamicPoolNICInfo, iface, ipVersion, oldIP string) (string, int, string) {
	for _, nic := range nics {
		if nic.Name != iface {
			continue
		}
		ipList := nic.IPv4
		if ipVersion == "ipv6" {
			ipList = nic.IPv6
		}
		for _, ip := range ipList {
			if ip.Address == oldIP {
				continue
			}
			// 跳过内网地址（IPv4）
			if ipVersion == "ipv4" {
				parsedIP := net.ParseIP(ip.Address)
				if parsedIP == nil || parsedIP.IsLoopback() || parsedIP.IsPrivate() || parsedIP.IsLinkLocalUnicast() {
					continue
				}
			}
			return ip.Address, ip.PrefixLen, ip.Gateway
		}
	}
	return "", 0, ""
}

// extractIP 从 CIDR 中提取 IP 地址
func extractIP(cidr string) string {
	if idx := strings.Index(cidr, "/"); idx > 0 {
		return cidr[:idx]
	}
	return cidr
}

// parseInt 简单解析整数
func parseInt(s string) (int, error) {
	var n int
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, fmt.Errorf("invalid int")
		}
		n = n*10 + int(s[i]-'0')
	}
	return n, nil
}
