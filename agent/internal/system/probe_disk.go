package system

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// probeDisks 探测磁盘信息（磁盘基本信息为静态，SMART为缓慢变动数据）
func probeDisks() []DiskInfo {
	disks := make([]DiskInfo, 0)
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return disks
	}
	mounts := detectMountpointsByDevice()

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") || strings.HasPrefix(name, "fd") || strings.HasPrefix(name, "sr") {
			continue
		}

		base := filepath.Join("/sys/block", name)
		path := "/dev/" + name
		model := strings.TrimSpace(readFirstExistingFile(filepath.Join(base, "device/model"), filepath.Join(base, "device/name")))
		vendor := strings.TrimSpace(readFirstExistingFile(filepath.Join(base, "device/vendor")))
		virtual := isVirtualBlockDevice(name, model, vendor)

		disk := DiskInfo{
			Name:        name,
			Path:        path,
			Model:       model,
			Serial:      strings.TrimSpace(readFirstExistingFile(filepath.Join(base, "device/serial"), filepath.Join(base, "serial"))),
			SizeBytes:   readUintFile(filepath.Join(base, "size")) * 512,
			Type:        detectDiskType(base, name, virtual),
			Virtual:     virtual,
			Rotational:  strings.TrimSpace(readFirstExistingFile(filepath.Join(base, "queue/rotational"))) == "1",
			Mountpoints: mounts[name],
		}

		if !virtual {
			disk.SMART = detectDiskSMART(path)
		}
		disk.Health = diskSMARTHealth(disk)
		disk.HealthDetail = diskSMARTDetail(disk)

		disks = append(disks, disk)
	}

	sort.Slice(disks, func(i, j int) bool { return disks[i].Name < disks[j].Name })
	return disks
}

func detectDiskType(base, name string, virtual bool) string {
	if virtual {
		return "Virtual"
	}
	if strings.HasPrefix(name, "nvme") {
		return "NVMe"
	}
	if strings.TrimSpace(readFirstExistingFile(filepath.Join(base, "queue/rotational"))) == "1" {
		return "HDD"
	}
	return "SSD"
}

func isVirtualBlockDevice(name, model, vendor string) bool {
	lower := strings.ToLower(strings.TrimSpace(name + " " + model + " " + vendor))
	if strings.HasPrefix(name, "vd") || strings.HasPrefix(name, "xvd") {
		return true
	}
	for _, token := range []string{
		"qemu", "virtio", "virtual", "vmware", "vbox", "xen",
		"amazon elastic block store", "google persistentdisk", "microsoft",
	} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func diskSMARTHealth(disk DiskInfo) string {
	if disk.Virtual {
		return "virtual"
	}
	if disk.SMART.Available && disk.Health != "" {
		return disk.Health
	}
	return smartHealth(disk.SMART)
}

func diskSMARTDetail(disk DiskInfo) string {
	if disk.Virtual {
		return "虚拟磁盘，真实 SMART/寿命/通电数据需在物理宿主机查看"
	}
	return smartDetail(disk.SMART)
}

func smartHealth(smart DiskSMART) string {
	if !smart.Available {
		return "unknown"
	}
	if smart.MediaErrors > 0 {
		return "failed"
	}
	return "ok"
}

func smartDetail(smart DiskSMART) string {
	if !smart.Available {
		return "smartctl not installed or no SMART output"
	}
	parts := []string{"SMART passed"}
	if smart.LifeUsedPercent != nil {
		parts = append(parts, fmt.Sprintf("寿命已用 %d%%", *smart.LifeUsedPercent))
	}
	if smart.PowerOnHours > 0 {
		parts = append(parts, fmt.Sprintf("通电 %dh", smart.PowerOnHours))
	}
	if smart.WrittenDataBytes > 0 {
		parts = append(parts, fmt.Sprintf("写入 %s", formatBytesText(smart.WrittenDataBytes)))
	}
	if smart.ReadDataBytes > 0 {
		parts = append(parts, fmt.Sprintf("读取 %s", formatBytesText(smart.ReadDataBytes)))
	}
	if smart.MediaErrors > 0 {
		parts = append(parts, fmt.Sprintf("介质错误 %d", smart.MediaErrors))
	}
	return strings.Join(parts, " | ")
}

// --- SMART 探测 ---

type smartctlOutput struct {
	SmartStatus *struct {
		Passed bool `json:"passed"`
	} `json:"smart_status"`
	PowerOnTime struct {
		Hours int64 `json:"hours"`
	} `json:"power_on_time"`
	PowerCycleCount    int64 `json:"power_cycle_count"`
	ATASmartAttributes struct {
		Table []smartctlAttribute `json:"table"`
	} `json:"ata_smart_attributes"`
	NVMe struct {
		PercentageUsed    uint64 `json:"percentage_used"`
		DataUnitsRead     uint64 `json:"data_units_read"`
		DataUnitsWritten  uint64 `json:"data_units_written"`
		HostReadCommands  uint64 `json:"host_reads"`
		HostWriteCommands uint64 `json:"host_writes"`
		PowerOnHours      uint64 `json:"power_on_hours"`
		PowerCycles       uint64 `json:"power_cycles"`
		MediaErrors       uint64 `json:"media_errors"`
	} `json:"nvme_smart_health_information_log"`
}

type smartctlAttribute struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Value int    `json:"value"`
	Raw   struct {
		Value  json.Number `json:"value"`
		String string      `json:"string"`
	} `json:"raw"`
}

func detectDiskSMART(path string) DiskSMART {
	smart := DiskSMART{}
	if !commandExists("smartctl") {
		return smart
	}

	out := runCommandOutput(8*time.Second, "smartctl", "-a", "-j", path)
	if strings.TrimSpace(out) == "" {
		return smart
	}

	var data smartctlOutput
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		return smart
	}

	smart.Available = true
	smart.PowerOnHours = data.PowerOnTime.Hours
	smart.PowerCycleCount = data.PowerCycleCount

	if data.NVMe.PowerOnHours > 0 {
		smart.PowerOnHours = uint64ToInt64(data.NVMe.PowerOnHours)
	}
	if data.NVMe.PowerCycles > 0 {
		smart.PowerCycleCount = uint64ToInt64(data.NVMe.PowerCycles)
	}
	if data.NVMe.PercentageUsed > 0 {
		if used, ok := smartPercentToInt(data.NVMe.PercentageUsed); ok {
			smart.LifeUsedPercent = &used
		}
	}
	smart.ReadDataBytes = data.NVMe.DataUnitsRead * 512000
	smart.WrittenDataBytes = data.NVMe.DataUnitsWritten * 512000
	smart.ReadCommands = data.NVMe.HostReadCommands
	smart.WriteCommands = data.NVMe.HostWriteCommands
	smart.MediaErrors = data.NVMe.MediaErrors

	parseATAAttributes(&smart, data.ATASmartAttributes.Table)
	if data.SmartStatus != nil && !data.SmartStatus.Passed {
		smart.MediaErrors++
	}
	return smart
}

func parseATAAttributes(smart *DiskSMART, attrs []smartctlAttribute) {
	for _, attr := range attrs {
		name := normalizeSMARTAttrName(attr.Name)
		raw := smartAttrRawUint(attr)
		rawText := smartAttrRawText(attr)
		switch name {
		case "poweronhours":
			if smart.PowerOnHours == 0 {
				if parsed, ok := smartAttrRawInt64(attr); ok {
					smart.PowerOnHours = parsed
				}
			}
		case "powercyclecount":
			if smart.PowerCycleCount == 0 {
				if parsed, ok := smartAttrRawInt64(attr); ok {
					smart.PowerCycleCount = parsed
				}
			}
		case "totallbaswritten":
			if smart.WrittenDataBytes == 0 {
				smart.WrittenDataBytes = raw * 512
			}
		case "totallbasread":
			if smart.ReadDataBytes == 0 {
				smart.ReadDataBytes = raw * 512
			}
		case "hostwrites32mib":
			if smart.WrittenDataBytes == 0 {
				smart.WrittenDataBytes = raw * 32 * 1024 * 1024
			}
		case "hostreads32mib":
			if smart.ReadDataBytes == 0 {
				smart.ReadDataBytes = raw * 32 * 1024 * 1024
			}
		case "hostwritecommands":
			smart.WriteCommands = raw
		case "hostreadcommands":
			smart.ReadCommands = raw
		case "wearlevelingcount":
			smart.WearLevelingCount = rawText
			if smart.LifeUsedPercent == nil && attr.Value > 0 && attr.Value <= 100 {
				used := 100 - attr.Value
				if used < 0 {
					used = 0
				}
				smart.LifeUsedPercent = &used
			}
		case "percentlifetimeremain", "mediawearoutindicator":
			if smart.LifeUsedPercent == nil {
				remaining := attr.Value
				if raw > 0 && raw <= 100 {
					if parsed, ok := smartPercentToInt(raw); ok {
						remaining = parsed
					}
				}
				used := 100 - remaining
				if used < 0 {
					used = 0
				}
				if used <= 100 {
					smart.LifeUsedPercent = &used
				}
			}
		case "percentageused":
			if smart.LifeUsedPercent == nil && raw <= 255 {
				if used, ok := smartPercentToInt(raw); ok {
					smart.LifeUsedPercent = &used
				}
			}
		case "erasefailcounttotal", "erasecount", "nandwrites", "programfailcnttotal":
			if smart.EraseCount == "" && rawText != "" {
				smart.EraseCount = rawText
			}
		case "mediaerrors":
			smart.MediaErrors = raw
		}
	}
}

func uint64ToInt64(value uint64) int64 {
	if value > 9223372036854775807 {
		return 0
	}
	return int64(value)
}

func smartPercentToInt(value uint64) (int, bool) {
	if value > 2147483647 {
		return 0, false
	}
	return int(value), true
}

func normalizeSMARTAttrName(name string) string {
	name = strings.ToLower(name)
	var b strings.Builder
	for _, ch := range name {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

func smartAttrRawText(attr smartctlAttribute) string {
	if attr.Raw.String != "" {
		return attr.Raw.String
	}
	if attr.Raw.Value != "" {
		return attr.Raw.Value.String()
	}
	return ""
}

func smartAttrRawInt64(attr smartctlAttribute) (int64, bool) {
	text := smartAttrRawText(attr)
	if value, err := strconv.ParseInt(text, 10, 64); err == nil {
		return value, true
	}
	digits := firstUintText(text)
	if digits == "" {
		return 0, false
	}
	value, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

func smartAttrRawUint(attr smartctlAttribute) uint64 {
	text := smartAttrRawText(attr)
	if value, err := strconv.ParseUint(text, 10, 64); err == nil {
		return value
	}
	digits := firstUintText(text)
	if digits == "" {
		return 0
	}
	value, _ := strconv.ParseUint(digits, 10, 64)
	return value
}

// --- 挂载点 ---

func detectMountpointsByDevice() map[string][]string {
	result := map[string][]string{}
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return result
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 || !strings.HasPrefix(fields[0], "/dev/") {
			continue
		}
		dev := strings.TrimPrefix(filepath.Base(fields[0]), "/dev/")
		parent := diskParentName(dev)
		result[parent] = append(result[parent], fields[1])
	}
	return result
}

func diskParentName(dev string) string {
	for _, suffix := range []string{"p1", "p2", "p3", "p4", "p5", "p6", "p7", "p8", "p9"} {
		if strings.HasSuffix(dev, suffix) && strings.HasPrefix(dev, "nvme") {
			return strings.TrimSuffix(dev, suffix)
		}
	}
	for len(dev) > 0 && dev[len(dev)-1] >= '0' && dev[len(dev)-1] <= '9' {
		dev = dev[:len(dev)-1]
	}
	return dev
}
