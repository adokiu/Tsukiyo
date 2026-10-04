package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"tsukiyo/master/internal/agent"
	"tsukiyo/master/internal/api/handlers"
	"tsukiyo/master/internal/api/middleware"
	"tsukiyo/master/internal/service/commerce"
	fin "tsukiyo/master/internal/service/finance"
	infra "tsukiyo/master/internal/service/infrastructure"
	inst "tsukiyo/master/internal/service/instance"
	mailSvc "tsukiyo/master/internal/service/mail"
	_ "tsukiyo/master/internal/service/payment/balance" // 余额支付驱动自注册
	_ "tsukiyo/master/internal/service/payment/epay"    // 易支付驱动自注册
	_ "tsukiyo/master/internal/service/payment/manual"  // 人工入账驱动自注册
	sup "tsukiyo/master/internal/service/support"
	sys "tsukiyo/master/internal/service/system"
	themeSvc "tsukiyo/master/internal/service/theme"
	usr "tsukiyo/master/internal/service/user"
)

// SetupRouter 配置路由
func SetupRouter(agentMgr *agent.Manager) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// 全局中间件
	r.Use(gin.Recovery())
	r.Use(middleware.SecurityHeadersMiddleware())
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
	// 用户前台 WebSocket 端点（实例实时指标，token 鉴权）
	r.GET("/ws/user/metrics", agentMgr.HandleUserMetricsWebSocket)
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
	agentMgr.OnDynamicBindingCheck = networkService.CheckDynamicBindingPools
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
	mailService := mailSvc.NewMailService()
	handlers.InitMailService(mailService)
	auditService := sys.NewAuditService()
	handlers.InitAuditService(auditService)
	themeService := themeSvc.NewThemeService()
	handlers.InitThemeService(themeService)
	financeService := fin.NewFinanceService()
	handlers.InitFinanceService(financeService)
	commerceService := commerce.NewCommerceService()
	handlers.InitCommerceService(commerceService)
	commerce.SetInstanceService(instanceService)
	supportService := sup.NewSupportService()
	handlers.InitSupportService(supportService)

	// 注入订单支付成功回调：外部支付渠道回调成功后触发订单履约
	financeService.SetOrderPaidCallback(func(orderID uuid.UUID, userID uint, username string) {
		commerceService.FulfillOrder(orderID, userID, username)
	})

	// API v1
	v1 := r.Group("/api/v1")

	// 公开接口 (无需认证)
	v1.GET("/init/status", handlers.GetInitStatus)
	v1.POST("/init/setup",
		middleware.RateLimitMiddleware("init_setup", 3, time.Hour, true),
		handlers.InitSetup,
	)
	v1.POST("/auth/login",
		middleware.RateLimitMiddleware("auth_login", 20, time.Minute, true),
		handlers.Login,
	)
	v1.POST("/auth/register",
		middleware.RateLimitMiddleware("auth_register", 10, time.Minute, true),
		handlers.Register,
	)
	v1.POST("/auth/register/send-code",
		middleware.RateLimitMiddleware("auth_register_code", 5, 10*time.Minute, true),
		handlers.SendRegisterCode,
	)

	// 公开站点信息 (无需认证，供用户前台初始化)
	v1.GET("/public/info", handlers.GetPublicInfo)

	// 公开商品接口 (无需认证，未登录用户可浏览商品)
	v1.GET("/public/products", handlers.GetPublicProducts)
	v1.GET("/public/products/:id", handlers.GetPublicProduct)
	v1.GET("/public/products/:id/images", handlers.ListProductNodeImages)
	v1.GET("/public/products/:id/bridges", handlers.ListProductNodeBridges)
	v1.GET("/public/product-categories", handlers.GetPublicProductCategories)

	// 管理员安全入口 (通过随机路径访问管理后台登录页)
	// 路由格式: /{entry_path} -> 前端管理员登录页
	// API: POST /api/v1/auth/admin/login -> 管理员登录
	r.GET("/:entry_path", func(c *gin.Context) {
		entryPath := c.Param("entry_path")
		// 如果路径匹配 API 前缀，跳过
		if len(entryPath) > 3 && (entryPath == "api" || entryPath == "ws" || entryPath == "themes" || entryPath == "console" || entryPath == "vnc" || entryPath == "health") {
			c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
			return
		}
		// 验证 entry_path 是否匹配
		handlers.ValidateAdminEntryPath()(c)
		if c.IsAborted() {
			return
		}
		// 返回管理后台登录页 HTML
		// 实际由静态文件服务处理，这里仅做路径验证
		c.JSON(http.StatusOK, gin.H{"entry_path": entryPath, "valid": true})
	})

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

		// 财务管理
		authGroup.GET("/finance/payment-drivers", handlers.GetPaymentDrivers)
		authGroup.GET("/finance/overview", handlers.GetFinanceOverview)
		authGroup.GET("/finance/bills", handlers.ListBills)
		authGroup.GET("/finance/bills/:id", handlers.GetBill)
		authGroup.GET("/finance/payment-channels", handlers.ListPaymentChannels)
		authGroup.POST("/finance/payment-channels", handlers.CreatePaymentChannel)
		authGroup.PUT("/finance/payment-channels/:id", handlers.UpdatePaymentChannel)
		authGroup.DELETE("/finance/payment-channels/:id", handlers.DeletePaymentChannel)

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

		// SMTP推送配置
		authGroup.GET("/settings/smtp", handlers.GetSMTPConfig)
		authGroup.PUT("/settings/smtp", handlers.UpdateSMTPConfig)
		authGroup.POST("/settings/smtp/test", handlers.TestSMTP)

		// 控制台（前端通过 GetInstanceConsole 获取直连 Agent 的 URL 和 Token）
		// 不再通过 Master 代理 WebSocket，减少带宽开销

		// 控制台凭据（通过 token 换取实例密码）
		authGroup.GET("/console/credentials", handlers.GetConsoleCredentials)

		// 主题管理
		authGroup.PUT("/theme/upload", handlers.UploadTheme)
		authGroup.GET("/theme/list", handlers.ListThemes)
		authGroup.GET("/theme/set", handlers.SetTheme)
		authGroup.POST("/theme/delete", handlers.DeleteTheme)
		authGroup.POST("/theme/import", handlers.ImportTheme)
		authGroup.POST("/theme/settings", handlers.UpdateThemeSettings)
		authGroup.GET("/theme/settings", handlers.GetThemeSettings)

		// 安全入口管理
		authGroup.GET("/admin/entry-path", handlers.GetAdminEntryPath)
		authGroup.POST("/admin/entry-path/regenerate", handlers.RegenerateAdminEntryPath)

		// 商品分类管理
		authGroup.GET("/commerce/categories", handlers.ListProductCategories)
		authGroup.POST("/commerce/categories", handlers.CreateProductCategory)
		authGroup.PUT("/commerce/categories/:id", handlers.UpdateProductCategory)
		authGroup.DELETE("/commerce/categories/:id", handlers.DeleteProductCategory)

		// 商品管理
		authGroup.GET("/commerce/products", handlers.ListProducts)
		authGroup.POST("/commerce/products", handlers.CreateProduct)
		authGroup.GET("/commerce/products/:id", handlers.GetProduct)
		authGroup.PUT("/commerce/products/:id", handlers.UpdateProduct)
		authGroup.DELETE("/commerce/products/:id", handlers.DeleteProduct)

		// 工单管理
		authGroup.GET("/tickets", handlers.ListTickets)
		authGroup.GET("/tickets/:id", handlers.GetTicket)
		authGroup.POST("/tickets/:id/reply", handlers.StaffReplyTicket)
		authGroup.PUT("/tickets/:id/status", handlers.UpdateTicketStatus)
		authGroup.PUT("/tickets/:id/assign", handlers.AssignTicket)
		authGroup.PUT("/tickets/:id/priority", handlers.UpdateTicketPriority)

		// 用户前台接口（需要认证，但不需要管理员权限）
		userGroup := v1.Group("")
		userGroup.Use(middleware.AuthMiddleware())
		{
			userGroup.GET("/auth/me", handlers.GetCurrentUser)
			userGroup.GET("/user/dashboard", handlers.GetUserDashboard)
			userGroup.GET("/user/instances", handlers.ListUserInstances)
			userGroup.GET("/user/instances/metrics", handlers.BatchUserInstanceMetrics)
			userGroup.GET("/user/instances/:id", handlers.GetUserInstance)
			userGroup.POST("/user/instances/:id/start", handlers.StartUserInstance)
			userGroup.POST("/user/instances/:id/stop", handlers.StopUserInstance)
			userGroup.POST("/user/instances/:id/restart", handlers.RestartUserInstance)
			userGroup.DELETE("/user/instances/:id", handlers.DeleteUserInstance)
			userGroup.GET("/user/instances/:id/console", handlers.GetUserInstanceConsole)
			userGroup.GET("/user/instances/:id/metrics", handlers.GetUserInstanceMetrics)
			userGroup.GET("/user/instances/:id/metrics/history", handlers.GetUserInstanceMetricsHistory)
			userGroup.POST("/user/instances/:id/reset-password", handlers.ResetUserInstancePassword)
			userGroup.POST("/user/instances/:id/reinstall", handlers.ReinstallUserInstance)
			userGroup.GET("/user/instances/:id/snapshots", handlers.ListUserSnapshots)
			userGroup.POST("/user/instances/:id/snapshots", handlers.CreateUserSnapshot)
			userGroup.POST("/user/instances/:id/snapshots/:name/restore", handlers.RestoreUserSnapshot)
			userGroup.DELETE("/user/instances/:id/snapshots/:name", handlers.DeleteUserSnapshot)

			// 用户充值
			userGroup.GET("/payment/channels", handlers.ListUserPaymentChannels)
			userGroup.POST("/payment/recharge", handlers.CreateRecharge)
			userGroup.GET("/payment/recharge/:bill_no/status", handlers.GetRechargeStatus)

			// 用户端商品下单（需要认证）
			userGroup.POST("/user/products/:id/order", handlers.UserCreateOrder)
			userGroup.GET("/user/products/:id/images", handlers.ListProductNodeImages)
			userGroup.GET("/user/products/:id/bridges", handlers.ListProductNodeBridges)

			// 购物车
			userGroup.GET("/user/cart", handlers.ListCart)
			userGroup.POST("/user/cart", handlers.AddToCart)
			userGroup.PUT("/user/cart/:id", handlers.UpdateCart)
			userGroup.DELETE("/user/cart/:id", handlers.RemoveFromCart)
			userGroup.DELETE("/user/cart", handlers.ClearCart)

			// 优惠码
			userGroup.POST("/user/coupon/validate", handlers.ValidateCoupon)

			// 结算与订单
			userGroup.POST("/user/checkout", handlers.Checkout)
			userGroup.POST("/user/orders/:id/pay", handlers.PayOrder)
			userGroup.GET("/user/orders/:id", handlers.GetOrder)
			userGroup.GET("/user/orders", handlers.ListOrders)

			// 账单与流水
			userGroup.GET("/user/bills", handlers.ListUserBills)
			userGroup.GET("/user/bills/:id", handlers.GetUserBill)
			userGroup.GET("/user/wallet/transactions", handlers.ListUserWalletTransactions)

			// 发票
			userGroup.GET("/user/orders/:id/invoice", handlers.GetInvoice)

			// 工单
			userGroup.GET("/user/tickets", handlers.ListUserTickets)
			userGroup.POST("/user/tickets", handlers.CreateUserTicket)
			userGroup.GET("/user/tickets/:id", handlers.GetUserTicket)
			userGroup.POST("/user/tickets/:id/reply", handlers.ReplyUserTicket)
			userGroup.POST("/user/tickets/:id/close", handlers.CloseUserTicket)
		}

	}

	// 支付回调 (无需认证，独立限流)
	v1.Any("/payment/notify/:channel_id",
		middleware.RateLimitMiddleware("payment_notify", 120, time.Minute, false),
		handlers.PaymentNotify,
	)

	// 主题静态文件路由和 SPA 回退由 SetupThemeStaticRoutes 处理
	// 在 main.go 中调用 SetupThemeStaticRoutes 注册

	zap.L().Info("路由配置完成")
	return r
}
