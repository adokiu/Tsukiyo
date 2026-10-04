package commerce

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// CommerceService 销售系统服务
type CommerceService struct{}

// NewCommerceService 创建销售系统服务
func NewCommerceService() *CommerceService {
	return &CommerceService{}
}

// ==================== 商品分类 ====================

// ListCategories 获取分类树
func (s *CommerceService) ListCategories() ([]models.ProductCategory, error) {
	var categories []models.ProductCategory
	if err := db.DB.Order("sort ASC, created_at ASC").Find(&categories).Error; err != nil {
		return nil, err
	}
	return buildCategoryTree(categories), nil
}

func buildCategoryTree(flat []models.ProductCategory) []models.ProductCategory {
	nodeMap := make(map[uuid.UUID]*models.ProductCategory)
	for i := range flat {
		nodeMap[flat[i].ID] = &flat[i]
	}
	for i := range flat {
		if flat[i].ParentID != nil {
			if parent, ok := nodeMap[*flat[i].ParentID]; ok {
				parent.Children = append(parent.Children, flat[i])
			}
		}
	}
	var roots []models.ProductCategory
	for i := range flat {
		if flat[i].ParentID == nil {
			roots = append(roots, *nodeMap[flat[i].ID])
		}
	}
	return roots
}

// CreateCategoryRequest 创建分类请求
type CreateCategoryRequest struct {
	ParentID    string `json:"parent_id"`
	Name        string `json:"name" binding:"required"`
	Sort        int    `json:"sort"`
	Description string `json:"description"`
}

// CreateCategory 创建分类
func (s *CommerceService) CreateCategory(req CreateCategoryRequest) (*models.ProductCategory, error) {
	category := models.ProductCategory{
		Name:        req.Name,
		Sort:        req.Sort,
		Description: req.Description,
		Status:      models.ProductCategoryStatusActive,
	}

	// 两级分类：如果有 parent_id，检查父分类不能有父分类
	if req.ParentID != "" {
		parentID, err := uuid.Parse(req.ParentID)
		if err != nil {
			return nil, fmt.Errorf("无效的父分类 ID")
		}
		var parent models.ProductCategory
		if err := db.DB.Where("id = ?", parentID).First(&parent).Error; err != nil {
			return nil, fmt.Errorf("父分类不存在")
		}
		if parent.ParentID != nil {
			return nil, fmt.Errorf("最多支持两级分类")
		}
		category.ParentID = &parentID
	}

	if err := db.DB.Create(&category).Error; err != nil {
		return nil, err
	}
	return &category, nil
}

// UpdateCategoryRequest 更新分类请求
type UpdateCategoryRequest struct {
	Name        *string `json:"name"`
	Sort        *int    `json:"sort"`
	Status      *string `json:"status"`
	Description *string `json:"description"`
}

// UpdateCategory 更新分类
func (s *CommerceService) UpdateCategory(id uuid.UUID, req UpdateCategoryRequest) error {
	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Sort != nil {
		updates["sort"] = *req.Sort
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if len(updates) == 0 {
		return nil
	}
	updates["updated_at"] = time.Now()

	result := db.DB.Model(&models.ProductCategory{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteCategory 删除分类
func (s *CommerceService) DeleteCategory(id uuid.UUID) error {
	// 检查是否有子分类
	var childCount int64
	db.DB.Model(&models.ProductCategory{}).Where("parent_id = ?", id).Count(&childCount)
	if childCount > 0 {
		return fmt.Errorf("该分类下有子分类，无法删除")
	}

	// 检查是否有商品
	var productCount int64
	db.DB.Model(&models.Product{}).Where("category_id = ?", id).Count(&productCount)
	if productCount > 0 {
		return fmt.Errorf("该分类下有商品，无法删除")
	}

	result := db.DB.Where("id = ?", id).Delete(&models.ProductCategory{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ==================== 商品 ====================

// ListProductsRequest 列表请求
type ListProductsRequest struct {
	Page       int
	PageSize   int
	Search     string
	CategoryID string
	Status     string
}

// ListProducts 获取商品列表
func (s *CommerceService) ListProducts(req ListProductsRequest) ([]models.Product, int64, error) {
	query := db.DB.Model(&models.Product{})
	if req.Search != "" {
		query = query.Where("name ILIKE ?", "%"+req.Search+"%")
	}
	if req.CategoryID != "" {
		categoryID, err := uuid.Parse(req.CategoryID)
		if err == nil {
			query = query.Where("category_id = ?", categoryID)
		}
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}

	var total int64
	query.Count(&total)

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	var products []models.Product
	if err := query.Order("sort ASC, created_at DESC").Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Find(&products).Error; err != nil {
		return nil, 0, err
	}
	return products, total, nil
}

// GetProduct 获取商品详情
func (s *CommerceService) GetProduct(id uuid.UUID) (*models.Product, error) {
	var product models.Product
	if err := db.DB.Where("id = ?", id).First(&product).Error; err != nil {
		return nil, gorm.ErrRecordNotFound
	}
	return &product, nil
}

// CreateProductRequest 创建商品请求
type CreateProductRequest struct {
	CategoryID string          `json:"category_id"`
	Name       string          `json:"name" binding:"required"`
	Type       string          `json:"type" binding:"required"`
	Sort       int             `json:"sort"`
	NodeIDs    json.RawMessage `json:"node_ids"`

	VCPUMin            int             `json:"vcpu_min"`
	VCPUMax            int             `json:"vcpu_max"`
	VCPUCustomMode     string          `json:"vcpu_custom_mode"`
	VCPUTiers          json.RawMessage `json:"vcpu_tiers"`
	VCPUUnitPriceCents int64           `json:"vcpu_unit_price_cents"`

	MemoryMinMB          int             `json:"memory_min_mb"`
	MemoryMaxMB          int             `json:"memory_max_mb"`
	MemoryCustomMode     string          `json:"memory_custom_mode"`
	MemoryTiers          json.RawMessage `json:"memory_tiers"`
	MemoryUnitPriceCents int64           `json:"memory_unit_price_cents"`
	MemoryUnit           string          `json:"memory_unit"`

	DiskMinMB          int             `json:"disk_min_mb"`
	DiskMaxMB          int             `json:"disk_max_mb"`
	DiskCustomMode     string          `json:"disk_custom_mode"`
	DiskTiers          json.RawMessage `json:"disk_tiers"`
	DiskUnitPriceCents int64           `json:"disk_unit_price_cents"`
	DiskStoragePools   json.RawMessage `json:"disk_storage_pools"`

	DataDiskAllow          bool            `json:"data_disk_allow"`
	DataDiskMinMB          int             `json:"data_disk_min_mb"`
	DataDiskMaxMB          int             `json:"data_disk_max_mb"`
	DataDiskCustomMode     string          `json:"data_disk_custom_mode"`
	DataDiskTiers          json.RawMessage `json:"data_disk_tiers"`
	DataDiskUnitPriceCents int64           `json:"data_disk_unit_price_cents"`
	DataDiskStoragePools   json.RawMessage `json:"data_disk_storage_pools"`

	NetworkDownMinMbps        int             `json:"network_down_min_mbps"`
	NetworkDownMaxMbps        int             `json:"network_down_max_mbps"`
	NetworkDownCustomMode     string          `json:"network_down_custom_mode"`
	NetworkDownTiers          json.RawMessage `json:"network_down_tiers"`
	NetworkUpMinMbps          int             `json:"network_up_min_mbps"`
	NetworkUpMaxMbps          int             `json:"network_up_max_mbps"`
	NetworkUpCustomMode       string          `json:"network_up_custom_mode"`
	NetworkUpTiers            json.RawMessage `json:"network_up_tiers"`
	NetworkDownUnitPriceCents int64           `json:"network_down_unit_price_cents"`
	NetworkUpUnitPriceCents   int64           `json:"network_up_unit_price_cents"`

	TrafficMinGB          int             `json:"traffic_min_gb"`
	TrafficMaxGB          int             `json:"traffic_max_gb"`
	TrafficCalcMode       string          `json:"traffic_calc_mode"`
	TrafficCustomMode     string          `json:"traffic_custom_mode"`
	TrafficTiers          json.RawMessage `json:"traffic_tiers"`
	TrafficUnitPriceCents int64           `json:"traffic_unit_price_cents"`

	NodeBridges json.RawMessage `json:"node_bridges"`

	IPv4Mode                string          `json:"ipv4_mode"`
	IPv4EIPPools            json.RawMessage `json:"ipv4_eip_pools"`
	IPv4EIPMin              int             `json:"ipv4_eip_min"`
	IPv4EIPMax              int             `json:"ipv4_eip_max"`
	IPv4EIPCustomMode       string          `json:"ipv4_eip_custom_mode"`
	IPv4EIPTiers            json.RawMessage `json:"ipv4_eip_tiers"`
	IPv4EIPUnitPriceCents   int64           `json:"ipv4_eip_unit_price_cents"`
	IPv4EIPAllowChange      bool            `json:"ipv4_eip_allow_change"`
	IPv4EIPChangePriceCents int64           `json:"ipv4_eip_change_price_cents"`

	NATPortMin            int             `json:"nat_port_min"`
	NATPortMax            int             `json:"nat_port_max"`
	NATPortCustomMode     string          `json:"nat_port_custom_mode"`
	NATPortTiers          json.RawMessage `json:"nat_port_tiers"`
	NATPortUnitPriceCents int64           `json:"nat_port_unit_price_cents"`

	IPv6Enabled  bool            `json:"ipv6_enabled"`
	IPv6Configs  json.RawMessage `json:"ipv6_configs"`
	IPv6EIPPools json.RawMessage `json:"ipv6_eip_pools"`

	Stock int `json:"stock"`

	BasePriceCents     int64   `json:"base_price_cents"`
	MinPaymentPeriod   string  `json:"min_payment_period"`
	TrialEnabled       bool    `json:"trial_enabled"`
	TrialHours         int     `json:"trial_hours"`
	TrialPriceCents    int64   `json:"trial_price_cents"`
	QuarterlyDiscount  float64 `json:"quarterly_discount"`
	HalfYearlyDiscount float64 `json:"half_yearly_discount"`
	YearlyDiscount     float64 `json:"yearly_discount"`
}

// CreateProduct 创建商品
func (s *CommerceService) CreateProduct(req CreateProductRequest) (*models.Product, error) {
	product := models.Product{
		Name:    req.Name,
		Type:    models.ProductType(req.Type),
		Status:  models.ProductStatusActive,
		Sort:    req.Sort,
		Stock:   req.Stock,
		NodeIDs: ensureJSONArray(req.NodeIDs),

		VCPUMin:            req.VCPUMin,
		VCPUMax:            req.VCPUMax,
		VCPUCustomMode:     models.CustomMode(req.VCPUCustomMode),
		VCPUTiers:          ensureJSONArray(req.VCPUTiers),
		VCPUUnitPriceCents: req.VCPUUnitPriceCents,

		MemoryMinMB:          req.MemoryMinMB,
		MemoryMaxMB:          req.MemoryMaxMB,
		MemoryCustomMode:     models.CustomMode(req.MemoryCustomMode),
		MemoryTiers:          ensureJSONArray(req.MemoryTiers),
		MemoryUnitPriceCents: req.MemoryUnitPriceCents,
		MemoryUnit:           models.MemoryUnit(req.MemoryUnit),

		DiskMinMB:          req.DiskMinMB,
		DiskMaxMB:          req.DiskMaxMB,
		DiskCustomMode:     models.CustomMode(req.DiskCustomMode),
		DiskTiers:          ensureJSONArray(req.DiskTiers),
		DiskUnitPriceCents: req.DiskUnitPriceCents,
		DiskStoragePools:   ensureJSONObject(req.DiskStoragePools),

		DataDiskAllow:          req.DataDiskAllow,
		DataDiskMinMB:          req.DataDiskMinMB,
		DataDiskMaxMB:          req.DataDiskMaxMB,
		DataDiskCustomMode:     models.CustomMode(req.DataDiskCustomMode),
		DataDiskTiers:          ensureJSONArray(req.DataDiskTiers),
		DataDiskUnitPriceCents: req.DataDiskUnitPriceCents,
		DataDiskStoragePools:   ensureJSONObject(req.DataDiskStoragePools),

		NetworkDownMinMbps:        req.NetworkDownMinMbps,
		NetworkDownMaxMbps:        req.NetworkDownMaxMbps,
		NetworkDownCustomMode:     models.CustomMode(req.NetworkDownCustomMode),
		NetworkDownTiers:          ensureJSONArray(req.NetworkDownTiers),
		NetworkUpMinMbps:          req.NetworkUpMinMbps,
		NetworkUpMaxMbps:          req.NetworkUpMaxMbps,
		NetworkUpCustomMode:       models.CustomMode(req.NetworkUpCustomMode),
		NetworkUpTiers:            ensureJSONArray(req.NetworkUpTiers),
		NetworkDownUnitPriceCents: req.NetworkDownUnitPriceCents,
		NetworkUpUnitPriceCents:   req.NetworkUpUnitPriceCents,

		TrafficMinGB:          req.TrafficMinGB,
		TrafficMaxGB:          req.TrafficMaxGB,
		TrafficCalcMode:       models.TrafficCalcMode(req.TrafficCalcMode),
		TrafficCustomMode:     models.CustomMode(req.TrafficCustomMode),
		TrafficTiers:          ensureJSONArray(req.TrafficTiers),
		TrafficUnitPriceCents: req.TrafficUnitPriceCents,

		NodeBridges: ensureJSONObject(req.NodeBridges),

		IPv4Mode:                models.ProductIPv4Mode(req.IPv4Mode),
		IPv4EIPPools:            ensureJSONObject(req.IPv4EIPPools),
		IPv4EIPMin:              req.IPv4EIPMin,
		IPv4EIPMax:              req.IPv4EIPMax,
		IPv4EIPCustomMode:       models.CustomMode(req.IPv4EIPCustomMode),
		IPv4EIPTiers:            ensureJSONArray(req.IPv4EIPTiers),
		IPv4EIPUnitPriceCents:   req.IPv4EIPUnitPriceCents,
		IPv4EIPAllowChange:      req.IPv4EIPAllowChange,
		IPv4EIPChangePriceCents: req.IPv4EIPChangePriceCents,

		NATPortMin:            req.NATPortMin,
		NATPortMax:            req.NATPortMax,
		NATPortCustomMode:     models.CustomMode(req.NATPortCustomMode),
		NATPortTiers:          ensureJSONArray(req.NATPortTiers),
		NATPortUnitPriceCents: req.NATPortUnitPriceCents,

		IPv6Enabled:  req.IPv6Enabled,
		IPv6Configs:  ensureJSONArray(req.IPv6Configs),
		IPv6EIPPools: ensureJSONObject(req.IPv6EIPPools),

		BasePriceCents:     req.BasePriceCents,
		MinPaymentPeriod:   models.PaymentPeriod(req.MinPaymentPeriod),
		TrialEnabled:       req.TrialEnabled,
		TrialHours:         req.TrialHours,
		TrialPriceCents:    req.TrialPriceCents,
		QuarterlyDiscount:  ensureDiscount(req.QuarterlyDiscount),
		HalfYearlyDiscount: ensureDiscount(req.HalfYearlyDiscount),
		YearlyDiscount:     ensureDiscount(req.YearlyDiscount),
	}

	if req.CategoryID != "" {
		categoryID, err := uuid.Parse(req.CategoryID)
		if err == nil {
			product.CategoryID = &categoryID
		}
	}

	if product.Type == "" {
		product.Type = models.ProductTypeContainer
	}
	if product.IPv4Mode == "" {
		product.IPv4Mode = models.ProductIPv4ModeNAT
	}
	if product.TrafficCalcMode == "" {
		product.TrafficCalcMode = models.TrafficCalcBoth
	}
	if product.MinPaymentPeriod == "" {
		product.MinPaymentPeriod = models.PaymentPeriodMonthly
	}

	if err := db.DB.Create(&product).Error; err != nil {
		zap.L().Error("创建商品失败", zap.Error(err))
		return nil, err
	}
	return &product, nil
}

// UpdateProductRequest 更新商品请求
type UpdateProductRequest struct {
	CategoryID *string         `json:"category_id"`
	Name       *string         `json:"name"`
	Type       *string         `json:"type"`
	Status     *string         `json:"status"`
	Sort       *int            `json:"sort"`
	NodeIDs    json.RawMessage `json:"node_ids"`

	VCPUMin            *int            `json:"vcpu_min"`
	VCPUMax            *int            `json:"vcpu_max"`
	VCPUCustomMode     *string         `json:"vcpu_custom_mode"`
	VCPUTiers          json.RawMessage `json:"vcpu_tiers"`
	VCPUUnitPriceCents *int64          `json:"vcpu_unit_price_cents"`

	MemoryMinMB          *int            `json:"memory_min_mb"`
	MemoryMaxMB          *int            `json:"memory_max_mb"`
	MemoryCustomMode     *string         `json:"memory_custom_mode"`
	MemoryTiers          json.RawMessage `json:"memory_tiers"`
	MemoryUnitPriceCents *int64          `json:"memory_unit_price_cents"`
	MemoryUnit           *string         `json:"memory_unit"`

	DiskMinMB          *int            `json:"disk_min_mb"`
	DiskMaxMB          *int            `json:"disk_max_mb"`
	DiskCustomMode     *string         `json:"disk_custom_mode"`
	DiskTiers          json.RawMessage `json:"disk_tiers"`
	DiskUnitPriceCents *int64          `json:"disk_unit_price_cents"`
	DiskStoragePools   json.RawMessage `json:"disk_storage_pools"`

	DataDiskAllow          *bool           `json:"data_disk_allow"`
	DataDiskMinMB          *int            `json:"data_disk_min_mb"`
	DataDiskMaxMB          *int            `json:"data_disk_max_mb"`
	DataDiskCustomMode     *string         `json:"data_disk_custom_mode"`
	DataDiskTiers          json.RawMessage `json:"data_disk_tiers"`
	DataDiskUnitPriceCents *int64          `json:"data_disk_unit_price_cents"`
	DataDiskStoragePools   json.RawMessage `json:"data_disk_storage_pools"`

	NetworkDownMinMbps        *int            `json:"network_down_min_mbps"`
	NetworkDownMaxMbps        *int            `json:"network_down_max_mbps"`
	NetworkDownCustomMode     *string         `json:"network_down_custom_mode"`
	NetworkDownTiers          json.RawMessage `json:"network_down_tiers"`
	NetworkUpMinMbps          *int            `json:"network_up_min_mbps"`
	NetworkUpMaxMbps          *int            `json:"network_up_max_mbps"`
	NetworkUpCustomMode       *string         `json:"network_up_custom_mode"`
	NetworkUpTiers            json.RawMessage `json:"network_up_tiers"`
	NetworkDownUnitPriceCents *int64          `json:"network_down_unit_price_cents"`
	NetworkUpUnitPriceCents   *int64          `json:"network_up_unit_price_cents"`

	TrafficMinGB          *int            `json:"traffic_min_gb"`
	TrafficMaxGB          *int            `json:"traffic_max_gb"`
	TrafficCalcMode       *string         `json:"traffic_calc_mode"`
	TrafficCustomMode     *string         `json:"traffic_custom_mode"`
	TrafficTiers          json.RawMessage `json:"traffic_tiers"`
	TrafficUnitPriceCents *int64          `json:"traffic_unit_price_cents"`

	NodeBridges json.RawMessage `json:"node_bridges"`

	IPv4Mode                *string         `json:"ipv4_mode"`
	IPv4EIPPools            json.RawMessage `json:"ipv4_eip_pools"`
	IPv4EIPMin              *int            `json:"ipv4_eip_min"`
	IPv4EIPMax              *int            `json:"ipv4_eip_max"`
	IPv4EIPCustomMode       *string         `json:"ipv4_eip_custom_mode"`
	IPv4EIPTiers            json.RawMessage `json:"ipv4_eip_tiers"`
	IPv4EIPUnitPriceCents   *int64          `json:"ipv4_eip_unit_price_cents"`
	IPv4EIPAllowChange      *bool           `json:"ipv4_eip_allow_change"`
	IPv4EIPChangePriceCents *int64          `json:"ipv4_eip_change_price_cents"`

	NATPortMin            *int            `json:"nat_port_min"`
	NATPortMax            *int            `json:"nat_port_max"`
	NATPortCustomMode     *string         `json:"nat_port_custom_mode"`
	NATPortTiers          json.RawMessage `json:"nat_port_tiers"`
	NATPortUnitPriceCents *int64          `json:"nat_port_unit_price_cents"`

	IPv6Enabled  *bool           `json:"ipv6_enabled"`
	IPv6Configs  json.RawMessage `json:"ipv6_configs"`
	IPv6EIPPools json.RawMessage `json:"ipv6_eip_pools"`

	Stock *int `json:"stock"`

	BasePriceCents     *int64   `json:"base_price_cents"`
	MinPaymentPeriod   *string  `json:"min_payment_period"`
	TrialEnabled       *bool    `json:"trial_enabled"`
	TrialHours         *int     `json:"trial_hours"`
	TrialPriceCents    *int64   `json:"trial_price_cents"`
	QuarterlyDiscount  *float64 `json:"quarterly_discount"`
	HalfYearlyDiscount *float64 `json:"half_yearly_discount"`
	YearlyDiscount     *float64 `json:"yearly_discount"`
}

// UpdateProduct 更新商品
func (s *CommerceService) UpdateProduct(id uuid.UUID, req UpdateProductRequest) error {
	updates := make(map[string]interface{})

	if req.CategoryID != nil {
		if *req.CategoryID == "" {
			updates["category_id"] = nil
		} else {
			categoryID, err := uuid.Parse(*req.CategoryID)
			if err == nil {
				updates["category_id"] = categoryID
			}
		}
	}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Type != nil {
		updates["type"] = *req.Type
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.Sort != nil {
		updates["sort"] = *req.Sort
	}
	if len(req.NodeIDs) > 0 {
		updates["node_ids"] = ensureJSONArray(req.NodeIDs)
	}

	if req.VCPUMin != nil {
		updates["vcpu_min"] = *req.VCPUMin
	}
	if req.VCPUMax != nil {
		updates["vcpu_max"] = *req.VCPUMax
	}
	if req.VCPUCustomMode != nil {
		updates["vcpu_custom_mode"] = *req.VCPUCustomMode
	}
	if len(req.VCPUTiers) > 0 {
		updates["vcpu_tiers"] = ensureJSONArray(req.VCPUTiers)
	}
	if req.VCPUUnitPriceCents != nil {
		updates["vcpu_unit_price_cents"] = *req.VCPUUnitPriceCents
	}

	if req.MemoryMinMB != nil {
		updates["memory_min_mb"] = *req.MemoryMinMB
	}
	if req.MemoryMaxMB != nil {
		updates["memory_max_mb"] = *req.MemoryMaxMB
	}
	if req.MemoryCustomMode != nil {
		updates["memory_custom_mode"] = *req.MemoryCustomMode
	}
	if len(req.MemoryTiers) > 0 {
		updates["memory_tiers"] = ensureJSONArray(req.MemoryTiers)
	}
	if req.MemoryUnitPriceCents != nil {
		updates["memory_unit_price_cents"] = *req.MemoryUnitPriceCents
	}
	if req.MemoryUnit != nil {
		updates["memory_unit"] = *req.MemoryUnit
	}

	if req.DiskMinMB != nil {
		updates["disk_min_mb"] = *req.DiskMinMB
	}
	if req.DiskMaxMB != nil {
		updates["disk_max_mb"] = *req.DiskMaxMB
	}
	if req.DiskCustomMode != nil {
		updates["disk_custom_mode"] = *req.DiskCustomMode
	}
	if len(req.DiskTiers) > 0 {
		updates["disk_tiers"] = ensureJSONArray(req.DiskTiers)
	}
	if req.DiskUnitPriceCents != nil {
		updates["disk_unit_price_cents"] = *req.DiskUnitPriceCents
	}
	if len(req.DiskStoragePools) > 0 {
		updates["disk_storage_pools"] = ensureJSONObject(req.DiskStoragePools)
	}

	if req.DataDiskAllow != nil {
		updates["data_disk_allow"] = *req.DataDiskAllow
	}
	if req.DataDiskMinMB != nil {
		updates["data_disk_min_mb"] = *req.DataDiskMinMB
	}
	if req.DataDiskMaxMB != nil {
		updates["data_disk_max_mb"] = *req.DataDiskMaxMB
	}
	if req.DataDiskCustomMode != nil {
		updates["data_disk_custom_mode"] = *req.DataDiskCustomMode
	}
	if len(req.DataDiskTiers) > 0 {
		updates["data_disk_tiers"] = ensureJSONArray(req.DataDiskTiers)
	}
	if req.DataDiskUnitPriceCents != nil {
		updates["data_disk_unit_price_cents"] = *req.DataDiskUnitPriceCents
	}
	if len(req.DataDiskStoragePools) > 0 {
		updates["data_disk_storage_pools"] = ensureJSONObject(req.DataDiskStoragePools)
	}

	if req.NetworkDownMinMbps != nil {
		updates["network_down_min_mbps"] = *req.NetworkDownMinMbps
	}
	if req.NetworkDownMaxMbps != nil {
		updates["network_down_max_mbps"] = *req.NetworkDownMaxMbps
	}
	if req.NetworkDownCustomMode != nil {
		updates["network_down_custom_mode"] = *req.NetworkDownCustomMode
	}
	if len(req.NetworkDownTiers) > 0 {
		updates["network_down_tiers"] = ensureJSONArray(req.NetworkDownTiers)
	}
	if req.NetworkUpMinMbps != nil {
		updates["network_up_min_mbps"] = *req.NetworkUpMinMbps
	}
	if req.NetworkUpMaxMbps != nil {
		updates["network_up_max_mbps"] = *req.NetworkUpMaxMbps
	}
	if req.NetworkUpCustomMode != nil {
		updates["network_up_custom_mode"] = *req.NetworkUpCustomMode
	}
	if len(req.NetworkUpTiers) > 0 {
		updates["network_up_tiers"] = ensureJSONArray(req.NetworkUpTiers)
	}
	if req.NetworkDownUnitPriceCents != nil {
		updates["network_down_unit_price_cents"] = *req.NetworkDownUnitPriceCents
	}
	if req.NetworkUpUnitPriceCents != nil {
		updates["network_up_unit_price_cents"] = *req.NetworkUpUnitPriceCents
	}

	if req.TrafficMinGB != nil {
		updates["traffic_min_gb"] = *req.TrafficMinGB
	}
	if req.TrafficMaxGB != nil {
		updates["traffic_max_gb"] = *req.TrafficMaxGB
	}
	if req.TrafficCalcMode != nil {
		updates["traffic_calc_mode"] = *req.TrafficCalcMode
	}
	if req.TrafficCustomMode != nil {
		updates["traffic_custom_mode"] = *req.TrafficCustomMode
	}
	if len(req.TrafficTiers) > 0 {
		updates["traffic_tiers"] = ensureJSONArray(req.TrafficTiers)
	}
	if req.TrafficUnitPriceCents != nil {
		updates["traffic_unit_price_cents"] = *req.TrafficUnitPriceCents
	}

	if len(req.NodeBridges) > 0 {
		updates["node_bridges"] = ensureJSONObject(req.NodeBridges)
	}

	if req.IPv4Mode != nil {
		updates["ipv4_mode"] = *req.IPv4Mode
	}
	if len(req.IPv4EIPPools) > 0 {
		updates["ipv4_eip_pools"] = ensureJSONObject(req.IPv4EIPPools)
	}
	if req.IPv4EIPMin != nil {
		updates["ipv4_eip_min"] = *req.IPv4EIPMin
	}
	if req.IPv4EIPMax != nil {
		updates["ipv4_eip_max"] = *req.IPv4EIPMax
	}
	if req.IPv4EIPCustomMode != nil {
		updates["ipv4_eip_custom_mode"] = *req.IPv4EIPCustomMode
	}
	if len(req.IPv4EIPTiers) > 0 {
		updates["ipv4_eip_tiers"] = ensureJSONArray(req.IPv4EIPTiers)
	}
	if req.IPv4EIPUnitPriceCents != nil {
		updates["ipv4_eip_unit_price_cents"] = *req.IPv4EIPUnitPriceCents
	}
	if req.IPv4EIPAllowChange != nil {
		updates["ipv4_eip_allow_change"] = *req.IPv4EIPAllowChange
	}
	if req.IPv4EIPChangePriceCents != nil {
		updates["ipv4_eip_change_price_cents"] = *req.IPv4EIPChangePriceCents
	}

	if req.NATPortMin != nil {
		updates["nat_port_min"] = *req.NATPortMin
	}
	if req.NATPortMax != nil {
		updates["nat_port_max"] = *req.NATPortMax
	}
	if req.NATPortCustomMode != nil {
		updates["nat_port_custom_mode"] = *req.NATPortCustomMode
	}
	if len(req.NATPortTiers) > 0 {
		updates["nat_port_tiers"] = ensureJSONArray(req.NATPortTiers)
	}
	if req.NATPortUnitPriceCents != nil {
		updates["nat_port_unit_price_cents"] = *req.NATPortUnitPriceCents
	}

	if req.IPv6Enabled != nil {
		updates["ipv6_enabled"] = *req.IPv6Enabled
	}
	if len(req.IPv6Configs) > 0 {
		updates["ipv6_configs"] = ensureJSONArray(req.IPv6Configs)
	}
	if len(req.IPv6EIPPools) > 0 {
		updates["ipv6_eip_pools"] = ensureJSONObject(req.IPv6EIPPools)
	}

	if req.Stock != nil {
		updates["stock"] = *req.Stock
	}

	if req.BasePriceCents != nil {
		updates["base_price_cents"] = *req.BasePriceCents
	}
	if req.MinPaymentPeriod != nil {
		updates["min_payment_period"] = *req.MinPaymentPeriod
	}
	if req.TrialEnabled != nil {
		updates["trial_enabled"] = *req.TrialEnabled
	}
	if req.TrialHours != nil {
		updates["trial_hours"] = *req.TrialHours
	}
	if req.TrialPriceCents != nil {
		updates["trial_price_cents"] = *req.TrialPriceCents
	}
	if req.QuarterlyDiscount != nil {
		updates["quarterly_discount"] = ensureDiscount(*req.QuarterlyDiscount)
	}
	if req.HalfYearlyDiscount != nil {
		updates["half_yearly_discount"] = ensureDiscount(*req.HalfYearlyDiscount)
	}
	if req.YearlyDiscount != nil {
		updates["yearly_discount"] = ensureDiscount(*req.YearlyDiscount)
	}

	if len(updates) == 0 {
		return nil
	}
	updates["updated_at"] = time.Now()

	result := db.DB.Model(&models.Product{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteProduct 删除商品
func (s *CommerceService) DeleteProduct(id uuid.UUID) error {
	result := db.DB.Where("id = ?", id).Delete(&models.Product{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ==================== 用户端商品接口 ====================

// ListUserProducts 获取上架商品列表（仅返回 active 状态）
func (s *CommerceService) ListUserProducts(categoryID string) ([]models.Product, error) {
	query := db.DB.Model(&models.Product{}).Where("status = ?", models.ProductStatusActive)
	if categoryID != "" {
		cid, err := uuid.Parse(categoryID)
		if err == nil {
			query = query.Where("category_id = ?", cid)
		}
	}
	var products []models.Product
	if err := query.Order("sort ASC, created_at DESC").Find(&products).Error; err != nil {
		return nil, err
	}
	return products, nil
}

// GetUserProduct 获取单个上架商品详情
func (s *CommerceService) GetUserProduct(id uuid.UUID) (*models.Product, error) {
	var product models.Product
	if err := db.DB.Where("id = ? AND status = ?", id, models.ProductStatusActive).First(&product).Error; err != nil {
		return nil, err
	}
	return &product, nil
}

// ListUserCategories 获取活跃分类列表（仅返回 active 状态）
func (s *CommerceService) ListUserCategories() ([]models.ProductCategory, error) {
	var categories []models.ProductCategory
	if err := db.DB.Where("status = ?", models.ProductCategoryStatusActive).Order("sort ASC, created_at ASC").Find(&categories).Error; err != nil {
		return nil, err
	}
	return buildCategoryTree(categories), nil
}

// ==================== 辅助函数 ====================

func ensureJSONArray(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage("[]")
	}
	return raw
}

func ensureJSONObject(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage("{}")
	}
	return raw
}

func ensureDiscount(v float64) float64 {
	if v <= 0 {
		return 1.00
	}
	return v
}
