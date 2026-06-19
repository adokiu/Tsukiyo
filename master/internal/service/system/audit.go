package system

import (
	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// AuditService 审计服务
type AuditService struct{}

// NewAuditService 创建审计服务
func NewAuditService() *AuditService {
	return &AuditService{}
}

// AuditLogInfo 审计日志信息
type AuditLogInfo struct {
	ID        string `json:"id"`
	UserID    uint   `json:"user_id"`
	Username  string `json:"username"`
	Action    string `json:"action"`
	Target    string `json:"target"`
	Detail    string `json:"detail"`
	IPAddress string `json:"ip_address"`
	Success   bool   `json:"success"`
	CreatedAt string `json:"created_at"`
}

// ListAuditLogs 获取审计日志列表
func (s *AuditService) ListAuditLogs(limit int) ([]AuditLogInfo, error) {
	var logs []models.AuditLog
	if err := db.DB.Order("created_at DESC").Limit(limit).Find(&logs).Error; err != nil {
		return nil, err
	}

	result := make([]AuditLogInfo, 0, len(logs))
	for _, log := range logs {
		result = append(result, AuditLogInfo{
			ID:        log.ID.String(),
			UserID:    log.UserID,
			Username:  log.Username,
			Action:    log.Action,
			Target:    log.Target,
			Detail:    log.Detail,
			IPAddress: log.IPAddress,
			Success:   log.Success,
			CreatedAt: log.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return result, nil
}
