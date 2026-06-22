package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"tsukiyo/master/internal/agent"
	"tsukiyo/master/internal/api"
	"tsukiyo/master/internal/api/handlers"
	"tsukiyo/master/internal/config"
	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/schedule"
	"tsukiyo/master/internal/security"
	infra "tsukiyo/master/internal/service/infrastructure"
	"tsukiyo/master/internal/task"
	"tsukiyo/master/pkg/logger"
)

func main() {
	// 初始化日志
	if err := logger.Init("info", "json", ""); err != nil {
		fmt.Fprintf(os.Stderr, "初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	zap.L().Info("Tsukiyo Master 启动中...")

	// 加载配置
	if err := config.Init(); err != nil {
		zap.L().Fatal("加载配置失败", zap.Error(err))
	}

	// 重新初始化日志 (使用配置文件中的级别)
	logger.Init(config.AppConfig.Log.Level, config.AppConfig.Log.Format, config.AppConfig.Log.OutputPath)

	// 初始化数据库
	if err := db.Init(&config.AppConfig.Database); err != nil {
		zap.L().Fatal("初始化数据库失败", zap.Error(err))
	}

	// 执行数据库迁移（golang-migrate，版本追踪 + advisory lock）
	if err := db.RunMigrations(&config.AppConfig.Database); err != nil {
		zap.L().Fatal("数据库迁移失败", zap.Error(err))
	}

	// 初始化 Redis
	if err := db.InitRedis(&config.AppConfig.Redis); err != nil {
		zap.L().Fatal("初始化 Redis 失败", zap.Error(err))
	}

	// 创建 Agent 管理器
	agentMgr := agent.NewManager()
	agentMgr.StartHeartbeatChecker()

	// 创建网络服务（供 Scheduler 释放资源使用）
	networkSvc := infra.NewNetworkService(agentMgr)

	// 启动任务调度器
	taskScheduler := task.NewScheduler(agentMgr, networkSvc)
	taskScheduler.Start()

	// 注入任务结果处理器
	agentMgr.OnTaskResult = taskScheduler.HandleTaskResult

	// 启动定时任务
	cronScheduler := schedule.NewScheduler()
	cronScheduler.Start()

	// 启动安全扫描器
	secScanner := security.NewScanner()
	secScanner.Start()

	// 配置路由
	router := api.SetupRouter(agentMgr)

	// 注入全局广播函数，handler 变更数据后通知前端刷新
	handlers.SetBroadcastFn(func(msgType string, payload interface{}) {
		agentMgr.BroadcastToFrontend(map[string]interface{}{
			"type":    msgType,
			"payload": payload,
		})
	})

	// HTTP 服务器 (同时处理 WebSocket)
	httpServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", config.AppConfig.Server.Host, config.AppConfig.Server.Port),
		Handler:      router,
		ReadTimeout:  config.AppConfig.Server.ReadTimeout,
		WriteTimeout: config.AppConfig.Server.WriteTimeout,
	}

	// 启动 HTTP 服务器
	go func() {
		zap.L().Info("HTTP 服务器启动", zap.String("addr", httpServer.Addr))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zap.L().Fatal("HTTP 服务器启动失败", zap.Error(err))
		}
	}()

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	zap.L().Info("正在关闭服务器...")

	// 停止后台服务
	taskScheduler.Stop()
	cronScheduler.Stop()
	secScanner.Stop()

	// 先关闭所有 WebSocket 连接
	agentMgr.CloseAll()

	// 直接关闭 HTTP listener，不等待残留连接
	if err := httpServer.Close(); err != nil {
		zap.L().Error("HTTP 服务器关闭失败", zap.Error(err))
	}

	zap.L().Info("服务器已关闭")
}
