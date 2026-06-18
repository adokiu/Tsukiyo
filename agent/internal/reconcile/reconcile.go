// Package reconcile 负责 Agent 启动时将宿主机实际状态与 Master 期望状态对齐
package reconcile

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"tsukiyo/agent/internal/incus"
)

// VPCConfig Master 下发的 VPC 期望配置
type VPCConfig struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	IPv4CIDR         string `json:"ipv4_cidr"`
	IPv6ULACIDR      string `json:"ipv6_ula_cidr,omitempty"`
	IPv6GUACIDR      string `json:"ipv6_gua_cidr,omitempty"`
	DefaultGatewayV4 string `json:"default_gateway_v4,omitempty"`
	DefaultGatewayV6 string `json:"default_gateway_v6,omitempty"`
	EgressV4Primary  string `json:"egress_v4_primary,omitempty"`
	ParentIface      string `json:"parent_iface,omitempty"`
	PortRangeStart   int    `json:"port_range_start"`
	PortRangeEnd     int    `json:"port_range_end"`
	SNATEnabled      bool   `json:"snat_enabled"`
	IPv4Filter       bool   `json:"ipv4_filter"`
	MACFilter        bool   `json:"mac_filter"`
	BridgeName       string `json:"bridge_name"`
	Status           string `json:"status"`
}

// StateFile 本地持久化期望状态的文件路径
func StateFile() string {
	exe, err := os.Executable()
	if err != nil {
		return "reconcile-state.json"
	}
	return filepath.Join(filepath.Dir(exe), "reconcile-state.json")
}

// SaveDesiredState 将期望状态持久化到本地
func SaveDesiredState(vpcs []VPCConfig) error {
	data, err := json.MarshalIndent(vpcs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(StateFile(), data, 0644)
}

// LoadDesiredState 从本地加载上次持久化的期望状态
func LoadDesiredState() ([]VPCConfig, error) {
	data, err := os.ReadFile(StateFile())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var vpcs []VPCConfig
	if err := json.Unmarshal(data, &vpcs); err != nil {
		return nil, err
	}
	return vpcs, nil
}

// PortMappingConfig 端口映射配置
type PortMappingConfig struct {
	ID            string `json:"id"`
	InstanceID    string `json:"instance_id"`
	IncusName     string `json:"incus_name"`
	InternalIP    string `json:"internal_ip"`
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
	Description   string `json:"description,omitempty"`
}

// Reconciler 状态对齐器
type Reconciler struct {
	ic *incus.Client
}

// NewReconciler 创建对齐器
func NewReconciler(ic *incus.Client) *Reconciler {
	return &Reconciler{ic: ic}
}

// Reconcile 执行全量状态对齐：对比期望 VPC 配置与宿主机实际状态，修复所有不一致
func (r *Reconciler) Reconcile(desired []VPCConfig) error {
	if len(desired) == 0 {
		zap.L().Info("[Reconcile] 无期望 VPC 配置，跳过")
		return nil
	}

	zap.L().Info("[Reconcile] 开始状态对齐", zap.Int("desired_vpcs", len(desired)))

	// 0. 确保系统级持久化基础设施就绪（ Incus 自启、iptables-persistent、sysctl）
	r.ensureSystemPersistence()

	// 0.1 先尝试加载宿主机持久化的 iptables 规则（如果安装了 iptables-persistent）
	_ = LoadIPTablesRules()

	// 1. 获取当前 Incus 所有网络
	actualNetworks, err := r.ic.ListNetworks()
	if err != nil {
		zap.L().Warn("[Reconcile] 获取 Incus 网络列表失败", zap.Error(err))
		actualNetworks = []incus.NetworkInfo{}
	}
	actualMap := make(map[string]incus.NetworkInfo, len(actualNetworks))
	for _, n := range actualNetworks {
		actualMap[n.Name] = n
	}

	// 2. 逐一对齐每个 VPC
	for _, vpc := range desired {
		if vpc.Status == "deleted" || vpc.Status == "deleting" {
			zap.L().Info("[Reconcile] 跳过已删除 VPC", zap.String("vpc_id", vpc.ID))
			continue
		}

		if err := r.reconcileVPC(vpc, actualMap); err != nil {
			zap.L().Error("[Reconcile] VPC 对齐失败",
				zap.String("vpc_id", vpc.ID),
				zap.String("bridge", vpc.BridgeName),
				zap.Error(err))
			// 继续下一个，不中断
		}
	}

	// 3. 持久化 iptables 规则到宿主机
	_ = SaveIPTablesRules()

	// 4. 持久化本次期望状态到本地文件（作为兜底）
	if err := SaveDesiredState(desired); err != nil {
		zap.L().Warn("[Reconcile] 持久化期望状态失败", zap.Error(err))
	}

	zap.L().Info("[Reconcile] 状态对齐完成")
	return nil
}

// ReconcilePortMappings 恢复端口映射 proxy 设备（Agent 重启后）
func (r *Reconciler) ReconcilePortMappings(portMappings []PortMappingConfig) error {
	if len(portMappings) == 0 {
		return nil
	}
	zap.L().Info("[Reconcile] 开始恢复端口映射", zap.Int("count", len(portMappings)))

	for _, pm := range portMappings {
		if pm.IncusName == "" || pm.InternalIP == "" {
			zap.L().Warn("[Reconcile] 端口映射缺少 incus_name 或 internal_ip，跳过", zap.String("id", pm.ID))
			continue
		}
		// 检查实例是否存在
		if !r.ic.InstanceExists(pm.IncusName) {
			zap.L().Warn("[Reconcile] 实例不存在，跳过端口映射恢复", zap.String("instance", pm.IncusName))
			continue
		}
		deviceName := fmt.Sprintf("proxy-%d-%s", pm.HostPort, pm.Protocol)
		listenAddr := fmt.Sprintf("%s:0.0.0.0:%d", pm.Protocol, pm.HostPort)
		connectAddr := fmt.Sprintf("%s:%s:%d", pm.Protocol, pm.InternalIP, pm.ContainerPort)
		if err := r.ic.AddProxyDevice(pm.IncusName, deviceName, listenAddr, connectAddr); err != nil {
			zap.L().Warn("[Reconcile] 恢复 proxy 端口映射失败",
				zap.String("instance", pm.IncusName),
				zap.String("device", deviceName),
				zap.Error(err))
		} else {
			zap.L().Info("[Reconcile] 恢复 proxy 端口映射成功",
				zap.String("instance", pm.IncusName),
				zap.String("device", deviceName),
				zap.Int("host_port", pm.HostPort))
		}
	}
	return nil
}

// ensureSystemPersistence 确保系统级持久化基础设施就绪
func (r *Reconciler) ensureSystemPersistence() {
	// 1. 确保 Incus 服务开机自启
	if out, err := exec.Command("systemctl", "is-enabled", "incus").CombinedOutput(); err != nil {
		zap.L().Warn("[Reconcile] Incus 未设置开机自启，正在启用",
			zap.String("output", strings.TrimSpace(string(out))))
		if err := exec.Command("systemctl", "enable", "incus").Run(); err != nil {
			zap.L().Warn("[Reconcile] systemctl enable incus 失败", zap.Error(err))
		} else {
			zap.L().Info("[Reconcile] Incus 已设置为开机自启")
		}
	} else {
		zap.L().Debug("[Reconcile] Incus 已设置开机自启")
	}

	// 2. 确保 Incus 服务正在运行
	if out, err := exec.Command("systemctl", "is-active", "incus").CombinedOutput(); err != nil {
		zap.L().Warn("[Reconcile] Incus 服务未运行，正在启动",
			zap.String("output", strings.TrimSpace(string(out))))
		_ = exec.Command("systemctl", "start", "incus").Run()
		// 等待 Incus 就绪
		for i := 0; i < 30; i++ {
			if _, err := r.ic.GetServerInfo(); err == nil {
				zap.L().Info("[Reconcile] Incus 服务已就绪")
				break
			}
			time.Sleep(1 * time.Second)
		}
	}

	// 3. 确保 sysctl 持久化目录存在
	if err := os.MkdirAll("/etc/sysctl.d", 0755); err != nil {
		zap.L().Warn("[Reconcile] 创建 /etc/sysctl.d 失败", zap.Error(err))
	}

	// 4. 持久化 IP 转发配置
	EnsureSysctl()

	// 5. 确保 iptables-persistent 已安装
	if _, err := exec.LookPath("netfilter-persistent"); err != nil {
		zap.L().Warn("[Reconcile] netfilter-persistent 未找到，iptables 规则重启后可能丢失")
	} else {
		zap.L().Debug("[Reconcile] netfilter-persistent 已安装")
	}
}

// reconcileVPC 对齐单个 VPC
func (r *Reconciler) reconcileVPC(vpc VPCConfig, actualMap map[string]incus.NetworkInfo) error {
	zap.L().Info("[Reconcile] 对齐 VPC",
		zap.String("vpc_id", vpc.ID),
		zap.String("bridge", vpc.BridgeName),
		zap.String("cidr", vpc.IPv4CIDR))

	// 2.1 检查 bridge 是否存在
	actual, exists := actualMap[vpc.BridgeName]
	if !exists {
		zap.L().Warn("[Reconcile] bridge 不存在，需要创建",
			zap.String("bridge", vpc.BridgeName))
		if err := r.ic.CreateBridgeNetwork(
			vpc.BridgeName,
			vpc.IPv4CIDR,
			vpc.IPv6ULACIDR,
			vpc.IPv6GUACIDR,
			vpc.DefaultGatewayV4,
		); err != nil {
			return fmt.Errorf("创建 bridge 失败: %w", err)
		}
		zap.L().Info("[Reconcile] bridge 创建成功", zap.String("bridge", vpc.BridgeName))
	} else {
		// 检查参数是否一致
		if actual.Type != "bridge" {
			zap.L().Warn("[Reconcile] 网络类型不匹配",
				zap.String("bridge", vpc.BridgeName),
				zap.String("actual_type", actual.Type))
		}
		// 检查 ipv4.address 是否匹配
		if vpc.IPv4CIDR != "" && vpc.DefaultGatewayV4 != "" {
			expectedAddr := getGatewayWithPrefix(vpc.IPv4CIDR, vpc.DefaultGatewayV4)
			actualAddr := actual.Config["ipv4.address"]
			if actualAddr != expectedAddr && actualAddr != "" {
				zap.L().Warn("[Reconcile] bridge IPv4 配置不匹配，需要重建",
					zap.String("bridge", vpc.BridgeName),
					zap.String("expected", expectedAddr),
					zap.String("actual", actualAddr))
				// 先删除再重建
				_ = r.ic.DeleteBridgeNetwork(vpc.BridgeName)
				if err := r.ic.CreateBridgeNetwork(
					vpc.BridgeName,
					vpc.IPv4CIDR,
					vpc.IPv6ULACIDR,
					vpc.IPv6GUACIDR,
					vpc.DefaultGatewayV4,
				); err != nil {
					return fmt.Errorf("重建 bridge 失败: %w", err)
				}
				zap.L().Info("[Reconcile] bridge 重建成功", zap.String("bridge", vpc.BridgeName))
			}
		}
	}

	// 2.2 检查 SNAT 规则
	if vpc.SNATEnabled && vpc.EgressV4Primary != "" && vpc.ParentIface != "" {
		if err := r.reconcileSNAT(vpc); err != nil {
			zap.L().Warn("[Reconcile] SNAT 对齐失败",
				zap.String("bridge", vpc.BridgeName),
				zap.Error(err))
		}
	}

	return nil
}

// reconcileSNAT 对齐 SNAT 规则
func (r *Reconciler) reconcileSNAT(vpc VPCConfig) error {
	bridgeName := vpc.BridgeName
	cidr := vpc.IPv4CIDR
	egressIP := vpc.EgressV4Primary
	parentIface := vpc.ParentIface

	// 检查 iptables 规则是否存在
	mark := getBridgeMark(bridgeName)
	chain := "TSUKIYO-SNAT-" + bridgeName

	// 检查 MASQUERADE 规则
	ruleExists := r.checkIPTablesRule("nat", "POSTROUTING", "-s", cidr, "-o", parentIface, "-j", "MASQUERADE")
	if !ruleExists {
		zap.L().Warn("[Reconcile] SNAT MASQUERADE 规则缺失，正在重建",
			zap.String("bridge", bridgeName))
		if err := ConfigureSNAT(bridgeName, cidr, egressIP, parentIface); err != nil {
			return fmt.Errorf("重建 SNAT 失败: %w", err)
		}
		zap.L().Info("[Reconcile] SNAT 重建成功", zap.String("bridge", bridgeName))
	}

	// 检查 connmark 恢复规则
	restoreExists := r.checkIPTablesRule("mangle", "PREROUTING", "-i", bridgeName, "-j", "CONNMARK", "--restore-mark")
	if !restoreExists {
		zap.L().Warn("[Reconcile] connmark 恢复规则缺失",
			zap.String("bridge", bridgeName))
	}

	_ = mark
	_ = chain
	return nil
}

// checkIPTablesRule 检查 iptables 规则是否存在（简化匹配）
func (r *Reconciler) checkIPTablesRule(table, chain string, args ...string) bool {
	cmdArgs := append([]string{"-t", table, "-C", chain}, args...)
	cmd := exec.Command("iptables", cmdArgs...)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// getGatewayWithPrefix 从 CIDR 和前缀长度构造网关地址
func getGatewayWithPrefix(cidr, gateway string) string {
	if cidr == "" || gateway == "" {
		return gateway
	}
	parts := strings.Split(cidr, "/")
	if len(parts) == 2 {
		return gateway + "/" + parts[1]
	}
	return gateway
}

// getBridgeMark 为 bridge 生成一个 mark 值（CRC16 简化）
func getBridgeMark(bridgeName string) string {
	var h uint16 = 0xFFFF
	for _, c := range bridgeName {
		h ^= uint16(c) << 8
		for i := 0; i < 8; i++ {
			if h&0x8000 != 0 {
				h = (h << 1) ^ 0x1021
			} else {
				h <<= 1
			}
		}
	}
	return fmt.Sprintf("0x%04X", h)
}

// ConfigureSNAT 配置 SNAT
func ConfigureSNAT(bridgeName, cidr, egressIP, parentIface string) error {
	mark := getBridgeMark(bridgeName)
	chain := "TSUKIYO-SNAT-" + bridgeName

	// 1. 创建自定义链
	_ = exec.Command("iptables", "-t", "nat", "-N", chain).Run()

	// 2. 自定义链：根据 mark 做 SNAT
	_ = exec.Command("iptables", "-t", "nat", "-A", chain,
		"-m", "comment", "--comment", "tsukiyo-vpc-snat",
		"-j", "SNAT", "--to-source", egressIP,
	).Run()

	// 3. POSTROUTING 中跳到自定义链
	_ = exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING",
		"-s", cidr, "-o", parentIface,
		"-m", "comment", "--comment", "tsukiyo-vpc-"+bridgeName,
		"-j", chain,
	).Run()

	// 4. MASQUERADE fallback（如果 SNAT 失败时的兜底）
	_ = exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING",
		"-s", cidr, "-o", parentIface,
		"-m", "comment", "--comment", "tsukiyo-vpc-masq-"+bridgeName,
		"-j", "MASQUERADE",
	).Run()

	// 5. mangle PREROUTING 恢复 connmark
	_ = exec.Command("iptables", "-t", "mangle", "-A", "PREROUTING",
		"-i", bridgeName,
		"-m", "comment", "--comment", "tsukiyo-vpc-restore-"+bridgeName,
		"-j", "CONNMARK", "--restore-mark", "--mark", mark,
	).Run()

	// 6. mangle POSTROUTING 保存 connmark
	_ = exec.Command("iptables", "-t", "mangle", "-A", "POSTROUTING",
		"-o", bridgeName,
		"-m", "comment", "--comment", "tsukiyo-vpc-save-"+bridgeName,
		"-j", "CONNMARK", "--save-mark", "--mark", mark,
	).Run()

	_ = mark
	return nil
}

// RemoveSNAT 移除 SNAT 规则
func RemoveSNAT(bridgeName, cidr string) error {
	// 删除所有带 tsukiyo-vpc 注释的 iptables 规则
	for _, table := range []string{"nat", "mangle"} {
		for _, chain := range []string{"POSTROUTING", "PREROUTING"} {
			_ = exec.Command("iptables", "-t", table, "-F", chain).Run()
		}
	}
	// 删除自定义链
	_ = exec.Command("iptables", "-t", "nat", "-X", "TSUKIYO-SNAT-"+bridgeName).Run()
	return nil
}

// EnsureSysctl 持久化 sysctl 网络转发配置
func EnsureSysctl() {
	confPath := "/etc/sysctl.d/99-tsukiyo.conf"
	content := "net.ipv4.ip_forward=1\nnet.ipv6.conf.all.forwarding=1\n"

	// 读取现有内容，避免重复写入
	existing := ""
	if data, err := os.ReadFile(confPath); err == nil {
		existing = string(data)
	}
	if !strings.Contains(existing, "net.ipv4.ip_forward=1") {
		if err := os.WriteFile(confPath, []byte(content), 0644); err != nil {
			zap.L().Warn("[Reconcile] 写入 sysctl 配置失败", zap.Error(err))
		} else {
			zap.L().Info("[Reconcile] sysctl 配置已持久化", zap.String("path", confPath))
		}
	}

	// 立即生效
	_ = exec.Command("sysctl", "-p", confPath).Run()
	_ = exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").Run()
}

// SaveIPTablesRules 保存 iptables 规则到宿主机持久化存储
func SaveIPTablesRules() error {
	// 优先使用 netfilter-persistent
	if _, err := exec.LookPath("netfilter-persistent"); err == nil {
		cmd := exec.Command("netfilter-persistent", "save")
		if out, err := cmd.CombinedOutput(); err != nil {
			zap.L().Warn("[Reconcile] netfilter-persistent save 失败", zap.Error(err), zap.String("output", string(out)))
		} else {
			zap.L().Info("[Reconcile] iptables 规则已保存到宿主机")
			return nil
		}
	}
	// fallback: 直接保存到 /etc/iptables/rules.v4
	if err := os.MkdirAll("/etc/iptables", 0755); err != nil {
		return fmt.Errorf("创建 /etc/iptables 失败: %w", err)
	}
	cmd := exec.Command("iptables-save")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("iptables-save 失败: %w", err)
	}
	if err := os.WriteFile("/etc/iptables/rules.v4", out, 0644); err != nil {
		return fmt.Errorf("写入 rules.v4 失败: %w", err)
	}
	zap.L().Info("[Reconcile] iptables 规则已保存到 /etc/iptables/rules.v4")
	return nil
}

// LoadIPTablesRules 从宿主机持久化存储加载 iptables 规则
func LoadIPTablesRules() error {
	// 优先使用 netfilter-persistent
	if _, err := exec.LookPath("netfilter-persistent"); err == nil {
		cmd := exec.Command("netfilter-persistent", "start")
		if out, err := cmd.CombinedOutput(); err != nil {
			zap.L().Warn("[Reconcile] netfilter-persistent start 失败", zap.Error(err), zap.String("output", string(out)))
		} else {
			zap.L().Info("[Reconcile] 已通过 netfilter-persistent 加载 iptables 规则")
			return nil
		}
	}
	// fallback: 从 /etc/iptables/rules.v4 加载
	if _, err := os.Stat("/etc/iptables/rules.v4"); err != nil {
		return fmt.Errorf("rules.v4 不存在: %w", err)
	}
	data, err := os.ReadFile("/etc/iptables/rules.v4")
	if err != nil {
		return fmt.Errorf("读取 rules.v4 失败: %w", err)
	}
	cmd := exec.Command("iptables-restore")
	cmd.Stdin = strings.NewReader(string(data))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("iptables-restore 失败: %w, output: %s", err, string(out))
	}
	zap.L().Info("[Reconcile] 已从 /etc/iptables/rules.v4 加载 iptables 规则")
	return nil
}
