package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"tsukiyo/master/internal/api/middleware"
	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/service/commerce"
	"tsukiyo/master/internal/service/infrastructure"
)

var commerceService *commerce.CommerceService

// InitCommerceService 初始化销售系统服务
func InitCommerceService(svc *commerce.CommerceService) {
	commerceService = svc
}

// ==================== 商品分类 ====================

// ListProductCategories 获取分类树
func ListProductCategories(c *gin.Context) {
	categories, err := commerceService.ListCategories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询分类失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": categories})
}

// CreateProductCategory 创建分类
func CreateProductCategory(c *gin.Context) {
	var req commerce.CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	category, err := commerceService.CreateCategory(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, category)
}

// UpdateProductCategory 更新分类
func UpdateProductCategory(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}
	var req commerce.UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	if err := commerceService.UpdateCategory(id, req); err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "分类不存在"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// DeleteProductCategory 删除分类
func DeleteProductCategory(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}
	if err := commerceService.DeleteCategory(id); err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "分类不存在"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ==================== 商品 ====================

// ListProducts 获取商品列表
func ListProducts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	req := commerce.ListProductsRequest{
		Page:       page,
		PageSize:   pageSize,
		Search:     c.Query("search"),
		CategoryID: c.Query("category_id"),
		Status:     c.Query("status"),
	}

	products, total, err := commerceService.ListProducts(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询商品失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":      products,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetProduct 获取商品详情
func GetProduct(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}
	product, err := commerceService.GetProduct(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "商品不存在"})
		return
	}
	c.JSON(http.StatusOK, product)
}

// CreateProduct 创建商品
func CreateProduct(c *gin.Context) {
	var req commerce.CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	product, err := commerceService.CreateProduct(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, product)
}

// UpdateProduct 更新商品
func UpdateProduct(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}
	var req commerce.UpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	if err := commerceService.UpdateProduct(id, req); err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "商品不存在"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// DeleteProduct 删除商品
func DeleteProduct(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}
	if err := commerceService.DeleteProduct(id); err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "商品不存在"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ==================== 用户端商品接口 ====================

// ListUserProducts 用户端获取上架商品列表
func ListUserProducts(c *gin.Context) {
	products, err := commerceService.ListUserProducts(c.Query("category_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询商品失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": products})
}

// GetUserProduct 用户端获取商品详情
func GetUserProduct(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}
	product, err := commerceService.GetUserProduct(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "商品不存在"})
		return
	}
	c.JSON(http.StatusOK, product)
}

// ListUserProductCategories 用户端获取活跃分类列表
func ListUserProductCategories(c *gin.Context) {
	categories, err := commerceService.ListUserCategories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询分类失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": categories})
}

// UserCreateOrder 用户端商品下单创建实例
func UserCreateOrder(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 401, "error": "未认证"})
		return
	}
	username := c.GetString("username")

	var req commerce.UserOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	result, err := commerceService.CreateUserOrder(userID, username, req)
	if err != nil {
		zap.L().Error("用户下单失败", zap.Uint("user_id", userID), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// ListProductNodeImages 用户端获取商品可用节点的镜像列表
func ListProductNodeImages(c *gin.Context) {
	productID := c.Param("id")
	pid, err := uuid.Parse(productID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的商品 ID"})
		return
	}
	var product models.Product
	if err := db.DB.Where("id = ? AND status = ?", pid, models.ProductStatusActive).First(&product).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "商品不存在"})
		return
	}

	var nodeIDs []string
	json.Unmarshal(product.NodeIDs, &nodeIDs)
	if len(nodeIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}

	imageType := ""
	if product.Type == "container" {
		imageType = "container"
	} else {
		imageType = "virtual-machine"
	}

	// 汇总所有关联节点的镜像
	allImages := []infrastructure.InstalledImage{}
	for _, nid := range nodeIDs {
		nodeUUID, err := uuid.Parse(nid)
		if err != nil {
			continue
		}
		images, err := imageService.ListInstalledImages(nodeUUID, imageType)
		if err != nil {
			continue
		}
		allImages = append(allImages, images...)
	}
	c.JSON(http.StatusOK, gin.H{"data": allImages})
}

// ListProductNodeBridges 用户端获取商品可用节点的网桥列表
func ListProductNodeBridges(c *gin.Context) {
	productID := c.Param("id")
	pid, err := uuid.Parse(productID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的商品 ID"})
		return
	}
	var product models.Product
	if err := db.DB.Where("id = ? AND status = ?", pid, models.ProductStatusActive).First(&product).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "商品不存在"})
		return
	}

	var nodeIDs []string
	json.Unmarshal(product.NodeIDs, &nodeIDs)
	if len(nodeIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}

	// 汇总所有关联节点的网桥
	allBridges := []models.Bridge{}
	for _, nid := range nodeIDs {
		var bridges []models.Bridge
		db.DB.Where("node_id = ? AND status = ?", nid, "active").Find(&bridges)
		allBridges = append(allBridges, bridges...)
	}
	c.JSON(http.StatusOK, gin.H{"data": allBridges})
}
