package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"

	"tsukiyo/agent/internal/config"
	"tsukiyo/agent/internal/console"
	"tsukiyo/agent/internal/incus"
	"tsukiyo/agent/internal/monitor"
	"tsukiyo/agent/internal/network"
	"tsukiyo/agent/internal/reconcile"
	"tsukiyo/agent/internal/security"
	"tsukiyo/agent/internal/system"
	"tsukiyo/agent/internal/task"
	"tsukiyo/agent/internal/ws"
	"tsukiyo/agent/pkg/logger"
)

const agentVersion = "1.0.0"

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "panic: %v\n", r)
			os.Exit(1)
		}
	}()

	configPath := flag.String("config", getDefaultConfigPath(), "配置文件路径")
	flag.Parse()

	// 初始化日志（使用默认配置先初始化）
	if err := logger.Init("info", "json", ""); err != nil {
		fmt.Fprintf(os.Stderr, "初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	zap.L().Info("Tsukiyo Agent 启动",
		zap.String("version", agentVersion),
		zap.String("config", *configPath))

	// 加载配置
	if err := config.Init(*configPath); err != nil {
		zap.L().Fatal("加载配置失败", zap.Error(err))
	}

	// 重新初始化日志（使用配置文件中的级别）
	logger.Init(config.AppConfig.Log.Level, config.AppConfig.Log.Format, config.AppConfig.Log.OutputPath)

	// 各模块引用
	var (
		incusClient    *incus.Client
		netManager     *network.Manager
		collector      *monitor.Collector
		scanner        *security.Scanner
		consoleHandler *console.Handler
		moduleMu       sync.Mutex
	)

	// 采集静态数据（CPU型号、内存条、GPU、环境工具版本等，仅启动时采集一次）
	system.InitStaticProbe()

	// 初始化 WebSocket 客户端
	wsClient := ws.NewClient(config.AppConfig)

	// 用于等待首次配置下发的 channel
	configReady := make(chan struct{})
	var initOnce sync.Once

	// 保存首次收到的配置数据，供 Incus 初始化后 reconcile
	var firstConfigData map[string]interface{}
	var firstConfigMu sync.Mutex

	// 设置配置处理器
	wsClient.SetConfigHandler(func(data map[string]interface{}) {
		initOnce.Do(func() {
			close(configReady)
		})

		// 首次收到配置时保存
		firstConfigMu.Lock()
		if firstConfigData == nil {
			firstConfigData = data
		}
		firstConfigMu.Unlock()

		moduleMu.Lock()
		ic := incusClient
		moduleMu.Unlock()

		if ic != nil {
			applyConfig(config.AppConfig, ic)

			// 解析 Bridge 配置并执行全量状态对齐
			if bridgesRaw, ok := data["bridges"]; ok {
				var bridges []reconcile.BridgeConfig
				if rawJSON, err := json.Marshal(bridgesRaw); err == nil {
					if err := json.Unmarshal(rawJSON, &bridges); err == nil {
						zap.L().Info("收到 Bridge 配置，执行状态对齐", zap.Int("count", len(bridges)))
						r := reconcile.NewReconciler(ic)
						if err := r.Reconcile(bridges); err != nil {
							zap.L().Error("Bridge 状态对齐失败", zap.Error(err))
						}

						// 解析 EIP 分配信息并矫正 nftables 规则
						if eipRaw, ok := data["eip_allocations"]; ok {
							var eipConfigs []reconcile.EIPAllocationConfig
							if eipJSON, err := json.Marshal(eipRaw); err == nil {
								if err := json.Unmarshal(eipJSON, &eipConfigs); err == nil {
									r.ReconcileEIPs(eipConfigs)
								}
							}
						}
					} else {
						zap.L().Warn("解析 Bridge 配置失败", zap.Error(err))
					}
				}
			} else {
				if bridges, err := reconcile.LoadDesiredState(); err == nil && len(bridges) > 0 {
					zap.L().Info("使用本地持久化 Bridge 状态进行恢复", zap.Int("count", len(bridges)))
					r := reconcile.NewReconciler(ic)
					_ = r.Reconcile(bridges)
				}
			}

			// 同步本地镜像列表
			go syncLocalImages(ic, wsClient)
		}
	})

	// 连接 Master
	if err := wsClient.Connect(); err != nil {
		zap.L().Fatal("连接 Master 失败", zap.Error(err))
	}

	// 等待 Master 下发配置
	zap.L().Info("等待 Master 下发配置...")
	select {
	case <-configReady:
		zap.L().Info("收到 Master 配置，开始初始化各模块")
	case <-time.After(5 * time.Minute):
		zap.L().Fatal("等待 Master 配置超时，请检查 Master 是否已对该节点进行初始化")
	}

	// 初始化 Incus 客户端
	ic, err := incus.NewClient(config.AppConfig.IncusSocketPath())
	if err != nil {
		zap.L().Fatal("初始化 Incus 客户端失败", zap.Error(err))
	}
	moduleMu.Lock()
	incusClient = ic
	moduleMu.Unlock()

	if info, err := ic.GetServerInfo(); err == nil {
		zap.L().Info("Incus 服务器连接成功",
			zap.String("version", info.Environment.ServerVersion),
			zap.String("storage", info.Environment.Storage),
			zap.String("driver", info.Environment.Driver))
	} else {
		zap.L().Warn("获取 Incus 服务器信息失败", zap.Error(err))
	}

	// 首次应用配置
	applyConfig(config.AppConfig, ic)

	// Incus 初始化后用首次收到的配置执行 reconcile
	firstConfigMu.Lock()
	cfgData := firstConfigData
	firstConfigMu.Unlock()
	if cfgData != nil {
		r := reconcile.NewReconciler(ic)

		// reconcile bridge 配置
		if bridgesRaw, ok := cfgData["bridges"]; ok {
			var bridges []reconcile.BridgeConfig
			if rawJSON, err := json.Marshal(bridgesRaw); err == nil {
				if err := json.Unmarshal(rawJSON, &bridges); err == nil {
					zap.L().Info("启动时执行 Bridge 状态对齐", zap.Int("count", len(bridges)))
					if err := r.Reconcile(bridges); err != nil {
						zap.L().Error("Bridge 状态对齐失败", zap.Error(err))
					}
				}
			}
		}

		// reconcile EIP nftables 规则
		if eipRaw, ok := cfgData["eip_allocations"]; ok {
			var eipConfigs []reconcile.EIPAllocationConfig
			if eipJSON, err := json.Marshal(eipRaw); err == nil {
				if err := json.Unmarshal(eipJSON, &eipConfigs); err == nil {
					r.ReconcileEIPs(eipConfigs)
				}
			}
		}

	} else {
		// 没有收到配置，尝试用本地持久化状态
		if bridges, err := reconcile.LoadDesiredState(); err == nil && len(bridges) > 0 {
			zap.L().Info("使用本地持久化 Bridge 状态进行恢复", zap.Int("count", len(bridges)))
			r := reconcile.NewReconciler(ic)
			_ = r.Reconcile(bridges)
		}
	}

	// 上报本地已有镜像列表
	if aliases, err := ic.ListImages(); err == nil && len(aliases) > 0 {
		if err := wsClient.SendLocalImages(aliases); err != nil {
			zap.L().Warn("上报本地镜像列表失败", zap.Error(err))
		} else {
			zap.L().Info("上报本地镜像列表成功", zap.Int("count", len(aliases)))
		}
	}

	// 初始化网络管理器
	netManager = network.NewManager(config.AppConfig.NetworkInterface(), config.AppConfig.EnableNAT(), config.AppConfig.EnableFirewall())
	zap.L().Info("网络管理器初始化完成",
		zap.String("interface", netManager.GetInterfaceName()),
		zap.Bool("nat", config.AppConfig.EnableNAT()),
		zap.Bool("firewall", config.AppConfig.EnableFirewall()))

	// 初始化任务执行器
	taskExecutor := task.NewExecutor(config.AppConfig, ic, netManager, wsClient)
	wsClient.SetTaskHandler(func(taskID string, taskType string, payload json.RawMessage) (json.RawMessage, error) {
		// 将 task_id 注入 payload
		var raw map[string]interface{}
		if err := json.Unmarshal(payload, &raw); err != nil {
			return taskExecutor.Execute(taskType, payload)
		}
		raw["task_id"] = taskID
		newPayload, err := json.Marshal(raw)
		if err != nil {
			return taskExecutor.Execute(taskType, payload)
		}
		return taskExecutor.Execute(taskType, newPayload)
	})

	// 注册 Master 同步请求处理器
	wsClient.SetRequestHandler(func(reqType string, payload json.RawMessage) (json.RawMessage, error) {
		switch reqType {
		case "get_storages":
			storages, err := ic.ListStoragePools()
			if err != nil {
				return nil, err
			}
			return json.Marshal(storages)
		case "get_instances":
			instances, err := ic.ListInstances()
			if err != nil {
				return nil, err
			}
			return json.Marshal(instances)
		case "get_images":
			images, err := ic.ListImages()
			if err != nil {
				return nil, err
			}
			return json.Marshal(images)
		case "list_remote_images":
			var req struct {
				Remote string `json:"remote"`
			}
			if err := json.Unmarshal(payload, &req); err != nil {
				return nil, fmt.Errorf("解析请求参数失败: %w", err)
			}
			if req.Remote == "" {
				req.Remote = "images:"
			}
			images, err := ic.ListRemoteImages(req.Remote)
			if err != nil {
				return nil, fmt.Errorf("获取远程镜像列表失败: %w", err)
			}
			return json.Marshal(map[string]interface{}{
				"images": images,
				"total":  len(images),
			})
		case "get_disks":
			return handleGetDisks()
		case "delete_storage":
			var req struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(payload, &req); err != nil {
				return nil, fmt.Errorf("解析删除存储池参数失败: %w", err)
			}
			detail, err := ic.GetStoragePool(req.Name)
			if err != nil {
				return nil, fmt.Errorf("查询存储池 %s 失败: %w", req.Name, err)
			}
			if len(detail.UsedBy) > 0 {
				return nil, fmt.Errorf("存储池 %s 仍有 %d 个资源在使用，无法删除", req.Name, len(detail.UsedBy))
			}
			if err := ic.DeleteStoragePool(req.Name); err != nil {
				return nil, err
			}
			return json.Marshal(map[string]interface{}{"deleted": true, "name": req.Name})
		case "get_volumes":
			var req struct {
				Pool string `json:"pool"`
			}
			if err := json.Unmarshal(payload, &req); err != nil {
				return nil, fmt.Errorf("解析获取卷列表参数失败: %w", err)
			}
			volumes, err := ic.ListStorageVolumes(req.Pool)
			if err != nil {
				return nil, err
			}
			return json.Marshal(volumes)
		case "get_storage_resources":
			var req struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(payload, &req); err != nil {
				return nil, fmt.Errorf("解析获取存储资源参数失败: %w", err)
			}
			res, err := ic.GetStoragePoolResources(req.Name)
			if err != nil {
				return nil, err
			}
			return json.Marshal(res)
		case "add_port_mapping":
			var req struct {
				InstanceID    string `json:"instance_id"`
				HostPort      int    `json:"host_port"`
				ContainerPort int    `json:"container_port"`
				Protocol      string `json:"protocol"`
				HostIP        string `json:"host_ip"`
			}
			if err := json.Unmarshal(payload, &req); err != nil {
				return nil, fmt.Errorf("解析端口映射参数失败: %w", err)
			}
			ips, err := ic.GetInstanceNetworkInfo(req.InstanceID)
			if err != nil || len(ips) == 0 {
				return nil, fmt.Errorf("获取实例 %s 内网 IP 失败", req.InstanceID)
			}
			hostIP := req.HostIP
			if idx := strings.Index(hostIP, "/"); idx > 0 {
				hostIP = hostIP[:idx]
			}
			if hostIP == "" {
				hostIP = "0.0.0.0"
			}
			deviceName := fmt.Sprintf("proxy-%d-%s", req.HostPort, req.Protocol)
			listenAddr := fmt.Sprintf("%s:%s:%d", req.Protocol, hostIP, req.HostPort)
			connectAddr := fmt.Sprintf("%s:%s:%d", req.Protocol, ips[0], req.ContainerPort)
			if err := ic.AddProxyDevice(req.InstanceID, deviceName, listenAddr, connectAddr); err != nil {
				return nil, fmt.Errorf("添加端口映射失败: %w", err)
			}
			return json.Marshal(map[string]interface{}{
				"success":     true,
				"device_name": deviceName,
				"listen":      listenAddr,
				"connect":     connectAddr,
			})
		case "del_port_mapping":
			var req struct {
				InstanceID string `json:"instance_id"`
				HostPort   int    `json:"host_port"`
				Protocol   string `json:"protocol"`
			}
			if err := json.Unmarshal(payload, &req); err != nil {
				return nil, fmt.Errorf("解析删除端口映射参数失败: %w", err)
			}
			deviceName := fmt.Sprintf("proxy-%d-%s", req.HostPort, req.Protocol)
			if err := ic.RemoveProxyDevice(req.InstanceID, deviceName); err != nil {
				return nil, fmt.Errorf("删除端口映射失败: %w", err)
			}
			return json.Marshal(map[string]interface{}{"deleted": true, "device_name": deviceName})
		case "reset_password":
			var req struct {
				InstanceID string `json:"instance_id"`
				Password   string `json:"password"`
			}
			if err := json.Unmarshal(payload, &req); err != nil {
				return nil, fmt.Errorf("解析重置密码参数失败: %w", err)
			}
			if err := ic.SetInstancePassword(req.InstanceID, req.Password); err != nil {
				return nil, fmt.Errorf("重置密码失败: %w", err)
			}
			return json.Marshal(map[string]interface{}{"success": true, "instance_id": req.InstanceID})
		case "add_firewall_rule":
			var req struct {
				Direction string `json:"direction"`
				Protocol  string `json:"protocol"`
				Source    string `json:"source"`
				Port      int    `json:"port"`
				Action    string `json:"action"`
			}
			if err := json.Unmarshal(payload, &req); err != nil {
				return nil, fmt.Errorf("解析防火墙规则参数失败: %w", err)
			}
			netManager.AddFirewallRule(req.Direction, req.Protocol, req.Source, req.Port, req.Action)
			return json.Marshal(map[string]interface{}{"success": true})
		case "del_firewall_rule":
			var req struct {
				Direction string `json:"direction"`
				Protocol  string `json:"protocol"`
				Source    string `json:"source"`
				Port      int    `json:"port"`
			}
			if err := json.Unmarshal(payload, &req); err != nil {
				return nil, fmt.Errorf("解析删除防火墙规则参数失败: %w", err)
			}
			netManager.RemoveFirewallRule(req.Direction, req.Protocol, req.Source, req.Port)
			return json.Marshal(map[string]interface{}{"deleted": true})
		case "add_ip":
			var req struct {
				CIDR      string `json:"cidr"`
				Interface string `json:"interface"`
			}
			if err := json.Unmarshal(payload, &req); err != nil {
				return nil, fmt.Errorf("解析参数失败: %w", err)
			}
			if req.CIDR == "" {
				return nil, fmt.Errorf("cidr 不能为空")
			}
			if err := netManager.BindIP(req.CIDR, req.Interface); err != nil {
				return nil, fmt.Errorf("添加 IP 失败: %w", err)
			}
			return json.Marshal(map[string]interface{}{"success": true})
		case "bridge_network":
			return taskExecutor.Execute("bridge_network", payload)
		case "bind_bridge_egress":
			return taskExecutor.Execute("bind_bridge_egress", payload)
		case "unbind_bridge_egress":
			return taskExecutor.Execute("unbind_bridge_egress", payload)
		case "create_partition":
			return taskExecutor.Execute("create_partition", payload)
		case "delete_partition":
			return taskExecutor.Execute("delete_partition", payload)
		case "format_disk":
			return taskExecutor.Execute("format_disk", payload)
		case "init_storage":
			return taskExecutor.Execute("init_storage", payload)
		default:
			return nil, fmt.Errorf("未知请求类型: %s", reqType)
		}
	})

	// 初始化监控采集器
	collector = monitor.NewCollector(config.AppConfig, wsClient, ic)
	collector.Start()

	// 初始化安全扫描器
	scanner = security.NewScanner(config.AppConfig, netManager, wsClient)
	scanner.Start()

	// 初始化控制台处理器（通过 WS 消息处理，不监听任何端口）
	consoleHandler = console.NewHandler()
	vncHandler := console.NewVNCHandler(config.AppConfig.IncusSocketPath())
	wsClient.SetConsoleHandler(func(msgType string, payload json.RawMessage) {
		var msg struct {
			SessionID string `json:"session_id"`
			Container string `json:"container"`
			Data      string `json:"data"`
			Cols      int    `json:"cols"`
			Rows      int    `json:"rows"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil {
			zap.L().Warn("解析控制台消息失败", zap.Error(err))
			return
		}
		switch msgType {
		case "console_ssh_start":
			if err := consoleHandler.StartSSH(msg.SessionID, msg.Container, func(streamType string, data []byte) {
				wsClient.SendConsoleMessage("console_data", map[string]interface{}{
					"session_id": msg.SessionID,
					"stream":     streamType,
					"data":       string(data),
				})
			}); err != nil {
				zap.L().Error("启动控制台会话失败", zap.Error(err))
				wsClient.SendConsoleMessage("console_error", map[string]interface{}{
					"session_id": msg.SessionID,
					"error":      err.Error(),
				})
			}
		case "console_ssh_input":
			consoleHandler.WriteInput(msg.SessionID, []byte(msg.Data))
		case "console_ssh_resize":
			if err := consoleHandler.ResizeSession(msg.SessionID, msg.Cols, msg.Rows); err != nil {
				zap.L().Warn("调整控制台窗口大小失败", zap.String("session_id", msg.SessionID), zap.Error(err))
			}
		case "console_ssh_close":
			consoleHandler.RemoveSession(msg.SessionID)
		case "console_vnc_start":
			if err := vncHandler.StartVNC(msg.SessionID, msg.Container, func(data []byte) {
				wsClient.SendConsoleMessage("console_vnc_data", map[string]interface{}{
					"session_id": msg.SessionID,
					"data":       string(data),
				})
			}); err != nil {
				zap.L().Error("启动 VNC 控制台失败", zap.Error(err))
				wsClient.SendConsoleMessage("console_vnc_error", map[string]interface{}{
					"session_id": msg.SessionID,
					"error":      err.Error(),
				})
			}
		case "console_vnc_input":
			vncHandler.WriteVNCInput(msg.SessionID, []byte(msg.Data))
		case "console_vnc_close":
			vncHandler.RemoveVNCSession(msg.SessionID)
		}
	})

	// 首次上报本地 Incus 镜像列表
	go func() {
		time.Sleep(3 * time.Second)
		if !ic.IsAvailable() {
			return
		}
		syncLocalImages(ic, wsClient)
	}()

	var wg sync.WaitGroup

	// systemd notify
	systemdNotify("READY=1")

	zap.L().Info("Agent 初始化完成，运行中...")

	// 等待退出信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	zap.L().Info("收到退出信号，正在关闭...")

	// 优雅关闭
	wsClient.Shutdown()
	collector.Stop()
	scanner.Stop()

	wg.Wait()
	zap.L().Info("Agent 已退出")
}

func syncLocalImages(ic *incus.Client, wsClient *ws.Client) {
	if !ic.IsAvailable() {
		return
	}
	aliases, err := ic.ListImages()
	if err != nil {
		if ic.IsAvailable() {
			zap.L().Warn("查询本地 Incus 镜像失败", zap.Error(err))
		}
		return
	}
	if len(aliases) == 0 {
		return
	}
	if err := wsClient.SendLocalImages(aliases); err != nil {
		zap.L().Warn("上报本地镜像列表失败", zap.Error(err))
	} else {
		zap.L().Info("本地镜像列表已上报", zap.Int("count", len(aliases)))
	}
}

func applyConfig(cfg *config.Config, ic *incus.Client) {
	// 同步镜像源 remote 到 Incus
	syncImageRemote(cfg, ic)
}

// syncImageRemote 根据配置同步镜像源 remote 到 Incus
func syncImageRemote(cfg *config.Config, ic *incus.Client) {
	remoteURL := cfg.ImageRemoteURL()
	remoteName, serverURL := parseRemoteConfig(remoteURL)
	if remoteName == "" || serverURL == "" {
		return
	}
	if err := ic.SyncRemote(remoteName, serverURL); err != nil {
		zap.L().Warn("同步镜像源 remote 失败", zap.String("remote", remoteName), zap.String("url", serverURL), zap.Error(err))
	} else {
		zap.L().Info("镜像源 remote 同步成功", zap.String("remote", remoteName), zap.String("url", serverURL))
	}
}

// parseRemoteConfig 解析镜像源配置，返回 remote 名称和服务器 URL
// "spiritlhl:" -> ("spiritlhl", "https://incusimages.spiritlhl.net")
// "images:" -> ("images", "https://images.linuxcontainers.org")
// "https://mirror.example.com" -> ("tsukiyo-mirror", "https://mirror.example.com")
// "" -> ("spiritlhl", "https://incusimages.spiritlhl.net")
func parseRemoteConfig(remoteURL string) (name, serverURL string) {
	remoteURL = strings.TrimSpace(remoteURL)
	remoteURL = strings.TrimSuffix(remoteURL, ":")
	switch remoteURL {
	case "", "spiritlhl":
		return "spiritlhl", "https://incusimages.spiritlhl.net"
	case "images":
		return "images", "https://images.linuxcontainers.org"
	default:
		if strings.HasPrefix(remoteURL, "http://") || strings.HasPrefix(remoteURL, "https://") {
			return "tsukiyo-mirror", strings.TrimRight(remoteURL, "/")
		}
		return "spiritlhl", "https://incusimages.spiritlhl.net"
	}
}

func systemdNotify(state string) {
	socket := os.Getenv("NOTIFY_SOCKET")
	if socket == "" {
		return
	}
	zap.L().Info("发送 systemd 通知", zap.String("state", state))
}

func getDefaultConfigPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "config.yaml"
	}
	return filepath.Join(filepath.Dir(exe), "config.yaml")
}

type partitionInfoResult struct {
	Device     string `json:"device"`
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	Used       int64  `json:"used,omitempty"`
	Type       string `json:"type,omitempty"`
	Filesystem string `json:"filesystem,omitempty"`
	MountPoint string `json:"mount_point,omitempty"`
	IsSystem   bool   `json:"is_system"`
}

type diskInfoResult struct {
	Device     string                `json:"device"`
	Size       int64                 `json:"size"`
	Used       int64                 `json:"used,omitempty"`
	Model      string                `json:"model,omitempty"`
	Serial     string                `json:"serial,omitempty"`
	Type       string                `json:"type,omitempty"`
	Filesystem string                `json:"filesystem,omitempty"`
	MountPoint string                `json:"mount_point,omitempty"`
	IsSystem   bool                  `json:"is_system"`
	IsInUse    bool                  `json:"is_in_use"`
	Partitions []partitionInfoResult `json:"partitions,omitempty"`
}

func handleGetDisks() (json.RawMessage, error) {
	// 获取各挂载点使用量
	usedMap := make(map[string]int64)
	dfCmd := exec.Command("df", "-B1", "--output=used,target")
	if dfOut, err := dfCmd.CombinedOutput(); err == nil {
		lines := strings.Split(string(dfOut), "\n")
		for i, line := range lines {
			if i == 0 {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				used, _ := strconv.ParseInt(fields[0], 10, 64)
				usedMap[fields[1]] = used
			}
		}
	}

	cmd := exec.Command("lsblk", "--json", "-b", "-o", "NAME,SIZE,TYPE,FSTYPE,MOUNTPOINT,MODEL,SERIAL")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("lsblk 执行失败: %w, output: %s", err, string(output))
	}

	var lsblkOutput struct {
		BlockDevices []struct {
			Name       string `json:"name"`
			Size       int64  `json:"size"`
			Type       string `json:"type"`
			Fstype     string `json:"fstype"`
			Mountpoint string `json:"mountpoint"`
			Model      string `json:"model"`
			Serial     string `json:"serial"`
			Children   []struct {
				Name       string `json:"name"`
				Size       int64  `json:"size"`
				Type       string `json:"type"`
				Fstype     string `json:"fstype"`
				Mountpoint string `json:"mountpoint"`
			} `json:"children"`
		} `json:"blockdevices"`
	}
	if err := json.Unmarshal(output, &lsblkOutput); err != nil {
		return nil, fmt.Errorf("解析 lsblk 输出失败: %w", err)
	}

	var disks []diskInfoResult
	for _, dev := range lsblkOutput.BlockDevices {
		if dev.Type != "disk" {
			continue
		}

		isSystem := false
		isInUse := false
		fstype := dev.Fstype
		mountpoint := dev.Mountpoint

		if mountpoint == "/" || mountpoint == "/boot" || strings.HasPrefix(mountpoint, "/boot") {
			isSystem = true
		}
		if fstype != "" || mountpoint != "" {
			isInUse = true
		}

		var partitions []partitionInfoResult
		for _, child := range dev.Children {
			if child.Mountpoint == "/" || child.Mountpoint == "/boot" || strings.HasPrefix(child.Mountpoint, "/boot") {
				isSystem = true
			}
			if child.Fstype != "" || child.Mountpoint != "" {
				isInUse = true
			}
			if fstype == "" && child.Fstype != "" {
				fstype = child.Fstype
			}
			if mountpoint == "" && child.Mountpoint != "" {
				mountpoint = child.Mountpoint
			}

			childDevice := "/dev/" + child.Name
			childIsSystem := child.Mountpoint == "/" || child.Mountpoint == "/boot" || strings.HasPrefix(child.Mountpoint, "/boot")

			partitions = append(partitions, partitionInfoResult{
				Device:     childDevice,
				Name:       child.Name,
				Size:       child.Size,
				Used:       usedMap[child.Mountpoint],
				Type:       child.Type,
				Filesystem: child.Fstype,
				MountPoint: child.Mountpoint,
				IsSystem:   childIsSystem,
			})
		}

		disks = append(disks, diskInfoResult{
			Device:     "/dev/" + dev.Name,
			Size:       dev.Size,
			Used:       usedMap[mountpoint],
			Model:      strings.TrimSpace(dev.Model),
			Serial:     strings.TrimSpace(dev.Serial),
			Type:       dev.Type,
			Filesystem: fstype,
			MountPoint: mountpoint,
			IsSystem:   isSystem,
			IsInUse:    isInUse,
			Partitions: partitions,
		})
	}

	return json.Marshal(disks)
}
