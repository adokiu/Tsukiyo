package system

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

// probeMemory 探测内存使用情况（动态数据，每次采集）
func probeMemory() MemoryInfo {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return MemoryInfo{}
	}
	defer f.Close()

	var total, available, free int64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		val, _ := strconv.ParseInt(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:":
			total = val / 1024 // MB
		case "MemAvailable:":
			available = val / 1024 // MB
		case "MemFree:":
			free = val / 1024 // MB
		}
	}

	used := total - available
	if available == 0 {
		used = total - free
	}

	return MemoryInfo{
		TotalMB: total,
		UsedMB:  used,
		FreeMB:  available,
	}
}

// probeMemoryModules 探测内存条信息（静态数据，启动时采集一次）
func probeMemoryModules() []MemModule {
	if !commandExists("dmidecode") {
		return nil
	}

	out := runCommandOutput(4*time.Second, "dmidecode", "-t", "memory")
	modules := make([]MemModule, 0)
	var current MemModule
	inDevice := false
	flush := func() {
		if !inDevice {
			return
		}
		if current.Size != "" && !strings.EqualFold(current.Size, "No Module Installed") {
			modules = append(modules, current)
		}
		current = MemModule{}
	}

	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "Memory Device") {
			flush()
			inDevice = true
			continue
		}
		if !inDevice {
			continue
		}
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "Locator":
			current.Locator = value
		case "Size":
			current.Size = value
		case "Type":
			current.Type = value
		case "Speed":
			current.Speed = value
		case "Manufacturer":
			current.Manufacturer = value
		case "Part Number":
			current.PartNumber = value
		case "Serial Number":
			current.SerialNumber = value
		}
	}
	flush()
	return modules
}
