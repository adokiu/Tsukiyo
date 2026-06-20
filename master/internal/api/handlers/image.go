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
	filterType := c.Query("type")
	filterArch := c.Query("arch")
	filterDistro := c.Query("distro")
	downloadedOnly := c.Query("downloaded_only") == "true"

	if nodeIDStr == "" {
		c.JSON(http.StatusOK, gin.H{"data": []ImageInfo{}, "total": 0})
		return
	}

	nodeID, err := uuid.Parse(nodeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 node_id"})
		return
	}

	result, err := imageService.ListImages(nodeID, filterType, filterArch, filterDistro, downloadedOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result, "total": len(result)})
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
}
