package ws

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// getTotalMemory 获取总内存 (MB)
func getTotalMemory() int64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseInt(fields[1], 10, 64)
				return kb / 1024 // MB
			}
		}
	}
	return 0
}

// getTotalDisk 获取总磁盘 (GB)
func getTotalDisk() int64 {
	var stat unix.Statfs_t
	if err := unix.Statfs("/", &stat); err != nil {
		return 0
	}
	total := stat.Blocks * uint64(stat.Bsize)
	return int64(total / 1024 / 1024 / 1024) // GB
}

// getIncusVersion 获取 Incus 版本
func getIncusVersion(socketPath string) (string, error) {
	cmd := exec.Command("incus", "version")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// getCPUUsage 获取 CPU 使用率
func getCPUUsage() float64 {
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
	if total > 0 {
		return float64(active) / float64(total) * 100.0
	}
	return 0.0
}

// getMemUsage 获取内存使用 (used, total MB)
func getMemUsage() (used int64, total int64) {
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

// parseSize 解析大小字符串
func parseSize(s string) int64 {
	if s == "" || s == "0" {
		return 0
	}
	s = strings.ToUpper(strings.TrimSpace(s))
	var multiplier int64 = 1
	if strings.HasSuffix(s, "GB") || strings.HasSuffix(s, "GIB") {
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "B"), "GI")
		s = strings.TrimSuffix(s, "G")
	} else if strings.HasSuffix(s, "MB") || strings.HasSuffix(s, "MIB") {
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "B"), "MI")
		s = strings.TrimSuffix(s, "M")
	} else if strings.HasSuffix(s, "KB") || strings.HasSuffix(s, "KIB") {
		multiplier = 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "B"), "KI")
		s = strings.TrimSuffix(s, "K")
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return int64(val * float64(multiplier))
}

// getDiskUsage 获取磁盘使用 (used, total GB)
func getDiskUsage() (used int64, total int64) {
	var stat unix.Statfs_t
	if err := unix.Statfs("/", &stat); err != nil {
		return 0, 0
	}
	totalBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bfree * uint64(stat.Bsize)
	total = int64(totalBytes / 1024 / 1024 / 1024)
	used = total - int64(freeBytes/1024/1024/1024)
	return
}

// getCPUUsageWithHistory 带历史记录的 CPU 使用率计算
type cpuHistory struct {
	active uint64
	total  uint64
	time   time.Time
}

var globalCPUHistory cpuHistory

func getCPUUsageWithHistory() float64 {
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

	prev := globalCPUHistory
	globalCPUHistory = cpuHistory{active: active, total: total, time: time.Now()}

	if prev.total == 0 || total <= prev.total {
		return 0.0
	}
	deltaActive := active - prev.active
	deltaTotal := total - prev.total
	if deltaTotal > 0 {
		return float64(deltaActive) / float64(deltaTotal) * 100.0
	}
	return 0.0
}
