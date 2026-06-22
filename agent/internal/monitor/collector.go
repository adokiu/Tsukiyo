package monitor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"tsukiyo/agent/internal/config"
	"tsukiyo/agent/internal/incus"
	"tsukiyo/agent/internal/network"
	"tsukiyo/agent/internal/system"
	"tsukiyo/agent/internal/ws"
)

// Collector 监控采集器
type Collector struct {
	cfg                *config.Config
	wsClient           *ws.Client
	incusClient        *incus.Client
	ctx                context.Context
	cancel             context.CancelFunc
	prevCPU            *cpuTimes
	prevNet            map[string]netCounters
	prevNetIO          *netIOCounters
	prevNetTime        time.Time
	prevInstanceNet    map[string]instanceNetCounters
	prevInstanceCPU    map[string]instanceCPUCounters
	prevInstanceDiskIO map[string]instanceDiskIOCounters
	prevCollectTime    time.Time
}

// instanceCPUCounters 实例CPU计数器（用于计算CPU使用率）
type instanceCPUCounters struct {
	cpuNanos int64
	time     time.Time
}

// instanceDiskIOCounters 实例磁盘IO计数器（用于计算速率和IOPS）
type instanceDiskIOCounters struct {
	readBytes  int64
	writeBytes int64
	readOps    int64
	writeOps   int64
	time       time.Time
}

// netIOCounters 网络IO总计数器
type netIOCounters struct {
	rxBytes uint64
	txBytes uint64
}

// instanceNetCounters 实例网络计数器（用于计算速率）
type instanceNetCounters struct {
	rxBytes int64
	txBytes int64
	time    time.Time
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
		cfg:                cfg,
		wsClient:           wsClient,
		incusClient:        incusClient,
		ctx:                ctx,
		cancel:             cancel,
		prevNet:            make(map[string]netCounters),
		prevInstanceNet:    make(map[string]instanceNetCounters),
		prevInstanceCPU:    make(map[string]instanceCPUCounters),
		prevInstanceDiskIO: make(map[string]instanceDiskIOCounters),
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
	if !c.incusClient.IsAvailable() {
		return
	}
	aliases, err := c.incusClient.ListImages()
	if err != nil {
		if c.incusClient.IsAvailable() {
			zap.L().Warn("定期镜像同步: 查询失败", zap.Error(err))
		}
		return
	}
	if err := c.wsClient.SendLocalImages(aliases); err != nil {
		zap.L().Warn("定期镜像同步: 上报失败", zap.Error(err))
	}
}

// collectAndReport 采集并上报
func (c *Collector) collectAndReport() {
	if !c.incusClient.IsAvailable() {
		return
	}

	instances, err := c.incusClient.ListInstances()
	if err != nil {
		if c.incusClient.IsAvailable() {
			zap.L().Warn("获取实例列表失败", zap.Error(err))
		}
		return
	}

	var metrics []ws.InstanceMetricPayload
	var statuses []ws.InstanceStatusPayload
	runningCount := 0

	for _, inst := range instances {
		// 从实例配置或设备配置解析磁盘总量
		// Incus 磁盘限制可能在 config["limits.disk"] 或 devices["root"]["size"] 中
		var diskTotalMB int64
		if diskLimit, ok := inst.Config["limits.disk"]; ok && diskLimit != "" {
			diskTotalMB = parseDiskLimitMB(diskLimit)
		}
		if diskTotalMB == 0 {
			if rootDev, ok := inst.Devices["root"].(map[string]interface{}); ok {
				if size, ok := rootDev["size"].(string); ok && size != "" {
					diskTotalMB = parseDiskLimitMB(size)
				}
			}
		}
		metric := c.collectInstanceMetrics(inst.Name, diskTotalMB)
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
func (c *Collector) collectInstanceMetrics(name string, diskTotalMB int64) *ws.InstanceMetricPayload {
	m, err := c.incusClient.GetInstanceMetrics(name)
	if err != nil {
		return nil
	}

	// CPU 使用率：基于两次采集的 CPU 纳秒差值 / 经过时间(纳秒) * 100
	now := time.Now()
	var cpuPercent float64
	if prev, exists := c.prevInstanceCPU[name]; exists {
		elapsed := now.Sub(prev.time).Nanoseconds()
		if elapsed > 0 {
			cpuDelta := m.CPUUsage - prev.cpuNanos
			if cpuDelta > 0 {
				cpuPercent = float64(cpuDelta) / float64(elapsed) * 100
				if cpuPercent > 100 {
					cpuPercent = 100
				}
			}
		}
	}
	c.prevInstanceCPU[name] = instanceCPUCounters{
		cpuNanos: m.CPUUsage,
		time:     now,
	}

	// 计算网络总流量（所有非 lo 接口的总和）
	var netInTotal, netOutTotal int64
	for iface, stat := range m.NetworkStats {
		if iface == "lo" {
			continue
		}
		netInTotal += stat.BytesReceived
		netOutTotal += stat.BytesSent
	}

	// 计算网络速率（基于上次采集的差值）
	var netInBps, netOutBps int64
	if prev, exists := c.prevInstanceNet[name]; exists {
		elapsed := now.Sub(prev.time).Seconds()
		if elapsed > 0 {
			// 计数器正常递增
			if netInTotal >= prev.rxBytes {
				netInBps = int64(float64(netInTotal-prev.rxBytes) / elapsed)
			}
			if netOutTotal >= prev.txBytes {
				netOutBps = int64(float64(netOutTotal-prev.txBytes) / elapsed)
			}
		}
	}
	c.prevInstanceNet[name] = instanceNetCounters{
		rxBytes: netInTotal,
		txBytes: netOutTotal,
		time:    now,
	}

	// 磁盘使用量（rootfs）- Incus state API 返回 bytes，转换为 MB
	var diskUsed, diskTotal int64
	if usage, ok := m.DiskUsage["root"]; ok {
		diskUsed = usage / 1024 / 1024 // bytes -> MB
	}
	diskTotal = diskTotalMB

	// 磁盘 IO 采集：通过 cgroup 读取容器所有进程的 IO 统计
	// /proc/<pid>/io 只统计 init 进程自身的 IO，不包括子进程（如 fio）
	// cgroup IO 统计包含容器内所有进程
	var diskReadBps, diskWriteBps, diskReadIops, diskWriteIops int64
	rb, wb, sr, sw := readContainerCgroupIO(name, m.PID)
	if prev, exists := c.prevInstanceDiskIO[name]; exists {
		elapsed := now.Sub(prev.time).Seconds()
		if elapsed > 0 {
			if rb >= prev.readBytes {
				diskReadBps = int64(float64(rb-prev.readBytes) / elapsed)
			}
			if wb >= prev.writeBytes {
				diskWriteBps = int64(float64(wb-prev.writeBytes) / elapsed)
			}
			if sr >= prev.readOps {
				diskReadIops = int64(float64(sr-prev.readOps) / elapsed)
			}
			if sw >= prev.writeOps {
				diskWriteIops = int64(float64(sw-prev.writeOps) / elapsed)
			}
		}
	}
	c.prevInstanceDiskIO[name] = instanceDiskIOCounters{
		readBytes:  rb,
		writeBytes: wb,
		readOps:    sr,
		writeOps:   sw,
		time:       now,
	}

	return &ws.InstanceMetricPayload{
		InstanceID:    name,
		CPUPercent:    math.Round(cpuPercent*100) / 100,
		MemUsed:       m.MemUsage / 1024 / 1024, // bytes -> MB
		MemTotal:      m.MemTotal / 1024 / 1024,
		DiskUsed:      diskUsed,
		DiskTotal:     diskTotal,
		DiskReadBps:   diskReadBps,
		DiskWriteBps:  diskWriteBps,
		DiskReadIops:  diskReadIops,
		DiskWriteIops: diskWriteIops,
		NetIn:         netInBps,
		NetOut:        netOutBps,
		NetInTotal:    netInTotal,
		NetOutTotal:   netOutTotal,
	}
}

// readProcIO 读取 /proc/<pid>/io 获取进程磁盘 IO 统计
// 返回 (readBytes, writeBytes, syscr, syscw)
// read_bytes: 实际从磁盘读取的字节数
// write_bytes: 实际写入磁盘的字节数
// syscr: 读系统调用次数（近似读 IOPS）
// syscw: 写系统调用次数（近似写 IOPS）
func readProcIO(pid int) (int64, int64, int64, int64) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/io", pid))
	if err != nil {
		return 0, 0, 0, 0
	}
	defer f.Close()

	var rb, wb, sr, sw int64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		val, _ := strconv.ParseInt(parts[1], 10, 64)
		switch parts[0] {
		case "read_bytes:":
			rb = val
		case "write_bytes:":
			wb = val
		case "syscr:":
			sr = val
		case "syscw:":
			sw = val
		}
	}
	return rb, wb, sr, sw
}

// readContainerCgroupIO 读取容器级 IO 统计
// 返回 (readBytes, writeBytes, readOps, writeOps)
// 优先使用 cgroup v2 io.stat（内核级精确统计，包含容器内所有进程）
// 回退到 cgroup v1 blkio.throttle.io_service_bytes_recursive
// 最后回退到累加 cgroup.procs 中所有 PID 的 /proc/<pid>/io
func readContainerCgroupIO(name string, pid int) (int64, int64, int64, int64) {
	// 查找容器 cgroup v2 路径
	cgroupDirs := []string{
		fmt.Sprintf("/sys/fs/cgroup/lxc.payload.%s", name),
	}

	// 通过 init 进程的 cgroup 路径查找
	if pid > 0 {
		if cgroupPath, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid)); err == nil {
			lines := strings.Split(string(cgroupPath), "\n")
			for _, line := range lines {
				parts := strings.SplitN(line, ":", 3)
				if len(parts) == 3 && (parts[1] == "" || parts[1] == "0") {
					cgroupSubpath := strings.TrimSpace(parts[2])
					if cgroupSubpath != "" && cgroupSubpath != "/" {
						cgroupDirs = append([]string{fmt.Sprintf("/sys/fs/cgroup%s", cgroupSubpath)}, cgroupDirs...)
					}
				}
			}
		}
	}

	// 优先尝试 cgroup v2 io.stat
	for _, dir := range cgroupDirs {
		ioStatPath := filepath.Join(dir, "io.stat")
		data, err := os.ReadFile(ioStatPath)
		if err != nil {
			continue
		}
		if rb, wb, rios, wios := parseCgroupV2IOStat(data); rb > 0 || wb > 0 || rios > 0 || wios > 0 {
			return rb, wb, rios, wios
		}
	}

	// 回退到 cgroup v1 blkio 统计
	cgroupV1Dirs := []string{
		fmt.Sprintf("/sys/fs/cgroup/blkio/lxc/%s", name),
		fmt.Sprintf("/sys/fs/cgroup/blkio/lxc.payload.%s", name),
	}
	if pid > 0 {
		if cgroupPath, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid)); err == nil {
			lines := strings.Split(string(cgroupPath), "\n")
			for _, line := range lines {
				parts := strings.SplitN(line, ":", 3)
				if len(parts) == 3 && parts[1] == "blkio" {
					cgroupSubpath := strings.TrimSpace(parts[2])
					if cgroupSubpath != "" && cgroupSubpath != "/" {
						cgroupV1Dirs = append([]string{fmt.Sprintf("/sys/fs/cgroup/blkio%s", cgroupSubpath)}, cgroupV1Dirs...)
					}
				}
			}
		}
	}
	for _, dir := range cgroupV1Dirs {
		// 尝试递归版本（包含子 cgroup）
		for _, fname := range []string{"blkio.throttle.io_service_bytes_recursive", "blkio.throttle.io_service_bytes"} {
			path := filepath.Join(dir, fname)
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			if rb, wb := parseCgroupV1BlkIO(data); rb > 0 || wb > 0 {
				// cgroup v1 blkio 没有直接的 IOPS 统计，用 rbytes/wbytes 除以块大小估算
				// 但更准确的方式是读取 blkio.throttle.io_serviced_recursive
				rios, wios := readCgroupV1BlkIOServiced(dir)
				return rb, wb, rios, wios
			}
		}
	}

	// 最终回退：累加 cgroup.procs 中所有 PID 的 /proc/<pid>/io
	for _, dir := range cgroupDirs {
		procsPath := filepath.Join(dir, "cgroup.procs")
		data, err := os.ReadFile(procsPath)
		if err != nil {
			continue
		}
		return sumProcIOs(data)
	}

	// 最后回退到 /proc/<pid>/io（仅 init 进程）
	if pid > 0 {
		return readProcIO(pid)
	}
	return 0, 0, 0, 0
}

// parseCgroupV2IOStat 解析 cgroup v2 io.stat 文件
// 格式示例:
// 8:0 rbytes=1234567 wbytes=987654 rios=100 wios=50
// 8:16 rbytes=0 wbytes=0 rios=0 wios=0
func parseCgroupV2IOStat(data []byte) (readBytes, writeBytes, readOps, writeOps int64) {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		// 跳过设备号字段（如 "8:0"）
		for _, field := range fields[1:] {
			parts := strings.SplitN(field, "=", 2)
			if len(parts) != 2 {
				continue
			}
			val, _ := strconv.ParseInt(parts[1], 10, 64)
			switch parts[0] {
			case "rbytes":
				readBytes += val
			case "wbytes":
				writeBytes += val
			case "rios":
				readOps += val
			case "wios":
				writeOps += val
			}
		}
	}
	return
}

// parseCgroupV1BlkIO 解析 cgroup v1 blkio.throttle.io_service_bytes 文件
// 格式示例:
// 8:0 Read 1234567
// 8:0 Write 987654
// 8:0 Sync 1234567
// 8:0 Async 0
// Total 2222221
func parseCgroupV1BlkIO(data []byte) (readBytes, writeBytes int64) {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		// 跳过 "Total" 行
		if fields[0] == "Total" {
			continue
		}
		val, _ := strconv.ParseInt(fields[2], 10, 64)
		switch fields[1] {
		case "Read":
			readBytes += val
		case "Write":
			writeBytes += val
		}
	}
	return
}

// readCgroupV1BlkIOServiced 读取 cgroup v1 blkio.throttle.io_serviced 获取 IOPS 计数
func readCgroupV1BlkIOServiced(dir string) (readOps, writeOps int64) {
	for _, fname := range []string{"blkio.throttle.io_serviced_recursive", "blkio.throttle.io_serviced"} {
		path := filepath.Join(dir, fname)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			line := scanner.Text()
			fields := strings.Fields(line)
			if len(fields) < 3 {
				continue
			}
			if fields[0] == "Total" {
				continue
			}
			val, _ := strconv.ParseInt(fields[2], 10, 64)
			switch fields[1] {
			case "Read":
				readOps += val
			case "Write":
				writeOps += val
			}
		}
		return
	}
	return 0, 0
}

// sumProcIOs 根据 PID 列表累加所有进程的 /proc/<pid>/io
func sumProcIOs(pidData []byte) (int64, int64, int64, int64) {
	var totalRb, totalWb, totalSr, totalSw int64
	scanner := bufio.NewScanner(strings.NewReader(string(pidData)))
	for scanner.Scan() {
		pidStr := strings.TrimSpace(scanner.Text())
		if pidStr == "" {
			continue
		}
		pid, err := strconv.Atoi(pidStr)
		if err != nil || pid <= 0 {
			continue
		}
		rb, wb, sr, sw := readProcIO(pid)
		totalRb += rb
		totalWb += wb
		totalSr += sr
		totalSw += sw
	}
	return totalRb, totalWb, totalSr, totalSw
}

// sendHeartbeat 发送心跳
func (c *Collector) sendHeartbeat() {
	if !c.wsClient.IsConnected() {
		return
	}

	cpuPercent := c.getCPUUsage()
	memUsed, memTotal := c.getMemUsage()
	diskUsed, diskTotal := c.getDiskUsage()
	netIn, netOut := c.getNetworkIO()
	uptime := getUptimeSeconds()

	instanceCount := 0
	running := 0
	if c.incusClient.IsAvailable() {
		instances, _ := c.incusClient.ListInstances()
		instanceCount = len(instances)
		for _, inst := range instances {
			if inst.Status == "Running" {
				running++
			}
		}
	}

	publicIPv4s := getPublicIPv4s()
	ipv6Prefixes := getIPv6Prefixes()

	// 采集网卡信息
	var networkInterfaces json.RawMessage
	hostInfo := system.Probe()
	if nicData, err := json.Marshal(hostInfo.NetworkInterfaces); err == nil {
		networkInterfaces = nicData
	}

	if err := c.wsClient.SendHeartbeat(cpuPercent, memUsed, memTotal, diskUsed, diskTotal,
		netIn, netOut, uptime, instanceCount, running, publicIPv4s, ipv6Prefixes, networkInterfaces); err != nil {
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

// getDiskUsage 获取所有物理磁盘使用量 (Linux /proc/mounts + unix.Statfs)
func (c *Collector) getDiskUsage() (used int64, total int64) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	// 需要跳过的虚拟文件系统类型
	skipFS := map[string]bool{
		"tmpfs": true, "devtmpfs": true, "proc": true, "sysfs": true,
		"cgroup": true, "cgroup2": true, "pstore": true, "securityfs": true,
		"mqueue": true, "hugetlbfs": true, "debugfs": true, "tracefs": true,
		"configfs": true, "fusectl": true, "fuse": true, "fuseblk": true,
		"rpc_pipefs": true, "bpf": true, "efivarfs": true,
	}

	seen := map[string]bool{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		device := fields[0]
		mountPoint := fields[1]
		fstype := fields[2]

		// 跳过虚拟文件系统
		if skipFS[fstype] {
			continue
		}
		// 跳过已处理的设备（同一设备可能挂载多次）
		if seen[device] {
			continue
		}
		// 跳过 overlay/docker 等容器文件系统
		if strings.HasPrefix(device, "overlay") {
			continue
		}

		seen[device] = true

		var stat unix.Statfs_t
		if err := unix.Statfs(mountPoint, &stat); err != nil {
			continue
		}

		totalBytes := stat.Blocks * uint64(stat.Bsize)
		freeBytes := stat.Bfree * uint64(stat.Bsize)
		total += int64(totalBytes / 1024 / 1024 / 1024) // GB
		used += int64(totalBytes/1024/1024/1024) - int64(freeBytes/1024/1024/1024)
	}

	return used, total
}

// getNetworkIO 获取网络IO速率 (bytes/s)
func (c *Collector) getNetworkIO() (rxSpeed int64, txSpeed int64) {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return 0, 0
	}

	var totalRx, totalTx uint64
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		iface := strings.TrimSuffix(fields[0], ":")
		if iface == "lo" {
			continue
		}
		rx, _ := strconv.ParseUint(fields[1], 10, 64)
		tx, _ := strconv.ParseUint(fields[9], 10, 64)
		totalRx += rx
		totalTx += tx
	}

	now := time.Now()
	if c.prevNetIO != nil && c.prevNetTime.Before(now) {
		elapsed := now.Sub(c.prevNetTime).Seconds()
		if elapsed > 0 {
			rxSpeed = int64(float64(totalRx-c.prevNetIO.rxBytes) / elapsed)
			txSpeed = int64(float64(totalTx-c.prevNetIO.txBytes) / elapsed)
			if rxSpeed < 0 {
				rxSpeed = 0
			}
			if txSpeed < 0 {
				txSpeed = 0
			}
		}
	}
	c.prevNetIO = &netIOCounters{rxBytes: totalRx, txBytes: totalTx}
	c.prevNetTime = now
	return
}

// getUptimeSeconds 获取系统运行时间 (秒)
func getUptimeSeconds() int64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	sec, _ := strconv.ParseFloat(fields[0], 64)
	return int64(sec)
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

// parseDiskLimitMB 解析 Incus limits.disk 配置值，返回 MB
// 支持格式: "10GB", "512MB", "1TB", "1024" (默认 bytes)
func parseDiskLimitMB(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	// 提取数字部分和单位部分
	var numStr string
	var unit string
	for i, r := range s {
		if (r >= '0' && r <= '9') || r == '.' {
			numStr = s[:i+1]
		} else {
			unit = strings.ToLower(strings.TrimSpace(s[i:]))
			break
		}
	}

	if numStr == "" {
		return 0
	}

	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0
	}

	switch unit {
	case "tb", "tib":
		return int64(num * 1024 * 1024)
	case "gb", "gib":
		return int64(num * 1024)
	case "mb", "mib":
		return int64(num)
	case "kb", "kib":
		return int64(num / 1024)
	case "b", "":
		return int64(num / 1024 / 1024)
	default:
		return int64(num / 1024 / 1024)
	}
}
