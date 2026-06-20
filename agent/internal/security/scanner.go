package security

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"tsukiyo/agent/internal/config"
	"tsukiyo/agent/internal/network"
	"tsukiyo/agent/internal/ws"
)

// Scanner 安全扫描器
type Scanner struct {
	cfg        *config.Config
	netMgr     *network.Manager
	wsClient   *ws.Client
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.RWMutex
	alerts     []SecurityAlert
	history    map[string]TrafficRecord
	lastScan   time.Time
}

// SecurityAlert 安全告警
type SecurityAlert struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	Severity   string    `json:"severity"`
	InstanceID string    `json:"instance_id,omitempty"`
	Token      string    `json:"token"`
	SourceIP   string    `json:"source_ip,omitempty"`
	TargetIP   string    `json:"target_ip,omitempty"`
	Details    string    `json:"details"`
	Timestamp  time.Time `json:"timestamp"`
	Resolved   bool      `json:"resolved"`
	Blocked    bool      `json:"blocked"`
}

// TrafficRecord 流量记录
type TrafficRecord struct {
	IP           string
	ConnCount    int
	BytesSent    int64
	BytesRecv    int64
	LastSeen     time.Time
	PortsScanned map[int]bool
	SMTPAttempts int
	LastPorts    string
}

// NewScanner 创建安全扫描器
func NewScanner(cfg *config.Config, netMgr *network.Manager, wsClient *ws.Client) *Scanner {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scanner{
		cfg:      cfg,
		netMgr:   netMgr,
		wsClient: wsClient,
		ctx:      ctx,
		cancel:   cancel,
		alerts:   make([]SecurityAlert, 0),
		history:  make(map[string]TrafficRecord),
	}
}

// Start 启动安全扫描
func (s *Scanner) Start() {
	if !s.cfg.EnableSecurityScan() {
		zap.L().Info("安全扫描已禁用")
		return
	}
	zap.L().Info("安全扫描器启动", zap.Duration("interval", s.cfg.ScanInterval()))
	go s.loop()
}

// Stop 停止扫描器
func (s *Scanner) Stop() {
	s.cancel()
}

// loop 主扫描循环
func (s *Scanner) loop() {
	ticker := time.NewTicker(s.cfg.ScanInterval())
	defer ticker.Stop()

	// 首次立即扫描
	s.runFullScan()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.runFullScan()
		}
	}
}

// runFullScan 执行完整扫描
func (s *Scanner) runFullScan() {
	zap.L().Debug("执行安全扫描")

	// 1. 扫描异常流量
	s.scanAbnormalTraffic()

	// 2. 扫描端口扫描行为
	s.scanPortScanning()

	// 3. 扫描 SMTP 滥用
	s.scanSMTPAbuse()

	// 4. 扫描挖矿行为
	s.scanMining()

	// 5. 扫描暴力破解
	s.scanBruteForce()

	// 6. 自动封锁策略
	s.applyAutoBlock()

	s.lastScan = time.Now()
}

// scanAbnormalTraffic 扫描异常流量 (流量突增 >5倍平均值)
func (s *Scanner) scanAbnormalTraffic() {
	// 使用 conntrack 获取连接统计
	records := s.getConnectionStats()

	for ip, record := range records {
		hist, exists := s.history[ip]
		if !exists {
			s.history[ip] = record
			continue
		}

		// 检测流量突增：发送流量 > 5倍历史平均
		if hist.BytesSent > 0 {
			avgSent := hist.BytesSent
			if record.BytesSent > avgSent*5 && record.BytesSent > 100*1024*1024 {
				// > 500MB 突增
				alert := s.createAlert("abnormal_traffic", "warning", "",
					fmt.Sprintf("IP %s 检测到异常流量突增: 发送 %s (历史平均 %s)",
						ip, formatBytes(record.BytesSent), formatBytes(avgSent)))
				alert.SourceIP = ip
				s.addAlert(alert)
			}
		}

		// 检测大量连接 (>1000 连接数)
		if record.ConnCount > 1000 {
			alert := s.createAlert("abnormal_traffic", "warning", "",
				fmt.Sprintf("IP %s 存在大量连接: %d 个", ip, record.ConnCount))
			alert.SourceIP = ip
			s.addAlert(alert)
		}

		// 更新历史
		s.history[ip] = record
	}
}

// scanPortScanning 扫描端口扫描行为 (短时间内访问大量不同端口)
func (s *Scanner) scanPortScanning() {
	// 使用 conntrack 获取端口访问记录
	cmd := exec.Command("conntrack", "-L", "-p", "tcp")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return
	}

	portAccess := make(map[string]map[int]bool) // ip -> ports
	re := regexp.MustCompile(`dst=(\S+).*?dport=(\d+)`)
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		matches := re.FindStringSubmatch(line)
		if len(matches) >= 3 {
			ip := matches[1]
			port, _ := strconv.Atoi(matches[2])
			if port > 0 {
				if portAccess[ip] == nil {
					portAccess[ip] = make(map[int]bool)
				}
				portAccess[ip][port] = true
			}
		}
	}

	// 检测扫描行为：访问 >50 个不同端口
	for ip, ports := range portAccess {
		if len(ports) > 50 {
			alert := s.createAlert("port_scan", "critical", "",
				fmt.Sprintf("IP %s 疑似端口扫描: 访问了 %d 个不同端口", ip, len(ports)))
			alert.SourceIP = ip
			s.addAlert(alert)
		}
	}
}

// scanSMTPAbuse 扫描 SMTP 滥用 (25, 587, 465 端口的异常连接)
func (s *Scanner) scanSMTPAbuse() {
	smtpPorts := []int{25, 587, 465}
	for _, port := range smtpPorts {
		cmd := exec.Command("conntrack", "-L", "-p", "tcp", "--dport", strconv.Itoa(port))
		out, err := cmd.CombinedOutput()
		if err != nil {
			continue
		}

		// 统计每个源 IP 的 SMTP 连接数
		ipCounts := make(map[string]int)
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if idx := strings.Index(line, "src="); idx >= 0 {
				parts := strings.Fields(line[idx:])
				if len(parts) > 0 {
					src := strings.TrimPrefix(parts[0], "src=")
					if src != "" && src != "127.0.0.1" {
						ipCounts[src]++
					}
				}
			}
		}

		// 检测滥用：单个 IP > 100 个 SMTP 连接
		for ip, count := range ipCounts {
			if count > 100 {
				alert := s.createAlert("smtp_abuse", "critical", "",
					fmt.Sprintf("IP %s 疑似 SMTP 滥用: %d 个连接到端口 %d", ip, count, port))
				alert.SourceIP = ip
				s.addAlert(alert)
			}
		}
	}
}

// scanMining 扫描挖矿行为 (检查常见矿池连接)
func (s *Scanner) scanMining() {
	// 常见矿池端口和域名模式
	miningPorts := []int{3333, 4444, 5555, 6666, 7777, 8888, 9999, 45700, 45560}
	miningPatterns := []string{"pool", "mine", "xmr", "eth", "btc", "stratum"}

	for _, port := range miningPorts {
		cmd := exec.Command("conntrack", "-L", "-p", "tcp", "--dport", strconv.Itoa(port))
		out, err := cmd.CombinedOutput()
		if err != nil {
			continue
		}

		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			lower := strings.ToLower(line)
			for _, pattern := range miningPatterns {
				if strings.Contains(lower, pattern) {
					// 提取源 IP
					if idx := strings.Index(line, "src="); idx >= 0 {
						parts := strings.Fields(line[idx:])
						if len(parts) > 0 {
							src := strings.TrimPrefix(parts[0], "src=")
							alert := s.createAlert("mining", "critical", "",
								fmt.Sprintf("IP %s 疑似挖矿行为: 连接到端口 %d (%s)", src, port, pattern))
							alert.SourceIP = src
							s.addAlert(alert)
						}
					}
				}
			}
		}
	}
}

// scanBruteForce 扫描暴力破解 (SSH/RDP 登录失败)
func (s *Scanner) scanBruteForce() {
	// 读取 auth.log 或 journalctl
	cmd := exec.Command("journalctl", "-u", "sshd", "--since", "5 minutes ago", "--no-pager", "-q")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return
	}

	// 统计失败登录
	failRegex := regexp.MustCompile(`Failed password for .* from (\S+)`)
	ipFailures := make(map[string]int)

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		matches := failRegex.FindStringSubmatch(scanner.Text())
		if len(matches) >= 2 {
			ip := matches[1]
			ipFailures[ip]++
		}
	}

	// 检测暴力破解：>10 次失败
	for ip, count := range ipFailures {
		if count > 10 {
			alert := s.createAlert("brute_force", "critical", "",
				fmt.Sprintf("IP %s SSH 暴力破解: %d 次失败登录", ip, count))
			alert.SourceIP = ip
			s.addAlert(alert)
		}
	}
}

// applyAutoBlock 自动封锁策略
func (s *Scanner) applyAutoBlock() {
	s.mu.RLock()
	alerts := make([]SecurityAlert, len(s.alerts))
	copy(alerts, s.alerts)
	s.mu.RUnlock()

	for _, alert := range alerts {
		if alert.Resolved || alert.Blocked {
			continue
		}

		shouldBlock := false
		switch alert.Type {
		case "brute_force":
			if alert.Severity == "critical" {
				shouldBlock = true
			}
		case "smtp_abuse", "mining", "port_scan":
			shouldBlock = true
		case "abnormal_traffic":
			// 只有严重级别的异常流量才封锁
			if alert.Severity == "critical" {
				shouldBlock = true
			}
		}

		if shouldBlock && alert.SourceIP != "" {
			if err := s.netMgr.BlockIP(alert.SourceIP); err != nil {
				zap.L().Error("自动封锁失败", zap.String("ip", alert.SourceIP), zap.Error(err))
			} else {
				s.mu.Lock()
				for i := range s.alerts {
					if s.alerts[i].ID == alert.ID {
						s.alerts[i].Blocked = true
						s.alerts[i].Resolved = true
						break
					}
				}
				s.mu.Unlock()
				zap.L().Warn("已自动封锁 IP", zap.String("ip", alert.SourceIP), zap.String("reason", alert.Type))
			}
		}
	}
}

// createAlert 创建告警
func (s *Scanner) createAlert(alertType, severity, instanceID, details string) SecurityAlert {
	return SecurityAlert{
		ID:         fmt.Sprintf("alert_%d", time.Now().UnixNano()),
		Type:       alertType,
		Severity:   severity,
		InstanceID: instanceID,
		Token:      s.cfg.Token,
		Details:    details,
		Timestamp:  time.Now(),
		Resolved:   false,
		Blocked:    false,
	}
}

// addAlert 添加告警并上报 Master
func (s *Scanner) addAlert(alert SecurityAlert) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, existing := range s.alerts {
		if existing.Type == alert.Type && existing.SourceIP == alert.SourceIP {
			if time.Since(existing.Timestamp) < time.Hour {
				return
			}
		}
	}

	s.alerts = append(s.alerts, alert)
	if len(s.alerts) > 1000 {
		s.alerts = s.alerts[len(s.alerts)-1000:]
	}

	zap.L().Warn("安全告警",
		zap.String("type", alert.Type),
		zap.String("severity", alert.Severity),
		zap.String("source_ip", alert.SourceIP),
		zap.String("details", alert.Details))

	if s.wsClient != nil {
		payload := ws.SecurityAlertPayload{
			InstanceID: alert.InstanceID,
			AlertType:  alert.Type,
			Severity:   alert.Severity,
			SourceIP:   alert.SourceIP,
			Details:    alert.Details,
			DetectedAt: alert.Timestamp.Unix(),
		}
		if err := s.wsClient.SendSecurityAlert(payload); err != nil {
			zap.L().Error("上报安全告警到 Master 失败",
				zap.String("alert_id", alert.ID),
				zap.Error(err))
		}
	}
}

// GetAlerts 获取告警列表
func (s *Scanner) GetAlerts() []SecurityAlert {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]SecurityAlert, len(s.alerts))
	copy(result, s.alerts)
	return result
}

// getConnectionStats 获取连接统计
func (s *Scanner) getConnectionStats() map[string]TrafficRecord {
	records := make(map[string]TrafficRecord)

	cmd := exec.Command("conntrack", "-L")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return records
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		// 解析 conntrack 输出
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		var src, dst string
		var bytesSent, bytesRecv int64
		for _, f := range fields {
			if strings.HasPrefix(f, "src=") {
				src = strings.TrimPrefix(f, "src=")
			}
			if strings.HasPrefix(f, "dst=") {
				dst = strings.TrimPrefix(f, "dst=")
			}
			if strings.HasPrefix(f, "bytes=") {
				vals := strings.Split(strings.TrimPrefix(f, "bytes="), "/")
				if len(vals) == 2 {
					bytesRecv, _ = strconv.ParseInt(vals[0], 10, 64)
					bytesSent, _ = strconv.ParseInt(vals[1], 10, 64)
				}
			}
		}

		if src != "" {
			r := records[src]
			r.IP = src
			r.ConnCount++
			r.BytesSent += bytesSent
			r.BytesRecv += bytesRecv
			r.LastSeen = time.Now()
			records[src] = r
		}
		if dst != "" {
			r := records[dst]
			r.IP = dst
			r.ConnCount++
			r.LastSeen = time.Now()
			records[dst] = r
		}
	}

	return records
}

// formatBytes 格式化字节
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// ToJSON 告警序列化
func (s *Scanner) ToJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(s.alerts)
}
