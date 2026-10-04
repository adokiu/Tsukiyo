package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/service/finance"
	"tsukiyo/master/internal/service/payment"
)

var financeService *finance.FinanceService

// InitFinanceService 初始化财务服务
func InitFinanceService(svc *finance.FinanceService) {
	financeService = svc
}

// =================== 支付驱动 ===================

// GetPaymentDrivers 获取所有已注册的支付驱动及其配置字段定义
func GetPaymentDrivers(c *gin.Context) {
	drivers := payment.ListDriverInfo()
	c.JSON(http.StatusOK, gin.H{
		"data":  drivers,
		"total": len(drivers),
	})
}

// =================== 财务概览 ===================

// GetFinanceOverview 获取财务概览
func GetFinanceOverview(c *gin.Context) {
	overview, err := financeService.GetOverview()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	// 获取最近30天统计
	stats, err := financeService.GetDailyStats(30)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询统计失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"overview":    overview,
		"daily_stats": stats,
	})
}

// =================== 账单管理 ===================

// ListBills 获取账单列表
func ListBills(c *gin.Context) {
	page := 1
	pageSize := 20
	if v := c.Query("page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			page = p
		}
	}
	if v := c.Query("per_page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			pageSize = p
		}
	}

	search := c.Query("search")
	billType := c.Query("filter_type")
	status := c.Query("filter_status")

	var startTime, endTime *time.Time
	if v := c.Query("start_time"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			startTime = &t
		}
	}
	if v := c.Query("end_time"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			endTime = &t
		}
	}

	bills, total, err := financeService.ListBills(page, pageSize, search, billType, status, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      bills,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetBill 获取账单详情
func GetBill(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的账单 ID"})
		return
	}

	bill, err := financeService.GetBill(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, bill)
}

// =================== 支付渠道管理 ===================

// ListPaymentChannels 获取支付渠道列表
func ListPaymentChannels(c *gin.Context) {
	channels, err := financeService.ListPaymentChannels()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	result := make([]gin.H, 0, len(channels))
	for _, ch := range channels {
		result = append(result, gin.H{
			"id":                   ch.ID,
			"name":                 ch.Name,
			"type":                 ch.Type,
			"config":               ch.Config,
			"icon":                 ch.Icon,
			"description":          ch.Description,
			"fee_bearer":           ch.FeeBearer,
			"fee_type":             ch.FeeType,
			"fee_fixed_cents":      ch.FeeFixedCents,
			"fee_percent":          ch.FeePercent,
			"min_amount":           ch.MinAmount,
			"max_amount":           ch.MaxAmount,
			"settlement_currency":  ch.SettlementCurrency,
			"exchange_rate_markup": ch.ExchangeRateMarkup,
			"status":               ch.Status,
			"sort_order":           ch.SortOrder,
			"created_at":           ch.CreatedAt,
			"updated_at":           ch.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  result,
		"total": len(result),
	})
}

// CreatePaymentChannelRequest 创建支付渠道请求
type CreatePaymentChannelRequest struct {
	Name               string  `json:"name" binding:"required,max=64"`
	Type               string  `json:"type" binding:"required,max=32"`
	Config             string  `json:"config"`
	Icon               string  `json:"icon"`
	Description        string  `json:"description"`
	FeeBearer          string  `json:"fee_bearer"`
	FeeType            string  `json:"fee_type"`
	FeeFixedCents      int64   `json:"fee_fixed_cents"`
	FeePercent         float64 `json:"fee_percent"`
	MinAmount          int64   `json:"min_amount"`
	MaxAmount          int64   `json:"max_amount"`
	SettlementCurrency string  `json:"settlement_currency"`
	ExchangeRateMarkup float64 `json:"exchange_rate_markup"`
	Status             string  `json:"status"`
	SortOrder          int     `json:"sort_order"`
}

// CreatePaymentChannel 创建支付渠道
func CreatePaymentChannel(c *gin.Context) {
	var req CreatePaymentChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	status := models.PaymentChannelStatusEnabled
	if req.Status == "disabled" {
		status = models.PaymentChannelStatusDisabled
	}

	settlementCurrency := req.SettlementCurrency
	if settlementCurrency == "" {
		settlementCurrency = "CNY"
	}

	feeBearer := models.FeeBearerUser
	if req.FeeBearer == "merchant" {
		feeBearer = models.FeeBearerMerchant
	}

	feeType := models.FeeTypeFixed
	switch req.FeeType {
	case "percent":
		feeType = models.FeeTypePercent
	case "fixed_plus_percent":
		feeType = models.FeeTypeFixedPlusPercent
	}

	channel := models.PaymentChannel{
		Name:               req.Name,
		Type:               req.Type,
		Config:             req.Config,
		Icon:               req.Icon,
		Description:        req.Description,
		FeeBearer:          feeBearer,
		FeeType:            feeType,
		FeeFixedCents:      req.FeeFixedCents,
		FeePercent:         req.FeePercent,
		MinAmount:          req.MinAmount,
		MaxAmount:          req.MaxAmount,
		SettlementCurrency: settlementCurrency,
		ExchangeRateMarkup: req.ExchangeRateMarkup,
		Status:             status,
		SortOrder:          req.SortOrder,
	}

	if err := financeService.CreatePaymentChannel(&channel); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}

	c.JSON(http.StatusCreated, channel)
}

// UpdatePaymentChannelRequest 更新支付渠道请求
type UpdatePaymentChannelRequest struct {
	Name               string   `json:"name"`
	Type               string   `json:"type"`
	Config             string   `json:"config"`
	Icon               string   `json:"icon"`
	Description        string   `json:"description"`
	FeeBearer          string   `json:"fee_bearer"`
	FeeType            string   `json:"fee_type"`
	FeeFixedCents      *int64   `json:"fee_fixed_cents"`
	FeePercent         *float64 `json:"fee_percent"`
	MinAmount          *int64   `json:"min_amount"`
	MaxAmount          *int64   `json:"max_amount"`
	SettlementCurrency string   `json:"settlement_currency"`
	ExchangeRateMarkup *float64 `json:"exchange_rate_markup"`
	Status             string   `json:"status"`
	SortOrder          *int     `json:"sort_order"`
}

// UpdatePaymentChannel 更新支付渠道
func UpdatePaymentChannel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的渠道 ID"})
		return
	}

	var req UpdatePaymentChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Type != "" {
		updates["type"] = req.Type
	}
	if req.Config != "" {
		updates["config"] = req.Config
	}
	if req.Icon != "" {
		updates["icon"] = req.Icon
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.FeeBearer != "" {
		updates["fee_bearer"] = req.FeeBearer
	}
	if req.FeeType != "" {
		updates["fee_type"] = req.FeeType
	}
	if req.FeeFixedCents != nil {
		updates["fee_fixed_cents"] = *req.FeeFixedCents
	}
	if req.FeePercent != nil {
		updates["fee_percent"] = *req.FeePercent
	}
	if req.MinAmount != nil {
		updates["min_amount"] = *req.MinAmount
	}
	if req.MaxAmount != nil {
		updates["max_amount"] = *req.MaxAmount
	}
	if req.SettlementCurrency != "" {
		updates["settlement_currency"] = req.SettlementCurrency
	}
	if req.ExchangeRateMarkup != nil {
		updates["exchange_rate_markup"] = *req.ExchangeRateMarkup
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无有效更新字段"})
		return
	}

	if err := financeService.UpdatePaymentChannel(uint(id), updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// DeletePaymentChannel 删除支付渠道
func DeletePaymentChannel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的渠道 ID"})
		return
	}

	if err := financeService.DeletePaymentChannel(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// =================== 用户前台充值 ===================

// ListUserPaymentChannels 获取启用的支付渠道列表 (用户前台)
func ListUserPaymentChannels(c *gin.Context) {
	channels, err := financeService.ListEnabledPaymentChannels()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":  channels,
		"total": len(channels),
	})
}

// CreateRecharge 创建充值订单
func CreateRecharge(c *gin.Context) {
	var req struct {
		ChannelID uint   `json:"channel_id" binding:"required"`
		Amount    string `json:"amount" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	amountF, err := strconv.ParseFloat(req.Amount, 64)
	if err != nil || amountF <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "金额格式错误"})
		return
	}
	amountCents := int64(amountF*100 + 0.5)

	userID := c.GetUint("user_id")

	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	host := c.Request.Host
	notifyURL := fmt.Sprintf("%s://%s/api/v1/payment/notify/%d", scheme, host, req.ChannelID)
	returnURL := fmt.Sprintf("%s://%s/profile?tab=recharge", scheme, host)

	payResult, bill, err := financeService.CreateRechargeOrder(req.ChannelID, userID, amountCents, notifyURL, returnURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"bill_no":  bill.BillNo,
		"pay_type": payResult.Type,
		"pay_url":  payResult.Data,
		"status":   bill.Status,
	})
}

// PaymentNotify 支付回调 (无需认证)
func PaymentNotify(c *gin.Context) {
	channelID, err := strconv.ParseUint(c.Param("channel_id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "fail")
		return
	}

	params := make(map[string]string)
	if c.Request.Method == "GET" {
		for k, v := range c.Request.URL.Query() {
			if len(v) > 0 {
				params[k] = v[0]
			}
		}
	} else {
		c.Request.ParseForm()
		for k, v := range c.Request.PostForm {
			if len(v) > 0 {
				params[k] = v[0]
			}
		}
		for k, v := range c.Request.URL.Query() {
			if _, ok := params[k]; !ok && len(v) > 0 {
				params[k] = v[0]
			}
		}
	}

	if err := financeService.HandlePaymentNotify(uint(channelID), params); err != nil {
		c.String(http.StatusBadRequest, "fail")
		return
	}

	c.String(http.StatusOK, "success")
}

// GetRechargeStatus 查询充值订单状态
func GetRechargeStatus(c *gin.Context) {
	billNo := c.Param("bill_no")
	bill, err := financeService.GetBillByNo(billNo)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "订单不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"bill_no":      bill.BillNo,
		"status":       bill.Status,
		"amount_cents": bill.AmountCents,
	})
}
