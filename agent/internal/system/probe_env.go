package system

import (
	"strings"
	"time"
)

// probeEnvironment 探测环境工具（静态数据，启动时采集一次）
// 仅检测当前项目（Incus 容器管理）所需的环境依赖
func probeEnvironment() []EnvCheck {
	checks := []EnvCheck{
		commandCheck("systemd", "服务管理器 systemd", true, "systemctl"),
		commandCheck("incus", "Incus 容器管理", true, "incus"),
		commandCheck("nft", "nftables 网络规则", true, "nft"),
		commandCheck("ip", "iproute2 网络工具", true, "ip"),
		commandCheck("smartctl", "硬盘健康检测", false, "smartctl"),
	}

	// 存储后端可用性检测
	checks = append(checks,
		storageCheck("btrfs", "BTRFS 存储", "btrfs"),
		storageCheck("lvm-thin", "LVM Thin 存储", "lvm"),
		storageCheck("zfs", "ZFS 存储", "zfs"),
	)

	checks = append(checks, EnvCheck{
		Key:      "dev-kvm",
		Label:    "/dev/kvm 硬件虚拟化",
		OK:       fileExists("/dev/kvm"),
		Required: false,
		Detail:   boolDetail(fileExists("/dev/kvm")),
	})

	checks = append(checks, EnvCheck{
		Key:      "ipv4-forward",
		Label:    "IPv4 转发",
		OK:       strings.TrimSpace(readFirstExistingFile("/proc/sys/net/ipv4/ip_forward")) == "1",
		Required: true,
		Detail:   strings.TrimSpace(readFirstExistingFile("/proc/sys/net/ipv4/ip_forward")),
	})

	return checks
}

func commandCheck(key, label string, required bool, cmd string) EnvCheck {
	ok := commandExists(cmd)
	detail := "missing"
	if ok {
		detail = commandVersionDetail(cmd)
		if detail == "" {
			detail = "installed"
		}
	}
	return EnvCheck{Key: key, Label: label, OK: ok, Required: required, Detail: detail}
}

func commandVersionDetail(cmd string) string {
	switch cmd {
	case "ip":
		return strings.TrimSpace(runCommandOutput(2*time.Second, "sh", "-c", "ip -V 2>&1 | head -n 1"))
	case "incus":
		return strings.TrimSpace(runCommandOutput(3*time.Second, "incus", "version"))
	default:
		return strings.TrimSpace(runCommandOutput(2*time.Second, "sh", "-c", cmd+" --version 2>&1 | head -n 1"))
	}
}

// storageCheck 检测存储后端可用性：命令存在 + 内核模块加载
func storageCheck(key, label, cmd string) EnvCheck {
	cmdOK := commandExists(cmd)
	modOK := kernelModuleLoaded(key)
	ok := cmdOK && modOK

	var details []string
	if cmdOK {
		v := commandVersionDetail(cmd)
		if v == "" {
			v = "installed"
		}
		details = append(details, v)
	} else {
		details = append(details, "command missing")
	}
	if modOK {
		details = append(details, "module loaded")
	} else {
		details = append(details, "module not loaded")
	}

	return EnvCheck{
		Key:      key,
		Label:    label,
		OK:       ok,
		Required: false,
		Detail:   strings.Join(details, ", "),
	}
}

// kernelModuleLoaded 检测内核模块是否加载
func kernelModuleLoaded(name string) bool {
	switch name {
	case "btrfs":
		return fileExists("/proc/fs/btrfs") || fileExists("/sys/module/btrfs")
	case "lvm-thin":
		return fileExists("/sys/module/dm_thin_pool") || commandExists("lvs")
	case "zfs":
		return fileExists("/sys/module/zfs") || fileExists("/proc/fs/zfs")
	default:
		return fileExists("/sys/module/" + name)
	}
}
