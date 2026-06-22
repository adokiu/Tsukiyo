package task

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"go.uber.org/zap"
)

func (e *Executor) handleApplyNetwork(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		Action        string `json:"action"`
		InstanceID    string `json:"instance_id"`
		IPAddress     string `json:"ip_address"`
		HostPort      int    `json:"host_port"`
		ContainerPort int    `json:"container_port"`
		Protocol      string `json:"protocol"`
		HostIP        string `json:"host_ip"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析网络应用任务参数失败", zap.Error(err))
		return nil, err
	}

	zap.L().Info("开始应用网络配置",
		zap.String("action", req.Action),
		zap.String("instance_id", req.InstanceID),
		zap.Int("host_port", req.HostPort),
		zap.Int("container_port", req.ContainerPort),
		zap.String("protocol", req.Protocol),
		zap.String("host_ip", req.HostIP))

	instanceIP := ""
	if info, err := e.incusClient.GetInstanceNetworkInfo(req.InstanceID); err == nil && len(info) > 0 {
		instanceIP = info[0]
	} else {
		zap.L().Warn("获取实例内部 IP 失败，proxy connect 地址可能为空", zap.Error(err))
	}

	hostIP := req.HostIP
	if hostIP == "" {
		return nil, fmt.Errorf("缺少 host_ip，无法配置端口映射")
	}

	deviceName := fmt.Sprintf("proxy-%d-%s", req.HostPort, req.Protocol)

	switch req.Action {
	case "add_ip":
		if err := e.netManager.BindIP(req.IPAddress, e.cfg.NetworkInterface()); err != nil {
			zap.L().Error("绑定 IP 失败", zap.Error(err))
			return nil, err
		}
		zap.L().Info("绑定 IP 成功", zap.String("ip", req.IPAddress))
	case "remove_ip":
		if err := e.netManager.UnbindIP(req.IPAddress, e.cfg.NetworkInterface()); err != nil {
			zap.L().Error("解绑 IP 失败", zap.Error(err))
			return nil, err
		}
		zap.L().Info("解绑 IP 成功", zap.String("ip", req.IPAddress))
	case "add_port":
		if instanceIP == "" {
			zap.L().Error("添加端口映射失败：实例内部 IP 为空")
			return nil, fmt.Errorf("实例内部 IP 为空，无法创建 proxy 端口映射")
		}
		listenIP := req.HostIP
		if idx := strings.Index(listenIP, "/"); idx > 0 {
			listenIP = listenIP[:idx]
		}
		if listenIP == "" {
			listenIP = "0.0.0.0"
		}
		listenAddr := fmt.Sprintf("%s:%s:%d", req.Protocol, listenIP, req.HostPort)
		connectAddr := fmt.Sprintf("%s:%s:%d", req.Protocol, instanceIP, req.ContainerPort)
		if err := e.incusClient.AddProxyDevice(req.InstanceID, deviceName, listenAddr, connectAddr); err != nil {
			zap.L().Error("添加 proxy 端口映射失败",
				zap.String("device", deviceName),
				zap.String("listen", listenAddr),
				zap.String("connect", connectAddr),
				zap.Error(err))
			return nil, fmt.Errorf("添加 proxy 端口映射失败: %w", err)
		}
		zap.L().Info("添加 proxy 端口映射成功",
			zap.String("device", deviceName),
			zap.String("listen", listenAddr),
			zap.String("connect", connectAddr))
	case "del_port":
		if err := e.incusClient.RemoveProxyDevice(req.InstanceID, deviceName); err != nil {
			zap.L().Error("删除 proxy 端口映射失败",
				zap.String("device", deviceName),
				zap.Error(err))
			return nil, fmt.Errorf("删除 proxy 端口映射失败: %w", err)
		}
		zap.L().Info("删除 proxy 端口映射成功", zap.String("device", deviceName))
	case "add_nat":
		zap.L().Warn("add_nat 已废弃，请使用 add_port")
	case "remove_nat":
		zap.L().Warn("remove_nat 已废弃")
	default:
		zap.L().Warn("未知的网络操作类型", zap.String("action", req.Action))
		return nil, fmt.Errorf("未知的网络操作类型: %s", req.Action)
	}

	return json.Marshal(map[string]string{"status": "applied"})
}

func (e *Executor) handleApplyFirewall(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		Rules []struct {
			Direction  string `json:"direction"`
			Protocol   string `json:"protocol"`
			Source     string `json:"source"`
			Port       int    `json:"port"`
			Action     string `json:"action"`
			InstanceID string `json:"instance_id"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}

	for _, rule := range req.Rules {
		switch rule.Action {
		case "allow":
			e.netManager.AddFirewallRule(rule.Direction, rule.Protocol, rule.Source, rule.Port, rule.Action)
		case "deny":
			e.netManager.AddFirewallRule(rule.Direction, rule.Protocol, rule.Source, rule.Port, rule.Action)
		case "remove":
			e.netManager.RemoveFirewallRule(rule.Direction, rule.Protocol, rule.Source, rule.Port)
		}
	}

	return json.Marshal(map[string]string{"status": "applied"})
}

// handleBridgeNetwork 处理 Bridge 网络配置任务（创建/更新/删除 Incus bridge）
func (e *Executor) handleBridgeNetwork(payload json.RawMessage) (json.RawMessage, error) {
	zap.L().Info("[Bridge] handleBridgeNetwork 被调用", zap.Int("payload_len", len(payload)))

	var req struct {
		BridgeID    string `json:"bridge_id"`
		Action      string `json:"action"`
		BridgeName  string `json:"bridge_name"`
		IPv4CIDR    string `json:"ipv4_cidr"`
		IPv6CIDR    string `json:"ipv6_cidr"`
		IPv4Gateway string `json:"ipv4_gateway"`
		IPv6Gateway string `json:"ipv6_gateway"`
		IPv4Enabled bool   `json:"ipv4_enabled"`
		IPv6Enabled bool   `json:"ipv6_enabled"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("[Bridge] 解析 payload 失败", zap.Error(err))
		return nil, fmt.Errorf("解析 Bridge 任务参数失败: %w", err)
	}

	zap.L().Info("[Bridge] 任务参数解析成功",
		zap.String("action", req.Action),
		zap.String("bridge_id", req.BridgeID),
		zap.String("bridge_name", req.BridgeName),
		zap.String("ipv4_cidr", req.IPv4CIDR),
		zap.String("gateway_v4", req.IPv4Gateway))

	switch req.Action {
	case "create":
		ipv6Addr := ""
		if req.IPv6Enabled && req.IPv6CIDR != "" {
			gw := req.IPv6Gateway
			if gw == "" {
				_, ipNet, _ := net.ParseCIDR(req.IPv6CIDR)
				if ipNet != nil {
					ip := ipNet.IP
					if len(ip) == 16 {
						ip[15] = 1
					}
					gw = ip.String()
				}
			}
			if gw != "" {
				maskStr := "64"
				if idx := strings.Index(req.IPv6CIDR, "/"); idx > 0 {
					maskStr = req.IPv6CIDR[idx+1:]
				}
				ipv6Addr = gw + "/" + maskStr
			}
		}
		if err := e.incusClient.CreateBridgeNetwork(req.BridgeName, req.IPv4CIDR, ipv6Addr, "", req.IPv4Gateway); err != nil {
			zap.L().Error("创建 bridge 网络失败", zap.String("bridge", req.BridgeName), zap.Error(err))
			e.incusClient.DeleteBridgeNetwork(req.BridgeName)
			return nil, fmt.Errorf("创建 bridge 网络失败: %w", err)
		}
		zap.L().Info("bridge 网络创建成功", zap.String("bridge", req.BridgeName))

	case "update":
		zap.L().Info("Bridge 更新完成", zap.String("bridge", req.BridgeName))

	case "delete":
		if err := e.incusClient.DeleteBridgeNetwork(req.BridgeName); err != nil {
			zap.L().Error("删除 bridge 网络失败", zap.String("bridge", req.BridgeName), zap.Error(err))
			return nil, fmt.Errorf("删除 bridge 网络失败: %w", err)
		}
		zap.L().Info("bridge 网络删除成功", zap.String("bridge", req.BridgeName))

	default:
		return nil, fmt.Errorf("未知的 Bridge 动作: %s", req.Action)
	}

	return json.Marshal(map[string]string{"status": "ok", "action": req.Action})
}

// handleBindBridgeEgress 通过 Incus 设置 Bridge 的 NAT 出口 IP
func (e *Executor) handleBindBridgeEgress(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		BridgeName string `json:"bridge_name"`
		EgressCIDR string `json:"egress_cidr"`
		Interface  string `json:"interface"`
		IPVersion  string `json:"ip_version"`
	}
	zap.L().Info("[BindEgress] 收到 payload", zap.String("payload", string(payload)))
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("[BindEgress] 解析参数失败", zap.Error(err), zap.String("payload", string(payload)))
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}

	zap.L().Info("[BindEgress] 绑定 Bridge 出口 EIP",
		zap.String("bridge", req.BridgeName),
		zap.String("egress_cidr", req.EgressCIDR),
		zap.String("ip_version", req.IPVersion))

	// 提取 EIP（去掉掩码）
	egressIP := req.EgressCIDR
	if idx := strings.Index(egressIP, "/"); idx > 0 {
		egressIP = egressIP[:idx]
	}

	// 先关闭 NAT 清除旧规则
	clearConfig := map[string]string{
		"ipv4.nat": "false",
	}
	zap.L().Info("[BindEgress] 步骤1: 关闭 NAT 清除旧规则", zap.String("bridge", req.BridgeName))
	if err := e.incusClient.UpdateBridgeNetwork(req.BridgeName, clearConfig); err != nil {
		zap.L().Warn("[BindEgress] 关闭 NAT 失败", zap.Error(err))
	}

	// 再开启 NAT 并设置新的出口地址
	config := map[string]string{
		"ipv4.nat":         "true",
		"ipv4.nat.address": egressIP,
	}
	zap.L().Info("[BindEgress] 步骤2: 设置新 NAT 出口", zap.String("bridge", req.BridgeName), zap.String("egress_ip", egressIP))
	if err := e.incusClient.UpdateBridgeNetwork(req.BridgeName, config); err != nil {
		zap.L().Error("[BindEgress] 绑定 Bridge 出口 EIP 失败", zap.Error(err))
		return nil, fmt.Errorf("绑定出口 EIP 失败: %w", err)
	}

	zap.L().Info("Bridge 出口 EIP 绑定成功",
		zap.String("bridge", req.BridgeName),
		zap.String("egress_ip", egressIP))

	return json.Marshal(map[string]string{"status": "ok"})
}

// handleUnbindBridgeEgress 通过 Incus 清除 Bridge 的 NAT 出口 IP
func (e *Executor) handleUnbindBridgeEgress(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		BridgeName string `json:"bridge_name"`
		EgressCIDR string `json:"egress_cidr"`
		Interface  string `json:"interface"`
		IPVersion  string `json:"ip_version"`
	}
	zap.L().Info("[UnbindEgress] 收到 payload", zap.String("payload", string(payload)))
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("[UnbindEgress] 解析参数失败", zap.Error(err), zap.String("payload", string(payload)))
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}

	zap.L().Info("[UnbindEgress] 解绑 Bridge 出口 EIP",
		zap.String("bridge", req.BridgeName),
		zap.String("ip_version", req.IPVersion))

	// 通过 Incus 清除 bridge 网络的 NAT 出口地址并关闭 NAT
	config := map[string]string{
		"ipv4.nat":         "false",
		"ipv4.nat.address": "",
	}

	if err := e.incusClient.UpdateBridgeNetwork(req.BridgeName, config); err != nil {
		zap.L().Error("解绑 Bridge 出口 EIP 失败", zap.Error(err))
		return nil, fmt.Errorf("解绑出口 EIP 失败: %w", err)
	}

	zap.L().Info("Bridge 出口 EIP 解绑成功",
		zap.String("bridge", req.BridgeName))

	return json.Marshal(map[string]string{"status": "ok"})
}
