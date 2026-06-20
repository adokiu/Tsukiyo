package task

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"

	"tsukiyo/agent/internal/incus"
	"tsukiyo/agent/internal/ws"
)

func (e *Executor) handleCreateInstance(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		TaskID           string                   `json:"task_id"`
		InstanceID       string                   `json:"instance_id"`
		Type             string                   `json:"type"`
		TemplateID       string                   `json:"template_id"`
		VCPU             float64                  `json:"vcpu"`
		MemoryMB         int                      `json:"memory_mb"`
		DiskGB           int                      `json:"disk_gb"`
		StoragePool      string                   `json:"storage_pool"`
		LoginMethod      string                   `json:"login_method"`
		SSHPassword      string                   `json:"ssh_password"`
		SSHPublicKey     string                   `json:"ssh_public_key"`
		ImageSource      string                   `json:"image_source"`
		IPv4Address      string                   `json:"ipv4_address"`
		IPv6Address      string                   `json:"ipv6_address"`
		NetworkDown      int                      `json:"network_down"`
		NetworkUp        int                      `json:"network_up"`
		IORead           int                      `json:"io_read"`
		IOWrite          int                      `json:"io_write"`
		DataDisks        []map[string]interface{} `json:"data_disks"`
		NATs             []map[string]interface{} `json:"nats"`
		PortMappings     []map[string]interface{} `json:"port_mappings"`
		MonthlyTraffic   int64                    `json:"monthly_traffic"`
		TrafficMode      string                   `json:"traffic_mode"`
		SnapshotLimit    int                      `json:"snapshot_limit"`
		AssignNAT        bool                     `json:"assign_nat"`
		PortMappingCount int                      `json:"port_mapping_count"`
		ExtraPorts       []int                    `json:"extra_ports"`
		BridgeID         string                   `json:"bridge_id,omitempty"`
		InternalIPv4     string                   `json:"internal_ipv4,omitempty"`
		GatewayV4        string                   `json:"gateway_v4,omitempty"`
		IPv4CIDR         string                   `json:"ipv4_cidr,omitempty"`
		BridgeName       string                   `json:"bridge_name,omitempty"`
		IPv4Filter       bool                     `json:"ipv4_filter,omitempty"`
		MACFilter        bool                     `json:"mac_filter,omitempty"`
		EgressV4Primary  string                   `json:"egress_v4_primary,omitempty"`
		ParentIface      string                   `json:"parent_iface,omitempty"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("创建实例参数解析失败", zap.Error(err))
		return nil, fmt.Errorf("解析任务参数失败: %w", err)
	}
	zap.L().Info("handleCreateInstance 参数解析完成",
		zap.String("instance_id", req.InstanceID),
		zap.String("type", req.Type),
		zap.String("template_id", req.TemplateID))

	e.wsClient.SendInstanceProgress(ws.InstanceProgressPayload{
		InstanceID: req.InstanceID,
		TaskID:     req.TaskID,
		Step:       1,
		Progress:   0,
		Message:    "开始创建实例",
		Status:     "running",
	})

	if !e.incusClient.ImageAliasExists(req.TemplateID) {
		zap.L().Error("镜像不存在，无法创建实例", zap.String("template_id", req.TemplateID))
		return nil, fmt.Errorf("镜像 %s 不存在，请先下载", req.TemplateID)
	}
	zap.L().Info("镜像存在，继续创建", zap.String("template_id", req.TemplateID))

	if req.BridgeName != "" && req.GatewayV4 != "" {
		zap.L().Info("确保 Bridge 存在", zap.String("bridge", req.BridgeName), zap.String("gateway", req.GatewayV4), zap.String("cidr", req.IPv4CIDR))
		if !e.incusClient.NetworkExists(req.BridgeName) {
			if err := e.incusClient.CreateBridgeNetwork(req.BridgeName, req.IPv4CIDR, "", "", req.GatewayV4); err != nil {
				zap.L().Warn("创建 bridge 失败", zap.String("bridge", req.BridgeName), zap.Error(err))
				return nil, fmt.Errorf("Bridge %s 创建失败: %w", req.BridgeName, err)
			}
			zap.L().Info("Bridge 创建成功", zap.String("bridge", req.BridgeName))
		} else {
			zap.L().Info("Bridge 已存在", zap.String("bridge", req.BridgeName))
		}
	}

	userData := buildCloudInitUserData(req.SSHPassword, req.SSHPublicKey, req.LoginMethod, req.InternalIPv4, req.GatewayV4, req.IPv4CIDR)
	zap.L().Info("cloud-init user-data 构建完成", zap.Int("len", len(userData)))

	networkConfig := ""

	var dataDisks []incus.DataDisk
	for _, dd := range req.DataDisks {
		disk := incus.DataDisk{
			Name:        getMapString(dd, "name"),
			SizeGB:      getMapInt(dd, "size_gb"),
			StoragePool: getMapString(dd, "storage_pool"),
			MountPoint:  getMapString(dd, "mount_point"),
		}
		dataDisks = append(dataDisks, disk)
	}
	zap.L().Info("数据盘配置完成", zap.Int("count", len(dataDisks)))

	createReq := incus.CreateInstanceRequest{
		Name:          req.InstanceID,
		TemplateID:    req.TemplateID,
		Type:          req.Type,
		VCPU:          req.VCPU,
		MemoryMB:      req.MemoryMB,
		DiskGB:        req.DiskGB,
		StoragePool:   req.StoragePool,
		IPv4Address:   req.IPv4Address,
		IPv6Address:   req.IPv6Address,
		UserData:      userData,
		NetworkConfig: networkConfig,
		DataDisks:     dataDisks,
		BridgeName:    req.BridgeName,
		InternalIPv4:  req.InternalIPv4,
		GatewayV4:     req.GatewayV4,
		IPv4CIDR:      req.IPv4CIDR,
		IPv4Filter:    req.IPv4Filter,
		MACFilter:     req.MACFilter,
		NetworkDown:   req.NetworkDown,
		NetworkUp:     req.NetworkUp,
	}

	name := req.InstanceID
	if e.incusClient.InstanceExists(name) {
		zap.L().Info("实例已存在，跳过 Incus 创建", zap.String("name", name))
	} else {
		zap.L().Info("调用 Incus CreateInstance",
			zap.String("name", req.InstanceID),
			zap.String("template_id", req.TemplateID),
			zap.String("type", req.Type))
		var err error
		name, err = e.incusClient.CreateInstance(createReq)
		if err != nil {
			zap.L().Error("Incus CreateInstance 失败", zap.String("name", req.InstanceID), zap.Error(err))
			e.wsClient.SendInstanceProgress(ws.InstanceProgressPayload{
				InstanceID: req.InstanceID,
				TaskID:     req.TaskID,
				Step:       0,
				Progress:   0,
				Message:    "创建实例失败",
				Error:      err.Error(),
				Status:     "error",
			})
			return nil, fmt.Errorf("创建实例失败: %w", err)
		}
		zap.L().Info("实例创建成功", zap.String("name", name))
	}

	e.wsClient.SendInstanceProgress(ws.InstanceProgressPayload{
		InstanceID: req.InstanceID,
		TaskID:     req.TaskID,
		Step:       2,
		Progress:   20,
		Message:    "服务器已接受请求",
		Status:     "running",
	})

	limits := map[string]string{}
	if req.VCPU > 0 {
		limits["limits.cpu"] = strconv.Itoa(int(req.VCPU))
	}
	if req.MemoryMB > 0 {
		limits["limits.memory"] = fmt.Sprintf("%dMB", req.MemoryMB)
	}
	if req.IORead > 0 {
		limits["limits.disk.read"] = fmt.Sprintf("%dMB", req.IORead)
	}
	if req.IOWrite > 0 {
		limits["limits.disk.write"] = fmt.Sprintf("%dMB", req.IOWrite)
	}
	if len(limits) > 0 {
		if err := e.incusClient.UpdateInstanceConfig(name, limits); err != nil {
			zap.L().Warn("设置资源限制失败", zap.Error(err))
		}
	}

	zap.L().Info("启动实例", zap.String("name", name))
	if err := e.incusClient.StartInstanceWithPool(name, req.StoragePool); err != nil {
		zap.L().Warn("启动实例失败", zap.Error(err))
	} else {
		waitForRunning(e.incusClient, name, 120)
	}

	e.wsClient.SendInstanceProgress(ws.InstanceProgressPayload{
		InstanceID: req.InstanceID,
		TaskID:     req.TaskID,
		Step:       3,
		Progress:   40,
		Message:    "配置网络",
		Status:     "running",
	})

	instanceIP := req.InternalIPv4
	if instanceIP == "" {
		instanceIP = req.IPv4Address
	}
	if instanceIP == "" {
		zap.L().Info("开始轮询获取实例内部 IP", zap.String("name", name))
		for i := 0; i < 60; i++ {
			if ipv4s, err := e.incusClient.GetInstanceNetworkInfo(name); err == nil && len(ipv4s) > 0 {
				instanceIP = ipv4s[0]
				zap.L().Info("获取实例内部 IP 成功", zap.String("ip", instanceIP), zap.Int("retry", i))
				break
			}
			time.Sleep(1 * time.Second)
		}
		if instanceIP == "" {
			zap.L().Error("60 秒内未能获取实例内部 IP，端口映射将不会创建", zap.String("name", name))
		}
	}

	isSSHPreinstalled := isSpiritlhlSource(req.ImageSource)
	if isSSHPreinstalled {
		zap.L().Info("镜像源预装SSH，跳过SSH安装", zap.String("name", name), zap.String("image_source", req.ImageSource))
	} else {
		e.wsClient.SendInstanceProgress(ws.InstanceProgressPayload{
			InstanceID: req.InstanceID,
			TaskID:     req.TaskID,
			Step:       4,
			Progress:   60,
			Message:    "安装并启动 SSH",
			Status:     "running",
		})
		zap.L().Info("exec 安装 SSH 开始", zap.String("name", name))
		if err := ensureSSHInContainer(name, req.LoginMethod, req.GatewayV4); err != nil {
			zap.L().Warn("exec 安装 SSH 失败", zap.String("name", name), zap.Error(err))
		} else {
			zap.L().Info("exec 安装 SSH 成功", zap.String("name", name))
		}
	}

	if req.SSHPassword != "" && (req.LoginMethod == "auto" || req.LoginMethod == "password") {
		if err := e.incusClient.SetInstancePassword(name, req.SSHPassword); err != nil {
			zap.L().Warn("exec 设置密码失败", zap.Error(err))
		}
	}

	var assignedPorts []map[string]interface{}

	if instanceIP == "" {
		zap.L().Warn("实例内部 IP 为空，跳过端口映射配置", zap.String("name", name))
	} else {
		addProxyMapping := func(hostPort, containerPort int, protocol, listenIP, desc string) error {
			if hostPort <= 0 || containerPort <= 0 {
				return nil
			}
			if protocol == "" {
				protocol = "tcp"
			}
			if listenIP == "" {
				return fmt.Errorf("端口映射缺少 host_ip, host_port=%d, container_port=%d", hostPort, containerPort)
			}
			deviceName := fmt.Sprintf("proxy-%d-%s", hostPort, protocol)
			listenAddr := fmt.Sprintf("%s:%s:%d", protocol, listenIP, hostPort)
			connectAddr := fmt.Sprintf("%s:%s:%d", protocol, instanceIP, containerPort)
			if err := e.incusClient.AddProxyDevice(name, deviceName, listenAddr, connectAddr); err != nil {
				return fmt.Errorf("添加 proxy 端口映射失败 device=%s: %w", deviceName, err)
			}
			entry := map[string]interface{}{
				"host_port":      hostPort,
				"container_port": containerPort,
				"protocol":       protocol,
			}
			if desc != "" {
				entry["description"] = desc
			}
			assignedPorts = append(assignedPorts, entry)
			zap.L().Info("proxy 端口映射添加成功",
				zap.String("device", deviceName),
				zap.Int("host_port", hostPort),
				zap.Int("container_port", containerPort))
			return nil
		}

		for _, pm := range req.PortMappings {
			hostPort := getMapInt(pm, "host_port")
			containerPort := getMapInt(pm, "container_port")
			protocol := getMapString(pm, "protocol")
			listenIP := getMapString(pm, "host_ip")
			desc := getMapString(pm, "description")
			if err := addProxyMapping(hostPort, containerPort, protocol, listenIP, desc); err != nil {
				return nil, fmt.Errorf("端口映射配置失败: %w", err)
			}
		}

		for _, nat := range req.NATs {
			if hostPort, ok := nat["host_port"].(float64); ok {
				if containerPort, ok := nat["container_port"].(float64); ok {
					protocol := "tcp"
					if p, ok := nat["protocol"].(string); ok && p != "" {
						protocol = p
					}
					desc := getMapString(nat, "description")
					listenIP := getMapString(nat, "host_ip")
					if err := addProxyMapping(int(hostPort), int(containerPort), protocol, listenIP, desc); err != nil {
						return nil, fmt.Errorf("端口映射配置失败: %w", err)
					}
				}
			}
		}

		if len(assignedPorts) == 0 {
			zap.L().Warn("Master 未下发端口映射，跳过", zap.String("name", name))
		}
	}

	e.wsClient.SendInstanceProgress(ws.InstanceProgressPayload{
		InstanceID: req.InstanceID,
		TaskID:     req.TaskID,
		Step:       5,
		Progress:   80,
		Message:    "配置端口映射",
		Status:     "running",
	})

	e.wsClient.SendInstanceProgress(ws.InstanceProgressPayload{
		InstanceID: req.InstanceID,
		TaskID:     req.TaskID,
		Step:       6,
		Progress:   100,
		Message:    "完成",
		Status:     "success",
	})

	return json.Marshal(map[string]interface{}{
		"name":           name,
		"status":         "running",
		"assigned_ports": assignedPorts,
		"instance_ip":    instanceIP,
	})
}
