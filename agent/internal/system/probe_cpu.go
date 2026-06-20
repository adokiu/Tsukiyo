package system

import (
	"bufio"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// probeCPU 探测 CPU 信息（静态数据，启动时采集一次）
func probeCPU() CPUInfo {
	probe := CPUInfo{
		Cores:        runtime.NumCPU(),
		Threads:      runtime.NumCPU(),
		Architecture: runtime.GOARCH,
	}

	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return probe
	}
	defer f.Close()

	seenFlags := map[string]bool{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.SplitN(line, ":", 2)
		if len(fields) != 2 {
			continue
		}
		key := strings.TrimSpace(fields[0])
		value := strings.TrimSpace(fields[1])

		switch key {
		case "model name", "Hardware", "Processor":
			if probe.Model == "" {
				probe.Model = value
			}
		case "cpu cores":
			if cores, err := strconv.Atoi(value); err == nil && cores > probe.Cores {
				probe.Cores = cores
			}
		case "flags", "Features":
			for _, flag := range strings.Fields(value) {
				if flag == "vmx" || flag == "svm" {
					probe.Virtualization = true
					probe.VirtualizationKey = flag
				}
				if !seenFlags[flag] {
					seenFlags[flag] = true
					probe.Flags = append(probe.Flags, flag)
				}
			}
		}
	}

	if probe.Model == "" {
		probe.Model = "Unknown"
	}
	sort.Strings(probe.Flags)
	return probe
}
