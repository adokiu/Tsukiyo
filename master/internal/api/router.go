package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"tsukiyo/master/internal/agent"
	"tsukiyo/master/internal/api/handlers"
	"tsukiyo/master/internal/api/middleware"
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
	// 前端 WebSocket 推送端点（节点心跳等实时数据）
	r.GET("/ws/nodes", agentMgr.HandleFrontendWebSocket)
	// 前端 WebSocket 推送端点（实例状态等实时数据）
	r.GET("/ws/instances", agentMgr.HandleFrontendWebSocket)
	// 控制台 WebSocket 端点（通过 token 鉴权，Master 代理转发到 Agent）
	r.GET("/api/v1/console/ssh", agentMgr.HandleConsoleWebSocket)
	r.GET("/api/v1/console/vnc", agentMgr.HandleVNCWebSocket)

	// 初始化服务层
	imageService := infra.NewImageService(agentMgr)
	handlers.InitImageService(imageService)
	nodeService := infra.NewNodeService(agentMgr)
	handlers.InitNodeService(nodeService)
	userService := usr.NewUserService()
	handlers.InitUserService(userService)
	networkService := infra.NewNetworkService(agentMgr)
	handlers.InitNetworkService(networkService)
	instanceService := inst.NewInstanceService(networkService, agentMgr)
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
		authGroup.POST("/nodes/:id/disks/partitions", handlers.CreatePartition)
		authGroup.DELETE("/nodes/:id/disks/partitions/:device", handlers.DeletePartition)
		authGroup.GET("/nodes/:id/storages", handlers.ListNodeStorages)
		authGroup.POST("/nodes/:id/storages/init", handlers.InitNodeStorage)
		authGroup.DELETE("/nodes/:id/storages/:name", handlers.DeleteNodeStorage)
		authGroup.GET("/nodes/:id/storages/:name/volumes", handlers.ListNodeVolumes)
		authGroup.GET("/nodes/:id/storages/:name/resources", handlers.GetNodeStorageResources)
		authGroup.GET("/nodes/:id/networks", handlers.GetNodeNetworks)
		authGroup.GET("/nodes/:id/bridges", handlers.GetNodeBridges)
		authGroup.GET("/nodes/:id/tasks", handlers.GetNodeTasks)
		authGroup.GET("/nodes/:id/security-alerts", handlers.GetNodeSecurityAlerts)

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
		authGroup.GET("/instances/:id/metrics/history", handlers.GetInstanceMetricsHistory)

		// 实例数据盘操作
		authGroup.POST("/instances/:id/disks", handlers.AddInstanceDisk)
		authGroup.DELETE("/instances/:id/disks/:disk_id", handlers.DeleteInstanceDisk)
		authGroup.PUT("/instances/:id/disks/:disk_id", handlers.ResizeInstanceDisk)

		// 实例封禁/解封/续期
		authGroup.POST("/instances/:id/ban", handlers.BanInstance)
		authGroup.POST("/instances/:id/unban", handlers.UnbanInstance)
		authGroup.POST("/instances/:id/renew", handlers.RenewInstance)
		// 管理员强制修改实例状态
		authGroup.POST("/instances/:id/status", handlers.SetInstanceStatus)

		// 实例网络配置
		authGroup.POST("/instances/:id/network", handlers.UpdateInstanceNetwork)

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
		authGroup.GET("/images/source", handlers.GetImageSource)
		authGroup.PUT("/images/source", handlers.UpdateImageSource)
		authGroup.POST("/images/refresh", handlers.RefreshImageCache)
		// 已安装镜像管理（agent上报）
		authGroup.GET("/images/installed", handlers.ListInstalledImages)
		authGroup.POST("/images/sync", handlers.SyncNodeImages)
		// 镜像分类管理
		authGroup.GET("/images/categories", handlers.ListImageCategories)
		authGroup.POST("/images/categories", handlers.CreateImageCategory)
		authGroup.PUT("/images/categories/:id", handlers.UpdateImageCategory)
		authGroup.DELETE("/images/categories/:id", handlers.DeleteImageCategory)
		// 镜像别名更新（分类、显示名、install_ssh）
		authGroup.PUT("/images/alias", handlers.UpdateImageAlias)
		// 重装系统镜像列表（按分类分组）
		authGroup.GET("/images/reinstall", handlers.ListReinstallImages)

		// 网桥管理
		authGroup.GET("/network/bridges", handlers.ListBridges)
		authGroup.POST("/network/bridges", handlers.CreateBridge)
		authGroup.GET("/network/bridges/:id", handlers.GetBridge)
		authGroup.PUT("/network/bridges/:id", handlers.UpdateBridge)
		authGroup.DELETE("/network/bridges/:id", handlers.DeleteBridge)
		authGroup.POST("/network/bridges/:id/bind-egress", handlers.BindBridgeEgress)
		authGroup.POST("/network/bridges/:id/unbind-egress", handlers.UnbindBridgeEgress)

		// EIP 资源池管理
		authGroup.GET("/network/eip-pools", handlers.ListEIPPools)
		authGroup.POST("/network/eip-pools", handlers.CreateEIPPool)
		authGroup.DELETE("/network/eip-pools/:id", handlers.DeleteEIPPool)
		authGroup.PUT("/network/eip-pools/:id", handlers.UpdateEIPPool)
		// 查询节点可用 EIP 数量
		authGroup.GET("/network/eip-available", handlers.CountAvailableEIP)
		// 列出池中可用 EIP 地址
		authGroup.GET("/network/eip-available-list", handlers.ListAvailableEIPs)
		// 列出 bridge IPv6 CIDR 中可用子段
		authGroup.GET("/network/bridge-ipv6-available", handlers.ListAvailableIPv6FromBridge)

		// EIP 分配管理
		authGroup.GET("/network/eip-allocations", handlers.ListEIPAllocations)
		authGroup.POST("/network/eip-allocations/allocate", handlers.AllocateEIP)
		authGroup.POST("/network/eip-allocations/:id/assign", handlers.AssignEIPToInstance)
		authGroup.POST("/network/eip-allocations/:id/release", handlers.ReleaseEIP)

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

		// 安全告警
		authGroup.GET("/security/alerts", handlers.ListSecurityAlerts)
		authGroup.GET("/security/summary", handlers.GetSecuritySummary)
		authGroup.POST("/security/alerts/:id/resolve", handlers.ResolveSecurityAlert)
		authGroup.POST("/security/alerts/:id/ignore", handlers.IgnoreSecurityAlert)

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

		// 控制台（前端通过 GetInstanceConsole 获取直连 Agent 的 URL 和 Token）
		// 不再通过 Master 代理 WebSocket，减少带宽开销

		// 控制台凭据（通过 token 换取实例密码）
		authGroup.GET("/console/credentials", handlers.GetConsoleCredentials)
	}

	// 404 处理
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
	})

	zap.L().Info("路由配置完成")
	return r
}
