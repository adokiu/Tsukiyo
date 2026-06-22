package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/geoip"
	"tsukiyo/master/internal/models"
)

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

		// 查询该节点所有已分配的实例 EIP 并下发（Agent 重启后重建 nftables 规则）
		var eipAllocs []models.EIPAllocation
		if err := db.DB.Where("node_id = ? AND status = ? AND usage = ?",
			nodeID, models.EIPAllocationAssigned, models.EIPUsageInstanceEIP).Find(&eipAllocs).Error; err == nil && len(eipAllocs) > 0 {
			eipConfigs := make([]map[string]interface{}, 0, len(eipAllocs))
			for _, alloc := range eipAllocs {
				var instance models.Instance
				var bridge models.Bridge
				var pool models.EIPPool
				db.DB.Where("id = ?", alloc.InstanceID).First(&instance)
				if instance.BridgeID != nil {
					db.DB.Where("id = ?", *instance.BridgeID).First(&bridge)
				}
				db.DB.Where("id = ?", alloc.PoolID).First(&pool)

				internalIP := instance.InternalIPv4
				if alloc.IPVersion == "ipv6" {
					internalIP = instance.InternalIPv6
				}

				eipConfigs = append(eipConfigs, map[string]interface{}{
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
					"eip_gateway":        pool.Gateway,
				})
			}
			cfg["eip_allocations"] = eipConfigs
			zap.L().Info("下发 EIP 分配信息到 Agent", zap.String("node_id", nodeID.String()), zap.Int("count", len(eipConfigs)))
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
