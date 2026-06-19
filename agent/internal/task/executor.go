package task

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"tsukiyo/agent/internal/config"
	"tsukiyo/agent/internal/image"
	"tsukiyo/agent/internal/incus"
	"tsukiyo/agent/internal/network"
	"tsukiyo/agent/internal/ws"
)

// Executor 任务执行器
type Executor struct {
	cfg             *config.Config
	incusClient     *incus.Client
	netManager      *network.Manager
	wsClient        *ws.Client
	downloadManager *image.DownloadManager
}

// NewExecutor 创建任务执行器
func NewExecutor(cfg *config.Config, incusClient *incus.Client, netManager *network.Manager, wsClient *ws.Client) *Executor {
	return &Executor{
		cfg:             cfg,
		incusClient:     incusClient,
		netManager:      netManager,
		wsClient:        wsClient,
		downloadManager: image.NewDownloadManager("/var/cache/tsukiyo/images"),
	}
}

// Execute 执行任务
func (e *Executor) Execute(taskType string, payload json.RawMessage) (json.RawMessage, error) {
	zap.L().Info("开始执行任务", zap.String("type", taskType))

	switch taskType {
	case "create_instance":
		return e.handleCreateInstance(payload)
	case "delete_instance":
		return e.handleDeleteInstance(payload)
	case "start_instance":
		return e.handleStartInstance(payload)
	case "stop_instance":
		return e.handleStopInstance(payload)
	case "restart_instance":
		return e.handleRestartInstance(payload)
	case "reinstall_instance":
		return e.handleReinstallInstance(payload)
	case "resize_instance":
		return e.handleResizeInstance(payload)
	case "reset_password":
		return e.handleResetPassword(payload)
	case "create_snapshot":
		return e.handleCreateSnapshot(payload)
	case "restore_snapshot":
		return e.handleRestoreSnapshot(payload)
	case "delete_snapshot":
		return e.handleDeleteSnapshot(payload)
	case "download_image":
		return e.handleDownloadImage(payload)
	case "cancel_image_download":
		return e.handleCancelImageDownload(payload)
	case "check_image":
		return e.handleCheckImage(payload)
	case "delete_image":
		return e.handleDeleteImage(payload)
	case "list_remote_images":
		return e.handleListRemoteImages(payload)
	case "apply_network":
		return e.handleApplyNetwork(payload)
	case "apply_firewall":
		return e.handleApplyFirewall(payload)
	case "format_disk":
		return e.handleFormatDisk(payload)
	case "init_storage":
		return e.handleInitStorage(payload)
	case "migrate_instance":
		return e.handleMigrateInstance(payload)
	case "vpc_network":
		zap.L().Info("执行 VPC 网络任务", zap.String("type", taskType))
		return e.handleVPCNetwork(payload)
	default:
		return nil, fmt.Errorf("未知任务类型: %s", taskType)
	}
}

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
		IPv4Address      string                   `json:"ipv4_address"`
		IPv6Address      string                   `json:"ipv6_address"`
		NetworkDown      int                      `json:"network_down"`
		NetworkUp        int                      `json:"network_up"`
		IORead           int                      `json:"io_read"`
		IOWrite          int                      `json:"io_write"`
		DataDisks        []map[string]interface{} `json:"data_disks"`
		NATs             []map[string]interface{} `json:"nats"`
		MonthlyTraffic   int64                    `json:"monthly_traffic"`
		TrafficMode      string                   `json:"traffic_mode"`
		SnapshotLimit    int                      `json:"snapshot_limit"`
		AssignNAT        bool                     `json:"assign_nat"`
		PortMappingCount int                      `json:"port_mapping_count"`
		ExtraPorts       []int                    `json:"extra_ports"`
		VPCID            string                   `json:"vpc_id,omitempty"`
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

	// 上报进度：开始创建
	e.wsClient.SendInstanceProgress(ws.InstanceProgressPayload{
		InstanceID: req.InstanceID,
		TaskID:     req.TaskID,
		Step:       1,
		Progress:   0,
		Message:    "开始创建实例",
		Status:     "running",
	})

	// 检查镜像是否存在
	zap.L().Info("检查镜像是否存在", zap.String("template_id", req.TemplateID))
	if !e.incusClient.ImageAliasExists(req.TemplateID) {
		zap.L().Error("镜像不存在，无法创建实例", zap.String("template_id", req.TemplateID))
		return nil, fmt.Errorf("镜像 %s 不存在，请先下载", req.TemplateID)
	}
	zap.L().Info("镜像存在，继续创建", zap.String("template_id", req.TemplateID))

	// 如果指定了 VPC bridge，先确保 bridge 存在
	if req.BridgeName != "" && req.GatewayV4 != "" {
		zap.L().Info("确保 VPC bridge 存在", zap.String("bridge", req.BridgeName), zap.String("gateway", req.GatewayV4), zap.String("cidr", req.IPv4CIDR))
		if !e.incusClient.NetworkExists(req.BridgeName) {
			if err := e.incusClient.CreateBridgeNetwork(req.BridgeName, req.IPv4CIDR, "", "", req.GatewayV4); err != nil {
				zap.L().Warn("创建 bridge 失败", zap.String("bridge", req.BridgeName), zap.Error(err))
				return nil, fmt.Errorf("VPC bridge %s 创建失败: %w", req.BridgeName, err)
			}
			zap.L().Info("VPC bridge 创建成功", zap.String("bridge", req.BridgeName))
		} else {
			zap.L().Info("VPC bridge 已存在", zap.String("bridge", req.BridgeName))
		}
	}

	// 构建 cloud-init user-data
	userData := buildCloudInitUserData(req.SSHPassword, req.SSHPublicKey, req.LoginMethod, req.InternalIPv4, req.GatewayV4, req.IPv4CIDR)
	zap.L().Info("cloud-init user-data 构建完成", zap.Int("len", len(userData)))

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
		Name:         req.InstanceID,
		TemplateID:   req.TemplateID,
		Type:         req.Type,
		VCPU:         req.VCPU,
		MemoryMB:     req.MemoryMB,
		DiskGB:       req.DiskGB,
		StoragePool:  req.StoragePool,
		IPv4Address:  req.IPv4Address,
		IPv6Address:  req.IPv6Address,
		UserData:     userData,
		DataDisks:    dataDisks,
		BridgeName:   req.BridgeName,
		InternalIPv4: req.InternalIPv4,
		GatewayV4:    req.GatewayV4,
		IPv4CIDR:     req.IPv4CIDR,
		IPv4Filter:   req.IPv4Filter,
		MACFilter:    req.MACFilter,
	}

	zap.L().Info("调用 Incus CreateInstance",
		zap.String("name", req.InstanceID),
		zap.String("template_id", req.TemplateID),
		zap.String("type", req.Type))
	name, err := e.incusClient.CreateInstance(createReq)
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

	// 上报进度：服务器接受请求
	e.wsClient.SendInstanceProgress(ws.InstanceProgressPayload{
		InstanceID: req.InstanceID,
		TaskID:     req.TaskID,
		Step:       2,
		Progress:   20,
		Message:    "服务器已接受请求",
		Status:     "running",
	})

	// 设置资源限制（带宽、IO）
	limits := map[string]string{}
	if req.VCPU > 0 {
		limits["limits.cpu"] = strconv.Itoa(int(req.VCPU))
	}
	if req.MemoryMB > 0 {
		limits["limits.memory"] = fmt.Sprintf("%dMB", req.MemoryMB)
	}
	if req.NetworkDown > 0 {
		limits["limits.network.egress"] = fmt.Sprintf("%dMbit", req.NetworkDown)
	}
	if req.NetworkUp > 0 {
		limits["limits.network.ingress"] = fmt.Sprintf("%dMbit", req.NetworkUp)
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

	// 自动启动实例
	zap.L().Info("启动实例", zap.String("name", name))
	if err := e.incusClient.StartInstance(name); err != nil {
		zap.L().Warn("启动实例失败", zap.Error(err))
	} else {
		// 等待实例变为 Running 状态
		waitForRunning(e.incusClient, name, 120)
		// 等待 sshd 启动（从宿主机检测容器内网 IP 的 22 端口）
		zap.L().Info("等待 sshd 启动", zap.String("name", name), zap.String("ip", req.InternalIPv4))
		if req.InternalIPv4 != "" {
			for i := 0; i < 180; i++ {
				conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:22", req.InternalIPv4), 2*time.Second)
				if err == nil {
					conn.Close()
					zap.L().Info("sshd 已启动", zap.String("name", name), zap.String("ip", req.InternalIPv4), zap.Int("wait_seconds", i))
					break
				}
				time.Sleep(1 * time.Second)
			}
		}
	}

	// 上报进度：配置网络
	e.wsClient.SendInstanceProgress(ws.InstanceProgressPayload{
		InstanceID: req.InstanceID,
		TaskID:     req.TaskID,
		Step:       3,
		Progress:   40,
		Message:    "配置网络",
		Status:     "running",
	})

	// 上报进度：配置 SSH
	e.wsClient.SendInstanceProgress(ws.InstanceProgressPayload{
		InstanceID: req.InstanceID,
		TaskID:     req.TaskID,
		Step:       4,
		Progress:   60,
		Message:    "配置 SSH",
		Status:     "running",
	})

	// 获取实例内部 IP
	// VPC 模式优先使用 internal_ipv4（master 分配的静态 IP）
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

	// 如果 cloud-init 未生效，使用 exec 回退设置密码
	if req.SSHPassword != "" && (req.LoginMethod == "auto" || req.LoginMethod == "password") {
		if err := e.incusClient.SetInstancePassword(name, req.SSHPassword); err != nil {
			zap.L().Warn("exec 设置密码失败", zap.Error(err))
		}
	}

	// 配置端口映射：使用 Incus 原生 proxy 设备替代 iptables DNAT
	var assignedPorts []map[string]interface{}
	// 对外监听 IP：优先使用 VPC SNAT 出口 IP，否则用宿主机主 IP
	hostIP := req.EgressV4Primary
	if idx := strings.Index(hostIP, "/"); idx > 0 {
		hostIP = hostIP[:idx]
	}
	if hostIP == "" {
		hostIP = network.GetMainIP()
	}

	if instanceIP == "" {
		zap.L().Warn("实例内部 IP 为空，跳过端口映射配置", zap.String("name", name))
	} else {
		for _, nat := range req.NATs {
			if hostPort, ok := nat["host_port"].(float64); ok {
				if containerPort, ok := nat["container_port"].(float64); ok {
					protocol := "tcp"
					if p, ok := nat["protocol"].(string); ok && p != "" {
						protocol = p
					}
					deviceName := fmt.Sprintf("proxy-%d-%s", int(hostPort), protocol)
					listenAddr := fmt.Sprintf("%s:%s:%d", protocol, hostIP, int(hostPort))
					connectAddr := fmt.Sprintf("%s:%s:%d", protocol, instanceIP, int(containerPort))
					if err := e.incusClient.AddProxyDevice(name, deviceName, listenAddr, connectAddr); err != nil {
						zap.L().Warn("添加 proxy 端口映射失败", zap.Error(err), zap.String("device", deviceName))
					} else {
						assignedPorts = append(assignedPorts, map[string]interface{}{
							"host_port":      int(hostPort),
							"container_port": int(containerPort),
							"protocol":       protocol,
						})
						zap.L().Info("proxy 端口映射添加成功", zap.String("device", deviceName), zap.Int("host_port", int(hostPort)))
					}
				}
			}
		}

		// 自动分配端口映射（NAT 模式）：根据 port_mapping_count 配额分配
		if req.AssignNAT {
			// 自动分配 SSH 端口映射（容器内部 22 -> 宿主机随机端口）
			sshHostPort, err := e.netManager.AllocatePort(20000, 65535)
			if err != nil {
				zap.L().Warn("分配 SSH 端口失败", zap.Error(err))
			} else {
				deviceName := fmt.Sprintf("proxy-%d-tcp", sshHostPort)
				listenAddr := fmt.Sprintf("tcp:%s:%d", hostIP, sshHostPort)
				connectAddr := fmt.Sprintf("tcp:%s:22", instanceIP)
				if err := e.incusClient.AddProxyDevice(name, deviceName, listenAddr, connectAddr); err != nil {
					zap.L().Warn("添加 SSH proxy 端口映射失败", zap.Error(err))
				} else {
					assignedPorts = append(assignedPorts, map[string]interface{}{
						"host_port":      sshHostPort,
						"container_port": 22,
						"protocol":       "tcp",
						"description":    "SSH",
					})
					zap.L().Info("SSH proxy 端口映射已分配", zap.Int("host_port", sshHostPort))
				}
			}
		}
	}

	// 上报进度：配置端口映射
	e.wsClient.SendInstanceProgress(ws.InstanceProgressPayload{
		InstanceID: req.InstanceID,
		TaskID:     req.TaskID,
		Step:       5,
		Progress:   80,
		Message:    "配置端口映射",
		Status:     "running",
	})

	// 上报进度：完成
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

// buildCloudInitUserData 生成 cloud-init user-data YAML，预配置 root 密码、SSH 公钥和网络
func buildCloudInitUserData(password, publicKey, loginMethod, internalIPv4, gatewayV4, ipv4CIDR string) string {
	var lines []string
	lines = append(lines, "#cloud-config")
	lines = append(lines, "users:")
	lines = append(lines, "  - name: root")
	lines = append(lines, "    lock_passwd: false")
	if publicKey != "" {
		lines = append(lines, "    ssh_authorized_keys:")
		lines = append(lines, fmt.Sprintf("      - %s", publicKey))
	}
	if password != "" {
		lines = append(lines, "chpasswd:")
		lines = append(lines, "  list: |")
		lines = append(lines, fmt.Sprintf("    root:%s", password))
		lines = append(lines, "  expire: false")
	}
	lines = append(lines, "ssh_pwauth: true")
	lines = append(lines, "disable_root: false")

	// 注入文件（MOTD + SSH 配置）
	lines = append(lines, "write_files:")
	// MOTD
	lines = append(lines, "  - path: /etc/profile.d/tsukiyo-motd.sh")
	lines = append(lines, "    permissions: '0755'")
	lines = append(lines, "    content: |")
	lines = append(lines, "      #!/bin/sh")
	lines = append(lines, "      case \"$-\" in *i*) ;; *) return 0 ;; esac")
	lines = append(lines, "      [ -n \"$SSH_CONNECTION\" ] || return 0")
	lines = append(lines, "      [ -n \"$TSUKIYO_MOTD_SHOWN\" ] && return 0")
	lines = append(lines, "      export TSUKIYO_MOTD_SHOWN=1")
	lines = append(lines, "      echo")
	lines = append(lines, "      printf \"\\033[38;5;196m        ,----,                                                              \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;202m      ,/   .\\`|                                                              \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;208m    ,\\`   .'  :                              ,-.                             \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;214m  ;    ;     /                          ,--/ /|   ,--,                      \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;220m.'___,/    ,'                    ,--, ,--. :/ | ,--.'|              ,---.   \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;226m|    :     |  .--.--.          ,'_ /| :  : ' /  |  |,              '   ,'\\\\  \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;154m;    |.';  ; /  /    '    .--. |  | : |  '  /   \\`--'_        .--, /   /   | \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;118m\\`----'  |  ||  :  /\\`./  ,'_ /| :  . | '  |   \\\\  '  | |  , ' , ' :'   | |: | \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;82m    '   :  ;|  :  ;_    |  ' | |  . . |  |   \\\\  '  : | /___/ \\: |'   | .; : \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;46m    |   |  ' \\\\  \\\\    \\`. |  | : ;  ; | |  | ' \\\\ \\`  : |__.  \\\\  ' ||   :    | \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;47m    '   :  |  \\`----.   \\\\:  | : ;  ; | |  | |. \\\\ |  | '.'|\\\\  ;   : \\\\   \\\\  /  \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;48m    ;   |.'  /  /\\`--'  /'  :  \\`--'   \\`'  : |--' |  |    ; \\\\  \\\\  ;  \\`----'   \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;49m    '---'   '--'.     / :  ,      .-./;  |,'    ;  :    ; \\\\  \\\\  :  \\`----'    \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;50m              \\`--'---'   \\`--\\`----'    '--'      |  ,   /   :  \\\\  \\\\          \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;51m                                                 ---\\`-'     \\\\  ' ;          \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;87m                                                             \\`--\\`           \\033[0m\\n\"")
	lines = append(lines, "      echo")
	lines = append(lines, "      echo \"Tsukiyo Virtualization System By aDokiu\"")
	lines = append(lines, "      echo \"Github       : https://github.com/adokiu/Tsukiyo\"")
	lines = append(lines, "      echo")
	lines = append(lines, "      echo \"Distribution : $(cat /etc/os-release 2>/dev/null | grep PRETTY_NAME | cut -d= -f2 | tr -d '\"' || echo 'Linux')\"")
	lines = append(lines, "      echo \"Kernel       : $(uname -r)\"")
	lines = append(lines, "      echo")
	// runcmd：安装 ssh + 配置 sshd + 重启 sshd（同步执行，确保完成）
	lines = append(lines, "runcmd:")
	lines = append(lines, "  - |")
	lines = append(lines, "    if [ -f /etc/debian_version ]; then")
	lines = append(lines, "        apt-get update && apt-get install -y openssh-server && systemctl enable --now ssh")
	lines = append(lines, "    elif [ -f /etc/alpine-release ]; then")
	lines = append(lines, "        apk add openssh && rc-update add sshd default && service sshd start")
	lines = append(lines, "    elif [ -f /etc/fedora-release ] || [ -f /etc/centos-release ] || [ -f /etc/rocky-release ] || [ -f /etc/almalinux-release ] || [ -f /etc/oracle-release ]; then")
	lines = append(lines, "        dnf install -y openssh-server && systemctl enable --now sshd")
	lines = append(lines, "    elif [ -f /etc/arch-release ]; then")
	lines = append(lines, "        pacman -S --noconfirm openssh && systemctl enable --now sshd")
	lines = append(lines, "    elif [ -f /etc/SUSE-brand ] || [ -f /etc/SuSE-release ]; then")
	lines = append(lines, "        zypper install -y openssh && systemctl enable --now sshd")
	lines = append(lines, "    fi")
	// 配置 sshd（仅在密码登录模式下）
	if loginMethod == "password" || loginMethod == "auto" {
		lines = append(lines, "    sed -i '/^PasswordAuthentication/d' /etc/ssh/sshd_config")
		lines = append(lines, "    sed -i '/^PermitRootLogin/d' /etc/ssh/sshd_config")
		lines = append(lines, "    sed -i '/^UsePAM/d' /etc/ssh/sshd_config")
		lines = append(lines, "    echo 'PasswordAuthentication yes' >> /etc/ssh/sshd_config")
		lines = append(lines, "    echo 'PermitRootLogin yes' >> /etc/ssh/sshd_config")
		lines = append(lines, "    echo 'UsePAM yes' >> /etc/ssh/sshd_config")
		lines = append(lines, "    systemctl restart sshd || service sshd restart")
	}
	lines = append(lines, "")

	return strings.Join(lines, "\n")
}

func getMapString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getMapInt(m map[string]interface{}, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	if v, ok := m[key].(int); ok {
		return v
	}
	return 0
}

func (e *Executor) handleDeleteInstance(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析删除实例任务参数失败", zap.Error(err))
		return nil, err
	}
	zap.L().Info("开始删除实例", zap.String("instance_id", req.InstanceID))

	// 先获取实例信息用于清理网络配置
	zap.L().Info("获取实例信息", zap.String("instance_id", req.InstanceID))
	info, err := e.incusClient.GetInstance(req.InstanceID)
	if err != nil {
		// 只有"不存在"才是真正的幂等成功，其他错误都要返回失败
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "Not found") {
			zap.L().Info("实例不存在，幂等删除成功", zap.String("instance_id", req.InstanceID))
			return json.Marshal(map[string]interface{}{"deleted": true, "not_found": true})
		}
		zap.L().Error("获取实例信息失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
		return nil, fmt.Errorf("获取实例信息失败: %w", err)
	}

	// 清理网络配置
	zap.L().Info("清理网络配置", zap.String("instance_id", req.InstanceID))
	if info.Devices != nil {
		if devs, ok := info.Devices["eth0"].(map[string]interface{}); ok {
			if ip, ok := devs["ipv4.address"].(string); ok && ip != "" {
				zap.L().Info("解绑 IP", zap.String("ip", ip))
				e.netManager.UnbindIP(ip, "")
			}
		}
	}

	// 删除实例（force=1）
	zap.L().Info("删除实例", zap.String("instance_id", req.InstanceID))
	if err := e.incusClient.DeleteInstance(req.InstanceID); err != nil {
		// "不存在"也算成功
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
	var req struct {
		InstanceID string `json:"instance_id"`
		TemplateID string `json:"template_id"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析重装实例任务参数失败", zap.Error(err))
		return nil, err
	}
	zap.L().Info("开始重装实例", zap.String("instance_id", req.InstanceID), zap.String("template_id", req.TemplateID))
	if err := e.incusClient.ReinstallInstance(req.InstanceID, req.TemplateID); err != nil {
		zap.L().Error("重装实例失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
		return nil, err
	}
	zap.L().Info("重装实例成功", zap.String("instance_id", req.InstanceID))
	return json.Marshal(map[string]string{"status": "reinstalled", "instance_id": req.InstanceID})
}

func (e *Executor) handleResizeInstance(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string  `json:"instance_id"`
		VCPU       float64 `json:"vcpu"`
		MemoryMB   int     `json:"memory_mb"`
		DiskGB     int     `json:"disk_gb"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析调整配置任务参数失败", zap.Error(err))
		return nil, err
	}
	zap.L().Info("开始调整实例配置", zap.String("instance_id", req.InstanceID), zap.Float64("vcpu", req.VCPU), zap.Int("memory_mb", req.MemoryMB), zap.Int("disk_gb", req.DiskGB))

	config := map[string]string{}
	if req.VCPU > 0 {
		config["limits.cpu"] = strconv.Itoa(int(req.VCPU))
	}
	if req.MemoryMB > 0 {
		config["limits.memory"] = fmt.Sprintf("%dMB", req.MemoryMB)
	}

	// 磁盘 resize
	if req.DiskGB > 0 {
		zap.L().Info("调整磁盘大小", zap.String("instance_id", req.InstanceID), zap.Int("disk_gb", req.DiskGB))
		devices := map[string]map[string]string{
			"root": {
				"path": "/",
				"pool": "default",
				"type": "disk",
				"size": fmt.Sprintf("%dGB", req.DiskGB),
			},
		}
		if err := e.incusClient.SetInstanceConfig(req.InstanceID, config, devices); err != nil {
			zap.L().Error("调整磁盘大小失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
			return nil, err
		}
	} else {
		zap.L().Info("调整 CPU 和内存", zap.String("instance_id", req.InstanceID))
		if err := e.incusClient.UpdateInstanceConfig(req.InstanceID, config); err != nil {
			zap.L().Error("调整 CPU 和内存失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
			return nil, err
		}
	}

	zap.L().Info("调整实例配置成功", zap.String("instance_id", req.InstanceID))
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

func (e *Executor) handleCreateSnapshot(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
		Name       string `json:"name"`
		Stateful   bool   `json:"stateful"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析创建快照任务参数失败", zap.Error(err))
		return nil, err
	}
	zap.L().Info("开始创建快照", zap.String("instance_id", req.InstanceID), zap.String("name", req.Name), zap.Bool("stateful", req.Stateful))
	if err := e.incusClient.CreateSnapshot(req.InstanceID, req.Name, req.Stateful); err != nil {
		zap.L().Error("创建快照失败", zap.String("instance_id", req.InstanceID), zap.String("name", req.Name), zap.Error(err))
		return nil, err
	}
	zap.L().Info("创建快照成功", zap.String("instance_id", req.InstanceID), zap.String("name", req.Name))
	return json.Marshal(map[string]string{"snapshot": req.Name, "instance_id": req.InstanceID})
}

func (e *Executor) handleRestoreSnapshot(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
		Name       string `json:"name"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析恢复快照任务参数失败", zap.Error(err))
		return nil, err
	}
	zap.L().Info("开始恢复快照", zap.String("instance_id", req.InstanceID), zap.String("name", req.Name))
	if err := e.incusClient.RestoreSnapshot(req.InstanceID, req.Name); err != nil {
		zap.L().Error("恢复快照失败", zap.String("instance_id", req.InstanceID), zap.String("name", req.Name), zap.Error(err))
		return nil, err
	}
	zap.L().Info("恢复快照成功", zap.String("instance_id", req.InstanceID), zap.String("name", req.Name))
	return json.Marshal(map[string]string{"status": "restored", "instance_id": req.InstanceID, "name": req.Name})
}

func (e *Executor) handleDeleteSnapshot(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
		Name       string `json:"name"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析删除快照任务参数失败", zap.Error(err))
		return nil, err
	}
	zap.L().Info("开始删除快照", zap.String("instance_id", req.InstanceID), zap.String("name", req.Name))
	if err := e.incusClient.DeleteSnapshot(req.InstanceID, req.Name); err != nil {
		zap.L().Error("删除快照失败", zap.String("instance_id", req.InstanceID), zap.String("name", req.Name), zap.Error(err))
		return nil, err
	}
	zap.L().Info("删除快照成功", zap.String("instance_id", req.InstanceID), zap.String("name", req.Name))
	return json.Marshal(map[string]interface{}{"deleted": true, "instance_id": req.InstanceID, "name": req.Name})
}

func (e *Executor) handleApplyNetwork(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		Action        string `json:"action"`
		InstanceID    string `json:"instance_id"`
		IPAddress     string `json:"ip_address"`
		HostPort      int    `json:"host_port"`
		ContainerPort int    `json:"container_port"`
		Protocol      string `json:"protocol"`
		HostIP        string `json:"host_ip"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("解析网络应用任务参数失败", zap.Error(err))
		return nil, err
	}

	zap.L().Info("开始应用网络配置",
		zap.String("action", req.Action),
		zap.String("instance_id", req.InstanceID),
		zap.Int("host_port", req.HostPort),
		zap.Int("container_port", req.ContainerPort),
		zap.String("protocol", req.Protocol),
		zap.String("host_ip", req.HostIP))

	// 获取实例内部 IP（用于 proxy connect 地址）
	instanceIP := ""
	if info, err := e.incusClient.GetInstanceNetworkInfo(req.InstanceID); err == nil && len(info) > 0 {
		instanceIP = info[0]
	} else {
		zap.L().Warn("获取实例内部 IP 失败，proxy connect 地址可能为空", zap.Error(err))
	}

	// 宿主机 listen IP
	hostIP := req.HostIP
	if hostIP == "" {
		hostIP = network.GetMainIP()
	}

	deviceName := fmt.Sprintf("proxy-%d-%s", req.HostPort, req.Protocol)

	switch req.Action {
	case "add_ip":
		if err := e.netManager.BindIP(req.IPAddress, e.cfg.NetworkInterface()); err != nil {
			zap.L().Error("绑定 IP 失败", zap.Error(err))
			return nil, err
		}
		zap.L().Info("绑定 IP 成功", zap.String("ip", req.IPAddress))
	case "remove_ip":
		if err := e.netManager.UnbindIP(req.IPAddress, e.cfg.NetworkInterface()); err != nil {
			zap.L().Error("解绑 IP 失败", zap.Error(err))
			return nil, err
		}
		zap.L().Info("解绑 IP 成功", zap.String("ip", req.IPAddress))
	case "add_port":
		if instanceIP == "" {
			zap.L().Error("添加端口映射失败：实例内部 IP 为空")
			return nil, fmt.Errorf("实例内部 IP 为空，无法创建 proxy 端口映射")
		}
		// listen 地址使用 payload 中的 host_ip（VPC SNAT 出口 IP），去掉 CIDR 后缀
		listenIP := req.HostIP
		if idx := strings.Index(listenIP, "/"); idx > 0 {
			listenIP = listenIP[:idx]
		}
		if listenIP == "" {
			listenIP = "0.0.0.0"
		}
		listenAddr := fmt.Sprintf("%s:%s:%d", req.Protocol, listenIP, req.HostPort)
		connectAddr := fmt.Sprintf("%s:%s:%d", req.Protocol, instanceIP, req.ContainerPort)
		if err := e.incusClient.AddProxyDevice(req.InstanceID, deviceName, listenAddr, connectAddr); err != nil {
			zap.L().Error("添加 proxy 端口映射失败",
				zap.String("device", deviceName),
				zap.String("listen", listenAddr),
				zap.String("connect", connectAddr),
				zap.Error(err))
			return nil, fmt.Errorf("添加 proxy 端口映射失败: %w", err)
		}
		zap.L().Info("添加 proxy 端口映射成功",
			zap.String("device", deviceName),
			zap.String("listen", listenAddr),
			zap.String("connect", connectAddr))
	case "del_port":
		if err := e.incusClient.RemoveProxyDevice(req.InstanceID, deviceName); err != nil {
			zap.L().Error("删除 proxy 端口映射失败",
				zap.String("device", deviceName),
				zap.Error(err))
			return nil, fmt.Errorf("删除 proxy 端口映射失败: %w", err)
		}
		zap.L().Info("删除 proxy 端口映射成功", zap.String("device", deviceName))
	case "add_nat":
		// 已废弃，使用 add_port
		zap.L().Warn("add_nat 已废弃，请使用 add_port")
	case "remove_nat":
		// 已废弃
		zap.L().Warn("remove_nat 已废弃，请使用 del_port")
	default:
		zap.L().Warn("未知的网络操作类型", zap.String("action", req.Action))
		return nil, fmt.Errorf("未知的网络操作类型: %s", req.Action)
	}

	return json.Marshal(map[string]string{"status": "applied"})
}

func (e *Executor) handleApplyFirewall(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		Rules []struct {
			Direction  string `json:"direction"`
			Protocol   string `json:"protocol"`
			Source     string `json:"source"`
			Port       int    `json:"port"`
			Action     string `json:"action"`
			InstanceID string `json:"instance_id"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}

	for _, rule := range req.Rules {
		switch rule.Action {
		case "allow":
			e.netManager.AddFirewallRule(rule.Direction, rule.Protocol, rule.Source, rule.Port, rule.Action)
		case "deny":
			e.netManager.AddFirewallRule(rule.Direction, rule.Protocol, rule.Source, rule.Port, rule.Action)
		case "remove":
			e.netManager.RemoveFirewallRule(rule.Direction, rule.Protocol, rule.Source, rule.Port)
		}
	}

	return json.Marshal(map[string]string{"status": "applied"})
}

func (e *Executor) handleFormatDisk(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		Device string `json:"device"`
		FS     string `json:"fs"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"status": "formatted"})
}

func (e *Executor) handleInitStorage(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		Driver      string `json:"driver"`
		Source      string `json:"source"`
		StoragePool string `json:"storage_pool"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"status": "initialized"})
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

// handleVPCNetwork 处理 VPC 网络配置任务（创建/更新/删除 Incus bridge）
func (e *Executor) handleVPCNetwork(payload json.RawMessage) (json.RawMessage, error) {
	zap.L().Info("[VPC] handleVPCNetwork 被调用", zap.Int("payload_len", len(payload)))

	var req struct {
		VPCID            string `json:"vpc_id"`
		Action           string `json:"action"`
		BridgeName       string `json:"bridge_name"`
		IPv4CIDR         string `json:"ipv4_cidr"`
		IPv6ULACIDR      string `json:"ipv6_ula_cidr"`
		IPv6GUACIDR      string `json:"ipv6_gua_cidr"`
		DefaultGatewayV4 string `json:"default_gateway_v4"`
		DefaultGatewayV6 string `json:"default_gateway_v6"`
		EgressV4Primary  string `json:"egress_v4_primary"`
		ParentIface      string `json:"parent_iface"`
		SNATEnabled      bool   `json:"snat_enabled"`
		IPv4Filter       bool   `json:"ipv4_filter"`
		MACFilter        bool   `json:"mac_filter"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		zap.L().Error("[VPC] 解析 payload 失败", zap.Error(err))
		return nil, fmt.Errorf("解析 VPC 任务参数失败: %w", err)
	}

	zap.L().Info("[VPC] 任务参数解析成功",
		zap.String("action", req.Action),
		zap.String("vpc_id", req.VPCID),
		zap.String("bridge_name", req.BridgeName),
		zap.String("ipv4_cidr", req.IPv4CIDR),
		zap.String("gateway_v4", req.DefaultGatewayV4),
		zap.String("egress", req.EgressV4Primary),
		zap.String("parent_iface", req.ParentIface))

	switch req.Action {
	case "create":
		if err := e.incusClient.CreateBridgeNetwork(req.BridgeName, req.IPv4CIDR, req.IPv6ULACIDR, req.IPv6GUACIDR, req.DefaultGatewayV4); err != nil {
			zap.L().Error("创建 bridge 网络失败", zap.String("bridge", req.BridgeName), zap.Error(err))
			return nil, fmt.Errorf("创建 bridge 网络失败: %w", err)
		}
		zap.L().Info("bridge 网络创建成功", zap.String("bridge", req.BridgeName))

	case "update":
		// 更新：Incus 原生 NAT 已启用，无需额外配置
		zap.L().Info("VPC 更新完成", zap.String("bridge", req.BridgeName))

	case "delete":
		if err := e.incusClient.DeleteBridgeNetwork(req.BridgeName); err != nil {
			zap.L().Error("删除 bridge 网络失败", zap.String("bridge", req.BridgeName), zap.Error(err))
			return nil, fmt.Errorf("删除 bridge 网络失败: %w", err)
		}
		zap.L().Info("bridge 网络删除成功", zap.String("bridge", req.BridgeName))

	default:
		return nil, fmt.Errorf("未知的 VPC 动作: %s", req.Action)
	}

	return json.Marshal(map[string]string{"status": "ok", "action": req.Action})
}

// waitForRunning 等待实例变为 running 状态
func waitForRunning(client *incus.Client, name string, timeoutSec int) {
	for i := 0; i < timeoutSec; i++ {
		if info, err := client.GetInstance(name); err == nil && info.Status == "Running" {
			return
		}
	}
}

func parseInt(v interface{}) int {
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case string:
		i, _ := strconv.Atoi(val)
		return i
	}
	return 0
}
