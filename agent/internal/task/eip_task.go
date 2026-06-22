package task

import (
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"tsukiyo/agent/internal/nftables"

	"go.uber.org/zap"
)

// eipAssignRequest EIP 分配请求
type eipAssignRequest struct {
	InstanceName     string `json:"instance_name"`
	InstanceIP       string `json:"instance_ip"`
	EIPCidr          string `json:"eip_cidr"`
	Interface        string `json:"interface"`
	IPVersion        string `json:"ip_version"`
	BridgeName       string `json:"bridge_name"`
	MappedInternalIP string `json:"mapped_internal_ip"`
	IPv4CIDR         string `json:"ipv4_cidr"`
	IPv6CIDR         string `json:"ipv6_cidr"`
	IPv4Gateway      string `json:"ipv4_gateway"`
	IPv6Gateway      string `json:"ipv6_gateway"`
	EIPGateway       string `json:"eip_gateway"`
}

// eipReleaseRequest EIP 释放请求
type eipReleaseRequest struct {
	InstanceName     string `json:"instance_name"`
	InstanceIP       string `json:"instance_ip"`
	EIPCidr          string `json:"eip_cidr"`
	Interface        string `json:"interface"`
	IPVersion        string `json:"ip_version"`
	BridgeName       string `json:"bridge_name"`
	MappedInternalIP string `json:"mapped_internal_ip"`
}

// handleAssignEIP 在 Agent 节点上为实例分配 EIP
func (e *Executor) handleAssignEIP(payload json.RawMessage) (json.RawMessage, error) {
	var req eipAssignRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("解析 EIP 分配参数失败: %w", err)
	}

	zap.L().Info("[AssignEIP] 开始分配实例 EIP",
		zap.String("instance", req.InstanceName),
		zap.String("eip_cidr", req.EIPCidr),
		zap.String("ip_version", req.IPVersion),
		zap.String("mapped_internal_ip", req.MappedInternalIP))

	if req.EIPCidr == "" {
		return nil, fmt.Errorf("eip_cidr 为空")
	}

	iface := req.Interface
	if iface == "" {
		iface = e.netManager.GetInterfaceName()
	}

	eipAddr := req.EIPCidr
	if idx := strings.Index(eipAddr, "/"); idx > 0 {
		eipAddr = eipAddr[:idx]
	}

	if req.IPVersion == "ipv6" {
		return e.assignEIPv6(req, iface, eipAddr)
	}
	return e.assignEIPv4(req, iface, eipAddr)
}

// assignEIPv4 IPv4 EIP 分配：EIP 绑定主机网卡 + Incus 配置内网IP + nftables SNAT/DNAT
func (e *Executor) assignEIPv4(req eipAssignRequest, iface, eipAddr string) (json.RawMessage, error) {
	// 步骤1: 将 EIP 绑定到物理网卡
	if err := e.netManager.BindIP(req.EIPCidr, iface); err != nil {
		zap.L().Error("[AssignEIP-v4] 绑定 EIP 到网卡失败",
			zap.String("eip", req.EIPCidr), zap.String("iface", iface), zap.Error(err))
		return nil, fmt.Errorf("绑定 EIP 到网卡失败: %w", err)
	}
	zap.L().Info("[AssignEIP-v4] EIP 绑定到网卡成功", zap.String("eip", eipAddr), zap.String("iface", iface))

	// 步骤2: 通过 Incus 配置容器的 ipv4.address（追加 mapped_internal_ip）
	// 每个 EIP 对应容器内一个内网 IP，通过 Incus 管理
	mappedIP := req.MappedInternalIP
	if mappedIP != "" && mappedIP != req.InstanceIP && req.InstanceName != "" {
		// 获取当前 ipv4.address
		getCmd := exec.Command("incus", "config", "device", "get", req.InstanceName, "eth0", "ipv4.address")
		getOut, err := getCmd.Output()
		if err == nil {
			current := strings.TrimSpace(string(getOut))
			if current == "" {
				current = req.InstanceIP
			}
			// 追加新的内网 IP
			newAddr := current + "," + mappedIP
			setCmd := exec.Command("incus", "config", "device", "set", req.InstanceName, "eth0", "ipv4.address", newAddr)
			if out, err := setCmd.CombinedOutput(); err != nil {
				zap.L().Warn("[AssignEIP-v4] Incus 配置 ipv4.address 追加失败（非致命）",
					zap.String("mapped_ip", mappedIP), zap.Error(err), zap.String("output", string(out)))
			} else {
				zap.L().Info("[AssignEIP-v4] Incus 配置 ipv4.address 追加成功", zap.String("addr", newAddr))
			}
		}
	}

	// 步骤3: 添加 nftables SNAT 规则
	// 来自容器的流量 SNAT 为 EIP
	// 容器默认路由使用 instanceIP 作为源 IP，mappedIP 追加后容器内可能有多个 IP
	// 需要同时匹配 instanceIP 和 mappedIP，确保所有出网流量都走 EIP
	snatSources := []string{req.InstanceIP}
	if mappedIP != "" && mappedIP != req.InstanceIP {
		snatSources = append(snatSources, mappedIP)
	}
	// 确保 tsukiyo 表和链存在
	nftables.EnsureTable()
	for _, src := range snatSources {
		if src == "" {
			continue
		}
		rule := fmt.Sprintf("oifname %s ip saddr %s snat to %s", iface, src, eipAddr)
		comment := fmt.Sprintf("tsukiyo-snat-%s-%s", eipAddr, src)
		if err := nftables.AddRule("postrouting", rule, comment); err != nil {
			zap.L().Error("[AssignEIP-v4] 添加 SNAT 规则失败",
				zap.String("snat_src", src), zap.String("eip", eipAddr),
				zap.Error(err))
			return nil, fmt.Errorf("添加 SNAT 规则失败: %w", err)
		}
		zap.L().Info("[AssignEIP-v4] SNAT 规则添加成功",
			zap.String("snat_src", src), zap.String("eip", eipAddr))
	}

	// 步骤4: 添加 nftables FORWARD 规则允许转发
	for _, src := range snatSources {
		if src == "" {
			continue
		}
		rule := fmt.Sprintf("ip saddr %s accept", src)
		comment := fmt.Sprintf("tsukiyo-fwd-%s-%s", eipAddr, src)
		nftables.AddRuleSilent("forward", rule, comment)
	}

	// 步骤5: 添加 DNAT 规则，入站流量转发到容器
	if req.InstanceIP != "" {
		rule := fmt.Sprintf("iifname %s ip daddr %s dnat to %s", iface, eipAddr, req.InstanceIP)
		comment := fmt.Sprintf("tsukiyo-dnat-%s", eipAddr)
		nftables.AddRuleSilent("prerouting", rule, comment)
	}

	// 步骤6: policy routing 确保回包从正确接口出去
	// 添加路由表：来自 EIP 的流量使用默认路由通过 iface
	rtTable := fmt.Sprintf("tsukiyo-eip-%s", strings.ReplaceAll(eipAddr, ".", "-"))
	// 查找或创建路由表
	exec.Command("sh", "-c", fmt.Sprintf("grep -q '%s' /etc/iproute2/rt_tables || echo '100 %s' >> /etc/iproute2/rt_tables", rtTable, rtTable)).Run()
	// 默认路由通过 iface
	exec.Command("ip", "route", "replace", "default", "dev", iface, "table", rtTable).Run()
	// 规则：来自 EIP 的流量使用该路由表
	exec.Command("ip", "rule", "add", "from", eipAddr, "table", rtTable).Run()

	zap.L().Info("[AssignEIP-v4] 实例 IPv4 EIP 分配成功",
		zap.String("instance", req.InstanceName), zap.String("eip", eipAddr))

	return json.Marshal(map[string]string{
		"status":   "ok",
		"eip":      eipAddr,
		"instance": req.InstanceName,
	})
}

// assignEIPv6 IPv6 EIP 分配：通过 Incus 配置 ipv6.address + 主机 NDP 代理 + 主机路由
// IPv6 EIP 从 bridge 子网分配，地址在 bridge CIDR 内
// Incus ipv6.address 只接受单个 IP，不支持 CIDR。分配非 /128 子段时：
//   - ipv6.address 设为子段第一个 IP（Incus 校验通过）
//   - 容器内部通过 incus exec 添加完整 CIDR 到 eth0
//   - 主机添加子段路由到 bridge + NDP 代理
func (e *Executor) assignEIPv6(req eipAssignRequest, iface, eipAddr string) (json.RawMessage, error) {
	bridgeName := req.BridgeName
	if bridgeName == "" {
		bridgeName = "tsukiyo-br0"
	}

	// 解析完整 CIDR
	eipCIDR := req.EIPCidr
	if !strings.Contains(eipCIDR, "/") {
		eipCIDR = eipAddr + "/128"
	}

	// 从 CIDR 中提取网络地址（子段第一个 IP），用于 Incus ipv6.address
	netIP := eipAddr
	if ip, ipNet, err := net.ParseCIDR(eipCIDR); err == nil {
		netIP = ip.Mask(ipNet.Mask).String()
	}

	// 判断是否为单地址（/128）
	isSingle := strings.HasSuffix(eipCIDR, "/128")

	// 步骤1: 开启 IPv6 转发和 NDP 代理
	exec.Command("sysctl", "-w", "net.ipv6.conf.all.forwarding=1").Run()
	exec.Command("sysctl", "-w", fmt.Sprintf("net.ipv6.conf.%s.proxy_ndp=1", iface)).Run()
	exec.Command("sysctl", "-w", fmt.Sprintf("net.ipv6.conf.%s.accept_ra=2", iface)).Run()

	// 步骤1.5: 确保 bridge 网络使用 DHCPv6 有状态模式（禁用 SLAAC）
	exec.Command("incus", "network", "set", bridgeName, "ipv6.dhcp", "true").Run()
	exec.Command("incus", "network", "set", bridgeName, "ipv6.dhcp.stateful", "true").Run()

	// 步骤2: 通过 Incus 配置容器的 ipv6.address（只传纯 IP 地址，不带前缀）
	if req.InstanceName != "" {
		cmd := exec.Command("incus", "config", "device", "set", req.InstanceName, "eth0", "ipv6.address", netIP)
		if out, err := cmd.CombinedOutput(); err != nil {
			zap.L().Error("[AssignEIP-v6] Incus 配置 ipv6.address 失败",
				zap.String("ipv6", netIP), zap.Error(err), zap.String("output", string(out)))
			return nil, fmt.Errorf("Incus 配置 ipv6.address 失败: %w", err)
		}
		zap.L().Info("[AssignEIP-v6] Incus 配置 ipv6.address 成功", zap.String("ipv6", netIP))

		// 设置容器 IPv6 网关（使用 bridge 的网关）
		gateway := req.EIPGateway
		if gateway == "" {
			gateway = req.IPv6Gateway
		}
		if gateway != "" {
			exec.Command("incus", "config", "device", "set", req.InstanceName, "eth0", "ipv6.gateway", gateway).Run()
		}

		// 非单地址时，在容器内部添加完整 CIDR 到 eth0
		if !isSingle {
			// 等待容器网络就绪后添加地址
			addCmd := exec.Command("incus", "exec", req.InstanceName, "--",
				"ip", "-6", "addr", "add", eipCIDR, "dev", "eth0")
			if out, err := addCmd.CombinedOutput(); err != nil {
				zap.L().Warn("[AssignEIP-v6] 容器内添加 CIDR 失败（容器可能未启动）",
					zap.String("cidr", eipCIDR), zap.Error(err), zap.String("output", string(out)))
			} else {
				zap.L().Info("[AssignEIP-v6] 容器内 CIDR 添加成功", zap.String("cidr", eipCIDR))
			}
		}
	}

	// 步骤3: 主机添加路由到 bridge（使用实际前缀长度）
	cmd := exec.Command("ip", "-6", "route", "replace", eipCIDR, "dev", bridgeName)
	if out, err := cmd.CombinedOutput(); err != nil {
		zap.L().Warn("[AssignEIP-v6] 主机添加路由失败",
			zap.String("cidr", eipCIDR), zap.String("bridge", bridgeName),
			zap.Error(err), zap.String("output", string(out)))
	} else {
		zap.L().Info("[AssignEIP-v6] 主机路由添加成功",
			zap.String("cidr", eipCIDR), zap.String("bridge", bridgeName))
	}

	// 步骤4: 添加 NDP 代理
	if err := e.netManager.AddIPv6Proxy(netIP, iface); err != nil {
		zap.L().Warn("[AssignEIP-v6] 添加 NDP 代理失败（非致命）",
			zap.String("ipv6", netIP), zap.Error(err))
	} else {
		zap.L().Info("[AssignEIP-v6] NDP 代理添加成功", zap.String("ipv6", netIP))
	}

	// 步骤5: 添加 nftables FORWARD 规则
	nftables.EnsureTable()
	rule := fmt.Sprintf("ip6 saddr %s accept", eipCIDR)
	comment := fmt.Sprintf("tsukiyo-fwd6-%s", netIP)
	nftables.AddRuleSilent("forward", rule, comment)

	zap.L().Info("[AssignEIP-v6] 实例 IPv6 EIP 分配成功",
		zap.String("instance", req.InstanceName), zap.String("ipv6", eipCIDR))

	return json.Marshal(map[string]string{
		"status":   "ok",
		"eip":      netIP,
		"instance": req.InstanceName,
	})
}

// handleReleaseEIP 在 Agent 节点上释放实例 EIP
func (e *Executor) handleReleaseEIP(payload json.RawMessage) (json.RawMessage, error) {
	var req eipReleaseRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("解析 EIP 释放参数失败: %w", err)
	}

	zap.L().Info("[ReleaseEIP] 开始释放实例 EIP",
		zap.String("instance", req.InstanceName),
		zap.String("eip_cidr", req.EIPCidr),
		zap.String("ip_version", req.IPVersion),
		zap.String("mapped_internal_ip", req.MappedInternalIP))

	if req.EIPCidr == "" {
		return nil, fmt.Errorf("eip_cidr 为空")
	}

	iface := req.Interface
	if iface == "" {
		iface = e.netManager.GetInterfaceName()
	}

	eipAddr := req.EIPCidr
	if idx := strings.Index(eipAddr, "/"); idx > 0 {
		eipAddr = eipAddr[:idx]
	}

	if req.IPVersion == "ipv6" {
		return e.releaseEIPv6(req, iface, eipAddr)
	}
	return e.releaseEIPv4(req, iface, eipAddr)
}

// releaseEIPv4 清理 IPv4 EIP：删除 SNAT + 从容器移除内网IP + 解绑 EIP
func (e *Executor) releaseEIPv4(req eipReleaseRequest, iface, eipAddr string) (json.RawMessage, error) {
	mappedIP := req.MappedInternalIP

	// 步骤1: 删除 nftables SNAT 规则
	if mappedIP != "" {
		nftables.DeleteRulesByCommentPrefix("postrouting", fmt.Sprintf("tsukiyo-snat-%s", eipAddr))
		zap.L().Info("[ReleaseEIP-v4] SNAT 规则已删除", zap.String("mapped_ip", mappedIP), zap.String("eip", eipAddr))
	}

	// 步骤2: 删除 FORWARD 规则
	if mappedIP != "" {
		nftables.DeleteRulesByCommentPrefix("forward", fmt.Sprintf("tsukiyo-fwd-%s", eipAddr))
	}

	// 步骤2.5: 删除 DNAT 规则
	nftables.DeleteRulesByCommentPrefix("prerouting", fmt.Sprintf("tsukiyo-dnat-%s", eipAddr))

	// 步骤3: 从容器内移除额外的内网 IP
	if mappedIP != "" && req.InstanceName != "" {
		prefixLen := "24"
		// 尝试删除，忽略错误（容器可能已停止）
		exec.Command("incus", "exec", req.InstanceName, "--",
			"ip", "addr", "del", fmt.Sprintf("%s/%s", mappedIP, prefixLen), "dev", "eth0").Run()
		zap.L().Info("[ReleaseEIP-v4] 容器内内网 IP 已移除", zap.String("mapped_ip", mappedIP))
	}

	// 步骤4: 从网卡解绑 EIP
	if err := e.netManager.UnbindIP(req.EIPCidr, iface); err != nil {
		zap.L().Warn("[ReleaseEIP-v4] 解绑 EIP 失败（可能已解绑）",
			zap.String("eip", req.EIPCidr), zap.Error(err))
	}

	zap.L().Info("[ReleaseEIP-v4] 实例 IPv4 EIP 释放成功",
		zap.String("instance", req.InstanceName), zap.String("eip", eipAddr))

	return json.Marshal(map[string]string{
		"status":   "ok",
		"eip":      eipAddr,
		"instance": req.InstanceName,
	})
}

// releaseEIPv6 清理 IPv6 EIP：通过 Incus 移除 ipv6.address + 删除主机路由 + 删除NDP代理 + 清理nftables
func (e *Executor) releaseEIPv6(req eipReleaseRequest, iface, eipAddr string) (json.RawMessage, error) {
	bridgeName := req.BridgeName
	if bridgeName == "" {
		bridgeName = "tsukiyo-br0"
	}

	eipCIDR := req.EIPCidr
	if !strings.Contains(eipCIDR, "/") {
		eipCIDR = eipAddr + "/128"
	}

	// 从 CIDR 中提取网络地址
	netIP := eipAddr
	if ip, ipNet, err := net.ParseCIDR(eipCIDR); err == nil {
		netIP = ip.Mask(ipNet.Mask).String()
	}
	isSingle := strings.HasSuffix(eipCIDR, "/128")

	// 步骤1: 通过 Incus 移除容器的 ipv6.address
	if req.InstanceName != "" {
		exec.Command("incus", "config", "device", "unset", req.InstanceName, "eth0", "ipv6.address").Run()
		zap.L().Info("[ReleaseEIP-v6] Incus ipv6.address 已移除", zap.String("ipv6", netIP))

		// 非单地址时，移除容器内的 CIDR
		if !isSingle {
			exec.Command("incus", "exec", req.InstanceName, "--",
				"ip", "-6", "addr", "del", eipCIDR, "dev", "eth0").Run()
			zap.L().Info("[ReleaseEIP-v6] 容器内 CIDR 已移除", zap.String("cidr", eipCIDR))
		}
	}

	// 步骤2: 删除主机路由
	exec.Command("ip", "-6", "route", "del", eipCIDR, "dev", bridgeName).Run()
	zap.L().Info("[ReleaseEIP-v6] 主机路由已删除", zap.String("cidr", eipCIDR))

	// 步骤3: 删除 NDP 代理
	exec.Command("ip", "-6", "neigh", "del", "proxy", netIP, "dev", iface).Run()
	zap.L().Info("[ReleaseEIP-v6] NDP 代理已移除", zap.String("ipv6", netIP))

	// 步骤4: 删除 nftables FORWARD 规则
	nftables.DeleteRulesByCommentPrefix("forward", fmt.Sprintf("tsukiyo-fwd6-%s", netIP))

	zap.L().Info("[ReleaseEIP-v6] 实例 IPv6 EIP 释放成功",
		zap.String("instance", req.InstanceName), zap.String("ipv6", eipCIDR))

	return json.Marshal(map[string]string{
		"status":   "ok",
		"eip":      netIP,
		"instance": req.InstanceName,
	})
}
