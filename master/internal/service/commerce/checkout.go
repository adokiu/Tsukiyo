package commerce

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/service/instance"
	"tsukiyo/master/internal/service/payment"
)

// ==================== 购物车 ====================

// AddCartRequest 添加购物车请求
type AddCartRequest struct {
	ProductID string          `json:"product_id" binding:"required"`
	Config    json.RawMessage `json:"config" binding:"required"`
	Quantity  int             `json:"quantity"`
}

// CartItemResponse 购物车项响应
type CartItemResponse struct {
	models.CartItem
	Product *models.Product `json:"product"`
}

// AddToCart 添加购物车
func (s *CommerceService) AddToCart(userID uint, req AddCartRequest) (*models.CartItem, error) {
	productID, err := uuid.Parse(req.ProductID)
	if err != nil {
		return nil, fmt.Errorf("无效的商品 ID")
	}

	var product models.Product
	if err := db.DB.Where("id = ? AND status = ?", productID, models.ProductStatusActive).First(&product).Error; err != nil {
		return nil, fmt.Errorf("商品不存在或已下架")
	}

	quantity := req.Quantity
	if quantity < 1 {
		quantity = 1
	}

	// 计算单价
	unitPrice := s.calculateConfigPrice(&product, req.Config)

	item := models.CartItem{
		UserID:         userID,
		ProductID:      productID,
		Config:         req.Config,
		UnitPriceCents: unitPrice,
		Quantity:       quantity,
	}

	if err := db.DB.Create(&item).Error; err != nil {
		return nil, fmt.Errorf("添加购物车失败: %w", err)
	}

	return &item, nil
}

// ListCart 获取购物车列表
func (s *CommerceService) ListCart(userID uint) ([]CartItemResponse, error) {
	var items []models.CartItem
	if err := db.DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&items).Error; err != nil {
		return nil, err
	}

	var productIDs []uuid.UUID
	for _, item := range items {
		productIDs = append(productIDs, item.ProductID)
	}

	var products []models.Product
	if len(productIDs) > 0 {
		db.DB.Where("id IN ?", productIDs).Find(&products)
	}

	productMap := make(map[uuid.UUID]*models.Product)
	for i := range products {
		productMap[products[i].ID] = &products[i]
	}

	result := make([]CartItemResponse, 0, len(items))
	for _, item := range items {
		result = append(result, CartItemResponse{
			CartItem: item,
			Product:  productMap[item.ProductID],
		})
	}

	return result, nil
}

// UpdateCartRequest 更新购物车请求
type UpdateCartRequest struct {
	Quantity *int            `json:"quantity"`
	Config   json.RawMessage `json:"config"`
}

// UpdateCart 更新购物车项
func (s *CommerceService) UpdateCart(userID uint, itemID string, req UpdateCartRequest) error {
	id, err := uuid.Parse(itemID)
	if err != nil {
		return fmt.Errorf("无效的购物车项 ID")
	}

	var item models.CartItem
	if err := db.DB.Where("id = ? AND user_id = ?", id, userID).First(&item).Error; err != nil {
		return fmt.Errorf("购物车项不存在")
	}

	updates := map[string]interface{}{}
	if req.Quantity != nil {
		if *req.Quantity < 1 {
			return fmt.Errorf("数量不能小于 1")
		}
		updates["quantity"] = *req.Quantity
	}
	if len(req.Config) > 0 {
		// 重新计算单价
		var product models.Product
		if err := db.DB.Where("id = ?", item.ProductID).First(&product).Error; err == nil {
			updates["unit_price_cents"] = s.calculateConfigPrice(&product, req.Config)
		}
		updates["config"] = req.Config
	}

	if len(updates) == 0 {
		return nil
	}

	return db.DB.Model(&item).Updates(updates).Error
}

// RemoveFromCart 从购物车移除
func (s *CommerceService) RemoveFromCart(userID uint, itemID string) error {
	id, err := uuid.Parse(itemID)
	if err != nil {
		return fmt.Errorf("无效的购物车项 ID")
	}
	return db.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&models.CartItem{}).Error
}

// ClearCart 清空购物车
func (s *CommerceService) ClearCart(userID uint) error {
	return db.DB.Where("user_id = ?", userID).Delete(&models.CartItem{}).Error
}

// ==================== 优惠码 ====================

// ValidateCouponRequest 验证优惠码请求
type ValidateCouponRequest struct {
	Code        string `json:"code" binding:"required"`
	AmountCents int64  `json:"amount_cents" binding:"required"`
}

// ValidateCouponResult 验证优惠码结果
type ValidateCouponResult struct {
	Valid         bool   `json:"valid"`
	DiscountCents int64  `json:"discount_cents"`
	Message       string `json:"message,omitempty"`
	CouponID      string `json:"coupon_id,omitempty"`
	CouponName    string `json:"coupon_name,omitempty"`
	CouponType    string `json:"coupon_type,omitempty"`
	CouponValue   int64  `json:"coupon_value,omitempty"`
}

// ValidateCoupon 验证优惠码
func (s *CommerceService) ValidateCoupon(userID uint, req ValidateCouponRequest) (*ValidateCouponResult, error) {
	var coupon models.Coupon
	if err := db.DB.Where("code = ? AND status = ?", req.Code, models.CouponStatusActive).First(&coupon).Error; err != nil {
		return &ValidateCouponResult{
			Valid:   false,
			Message: "优惠码无效或已失效",
		}, nil
	}

	now := time.Now()
	if coupon.ValidFrom != nil && now.Before(*coupon.ValidFrom) {
		return &ValidateCouponResult{
			Valid:   false,
			Message: "优惠码尚未生效",
		}, nil
	}
	if coupon.ValidTo != nil && now.After(*coupon.ValidTo) {
		return &ValidateCouponResult{
			Valid:   false,
			Message: "优惠码已过期",
		}, nil
	}
	if coupon.UsageLimit > 0 && coupon.UsedCount >= coupon.UsageLimit {
		return &ValidateCouponResult{
			Valid:   false,
			Message: "优惠码已用完",
		}, nil
	}
	if req.AmountCents < coupon.MinAmountCents {
		return &ValidateCouponResult{
			Valid:   false,
			Message: fmt.Sprintf("需满 %.2f 元才可使用", float64(coupon.MinAmountCents)/100),
		}, nil
	}

	var discount int64
	switch coupon.Type {
	case models.CouponTypeFixed:
		discount = coupon.Value
		if discount > req.AmountCents {
			discount = req.AmountCents
		}
	case models.CouponTypePercent:
		discount = req.AmountCents * coupon.Value / 100
		if coupon.MaxDiscountCents > 0 && discount > coupon.MaxDiscountCents {
			discount = coupon.MaxDiscountCents
		}
	default:
		return &ValidateCouponResult{
			Valid:   false,
			Message: "优惠码类型无效",
		}, nil
	}

	return &ValidateCouponResult{
		Valid:         true,
		DiscountCents: discount,
		CouponID:      coupon.ID.String(),
		CouponName:    coupon.Name,
		CouponType:    string(coupon.Type),
		CouponValue:   coupon.Value,
	}, nil
}

// ==================== 结算（创建订单） ====================

// CheckoutItem 结算项
type CheckoutItem struct {
	ProductID string          `json:"product_id" binding:"required"`
	Config    json.RawMessage `json:"config" binding:"required"`
	Quantity  int             `json:"quantity"`
}

// CheckoutRequest 结算请求
type CheckoutRequest struct {
	Items      []CheckoutItem `json:"items" binding:"required,min=1"`
	CouponCode string         `json:"coupon_code"`
}

// CheckoutResult 结算结果
type CheckoutResult struct {
	OrderID       string `json:"order_id"`
	OrderNo       string `json:"order_no"`
	SubtotalCents int64  `json:"subtotal_cents"`
	DiscountCents int64  `json:"discount_cents"`
	TotalCents    int64  `json:"total_cents"`
}

// Checkout 结算创建订单
func (s *CommerceService) Checkout(userID uint, username string, req CheckoutRequest) (*CheckoutResult, error) {
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("结算项不能为空")
	}

	// 计算小计
	var subtotal int64
	var orderItems []models.OrderItem
	var productIDs []uuid.UUID

	for _, item := range req.Items {
		productID, err := uuid.Parse(item.ProductID)
		if err != nil {
			return nil, fmt.Errorf("无效的商品 ID: %s", item.ProductID)
		}

		var product models.Product
		if err := db.DB.Where("id = ? AND status = ?", productID, models.ProductStatusActive).First(&product).Error; err != nil {
			return nil, fmt.Errorf("商品 %s 不存在或已下架", productID)
		}

		quantity := item.Quantity
		if quantity < 1 {
			quantity = 1
		}

		unitPrice := s.calculateConfigPrice(&product, item.Config)
		subtotalCents := unitPrice * int64(quantity)
		subtotal += subtotalCents

		productIDs = append(productIDs, productID)

		orderItems = append(orderItems, models.OrderItem{
			ProductID:      productID,
			Config:         item.Config,
			UnitPriceCents: unitPrice,
			Quantity:       quantity,
			SubtotalCents:  subtotalCents,
		})
	}

	// 优惠码折扣
	var discountCents int64
	var couponID *uuid.UUID
	var couponCode string
	if req.CouponCode != "" {
		result, err := s.ValidateCoupon(userID, ValidateCouponRequest{
			Code:        req.CouponCode,
			AmountCents: subtotal,
		})
		if err != nil || !result.Valid {
			return nil, fmt.Errorf("优惠码无效: %s", result.Message)
		}
		discountCents = result.DiscountCents
		couponCode = req.CouponCode
		cid, _ := uuid.Parse(result.CouponID)
		couponID = &cid
	}

	totalCents := subtotal - discountCents
	if totalCents < 0 {
		totalCents = 0
	}

	// 生成订单号
	orderNo := fmt.Sprintf("O%s%06d", time.Now().Format("20060102150405"), userID)

	// 创建订单（事务）
	var order models.Order
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		order = models.Order{
			OrderNo:       orderNo,
			UserID:        userID,
			Username:      username,
			SubtotalCents: subtotal,
			DiscountCents: discountCents,
			TotalCents:    totalCents,
			CouponID:      couponID,
			CouponCode:    couponCode,
			Status:        models.OrderStatusPending,
			Description:   fmt.Sprintf("订单包含 %d 个商品", len(orderItems)),
			ExpiresAt:     func() *time.Time { t := time.Now().Add(30 * time.Minute); return &t }(),
		}

		if err := tx.Create(&order).Error; err != nil {
			return fmt.Errorf("创建订单失败: %w", err)
		}

		for i := range orderItems {
			orderItems[i].OrderID = order.ID
		}
		if err := tx.Create(&orderItems).Error; err != nil {
			return fmt.Errorf("创建订单项失败: %w", err)
		}

		// 如果使用了优惠码，增加已用次数
		if couponID != nil {
			if err := tx.Model(&models.Coupon{}).Where("id = ?", couponID).
				UpdateColumn("used_count", gorm.Expr("used_count + 1")).Error; err != nil {
				return fmt.Errorf("更新优惠码使用次数失败: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &CheckoutResult{
		OrderID:       order.ID.String(),
		OrderNo:       orderNo,
		SubtotalCents: subtotal,
		DiscountCents: discountCents,
		TotalCents:    totalCents,
	}, nil
}

// ==================== 支付订单 ====================

// PayOrderRequest 支付订单请求
type PayOrderRequest struct {
	OrderID   string `json:"order_id" binding:"required"`
	Method    string `json:"method" binding:"required,oneof=balance channel"`
	ChannelID *uint  `json:"channel_id"`
}

// PayOrderResult 支付订单结果
type PayOrderResult struct {
	OrderID string `json:"order_id"`
	OrderNo string `json:"order_no"`
	Status  string `json:"status"`
	PayURL  string `json:"pay_url,omitempty"`
}

// PayOrder 支付订单
func (s *CommerceService) PayOrder(userID uint, username string, req PayOrderRequest, notifyURLBase, returnURL string) (*PayOrderResult, error) {
	orderID, err := uuid.Parse(req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("无效的订单 ID")
	}

	var order models.Order
	if err := db.DB.Where("id = ? AND user_id = ?", orderID, userID).First(&order).Error; err != nil {
		return nil, fmt.Errorf("订单不存在")
	}

	if order.Status != models.OrderStatusPending {
		return nil, fmt.Errorf("订单状态不允许支付")
	}

	// 检查是否过期
	if order.ExpiresAt != nil && time.Now().After(*order.ExpiresAt) {
		db.DB.Model(&order).Update("status", models.OrderStatusCancelled)
		return nil, fmt.Errorf("订单已过期，请重新结算")
	}

	if req.Method == "balance" {
		// 余额支付
		return s.payOrderByBalance(&order, userID, username)
	}

	// 外部支付渠道
	if req.ChannelID == nil {
		return nil, fmt.Errorf("请选择支付渠道")
	}

	return s.payOrderByChannel(&order, userID, username, *req.ChannelID, notifyURLBase, returnURL)
}

// payOrderByBalance 余额支付
func (s *CommerceService) payOrderByBalance(order *models.Order, userID uint, username string) (*PayOrderResult, error) {
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		// 加行锁查余额
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", userID).First(&user).Error; err != nil {
			return fmt.Errorf("用户不存在")
		}

		if user.BalanceCents < order.TotalCents {
			return fmt.Errorf("余额不足，需要 ¥%.2f，当前余额 ¥%.2f", float64(order.TotalCents)/100, float64(user.BalanceCents)/100)
		}

		balanceBefore := user.BalanceCents
		balanceAfter := balanceBefore - order.TotalCents

		// 扣减余额
		if err := tx.Model(&models.User{}).Where("id = ?", userID).Update("balance_cents", balanceAfter).Error; err != nil {
			return fmt.Errorf("扣减余额失败: %w", err)
		}

		// 创建账单
		billNo := fmt.Sprintf("P%s%06d", time.Now().Format("20060102150405"), userID)
		bill := models.Bill{
			BillNo:        billNo,
			UserID:        userID,
			Username:      username,
			Type:          models.BillTypeConsume,
			Status:        models.BillStatusPaid,
			AmountCents:   order.TotalCents,
			BalanceBefore: balanceBefore,
			BalanceAfter:  balanceAfter,
			Description:   fmt.Sprintf("订单支付: %s", order.OrderNo),
		}
		if err := tx.Create(&bill).Error; err != nil {
			return fmt.Errorf("创建账单失败: %w", err)
		}

		// 钱包流水
		txRecord := models.WalletTransaction{
			UserID:      userID,
			AmountCents: -order.TotalCents,
			Type:        "consume",
			BillID:      &bill.ID,
			Description: fmt.Sprintf("订单支付: %s", order.OrderNo),
		}
		if err := tx.Create(&txRecord).Error; err != nil {
			return fmt.Errorf("记录流水失败: %w", err)
		}

		// 更新订单状态
		now := time.Now()
		if err := tx.Model(order).Updates(map[string]interface{}{
			"status":         models.OrderStatusPaid,
			"payment_method": "balance",
			"bill_id":        bill.ID,
			"paid_at":        &now,
		}).Error; err != nil {
			return fmt.Errorf("更新订单状态失败: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// 支付成功后创建实例
	s.fulfillOrder(order.ID, userID, username)

	return &PayOrderResult{
		OrderID: order.ID.String(),
		OrderNo: order.OrderNo,
		Status:  string(models.OrderStatusPaid),
	}, nil
}

// payOrderByChannel 外部支付渠道
func (s *CommerceService) payOrderByChannel(order *models.Order, userID uint, username string, channelID uint, notifyURLBase, returnURL string) (*PayOrderResult, error) {
	// 查询渠道
	var channel models.PaymentChannel
	if err := db.DB.Where("id = ? AND status = ? AND type NOT IN ?", channelID, models.PaymentChannelStatusEnabled, []string{"balance", "manual"}).First(&channel).Error; err != nil {
		return nil, fmt.Errorf("支付渠道不存在或不可用")
	}

	// 创建待支付账单
	billNo := fmt.Sprintf("O%s%06d", time.Now().Format("20060102150405"), userID)
	expiresAt := time.Now().Add(10 * time.Minute)
	bill := models.Bill{
		BillNo:             billNo,
		UserID:             userID,
		Username:           username,
		Type:               models.BillTypeConsume,
		Status:             models.BillStatusPending,
		AmountCents:        order.TotalCents,
		PaymentChannelID:   &channel.ID,
		PaymentChannelName: channel.Name,
		Description:        fmt.Sprintf("订单支付: %s", order.OrderNo),
		ExpiresAt:          &expiresAt,
	}
	if err := db.DB.Create(&bill).Error; err != nil {
		return nil, fmt.Errorf("创建账单失败: %w", err)
	}

	// 更新订单
	db.DB.Model(order).Updates(map[string]interface{}{
		"payment_method":     "channel",
		"payment_channel_id": channel.ID,
		"bill_id":            bill.ID,
		"expires_at":         &expiresAt,
	})

	// 调用支付驱动
	var configMap map[string]string
	if channel.Config != "" {
		json.Unmarshal([]byte(channel.Config), &configMap)
	}

	driver, ok := payment.GetDriver(channel.Type)
	if !ok {
		return nil, fmt.Errorf("支付驱动 %s 未注册", channel.Type)
	}

	payResult, err := driver.Pay(configMap, payment.PayOrder{
		TradeNo:     billNo,
		TotalAmount: order.TotalCents,
		NotifyURL:   fmt.Sprintf("%s/payment/notify/%d", notifyURLBase, channel.ID),
		ReturnURL:   returnURL,
		UserID:      userID,
	})
	if err != nil {
		db.DB.Model(&bill).Update("status", models.BillStatusFailed)
		return nil, fmt.Errorf("发起支付失败: %w", err)
	}

	return &PayOrderResult{
		OrderID: order.ID.String(),
		OrderNo: order.OrderNo,
		Status:  string(models.OrderStatusPending),
		PayURL:  payResult.Data,
	}, nil
}

// FulfillOrder 履约：创建实例（导出方法，供外部支付回调调用）
func (s *CommerceService) FulfillOrder(orderID uuid.UUID, userID uint, username string) {
	s.fulfillOrder(orderID, userID, username)
}

// fulfillOrder 履约：创建实例
func (s *CommerceService) fulfillOrder(orderID uuid.UUID, userID uint, username string) {
	var order models.Order
	if err := db.DB.Where("id = ?", orderID).First(&order).Error; err != nil {
		zap.L().Error("查询订单失败", zap.String("order_id", orderID.String()), zap.Error(err))
		return
	}

	var items []models.OrderItem
	if err := db.DB.Where("order_id = ?", orderID).Find(&items).Error; err != nil {
		zap.L().Error("查询订单项失败", zap.String("order_id", orderID.String()), zap.Error(err))
		return
	}

	allSuccess := true
	for _, item := range items {
		for i := 0; i < item.Quantity; i++ {
			inst, err := s.createInstanceFromOrderItem(item, userID, username)
			if err != nil {
				zap.L().Error("订单项创建实例失败",
					zap.String("order_id", orderID.String()),
					zap.String("order_item_id", item.ID.String()),
					zap.Error(err))
				allSuccess = false
				continue
			}
			// 关联实例到订单项
			db.DB.Model(&item).Update("instance_id", inst.ID)
		}
	}

	finalStatus := models.OrderStatusCompleted
	if !allSuccess {
		finalStatus = models.OrderStatusFailed
	}
	db.DB.Model(&order).Update("status", finalStatus)
}

// createInstanceFromOrderItem 从订单项创建实例
func (s *CommerceService) createInstanceFromOrderItem(item models.OrderItem, userID uint, username string) (*models.Instance, error) {
	var product models.Product
	if err := db.DB.Where("id = ?", item.ProductID).First(&product).Error; err != nil {
		return nil, fmt.Errorf("商品不存在: %w", err)
	}

	// 解析配置
	var cfg struct {
		Name          string `json:"name"`
		VCPU          int    `json:"vcpu"`
		MemoryMB      int    `json:"memory_mb"`
		DiskMB        int    `json:"disk_mb"`
		DataDiskMB    int    `json:"data_disk_mb"`
		PaymentPeriod string `json:"payment_period"`
		TemplateID    string `json:"template_id"`
		ImageKey      string `json:"image_key"`
		NodeID        string `json:"node_id"`
		BridgeID      string `json:"bridge_id"`
		LoginMethod   string `json:"login_method"`
		SSHPassword   string `json:"ssh_password"`
		SSHPublicKey  string `json:"ssh_public_key"`
		Trial         bool   `json:"trial"`
	}
	if err := json.Unmarshal(item.Config, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	// 确定节点
	nodeID := cfg.NodeID
	if nodeID == "" {
		var nodeIDs []string
		json.Unmarshal(product.NodeIDs, &nodeIDs)
		for _, nid := range nodeIDs {
			var node models.Node
			if err := db.DB.Where("id = ?", nid).First(&node).Error; err == nil && node.IsHealthy() {
				nodeID = nid
				break
			}
		}
		if nodeID == "" {
			return nil, fmt.Errorf("商品配置的节点均不可用")
		}
	}

	// 确定网桥
	bridgeID := cfg.BridgeID
	if bridgeID == "" {
		var nodeBridges map[string]string
		json.Unmarshal(product.NodeBridges, &nodeBridges)
		if bid, ok := nodeBridges[nodeID]; ok && bid != "" {
			bridgeID = bid
		}
	}

	// 解析存储池
	var diskStoragePools map[string]string
	json.Unmarshal(product.DiskStoragePools, &diskStoragePools)
	storagePool := diskStoragePools[nodeID]

	var dataDiskStoragePools map[string]string
	json.Unmarshal(product.DataDiskStoragePools, &dataDiskStoragePools)
	dataDiskStoragePool := dataDiskStoragePools[nodeID]

	// 计算过期时间
	var expiresAt *time.Time
	if cfg.Trial && product.TrialEnabled {
		exp := time.Now().Add(time.Duration(product.TrialHours) * time.Hour)
		expiresAt = &exp
	} else {
		months := periodMonths[cfg.PaymentPeriod]
		if months == 0 {
			months = 1
		}
		exp := time.Now().AddDate(0, months, 0)
		expiresAt = &exp
	}

	loginMethod := cfg.LoginMethod
	if loginMethod == "" {
		loginMethod = "auto"
	}

	vcpu := cfg.VCPU
	if vcpu < product.VCPUMin {
		vcpu = product.VCPUMin
	}
	if product.VCPUCustomMode == "unlimited" && vcpu > product.VCPUMax {
		vcpu = product.VCPUMax
	}
	memoryMB := cfg.MemoryMB
	if memoryMB < product.MemoryMinMB {
		memoryMB = product.MemoryMinMB
	}
	if product.MemoryCustomMode == "unlimited" && memoryMB > product.MemoryMaxMB {
		memoryMB = product.MemoryMaxMB
	}
	diskMB := cfg.DiskMB
	if diskMB < product.DiskMinMB {
		diskMB = product.DiskMinMB
	}
	if product.DiskCustomMode == "unlimited" && diskMB > product.DiskMaxMB {
		diskMB = product.DiskMaxMB
	}

	createReq := instance.CreateInstanceRequest{
		Name:            generateInstanceName(cfg.Name),
		Type:            string(product.Type),
		TemplateID:      cfg.TemplateID,
		ImageKey:        cfg.ImageKey,
		NodeID:          nodeID,
		BridgeID:        bridgeID,
		AssignToUserID:  userID,
		LoginMethod:     loginMethod,
		VCPU:            float64(vcpu),
		MemoryMB:        memoryMB,
		DiskMB:          diskMB,
		StoragePool:     storagePool,
		SSHPassword:     cfg.SSHPassword,
		SSHPublicKey:    cfg.SSHPublicKey,
		NetworkDownMbps: product.NetworkDownMinMbps,
		NetworkUpMbps:   product.NetworkUpMinMbps,
		SnapshotLimit:   3,
		ExpiresAt:       expiresAt,
	}

	if cfg.DataDiskMB > 0 && product.DataDiskAllow {
		createReq.DataDisks = []instance.DataDiskRequest{
			{
				Name:        "datadisk1",
				SizeMB:      cfg.DataDiskMB,
				StoragePool: dataDiskStoragePool,
				MountPoint:  "/mnt/data",
			},
		}
	}

	if product.TrafficMinGB > 0 {
		createReq.MonthlyTrafficGB = int64(product.TrafficMinGB)
	}
	createReq.TrafficMode = string(product.TrafficCalcMode)

	if product.IPv4Mode == "eip" {
		createReq.AssignEIPv4 = true
		createReq.EIPv4Count = 1
	}
	if product.IPv6Enabled {
		var ipv6Configs []struct {
			PrefixLen int `json:"prefix_len"`
			Min       int `json:"min"`
		}
		json.Unmarshal(product.IPv6Configs, &ipv6Configs)
		ipv6Count := 1
		ipv6PrefixLen := 128
		if len(ipv6Configs) > 0 {
			ipv6PrefixLen = ipv6Configs[0].PrefixLen
			if ipv6Configs[0].Min > 0 {
				ipv6Count = ipv6Configs[0].Min
			}
		}
		createReq.AssignEIPv6 = true
		createReq.EIPv6Count = ipv6Count
		createReq.EIPv6PrefixLen = ipv6PrefixLen
	}
	if product.IPv4Mode == "nat" {
		createReq.PortMappingCount = product.NATPortMin
		if createReq.PortMappingCount <= 0 {
			createReq.PortMappingCount = 1
		}
	}

	inst, _, err := instanceService.CreateInstance(createReq)
	if err != nil {
		return nil, fmt.Errorf("创建实例失败: %w", err)
	}

	return inst, nil
}

// calculateConfigPrice 计算配置单价
func (s *CommerceService) calculateConfigPrice(product *models.Product, config json.RawMessage) int64 {
	var cfg struct {
		VCPU          int    `json:"vcpu"`
		MemoryMB      int    `json:"memory_mb"`
		DiskMB        int    `json:"disk_mb"`
		DataDiskMB    int    `json:"data_disk_mb"`
		PaymentPeriod string `json:"payment_period"`
		Trial         bool   `json:"trial"`
	}
	json.Unmarshal(config, &cfg)

	req := UserOrderRequest{
		VCPU:          cfg.VCPU,
		MemoryMB:      cfg.MemoryMB,
		DiskMB:        cfg.DiskMB,
		DataDiskMB:    cfg.DataDiskMB,
		PaymentPeriod: cfg.PaymentPeriod,
		Trial:         cfg.Trial,
	}
	if req.PaymentPeriod == "" {
		req.PaymentPeriod = "monthly"
	}

	return CalculateOrderPrice(product, &req)
}

// GetOrder 获取订单详情
func (s *CommerceService) GetOrder(userID uint, orderID string) (*models.Order, error) {
	id, err := uuid.Parse(orderID)
	if err != nil {
		return nil, fmt.Errorf("无效的订单 ID")
	}

	var order models.Order
	if err := db.DB.Where("id = ? AND user_id = ?", id, userID).First(&order).Error; err != nil {
		return nil, fmt.Errorf("订单不存在")
	}

	// 如果订单已过期且仍为 pending，标记为已取消
	if order.Status == models.OrderStatusPending && order.ExpiresAt != nil && time.Now().After(*order.ExpiresAt) {
		db.DB.Model(&order).Update("status", models.OrderStatusCancelled)
		order.Status = models.OrderStatusCancelled
	}

	var items []models.OrderItem
	db.DB.Where("order_id = ?", order.ID).Find(&items)
	order.Items = items

	return &order, nil
}

// ListOrders 获取订单列表
func (s *CommerceService) ListOrders(userID uint) ([]models.Order, error) {
	var orders []models.Order
	if err := db.DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&orders).Error; err != nil {
		return nil, err
	}
	return orders, nil
}

// InvoiceItemData 发票订单项数据
type InvoiceItemData struct {
	OrderItemID    uuid.UUID `json:"order_item_id"`
	ProductName    string    `json:"product_name"`
	Quantity       int       `json:"quantity"`
	UnitPriceCents int64     `json:"unit_price_cents"`
	SubtotalCents  int64     `json:"subtotal_cents"`
	// 配置详情
	VCPU          int    `json:"vcpu"`
	MemoryMB      int    `json:"memory_mb"`
	DiskMB        int    `json:"disk_mb"`
	DataDiskMB    int    `json:"data_disk_mb"`
	PaymentPeriod string `json:"payment_period"`
	TemplateID    string `json:"template_id"`
	ImageKey      string `json:"image_key"`
	LoginMethod   string `json:"login_method"`
	// 实例信息（如果已创建）
	InstanceID       *string `json:"instance_id,omitempty"`
	InstanceName     string  `json:"instance_name,omitempty"`
	InternalIPv4     string  `json:"internal_ipv4,omitempty"`
	InternalIPv6     string  `json:"internal_ipv6,omitempty"`
	NetworkDownMbps  int     `json:"network_down_mbps"`
	NetworkUpMbps    int     `json:"network_up_mbps"`
	MonthlyTrafficGB int64   `json:"monthly_traffic_gb"`
	IPv4Mode         string  `json:"ipv4_mode,omitempty"`
	IPv6Mode         string  `json:"ipv6_mode,omitempty"`
	ExpiresAt        *string `json:"expires_at,omitempty"`
}

// InvoiceData 发票数据
type InvoiceData struct {
	// 站点信息
	SiteName     string `json:"site_name"`
	SiteURL      string `json:"site_url"`
	ContactEmail string `json:"contact_email"`
	// 用户信息
	Username string `json:"username"`
	Email    string `json:"email"`
	// 订单信息
	OrderID       uuid.UUID `json:"order_id"`
	OrderNo       string    `json:"order_no"`
	Status        string    `json:"status"`
	SubtotalCents int64     `json:"subtotal_cents"`
	DiscountCents int64     `json:"discount_cents"`
	TotalCents    int64     `json:"total_cents"`
	CouponCode    string    `json:"coupon_code,omitempty"`
	PaymentMethod string    `json:"payment_method,omitempty"`
	CreatedAt     string    `json:"created_at"`
	PaidAt        *string   `json:"paid_at,omitempty"`
	// 账单信息
	BillNo             string `json:"bill_no,omitempty"`
	BillStatus         string `json:"bill_status,omitempty"`
	PaymentChannelName string `json:"payment_channel_name,omitempty"`
	// 订单项
	Items []InvoiceItemData `json:"items"`
}

// GetInvoice 获取发票数据
func (s *CommerceService) GetInvoice(userID uint, orderID string) (*InvoiceData, error) {
	id, err := uuid.Parse(orderID)
	if err != nil {
		return nil, fmt.Errorf("无效的订单 ID")
	}

	var order models.Order
	if err := db.DB.Where("id = ? AND user_id = ?", id, userID).First(&order).Error; err != nil {
		return nil, fmt.Errorf("订单不存在")
	}

	var items []models.OrderItem
	if err := db.DB.Where("order_id = ?", id).Find(&items).Error; err != nil {
		return nil, fmt.Errorf("查询订单项失败")
	}

	// 查询站点配置
	var site models.SiteConfig
	db.DB.First(&site)

	// 查询用户
	var user models.User
	db.DB.Where("id = ?", userID).First(&user)

	invoice := &InvoiceData{
		SiteName:      site.SiteName,
		SiteURL:       site.SiteURL,
		ContactEmail:  site.ContactEmail,
		Username:      user.Username,
		Email:         user.Email,
		OrderID:       order.ID,
		OrderNo:       order.OrderNo,
		Status:        string(order.Status),
		SubtotalCents: order.SubtotalCents,
		DiscountCents: order.DiscountCents,
		TotalCents:    order.TotalCents,
		CouponCode:    order.CouponCode,
		PaymentMethod: order.PaymentMethod,
		CreatedAt:     order.CreatedAt.Format("2006-01-02 15:04:05"),
	}

	if order.PaidAt != nil {
		paidAtStr := order.PaidAt.Format("2006-01-02 15:04:05")
		invoice.PaidAt = &paidAtStr
	}

	// 查询关联账单
	if order.BillID != nil {
		var bill models.Bill
		if err := db.DB.Where("id = ?", *order.BillID).First(&bill).Error; err == nil {
			invoice.BillNo = bill.BillNo
			invoice.BillStatus = string(bill.Status)
			invoice.PaymentChannelName = bill.PaymentChannelName
		}
	}

	// 构建订单项
	for _, item := range items {
		var product models.Product
		db.DB.Where("id = ?", item.ProductID).First(&product)

		invItem := InvoiceItemData{
			OrderItemID:    item.ID,
			ProductName:    product.Name,
			Quantity:       item.Quantity,
			UnitPriceCents: item.UnitPriceCents,
			SubtotalCents:  item.SubtotalCents,
		}

		// 解析配置
		var cfg struct {
			Name          string `json:"name"`
			VCPU          int    `json:"vcpu"`
			MemoryMB      int    `json:"memory_mb"`
			DiskMB        int    `json:"disk_mb"`
			DataDiskMB    int    `json:"data_disk_mb"`
			PaymentPeriod string `json:"payment_period"`
			TemplateID    string `json:"template_id"`
			ImageKey      string `json:"image_key"`
			LoginMethod   string `json:"login_method"`
		}
		json.Unmarshal(item.Config, &cfg)

		invItem.VCPU = cfg.VCPU
		invItem.MemoryMB = cfg.MemoryMB
		invItem.DiskMB = cfg.DiskMB
		invItem.DataDiskMB = cfg.DataDiskMB
		invItem.PaymentPeriod = cfg.PaymentPeriod
		invItem.TemplateID = cfg.TemplateID
		invItem.ImageKey = cfg.ImageKey
		invItem.LoginMethod = cfg.LoginMethod

		// 查询关联实例
		if item.InstanceID != nil {
			var inst models.Instance
			if err := db.DB.Where("id = ?", *item.InstanceID).First(&inst).Error; err == nil {
				instID := inst.ID.String()
				invItem.InstanceID = &instID
				invItem.InstanceName = inst.Name
				invItem.InternalIPv4 = inst.InternalIPv4
				invItem.InternalIPv6 = inst.InternalIPv6
				invItem.NetworkDownMbps = inst.NetworkDownMbps
				invItem.NetworkUpMbps = inst.NetworkUpMbps
				invItem.MonthlyTrafficGB = inst.MonthlyTrafficGB
				invItem.IPv4Mode = inst.IPv4Mode
				invItem.IPv6Mode = inst.IPv6Mode
				if inst.ExpiresAt != nil {
					expStr := inst.ExpiresAt.Format("2006-01-02 15:04:05")
					invItem.ExpiresAt = &expStr
				}
			}
		}

		invoice.Items = append(invoice.Items, invItem)
	}

	return invoice, nil
}
