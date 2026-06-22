package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"tsukiyo/master/internal/service/infrastructure"
)

var imageService *infrastructure.ImageService

// InitImageService 初始化镜像服务
func InitImageService(svc *infrastructure.ImageService) {
	imageService = svc
}

// ImageInfo 镜像信息（含节点下载状态）
type ImageInfo = infrastructure.ImageInfo

// ListImages 获取镜像列表
func ListImages(c *gin.Context) {
	nodeIDStr := c.Query("node_id")
	q := ParseListQuery(c)

	// 镜像列表前端无分页控件，不传 per_page 时返回全部
	if c.Query("per_page") == "" {
		q.PerPage = 0
	}

	if nodeIDStr == "" {
		c.JSON(http.StatusOK, ListResponse{
			Data:    []ImageInfo{},
			Total:   0,
			Page:    q.Page,
			PerPage: q.PerPage,
		})
		return
	}

	nodeID, err := uuid.Parse(nodeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 node_id"})
		return
	}

	filterType := q.Filters["type"]
	filterArch := q.Filters["arch"]
	filterDistro := q.Filters["distro"]
	downloadedOnly := q.Filters["downloaded"] == "true"

	result, err := imageService.ListImages(nodeID, q.Search, filterType, filterArch, filterDistro, downloadedOnly, q.Page, q.PerPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, ListResponse{
		Data:    result,
		Total:   int64(len(result)),
		Page:    q.Page,
		PerPage: q.PerPage,
	})
}

// DownloadImage 下载镜像
func DownloadImage(c *gin.Context) {
	var req struct {
		NodeID   string `json:"node_id" binding:"required"`
		ImageKey string `json:"image_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id 和 image_key 必填"})
		return
	}

	nodeID, err := uuid.Parse(req.NodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 node_id"})
		return
	}

	userID, _ := c.Get("user_id")
	taskID, err := imageService.DownloadImage(nodeID, req.ImageKey, userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"task_id": taskID.String(), "image_key": req.ImageKey,
		"node_id": req.NodeID, "message": "下载任务已下发",
	})
	BroadcastDataRefresh("images", req.NodeID)
}

// GetImageProgress 查询镜像下载进度
func GetImageProgress(c *gin.Context) {
	imageKey := c.Query("image_key")
	if imageKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image_key 必填"})
		return
	}
	nodeIDStr := c.Query("node_id")
	if nodeIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id 必填"})
		return
	}

	nodeID, err := uuid.Parse(nodeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 node_id"})
		return
	}

	downloaded, stage, progress, downloadedBytes, totalBytes, speedBps, errMsg := imageService.GetImageProgress(nodeID, imageKey)

	c.JSON(http.StatusOK, gin.H{
		"image_key": imageKey, "node_id": nodeIDStr,
		"downloaded": downloaded, "stage": stage, "progress": progress,
		"downloaded_bytes": downloadedBytes, "total_bytes": totalBytes,
		"speed_bps": speedBps, "error": errMsg,
	})
}

// CancelImageDownload 取消镜像下载
func CancelImageDownload(c *gin.Context) {
	var req struct {
		NodeID   string `json:"node_id" binding:"required"`
		ImageKey string `json:"image_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id 和 image_key 必填"})
		return
	}

	nodeID, err := uuid.Parse(req.NodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 node_id"})
		return
	}

	userID, _ := c.Get("user_id")
	_, err = imageService.CancelImageDownload(nodeID, req.ImageKey, userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "取消任务已下发"})
	BroadcastDataRefresh("images", req.NodeID)
}

// DeleteImage 删除镜像
func DeleteImage(c *gin.Context) {
	var req struct {
		NodeID   string `json:"node_id" binding:"required"`
		ImageKey string `json:"image_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id 和 image_key 必填"})
		return
	}

	nodeID, err := uuid.Parse(req.NodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 node_id"})
		return
	}

	userID, _ := c.Get("user_id")
	taskID, err := imageService.DeleteImage(nodeID, req.ImageKey, userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"task_id": taskID.String(), "image_key": req.ImageKey,
		"node_id": req.NodeID, "message": "删除任务已下发",
	})
	BroadcastDataRefresh("images", req.NodeID)
}

// ListRemoteImages 获取远程镜像列表（异步任务）
func ListRemoteImages(c *gin.Context) {
	var req struct {
		NodeID string `json:"node_id" binding:"required"`
		Remote string `json:"remote"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id 必填"})
		return
	}

	nodeID, err := uuid.Parse(req.NodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 node_id"})
		return
	}

	taskID, err := imageService.ListRemoteImages(nodeID, req.Remote)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"task_id": taskID.String(), "message": "获取远程镜像列表任务已下发"})
}

// GetImageSource 获取当前镜像源
func GetImageSource(c *gin.Context) {
	source, err := imageService.GetImageSource()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"source": source})
}

// UpdateImageSource 切换镜像源并刷新缓存
func UpdateImageSource(c *gin.Context) {
	var req struct {
		Source string `json:"source" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source 必填"})
		return
	}
	if err := imageService.SetImageSource(req.Source); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "镜像源已切换，缓存已刷新"})
	BroadcastDataRefresh("images", "")
}

// RefreshImageCache 刷新镜像缓存
func RefreshImageCache(c *gin.Context) {
	nodeIDStr := c.Query("node_id")
	if nodeIDStr != "" {
		nodeID, err := uuid.Parse(nodeIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 node_id"})
			return
		}
		if err := imageService.RefreshImageCacheByNode(nodeID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		source, err := imageService.GetImageSource()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		baseURL := infrastructure.StreamsRemoteToBaseURL(source)
		if err := imageService.RefreshImageCache(baseURL, ""); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": "镜像缓存已刷新"})
	BroadcastDataRefresh("images", "")
}

// ListInstalledImages 获取节点已安装镜像列表（agent上报）
func ListInstalledImages(c *gin.Context) {
	nodeIDStr := c.Query("node_id")
	if nodeIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id 必填"})
		return
	}
	nodeID, err := uuid.Parse(nodeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 node_id"})
		return
	}
	imageType := c.Query("type")

	result, err := imageService.ListInstalledImages(nodeID, imageType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// ListImageCategories 获取镜像分类列表
func ListImageCategories(c *gin.Context) {
	nodeIDStr := c.Query("node_id")
	if nodeIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id 必填"})
		return
	}
	nodeID, err := uuid.Parse(nodeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 node_id"})
		return
	}
	imageType := c.Query("type")

	result, err := imageService.ListCategories(nodeID, imageType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// CreateImageCategory 创建镜像分类
func CreateImageCategory(c *gin.Context) {
	var req struct {
		NodeID    string `json:"node_id" binding:"required"`
		Name      string `json:"name" binding:"required"`
		ImageType string `json:"image_type" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id, name, image_type 必填"})
		return
	}
	nodeID, err := uuid.Parse(req.NodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 node_id"})
		return
	}
	result, err := imageService.CreateCategory(nodeID, req.Name, req.ImageType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": result})
}

// UpdateImageCategory 更新镜像分类
func UpdateImageCategory(c *gin.Context) {
	categoryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 category_id"})
		return
	}
	var req struct {
		Name      string `json:"name"`
		SortOrder int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}
	if err := imageService.UpdateCategory(categoryID, req.Name, req.SortOrder); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "分类已更新"})
}

// DeleteImageCategory 删除镜像分类
func DeleteImageCategory(c *gin.Context) {
	categoryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 category_id"})
		return
	}
	if err := imageService.DeleteCategory(categoryID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "分类已删除"})
}

// UpdateImageAlias 更新镜像别名（分类、显示名、install_ssh）
func UpdateImageAlias(c *gin.Context) {
	var req struct {
		NodeID       string  `json:"node_id" binding:"required"`
		Fingerprint  string  `json:"fingerprint" binding:"required"`
		ImageType    string  `json:"image_type" binding:"required"`
		CategoryID   *string `json:"category_id"`
		CategoryName *string `json:"category_name"`
		DisplayName  string  `json:"display_name"`
		InstallSSH   *bool   `json:"install_ssh"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}
	nodeID, err := uuid.Parse(req.NodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 node_id"})
		return
	}

	var catID *uuid.UUID
	if req.CategoryID != nil && *req.CategoryID != "" {
		parsed, err := uuid.Parse(*req.CategoryID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 category_id"})
			return
		}
		catID = &parsed
	}

	if err := imageService.UpdateImageAlias(nodeID, req.Fingerprint, req.ImageType, catID, req.CategoryName, req.DisplayName, req.InstallSSH); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "镜像别名已更新"})
}

// SyncNodeImages 触发节点镜像同步
func SyncNodeImages(c *gin.Context) {
	nodeIDStr := c.Query("node_id")
	if nodeIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id 必填"})
		return
	}
	nodeID, err := uuid.Parse(nodeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 node_id"})
		return
	}
	if err := imageService.SyncNodeImages(nodeID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "同步任务已触发"})
}

// ListReinstallImages 获取重装系统可选镜像列表（按分类分组）
func ListReinstallImages(c *gin.Context) {
	nodeIDStr := c.Query("node_id")
	if nodeIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id 必填"})
		return
	}
	nodeID, err := uuid.Parse(nodeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 node_id"})
		return
	}
	imageType := c.Query("type")
	if imageType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type 必填"})
		return
	}

	result, err := imageService.ListReinstallImages(nodeID, imageType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}
