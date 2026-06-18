package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Config 主控配置
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Log      LogConfig      `mapstructure:"log"`
	Agent    AgentConfig    `mapstructure:"agent"`
}

// ServerConfig 服务配置
type ServerConfig struct {
	Host          string        `mapstructure:"host"`
	Port          int           `mapstructure:"port"`
	ReadTimeout   time.Duration `mapstructure:"read_timeout"`
	WriteTimeout  time.Duration `mapstructure:"write_timeout"`
	MaxHeaderSize int           `mapstructure:"max_header_size"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
	MaxConns int    `mapstructure:"max_conns"`
	MinConns int    `mapstructure:"min_conns"`
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
	MaxRetries   int           `mapstructure:"max_retries"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// JWTConfig JWT 配置
type JWTConfig struct {
	Secret    string        `mapstructure:"secret"`
	ExpiresIn time.Duration `mapstructure:"expires_in"`
	Issuer    string        `mapstructure:"issuer"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	OutputPath string `mapstructure:"output_path"`
}

// AgentConfig Agent 相关配置
type AgentConfig struct {
	HeartbeatInterval    time.Duration `mapstructure:"heartbeat_interval"`
	HeartbeatTimeout     time.Duration `mapstructure:"heartbeat_timeout"`
	TaskTimeout          time.Duration `mapstructure:"task_timeout"`
	MetricsRetentionDays int           `mapstructure:"metrics_retention_days"`
}

var AppConfig *Config

// Init 加载配置
func Init() error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath("/etc/tsukiyo/master")

	setDefaults()

	viper.AutomaticEnv()
	viper.SetEnvPrefix("TSUKIYO")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("读取配置文件失败: %w", err)
		}
		zap.L().Warn("未找到配置文件，使用默认配置")
	}

	AppConfig = &Config{}
	if err := viper.Unmarshal(AppConfig); err != nil {
		return fmt.Errorf("解析配置失败: %w", err)
	}

	if AppConfig.JWT.Secret == "" {
		AppConfig.JWT.Secret = generateRandomSecret(32)
		zap.L().Warn("JWT Secret 未配置，已自动生成随机密钥")
	}

	zap.L().Info("配置加载完成",
		zap.Int("server_port", AppConfig.Server.Port),
		zap.String("database_host", AppConfig.Database.Host),
		zap.String("redis_host", AppConfig.Redis.Host),
	)

	return nil
}

func setDefaults() {
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.read_timeout", "30s")
	viper.SetDefault("server.write_timeout", "30s")
	viper.SetDefault("server.max_header_size", 1048576)

	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.user", "tsukiyo")
	viper.SetDefault("database.dbname", "tsukiyo")
	viper.SetDefault("database.sslmode", "disable")
	viper.SetDefault("database.max_conns", 100)
	viper.SetDefault("database.min_conns", 10)

	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 100)
	viper.SetDefault("redis.min_idle_conns", 10)
	viper.SetDefault("redis.max_retries", 3)
	viper.SetDefault("redis.dial_timeout", "5s")
	viper.SetDefault("redis.read_timeout", "3s")
	viper.SetDefault("redis.write_timeout", "3s")

	viper.SetDefault("jwt.expires_in", "24h")
	viper.SetDefault("jwt.issuer", "tsukiyo")

	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "json")
	viper.SetDefault("log.output_path", "")

	viper.SetDefault("agent.heartbeat_interval", "5s")
	viper.SetDefault("agent.heartbeat_timeout", "60s")
	viper.SetDefault("agent.task_timeout", "600s")
	viper.SetDefault("agent.metrics_retention_days", 30)
}

func (d *DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode)
}

func (d *DatabaseConfig) URL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.DBName, d.SSLMode)
}

func generateRandomSecret(length int) string {
	charset := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[i%len(charset)]
	}
	return string(b)
}
