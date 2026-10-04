package service

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// MailService 邮件推送服务
type MailService struct {
	cfg *models.SMTPConfig
}

// NewMailService 创建邮件服务
func NewMailService() *MailService {
	return &MailService{}
}

// NewMailServiceWithConfig 使用指定配置创建邮件服务（不查数据库）
func NewMailServiceWithConfig(cfg *models.SMTPConfig) *MailService {
	return &MailService{cfg: cfg}
}

// SendMailDirect 直接使用内置配置发送邮件（不查数据库）
func (s *MailService) SendMailDirect(to []string, subject, body string) error {
	if s.cfg == nil {
		return fmt.Errorf("SMTP配置为空")
	}
	return s.sendWithConfig(s.cfg, to, subject, body)
}

// getSMTPConfig 从数据库获取SMTP配置
func (s *MailService) getSMTPConfig() (*models.SMTPConfig, error) {
	if s.cfg != nil {
		return s.cfg, nil
	}
	var cfg models.SMTPConfig
	if err := db.DB.First(&cfg).Error; err != nil {
		return nil, fmt.Errorf("SMTP配置不存在: %w", err)
	}
	return &cfg, nil
}

// IsEnabled 检查SMTP是否启用
func (s *MailService) IsEnabled() bool {
	cfg, err := s.getSMTPConfig()
	if err != nil {
		return false
	}
	return cfg.Enabled
}

// IsVerifyEmailEnabled 检查是否启用了注册邮箱验证
func (s *MailService) IsVerifyEmailEnabled() bool {
	cfg, err := s.getSMTPConfig()
	if err != nil {
		return false
	}
	return cfg.Enabled && cfg.VerifyEmail
}

// SendMail 发送邮件
func (s *MailService) SendMail(to []string, subject, body string) error {
	cfg, err := s.getSMTPConfig()
	if err != nil {
		return fmt.Errorf("SMTP配置不存在: %w", err)
	}
	if !cfg.Enabled {
		return fmt.Errorf("SMTP未启用")
	}

	return s.sendWithConfig(cfg, to, subject, body)
}

// sendWithConfig 使用指定配置发送邮件
func (s *MailService) sendWithConfig(cfg *models.SMTPConfig, to []string, subject, body string) error {
	addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))

	headers := map[string]string{
		"From":         fmt.Sprintf("%s <%s>", cfg.FromName, cfg.FromEmail),
		"To":           strings.Join(to, ", "),
		"Subject":      subject,
		"MIME-Version": "1.0",
		"Content-Type": "text/html; charset=UTF-8",
		"Date":         time.Now().Format(time.RFC1123Z),
	}

	var msgBuilder strings.Builder
	for k, v := range headers {
		msgBuilder.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msgBuilder.WriteString("\r\n")
	msgBuilder.WriteString(body)
	msg := msgBuilder.String()

	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)

	switch cfg.Encryption {
	case "ssl":
		return s.sendSSL(addr, cfg.Host, auth, cfg.FromEmail, to, msg)
	case "none":
		return smtp.SendMail(addr, auth, cfg.FromEmail, to, []byte(msg))
	default: // starttls
		return s.sendStartTLS(addr, cfg.Host, auth, cfg.FromEmail, to, msg)
	}
}

// sendStartTLS 使用STARTTLS发送
func (s *MailService) sendStartTLS(addr, host string, auth smtp.Auth, from string, to []string, msg string) error {
	conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
	if err != nil {
		return fmt.Errorf("连接SMTP服务器失败: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("创建SMTP客户端失败: %w", err)
	}
	defer client.Close()

	if err = client.StartTLS(&tls.Config{ServerName: host}); err != nil {
		return fmt.Errorf("STARTTLS失败: %w", err)
	}

	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP认证失败: %w", err)
	}

	if err = client.Mail(from); err != nil {
		return fmt.Errorf("设置发件人失败: %w", err)
	}

	for _, recipient := range to {
		if err = client.Rcpt(recipient); err != nil {
			return fmt.Errorf("设置收件人失败: %w", err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("打开DATA失败: %w", err)
	}
	defer w.Close()

	if _, err = w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("写入邮件内容失败: %w", err)
	}

	return client.Quit()
}

// sendSSL 使用SSL/TLS直接加密连接发送
func (s *MailService) sendSSL(addr, host string, auth smtp.Auth, from string, to []string, msg string) error {
	tlsConfig := &tls.Config{ServerName: host}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("SSL连接SMTP服务器失败: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("创建SMTP客户端失败: %w", err)
	}
	defer client.Close()

	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP认证失败: %w", err)
	}

	if err = client.Mail(from); err != nil {
		return fmt.Errorf("设置发件人失败: %w", err)
	}

	for _, recipient := range to {
		if err = client.Rcpt(recipient); err != nil {
			return fmt.Errorf("设置收件人失败: %w", err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("打开DATA失败: %w", err)
	}
	defer w.Close()

	if _, err = w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("写入邮件内容失败: %w", err)
	}

	return client.Quit()
}

// SendTestMail 发送测试邮件
func (s *MailService) SendTestMail(to string) error {
	subject := "SMTP测试邮件"
	body := `
<div style="max-width:600px;margin:0 auto;font-family:sans-serif;">
  <h2>SMTP测试</h2>
  <p>这是一封测试邮件，如果您收到此邮件，说明SMTP配置正确。</p>
</div>
`
	return s.SendMail([]string{to}, subject, body)
}
