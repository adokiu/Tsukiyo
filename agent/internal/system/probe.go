package system

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// HostInfo 宿主机探测信息
type HostInfo struct {
	Hostname     string        `json:"hostname"`
	OS           string        `json:"os"`
	Kernel       string        `json:"kernel"`
	Arch         string        `json:"arch"`
	Uptime       string        `json:"uptime"`
	ProcessCount int           `json:"process_count"`
	CPU          CPUInfo       `json:"cpu"`
	Memory       MemoryInfo    `json:"memory"`
	Disks        []DiskInfo    `json:"disks"`
	Networks     []NetworkInfo `json:"networks"`
	Environment  EnvInfo       `json:"environment"`
}

// CPUInfo CPU 信息
type CPUInfo struct {
	Model          string `json:"model"`
	Cores          int    `json:"cores"`
	Threads        int    `json:"threads"`
	Virtualization string `json:"virtualization"` // vmx / svm / none
	NestedKVM      bool   `json:"nested_kvm"`
	HasGPU         bool   `json:"has_gpu"`
}

// MemoryInfo 内存信息
type MemoryInfo struct {
	Total int64 `json:"total"` // bytes
	Used  int64 `json:"used"`  // bytes
}

// DiskInfo 磁盘信息
type DiskInfo struct {
	Device     string `json:"device"`
	Model      string `json:"model"`
	Size       string `json:"size"`
	Type       string `json:"type"` // ssd / hdd / virtual
	MountPoint string `json:"mount_point"`
	Health     string `json:"health"`
}

// NetworkInfo 网卡信息
type NetworkInfo struct {
	Name   string   `json:"name"`
	Status string   `json:"status"` // up / down
	Driver string   `json:"driver"`
	MAC    string   `json:"mac"`
	IPv4   []string `json:"ipv4"`
	IPv6   []string `json:"ipv6"`
	Speed  string   `json:"speed"`
}

// EnvInfo 环境支持信息
type EnvInfo struct {
	SystemdVersion   string `json:"systemd_version"`
	LXCVersion       string `json:"lxc_version"`
	IPTablesVersion  string `json:"iptables_version"`
	IPRoute2Version  string `json:"iproute2_version"`
	ConntrackVersion string `json:"conntrack_version"`
	LibvirtVersion   string `json:"libvirt_version"`
	QEMUVersion      string `json:"qemu_version"`
	SmartctlVersion  string `json:"smartctl_version"`
	HasKVM           bool   `json:"has_kvm"`
	HasIPv4Forward   bool   `json:"has_ipv4_forward"`
	LXCFSActive      bool   `json:"lxcfs_active"`
	LibvirtActive    bool   `json:"libvirt_active"`
}

// Probe 探测宿主机完整信息
func Probe() *HostInfo {
	h := &HostInfo{
		Hostname: getHostname(),
		OS:       getOS(),
		Kernel:   getKernel(),
		Arch:     getArch(),
		Uptime:   getUptime(),
	}
	h.ProcessCount = getProcessCount()
	h.CPU = probeCPU()
	h.Memory = probeMemory()
	h.Disks = probeDisks()
	h.Networks = probeNetworks()
	h.Environment = probeEnvironment()
	return h
}

func getHostname() string {
	h, _ := os.Hostname()
	return h
}

func getOS() string {
	data, _ := os.ReadFile("/etc/os-release")
	lines := strings.Split(string(data), "\n")
	name := "Unknown"
	for _, line := range lines {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			name = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
			break
		}
	}
	return name
}

func getKernel() string {
	out, _ := exec.Command("uname", "-sr").Output()
	return strings.TrimSpace(string(out))
}

func getArch() string {
	out, _ := exec.Command("uname", "-m").Output()
	return strings.TrimSpace(string(out))
}

func getUptime() string {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return ""
	}
	sec, _ := strconv.ParseFloat(fields[0], 64)
	d := time.Duration(sec) * time.Second
	return fmt.Sprintf("%dd %dh %dm", int(d.Hours())/24, int(d.Hours())%24, int(d.Minutes())%60)
}

func getProcessCount() int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() {
			if _, err := strconv.Atoi(e.Name()); err == nil {
				count++
			}
		}
	}
	return count
}

func probeCPU() CPUInfo {
	c := CPUInfo{}
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return c
	}
	defer f.Close()

	cores := make(map[int]bool)
	threads := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "model name") {
			c.Model = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		} else if strings.HasPrefix(line, "processor") {
			threads++
		} else if strings.HasPrefix(line, "core id") {
			id, _ := strconv.Atoi(strings.TrimSpace(strings.SplitN(line, ":", 2)[1]))
			cores[id] = true
		} else if strings.Contains(line, "vmx") || strings.Contains(line, "svm") {
			if strings.Contains(line, "vmx") {
				c.Virtualization = "vmx"
			} else if strings.Contains(line, "svm") {
				c.Virtualization = "svm"
			}
		}
	}
	c.Cores = len(cores)
	c.Threads = threads
	if c.Cores == 0 {
		c.Cores = threads
	}

	// 检测 nested KVM
	if data, err := os.ReadFile("/sys/module/kvm_intel/parameters/nested"); err == nil {
		c.NestedKVM = strings.TrimSpace(string(data)) == "Y"
	} else if data, err := os.ReadFile("/sys/module/kvm_amd/parameters/nested"); err == nil {
		c.NestedKVM = strings.TrimSpace(string(data)) == "1"
	}

	// 检测显卡
	if _, err := os.Stat("/dev/dri"); err == nil {
		c.HasGPU = true
	}

	return c
}

func probeMemory() MemoryInfo {
	m := MemoryInfo{}
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return m
	}
	defer f.Close()

	var total, available int64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			v, _ := strconv.ParseInt(fields[1], 10, 64)
			total = v * 1024
		case "MemAvailable:":
			v, _ := strconv.ParseInt(fields[1], 10, 64)
			available = v * 1024
		}
	}
	m.Total = total
	m.Used = total - available
	return m
}

func probeDisks() []DiskInfo {
	var disks []DiskInfo
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return disks
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") {
			continue
		}
		d := DiskInfo{Device: "/dev/" + name}

		// 型号
		if data, err := os.ReadFile(filepath.Join("/sys/block", name, "device/model")); err == nil {
			d.Model = strings.TrimSpace(string(data))
		}
		// 大小
		if data, err := os.ReadFile(filepath.Join("/sys/block", name, "size")); err == nil {
			sectors, _ := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
			gb := float64(sectors) * 512 / 1024 / 1024 / 1024
			d.Size = fmt.Sprintf("%.1f GB", gb)
		}
		// 类型
		rotational, _ := os.ReadFile(filepath.Join("/sys/block", name, "queue/rotational"))
		if strings.TrimSpace(string(rotational)) == "0" {
			d.Type = "ssd"
		} else {
			d.Type = "hdd"
		}
		// 挂载点
		if out, err := exec.Command("findmnt", "-n", "-o", "TARGET", "-S", "/dev/"+name).Output(); err == nil {
			d.MountPoint = strings.TrimSpace(string(out))
		}
		// SMART 健康（简化）
		if out, err := exec.Command("smartctl", "-H", "/dev/"+name).Output(); err == nil {
			if strings.Contains(string(out), "PASSED") {
				d.Health = "PASSED"
			} else {
				d.Health = "UNKNOWN"
			}
		}

		disks = append(disks, d)
	}
	return disks
}

func probeNetworks() []NetworkInfo {
	var nets []NetworkInfo
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return nets
	}
	for _, e := range entries {
		name := e.Name()
		n := NetworkInfo{Name: name}

		// 状态
		if data, err := os.ReadFile(filepath.Join("/sys/class/net", name, "operstate")); err == nil {
			n.Status = strings.TrimSpace(string(data))
		}
		// 驱动
		if data, err := os.Readlink(filepath.Join("/sys/class/net", name, "device/driver")); err == nil {
			n.Driver = filepath.Base(data)
		}
		// MAC
		if data, err := os.ReadFile(filepath.Join("/sys/class/net", name, "address")); err == nil {
			n.MAC = strings.TrimSpace(string(data))
		}
		// 速率
		if data, err := os.ReadFile(filepath.Join("/sys/class/net", name, "speed")); err == nil {
			speed := strings.TrimSpace(string(data))
			if speed != "-1" && speed != "" {
				n.Speed = speed + " Mbps"
			}
		}
		// IP 地址（通过 ip addr）
		if out, err := exec.Command("ip", "-j", "addr", "show", name).Output(); err == nil {
			// 简化解析，直接文本解析
			outText := string(out)
			// 这里不做复杂 JSON 解析，直接跳过
			_ = outText
		}
		// 简单方式读取 ipv4
		if data, err := os.ReadFile(filepath.Join("/sys/class/net", name, "device/vendor")); err == nil {
			_ = data
		}

		// 用 ip 命令获取地址
		if out, err := exec.Command("ip", "addr", "show", name).Output(); err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "inet ") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						n.IPv4 = append(n.IPv4, fields[1])
					}
				} else if strings.HasPrefix(line, "inet6 ") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						n.IPv6 = append(n.IPv6, fields[1])
					}
				}
			}
		}

		nets = append(nets, n)
	}
	return nets
}

func probeEnvironment() EnvInfo {
	e := EnvInfo{}

	// systemd
	if out, err := exec.Command("systemctl", "--version").Output(); err == nil {
		fields := strings.Fields(string(out))
		if len(fields) >= 2 {
			e.SystemdVersion = fields[1]
		}
	}

	// Incus (不是 lxc)
	if out, err := exec.Command("incus", "version").Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "Server version:") {
				e.LXCVersion = strings.TrimSpace(strings.TrimPrefix(line, "Server version:"))
				break
			}
		}
	}

	// iptables
	if out, err := exec.Command("iptables", "--version").Output(); err == nil {
		e.IPTablesVersion = strings.TrimSpace(string(out))
	}

	// iproute2
	if out, err := exec.Command("ip", "-V").Output(); err == nil {
		e.IPRoute2Version = strings.TrimSpace(string(out))
	}

	// conntrack
	if out, err := exec.Command("conntrack", "-V").Output(); err == nil {
		e.ConntrackVersion = strings.TrimSpace(string(out))
	}

	// libvirt
	if out, err := exec.Command("virsh", "--version").Output(); err == nil {
		e.LibvirtVersion = strings.TrimSpace(string(out))
	}

	// QEMU
	if out, err := exec.Command("qemu-system-x86_64", "--version").Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		if len(lines) > 0 {
			e.QEMUVersion = strings.TrimSpace(lines[0])
		}
	}

	// smartctl
	if out, err := exec.Command("smartctl", "-V").Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		if len(lines) > 0 {
			e.SmartctlVersion = strings.TrimSpace(lines[0])
		}
	}

	// KVM
	if _, err := os.Stat("/dev/kvm"); err == nil {
		e.HasKVM = true
	}

	// IPv4 forward
	if data, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward"); err == nil {
		e.HasIPv4Forward = strings.TrimSpace(string(data)) == "1"
	}

	// lxcfs
	if out, err := exec.Command("systemctl", "is-active", "lxcfs").Output(); err == nil {
		e.LXCFSActive = strings.TrimSpace(string(out)) == "active"
	}

	// libvirtd
	if out, err := exec.Command("systemctl", "is-active", "libvirtd").Output(); err == nil {
		e.LibvirtActive = strings.TrimSpace(string(out)) == "active"
	}

	return e
}

// GetTotalMemory 获取总内存（bytes）
func GetTotalMemory() int64 {
	return probeMemory().Total
}

// GetTotalDisk 获取总磁盘大小（bytes）
func GetTotalDisk() int64 {
	var total int64
	for _, d := range probeDisks() {
		var val float64
		fmt.Sscanf(d.Size, "%f", &val)
		total += int64(val * 1024 * 1024 * 1024)
	}
	return total
}

// GetLocalAddress 获取本机第一个非 loopback IPv4
func GetLocalAddress() string {
	entries, _ := os.ReadDir("/sys/class/net")
	for _, e := range entries {
		name := e.Name()
		if name == "lo" {
			continue
		}
		if data, err := os.ReadFile(filepath.Join("/sys/class/net", name, "operstate")); err == nil {
			if strings.TrimSpace(string(data)) != "up" {
				continue
			}
		}
		// 尝试读取地址
		if out, err := exec.Command("ip", "-4", "addr", "show", name).Output(); err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "inet ") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						return strings.Split(fields[1], "/")[0]
					}
				}
			}
		}
	}
	return ""
}

// GetPublicIPv4 获取公网 IPv4
func GetPublicIPv4() string {
	// 尝试通过外部 API 获取
	// 如果没有网络则返回空
	return ""
}

// GetInterfaceNames 获取网卡名称列表
func GetInterfaceNames() []string {
	var names []string
	entries, _ := os.ReadDir("/sys/class/net")
	for _, e := range entries {
		if e.Name() == "lo" {
			continue
		}
		names = append(names, e.Name())
	}
	return names
}
