package db

import (
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"tsukiyo/master/internal/config"
	"tsukiyo/master/internal/models"
)

var DB *gorm.DB

// Init 初始化数据库连接
func Init(cfg *config.DatabaseConfig) error {
	gormLogger := logger.New(
		zap.NewStdLog(zap.L()),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Error,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger:                                   gormLogger,
		CreateBatchSize:                          100,
		PrepareStmt:                              true,
		SkipDefaultTransaction:                   false,
		FullSaveAssociations:                     false,
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("获取底层 SQL 连接失败: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxConns)
	sqlDB.SetMaxIdleConns(cfg.MinConns)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	DB = db

	zap.L().Info("数据库连接成功",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("dbname", cfg.DBName),
	)

	return nil
}

// AutoMigrate 自动迁移数据库表结构
func AutoMigrate() error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}

	// 清理残留的外键约束，防止 many2many 标签遗留的 FK 阻塞后续操作
	dropStaleForeignKeys()

	// 若 users 表主键类型和当前模型不一致，重建整个 schema
	if err := recreateSchemaIfNeeded(); err != nil {
		return err
	}

	modelsList := []interface{}{
		&models.User{},
		&models.UserGroup{},
		&models.UserGroupMember{},
		&models.Permission{},
		&models.GroupPermission{},
		&models.Node{},
		&models.Instance{},
		&models.IPv6Prefix{},
		&models.PublicIPPool{},
		&models.PortMapping{},
		&models.FirewallRule{},
		&models.NodeImage{},
		&models.AuditLog{},
		&models.Task{},
		&models.TaskLog{},
		&models.Snapshot{},
		&models.DataDisk{},
		&models.NATConfig{},
		&models.InstanceMetric{},
		&models.SiteConfig{},
		&models.VPCNetwork{},
		&models.IPPoolEntry{},
	}

	for _, m := range modelsList {
		if err := DB.AutoMigrate(m); err != nil {
			return fmt.Errorf("迁移表失败: %w", err)
		}
	}

	// 创建时序表分区 (instance_metrics 按月分区)
	if err := createMetricsPartitions(); err != nil {
		zap.L().Warn("创建监控指标分区失败", zap.Error(err))
	}

	// 初始化默认权限数据
	if err := initDefaultPermissions(); err != nil {
		return fmt.Errorf("初始化权限数据失败: %w", err)
	}

	// 初始化默认用户组
	if err := initDefaultGroups(); err != nil {
		return fmt.Errorf("初始化用户组失败: %w", err)
	}

	// 初始化站点配置
	if err := initDefaultSiteConfig(); err != nil {
		return fmt.Errorf("初始化站点配置失败: %w", err)
	}

	zap.L().Info("数据库迁移完成")
	return nil
}

// dropStaleForeignKeys 清理 GORM many2many 自动生成的残留外键约束
func dropStaleForeignKeys() {
	staleConstraints := []struct {
		table      string
		constraint string
	}{
		{"user_group_members", "fk_user_group_members_user"},
		{"user_group_members", "fk_user_group_members_user_group"},
		{"group_permissions", "fk_group_permissions_user_group"},
		{"group_permissions", "fk_group_permissions_permission"},
	}
	for _, c := range staleConstraints {
		sql := fmt.Sprintf(
			"ALTER TABLE IF EXISTS %s DROP CONSTRAINT IF EXISTS %s",
			c.table, c.constraint,
		)
		DB.Exec(sql)
	}
}

// recreateSchemaIfNeeded 检测 schema 是否与当前模型兼容，不兼容时重建
func recreateSchemaIfNeeded() error {
	var colType string
	err := DB.Raw(`
		SELECT data_type FROM information_schema.columns
		WHERE table_name = 'users' AND column_name = 'id'
	`).Scan(&colType).Error
	if err != nil || colType == "" {
		// 表不存在，无需处理
		return nil
	}
	if colType == "bigint" || colType == "integer" {
		return nil
	}
	// 类型不匹配，重建 schema
	zap.L().Warn("检测到 users 表主键类型不兼容，重建 schema", zap.String("current_type", colType))
	if err := DB.Exec("DROP SCHEMA public CASCADE").Error; err != nil {
		return fmt.Errorf("删除 schema 失败: %w", err)
	}
	if err := DB.Exec("CREATE SCHEMA public").Error; err != nil {
		return fmt.Errorf("创建 schema 失败: %w", err)
	}
	if err := DB.Exec("GRANT ALL ON SCHEMA public TO postgres").Error; err != nil {
		return fmt.Errorf("授权 schema 失败: %w", err)
	}
	if err := DB.Exec("GRANT ALL ON SCHEMA public TO public").Error; err != nil {
		return fmt.Errorf("授权 schema 失败: %w", err)
	}
	return nil
}

// createMetricsPartitions 创建监控指标表分区
func createMetricsPartitions() error {
	// PostgreSQL 14+ 支持声明式分区，此处创建按月分区
	sql := `
		DO $$
		BEGIN
			-- 如果表已存在且不是分区表，先跳过
			IF EXISTS (
				SELECT 1 FROM information_schema.tables 
				WHERE table_name = 'instance_metrics' 
				AND table_schema = 'public'
			) THEN
				RETURN;
			END IF;
		END $$;
	`
	return DB.Exec(sql).Error
}

// initDefaultPermissions 初始化默认权限数据
func initDefaultPermissions() error {
	permissions := []models.Permission{
		// 实例权限
		{ID: "instance:create", Name: "创建实例", Resource: "instance", Action: "create"},
		{ID: "instance:read", Name: "查看实例", Resource: "instance", Action: "read"},
		{ID: "instance:update", Name: "更新实例", Resource: "instance", Action: "update"},
		{ID: "instance:delete", Name: "删除实例", Resource: "instance", Action: "delete"},
		{ID: "instance:start", Name: "启动实例", Resource: "instance", Action: "start"},
		{ID: "instance:stop", Name: "停止实例", Resource: "instance", Action: "stop"},
		{ID: "instance:restart", Name: "重启实例", Resource: "instance", Action: "restart"},
		{ID: "instance:console", Name: "实例控制台", Resource: "instance", Action: "console"},
		{ID: "instance:snapshot", Name: "实例快照", Resource: "instance", Action: "snapshot"},
		{ID: "instance:reinstall", Name: "重装实例", Resource: "instance", Action: "reinstall"},
		{ID: "instance:migrate", Name: "迁移实例", Resource: "instance", Action: "migrate"},
		// 用户权限
		{ID: "user:create", Name: "创建用户", Resource: "user", Action: "create"},
		{ID: "user:read", Name: "查看用户", Resource: "user", Action: "read"},
		{ID: "user:update", Name: "更新用户", Resource: "user", Action: "update"},
		{ID: "user:delete", Name: "删除用户", Resource: "user", Action: "delete"},
		{ID: "user:group_manage", Name: "管理用户组", Resource: "user", Action: "group_manage"},
		// 节点权限
		{ID: "node:create", Name: "创建节点", Resource: "node", Action: "create"},
		{ID: "node:read", Name: "查看节点", Resource: "node", Action: "read"},
		{ID: "node:update", Name: "更新节点", Resource: "node", Action: "update"},
		{ID: "node:delete", Name: "删除节点", Resource: "node", Action: "delete"},
		{ID: "node:monitor", Name: "监控节点", Resource: "node", Action: "monitor"},
		// 网络权限
		{ID: "network:manage", Name: "管理网络", Resource: "network", Action: "manage"},
		{ID: "network:ip_allocate", Name: "分配 IP", Resource: "network", Action: "ip_allocate"},
		{ID: "network:port_forward", Name: "端口转发", Resource: "network", Action: "port_forward"},
		// 镜像权限
		{ID: "image:create", Name: "创建镜像", Resource: "image", Action: "create"},
		{ID: "image:read", Name: "查看镜像", Resource: "image", Action: "read"},
		{ID: "image:update", Name: "更新镜像", Resource: "image", Action: "update"},
		{ID: "image:delete", Name: "删除镜像", Resource: "image", Action: "delete"},
		{ID: "image:download", Name: "下载镜像", Resource: "image", Action: "download"},
		// 审计权限
		{ID: "audit:read", Name: "查看审计日志", Resource: "audit", Action: "read"},
		{ID: "audit:manage", Name: "管理审计日志", Resource: "audit", Action: "manage"},
		// 系统权限
		{ID: "system:config", Name: "系统配置", Resource: "system", Action: "config"},
		{ID: "system:log", Name: "系统日志", Resource: "system", Action: "log"},
		{ID: "system:backup", Name: "系统备份", Resource: "system", Action: "backup"},
	}

	for _, perm := range permissions {
		var existing models.Permission
		if err := DB.Where("id = ?", perm.ID).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := DB.Create(&perm).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}
	return nil
}

// initDefaultGroups 初始化默认用户组
func initDefaultGroups() error {
	// admin 组
	adminGroup := models.UserGroup{
		Name:        "admin",
		Description: "系统管理员组，拥有所有权限",
		IsBuiltin:   true,
	}
	if err := DB.Where("name = ?", "admin").FirstOrCreate(&adminGroup).Error; err != nil {
		return fmt.Errorf("创建 admin 组失败: %w", err)
	}

	// 给 admin 组分配所有权限（跳过已存在的）
	var perms []models.Permission
	if err := DB.Find(&perms).Error; err != nil {
		return err
	}
	for _, perm := range perms {
		gp := models.GroupPermission{
			GroupID:      adminGroup.ID,
			PermissionID: perm.ID,
			Scope:        "all",
		}
		DB.Where("group_id = ? AND permission_id = ?", gp.GroupID, gp.PermissionID).FirstOrCreate(&gp)
	}

	// user 组
	userGroup := models.UserGroup{
		Name:        "user",
		Description: "普通用户组，只能管理自己的实例",
		IsBuiltin:   true,
	}
	if err := DB.Where("name = ?", "user").FirstOrCreate(&userGroup).Error; err != nil {
		return fmt.Errorf("创建 user 组失败: %w", err)
	}

	// 给 user 组分配实例 own 权限（跳过已存在的）
	userPerms := []string{
		"instance:read", "instance:start", "instance:stop",
		"instance:restart", "instance:console", "instance:snapshot",
		"instance:reinstall",
	}
	for _, permID := range userPerms {
		gp := models.GroupPermission{
			GroupID:      userGroup.ID,
			PermissionID: permID,
			Scope:        "own",
		}
		DB.Where("group_id = ? AND permission_id = ?", gp.GroupID, gp.PermissionID).FirstOrCreate(&gp)
	}

	return nil
}

// initDefaultSiteConfig 初始化默认站点配置
func initDefaultSiteConfig() error {
	var count int64
	if err := DB.Model(&models.SiteConfig{}).Count(&count).Error; err != nil {
		return err
	}

	if count == 0 {
		defaultConfig := models.SiteConfig{
			SiteName:       "Tsukiyo",
			IncusRemoteURL: "images:",
		}
		if err := DB.Create(&defaultConfig).Error; err != nil {
			return err
		}
	} else {
		// 更新现有记录的 IncusRemoteURL（如果为空）
		var site models.SiteConfig
		if err := DB.First(&site).Error; err != nil {
			return err
		}
		if site.IncusRemoteURL == "" {
			site.IncusRemoteURL = "images:"
			if err := DB.Save(&site).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
