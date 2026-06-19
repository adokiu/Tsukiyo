package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"tsukiyo/master/internal/agent"
	"tsukiyo/master/internal/api/handlers"
	"tsukiyo/master/internal/api/middleware"
	"tsukiyo/master/internal/console"
	infra "tsukiyo/master/internal/service/infrastructure"
	inst "tsukiyo/master/internal/service/instance"
	sys "tsukiyo/master/internal/service/system"
	usr "tsukiyo/master/internal/service/user"
)

// SetupRouter 配置路由
func SetupRouter(agentMgr *agent.Manager) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// 全局中间件
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware())
	r.Use(gin.Logger())

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Agent WebSocket 连接端点
	r.GET("/ws/agent", agentMgr.HandleWebSocket)
	// 前端 WebSocket 推送端点（镜像进度等实时数据）
	r.GET("/ws/images", agentMgr.HandleFrontendWebSocket)
	// 前端 WebSocket 推送端点（任务状态等实时数据）
	r.GET("/ws/tasks", agentMgr.HandleFrontendWebSocket)

	// 初始化服务层
	imageService := infra.NewImageService(agentMgr)
	handlers.InitImageService(imageService)
	nodeService := infra.NewNodeService(agentMgr)
	handlers.InitNodeService(nodeService)
	userService := usr.NewUserService()
	handlers.InitUserService(userService)
	networkService := infra.NewNetworkService(agentMgr)
	handlers.InitNetworkService(networkService)
	instanceService := inst.NewInstanceService()
	handlers.InitInstanceService(instanceService)
	snapshotService := inst.NewSnapshotService()
	handlers.InitSnapshotService(snapshotService)
	taskService := inst.NewTaskService()
	handlers.InitTaskService(taskService)
	storageService := infra.NewStorageService(agentMgr)
	handlers.InitStorageService(storageService)
	authService := usr.NewAuthService()
	handlers.InitAuthService(authService)
	auditService := sys.NewAuditService()
	handlers.InitAuditService(auditService)

	// API v1
	v1 := r.Group("/api/v1")

	// 公开接口 (无需认证)
	v1.GET("/init/status", handlers.GetInitStatus)
	v1.POST("/init/setup", handlers.InitSetup)
	v1.POST("/auth/login", handlers.Login)
	v1.POST("/auth/register", handlers.Register)

	// 需要认证的接口
	authGroup := v1.Group("")
	authGroup.Use(middleware.AuthMiddleware())
	authGroup.Use(middleware.RBACMiddleware())
	{
		authGroup.POST("/auth/logout", handlers.Logout)
		authGroup.POST("/auth/change-password", handlers.ChangePassword)

		// 用户管理
		authGroup.GET("/users", handlers.ListUsers)
		authGroup.GET("/users/:id", handlers.GetUser)
		authGroup.PUT("/users/:id", handlers.UpdateUser)
		authGroup.DELETE("/users/:id", handlers.DeleteUser)

		// 用户组管理
		authGroup.GET("/user-groups", handlers.ListUserGroups)
		authGroup.POST("/user-groups", handlers.CreateUserGroup)
		authGroup.PUT("/user-groups/:id", handlers.UpdateUserGroup)
		authGroup.DELETE("/user-groups/:id", handlers.DeleteUserGroup)

		// 节点管理
		authGroup.GET("/nodes", handlers.ListNodes)
		authGroup.POST("/nodes", handlers.CreateNode)
		authGroup.GET("/nodes/:id", handlers.GetNode)
		authGroup.PUT("/nodes/:id/config", handlers.UpdateNodeConfig)
		authGroup.DELETE("/nodes/:id", handlers.DeleteNode)
		authGroup.GET("/nodes/:id/disks", handlers.ListNodeDisks)
		authGroup.POST("/nodes/:id/disks/format", handlers.FormatNodeDisk)
		authGroup.GET("/nodes/:id/storages", handlers.ListNodeStorages)
		authGroup.POST("/nodes/:id/storages/init", handlers.InitNodeStorage)
		authGroup.GET("/nodes/:id/networks", handlers.GetNodeNetworks)

		// 实例管理
		authGroup.GET("/instances", handlers.ListInstances)
		authGroup.POST("/instances", handlers.CreateInstance)
		authGroup.GET("/instances/:id", handlers.GetInstance)
		authGroup.PUT("/instances/:id", handlers.UpdateInstance)
		authGroup.DELETE("/instances/:id", handlers.DeleteInstance)
		authGroup.POST("/instances/:id/start", handlers.StartInstance)
		authGroup.POST("/instances/:id/stop", handlers.StopInstance)
		authGroup.POST("/instances/:id/restart", handlers.RestartInstance)
		authGroup.POST("/instances/:id/reinstall", handlers.ReinstallInstance)
		authGroup.POST("/instances/:id/resize", handlers.ResizeInstance)
		authGroup.POST("/instances/:id/reset-password", handlers.ResetInstancePassword)
		authGroup.GET("/instances/:id/console", handlers.GetInstanceConsole)
		authGroup.GET("/instances/:id/metrics", handlers.GetInstanceMetrics)

		// 快照
		authGroup.GET("/instances/:id/snapshots", handlers.ListSnapshots)
		authGroup.POST("/instances/:id/snapshots", handlers.CreateSnapshot)
		authGroup.POST("/instances/:id/snapshots/:name/restore", handlers.RestoreSnapshot)
		authGroup.DELETE("/instances/:id/snapshots/:name", handlers.DeleteSnapshot)

		// 镜像管理（预制模板，不支持手动创建）
		authGroup.GET("/images", handlers.ListImages)
		authGroup.POST("/images/remote/list", handlers.ListRemoteImages)
		authGroup.POST("/images/download", handlers.DownloadImage)
		authGroup.GET("/images/progress", handlers.GetImageProgress)
		authGroup.POST("/images/cancel", handlers.CancelImageDownload)
		authGroup.DELETE("/images", handlers.DeleteImage)

		// VPC 网络管理
		authGroup.GET("/network/vpcs", handlers.ListVPCs)
		authGroup.POST("/network/vpcs", handlers.CreateVPC)
		authGroup.GET("/network/vpcs/:id", handlers.GetVPC)
		authGroup.PUT("/network/vpcs/:id", handlers.UpdateVPC)
		authGroup.DELETE("/network/vpcs/:id", handlers.DeleteVPC)

		// IP 池管理
		authGroup.GET("/network/pools", handlers.ListIPPools)
		authGroup.POST("/network/pools", handlers.AddIPPool)
		authGroup.DELETE("/network/pools/:id", handlers.DeleteIPPool)
		authGroup.GET("/network/prefixes", handlers.ListIPv6Prefixes)

		// 端口映射
		authGroup.GET("/network/port-mappings", handlers.ListPortMappings)
		authGroup.POST("/network/port-mappings", handlers.AddPortMapping)
		authGroup.DELETE("/network/port-mappings/:id", handlers.DeletePortMapping)

		// 防火墙
		authGroup.GET("/network/firewall", handlers.ListFirewallRules)
		authGroup.POST("/network/firewall", handlers.AddFirewallRule)
		authGroup.PUT("/network/firewall/:id", handlers.UpdateFirewallRule)
		authGroup.DELETE("/network/firewall/:id", handlers.DeleteFirewallRule)

		// 批量操作
		authGroup.POST("/batch/create", handlers.BatchCreate)
		authGroup.POST("/batch/action", handlers.BatchAction)

		// 安全
		authGroup.GET("/security/alerts", handlers.ListSecurityAlerts)
		authGroup.GET("/security/summary", handlers.GetSecuritySummary)

		// 审计日志
		authGroup.GET("/audit-logs", handlers.ListAuditLogs)

		// 任务队列
		authGroup.GET("/tasks", handlers.ListTasks)
		authGroup.GET("/tasks/:id", handlers.GetTask)
		authGroup.GET("/tasks/:id/logs", handlers.GetTaskLogs)

		// 仪表盘
		authGroup.GET("/dashboard", handlers.GetDashboard)

		// 站点配置
		authGroup.GET("/settings/site", handlers.GetSiteConfig)
		authGroup.PUT("/settings/site", handlers.UpdateSiteConfig)

		// 控制台
		authGroup.GET("/console/ssh", console.HandleWebSSH(agentMgr))
		authGroup.GET("/console/vnc", console.HandleWebVNC(agentMgr))
	}

	// 404 处理
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
	})

	zap.L().Info("路由配置完成")
	return r
}
