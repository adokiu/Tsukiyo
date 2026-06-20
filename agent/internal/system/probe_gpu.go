package system

import (
	"strings"
	"time"
)

// probeGPUs 探测显卡信息（静态数据，启动时采集一次）
func probeGPUs() []GPUInfo {
	gpus := make([]GPUInfo, 0)
	out := runCommandOutput(3*time.Second, "sh", "-c",
		"lspci -nnk 2>/dev/null | grep -iEA3 'vga|3d|display' || true")

	var current *GPUInfo
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "vga") || strings.Contains(lower, "3d controller") || strings.Contains(lower, "display controller") {
			gpus = append(gpus, GPUInfo{
				Name:   trimmed,
				Vendor: detectGPUVendor(trimmed),
				Type:   detectGPUType(trimmed),
			})
			current = &gpus[len(gpus)-1]
			continue
		}
		if current != nil && strings.HasPrefix(trimmed, "Kernel driver in use:") {
			current.Driver = strings.TrimSpace(strings.TrimPrefix(trimmed, "Kernel driver in use:"))
		}
	}
	return gpus
}

func detectGPUVendor(value string) string {
	lower := strings.ToLower(value)
	switch {
	case strings.Contains(lower, "intel"):
		return "Intel"
	case strings.Contains(lower, "nvidia"):
		return "NVIDIA"
	case strings.Contains(lower, "amd") || strings.Contains(lower, "ati"):
		return "AMD"
	default:
		return "Unknown"
	}
}

func detectGPUType(value string) string {
	lower := strings.ToLower(value)
	if strings.Contains(lower, "intel") {
		return "integrated"
	}
	return "discrete"
}

func hasIntegratedGPU(gpus []GPUInfo) bool {
	for _, gpu := range gpus {
		if gpu.Type == "integrated" {
			return true
		}
	}
	return false
}
