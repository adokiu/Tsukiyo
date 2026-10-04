package support

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

type SupportService struct{}

func NewSupportService() *SupportService {
	return &SupportService{}
}

// CreateTicketRequest 创建工单请求
type CreateTicketRequest struct {
	Subject    string `json:"subject" binding:"required"`
	Content    string `json:"content" binding:"required"`
	Type       string `json:"type" binding:"required"`
	Priority   string `json:"priority" binding:"required"`
	ProductID  string `json:"product_id"`
	InstanceID string `json:"instance_id"`
}

// ReplyTicketRequest 回复工单请求
type ReplyTicketRequest struct {
	Content string `json:"content" binding:"required"`
}

// generateTicketNo 生成工单号 格式: TK-YYYYMMDD-XXXX
func generateTicketNo() string {
	now := time.Now()
	return fmt.Sprintf("TK-%s-%04d", now.Format("20060102"), now.Unix()%10000)
}

// CreateTicket 用户创建工单
func (s *SupportService) CreateTicket(userID uint, req *CreateTicketRequest) (*models.Ticket, error) {
	ticketType := models.TicketType(req.Type)
	switch ticketType {
	case models.TicketTypeFault, models.TicketTypeFinance, models.TicketTypeSales,
		models.TicketTypeTech, models.TicketTypeComplaint, models.TicketTypeOther:
	default:
		return nil, errors.New("无效的工单类型")
	}

	priority := models.TicketPriority(req.Priority)
	switch priority {
	case models.TicketPriorityLow, models.TicketPriorityNormal,
		models.TicketPriorityHigh, models.TicketPriorityUrgent:
	default:
		return nil, errors.New("无效的优先级")
	}

	ticket := &models.Ticket{
		TicketNo: generateTicketNo(),
		UserID:   userID,
		Subject:  req.Subject,
		Content:  req.Content,
		Type:     ticketType,
		Priority: priority,
		Status:   models.TicketStatusOpen,
	}

	if req.ProductID != "" {
		pid, err := uuid.Parse(req.ProductID)
		if err != nil {
			return nil, errors.New("无效的商品ID")
		}
		ticket.ProductID = &pid
	}

	if req.InstanceID != "" {
		iid, err := uuid.Parse(req.InstanceID)
		if err != nil {
			return nil, errors.New("无效的实例ID")
		}
		ticket.InstanceID = &iid
	}

	if err := db.DB.Create(ticket).Error; err != nil {
		return nil, fmt.Errorf("创建工单失败: %w", err)
	}

	return ticket, nil
}

// ListUserTickets 用户查看自己的工单列表
func (s *SupportService) ListUserTickets(userID uint, page, pageSize int, status string) ([]models.Ticket, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	var tickets []models.Ticket
	var total int64

	query := db.DB.Model(&models.Ticket{}).Where("user_id = ?", userID)
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Preload("Product").Preload("Instance").
		Order("created_at DESC").
		Offset(offset).Limit(pageSize).
		Find(&tickets).Error; err != nil {
		return nil, 0, err
	}

	return tickets, total, nil
}

// GetUserTicket 用户查看工单详情（含回复）
func (s *SupportService) GetUserTicket(userID uint, ticketID uuid.UUID) (*models.Ticket, error) {
	var ticket models.Ticket
	if err := db.DB.Preload("Product").Preload("Instance").
		Preload("Replies", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("Replies.User").
		Where("id = ? AND user_id = ?", ticketID, userID).
		First(&ticket).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &ticket, nil
}

// UserReplyTicket 用户回复工单
func (s *SupportService) UserReplyTicket(userID uint, ticketID uuid.UUID, content string) (*models.TicketReply, error) {
	var ticket models.Ticket
	if err := db.DB.Where("id = ? AND user_id = ?", ticketID, userID).First(&ticket).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}

	if ticket.Status == models.TicketStatusClosed {
		return nil, errors.New("工单已关闭，无法回复")
	}

	reply := &models.TicketReply{
		TicketID: ticketID,
		UserID:   userID,
		Content:  content,
		IsStaff:  false,
	}

	if err := db.DB.Create(reply).Error; err != nil {
		return nil, fmt.Errorf("回复失败: %w", err)
	}

	// 用户回复后，工单状态变为处理中
	if ticket.Status == models.TicketStatusWaiting || ticket.Status == models.TicketStatusResolved {
		db.DB.Model(&ticket).Update("status", models.TicketStatusInProgress)
	}

	return reply, nil
}

// UserCloseTicket 用户关闭工单
func (s *SupportService) UserCloseTicket(userID uint, ticketID uuid.UUID) error {
	var ticket models.Ticket
	if err := db.DB.Where("id = ? AND user_id = ?", ticketID, userID).First(&ticket).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return gorm.ErrRecordNotFound
		}
		return err
	}

	if ticket.Status == models.TicketStatusClosed {
		return errors.New("工单已关闭")
	}

	now := time.Now()
	return db.DB.Model(&ticket).Updates(map[string]interface{}{
		"status":    models.TicketStatusClosed,
		"closed_at": &now,
	}).Error
}

// ==================== 管理员端 ====================

// ListTickets 管理员查看所有工单
func (s *SupportService) ListTickets(page, pageSize int, status, ticketType, priority string, assigneeID *uint) ([]models.Ticket, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	var tickets []models.Ticket
	var total int64

	query := db.DB.Model(&models.Ticket{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if ticketType != "" {
		query = query.Where("type = ?", ticketType)
	}
	if priority != "" {
		query = query.Where("priority = ?", priority)
	}
	if assigneeID != nil {
		query = query.Where("assignee_id = ?", *assigneeID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Preload("User").Preload("Product").Preload("Instance").Preload("Assignee").
		Order("created_at DESC").
		Offset(offset).Limit(pageSize).
		Find(&tickets).Error; err != nil {
		return nil, 0, err
	}

	return tickets, total, nil
}

// GetTicket 管理员查看工单详情
func (s *SupportService) GetTicket(ticketID uuid.UUID) (*models.Ticket, error) {
	var ticket models.Ticket
	if err := db.DB.Preload("User").Preload("Product").Preload("Instance").Preload("Assignee").
		Preload("Replies", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("Replies.User").
		Where("id = ?", ticketID).
		First(&ticket).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &ticket, nil
}

// StaffReplyTicket 管理员回复工单
func (s *SupportService) StaffReplyTicket(staffID uint, ticketID uuid.UUID, content string) (*models.TicketReply, error) {
	var ticket models.Ticket
	if err := db.DB.Where("id = ?", ticketID).First(&ticket).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}

	reply := &models.TicketReply{
		TicketID: ticketID,
		UserID:   staffID,
		Content:  content,
		IsStaff:  true,
	}

	if err := db.DB.Create(reply).Error; err != nil {
		return nil, fmt.Errorf("回复失败: %w", err)
	}

	// 管理员回复后，工单状态变为等待用户回复
	if ticket.Status == models.TicketStatusOpen || ticket.Status == models.TicketStatusInProgress {
		db.DB.Model(&ticket).Update("status", models.TicketStatusWaiting)
	}

	return reply, nil
}

// UpdateTicketStatus 管理员更新工单状态
func (s *SupportService) UpdateTicketStatus(ticketID uuid.UUID, status string) error {
	ticketStatus := models.TicketStatus(status)
	switch ticketStatus {
	case models.TicketStatusOpen, models.TicketStatusInProgress,
		models.TicketStatusWaiting, models.TicketStatusResolved, models.TicketStatusClosed:
	default:
		return errors.New("无效的工单状态")
	}

	var ticket models.Ticket
	if err := db.DB.Where("id = ?", ticketID).First(&ticket).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return gorm.ErrRecordNotFound
		}
		return err
	}

	updates := map[string]interface{}{"status": ticketStatus}
	if ticketStatus == models.TicketStatusClosed {
		now := time.Now()
		updates["closed_at"] = &now
	}

	return db.DB.Model(&ticket).Updates(updates).Error
}

// AssignTicket 管理员分配工单
func (s *SupportService) AssignTicket(ticketID uuid.UUID, assigneeID uint) error {
	var ticket models.Ticket
	if err := db.DB.Where("id = ?", ticketID).First(&ticket).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return gorm.ErrRecordNotFound
		}
		return err
	}

	return db.DB.Model(&ticket).Update("assignee_id", assigneeID).Error
}

// UpdateTicketPriority 管理员更新工单优先级
func (s *SupportService) UpdateTicketPriority(ticketID uuid.UUID, priority string) error {
	ticketPriority := models.TicketPriority(priority)
	switch ticketPriority {
	case models.TicketPriorityLow, models.TicketPriorityNormal,
		models.TicketPriorityHigh, models.TicketPriorityUrgent:
	default:
		return errors.New("无效的优先级")
	}

	var ticket models.Ticket
	if err := db.DB.Where("id = ?", ticketID).First(&ticket).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return gorm.ErrRecordNotFound
		}
		return err
	}

	return db.DB.Model(&ticket).Update("priority", ticketPriority).Error
}
