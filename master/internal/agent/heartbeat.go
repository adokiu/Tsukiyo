package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

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
