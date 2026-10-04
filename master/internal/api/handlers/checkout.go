package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"tsukiyo/master/internal/api/middleware"
	"tsukiyo/master/internal/service/commerce"
)

// ==================== 购物车 ====================

// AddToCart 添加购物车
func AddToCart(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 401, "error": "未认证"})
		return
	}

	var req commerce.AddCartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	item, err := commerceService.AddToCart(userID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, item)
}

// ListCart 获取购物车列表
func ListCart(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 401, "error": "未认证"})
		return
	}

	items, err := commerceService.ListCart(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// UpdateCart 更新购物车项
func UpdateCart(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 401, "error": "未认证"})
		return
	}

	itemID := c.Param("id")
	var req commerce.UpdateCartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if err := commerceService.UpdateCart(userID, itemID, req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// RemoveFromCart 从购物车移除
func RemoveFromCart(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 401, "error": "未认证"})
		return
	}

	itemID := c.Param("id")
	if err := commerceService.RemoveFromCart(userID, itemID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ClearCart 清空购物车
func ClearCart(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 401, "error": "未认证"})
		return
	}

	if err := commerceService.ClearCart(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "清空失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已清空"})
}

// ==================== 优惠码 ====================

// ValidateCoupon 验证优惠码
func ValidateCoupon(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 401, "error": "未认证"})
		return
	}

	var req commerce.ValidateCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	result, err := commerceService.ValidateCoupon(userID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// ==================== 结算 ====================

// Checkout 结算创建订单
func Checkout(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 401, "error": "未认证"})
		return
	}
	username := c.GetString("username")

	var req commerce.CheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	result, err := commerceService.Checkout(userID, username, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// PayOrder 支付订单
func PayOrder(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 401, "error": "未认证"})
		return
	}
	username := c.GetString("username")

	var req commerce.PayOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	notifyURLBase := fmt.Sprintf("%s://%s/api/v1", scheme, c.Request.Host)
	returnURL := fmt.Sprintf("%s://%s/payment/result", scheme, c.Request.Host)

	result, err := commerceService.PayOrder(userID, username, req, notifyURLBase, returnURL)
	if err != nil {
		zap.L().Error("支付订单失败", zap.Uint("user_id", userID), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetOrder 获取订单详情
func GetOrder(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 401, "error": "未认证"})
		return
	}

	orderID := c.Param("id")
	order, err := commerceService.GetOrder(userID, orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, order)
}

// ListOrders 获取订单列表
func ListOrders(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 401, "error": "未认证"})
		return
	}

	orders, err := commerceService.ListOrders(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": orders})
}

// ==================== 账单与流水 ====================

// ListUserBills 获取用户账单列表
func ListUserBills(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 401, "error": "未认证"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	bills, total, err := financeService.ListUserBills(userID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": bills, "total": total, "page": page, "page_size": pageSize})
}

// GetUserBill 获取用户账单详情
func GetUserBill(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 401, "error": "未认证"})
		return
	}

	billID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的账单 ID"})
		return
	}

	bill, err := financeService.GetUserBill(userID, uint(billID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, bill)
}

// ListUserWalletTransactions 获取用户钱包流水
func ListUserWalletTransactions(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 401, "error": "未认证"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	txs, total, err := financeService.ListUserWalletTransactions(userID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": txs, "total": total, "page": page, "page_size": pageSize})
}

// GetInvoice 获取发票数据
func GetInvoice(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 401, "error": "未认证"})
		return
	}

	orderID := c.Param("id")
	invoice, err := commerceService.GetInvoice(userID, orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, invoice)
}
