package db

import (
	"embed"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	migratePostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"go.uber.org/zap"

	"tsukiyo/master/internal/config"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// RunMigrations 执行数据库迁移（使用 golang-migrate）
// 自动获取 advisory lock 防止并发执行，完成后释放
func RunMigrations(cfg *config.DatabaseConfig) error {
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("获取底层 SQL 连接失败: %w", err)
	}

	driver, err := migratePostgres.WithInstance(sqlDB, &migratePostgres.Config{
		MigrationsTable: "schema_migrations",
	})
	if err != nil {
		return fmt.Errorf("创建 migrate 数据库驱动失败: %w", err)
	}

	subFS, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("读取嵌入迁移目录失败: %w", err)
	}

	source, err := iofs.New(subFS, ".")
	if err != nil {
		return fmt.Errorf("创建 migrate 源驱动失败: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return fmt.Errorf("初始化 migrate 实例失败: %w", err)
	}

	version, dirty, _ := m.Version()
	zap.L().Info("当前数据库迁移版本", zap.Uint("version", version), zap.Bool("dirty", dirty))

	if dirty {
		// force 到 version-1，这样 m.Up() 会重新执行失败的迁移
		forceVersion := int(version) - 1
		if forceVersion < 0 {
			forceVersion = 0
		}
		zap.L().Warn("数据库迁移状态为 dirty，尝试强制修复", zap.Uint("version", version), zap.Int("force_to", forceVersion))
		if err := m.Force(forceVersion); err != nil {
			return fmt.Errorf("修复 dirty 状态失败: %w", err)
		}
	}

	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			zap.L().Info("数据库 schema 已是最新版本")
			return nil
		}
		return fmt.Errorf("执行数据库迁移失败: %w", err)
	}

	newVersion, _, _ := m.Version()
	zap.L().Info("数据库迁移完成", zap.Uint("new_version", newVersion))
	return nil
}
