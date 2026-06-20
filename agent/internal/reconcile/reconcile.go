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

// BridgeConfig Master 下发的 Bridge 期望配置
type BridgeConfig struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	BridgeName     string   `json:"bridge_name"`
	IPv4Enabled    bool     `json:"ipv4_enabled"`
	IPv4CIDR       string   `json:"ipv4_cidr"`
	IPv4Gateway    string   `json:"ipv4_gateway"`
	IPv6Enabled    bool     `json:"ipv6_enabled"`
	IPv6CIDR       string   `json:"ipv6_cidr"`
	IPv6Gateway    string   `json:"ipv6_gateway"`
	DNSServers     []string `json:"dns_servers"`
	NATEgressIPv4  string   `json:"nat_egress_ipv4,omitempty"`
	NATEgressIPv6  string   `json:"nat_egress_ipv6,omitempty"`
	PortRangeStart int      `json:"port_range_start"`
	PortRangeEnd   int      `json:"port_range_end"`
	Status         string   `json:"status"`
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
func SaveDesiredState(bridges []BridgeConfig) error {
	data, err := json.MarshalIndent(bridges, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(StateFile(), data, 0644)
}

// LoadDesiredState 从本地加载上次持久化的期望状态
func LoadDesiredState() ([]BridgeConfig, error) {
	data, err := os.ReadFile(StateFile())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var bridges []BridgeConfig
	if err := json.Unmarshal(data, &bridges); err != nil {
		return nil, err
	}
	return bridges, nil
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
	IPVersion     string `json:"ip_version"`
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

// Reconcile 执行全量状态对齐：对比期望 Bridge 配置与宿主机实际状态，修复所有不一致
func (r *Reconciler) Reconcile(desired []BridgeConfig) error {
	if len(desired) == 0 {
		zap.L().Info("[Reconcile] 无期望 Bridge 配置，跳过")
		return nil
	}

	zap.L().Info("[Reconcile] 开始状态对齐", zap.Int("desired_bridges", len(desired)))

	// 0. 确保系统级持久化基础设施就绪
	r.ensureSystemPersistence()

	if !r.ic.IsAvailable() {
		zap.L().Warn("[Reconcile] Incus 不可用，跳过网络对齐")
		return nil
	}

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

	// 2. 逐一对齐每个 Bridge
	for _, bridge := range desired {
		if bridge.Status == "deleted" || bridge.Status == "deleting" {
			zap.L().Info("[Reconcile] 跳过已删除 Bridge", zap.String("bridge_id", bridge.ID))
			continue
		}

		if err := r.reconcileBridge(bridge, actualMap); err != nil {
			zap.L().Error("[Reconcile] Bridge 对齐失败",
				zap.String("bridge_id", bridge.ID),
				zap.String("bridge", bridge.BridgeName),
				zap.Error(err))
		}
	}

	// 3. 持久化本次期望状态
	if err := SaveDesiredState(desired); err != nil {
		zap.L().Warn("[Reconcile] 持久化期望状态失败", zap.Error(err))
	}

	// 4. 检查所有现有 bridge 网络的 ipv4.nat
	r.EnsureAllBridgeNAT()

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

		// 检查 proxy 设备是否已存在
		if exists, err := r.ic.DeviceExists(pm.IncusName, deviceName); err == nil && exists {
			zap.L().Debug("[Reconcile] proxy 设备已存在，跳过", zap.String("instance", pm.IncusName), zap.String("device", deviceName))
			continue
		}

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

}

// reconcileBridge 对齐单个 Bridge
func (r *Reconciler) reconcileBridge(bridge BridgeConfig, actualMap map[string]incus.NetworkInfo) error {
	zap.L().Info("[Reconcile] 对齐 Bridge",
		zap.String("bridge_id", bridge.ID),
		zap.String("bridge", bridge.BridgeName),
		zap.String("cidr", bridge.IPv4CIDR))

	actual, exists := actualMap[bridge.BridgeName]
	if !exists {
		zap.L().Info("[Reconcile] bridge 不存在，创建",
			zap.String("bridge", bridge.BridgeName))
		if err := r.ic.CreateBridgeNetwork(
			bridge.BridgeName,
			bridge.IPv4CIDR,
			bridge.IPv6CIDR,
			"",
			bridge.IPv4Gateway,
		); err != nil {
			return fmt.Errorf("创建 bridge 失败: %w", err)
		}
		zap.L().Info("[Reconcile] bridge 创建成功", zap.String("bridge", bridge.BridgeName))

		// 创建后立即设置 NAT 出口 IP
		if bridge.IPv4Enabled && bridge.NATEgressIPv4 != "" {
			egressIP := bridge.NATEgressIPv4
			if idx := strings.Index(egressIP, "/"); idx > 0 {
				egressIP = egressIP[:idx]
			}
			if err := r.ic.UpdateBridgeNetwork(bridge.BridgeName, map[string]string{
				"ipv4.nat":         "true",
				"ipv4.nat.address": egressIP,
			}); err != nil {
				zap.L().Error("[Reconcile] 创建后设置 ipv4.nat.address 失败", zap.Error(err))
			} else {
				zap.L().Info("[Reconcile] 创建后已设置 ipv4.nat.address", zap.String("bridge", bridge.BridgeName), zap.String("egress_ip", egressIP))
			}
		}
		if bridge.IPv6Enabled && bridge.NATEgressIPv6 != "" {
			egressIP := bridge.NATEgressIPv6
			if idx := strings.Index(egressIP, "/"); idx > 0 {
				egressIP = egressIP[:idx]
			}
			if err := r.ic.UpdateBridgeNetwork(bridge.BridgeName, map[string]string{
				"ipv6.nat":         "true",
				"ipv6.nat.address": egressIP,
			}); err != nil {
				zap.L().Error("[Reconcile] 创建后设置 ipv6.nat.address 失败", zap.Error(err))
			} else {
				zap.L().Info("[Reconcile] 创建后已设置 ipv6.nat.address", zap.String("bridge", bridge.BridgeName), zap.String("egress_ip", egressIP))
			}
		}
	} else {
		if actual.Type != "bridge" {
			zap.L().Warn("[Reconcile] 网络类型不匹配",
				zap.String("bridge", bridge.BridgeName),
				zap.String("actual_type", actual.Type))
		}

		// 检查 ipv4.address 是否匹配
		if bridge.IPv4Enabled && bridge.IPv4CIDR != "" && bridge.IPv4Gateway != "" {
			expectedAddr := getGatewayWithPrefix(bridge.IPv4CIDR, bridge.IPv4Gateway)
			actualAddr := actual.Config["ipv4.address"]
			if actualAddr != expectedAddr && actualAddr != "" {
				zap.L().Warn("[Reconcile] bridge IPv4 配置不匹配，重建",
					zap.String("bridge", bridge.BridgeName),
					zap.String("expected", expectedAddr),
					zap.String("actual", actualAddr))
				_ = r.ic.DeleteBridgeNetwork(bridge.BridgeName)
				if err := r.ic.CreateBridgeNetwork(
					bridge.BridgeName,
					bridge.IPv4CIDR,
					bridge.IPv6CIDR,
					"",
					bridge.IPv4Gateway,
				); err != nil {
					return fmt.Errorf("重建 bridge 失败: %w", err)
				}
				zap.L().Info("[Reconcile] bridge 重建成功", zap.String("bridge", bridge.BridgeName))
			}
		}

		// 确保 ipv4.nat=true
		if bridge.IPv4Enabled && actual.Config["ipv4.nat"] != "true" {
			zap.L().Warn("[Reconcile] bridge ipv4.nat 未启用，修复",
				zap.String("bridge", bridge.BridgeName))
			if err := r.ic.UpdateBridgeNetwork(bridge.BridgeName, map[string]string{"ipv4.nat": "true"}); err != nil {
				zap.L().Error("[Reconcile] 更新 ipv4.nat 失败", zap.Error(err))
			} else {
				zap.L().Info("[Reconcile] ipv4.nat 已启用", zap.String("bridge", bridge.BridgeName))
			}
		}

		// 检查 ipv4.nat.address 是否匹配
		if bridge.IPv4Enabled && bridge.NATEgressIPv4 != "" {
			expectedEgressIP := bridge.NATEgressIPv4
			if idx := strings.Index(expectedEgressIP, "/"); idx > 0 {
				expectedEgressIP = expectedEgressIP[:idx]
			}
			actualNatAddr := actual.Config["ipv4.nat.address"]
			if actualNatAddr != expectedEgressIP {
				zap.L().Warn("[Reconcile] bridge ipv4.nat.address 不匹配，修复",
					zap.String("bridge", bridge.BridgeName),
					zap.String("expected", expectedEgressIP),
					zap.String("actual", actualNatAddr))
				if err := r.ic.UpdateBridgeNetwork(bridge.BridgeName, map[string]string{
					"ipv4.nat":         "true",
					"ipv4.nat.address": expectedEgressIP,
				}); err != nil {
					zap.L().Error("[Reconcile] 更新 ipv4.nat.address 失败", zap.Error(err))
				} else {
					zap.L().Info("[Reconcile] ipv4.nat.address 已修复", zap.String("bridge", bridge.BridgeName), zap.String("egress_ip", expectedEgressIP))
				}
			}
		} else if bridge.IPv4Enabled && bridge.NATEgressIPv4 == "" && actual.Config["ipv4.nat.address"] != "" {
			// 期望没有出口 IP 但实际有，清除
			zap.L().Warn("[Reconcile] bridge ipv4.nat.address 应为空，清除",
				zap.String("bridge", bridge.BridgeName),
				zap.String("actual", actual.Config["ipv4.nat.address"]))
			if err := r.ic.UpdateBridgeNetwork(bridge.BridgeName, map[string]string{
				"ipv4.nat.address": "",
			}); err != nil {
				zap.L().Error("[Reconcile] 清除 ipv4.nat.address 失败", zap.Error(err))
			}
		}

		// 检查 ipv6.nat.address 是否匹配
		if bridge.IPv6Enabled && bridge.NATEgressIPv6 != "" {
			expectedEgressIP := bridge.NATEgressIPv6
			if idx := strings.Index(expectedEgressIP, "/"); idx > 0 {
				expectedEgressIP = expectedEgressIP[:idx]
			}
			actualNatAddr := actual.Config["ipv6.nat.address"]
			if actualNatAddr != expectedEgressIP {
				zap.L().Warn("[Reconcile] bridge ipv6.nat.address 不匹配，修复",
					zap.String("bridge", bridge.BridgeName),
					zap.String("expected", expectedEgressIP),
					zap.String("actual", actualNatAddr))
				if err := r.ic.UpdateBridgeNetwork(bridge.BridgeName, map[string]string{
					"ipv6.nat":         "true",
					"ipv6.nat.address": expectedEgressIP,
				}); err != nil {
					zap.L().Error("[Reconcile] 更新 ipv6.nat.address 失败", zap.Error(err))
				} else {
					zap.L().Info("[Reconcile] ipv6.nat.address 已修复", zap.String("bridge", bridge.BridgeName), zap.String("egress_ip", expectedEgressIP))
				}
			}
		} else if bridge.IPv6Enabled && bridge.NATEgressIPv6 == "" && actual.Config["ipv6.nat.address"] != "" {
			zap.L().Warn("[Reconcile] bridge ipv6.nat.address 应为空，清除",
				zap.String("bridge", bridge.BridgeName),
				zap.String("actual", actual.Config["ipv6.nat.address"]))
			if err := r.ic.UpdateBridgeNetwork(bridge.BridgeName, map[string]string{
				"ipv6.nat.address": "",
			}); err != nil {
				zap.L().Error("[Reconcile] 清除 ipv6.nat.address 失败", zap.Error(err))
			}
		}
	}

	return nil
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

// EnsureAllBridgeNAT 确保所有现有 bridge 网络都启用了 NAT
func (r *Reconciler) EnsureAllBridgeNAT() {
	if !r.ic.IsAvailable() {
		return
	}
	networks, err := r.ic.ListNetworks()
	if err != nil {
		if r.ic.IsAvailable() {
			zap.L().Warn("[Reconcile] 获取网络列表失败", zap.Error(err))
		}
		return
	}

	for _, net := range networks {
		if net.Type != "bridge" {
			continue
		}
		if net.Config["ipv4.nat"] != "true" {
			zap.L().Warn("[Reconcile] 检测到 bridge 未启用 NAT，正在修复",
				zap.String("bridge", net.Name),
				zap.String("current", net.Config["ipv4.nat"]))
			if err := r.ic.UpdateBridgeNetwork(net.Name, map[string]string{"ipv4.nat": "true"}); err != nil {
				zap.L().Error("[Reconcile] 更新 bridge NAT 失败",
					zap.String("bridge", net.Name),
					zap.Error(err))
			} else {
				zap.L().Info("[Reconcile] bridge NAT 已启用",
					zap.String("bridge", net.Name))
			}
		}
	}
}
