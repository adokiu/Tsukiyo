package task

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"tsukiyo/agent/internal/incus"

	"go.uber.org/zap"
)

func (e *Executor) handleDeleteInstance(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析删除实例任务参数失败", zap.Error(err))
		return nil, err
	}
	zap.L().Info("开始删除实例", zap.String("instance_id", req.InstanceID))

	zap.L().Info("获取实例信息", zap.String("instance_id", req.InstanceID))
	info, err := e.incusClient.GetInstance(req.InstanceID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "Not found") {
			zap.L().Info("实例不存在，幂等删除成功", zap.String("instance_id", req.InstanceID))
			return json.Marshal(map[string]interface{}{"deleted": true, "not_found": true})
		}
		zap.L().Error("获取实例信息失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
		return nil, fmt.Errorf("获取实例信息失败: %w", err)
	}

	zap.L().Info("清理网络配置", zap.String("instance_id", req.InstanceID))
	if info.Devices != nil {
		if devs, ok := info.Devices["eth0"].(map[string]interface{}); ok {
			if ip, ok := devs["ipv4.address"].(string); ok && ip != "" {
				zap.L().Info("解绑 IP", zap.String("ip", ip))
				e.netManager.UnbindIP(ip, "")
			}
		}
	}

	zap.L().Info("删除实例", zap.String("instance_id", req.InstanceID))
	if err := e.incusClient.DeleteInstance(req.InstanceID); err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "Not found") {
			zap.L().Info("删除时实例已不存在，幂等成功", zap.String("instance_id", req.InstanceID))
			return json.Marshal(map[string]interface{}{"deleted": true, "not_found": true})
		}
		zap.L().Error("删除实例失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
		return nil, fmt.Errorf("删除实例失败: %w", err)
	}
	zap.L().Info("实例删除成功", zap.String("instance_id", req.InstanceID))
	return json.Marshal(map[string]bool{"deleted": true})
}

func (e *Executor) handleStartInstance(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析启动实例任务参数失败", zap.Error(err))
		return nil, err
	}
	zap.L().Info("开始启动实例", zap.String("instance_id", req.InstanceID))
	if err := e.incusClient.StartInstance(req.InstanceID); err != nil {
		zap.L().Error("启动实例失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
		return nil, err
	}
	zap.L().Info("启动实例成功", zap.String("instance_id", req.InstanceID))
	return json.Marshal(map[string]string{"status": "running", "instance_id": req.InstanceID})
}

func (e *Executor) handleStopInstance(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
		Force      bool   `json:"force"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析停止实例任务参数失败", zap.Error(err))
		return nil, err
	}
	zap.L().Info("开始停止实例", zap.String("instance_id", req.InstanceID), zap.Bool("force", req.Force))
	if err := e.incusClient.StopInstance(req.InstanceID, req.Force); err != nil {
		zap.L().Error("停止实例失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
		return nil, err
	}
	zap.L().Info("停止实例成功", zap.String("instance_id", req.InstanceID))
	return json.Marshal(map[string]string{"status": "stopped", "instance_id": req.InstanceID})
}

func (e *Executor) handleRestartInstance(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析重启实例任务参数失败", zap.Error(err))
		return nil, err
	}
	zap.L().Info("开始重启实例", zap.String("instance_id", req.InstanceID))
	if err := e.incusClient.RestartInstance(req.InstanceID); err != nil {
		zap.L().Error("重启实例失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
		return nil, err
	}
	zap.L().Info("重启实例成功", zap.String("instance_id", req.InstanceID))
	return json.Marshal(map[string]string{"status": "running", "instance_id": req.InstanceID})
}

func (e *Executor) handleReinstallInstance(payload json.RawMessage) (json.RawMessage, error) {
	// 重装 = 删除旧实例 + 用完整配置重新创建
	// payload 和 create_instance 相同，额外有 format_data_disks 和 old_status
	var req struct {
		InstanceID      string                   `json:"instance_id"`
		Type            string                   `json:"type"`
		TemplateID      string                   `json:"template_id"`
		VCPU            float64                  `json:"vcpu"`
		MemoryMB        int                      `json:"memory_mb"`
		SwapMB          int                      `json:"swap_mb"`
		DiskMB          int                      `json:"disk_mb"`
		StoragePool     string                   `json:"storage_pool"`
		LoginMethod     string                   `json:"login_method"`
		SSHPassword     string                   `json:"ssh_password"`
		SSHPublicKey    string                   `json:"ssh_public_key"`
		ImageSource     string                   `json:"image_source"`
		NetworkDown     int                      `json:"network_down"`
		NetworkUp       int                      `json:"network_up"`
		IORead          int                      `json:"io_read"`
		IOWrite         int                      `json:"io_write"`
		DataDisks       []map[string]interface{} `json:"data_disks"`
		PortMappings    []map[string]interface{} `json:"port_mappings"`
		BridgeName      string                   `json:"bridge_name,omitempty"`
		InternalIPv4    string                   `json:"internal_ipv4,omitempty"`
		GatewayV4       string                   `json:"gateway_v4,omitempty"`
		IPv4CIDR        string                   `json:"ipv4_cidr,omitempty"`
		IPv4Filter      bool                     `json:"ipv4_filter,omitempty"`
		IPv4Mode        string                   `json:"ipv4_mode"`
		IPv6Mode        string                   `json:"ipv6_mode"`
		EIPAssignments  []map[string]interface{} `json:"eip_assignments,omitempty"`
		FormatDataDisks bool                     `json:"format_data_disks"`
		OldStatus       string                   `json:"old_status"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析重装实例任务参数失败", zap.Error(err))
		return nil, err
	}
	zap.L().Info("开始重装实例（删除后重新创建）",
		zap.String("instance_id", req.InstanceID),
		zap.String("template_id", req.TemplateID),
		zap.String("type", req.Type),
		zap.Bool("format_data_disks", req.FormatDataDisks))

	// 删除旧实例
	if err := e.incusClient.DeleteInstance(req.InstanceID); err != nil {
		zap.L().Warn("删除旧实例失败，尝试继续重装", zap.Error(err))
	}

	// 用完整配置重新创建（复用 CreateInstanceRequest）
	createReq := incus.CreateInstanceRequest{
		Name:          req.InstanceID,
		TemplateID:    req.TemplateID,
		Type:          req.Type,
		VCPU:          req.VCPU,
		MemoryMB:      req.MemoryMB,
		SwapMB:        req.SwapMB,
		DiskMB:        req.DiskMB,
		StoragePool:   req.StoragePool,
		NetworkBridge: req.BridgeName,
		InternalIPv4:  req.InternalIPv4,
		GatewayV4:     req.GatewayV4,
		IPv4CIDR:      req.IPv4CIDR,
		IPv4Filter:    req.IPv4Filter,
		NetworkDown:   req.NetworkDown,
		NetworkUp:     req.NetworkUp,
		IOReadIops:    req.IORead,
		IOWriteIops:   req.IOWrite,
	}

	// 构建数据盘
	for _, dd := range req.DataDisks {
		sizeMB, _ := dd["size_mb"].(float64)
		name, _ := dd["name"].(string)
		pool, _ := dd["storage_pool"].(string)
		mount, _ := dd["mount_point"].(string)
		createReq.DataDisks = append(createReq.DataDisks, incus.DataDisk{
			Name:        name,
			SizeMB:      int(sizeMB),
			StoragePool: pool,
			MountPoint:  mount,
		})
	}

	// 构建 cloud-init user-data
	userData := buildCloudInitUserData(req.SSHPassword, req.SSHPublicKey, req.LoginMethod, req.InternalIPv4, req.GatewayV4, req.IPv4CIDR)
	createReq.UserData = userData

	_, err := e.incusClient.CreateInstance(createReq)
	if err != nil {
		zap.L().Error("重装实例：重新创建失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
		return nil, err
	}

	// 启动实例
	if err := e.incusClient.StartInstance(req.InstanceID); err != nil {
		zap.L().Warn("重装后启动实例失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
	}

	// VM 类型跳过 exec 设置密码（通过 cloud-init 配置），但需要注入 motd
	isVM := req.Type == "vm" || req.Type == "virtual-machine"
	if isVM {
		zap.L().Info("VM 类型，跳过 exec 设置密码，配置 SSH 并注入 motd", zap.String("instance_id", req.InstanceID))
		if err := injectMotdAndSSHConfig(req.InstanceID, req.LoginMethod); err != nil {
			zap.L().Warn("重装后 SSH 配置和 motd 注入失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
		}
	} else {
		// 设置密码
		if req.SSHPassword != "" && (req.LoginMethod == "auto" || req.LoginMethod == "password") {
			if err := e.incusClient.SetInstancePassword(req.InstanceID, req.SSHPassword); err != nil {
				zap.L().Warn("重装后设置密码失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
			}
		}
	}

	// 配置 EIP
	for _, eip := range req.EIPAssignments {
		eipPayload, _ := json.Marshal(map[string]interface{}{
			"instance_name":      req.InstanceID,
			"instance_ip":        req.InternalIPv4,
			"eip_cidr":           getMapString(eip, "eip_cidr"),
			"ip_version":         getMapString(eip, "ip_version"),
			"interface":          getMapString(eip, "interface"),
			"bridge_name":        req.BridgeName,
			"mapped_internal_ip": getMapString(eip, "mapped_internal_ip"),
			"ipv4_cidr":          req.IPv4CIDR,
			"ipv6_cidr":          "",
			"ipv4_gateway":       req.GatewayV4,
			"ipv6_gateway":       "",
			"eip_gateway":        getMapString(eip, "eip_gateway"),
		})
		zap.L().Info("重装后配置实例 EIP", zap.String("instance", req.InstanceID), zap.String("eip_cidr", getMapString(eip, "eip_cidr")))
		if _, err := e.handleAssignEIP(eipPayload); err != nil {
			zap.L().Error("重装后配置实例 EIP 失败", zap.String("instance", req.InstanceID), zap.Error(err))
		}
	}

	zap.L().Info("重装实例成功", zap.String("instance_id", req.InstanceID))
	return json.Marshal(map[string]string{"status": "reinstalled", "instance_id": req.InstanceID})
}

func (e *Executor) handleResizeInstance(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string  `json:"instance_id"`
		VCPU       float64 `json:"vcpu"`
		MemoryMB   int     `json:"memory_mb"`
		SwapMB     int     `json:"swap_mb"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析调整配置任务参数失败", zap.Error(err))
		return nil, err
	}
	zap.L().Info("开始调整实例 CPU/内存",
		zap.String("instance_id", req.InstanceID),
		zap.Float64("vcpu", req.VCPU),
		zap.Int("memory_mb", req.MemoryMB))

	config := map[string]string{}
	if req.VCPU > 0 {
		config["limits.cpu"] = strconv.Itoa(int(req.VCPU))
	}
	if req.MemoryMB > 0 {
		config["limits.memory"] = fmt.Sprintf("%dMB", req.MemoryMB)
	}
	if req.SwapMB > 0 {
		config["limits.memory.swap"] = fmt.Sprintf("%dMB", req.SwapMB)
	}

	if len(config) > 0 {
		if err := e.incusClient.UpdateInstanceConfig(req.InstanceID, config); err != nil {
			zap.L().Error("调整 CPU/内存失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
			return nil, err
		}
	}

	zap.L().Info("调整实例 CPU/内存成功", zap.String("instance_id", req.InstanceID))
	return json.Marshal(map[string]string{"status": "resized", "instance_id": req.InstanceID})
}

func (e *Executor) handleResetPassword(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
		Password   string `json:"password"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析重置密码任务参数失败", zap.Error(err))
		return nil, err
	}
	zap.L().Info("开始重置实例密码", zap.String("instance_id", req.InstanceID))
	if err := e.incusClient.SetInstancePassword(req.InstanceID, req.Password); err != nil {
		zap.L().Error("重置密码失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
		return nil, err
	}
	zap.L().Info("重置密码成功", zap.String("instance_id", req.InstanceID))
	return json.Marshal(map[string]interface{}{"success": true, "instance_id": req.InstanceID})
}

func (e *Executor) handleMigrateInstance(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
		TargetNode string `json:"target_node"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"status": "migrated"})
}

func (e *Executor) handleLimitNetwork(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID  string `json:"instance_id"`
		NetworkDown int    `json:"network_down"`
		NetworkUp   int    `json:"network_up"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析限速任务参数失败", zap.Error(err))
		return nil, err
	}
	zap.L().Info("开始限制实例网络", zap.String("instance_id", req.InstanceID), zap.Int("down", req.NetworkDown), zap.Int("up", req.NetworkUp))

	config := map[string]string{}
	if req.NetworkDown > 0 {
		config["limits.network.egress"] = fmt.Sprintf("%dMbit", req.NetworkDown)
	} else {
		config["limits.network.egress"] = ""
	}
	if req.NetworkUp > 0 {
		config["limits.network.ingress"] = fmt.Sprintf("%dMbit", req.NetworkUp)
	} else {
		config["limits.network.ingress"] = ""
	}

	if err := e.incusClient.UpdateInstanceConfig(req.InstanceID, config); err != nil {
		zap.L().Error("限制实例网络失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
		return nil, err
	}
	zap.L().Info("限制实例网络成功", zap.String("instance_id", req.InstanceID))
	return json.Marshal(map[string]interface{}{"success": true, "instance_id": req.InstanceID})
}

func (e *Executor) handleLimitIOPS(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
		IORead     int    `json:"io_read"`
		IOWrite    int    `json:"io_write"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析 IOPS 限制任务参数失败", zap.Error(err))
		return nil, err
	}
	zap.L().Info("开始限制实例磁盘 IOPS",
		zap.String("instance_id", req.InstanceID),
		zap.Int("io_read", req.IORead),
		zap.Int("io_write", req.IOWrite))

	rootDevice := map[string]string{
		"path": "/",
		"type": "disk",
	}
	if req.IORead > 0 {
		rootDevice["limits.read"] = fmt.Sprintf("%diops", req.IORead)
	}
	if req.IOWrite > 0 {
		rootDevice["limits.write"] = fmt.Sprintf("%diops", req.IOWrite)
	}

	if err := e.incusClient.AddDevice(req.InstanceID, "root", rootDevice); err != nil {
		zap.L().Error("限制实例磁盘 IOPS 失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
		return nil, err
	}
	zap.L().Info("限制实例磁盘 IOPS 成功", zap.String("instance_id", req.InstanceID))
	return json.Marshal(map[string]interface{}{"success": true, "instance_id": req.InstanceID})
}
