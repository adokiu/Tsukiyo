package security

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"tsukiyo/master/internal/db"
)

// Alert 安全告警
type Alert struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Severity    string    `json:"severity"`
	InstanceID  string    `json:"instance_id,omitempty"`
	NodeID      string    `json:"node_id,omitempty"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
	Resolved    bool      `json:"resolved"`
}

// Scanner 安全扫描器
type Scanner struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// NewScanner 创建安全扫描器
func NewScanner() *Scanner {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scanner{ctx: ctx, cancel: cancel}
}

// Start 启动安全扫描
func (s *Scanner) Start() {
	zap.L().Info("安全扫描器启动")
	// 每5分钟扫描一次异常
	go s.runTicker(5*time.Minute, s.scanAnomalies)
}

// Stop 停止扫描器
func (s *Scanner) Stop() {
	s.cancel()
	zap.L().Info("安全扫描器停止")
}

// runTicker 定时器
func (s *Scanner) runTicker(interval time.Duration, fn func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			fn()
		}
	}
}

// scanAnomalies 扫描异常行为
func (s *Scanner) scanAnomalies() {
	// 检查异常流量
	s.checkAbnormalTraffic()

	// 检查暴力破解
	s.checkBruteForce()

	// 检查异常登录
	s.checkAbnormalLogins()
}

// checkAbnormalTraffic 检查异常流量
func (s *Scanner) checkAbnormalTraffic() {
	// 检查流量突增 (> 5倍平均值)
	ctx := context.Background()

	// 从 Redis 获取流量统计
	// 简化实现：标记可疑实例
	data, _ := db.RedisClient.Get(ctx, "security:traffic_anomaly").Result()
	if data == "" {
		return
	}

	alert := Alert{
		ID:        uuid.New().String(),
		Type:      "abnormal_traffic",
		Severity:  "warning",
		Description: "检测到异常流量",
		Timestamp: time.Now(),
	}

	s.storeAlert(alert)
}

// checkBruteForce 检查暴力破解
func (s *Scanner) checkBruteForce() {
	ctx := context.Background()

	// 检查登录失败次数
	failedLogins, _ := db.RedisClient.Get(ctx, "security:failed_logins").Int()
	if failedLogins > 10 {
		alert := Alert{
			ID:          uuid.New().String(),
			Type:        "brute_force",
			Severity:      "critical",
			Description: fmt.Sprintf("检测到暴力破解攻击，失败次数: %d", failedLogins),
			Timestamp:   time.Now(),
		}
		s.storeAlert(alert)
	}
}

// checkAbnormalLogins 检查异常登录
func (s *Scanner) checkAbnormalLogins() {
	// 检查非工作时间登录、异地登录等
	// 简化实现
}

// storeAlert 存储告警到 Redis
func (s *Scanner) storeAlert(alert Alert) {
	ctx := context.Background()
	key := "security:alerts"
	data, _ := json.Marshal(alert)
	db.RedisClient.LPush(ctx, key, data)
	db.RedisClient.LTrim(ctx, key, 0, 999) // 保留最近1000条
	db.RedisClient.Expire(ctx, key, 7*24*time.Hour)

	zap.L().Warn("安全告警",
		zap.String("type", alert.Type),
		zap.String("severity", alert.Severity),
		zap.String("desc", alert.Description))
}

// GetAlerts 获取安全告警列表
func GetAlerts(limit int64) ([]Alert, error) {
	ctx := context.Background()
	key := "security:alerts"
	items, err := db.RedisClient.LRange(ctx, key, 0, limit-1).Result()
	if err != nil {
		return nil, err
	}

	alerts := make([]Alert, 0, len(items))
	for _, item := range items {
		var alert Alert
		if err := json.Unmarshal([]byte(item), &alert); err == nil {
			alerts = append(alerts, alert)
		}
	}

	return alerts, nil
}

// GetSummary 获取安全汇总
func GetSummary() map[string]interface{} {
	ctx := context.Background()
	alertCount, _ := db.RedisClient.LLen(ctx, "security:alerts").Result()

	blockedIPs, _ := db.RedisClient.SCard(ctx, "security:blocked_ips").Result()

	return map[string]interface{}{
		"alerts":       alertCount,
		"blocked_ips":  blockedIPs,
		"scan_events":  0,
		"brute_force":  0,
		"mining_detected": 0,
	}
}
