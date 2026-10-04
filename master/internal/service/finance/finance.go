package finance

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/service/payment"
)

// FinanceService 财务服务
type FinanceService struct {
	orderPaidCallback func(orderID uuid.UUID, userID uint, username string)
}

// NewFinanceService 创建财务服务
func NewFinanceService() *FinanceService {
	return &FinanceService{}
}

// SetOrderPaidCallback 设置订单支付成功回调（由 commerce service 注入）
func (s *FinanceService) SetOrderPaidCallback(cb func(orderID uuid.UUID, userID uint, username string)) {
	s.orderPaidCallback = cb
}

// =================== 支付渠道 ===================

// ListPaymentChannels 获取支付渠道列表
func (s *FinanceService) ListPaymentChannels() ([]models.PaymentChannel, error) {
	var channels []models.PaymentChannel
	if err := db.DB.Order("sort_order ASC, id ASC").Find(&channels).Error; err != nil {
		zap.L().Error("查询支付渠道失败", zap.Error(err))
		return nil, err
	}
	return channels, nil
}

// CreatePaymentChannel 创建支付渠道
func (s *FinanceService) CreatePaymentChannel(channel *models.PaymentChannel) error {
	if err := db.DB.Create(channel).Error; err != nil {
		zap.L().Error("创建支付渠道失败", zap.Error(err))
		return err
	}
	return nil
}

// UpdatePaymentChannel 更新支付渠道
func (s *FinanceService) UpdatePaymentChannel(id uint, updates map[string]interface{}) error {
	if err := db.DB.Model(&models.PaymentChannel{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		zap.L().Error("更新支付渠道失败", zap.Error(err))
		return err
	}
	return nil
}

// DeletePaymentChannel 删除支付渠道
func (s *FinanceService) DeletePaymentChannel(id uint) error {
	if err := db.DB.Delete(&models.PaymentChannel{}, id).Error; err != nil {
		zap.L().Error("删除支付渠道失败", zap.Error(err))
		return err
	}
	return nil
}

// =================== 账单 ===================

// ListBills 获取账单列表
func (s *FinanceService) ListBills(page, pageSize int, search, billType, status string, startTime, endTime *time.Time) ([]models.Bill, int64, error) {
	query := db.DB.Model(&models.Bill{})
	if search != "" {
		query = query.Where("bill_no ILIKE ? OR username ILIKE ? OR transaction_no ILIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}
	if billType != "" {
		query = query.Where("type = ?", billType)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if startTime != nil {
		query = query.Where("created_at >= ?", startTime)
	}
	if endTime != nil {
		query = query.Where("created_at <= ?", endTime)
	}

	var total int64
	query.Count(&total)

	var bills []models.Bill
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&bills).Error; err != nil {
		zap.L().Error("查询账单列表失败", zap.Error(err))
		return nil, 0, err
	}
	return bills, total, nil
}

// GetBill 获取账单详情
func (s *FinanceService) GetBill(id uint) (*models.Bill, error) {
	var bill models.Bill
	if err := db.DB.Where("id = ?", id).First(&bill).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("账单不存在")
		}
		return nil, err
	}
	return &bill, nil
}

// =================== 财务概览 ===================

// FinanceOverview 财务概览数据
type FinanceOverview struct {
	TotalRechargeCents  int64 `json:"total_recharge_cents"`
	TotalConsumeCents   int64 `json:"total_consume_cents"`
	TotalRefundCents    int64 `json:"total_refund_cents"`
	MonthRechargeCents  int64 `json:"month_recharge_cents"`
	MonthConsumeCents   int64 `json:"month_consume_cents"`
	TodayRechargeCents  int64 `json:"today_recharge_cents"`
	TodayConsumeCents   int64 `json:"today_consume_cents"`
	PendingBillsCount   int64 `json:"pending_bills_count"`
	TotalBalanceCents   int64 `json:"total_balance_cents"`
	ActiveChannelsCount int64 `json:"active_channels_count"`
}

// GetOverview 获取财务概览
func (s *FinanceService) GetOverview() (*FinanceOverview, error) {
	var overview FinanceOverview

	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// 充值总额
	db.DB.Model(&models.Bill{}).Where("type = ? AND status = ?", models.BillTypeRecharge, models.BillStatusPaid).
		Select("COALESCE(SUM(amount_cents), 0)").Scan(&overview.TotalRechargeCents)

	// 消费总额
	db.DB.Model(&models.Bill{}).Where("type = ? AND status = ?", models.BillTypeConsume, models.BillStatusPaid).
		Select("COALESCE(SUM(amount_cents), 0)").Scan(&overview.TotalConsumeCents)

	// 退款总额
	db.DB.Model(&models.Bill{}).Where("type = ? AND status = ?", models.BillTypeRefund, models.BillStatusRefunded).
		Select("COALESCE(SUM(amount_cents), 0)").Scan(&overview.TotalRefundCents)

	// 本月充值
	db.DB.Model(&models.Bill{}).Where("type = ? AND status = ? AND created_at >= ?", models.BillTypeRecharge, models.BillStatusPaid, monthStart).
		Select("COALESCE(SUM(amount_cents), 0)").Scan(&overview.MonthRechargeCents)

	// 本月消费
	db.DB.Model(&models.Bill{}).Where("type = ? AND status = ? AND created_at >= ?", models.BillTypeConsume, models.BillStatusPaid, monthStart).
		Select("COALESCE(SUM(amount_cents), 0)").Scan(&overview.MonthConsumeCents)

	// 今日充值
	db.DB.Model(&models.Bill{}).Where("type = ? AND status = ? AND created_at >= ?", models.BillTypeRecharge, models.BillStatusPaid, todayStart).
		Select("COALESCE(SUM(amount_cents), 0)").Scan(&overview.TodayRechargeCents)

	// 今日消费
	db.DB.Model(&models.Bill{}).Where("type = ? AND status = ? AND created_at >= ?", models.BillTypeConsume, models.BillStatusPaid, todayStart).
		Select("COALESCE(SUM(amount_cents), 0)").Scan(&overview.TodayConsumeCents)

	// 待处理账单数
	db.DB.Model(&models.Bill{}).Where("status = ?", models.BillStatusPending).Count(&overview.PendingBillsCount)

	// 所有用户余额总和
	db.DB.Model(&models.User{}).Where("status != ?", models.UserStatusDeleted).
		Select("COALESCE(SUM(balance_cents), 0)").Scan(&overview.TotalBalanceCents)

	// 启用的支付渠道数
	db.DB.Model(&models.PaymentChannel{}).Where("status = ?", models.PaymentChannelStatusEnabled).Count(&overview.ActiveChannelsCount)

	return &overview, nil
}

// DailyStat 日统计
type DailyStat struct {
	Date          string `json:"date"`
	RechargeCents int64  `json:"recharge_cents"`
	ConsumeCents  int64  `json:"consume_cents"`
}

// GetDailyStats 获取最近N天统计
func (s *FinanceService) GetDailyStats(days int) ([]DailyStat, error) {
	now := time.Now()
	startDate := now.AddDate(0, 0, -days+1)
	startDateTime := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, startDate.Location())

	type rawStat struct {
		Date          time.Time `json:"date"`
		RechargeCents int64     `json:"recharge_cents"`
		ConsumeCents  int64     `json:"consume_cents"`
	}

	var rechargeStats []rawStat
	db.DB.Model(&models.Bill{}).
		Select("DATE(created_at) as date, COALESCE(SUM(amount_cents), 0) as recharge_cents").
		Where("type = ? AND status = ? AND created_at >= ?", models.BillTypeRecharge, models.BillStatusPaid, startDateTime).
		Group("DATE(created_at)").
		Scan(&rechargeStats)

	var consumeStats []rawStat
	db.DB.Model(&models.Bill{}).
		Select("DATE(created_at) as date, COALESCE(SUM(amount_cents), 0) as consume_cents").
		Where("type = ? AND status = ? AND created_at >= ?", models.BillTypeConsume, models.BillStatusPaid, startDateTime).
		Group("DATE(created_at)").
		Scan(&consumeStats)

	rechargeMap := make(map[string]int64)
	for _, r := range rechargeStats {
		rechargeMap[r.Date.Format("2006-01-02")] = r.RechargeCents
	}
	consumeMap := make(map[string]int64)
	for _, r := range consumeStats {
		consumeMap[r.Date.Format("2006-01-02")] = r.ConsumeCents
	}

	result := make([]DailyStat, 0, days)
	for i := 0; i < days; i++ {
		d := startDate.AddDate(0, 0, i)
		dateStr := d.Format("2006-01-02")
		result = append(result, DailyStat{
			Date:          dateStr,
			RechargeCents: rechargeMap[dateStr],
			ConsumeCents:  consumeMap[dateStr],
		})
	}
	return result, nil
}

// =================== 用户充值 ===================

// ListEnabledPaymentChannels 获取启用的支付渠道列表 (用户前台)
// 过滤掉 balance(余额支付, 充值无意义) 和 manual(管理员手动支付, 仅后台使用)
func (s *FinanceService) ListEnabledPaymentChannels() ([]models.PaymentChannel, error) {
	var channels []models.PaymentChannel
	if err := db.DB.Where("status = ? AND type NOT IN ?", models.PaymentChannelStatusEnabled, []string{"balance", "manual"}).
		Order("sort_order ASC, id ASC").Find(&channels).Error; err != nil {
		return nil, err
	}
	return channels, nil
}

// CreateRechargeOrder 创建充值订单并调用驱动发起支付
func (s *FinanceService) CreateRechargeOrder(channelID, userID uint, amountCents int64, notifyURL, returnURL string) (*payment.PayResult, *models.Bill, error) {
	// 查询渠道 (排除 balance 和 manual, 用户不能用于充值)
	var channel models.PaymentChannel
	if err := db.DB.Where("id = ? AND status = ? AND type NOT IN ?", channelID, models.PaymentChannelStatusEnabled, []string{"balance", "manual"}).First(&channel).Error; err != nil {
		return nil, nil, fmt.Errorf("支付渠道不存在或不可用")
	}

	// 校验金额
	if amountCents <= 0 {
		return nil, nil, fmt.Errorf("充值金额必须大于0")
	}
	if channel.MinAmount > 0 && amountCents < channel.MinAmount {
		return nil, nil, fmt.Errorf("最低充值金额为 %.2f 元", float64(channel.MinAmount)/100)
	}
	if channel.MaxAmount > 0 && amountCents > channel.MaxAmount {
		return nil, nil, fmt.Errorf("最高充值金额为 %.2f 元", float64(channel.MaxAmount)/100)
	}

	// 查询用户
	var user models.User
	if err := db.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, nil, fmt.Errorf("用户不存在")
	}

	// 生成账单号
	billNo := fmt.Sprintf("R%s%06d", time.Now().Format("20060102150405"), userID)

	// 创建待支付账单
	bill := models.Bill{
		BillNo:             billNo,
		UserID:             userID,
		Username:           user.Username,
		Type:               models.BillTypeRecharge,
		Status:             models.BillStatusPending,
		AmountCents:        amountCents,
		BalanceBefore:      user.BalanceCents,
		BalanceAfter:       user.BalanceCents, // 支付成功后才更新
		PaymentChannelID:   &channel.ID,
		PaymentChannelName: channel.Name,
		Description:        fmt.Sprintf("充值: %s", channel.Name),
		ExpiresAt:          &[]time.Time{time.Now().Add(10 * time.Minute)}[0],
	}
	if err := db.DB.Create(&bill).Error; err != nil {
		return nil, nil, fmt.Errorf("创建账单失败: %w", err)
	}

	// 解析渠道配置
	var configMap map[string]string
	if channel.Config != "" {
		if err := json.Unmarshal([]byte(channel.Config), &configMap); err != nil {
			return nil, nil, fmt.Errorf("解析渠道配置失败: %w", err)
		}
	}

	// 获取驱动
	driver, ok := payment.GetDriver(channel.Type)
	if !ok {
		return nil, nil, fmt.Errorf("支付驱动 %s 未注册", channel.Type)
	}

	// 调用驱动发起支付
	payResult, err := driver.Pay(configMap, payment.PayOrder{
		TradeNo:     billNo,
		TotalAmount: amountCents,
		NotifyURL:   notifyURL,
		ReturnURL:   returnURL,
		UserID:      userID,
	})
	if err != nil {
		// 支付发起失败, 标记账单为失败
		db.DB.Model(&bill).Update("status", models.BillStatusFailed)
		return nil, nil, fmt.Errorf("发起支付失败: %w", err)
	}

	return payResult, &bill, nil
}

// HandlePaymentNotify 处理支付回调
func (s *FinanceService) HandlePaymentNotify(channelID uint, params map[string]string) error {
	// 查询渠道
	var channel models.PaymentChannel
	if err := db.DB.Where("id = ?", channelID).First(&channel).Error; err != nil {
		return fmt.Errorf("支付渠道不存在")
	}

	// 获取驱动
	driver, ok := payment.GetDriver(channel.Type)
	if !ok {
		return fmt.Errorf("支付驱动 %s 未注册", channel.Type)
	}

	// 解析渠道配置
	var configMap map[string]string
	if channel.Config != "" {
		if err := json.Unmarshal([]byte(channel.Config), &configMap); err != nil {
			return fmt.Errorf("解析渠道配置失败: %w", err)
		}
	}

	// 验证回调
	result, err := driver.Notify(configMap, params)
	if err != nil {
		return fmt.Errorf("回调验证失败: %w", err)
	}

	// 查询账单
	var bill models.Bill
	if err := db.DB.Where("bill_no = ?", result.TradeNo).First(&bill).Error; err != nil {
		return fmt.Errorf("账单不存在: %s", result.TradeNo)
	}

	// 防止重复处理
	if bill.Status == models.BillStatusPaid {
		return nil
	}

	// 检查是否已过期
	if bill.ExpiresAt != nil && time.Now().After(*bill.ExpiresAt) {
		// 标记为已取消
		db.DB.Model(&bill).Update("status", models.BillStatusCancelled)
		return fmt.Errorf("账单已过期: %s", bill.BillNo)
	}

	// 根据账单类型区分处理
	if bill.Type == models.BillTypeConsume {
		var paidOrder *models.Order
		// 订单支付回调：更新账单 + 更新订单状态
		err := db.DB.Transaction(func(tx *gorm.DB) error {
			// 更新账单
			if err := tx.Model(&bill).Updates(map[string]interface{}{
				"status":         models.BillStatusPaid,
				"transaction_no": result.CallbackNo,
			}).Error; err != nil {
				return fmt.Errorf("更新账单失败: %w", err)
			}

			// 查找关联的订单
			var order models.Order
			if bill.ID != 0 {
				if err := tx.Where("bill_id = ?", bill.ID).First(&order).Error; err != nil {
					zap.L().Warn("订单支付回调: 未找到关联订单", zap.String("bill_no", bill.BillNo))
					return nil
				}

				now := time.Now()
				if err := tx.Model(&order).Updates(map[string]interface{}{
					"status":  models.OrderStatusPaid,
					"paid_at": &now,
				}).Error; err != nil {
					return fmt.Errorf("更新订单状态失败: %w", err)
				}
				paidOrder = &order
			}

			return nil
		})
		if err != nil {
			return err
		}
		// 事务成功后异步履约
		if paidOrder != nil && s.orderPaidCallback != nil {
			go s.orderPaidCallback(paidOrder.ID, bill.UserID, bill.Username)
		}
		return nil
	}

	// 充值账单：事务更新账单 + 加余额 + 记录流水
	return db.DB.Transaction(func(tx *gorm.DB) error {
		// 查询用户当前余额
		var user models.User
		if err := tx.Where("id = ?", bill.UserID).First(&user).Error; err != nil {
			return fmt.Errorf("用户不存在")
		}

		balanceBefore := user.BalanceCents
		balanceAfter := balanceBefore + bill.AmountCents

		// 更新账单
		if err := tx.Model(&bill).Updates(map[string]interface{}{
			"status":         models.BillStatusPaid,
			"transaction_no": result.CallbackNo,
			"balance_before": balanceBefore,
			"balance_after":  balanceAfter,
		}).Error; err != nil {
			return fmt.Errorf("更新账单失败: %w", err)
		}

		// 用户加余额
		if err := tx.Model(&models.User{}).Where("id = ?", bill.UserID).
			Update("balance_cents", balanceAfter).Error; err != nil {
			return fmt.Errorf("更新余额失败: %w", err)
		}

		// 记录钱包流水
		txRecord := models.WalletTransaction{
			UserID:      bill.UserID,
			AmountCents: bill.AmountCents,
			Type:        "recharge",
			BillID:      &bill.ID,
			Description: fmt.Sprintf("充值: %s", channel.Name),
		}
		if err := tx.Create(&txRecord).Error; err != nil {
			return fmt.Errorf("记录流水失败: %w", err)
		}

		return nil
	})
}

// GetBillByNo 根据账单号查询账单
func (s *FinanceService) GetBillByNo(billNo string) (*models.Bill, error) {
	var bill models.Bill
	if err := db.DB.Where("bill_no = ?", billNo).First(&bill).Error; err != nil {
		return nil, fmt.Errorf("账单不存在")
	}
	// 如果账单已过期且仍为 pending，标记为已取消
	if bill.Status == models.BillStatusPending && bill.ExpiresAt != nil && time.Now().After(*bill.ExpiresAt) {
		db.DB.Model(&bill).Update("status", models.BillStatusCancelled)
		bill.Status = models.BillStatusCancelled
	}
	return &bill, nil
}

// ListUserBills 获取用户账单列表
func (s *FinanceService) ListUserBills(userID uint, page, pageSize int) ([]models.Bill, int64, error) {
	query := db.DB.Model(&models.Bill{}).Where("user_id = ?", userID)

	var total int64
	query.Count(&total)

	var bills []models.Bill
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&bills).Error; err != nil {
		return nil, 0, err
	}
	return bills, total, nil
}

// GetUserBill 获取用户账单详情
func (s *FinanceService) GetUserBill(userID uint, billID uint) (*models.Bill, error) {
	var bill models.Bill
	if err := db.DB.Where("id = ? AND user_id = ?", billID, userID).First(&bill).Error; err != nil {
		return nil, fmt.Errorf("账单不存在")
	}
	if bill.Status == models.BillStatusPending && bill.ExpiresAt != nil && time.Now().After(*bill.ExpiresAt) {
		db.DB.Model(&bill).Update("status", models.BillStatusCancelled)
		bill.Status = models.BillStatusCancelled
	}
	return &bill, nil
}

// ListUserWalletTransactions 获取用户钱包流水
func (s *FinanceService) ListUserWalletTransactions(userID uint, page, pageSize int) ([]models.WalletTransaction, int64, error) {
	query := db.DB.Model(&models.WalletTransaction{}).Where("user_id = ?", userID)

	var total int64
	query.Count(&total)

	var txs []models.WalletTransaction
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&txs).Error; err != nil {
		return nil, 0, err
	}
	return txs, total, nil
}
