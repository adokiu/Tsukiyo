package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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
	"tsukiyo/agent/internal/task"
	"tsukiyo/agent/internal/ws"
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

	// 初始化日志
	logger, _ := zap.NewProduction()
	if os.Getenv("DEBUG") == "1" {
		logger, _ = zap.NewDevelopment()
	}
	zap.ReplaceGlobals(logger)
	defer logger.Sync()

	zap.L().Info("Tsukiyo Agent 启动",
		zap.String("version", agentVersion),
		zap.String("config", *configPath))

	// 加载配置（仅 master + token）
	cfg, err := config.Load(*configPath)
	if err != nil {
		zap.L().Fatal("加载配置失败", zap.Error(err))
	}

	// 各模块引用，初始化后赋值，后续 configHandler 可访问
	var (
		incusClient  *incus.Client
		netManager   *network.Manager
		collector    *monitor.Collector
		scanner      *security.Scanner
		consoleProxy *console.Proxy
		moduleMu     sync.Mutex
	)

	// 初始化 WebSocket 客户端
	wsClient := ws.NewClient(cfg)

	// 用于等待首次配置下发的 channel
	configReady := make(chan struct{})
	var initOnce sync.Once

	// 设置配置处理器
	wsClient.SetConfigHandler(func(data map[string]interface{}) {
		// 首次收到配置：解除 main 阻塞
		initOnce.Do(func() {
			close(configReady)
		})

		// 每次收到配置都尝试应用（幂等）
		moduleMu.Lock()
		ic := incusClient
		moduleMu.Unlock()

		if ic != nil {
			applyConfig(cfg, ic)

			// 解析 VPC 配置并执行全量状态对齐（宿主机重启后恢复 bridge + SNAT）
			if vpcsRaw, ok := data["vpcs"]; ok {
				var vpcs []reconcile.VPCConfig
				if rawJSON, err := json.Marshal(vpcsRaw); err == nil {
					if err := json.Unmarshal(rawJSON, &vpcs); err == nil {
						zap.L().Info("收到 VPC 配置，执行状态对齐", zap.Int("count", len(vpcs)))
						r := reconcile.NewReconciler(ic)
						if err := r.Reconcile(vpcs); err != nil {
							zap.L().Error("VPC 状态对齐失败", zap.Error(err))
						}
					} else {
						zap.L().Warn("解析 VPC 配置失败", zap.Error(err))
					}
				}
			} else {
				// Master 未下发 VPC 配置，尝试用本地持久化状态恢复
				if vpcs, err := reconcile.LoadDesiredState(); err == nil && len(vpcs) > 0 {
					zap.L().Info("使用本地持久化 VPC 状态进行恢复", zap.Int("count", len(vpcs)))
					r := reconcile.NewReconciler(ic)
					_ = r.Reconcile(vpcs)
				}
			}

			// 解析端口映射配置并恢复 proxy 设备（宿主机重启后恢复端口映射）
			if pmsRaw, ok := data["port_mappings"]; ok {
				var pms []reconcile.PortMappingConfig
				if rawJSON, err := json.Marshal(pmsRaw); err == nil {
					if err := json.Unmarshal(rawJSON, &pms); err == nil {
						zap.L().Info("收到端口映射配置，执行恢复", zap.Int("count", len(pms)))
						r := reconcile.NewReconciler(ic)
						_ = r.ReconcilePortMappings(pms)
					} else {
						zap.L().Warn("解析端口映射配置失败", zap.Error(err))
					}
				}
			}

			// 每次收到 Master 配置（含重连后）都同步本地镜像列表
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

	// ========== 初始化 Incus 客户端 ==========
	ic, err := incus.NewClient(cfg.IncusSocketPath())
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

	// 首次应用配置（存储池等）
	applyConfig(cfg, ic)

	// 上报本地已有镜像列表
	if aliases, err := ic.ListImages(); err == nil && len(aliases) > 0 {
		if err := wsClient.SendLocalImages(aliases); err != nil {
			zap.L().Warn("上报本地镜像列表失败", zap.Error(err))
		} else {
			zap.L().Info("上报本地镜像列表成功", zap.Int("count", len(aliases)))
		}
	}

	// ========== 初始化网络管理器 ==========
	netManager = network.NewManager(cfg.NetworkInterface(), cfg.EnableNAT(), cfg.EnableFirewall())
	zap.L().Info("网络管理器初始化完成",
		zap.String("interface", netManager.GetInterfaceName()),
		zap.Bool("nat", cfg.EnableNAT()),
		zap.Bool("firewall", cfg.EnableFirewall()))

	// ========== 初始化任务执行器 ==========
	taskExecutor := task.NewExecutor(cfg, ic, netManager, wsClient)
	wsClient.SetTaskHandler(func(taskID string, taskType string, payload json.RawMessage) (json.RawMessage, error) {
		return taskExecutor.Execute(taskType, payload)
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
		default:
			return nil, fmt.Errorf("未知请求类型: %s", reqType)
		}
	})

	// ========== 初始化监控采集器 ==========
	collector = monitor.NewCollector(cfg, wsClient, ic)
	collector.Start()

	// ========== 初始化安全扫描器 ==========
	scanner = security.NewScanner(cfg, netManager)
	scanner.Start()

	// ========== 初始化控制台代理 ==========
	consoleProxy = console.NewProxy(cfg)
	go func() {
		if err := consoleProxy.ServeHTTP(); err != nil {
			zap.L().Error("控制台代理异常", zap.Error(err))
		}
	}()

	// ========== 首次上报本地 Incus 镜像列表 ==========
	go func() {
		time.Sleep(3 * time.Second)
		syncLocalImages(ic, wsClient)
	}()

	// ========== 确保所有 bridge 网络启用 NAT ==========
	go func() {
		time.Sleep(2 * time.Second)
		r := reconcile.NewReconciler(ic)
		r.EnsureAllBridgeNAT()
	}()

	// ========== 健康检查 HTTP ==========
	var wg sync.WaitGroup
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		status := map[string]interface{}{
			"status":    "healthy",
			"version":   agentVersion,
			"connected": wsClient.IsConnected(),
			"timestamp": time.Now().Unix(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})
	healthMux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		ready := wsClient.IsConnected()
		if ready {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ready":true}`))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"ready":false}`))
		}
	})
	healthServer := &http.Server{
		Addr:    ":9090",
		Handler: healthMux,
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		zap.L().Info("健康检查服务启动", zap.String("addr", ":9090"))
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zap.L().Error("健康检查服务异常", zap.Error(err))
		}
	}()

	// systemd notify (如果可用)
	systemdNotify("READY=1")

	zap.L().Info("Agent 初始化完成，运行中...")

	// 等待退出信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	zap.L().Info("收到退出信号，正在关闭...")

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	healthServer.Shutdown(ctx)

	wsClient.Shutdown()
	collector.Stop()
	scanner.Stop()

	wg.Wait()
	zap.L().Info("Agent 已退出")
}

// syncLocalImages 查询 Incus 本地镜像并上报 Master
func syncLocalImages(ic *incus.Client, wsClient *ws.Client) {
	aliases, err := ic.ListImages()
	if err != nil {
		zap.L().Warn("查询本地 Incus 镜像失败", zap.Error(err))
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

// applyConfig 应用 Master 下发的配置（幂等，每次收到 config 消息都调用）
func applyConfig(cfg *config.Config, ic *incus.Client) {
	poolName := cfg.DefaultStoragePool()
	poolType := cfg.StoragePoolType()
	poolSource := cfg.StoragePoolSource()

	if poolName == "" || poolType == "" {
		zap.L().Debug("存储池配置未指定，跳过")
		return
	}

	if ic.StoragePoolExists(poolName) {
		zap.L().Info("存储池已存在，跳过创建",
			zap.String("pool", poolName))
		return
	}

	zap.L().Info("正在创建存储池",
		zap.String("pool", poolName),
		zap.String("type", poolType),
		zap.String("source", poolSource))

	if err := ic.CreateStoragePool(poolName, poolType, poolSource); err != nil {
		zap.L().Error("创建存储池失败",
			zap.String("pool", poolName),
			zap.Error(err))
	} else {
		zap.L().Info("存储池创建成功", zap.String("pool", poolName))
	}
}

// systemdNotify 发送 systemd 通知 (如果支持)
func systemdNotify(state string) {
	socket := os.Getenv("NOTIFY_SOCKET")
	if socket == "" {
		return
	}
	// 简化的 systemd 通知
	zap.L().Info("发送 systemd 通知", zap.String("state", state))
}

// getDefaultConfigPath 返回可执行文件同目录下的 config.yaml 路径
func getDefaultConfigPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "config.yaml"
	}
	return filepath.Join(filepath.Dir(exe), "config.yaml")
}
