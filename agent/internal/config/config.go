package config

import (
	"fmt"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// Config Agent 配置，master 和 token 从配置文件读取，其余由 Master 动态下发
type Config struct {
	Master string `mapstructure:"master"`
	Token  string `mapstructure:"token"`

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
	defaultStoragePool string
	storagePoolType    string
	storagePoolSource  string
}

// Load 加载配置
func Load(path string) (*Config, error) {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	if cfg.Master == "" {
		return nil, fmt.Errorf("master 不能为空")
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("token 不能为空")
	}

	return &cfg, nil
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
	if v, ok := data["default_storage_pool"].(string); ok && v != "" {
		c.defaultStoragePool = v
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

// DefaultStoragePool 返回默认存储池名称
func (c *Config) DefaultStoragePool() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.defaultStoragePool
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
