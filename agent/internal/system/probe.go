package system

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// HostInfo 宿主机探测信息（对齐 CLICD HostProbeReport）
type HostInfo struct {
	GeneratedAt       string         `json:"generated_at"`
	Hostname          string         `json:"hostname"`
	OS                string         `json:"os"`
	Kernel            string         `json:"kernel"`
	CPU               CPUInfo        `json:"cpu"`
	Memory            MemoryInfo     `json:"memory"`
	MemModules        []MemModule    `json:"mem_modules"`
	Disks             []DiskInfo     `json:"disks"`
	NetworkInterfaces []NetworkInfo  `json:"network_interfaces"`
	PublicIPv4        []string       `json:"public_ipv4"`
	IPv4Addresses     []IPProbe      `json:"ipv4_addresses"`
	IPv4Prefixes      []IPv4Prefix   `json:"ipv4_prefixes"`
	IPv6Addresses     []IPProbe      `json:"ipv6_addresses"`
	IPv6Prefixes      []IPv6Prefix   `json:"ipv6_prefixes"`
	Gateways          []GatewayProbe `json:"gateways"`
	GPUs              []GPUInfo      `json:"gpus"`
	Runtime           RuntimeProbe   `json:"runtime"`
	System            SystemProbe    `json:"system"`
	Environment       []EnvCheck     `json:"environment"`
}

// CPUInfo CPU 信息
type CPUInfo struct {
	Model             string   `json:"model"`
	Cores             int      `json:"cores"`
	Threads           int      `json:"threads"`
	Architecture      string   `json:"architecture"`
	Flags             []string `json:"flags"`
	HasIntegratedGPU  bool     `json:"has_integrated_gpu"`
	Virtualization    bool     `json:"virtualization"`
	VirtualizationKey string   `json:"virtualization_key"`
}

// MemoryInfo 内存信息
type MemoryInfo struct {
	TotalMB int64 `json:"total_mb"`
	UsedMB  int64 `json:"used_mb"`
	FreeMB  int64 `json:"free_mb"`
}

// MemModule 内存条信息
type MemModule struct {
	Locator      string `json:"locator"`
	Size         string `json:"size"`
	Type         string `json:"type"`
	Speed        string `json:"speed"`
	Manufacturer string `json:"manufacturer"`
	PartNumber   string `json:"part_number"`
	SerialNumber string `json:"serial_number"`
}

// DiskInfo 磁盘信息
type DiskInfo struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Model        string    `json:"model"`
	Serial       string    `json:"serial"`
	SizeBytes    uint64    `json:"size_bytes"`
	Type         string    `json:"type"`
	Virtual      bool      `json:"virtual"`
	Rotational   bool      `json:"rotational"`
	Mountpoints  []string  `json:"mountpoints"`
	Health       string    `json:"health"`
	HealthDetail string    `json:"health_detail"`
	SMART        DiskSMART `json:"smart"`
}

// DiskSMART SMART 健康信息
type DiskSMART struct {
	Available         bool   `json:"available"`
	LifeUsedPercent   *int   `json:"life_used_percent,omitempty"`
	PowerOnHours      int64  `json:"power_on_hours,omitempty"`
	PowerCycleCount   int64  `json:"power_cycle_count,omitempty"`
	ReadDataBytes     uint64 `json:"read_data_bytes,omitempty"`
	WrittenDataBytes  uint64 `json:"written_data_bytes,omitempty"`
	ReadCommands      uint64 `json:"read_commands,omitempty"`
	WriteCommands     uint64 `json:"write_commands,omitempty"`
	WearLevelingCount string `json:"wear_leveling_count,omitempty"`
	EraseCount        string `json:"erase_count,omitempty"`
	MediaErrors       uint64 `json:"media_errors,omitempty"`
}

// NetworkInfo 网卡信息
type NetworkInfo struct {
	Name      string    `json:"name"`
	MAC       string    `json:"mac"`
	State     string    `json:"state"`
	SpeedMbps int       `json:"speed_mbps"`
	Driver    string    `json:"driver"`
	Model     string    `json:"model"`
	IPv4      []IPProbe `json:"ipv4"`
	IPv6      []IPProbe `json:"ipv6"`
}

// IPProbe IP 地址信息
type IPProbe struct {
	Interface string `json:"interface"`
	Address   string `json:"address"`
	PrefixLen int    `json:"prefix_len"`
	Scope     string `json:"scope"`
	Gateway   string `json:"gateway,omitempty"`
}

// IPv4Prefix IPv4 段信息
type IPv4Prefix struct {
	Interface  string `json:"interface"`
	Address    string `json:"address"`
	Prefix     string `json:"prefix"`
	PrefixLen  int    `json:"prefix_len"`
	SubnetMask string `json:"subnet_mask"`
	Gateway    string `json:"gateway"`
	Source     string `json:"source"`
}

// IPv6Prefix IPv6 段信息
type IPv6Prefix struct {
	Address   string `json:"address"`
	PrefixLen int    `json:"prefix_len"`
	Interface string `json:"interface"`
	Gateway   string `json:"gateway"`
}

// GatewayProbe 网关信息
type GatewayProbe struct {
	Family    string `json:"family"`
	Interface string `json:"interface"`
	Gateway   string `json:"gateway"`
}

// GPUInfo 显卡信息
type GPUInfo struct {
	Name   string `json:"name"`
	Vendor string `json:"vendor"`
	Driver string `json:"driver"`
	Type   string `json:"type"`
}

// RuntimeProbe 运行能力探测
type RuntimeProbe struct {
	LXCAvailable         bool   `json:"lxc_available"`
	KVMAvailable         bool   `json:"kvm_available"`
	DevKVM               bool   `json:"dev_kvm"`
	NestedVirtualization bool   `json:"nested_virtualization"`
	NestedDetail         string `json:"nested_detail"`
}

// SystemProbe 系统运行状态
type SystemProbe struct {
	UptimeSeconds int64  `json:"uptime_seconds"`
	UptimeText    string `json:"uptime_text"`
	ProcessCount  int    `json:"process_count"`
}

// EnvCheck 环境工具检测项
type EnvCheck struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	OK       bool   `json:"ok"`
	Required bool   `json:"required"`
	Detail   string `json:"detail"`
}

// staticCache 缓存启动时采集的静态数据
var staticCache *staticData

type staticData struct {
	hostname   string
	os         string
	kernel     string
	cpu        CPUInfo
	memModules []MemModule
	gpus       []GPUInfo
	env        []EnvCheck
}

// InitStaticProbe 启动时采集一次静态数据（CPU型号、内存条、GPU、环境工具版本等）
func InitStaticProbe() {
	staticCache = &staticData{
		hostname:   getHostname(),
		os:         getOS(),
		kernel:     getKernel(),
		cpu:        probeCPU(),
		memModules: probeMemoryModules(),
		gpus:       probeGPUs(),
		env:        probeEnvironment(),
	}
	staticCache.cpu.HasIntegratedGPU = hasIntegratedGPU(staticCache.gpus)
}

// Probe 探测宿主机完整信息（动态数据每次采集，静态数据从缓存读取）
func Probe() *HostInfo {
	if staticCache == nil {
		InitStaticProbe()
	}

	nics := probeNICs()
	gateways := probeGateways()
	ipv4Addrs := collectIPv4Addresses(nics)
	ipv6Addrs := collectIPv6Addresses(nics)
	ipv4Prefixes := probeIPv4Prefixes(nics, gateways)
	ipv6Prefixes := probeIPv6Prefixes()
	publicIPv4 := probeAllPublicIPv4(nics)
	disks := probeDisks()
	env := staticCache.env
	runtime := probeRuntime(env)

	h := &HostInfo{
		GeneratedAt:       time.Now().Format("2006-01-02 15:04:05"),
		Hostname:          staticCache.hostname,
		OS:                staticCache.os,
		Kernel:            staticCache.kernel,
		CPU:               staticCache.cpu,
		Memory:            probeMemory(),
		MemModules:        staticCache.memModules,
		Disks:             disks,
		NetworkInterfaces: nics,
		PublicIPv4:        publicIPv4,
		IPv4Addresses:     ipv4Addrs,
		IPv4Prefixes:      ipv4Prefixes,
		IPv6Addresses:     ipv6Addrs,
		IPv6Prefixes:      ipv6Prefixes,
		Gateways:          gateways,
		GPUs:              staticCache.gpus,
		Runtime:           runtime,
		System:            probeSystem(),
		Environment:       env,
	}
	return h
}

// --- 基础信息 ---

func getHostname() string {
	h, _ := os.Hostname()
	return h
}

func getOS() string {
	data, _ := os.ReadFile("/etc/os-release")
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
		}
	}
	return strings.TrimSpace(runCommandOutput(2*time.Second, "uname", "-o"))
}

func getKernel() string {
	return strings.TrimSpace(runCommandOutput(2*time.Second, "uname", "-srmo"))
}

func getArch() string {
	return runtime.GOARCH
}

// --- 系统运行状态 ---

func probeSystem() SystemProbe {
	uptime := int64(0)
	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) > 0 {
			val, _ := strconv.ParseFloat(fields[0], 64)
			uptime = int64(val)
		}
	}
	return SystemProbe{
		UptimeSeconds: uptime,
		UptimeText:    formatDurationText(uptime),
		ProcessCount:  countProcesses(),
	}
}

func formatDurationText(seconds int64) string {
	days := seconds / 86400
	seconds %= 86400
	hours := seconds / 3600
	seconds %= 3600
	minutes := seconds / 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

func countProcesses() int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if _, err := strconv.Atoi(entry.Name()); err == nil {
			count++
		}
	}
	return count
}

// --- 运行能力 ---

func probeRuntime(env []EnvCheck) RuntimeProbe {
	devKVM := fileExists("/dev/kvm")
	nested, detail := probeNestedVirtualization()
	incusOK := commandExists("incus")
	return RuntimeProbe{
		LXCAvailable:         incusOK,
		KVMAvailable:         devKVM,
		DevKVM:               devKVM,
		NestedVirtualization: nested,
		NestedDetail:         detail,
	}
}

func probeNestedVirtualization() (bool, string) {
	paths := []string{
		"/sys/module/kvm_intel/parameters/nested",
		"/sys/module/kvm_amd/parameters/nested",
	}
	for _, path := range paths {
		value := strings.TrimSpace(readFirstExistingFile(path))
		if value == "" {
			continue
		}
		enabled := strings.EqualFold(value, "Y") || value == "1"
		return enabled, filepath.Base(filepath.Dir(filepath.Dir(path))) + "=" + value
	}
	if fileExists("/dev/kvm") {
		return true, "/dev/kvm present"
	}
	return false, "no kvm nested parameter or /dev/kvm"
}

func envCheckOK(checks []EnvCheck, key string) bool {
	for _, c := range checks {
		if c.Key == key {
			return c.OK
		}
	}
	return false
}

// --- 兼容旧接口 ---

// GetTotalMemory 获取总内存（bytes）
func GetTotalMemory() int64 {
	return probeMemory().TotalMB * 1024 * 1024
}

// GetTotalDisk 获取总磁盘大小（bytes）
func GetTotalDisk() int64 {
	var total int64
	for _, d := range probeDisks() {
		total += int64(d.SizeBytes)
	}
	return total
}

// GetLocalAddress 获取本机第一个非 loopback IPv4
func GetLocalAddress() string {
	nics := probeNICs()
	for _, nic := range nics {
		if nic.State != "up" {
			continue
		}
		for _, ip := range nic.IPv4 {
			return ip.Address
		}
	}
	return ""
}

// GetPublicIPv4 获取公网 IPv4（通过外部 API）
func GetPublicIPv4() string {
	req, err := http.NewRequest("GET", "https://api-ipv4.ip.sb/ip", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 Tsukiyo/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	ip := strings.TrimSpace(string(data))
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() == nil {
		return ""
	}
	return ip
}

// GetPublicIPv6 获取公网 IPv6（通过外部 API）
func GetPublicIPv6() string {
	req, err := http.NewRequest("GET", "https://api-ipv6.ip.sb/ip", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 Tsukiyo/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	ip := strings.TrimSpace(string(data))
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() != nil {
		return ""
	}
	return ip
}

// GetInterfaceNames 获取网卡名称列表
func GetInterfaceNames() []string {
	nics := probeNICs()
	names := make([]string, 0, len(nics))
	for _, nic := range nics {
		names = append(names, nic.Name)
	}
	return names
}
