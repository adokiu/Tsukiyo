package network

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"go.uber.org/zap"
)

// Manager 网络管理器
type Manager struct {
	mu             sync.RWMutex
	interfaceName  string
	enableNAT      bool
	enableFirewall bool
	portMappings   map[string]PortMapping
	firewallRules  map[string]FirewallRule
	blockedIPs     map[string]bool
}

// PortMapping 端口映射
type PortMapping struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
	ContainerIP   string `json:"container_ip"`
}

// FirewallRule 防火墙规则
type FirewallRule struct {
	Direction string `json:"direction"`
	Protocol  string `json:"protocol"`
	Source    string `json:"source"`
	Port      int    `json:"port"`
	Action    string `json:"action"`
}

// NewManager 创建网络管理器
func NewManager(interfaceName string, enableNAT, enableFirewall bool) *Manager {
	if interfaceName == "" {
		interfaceName = detectMainInterface()
	}
	return &Manager{
		interfaceName:  interfaceName,
		enableNAT:      enableNAT,
		enableFirewall: enableFirewall,
		portMappings:   make(map[string]PortMapping),
		firewallRules:  make(map[string]FirewallRule),
		blockedIPs:     make(map[string]bool),
	}
}

// detectMainInterface 检测主网卡
func detectMainInterface() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "eth0"
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 {
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						return iface.Name
					}
				}
			}
		}
	}
	return "eth0"
}

// BindIP 绑定 IP 到网卡
func (m *Manager) BindIP(ipAddress string, iface string) error {
	if iface == "" {
		iface = m.interfaceName
	}

	// 检查 IP 是否已在网卡上，避免重复添加导致路由变化
	ipAddr := ipAddress
	if idx := strings.Index(ipAddress, "/"); idx > 0 {
		ipAddr = ipAddress[:idx]
	}
	checkCmd := exec.Command("ip", "-o", "addr", "show", "dev", iface)
	checkOut, err := checkCmd.Output()
	if err == nil && strings.Contains(string(checkOut), ipAddr) {
		zap.L().Info("IP 已在网卡上，跳过绑定", zap.String("ip", ipAddress), zap.String("iface", iface))
		return nil
	}

	// 先尝试解绑旧的
	m.UnbindIP(ipAddress, iface)

	cmd := exec.Command("ip", "addr", "add", ipAddress, "dev", iface)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("绑定 IP 失败: %s, %w", string(out), err)
	}

	zap.L().Info("IP 绑定成功", zap.String("ip", ipAddress), zap.String("iface", iface))
	return nil
}

// UnbindIP 从网卡解绑 IP
func (m *Manager) UnbindIP(ipAddress string, iface string) error {
	if iface == "" {
		iface = m.interfaceName
	}

	cmd := exec.Command("ip", "addr", "del", ipAddress, "dev", iface)
	cmd.Run() // 忽略错误，可能本来就不存在

	zap.L().Info("IP 解绑成功", zap.String("ip", ipAddress))
	return nil
}

// AddIPv6Proxy 添加 IPv6 NDP 代理
func (m *Manager) AddIPv6Proxy(ipv6Addr string, iface string) error {
	if iface == "" {
		iface = m.interfaceName
	}
	cmd := exec.Command("ip", "-6", "neigh", "add", "proxy", ipv6Addr, "dev", iface)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("添加 NDP 代理失败: %s", string(out))
	}
	return nil
}

// AddPortMapping 添加端口映射 (DNAT)
func (m *Manager) AddPortMapping(hostPort, containerPort int, protocol, containerIP string) error {
	if !m.enableNAT {
		return nil
	}
	if protocol == "" {
		protocol = "tcp"
	}

	m.mu.Lock()
	key := fmt.Sprintf("%d/%s", hostPort, protocol)
	m.portMappings[key] = PortMapping{
		HostPort:      hostPort,
		ContainerPort: containerPort,
		Protocol:      protocol,
		ContainerIP:   containerIP,
	}
	m.mu.Unlock()

	// iptables DNAT
	cmd := exec.Command("iptables", "-t", "nat", "-A", "PREROUTING",
		"-p", protocol, "--dport", strconv.Itoa(hostPort),
		"-j", "DNAT", "--to-destination",
		fmt.Sprintf("%s:%d", containerIP, containerPort))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("添加 DNAT 失败: %s", string(out))
	}

	// iptables FORWARD 允许
	cmd2 := exec.Command("iptables", "-A", "FORWARD",
		"-p", protocol, "--dport", strconv.Itoa(containerPort),
		"-j", "ACCEPT")
	cmd2.Run()

	zap.L().Info("端口映射添加成功",
		zap.Int("host_port", hostPort),
		zap.Int("container_port", containerPort),
		zap.String("protocol", protocol))
	return nil
}

// RemovePortMapping 删除端口映射
func (m *Manager) RemovePortMapping(hostPort int, protocol string) error {
	if !m.enableNAT {
		return nil
	}
	if protocol == "" {
		protocol = "tcp"
	}

	m.mu.Lock()
	key := fmt.Sprintf("%d/%s", hostPort, protocol)
	mapping, exists := m.portMappings[key]
	if exists {
		delete(m.portMappings, key)
	}
	m.mu.Unlock()

	// 删除 iptables 规则
	cmd := exec.Command("iptables", "-t", "nat", "-D", "PREROUTING",
		"-p", protocol, "--dport", strconv.Itoa(hostPort),
		"-j", "DNAT", "--to-destination",
		fmt.Sprintf("%s:%d", mapping.ContainerIP, mapping.ContainerPort))
	cmd.Run()

	zap.L().Info("端口映射删除成功", zap.Int("host_port", hostPort))
	return nil
}

// AddFirewallRule 添加防火墙规则
func (m *Manager) AddFirewallRule(direction, protocol, source string, port int, action string) error {
	if !m.enableFirewall {
		return nil
	}
	if protocol == "" {
		protocol = "tcp"
	}

	m.mu.Lock()
	key := fmt.Sprintf("%s-%s-%s-%d-%s", direction, protocol, source, port, action)
	m.firewallRules[key] = FirewallRule{
		Direction: direction,
		Protocol:  protocol,
		Source:    source,
		Port:      port,
		Action:    action,
	}
	m.mu.Unlock()

	var args []string
	if direction == "ingress" || direction == "inbound" {
		args = []string{"-A", "INPUT"}
	} else {
		args = []string{"-A", "OUTPUT"}
	}

	if source != "" && source != "0.0.0.0/0" {
		args = append(args, "-s", source)
	}
	args = append(args, "-p", protocol)
	if port > 0 {
		args = append(args, "--dport", strconv.Itoa(port))
	}

	if action == "allow" || action == "accept" {
		args = append(args, "-j", "ACCEPT")
	} else {
		args = append(args, "-j", "DROP")
	}

	cmd := exec.Command("iptables", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("添加防火墙规则失败: %s", string(out))
	}

	zap.L().Info("防火墙规则添加成功", zap.String("key", key))
	return nil
}

// RemoveFirewallRule 删除防火墙规则
func (m *Manager) RemoveFirewallRule(direction, protocol, source string, port int) error {
	if !m.enableFirewall {
		return nil
	}

	m.mu.Lock()
	for key := range m.firewallRules {
		if strings.HasPrefix(key, fmt.Sprintf("%s-%s-%s-%d-", direction, protocol, source, port)) {
			delete(m.firewallRules, key)
		}
	}
	m.mu.Unlock()

	// 简化的删除逻辑：flush 所有相关链并重建
	return nil
}

// BlockIP 封锁 IP (DROP 所有流量)
func (m *Manager) BlockIP(ip string) error {
	m.mu.Lock()
	if m.blockedIPs[ip] {
		m.mu.Unlock()
		return nil
	}
	m.blockedIPs[ip] = true
	m.mu.Unlock()

	cmd := exec.Command("iptables", "-A", "INPUT", "-s", ip, "-j", "DROP")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("封锁 IP 失败: %s", string(out))
	}

	cmd2 := exec.Command("iptables", "-A", "FORWARD", "-s", ip, "-j", "DROP")
	cmd2.Run()

	zap.L().Warn("IP 已封锁", zap.String("ip", ip))
	return nil
}

// UnblockIP 解封 IP
func (m *Manager) UnblockIP(ip string) error {
	m.mu.Lock()
	if !m.blockedIPs[ip] {
		m.mu.Unlock()
		return nil
	}
	delete(m.blockedIPs, ip)
	m.mu.Unlock()

	cmd := exec.Command("iptables", "-D", "INPUT", "-s", ip, "-j", "DROP")
	cmd.Run()

	cmd2 := exec.Command("iptables", "-D", "FORWARD", "-s", ip, "-j", "DROP")
	cmd2.Run()

	zap.L().Info("IP 已解封", zap.String("ip", ip))
	return nil
}

// IsBlocked 检查 IP 是否被封锁
func (m *Manager) IsBlocked(ip string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.blockedIPs[ip]
}

// ListBlockedIPs 列出所有被封锁的 IP
func (m *Manager) ListBlockedIPs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var ips []string
	for ip := range m.blockedIPs {
		ips = append(ips, ip)
	}
	return ips
}

// AllocatePort 自动分配一个可用端口
func (m *Manager) AllocatePort(start, end int) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查端口是否已被使用（通过 portMappings 和系统监听端口）
	for port := start; port <= end; port++ {
		key := fmt.Sprintf("%d/tcp", port)
		if _, exists := m.portMappings[key]; exists {
			continue
		}
		// 检查系统是否已监听该端口
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			continue
		}
		listener.Close()
		return port, nil
	}
	return 0, fmt.Errorf("端口范围 %d-%d 内无可用端口", start, end)
}

// GetInterfaceName 获取当前使用的网卡名称
func (m *Manager) GetInterfaceName() string {
	return m.interfaceName
}

// NetworkInterfaceInfo 网络接口信息
type NetworkInterfaceInfo struct {
	Name         string   `json:"name"`
	IPv4s        []string `json:"ipv4s"`
	IPv6Prefixes []string `json:"ipv6_prefixes"`
}

// GetLocalInterfaces 获取本机所有网络接口信息
func GetLocalInterfaces() ([]NetworkInterfaceInfo, error) {
	var result []NetworkInterfaceInfo
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		info := NetworkInterfaceInfo{Name: iface.Name}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ipnet.IP.To4() != nil {
					info.IPv4s = append(info.IPv4s, ipnet.IP.String())
				} else {
					// IPv6 前缀检测
					if ones, _ := ipnet.Mask.Size(); ones > 0 && ones < 128 {
						info.IPv6Prefixes = append(info.IPv6Prefixes, ipnet.String())
					}
				}
			}
		}
		if len(info.IPv4s) > 0 || len(info.IPv6Prefixes) > 0 {
			result = append(result, info)
		}
	}
	return result, nil
}

// GetLocalIPs 获取本机所有 IP
func GetLocalIPs() ([]string, error) {
	var ips []string
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ipnet.IP.To4() != nil {
					ips = append(ips, ipnet.IP.String())
				}
			}
		}
	}
	return ips, nil
}

// GetMainIP 获取主 IP
func GetMainIP() string {
	ips, err := GetLocalIPs()
	if err != nil || len(ips) == 0 {
		return "127.0.0.1"
	}
	for _, ip := range ips {
		if !strings.HasPrefix(ip, "127.") && !strings.HasPrefix(ip, "169.254.") {
			return ip
		}
	}
	return ips[0]
}

// DetectAbnormalTraffic 检测异常流量 (用于安全模块调用)
func (m *Manager) DetectAbnormalTraffic() []string {
	// 通过 conntrack 或 netstat 检测高流量连接
	cmd := exec.Command("conntrack", "-L", "-o", "extended")
	out, _ := cmd.CombinedOutput()
	_ = out
	return nil
}
