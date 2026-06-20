package task

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

func (e *Executor) handleFormatDisk(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		TaskID string `json:"task_id"`
		Device string `json:"device"`
		Type   string `json:"type"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("解析格式化磁盘参数失败: %w", err)
	}

	e.wsClient.SendTaskLog(req.TaskID, "info", fmt.Sprintf("开始格式化磁盘 %s 为 %s", req.Device, req.Type))
	zap.L().Info("开始格式化磁盘", zap.String("device", req.Device), zap.String("type", req.Type))

	if req.Device == "" {
		return nil, fmt.Errorf("设备路径不能为空")
	}
	if !strings.HasPrefix(req.Device, "/dev/") {
		return nil, fmt.Errorf("无效的设备路径: %s", req.Device)
	}

	checkCmd := exec.Command("lsblk", "-no", "MOUNTPOINT", req.Device)
	checkOut, err := checkCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("检查设备 %s 挂载状态失败: %w", req.Device, err)
	}
	mountPoints := strings.TrimSpace(string(checkOut))
	for _, mp := range strings.Split(mountPoints, "\n") {
		mp = strings.TrimSpace(mp)
		if mp == "/" || mp == "/boot" || mp == "/boot/efi" || strings.HasPrefix(mp, "/boot") {
			return nil, fmt.Errorf("设备 %s 包含系统分区挂载点 %s，禁止格式化", req.Device, mp)
		}
	}

	for _, mp := range strings.Split(mountPoints, "\n") {
		mp = strings.TrimSpace(mp)
		if mp != "" && mp != "/" && !strings.HasPrefix(mp, "/boot") {
			e.wsClient.SendTaskLog(req.TaskID, "info", fmt.Sprintf("卸载挂载点 %s", mp))
			umountCmd := exec.Command("umount", "-l", mp)
			umountCmd.CombinedOutput()
		}
	}

	zpoolCmd := exec.Command("zpool", "list", "-H", "-o", "name")
	zpoolOut, err := zpoolCmd.Output()
	if err == nil {
		zpoolLines := strings.Split(strings.TrimSpace(string(zpoolOut)), "\n")
		for _, poolName := range zpoolLines {
			poolName = strings.TrimSpace(poolName)
			if poolName == "" || poolName == "no pools available" {
				continue
			}
			statusCmd := exec.Command("zpool", "status", "-L", poolName)
			statusOut, err := statusCmd.Output()
			if err != nil {
				continue
			}
			if strings.Contains(string(statusOut), req.Device) {
				e.wsClient.SendTaskLog(req.TaskID, "info", fmt.Sprintf("设备被 ZFS pool %s 占用，正在 export", poolName))
				exportCmd := exec.Command("zpool", "export", "-f", poolName)
				if exportOut, err := exportCmd.CombinedOutput(); err != nil {
					e.wsClient.SendTaskLog(req.TaskID, "warn", fmt.Sprintf("export ZFS pool %s 失败: %s", poolName, strings.TrimSpace(string(exportOut))))
				} else {
					e.wsClient.SendTaskLog(req.TaskID, "info", fmt.Sprintf("ZFS pool %s 已 export", poolName))
				}
			}
		}
	}

	pvsCmd := exec.Command("pvs", "--noheadings", "-o", "pv_name")
	pvsOut, err := pvsCmd.Output()
	if err == nil {
		pvsLines := strings.Split(strings.TrimSpace(string(pvsOut)), "\n")
		for _, pvLine := range pvsLines {
			pvName := strings.TrimSpace(pvLine)
			if pvName == req.Device {
				e.wsClient.SendTaskLog(req.TaskID, "info", fmt.Sprintf("设备是 LVM PV，正在移除"))
				vgRemoveCmd := exec.Command("pvremove", "-ff", "-y", req.Device)
				vgRemoveCmd.CombinedOutput()
				break
			}
		}
	}

	pools, err := e.incusClient.ListStoragePools()
	if err == nil {
		for _, pool := range pools {
			if pool.Source == req.Device || pool.Source == strings.TrimPrefix(req.Device, "/dev/") {
				e.wsClient.SendTaskLog(req.TaskID, "info", fmt.Sprintf("设备被存储池 %s 占用，正在删除存储池", pool.Name))
				zap.L().Info("设备被存储池占用，先删除存储池", zap.String("device", req.Device), zap.String("pool", pool.Name))
				if err := e.incusClient.DeleteStoragePool(pool.Name); err != nil {
					e.wsClient.SendTaskLog(req.TaskID, "error", fmt.Sprintf("删除存储池 %s 失败: %v", pool.Name, err))
					return nil, fmt.Errorf("删除占用设备 %s 的存储池 %s 失败: %w", req.Device, pool.Name, err)
				}
				e.wsClient.SendTaskLog(req.TaskID, "info", fmt.Sprintf("存储池 %s 已删除", pool.Name))
				zap.L().Info("存储池已删除，继续格式化", zap.String("pool", pool.Name))
			}
		}
	}

	e.wsClient.SendTaskLog(req.TaskID, "info", "正在清除设备签名")
	wipeCmd := exec.Command("wipefs", "-a", req.Device)
	if wipeOut, err := wipeCmd.CombinedOutput(); err != nil {
		e.wsClient.SendTaskLog(req.TaskID, "error", fmt.Sprintf("清除签名失败: %s", strings.TrimSpace(string(wipeOut))))
		return nil, fmt.Errorf("wipefs 清除 %s 签名失败: %w, output: %s", req.Device, err, string(wipeOut))
	}

	switch req.Type {
	case "zfs":
		e.wsClient.SendTaskLog(req.TaskID, "info", "ZFS 模式，无需 mkfs")
		zap.L().Info("ZFS 模式无需 mkfs，设备已准备就绪", zap.String("device", req.Device))
	case "btrfs":
		e.wsClient.SendTaskLog(req.TaskID, "info", "正在创建 btrfs 文件系统")
		fmtCmd := exec.Command("mkfs.btrfs", "-f", req.Device)
		if out, err := fmtCmd.CombinedOutput(); err != nil {
			e.wsClient.SendTaskLog(req.TaskID, "error", fmt.Sprintf("btrfs 格式化失败: %s", strings.TrimSpace(string(out))))
			return nil, fmt.Errorf("btrfs 格式化 %s 失败: %w, output: %s", req.Device, err, string(out))
		}
	case "lvm", "lvm-thin":
		e.wsClient.SendTaskLog(req.TaskID, "info", "正在创建 LVM 物理卷")
		pvCmd := exec.Command("pvcreate", "-f", req.Device)
		if out, err := pvCmd.CombinedOutput(); err != nil {
			e.wsClient.SendTaskLog(req.TaskID, "error", fmt.Sprintf("pvcreate 失败: %s", strings.TrimSpace(string(out))))
			return nil, fmt.Errorf("pvcreate %s 失败: %w, output: %s", req.Device, err, string(out))
		}
	default:
		e.wsClient.SendTaskLog(req.TaskID, "info", "正在创建 ext4 文件系统")
		fmtCmd := exec.Command("mkfs.ext4", "-F", req.Device)
		if out, err := fmtCmd.CombinedOutput(); err != nil {
			e.wsClient.SendTaskLog(req.TaskID, "error", fmt.Sprintf("ext4 格式化失败: %s", strings.TrimSpace(string(out))))
			return nil, fmt.Errorf("ext4 格式化 %s 失败: %w, output: %s", req.Device, err, string(out))
		}
	}

	e.wsClient.SendTaskLog(req.TaskID, "info", fmt.Sprintf("磁盘 %s 格式化为 %s 完成", req.Device, req.Type))
	zap.L().Info("磁盘格式化完成", zap.String("device", req.Device), zap.String("type", req.Type))
	return json.Marshal(map[string]string{
		"status": "formatted",
		"device": req.Device,
		"type":   req.Type,
	})
}

func (e *Executor) handleInitStorage(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		TaskID       string `json:"task_id"`
		Name         string `json:"name"`
		Driver       string `json:"driver"`
		Source       string `json:"source"`
		Size         string `json:"size"`
		ThinpoolName string `json:"thinpool_name"`
		ZfsPoolName  string `json:"zfs_pool_name"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("解析初始化存储池参数失败: %w", err)
	}

	e.wsClient.SendTaskLog(req.TaskID, "info", fmt.Sprintf("开始初始化存储池 %s（驱动: %s）", req.Name, req.Driver))
	zap.L().Info("开始初始化存储池",
		zap.String("name", req.Name),
		zap.String("driver", req.Driver),
		zap.String("source", req.Source),
		zap.String("size", req.Size))

	if req.Name == "" {
		return nil, fmt.Errorf("存储池名称不能为空")
	}
	if req.Driver == "" {
		return nil, fmt.Errorf("存储池驱动不能为空")
	}

	if e.incusClient.StoragePoolExists(req.Name) {
		e.wsClient.SendTaskLog(req.TaskID, "info", fmt.Sprintf("存储池 %s 已存在，跳过创建", req.Name))
		zap.L().Info("存储池已存在，跳过创建", zap.String("name", req.Name))
		return json.Marshal(map[string]string{
			"status": "exists",
			"name":   req.Name,
		})
	}

	config := map[string]string{}
	if req.Source != "" {
		config["source"] = req.Source
	}
	if req.Size != "" {
		config["size"] = req.Size
	}
	if req.ThinpoolName != "" {
		config["lvm.thinpool_name"] = req.ThinpoolName
	}
	if req.ZfsPoolName != "" {
		config["zfs.pool_name"] = req.ZfsPoolName
	}

	if err := e.incusClient.CreateStoragePoolWithConfig(req.Name, req.Driver, config); err != nil {
		e.wsClient.SendTaskLog(req.TaskID, "error", fmt.Sprintf("创建存储池失败: %v", err))
		return nil, fmt.Errorf("创建存储池 %s 失败: %w", req.Name, err)
	}

	e.wsClient.SendTaskLog(req.TaskID, "info", fmt.Sprintf("存储池 %s 初始化完成", req.Name))
	zap.L().Info("存储池初始化完成", zap.String("name", req.Name), zap.String("driver", req.Driver))
	return json.Marshal(map[string]string{
		"status": "initialized",
		"name":   req.Name,
		"driver": req.Driver,
	})
}

// handleCreatePartition 创建磁盘分区
func (e *Executor) handleCreatePartition(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		TaskID string `json:"task_id"`
		Device string `json:"device"`
		SizeGB int    `json:"size_gb"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("解析创建分区参数失败: %w", err)
	}

	e.wsClient.SendTaskLog(req.TaskID, "info", fmt.Sprintf("开始在 %s 上创建 %dGB 分区", req.Device, req.SizeGB))

	if req.Device == "" {
		return nil, fmt.Errorf("设备路径不能为空")
	}
	if !strings.HasPrefix(req.Device, "/dev/") {
		return nil, fmt.Errorf("无效的设备路径: %s", req.Device)
	}
	if req.SizeGB <= 0 {
		return nil, fmt.Errorf("分区大小必须大于 0")
	}

	checkCmd := exec.Command("lsblk", "-no", "MOUNTPOINT", req.Device)
	checkOut, err := checkCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("检查设备 %s 挂载状态失败: %w", req.Device, err)
	}
	mountPoints := strings.TrimSpace(string(checkOut))
	for _, mp := range strings.Split(mountPoints, "\n") {
		mp = strings.TrimSpace(mp)
		if mp == "/" || mp == "/boot" || strings.HasPrefix(mp, "/boot") {
			return nil, fmt.Errorf("设备 %s 包含系统分区挂载点 %s，禁止操作", req.Device, mp)
		}
	}

	sizeStr := fmt.Sprintf("+%dG", req.SizeGB)
	cmd := exec.Command("sgdisk", "-n", "0:0:"+sizeStr, "-t", "0:8300", req.Device)
	out, err := cmd.CombinedOutput()
	if err != nil {
		e.wsClient.SendTaskLog(req.TaskID, "error", fmt.Sprintf("创建分区失败: %s", strings.TrimSpace(string(out))))
		return nil, fmt.Errorf("创建分区失败: %w, output: %s", err, string(out))
	}

	partprobeCmd := exec.Command("partprobe", req.Device)
	partprobeCmd.Run()

	listCmd := exec.Command("lsblk", "-no", "NAME", req.Device)
	listOut, err := listCmd.Output()
	if err != nil {
		return json.Marshal(map[string]string{
			"status": "created",
			"device": req.Device,
		})
	}
	lines := strings.Split(strings.TrimSpace(string(listOut)), "\n")
	var lastPart string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != strings.TrimPrefix(req.Device, "/dev/") && line != "" {
			lastPart = line
		}
	}
	partDevice := ""
	if lastPart != "" {
		partDevice = "/dev/" + lastPart
	}

	e.wsClient.SendTaskLog(req.TaskID, "info", fmt.Sprintf("分区 %s 创建成功", partDevice))
	zap.L().Info("创建分区成功", zap.String("device", req.Device), zap.String("partition", partDevice))
	return json.Marshal(map[string]string{
		"status":    "created",
		"device":    req.Device,
		"partition": partDevice,
	})
}

// handleDeletePartition 删除磁盘分区
func (e *Executor) handleDeletePartition(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		TaskID string `json:"task_id"`
		Device string `json:"device"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("解析删除分区参数失败: %w", err)
	}

	e.wsClient.SendTaskLog(req.TaskID, "info", fmt.Sprintf("开始删除分区 %s", req.Device))

	if req.Device == "" {
		return nil, fmt.Errorf("设备路径不能为空")
	}
	if !strings.HasPrefix(req.Device, "/dev/") {
		return nil, fmt.Errorf("无效的设备路径: %s", req.Device)
	}

	checkCmd := exec.Command("lsblk", "-no", "MOUNTPOINT", req.Device)
	checkOut, err := checkCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("检查设备 %s 挂载状态失败: %w", req.Device, err)
	}
	mountPoints := strings.TrimSpace(string(checkOut))
	for _, mp := range strings.Split(mountPoints, "\n") {
		mp = strings.TrimSpace(mp)
		if mp == "/" || mp == "/boot" || strings.HasPrefix(mp, "/boot") {
			return nil, fmt.Errorf("设备 %s 是系统分区，禁止删除", req.Device)
		}
	}

	wipeCmd := exec.Command("wipefs", "-a", req.Device)
	wipeCmd.CombinedOutput()

	devName := strings.TrimPrefix(req.Device, "/dev/")
	parentDisk := ""
	partNum := ""

	if strings.Contains(devName, "p") && strings.HasPrefix(devName, "nvme") {
		idx := strings.LastIndex(devName, "p")
		parentDisk = devName[:idx]
		partNum = devName[idx+1:]
	} else {
		for i := len(devName) - 1; i >= 0; i-- {
			if devName[i] < '0' || devName[i] > '9' {
				parentDisk = devName[:i+1]
				partNum = devName[i+1:]
				break
			}
		}
	}

	if parentDisk == "" || partNum == "" {
		return nil, fmt.Errorf("无法解析分区设备名: %s", req.Device)
	}

	cmd := exec.Command("sgdisk", "-d", partNum, "/dev/"+parentDisk)
	out, err := cmd.CombinedOutput()
	if err != nil {
		e.wsClient.SendTaskLog(req.TaskID, "error", fmt.Sprintf("删除分区失败: %s", strings.TrimSpace(string(out))))
		return nil, fmt.Errorf("删除分区失败: %w, output: %s", err, string(out))
	}

	partprobeCmd := exec.Command("partprobe", "/dev/"+parentDisk)
	partprobeCmd.Run()

	e.wsClient.SendTaskLog(req.TaskID, "info", fmt.Sprintf("分区 %s 已删除", req.Device))
	zap.L().Info("删除分区成功", zap.String("device", req.Device))
	return json.Marshal(map[string]string{
		"status": "deleted",
		"device": req.Device,
	})
}

// handleDeleteStorage 删除存储池
func (e *Executor) handleDeleteStorage(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		TaskID string `json:"task_id"`
		Name   string `json:"name"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("解析删除存储池参数失败: %w", err)
	}

	if req.Name == "" {
		return nil, fmt.Errorf("存储池名称不能为空")
	}

	e.wsClient.SendTaskLog(req.TaskID, "info", fmt.Sprintf("开始删除存储池 %s", req.Name))

	detail, err := e.incusClient.GetStoragePool(req.Name)
	if err != nil {
		e.wsClient.SendTaskLog(req.TaskID, "error", fmt.Sprintf("获取存储池信息失败: %v", err))
		return nil, fmt.Errorf("获取存储池 %s 信息失败: %w", req.Name, err)
	}

	if len(detail.UsedBy) > 0 {
		e.wsClient.SendTaskLog(req.TaskID, "error", fmt.Sprintf("存储池 %s 仍有 %d 个资源在使用，无法删除", req.Name, len(detail.UsedBy)))
		return nil, fmt.Errorf("存储池 %s 仍有 %d 个资源在使用，无法删除", req.Name, len(detail.UsedBy))
	}

	e.wsClient.SendTaskLog(req.TaskID, "info", "正在删除存储池")
	if err := e.incusClient.DeleteStoragePool(req.Name); err != nil {
		e.wsClient.SendTaskLog(req.TaskID, "error", fmt.Sprintf("删除存储池失败: %v", err))
		return nil, fmt.Errorf("删除存储池 %s 失败: %w", req.Name, err)
	}

	e.wsClient.SendTaskLog(req.TaskID, "info", fmt.Sprintf("存储池 %s 已删除", req.Name))
	zap.L().Info("删除存储池成功", zap.String("name", req.Name))
	return json.Marshal(map[string]interface{}{
		"status":  "deleted",
		"name":    req.Name,
		"deleted": true,
	})
}
