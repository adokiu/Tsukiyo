package system

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// probeNICs 探测网卡信息（动态数据，每次采集）
func probeNICs() []NetworkInfo {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return nil
	}

	ipv4, ipv6 := detectInterfaceIPs()
	nics := make([]NetworkInfo, 0)

	for _, entry := range entries {
		name := entry.Name()
		if name == "lo" {
			continue
		}

		base := filepath.Join("/sys/class/net", name)
		speed, _ := strconv.Atoi(strings.TrimSpace(readFirstExistingFile(filepath.Join(base, "speed"))))

		nic := NetworkInfo{
			Name:      name,
			MAC:       strings.TrimSpace(readFirstExistingFile(filepath.Join(base, "address"))),
			State:     strings.TrimSpace(readFirstExistingFile(filepath.Join(base, "operstate"))),
			SpeedMbps: speed,
			Driver: strings.TrimSpace(runCommandOutput(2*time.Second, "sh", "-c",
				fmt.Sprintf("basename $(readlink -f /sys/class/net/%s/device/driver 2>/dev/null) 2>/dev/null", shellQuoteSimple(name)))),
			Model: detectNICModel(name),
			IPv4:  ipv4[name],
			IPv6:  ipv6[name],
		}
		nics = append(nics, nic)
	}

	sort.Slice(nics, func(i, j int) bool { return nics[i].Name < nics[j].Name })
	return nics
}

func detectNICModel(name string) string {
	out := runCommandOutput(2*time.Second, "sh", "-c",
		fmt.Sprintf("lspci -D 2>/dev/null | grep -iE 'ethernet|network' | head -n 1 || true"))
	if out != "" {
		return strings.TrimSpace(out)
	}
	return strings.TrimSpace(readFirstExistingFile(filepath.Join("/sys/class/net", name, "device", "uevent")))
}

func detectInterfaceIPs() (map[string][]IPProbe, map[string][]IPProbe) {
	ipv4 := map[string][]IPProbe{}
	ipv6 := map[string][]IPProbe{}

	// 探测网关：路由表 + DHCP 租约
	v4Gateways := detectDefaultGateways("-4")
	v6Gateways := detectDefaultGateways("-6")
	dhcpGateways := detectDhcpGateways()
	// 合并 DHCP 网关，补充路由表没覆盖的网卡
	for iface, gw := range dhcpGateways {
		if _, exists := v4Gateways[iface]; !exists {
			v4Gateways[iface] = gw
		}
	}

	for _, family := range []struct {
		arg      string
		dst      *map[string][]IPProbe
		gateways map[string]string
	}{{"-4", &ipv4, v4Gateways}, {"-6", &ipv6, v6Gateways}} {
		out := runCommandOutput(3*time.Second, "ip", "-o", family.arg, "addr", "show")
		for _, line := range strings.Split(out, "\n") {
			fields := strings.Fields(line)
			if len(fields) < 4 {
				continue
			}
			iface := strings.TrimSuffix(fields[1], ":")
			addr := fields[3]
			ip, network, err := net.ParseCIDR(addr)
			if err != nil || ip == nil || network == nil {
				continue
			}
			ones, _ := network.Mask.Size()
			scope := ""
			for i, field := range fields {
				if field == "scope" && i+1 < len(fields) {
					scope = fields[i+1]
				}
			}
			gw := family.gateways[iface]
			(*family.dst)[iface] = append((*family.dst)[iface], IPProbe{
				Interface: iface,
				Address:   ip.String(),
				PrefixLen: ones,
				Scope:     scope,
				Gateway:   gw,
			})
		}
	}
	return ipv4, ipv6
}

// detectDefaultGateways 探测网关，返回 iface -> gateway 映射
// 优先取默认路由网关，没有则取该网卡上所有路由的 via 网关
func detectDefaultGateways(familyArg string) map[string]string {
	result := map[string]string{}
	// 先查默认路由
	defaultOut := runCommandOutput(3*time.Second, "ip", "-o", familyArg, "route", "show", "default")
	for _, line := range strings.Split(defaultOut, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		if fields[0] != "default" || fields[1] != "via" {
			continue
		}
		gw := fields[2]
		iface := ""
		for i, field := range fields {
			if field == "dev" && i+1 < len(fields) {
				iface = fields[i+1]
				break
			}
		}
		if iface != "" && gw != "" {
			result[iface] = gw
		}
	}
	// 再查所有路由，补充没有默认路由的网卡的网关
	allOut := runCommandOutput(3*time.Second, "ip", "-o", familyArg, "route", "show")
	for _, line := range strings.Split(allOut, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		gw := ""
		iface := ""
		for i, field := range fields {
			if field == "via" && i+1 < len(fields) {
				gw = fields[i+1]
			}
			if field == "dev" && i+1 < len(fields) {
				iface = fields[i+1]
			}
		}
		if iface != "" && gw != "" {
			if _, exists := result[iface]; !exists {
				result[iface] = gw
			}
		}
	}
	return result
}

// detectDhcpGateways 从 DHCP 租约文件中读取网关，返回 iface -> gateway 映射
func detectDhcpGateways() map[string]string {
	result := map[string]string{}

	// dhclient 租约文件: /var/lib/dhcp/dhclient*.leases
	// systemd-networkd 租约文件: /run/systemd/netif/leases/*
	dhcpDirs := []string{"/var/lib/dhcp", "/run/systemd/netif/leases"}
	for _, dir := range dhcpDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.Contains(name, "lease") {
				continue
			}
			iface, gw := parseDhcpLeaseFile(filepath.Join(dir, name))
			if iface != "" && gw != "" {
				if _, exists := result[iface]; !exists {
					result[iface] = gw
				}
			}
		}
	}
	return result
}

// parseDhcpLeaseFile 从租约文件中解析网卡名和网关
func parseDhcpLeaseFile(path string) (string, string) {
	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	iface := ""
	gateway := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// systemd-networkd leases 格式: INTERFACE=eth1
		if strings.HasPrefix(line, "INTERFACE=") {
			iface = strings.Trim(strings.TrimPrefix(line, "INTERFACE="), "\"")
		}
		// systemd-networkd leases 格式: ROUTES=10.18.93.254/32
		if strings.HasPrefix(line, "ROUTES=") {
			val := strings.Trim(strings.TrimPrefix(line, "ROUTES="), "\"")
			parts := strings.Split(val, ",")
			for _, p := range parts {
				ipStr := strings.Split(p, "/")[0]
				if net.ParseIP(ipStr) != nil {
					gateway = ipStr
					break
				}
			}
		}
		// dhclient leases 格式: interface "eth1";
		if strings.HasPrefix(line, "interface") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				iface = strings.Trim(strings.TrimSuffix(fields[1], ";"), "\"")
			}
		}
		// dhclient leases 格式: option routers 10.18.93.254;
		if strings.Contains(line, "option routers") {
			idx := strings.Index(line, "option routers")
			rest := strings.TrimSpace(line[idx+len("option routers"):])
			rest = strings.TrimSuffix(rest, ";")
			rest = strings.TrimSpace(rest)
			if net.ParseIP(rest) != nil {
				gateway = rest
			}
		}
	}
	return iface, gateway
}

func probeAllPublicIPv4(nics []NetworkInfo) []string {
	seen := map[string]bool{}
	result := make([]string, 0)

	out := runCommandOutput(3*time.Second, "ip", "-o", "-4", "addr", "show", "scope", "global")
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		ip, _, err := net.ParseCIDR(fields[3])
		if err != nil || ip == nil {
			continue
		}
		if !isPublicIPv4(ip) {
			continue
		}
		value := ip.String()
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func collectIPv4Addresses(nics []NetworkInfo) []IPProbe {
	result := make([]IPProbe, 0)
	for _, nic := range nics {
		for _, ip := range nic.IPv4 {
			parsed := net.ParseIP(ip.Address)
			if isPublicIPv4(parsed) {
				result = append(result, ip)
			}
		}
	}
	return result
}

func collectIPv6Addresses(nics []NetworkInfo) []IPProbe {
	result := make([]IPProbe, 0)
	for _, nic := range nics {
		for _, ip := range nic.IPv6 {
			if ip.Scope == "global" {
				result = append(result, ip)
			}
		}
	}
	return result
}

// --- IPv4 段 ---

func probeIPv4Prefixes(nics []NetworkInfo, gateways []GatewayProbe) []IPv4Prefix {
	result := make([]IPv4Prefix, 0)
	seen := map[string]bool{}
	gatewayByIface := map[string]string{}
	for _, gateway := range gateways {
		if gateway.Family == "ipv4" && gateway.Interface != "" && gateway.Gateway != "" {
			gatewayByIface[gateway.Interface] = gateway.Gateway
		}
	}

	add := func(item IPv4Prefix) {
		if item.Prefix == "" || item.Interface == "" {
			return
		}
		key := item.Interface + "|" + item.Prefix
		if seen[key] {
			return
		}
		seen[key] = true
		result = append(result, item)
	}

	for _, nic := range nics {
		for _, ip := range nic.IPv4 {
			parsed := net.ParseIP(ip.Address)
			if !isPublicIPv4(parsed) || ip.PrefixLen <= 0 || ip.PrefixLen > 32 {
				continue
			}
			prefix, subnet := ipv4PrefixAndMask(parsed, ip.PrefixLen)
			add(IPv4Prefix{
				Interface:  nic.Name,
				Address:    ip.Address,
				Prefix:     prefix,
				PrefixLen:  ip.PrefixLen,
				SubnetMask: subnet,
				Gateway:    gatewayByIface[nic.Name],
				Source:     "address",
			})
		}
	}

	for _, item := range detectIPv4RoutePrefixes(gatewayByIface) {
		add(item)
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Interface == result[j].Interface {
			return result[i].Prefix < result[j].Prefix
		}
		return result[i].Interface < result[j].Interface
	})
	return result
}

func detectIPv4RoutePrefixes(gatewayByIface map[string]string) []IPv4Prefix {
	out := runCommandOutput(3*time.Second, "ip", "-4", "route", "show")
	result := make([]IPv4Prefix, 0)
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] == "default" {
			continue
		}
		_, network, err := net.ParseCIDR(fields[0])
		if err != nil || network == nil || network.IP.To4() == nil {
			continue
		}
		if !isPublicIPv4(network.IP) {
			continue
		}
		iface := ""
		gateway := ""
		src := ""
		for i, field := range fields {
			if field == "dev" && i+1 < len(fields) {
				iface = fields[i+1]
			}
			if field == "via" && i+1 < len(fields) {
				gateway = fields[i+1]
			}
			if field == "src" && i+1 < len(fields) {
				src = fields[i+1]
			}
		}
		if iface == "" || isContainerLikeInterfaceName(iface) {
			continue
		}
		ones, bits := network.Mask.Size()
		if bits != 32 || ones <= 0 || ones > 32 {
			continue
		}
		if gateway == "" {
			gateway = gatewayByIface[iface]
		}
		prefix, subnet := ipv4PrefixAndMask(network.IP, ones)
		result = append(result, IPv4Prefix{
			Interface:  iface,
			Address:    src,
			Prefix:     prefix,
			PrefixLen:  ones,
			SubnetMask: subnet,
			Gateway:    gateway,
			Source:     "route",
		})
	}
	return result
}

func ipv4PrefixAndMask(ip net.IP, prefixLen int) (string, string) {
	v4 := ip.To4()
	if v4 == nil {
		return "", ""
	}
	mask := net.CIDRMask(prefixLen, 32)
	network := v4.Mask(mask)
	subnet := fmt.Sprintf("%d.%d.%d.%d", mask[0], mask[1], mask[2], mask[3])
	return fmt.Sprintf("%s/%d", network.String(), prefixLen), subnet
}

func isPublicIPv4(ip net.IP) bool {
	ip = ip.To4()
	if ip == nil {
		return false
	}
	return !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsMulticast() && !ip.IsUnspecified()
}

func isContainerLikeInterfaceName(iface string) bool {
	prefixes := []string{"lo", "lxc", "docker", "br-", "veth", "virbr", "cni", "flannel", "cali", "kube", "dummy", "ifb"}
	for _, prefix := range prefixes {
		if iface == prefix || strings.HasPrefix(iface, prefix) {
			return true
		}
	}
	return false
}

// --- IPv6 段 ---

func probeIPv6Prefixes() []IPv6Prefix {
	out := runCommandOutput(3*time.Second, "ip", "-6", "route", "show", "default")
	result := make([]IPv6Prefix, 0)
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		gateway := ""
		iface := ""
		for i, field := range fields {
			if field == "via" && i+1 < len(fields) {
				gateway = fields[i+1]
			}
			if field == "dev" && i+1 < len(fields) {
				iface = fields[i+1]
			}
		}
		addr := fields[0]
		if addr == "default" || addr == "" {
			continue
		}
		var prefixLen int
		if idx := strings.Index(addr, "/"); idx > 0 {
			prefixLen, _ = strconv.Atoi(addr[idx+1:])
			addr = addr[:idx]
		}
		result = append(result, IPv6Prefix{
			Address:   addr,
			PrefixLen: prefixLen,
			Interface: iface,
			Gateway:   gateway,
		})
	}
	return result
}

// --- 网关 ---

func probeGateways() []GatewayProbe {
	gateways := make([]GatewayProbe, 0)
	for _, item := range []struct {
		family string
		args   []string
	}{{"ipv4", []string{"-4", "route", "show", "default"}}, {"ipv6", []string{"-6", "route", "show", "default"}}} {
		out := runCommandOutput(3*time.Second, "ip", item.args...)
		for _, line := range strings.Split(out, "\n") {
			fields := strings.Fields(line)
			gw := ""
			iface := ""
			for i, field := range fields {
				if field == "via" && i+1 < len(fields) {
					gw = fields[i+1]
				}
				if field == "dev" && i+1 < len(fields) {
					iface = fields[i+1]
				}
			}
			if gw != "" || iface != "" {
				gateways = append(gateways, GatewayProbe{Family: item.family, Interface: iface, Gateway: gw})
			}
		}
	}
	return gateways
}
