package monitor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"tsukiyo/agent/internal/config"
	"tsukiyo/agent/internal/incus"
	"tsukiyo/agent/internal/network"
	"tsukiyo/agent/internal/ws"
)

// Collector 监控采集器
type Collector struct {
	cfg         *config.Config
	wsClient    *ws.Client
	incusClient *incus.Client
	ctx         context.Context
	cancel      context.CancelFunc
	prevCPU     *cpuTimes
	prevNet     map[string]netCounters
}

// cpuTimes CPU 时间统计
type cpuTimes struct {
	user   uint64
	system uint64
	idle   uint64
}

// netCounters 网络计数器
type netCounters struct {
	bytesRecv uint64
	bytesSent uint64
}

// NewCollector 创建监控采集器
func NewCollector(cfg *config.Config, wsClient *ws.Client, incusClient *incus.Client) *Collector {
	ctx, cancel := context.WithCancel(context.Background())
	return &Collector{
		cfg:         cfg,
		wsClient:    wsClient,
		incusClient: incusClient,
		ctx:         ctx,
		cancel:      cancel,
		prevNet:     make(map[string]netCounters),
	}
}

// Start 启动监控采集循环
func (c *Collector) Start() {
	zap.L().Info("监控采集器启动", zap.Duration("interval", c.cfg.MetricsInterval()))
	go c.loop()
	go c.heartbeatLoop()
	go c.imageSyncLoop()
}

// Stop 停止监控采集
func (c *Collector) Stop() {
	c.cancel()
}

// loop 主采集循环
func (c *Collector) loop() {
	ticker := time.NewTicker(c.cfg.MetricsInterval())
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.collectAndReport()
		}
	}
}

// heartbeatLoop 心跳循环
func (c *Collector) heartbeatLoop() {
	ticker := time.NewTicker(c.cfg.HeartbeatInterval())
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.sendHeartbeat()
		}
	}
}

// imageSyncLoop 定期同步本地 Incus 镜像列表到 Master
func (c *Collector) imageSyncLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.syncImages()
		}
	}
}

// syncImages 查询 Incus 本地镜像并上报
func (c *Collector) syncImages() {
	if !c.wsClient.IsConnected() {
		return
	}
	aliases, err := c.incusClient.ListImages()
	if err != nil {
		zap.L().Warn("定期镜像同步: 查询失败", zap.Error(err))
		return
	}
	if err := c.wsClient.SendLocalImages(aliases); err != nil {
		zap.L().Warn("定期镜像同步: 上报失败", zap.Error(err))
	}
}

// collectAndReport 采集并上报
func (c *Collector) collectAndReport() {
	instances, err := c.incusClient.ListInstances()
	if err != nil {
		zap.L().Warn("获取实例列表失败", zap.Error(err))
		return
	}

	var metrics []ws.InstanceMetricPayload
	var statuses []ws.InstanceStatusPayload
	runningCount := 0

	for _, inst := range instances {
		metric := c.collectInstanceMetrics(inst.Name)
		if metric != nil {
			metrics = append(metrics, *metric)
		}

		status := ws.InstanceStatusPayload{
			InstanceID: inst.Name,
			Status:     inst.Status,
		}
		if inst.Status == "Running" {
			runningCount++
			// 获取实例内部 IP
			if ipv4s, err := c.incusClient.GetInstanceNetworkInfo(inst.Name); err == nil && len(ipv4s) > 0 {
				status.IPv4 = ipv4s[0]
			}
		}
		statuses = append(statuses, status)
	}

	// 上报实例状态
	if len(statuses) > 0 {
		if err := c.wsClient.SendInstanceStatus(statuses); err != nil {
			zap.L().Warn("上报实例状态失败", zap.Error(err))
		}
	}

	// 上报监控指标
	if len(metrics) > 0 {
		if err := c.wsClient.SendMetrics(metrics); err != nil {
			zap.L().Warn("上报监控指标失败", zap.Error(err))
		}
	}
}

// collectInstanceMetrics 采集单个实例指标
func (c *Collector) collectInstanceMetrics(name string) *ws.InstanceMetricPayload {
	m, err := c.incusClient.GetInstanceMetrics(name)
	if err != nil {
		return nil
	}

	var cpuPercent float64
	if m.MemTotal > 0 {
		cpuPercent = float64(m.CPUUsage) / float64(m.MemTotal) * 100
		if cpuPercent > 100 {
			cpuPercent = 100
		}
	}

	return &ws.InstanceMetricPayload{
		InstanceID: name,
		CPUPercent: math.Round(cpuPercent*100) / 100,
		MemUsed:    m.MemUsage / 1024 / 1024, // bytes -> MB
		MemTotal:   m.MemTotal / 1024 / 1024,
		DiskRead:   0,
		DiskWrite:  0,
		NetIn:      0,
		NetOut:     0,
	}
}

// sendHeartbeat 发送心跳
func (c *Collector) sendHeartbeat() {
	if !c.wsClient.IsConnected() {
		return
	}

	cpuPercent := c.getCPUUsage()
	memUsed, memTotal := c.getMemUsage()
	diskUsed, diskTotal := c.getDiskUsage()

	instances, _ := c.incusClient.ListInstances()
	running := 0
	for _, inst := range instances {
		if inst.Status == "Running" {
			running++
		}
	}

	publicIPv4s := getPublicIPv4s()
	ipv6Prefixes := getIPv6Prefixes()

	if err := c.wsClient.SendHeartbeat(cpuPercent, memUsed, memTotal, diskUsed, diskTotal,
		len(instances), running, publicIPv4s, ipv6Prefixes); err != nil {
		zap.L().Warn("发送心跳失败", zap.Error(err))
	}
}

// getCPUUsage 获取 CPU 使用率 (Linux /proc/stat)
func (c *Collector) getCPUUsage() float64 {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0.0
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return 0.0
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0.0
	}
	var user, nice, system, idle, iowait, irq, softirq uint64
	fmt.Sscanf(fields[1], "%d", &user)
	fmt.Sscanf(fields[2], "%d", &nice)
	fmt.Sscanf(fields[3], "%d", &system)
	fmt.Sscanf(fields[4], "%d", &idle)
	if len(fields) > 5 {
		fmt.Sscanf(fields[5], "%d", &iowait)
	}
	if len(fields) > 6 {
		fmt.Sscanf(fields[6], "%d", &irq)
	}
	if len(fields) > 7 {
		fmt.Sscanf(fields[7], "%d", &softirq)
	}

	total := user + nice + system + idle + iowait + irq + softirq
	active := total - idle

	if c.prevCPU != nil {
		deltaActive := active - c.prevCPU.user
		deltaTotal := total - c.prevCPU.system
		if deltaTotal > 0 {
			usage := 100.0 * float64(deltaActive) / float64(deltaTotal)
			if usage < 0 {
				usage = 0
			}
			if usage > 100 {
				usage = 100
			}
			c.prevCPU = &cpuTimes{user: active, system: total, idle: idle}
			return math.Round(usage*100) / 100
		}
	}
	c.prevCPU = &cpuTimes{user: active, system: total, idle: idle}
	return 0.0
}

// getMemUsage 获取内存使用 (Linux /proc/meminfo)
func (c *Collector) getMemUsage() (used int64, total int64) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	var memTotal, memAvailable int64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			memTotal, _ = strconv.ParseInt(fields[1], 10, 64)
		case "MemAvailable:":
			memAvailable, _ = strconv.ParseInt(fields[1], 10, 64)
		}
	}

	total = memTotal / 1024 // MB
	if memAvailable > 0 {
		used = total - (memAvailable / 1024)
	} else {
		used = total / 2
	}
	return
}

// getDiskUsage 获取磁盘使用 (Linux unix.Statfs)
func (c *Collector) getDiskUsage() (used int64, total int64) {
	var stat unix.Statfs_t
	if err := unix.Statfs("/", &stat); err != nil {
		return 0, 0
	}
	totalBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bfree * uint64(stat.Bsize)
	total = int64(totalBytes / 1024 / 1024 / 1024) // GB
	used = total - int64(freeBytes/1024/1024/1024)
	return
}

// getPublicIPv4s 获取公网 IPv4
func getPublicIPv4s() []string {
	ips, err := network.GetLocalIPs()
	if err != nil {
		return nil
	}
	var publicIPs []string
	for _, ip := range ips {
		if !isPrivateIP(ip) {
			publicIPs = append(publicIPs, ip)
		}
	}
	return publicIPs
}

// getIPv6Prefixes 获取 IPv6 前缀
func getIPv6Prefixes() []string {
	var prefixes []string
	interfaces, err := network.GetLocalInterfaces()
	if err != nil {
		return nil
	}
	for _, iface := range interfaces {
		for _, prefix := range iface.IPv6Prefixes {
			prefixes = append(prefixes, prefix)
		}
	}
	return prefixes
}

// isPrivateIP 判断是否为内网 IP
func isPrivateIP(ip string) bool {
	if strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "127.") {
		return true
	}
	if strings.HasPrefix(ip, "172.") {
		parts := strings.Split(ip, ".")
		if len(parts) > 1 {
			second, _ := strconv.Atoi(parts[1])
			if second >= 16 && second <= 31 {
				return true
			}
		}
	}
	return false
}

// GetSystemMetrics 获取系统级指标 (用于安全扫描模块)
func GetSystemMetrics() (cpuPercent float64, memUsed int64, memTotal int64) {
	var stat unix.Statfs_t
	if err := unix.Statfs("/", &stat); err == nil {
		memTotal = int64(stat.Blocks*uint64(stat.Bsize)) / 1024 / 1024
		memUsed = memTotal - int64(stat.Bfree*uint64(stat.Bsize))/1024/1024
	}
	cpuPercent = 0.0
	return
}

// InstanceMetricsJSON 实例指标 JSON 序列化辅助
func InstanceMetricsJSON(metrics []ws.InstanceMetricPayload) ([]byte, error) {
	return json.Marshal(metrics)
}

// NetworkInterface 网络接口信息
type NetworkInterface struct {
	Name         string   `json:"name"`
	IPv4s        []string `json:"ipv4s"`
	IPv6Prefixes []string `json:"ipv6_prefixes"`
}
