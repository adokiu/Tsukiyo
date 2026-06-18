package task

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"tsukiyo/agent/internal/config"
	"tsukiyo/agent/internal/image"
	"tsukiyo/agent/internal/incus"
	"tsukiyo/agent/internal/network"
	"tsukiyo/agent/internal/reconcile"
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
	userData := buildCloudInitUserData(req.SSHPassword, req.SSHPublicKey, req.LoginMethod)
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
		return nil, fmt.Errorf("创建实例失败: %w", err)
	}
	zap.L().Info("实例创建成功", zap.String("name", name))

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
	}

	// 安装并启动 openssh-server（Alpine 等镜像默认不装 ssh）
	zap.L().Info("检查并安装 SSH 服务", zap.String("name", name))
	if err := e.incusClient.EnsureSSHInstalled(name); err != nil {
		zap.L().Warn("安装 SSH 服务失败", zap.String("name", name), zap.Error(err))
	} else {
		zap.L().Info("SSH 服务已就绪", zap.String("name", name))
	}

	// 获取实例内部 IP（轮询等待，最多 60 秒）
	instanceIP := req.IPv4Address
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

	return json.Marshal(map[string]interface{}{
		"name":           name,
		"status":         "running",
		"assigned_ports": assignedPorts,
		"instance_ip":    instanceIP,
	})
}

// buildCloudInitUserData 生成 cloud-init user-data YAML，预配置 root 密码和 SSH 公钥
// 覆盖主流发行版：Debian/Ubuntu（cloud-init）、RHEL/CentOS（cloud-init）、Alpine（tiny-cloud/alpine-conf）
func buildCloudInitUserData(password, publicKey, loginMethod string) string {
	if password == "" && publicKey == "" {
		return ""
	}
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

	// 确保 SSH 服务运行并允许 root 密码登录
	lines = append(lines, "runcmd:")
	lines = append(lines, "  - |")
	lines = append(lines, "    # 确保 SSH 服务安装并运行")
	lines = append(lines, "    if command -v apk >/dev/null 2>&1; then")
	lines = append(lines, "      apk add --no-cache openssh-server 2>/dev/null || true")
	lines = append(lines, "      rc-update add sshd boot 2>/dev/null || true")
	lines = append(lines, "      rc-service sshd start 2>/dev/null || true")
	lines = append(lines, "    elif command -v apt-get >/dev/null 2>&1; then")
	lines = append(lines, "      apt-get update -qq && apt-get install -y -qq openssh-server 2>/dev/null || true")
	lines = append(lines, "      systemctl enable sshd 2>/dev/null || systemctl enable ssh 2>/dev/null || true")
	lines = append(lines, "      systemctl start sshd 2>/dev/null || systemctl start ssh 2>/dev/null || true")
	lines = append(lines, "    elif command -v yum >/dev/null 2>&1; then")
	lines = append(lines, "      yum install -y openssh-server 2>/dev/null || true")
	lines = append(lines, "      systemctl enable sshd 2>/dev/null || true")
	lines = append(lines, "      systemctl start sshd 2>/dev/null || true")
	lines = append(lines, "    fi")
	lines = append(lines, "  - |")
	lines = append(lines, "    # 允许 root 密码登录")
	lines = append(lines, "    if [ -f /etc/ssh/sshd_config ]; then")
	lines = append(lines, "      sed -i 's/^#*PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config")
	lines = append(lines, "      sed -i 's/^#*PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config")
	lines = append(lines, "      sed -i 's/^#*ChallengeResponseAuthentication.*/ChallengeResponseAuthentication yes/' /etc/ssh/sshd_config")
	lines = append(lines, "      if command -v sshd >/dev/null 2>&1; then")
	lines = append(lines, "        sshd -t 2>/dev/null && killall -HUP sshd 2>/dev/null || true")
	lines = append(lines, "      fi")
	lines = append(lines, "    fi")

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
		return nil, err
	}
	zap.L().Info("handleDeleteInstance 开始", zap.String("instance_id", req.InstanceID))

	// 先获取实例信息用于清理网络配置
	info, err := e.incusClient.GetInstance(req.InstanceID)
	if err != nil {
		// 只有"不存在"才是真正的幂等成功，其他错误都要返回失败
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "Not found") {
			zap.L().Info("实例不存在，幂等删除成功", zap.String("instance_id", req.InstanceID))
			return json.Marshal(map[string]interface{}{"deleted": true, "not_found": true})
		}
		return nil, fmt.Errorf("获取实例信息失败: %w", err)
	}

	// 清理网络配置
	if info.Devices != nil {
		if devs, ok := info.Devices["eth0"].(map[string]interface{}); ok {
			if ip, ok := devs["ipv4.address"].(string); ok && ip != "" {
				e.netManager.UnbindIP(ip, "")
			}
		}
	}

	// 删除实例（force=1）
	if err := e.incusClient.DeleteInstance(req.InstanceID); err != nil {
		// "不存在"也算成功
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "Not found") {
			zap.L().Info("删除时实例已不存在，幂等成功", zap.String("instance_id", req.InstanceID))
			return json.Marshal(map[string]interface{}{"deleted": true, "not_found": true})
		}
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
		return nil, err
	}
	zap.L().Info("handleStartInstance 开始", zap.String("instance_id", req.InstanceID))
	if err := e.incusClient.StartInstance(req.InstanceID); err != nil {
		zap.L().Error("启动实例失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
		return nil, err
	}
	zap.L().Info("启动实例成功", zap.String("instance_id", req.InstanceID))
	return json.Marshal(map[string]string{"status": "running"})
}

func (e *Executor) handleStopInstance(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
		Force      bool   `json:"force"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	zap.L().Info("handleStopInstance 开始", zap.String("instance_id", req.InstanceID), zap.Bool("force", req.Force))
	if err := e.incusClient.StopInstance(req.InstanceID, req.Force); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"status": "stopped"})
}

func (e *Executor) handleRestartInstance(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	zap.L().Info("handleRestartInstance 开始", zap.String("instance_id", req.InstanceID))
	if err := e.incusClient.RestartInstance(req.InstanceID); err != nil {
		zap.L().Error("重启实例失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
		return nil, err
	}
	zap.L().Info("重启实例成功", zap.String("instance_id", req.InstanceID))
	return json.Marshal(map[string]string{"status": "running"})
}

func (e *Executor) handleReinstallInstance(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
		TemplateID string `json:"template_id"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	zap.L().Info("handleReinstallInstance 开始", zap.String("instance_id", req.InstanceID), zap.String("template_id", req.TemplateID))
	if err := e.incusClient.ReinstallInstance(req.InstanceID, req.TemplateID); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"status": "reinstalled"})
}

func (e *Executor) handleResizeInstance(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string  `json:"instance_id"`
		VCPU       float64 `json:"vcpu"`
		MemoryMB   int     `json:"memory_mb"`
		DiskGB     int     `json:"disk_gb"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	zap.L().Info("handleResizeInstance 开始", zap.String("instance_id", req.InstanceID), zap.Float64("vcpu", req.VCPU), zap.Int("memory_mb", req.MemoryMB), zap.Int("disk_gb", req.DiskGB))

	config := map[string]string{}
	if req.VCPU > 0 {
		config["limits.cpu"] = strconv.Itoa(int(req.VCPU))
	}
	if req.MemoryMB > 0 {
		config["limits.memory"] = fmt.Sprintf("%dMB", req.MemoryMB)
	}

	// 磁盘 resize
	if req.DiskGB > 0 {
		devices := map[string]map[string]string{
			"root": {
				"path": "/",
				"pool": "default",
				"type": "disk",
				"size": fmt.Sprintf("%dGB", req.DiskGB),
			},
		}
		if err := e.incusClient.SetInstanceConfig(req.InstanceID, config, devices); err != nil {
			return nil, err
		}
	} else {
		if err := e.incusClient.UpdateInstanceConfig(req.InstanceID, config); err != nil {
			return nil, err
		}
	}

	return json.Marshal(map[string]string{"status": "resized"})
}

func (e *Executor) handleResetPassword(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
		Password   string `json:"password"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	zap.L().Info("handleResetPassword 开始", zap.String("instance_id", req.InstanceID))
	if err := e.incusClient.SetInstancePassword(req.InstanceID, req.Password); err != nil {
		zap.L().Error("重置密码失败", zap.String("instance_id", req.InstanceID), zap.Error(err))
		return nil, err
	}
	zap.L().Info("重置密码成功", zap.String("instance_id", req.InstanceID))
	return json.Marshal(map[string]bool{"success": true})
}

func (e *Executor) handleCreateSnapshot(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
		Name       string `json:"name"`
		Stateful   bool   `json:"stateful"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	if err := e.incusClient.CreateSnapshot(req.InstanceID, req.Name, req.Stateful); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"snapshot": req.Name})
}

func (e *Executor) handleRestoreSnapshot(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
		Name       string `json:"name"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	zap.L().Info("handleRestoreSnapshot 开始", zap.String("instance_id", req.InstanceID), zap.String("name", req.Name))
	if err := e.incusClient.RestoreSnapshot(req.InstanceID, req.Name); err != nil {
		zap.L().Error("恢复快照失败", zap.String("instance_id", req.InstanceID), zap.String("name", req.Name), zap.Error(err))
		return nil, err
	}
	zap.L().Info("恢复快照成功", zap.String("instance_id", req.InstanceID), zap.String("name", req.Name))
	return json.Marshal(map[string]string{"status": "restored"})
}

func (e *Executor) handleDeleteSnapshot(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		InstanceID string `json:"instance_id"`
		Name       string `json:"name"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	zap.L().Info("handleDeleteSnapshot 开始", zap.String("instance_id", req.InstanceID), zap.String("name", req.Name))
	if err := e.incusClient.DeleteSnapshot(req.InstanceID, req.Name); err != nil {
		zap.L().Error("删除快照失败", zap.String("instance_id", req.InstanceID), zap.String("name", req.Name), zap.Error(err))
		return nil, err
	}
	zap.L().Info("删除快照成功", zap.String("instance_id", req.InstanceID), zap.String("name", req.Name))
	return json.Marshal(map[string]bool{"deleted": true})
}

func (e *Executor) handleDownloadImage(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ImageID   string `json:"image_id"`
		ImageType string `json:"image_type"` // container / vm
		Source    string `json:"source"`     // container: "images:ubuntu/24.04/cloud", vm: URL
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("解析下载参数失败: %w", err)
	}

	zap.L().Info("开始下载镜像",
		zap.String("image_id", req.ImageID),
		zap.String("type", req.ImageType),
		zap.String("source", req.Source))

	switch req.ImageType {
	case "container":
		return e.downloadContainerImage(req.ImageID, req.Source)
	case "vm":
		return e.downloadVMImage(req.ImageID, req.Source)
	default:
		return nil, fmt.Errorf("未知镜像类型: %s", req.ImageType)
	}
}

// downloadContainerImage 使用 incus image copy 下载容器镜像，解析 stderr 实时进度
func (e *Executor) downloadContainerImage(imageID, source string) (json.RawMessage, error) {
	// 已存在则跳过
	if e.incusClient.ImageAliasExists(imageID) {
		e.wsClient.SendImageProgress(ws.ImageProgressPayload{
			ImageID:  imageID,
			Stage:    "done",
			Progress: 100,
		})
		return json.Marshal(map[string]string{"status": "already_exists"})
	}

	e.wsClient.SendImageProgress(ws.ImageProgressPayload{
		ImageID:  imageID,
		Stage:    "downloading",
		Progress: 0,
	})

	args := []string{"image", "copy", source, "local:", "--alias", imageID, "--auto-update"}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "incus", args...)
	zap.L().Info("执行 incus image copy", zap.String("cmd", strings.Join(cmd.Args, " ")))

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("创建 stderr pipe 失败: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 incus image copy 失败: %w", err)
	}

	// incus 用 \r 覆盖同一行输出进度，bufio.Scanner 按 \n 切分会阻塞到命令结束。
	// 改用逐字节读取，按 \r 或 \n 切分，实现实时进度捕获。
	var lastSpeed int64
	go func() {
		buf := make([]byte, 4096)
		var lineBuf []byte
		for {
			n, readErr := stderr.Read(buf)
			if n > 0 {
				for i := 0; i < n; i++ {
					ch := buf[i]
					if ch == '\r' || ch == '\n' {
						if len(lineBuf) > 0 {
							line := string(lineBuf)
							lineBuf = lineBuf[:0]
							pct, speed := parseIncusImageCopyProgress(line)
							if pct >= 0 {
								lastSpeed = speed
								e.wsClient.SendImageProgress(ws.ImageProgressPayload{
									ImageID:  imageID,
									Stage:    "downloading",
									Progress: pct,
									SpeedBps: speed,
								})
							}
						}
					} else {
						lineBuf = append(lineBuf, ch)
					}
				}
			}
			if readErr != nil {
				if len(lineBuf) > 0 {
					line := string(lineBuf)
					pct, speed := parseIncusImageCopyProgress(line)
					if pct >= 0 {
						lastSpeed = speed
					}
				}
				return
			}
		}
	}()

	waitErr := cmd.Wait()
	if waitErr != nil {
		e.wsClient.SendImageProgress(ws.ImageProgressPayload{
			ImageID: imageID,
			Stage:   "error",
			Error:   waitErr.Error(),
		})
		return nil, fmt.Errorf("incus image copy 失败: %w", waitErr)
	}

	e.wsClient.SendImageProgress(ws.ImageProgressPayload{
		ImageID:  imageID,
		Stage:    "done",
		Progress: 100,
		SpeedBps: lastSpeed,
	})
	// 下载完成后立即上报本地镜像列表，确保 Master 状态同步
	go func() {
		aliases, err := e.incusClient.ListImages()
		if err == nil {
			e.wsClient.SendLocalImages(aliases)
		}
	}()
	return json.Marshal(map[string]string{"status": "completed"})
}

// parseIncusImageCopyProgress 解析 incus image copy 的 stderr 进度输出
// 格式示例: "Copying the image: 100% (45.23MB/s)"
func parseIncusImageCopyProgress(line string) (percent int, speedBps int64) {
	// 匹配百分比
	pctRe := regexp.MustCompile(`(\d+)%`)
	pctMatch := pctRe.FindStringSubmatch(line)
	if len(pctMatch) >= 2 {
		pct, _ := strconv.Atoi(pctMatch[1])
		percent = pct
	} else {
		percent = -1 // 未匹配到百分比
	}

	// 匹配速度，如 45.23MB/s、1.2GB/s、500KB/s
	speedRe := regexp.MustCompile(`\(([\d.]+)\s*(B|KB|MB|GB|TB)/s\)`)
	speedMatch := speedRe.FindStringSubmatch(line)
	if len(speedMatch) >= 3 {
		val, _ := strconv.ParseFloat(speedMatch[1], 64)
		unit := speedMatch[2]
		switch unit {
		case "B":
			speedBps = int64(val)
		case "KB":
			speedBps = int64(val * 1024)
		case "MB":
			speedBps = int64(val * 1024 * 1024)
		case "GB":
			speedBps = int64(val * 1024 * 1024 * 1024)
		case "TB":
			speedBps = int64(val * 1024 * 1024 * 1024 * 1024)
		}
	}
	return
}

// downloadVMImage 下载 VM 镜像：固定 .qcow2 路径，HTTP 下载，qemu-img convert 转换，再导入 Incus
func (e *Executor) downloadVMImage(imageID, url string) (json.RawMessage, error) {
	if url == "" {
		return nil, fmt.Errorf("VM 镜像 %s 无下载地址（需手动上传）", imageID)
	}

	cacheDir := "/var/cache/tsukiyo/images"
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("创建缓存目录失败: %w", err)
	}

	target := filepath.Join(cacheDir, imageID+".qcow2")
	tmp := target + ".tmp"

	// 检查是否已存在
	if _, err := os.Stat(target); err == nil {
		e.wsClient.SendImageProgress(ws.ImageProgressPayload{
			ImageID:  imageID,
			Stage:    "done",
			Progress: 100,
		})
		return json.Marshal(map[string]string{"status": "already_exists"})
	}

	// 立即发送 downloading 初始状态
	e.wsClient.SendImageProgress(ws.ImageProgressPayload{
		ImageID:  imageID,
		Stage:    "downloading",
		Progress: 0,
	})

	// 直接 HTTP 下载（照抄 CLICD）
	_ = os.Remove(tmp)
	if err := downloadFileWithProgress(url, tmp, imageID, e.wsClient); err != nil {
		_ = os.Remove(tmp)
		e.wsClient.SendImageProgress(ws.ImageProgressPayload{
			ImageID: imageID,
			Stage:   "error",
			Error:   "下载失败: " + err.Error(),
		})
		return nil, fmt.Errorf("下载 VM 镜像失败: %w", err)
	}

	// 转换为 qcow2（照抄 CLICD normalizeQCOW2）
	e.wsClient.SendImageProgress(ws.ImageProgressPayload{
		ImageID:  imageID,
		Stage:    "converting",
		Progress: 100,
	})
	if err := normalizeQCOW2(tmp, target); err != nil {
		_ = os.Remove(tmp)
		_ = os.Remove(target)
		e.wsClient.SendImageProgress(ws.ImageProgressPayload{
			ImageID: imageID,
			Stage:   "error",
			Error:   "转换失败: " + err.Error(),
		})
		return nil, fmt.Errorf("转换 qcow2 失败: %w", err)
	}
	_ = os.Remove(tmp)

	// 导入 Incus
	e.wsClient.SendImageProgress(ws.ImageProgressPayload{
		ImageID:  imageID,
		Stage:    "importing",
		Progress: 100,
	})
	if err := e.incusClient.ImportImageFromFile(imageID, target); err != nil {
		_ = os.Remove(target)
		e.wsClient.SendImageProgress(ws.ImageProgressPayload{
			ImageID: imageID,
			Stage:   "error",
			Error:   "导入失败: " + err.Error(),
		})
		return nil, fmt.Errorf("导入 Incus 失败: %w", err)
	}
	zap.L().Info("VM 镜像导入成功", zap.String("image_id", imageID))

	e.wsClient.SendImageProgress(ws.ImageProgressPayload{
		ImageID:  imageID,
		Stage:    "done",
		Progress: 100,
	})

	// 导入完成后立即上报本地镜像列表
	go func() {
		aliases, err := e.incusClient.ListImages()
		if err == nil {
			e.wsClient.SendLocalImages(aliases)
		}
	}()

	return json.Marshal(map[string]string{"status": "completed"})
}

// downloadFileWithProgress 直接 HTTP 下载文件并推送进度（照抄 CLICD）
func downloadFileWithProgress(url, target, imageID string, wsClient *ws.Client) error {
	client := http.Client{
		Timeout: 30 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			if ua := via[0].Header.Get("User-Agent"); ua != "" {
				req.Header.Set("User-Agent", ua)
			}
			return nil
		},
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()

	total := resp.ContentLength
	if total < 0 {
		total = 0
	}

	buf := make([]byte, 256*1024)
	var downloaded int64
	var lastReport time.Time
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			written, writeErr := out.Write(buf[:n])
			downloaded += int64(written)
			if writeErr != nil {
				return writeErr
			}
			if written != n {
				return io.ErrShortWrite
			}
			now := time.Now()
			if lastReport.IsZero() || now.Sub(lastReport) >= 1*time.Second {
				lastReport = now
				pct := 0
				if total > 0 {
					pct = int(downloaded * 100 / total)
					if pct > 99 {
						pct = 99
					}
				}
				wsClient.SendImageProgress(ws.ImageProgressPayload{
					ImageID:         imageID,
					Stage:           "downloading",
					Progress:        pct,
					DownloadedBytes: downloaded,
					TotalBytes:      total,
				})
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	return out.Sync()
}

// normalizeQCOW2 使用 qemu-img convert 转换镜像为标准 qcow2（照抄 CLICD）
func normalizeQCOW2(src, target string) error {
	cmd := exec.Command("qemu-img", "convert", "-O", "qcow2", src, target)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("qemu-img convert 失败: %v, output: %s", err, string(output))
	}
	return nil
}

// handleCancelImageDownload 取消镜像下载
func (e *Executor) handleCancelImageDownload(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ImageID string `json:"image_id"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}

	if task := e.downloadManager.GetTask(req.ImageID); task != nil {
		task.Cancel()
		e.downloadManager.RemoveTask(req.ImageID)
	}

	e.wsClient.SendImageProgress(ws.ImageProgressPayload{
		ImageID: req.ImageID,
		Stage:   "canceled",
	})

	return json.Marshal(map[string]string{"status": "canceled"})
}

// handleCheckImage 检查镜像是否已下载
func (e *Executor) handleCheckImage(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ImageID   string `json:"image_id"`
		ImageType string `json:"image_type"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}

	exists := false
	switch req.ImageType {
	case "container":
		exists = e.incusClient.ImageAliasExists(req.ImageID)
	case "vm":
		// 优先检查 Incus 中是否存在该别名（下载完成后已导入）
		if e.incusClient.ImageAliasExists(req.ImageID) {
			exists = true
		} else {
			// 备选：检查下载任务是否完成（导入前状态）
			task := e.downloadManager.GetTask(req.ImageID)
			if task != nil && task.GetStatus() == image.StatusCompleted {
				exists = true
			}
		}
	}

	return json.Marshal(map[string]interface{}{
		"image_id":   req.ImageID,
		"downloaded": exists,
	})
}

// handleDeleteImage 删除已下载的镜像
func (e *Executor) handleDeleteImage(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ImageID   string `json:"image_id"`
		ImageType string `json:"image_type"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}

	switch req.ImageType {
	case "container":
		// 通过 incus image delete 删除
		cmd := exec.Command("incus", "image", "delete", req.ImageID)
		if output, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("删除容器镜像失败: %v: %s", err, string(output))
		}
	case "vm":
		// 先通过 incus image delete 删除镜像
		cmd := exec.Command("incus", "image", "delete", req.ImageID)
		if output, err := cmd.CombinedOutput(); err != nil {
			zap.L().Warn("删除 VM 镜像失败", zap.String("image_id", req.ImageID), zap.Error(err), zap.String("output", string(output)))
		}
		e.downloadManager.RemoveTask(req.ImageID)
	}

	return json.Marshal(map[string]string{"status": "deleted"})
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
		zap.L().Error("handleApplyNetwork 解析 payload 失败", zap.Error(err))
		return nil, err
	}

	zap.L().Info("handleApplyNetwork 开始",
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

		// 配置 SNAT（如果启用且指定了出口 IP）
		if req.SNATEnabled && req.EgressV4Primary != "" && req.ParentIface != "" {
			if err := reconcile.ConfigureSNAT(req.BridgeName, req.IPv4CIDR, req.EgressV4Primary, req.ParentIface); err != nil {
				zap.L().Warn("配置 SNAT 失败", zap.Error(err))
			} else {
				zap.L().Info("SNAT 配置成功", zap.String("egress", req.EgressV4Primary))
			}
		}

	case "update":
		// 更新：重新配置 SNAT 等
		if req.EgressV4Primary != "" && req.ParentIface != "" {
			if req.SNATEnabled {
				if err := reconcile.ConfigureSNAT(req.BridgeName, req.IPv4CIDR, req.EgressV4Primary, req.ParentIface); err != nil {
					zap.L().Warn("更新 SNAT 失败", zap.Error(err))
				}
			} else {
				if err := reconcile.RemoveSNAT(req.BridgeName, req.IPv4CIDR); err != nil {
					zap.L().Warn("移除 SNAT 失败", zap.Error(err))
				}
			}
		}
		zap.L().Info("VPC 更新完成", zap.String("bridge", req.BridgeName))

	case "delete":
		// 先移除 SNAT 规则
		if req.IPv4CIDR != "" {
			if err := reconcile.RemoveSNAT(req.BridgeName, req.IPv4CIDR); err != nil {
				zap.L().Warn("移除 SNAT 失败", zap.Error(err))
			}
		}
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
