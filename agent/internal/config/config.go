package config

import (
	"fmt"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// Config Agent 配置
type Config struct {
	Master string    `mapstructure:"master"`
	Token  string    `mapstructure:"token"`
	Log    LogConfig `mapstructure:"log"`

	mu                 sync.RWMutex
	incusSocketPath    string
	metricsInterval    time.Duration
	heartbeatInterval  time.Duration
	networkInterface   string
	enableNAT          bool
	enableFirewall     bool
	enableSecurityScan bool
	scanInterval       time.Duration
	consoleBindAddr    string
	agentURL           string
	imageRemoteURL     string
	storagePoolType    string
	storagePoolSource  string
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	OutputPath string `mapstructure:"output_path"`
}

var AppConfig *Config

// Init 初始化配置
func Init(configPath string) error {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	setDefaults()

	viper.AutomaticEnv()
	viper.SetEnvPrefix("TSUKIYO_AGENT")

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	AppConfig = &Config{}
	if err := viper.Unmarshal(AppConfig); err != nil {
		return fmt.Errorf("解析配置失败: %w", err)
	}

	if AppConfig.Master == "" {
		return fmt.Errorf("master 不能为空")
	}
	if AppConfig.Token == "" {
		return fmt.Errorf("token 不能为空")
	}

	return nil
}

// Load 加载配置（兼容旧接口）
func Load(path string) (*Config, error) {
	if err := Init(path); err != nil {
		return nil, err
	}
	return AppConfig, nil
}

func setDefaults() {
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "json")
	viper.SetDefault("log.output_path", "")
}

// UpdateFromMaster 由 Master 下发的配置更新
func (c *Config) UpdateFromMaster(data map[string]interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if v, ok := data["incus_socket_path"].(string); ok && v != "" {
		c.incusSocketPath = v
	}
	if v, ok := data["metrics_interval"].(float64); ok {
		c.metricsInterval = time.Duration(v) * time.Second
	}
	if v, ok := data["heartbeat_interval"].(float64); ok {
		c.heartbeatInterval = time.Duration(v) * time.Second
	}
	if v, ok := data["network_interface"].(string); ok {
		c.networkInterface = v
	}
	if v, ok := data["enable_nat"].(bool); ok {
		c.enableNAT = v
	}
	if v, ok := data["enable_firewall"].(bool); ok {
		c.enableFirewall = v
	}
	if v, ok := data["enable_security_scan"].(bool); ok {
		c.enableSecurityScan = v
	}
	if v, ok := data["scan_interval"].(float64); ok {
		c.scanInterval = time.Duration(v) * time.Second
	}
	if v, ok := data["console_bind_addr"].(string); ok && v != "" {
		c.consoleBindAddr = v
	}
	if v, ok := data["agent_url"].(string); ok && v != "" {
		c.agentURL = v
	}
	if v, ok := data["image_remote_url"].(string); ok {
		c.imageRemoteURL = v
	}
	if v, ok := data["storage_pool_type"].(string); ok && v != "" {
		c.storagePoolType = v
	}
	if v, ok := data["storage_pool_source"].(string); ok && v != "" {
		c.storagePoolSource = v
	}
}

// IsInitialized 检查是否已收到 Master 下发的配置
func (c *Config) IsInitialized() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.incusSocketPath != ""
}

// LogLevel 返回日志级别
func (c *Config) LogLevel() string {
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		return v
	}
	return "info"
}

// IncusSocketPath 返回 Incus Socket 路径
func (c *Config) IncusSocketPath() string {
	if v := os.Getenv("INCUS_SOCKET"); v != "" {
		return v
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.incusSocketPath
}

// IncusRemote 返回 Incus 远程
func (c *Config) IncusRemote() string { return "local" }

// MetricsInterval 返回监控采集间隔
func (c *Config) MetricsInterval() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.metricsInterval
}

// HeartbeatInterval 返回心跳间隔
func (c *Config) HeartbeatInterval() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.heartbeatInterval
}

// NetworkInterface 返回网络接口
func (c *Config) NetworkInterface() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.networkInterface
}

// EnableNAT 返回是否启用 NAT
func (c *Config) EnableNAT() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enableNAT
}

// EnableFirewall 返回是否启用防火墙
func (c *Config) EnableFirewall() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enableFirewall
}

// EnableSecurityScan 返回是否启用安全扫描
func (c *Config) EnableSecurityScan() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enableSecurityScan
}

// ScanInterval 返回扫描间隔
func (c *Config) ScanInterval() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.scanInterval
}

// ConsoleBindAddr 返回控制台绑定地址
func (c *Config) ConsoleBindAddr() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.consoleBindAddr
}

// AgentURL 返回 Agent 外部可访问 URL
func (c *Config) AgentURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.agentURL
}

// ImageRemoteURL 返回节点配置的镜像源 URL
func (c *Config) ImageRemoteURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.imageRemoteURL
}

// StoragePoolType 返回存储池类型
func (c *Config) StoragePoolType() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.storagePoolType
}

// StoragePoolSource 返回存储池源路径/设备
func (c *Config) StoragePoolSource() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.storagePoolSource
}

// MasterWSURL 返回 Master WebSocket 地址
func (c *Config) MasterWSURL() string {
	return c.Master + "/ws/agent"
}

// MasterAPIURL 返回 Master REST API 地址
func (c *Config) MasterAPIURL() string {
	u, err := url.Parse(c.Master)
	if err != nil {
		return ""
	}
	scheme := "http"
	if u.Scheme == "wss" {
		scheme = "https"
	} else if u.Scheme == "ws" {
		scheme = "http"
	}
	u.Scheme = scheme
	u.Path = "/api/v1"
	return u.String()
}
