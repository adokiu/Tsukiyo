package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"tsukiyo/master/internal/agent"
	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// ImageInfo 镜像信息（含节点下载状态）
type ImageInfo struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	Distro          string `json:"distro,omitempty"`
	Release         string `json:"release,omitempty"`
	Arch            string `json:"arch"`
	URL             string `json:"url,omitempty"`
	Description     string `json:"description,omitempty"`
	Enabled         bool   `json:"enabled"`
	Desktop         string `json:"desktop,omitempty"`
	Downloaded      bool   `json:"downloaded"`      // 是否已在节点下载完成
	Stage           string `json:"stage,omitempty"` // downloading / completed / error
	Progress        int    `json:"progress"`        // 0-100
	DownloadedBytes int64  `json:"downloaded_bytes"`
	TotalBytes      int64  `json:"total_bytes"`
	SpeedBps        int64  `json:"speed_bps"`
	Error           string `json:"download_error,omitempty"`
}

// ListImages 获取镜像列表（预制模板 + 节点下载状态）
// 参数: ?type=container|vm  &node_id=xxx
func ListImages(c *gin.Context) {
	imageType := c.Query("type")
	nodeIDStr := c.Query("node_id")

	query := db.DB.Order("type DESC, name ASC")
	if imageType != "" {
		query = query.Where("type = ?", imageType)
	}

	var images []models.ImageTemplate
	if err := query.Find(&images).Error; err != nil {
		zap.L().Error("查询镜像列表失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	// 三层数据源合并：DB NodeImage（持久化）+ 内存 imageProgress（实时进度）
	// DB 提供 downloaded 权威状态，内存提供正在下载的实时进度
	type mergedStatus struct {
		Downloaded      bool
		Stage           string
		Progress        int
		DownloadedBytes int64
		TotalBytes      int64
		SpeedBps        int64
		Error           string
	}
	var statusMap map[string]*mergedStatus

	if nodeIDStr != "" {
		nodeID, err := uuid.Parse(nodeIDStr)
		if err == nil {
			statusMap = make(map[string]*mergedStatus)

			// 1) 从 DB 加载该节点所有镜像状态（权威数据源，Master 重启后仍可用）
			var nodeImages []models.NodeImage
			db.DB.Where("node_id = ?", nodeID.String()).Find(&nodeImages)
			for _, ni := range nodeImages {
				switch ni.Status {
				case "downloaded":
					statusMap[ni.ImageID] = &mergedStatus{
						Downloaded: true,
						Stage:      "done",
						Progress:   100,
					}
				case "downloading":
					statusMap[ni.ImageID] = &mergedStatus{
						Stage:    "downloading",
						Progress: 0,
					}
				case "error":
					statusMap[ni.ImageID] = &mergedStatus{
						Stage: "error",
						Error: ni.Status, // 如有需要后续可扩展 error 字段
					}
				}
			}

			// 2) 叠加内存中的实时进度（正在下载、出错等动态状态覆盖 DB 静态数据）
			if agentMgr != nil {
				liveProgress := agentMgr.GetImageProgress(nodeID)
				for k, v := range liveProgress {
					if v.Stage == "downloading" || v.Stage == "error" || v.Stage == "canceled" {
						statusMap[k] = &mergedStatus{
							Stage:           v.Stage,
							Progress:        v.Progress,
							DownloadedBytes: v.DownloadedBytes,
							TotalBytes:      v.TotalBytes,
							SpeedBps:        v.SpeedBps,
							Error:           v.Error,
						}
					} else if v.Stage == "done" {
						statusMap[k] = &mergedStatus{
							Downloaded: true,
							Stage:      "done",
							Progress:   100,
						}
					}
				}
			}
		}
	}

	result := make([]ImageInfo, 0, len(images))
	for _, img := range images {
		info := ImageInfo{
			ID:          img.ID,
			Name:        img.Name,
			Type:        string(img.Type),
			Distro:      img.Distro,
			Release:     img.Release,
			Arch:        img.Arch,
			URL:         img.URL,
			Description: img.Description,
			Enabled:     img.Enabled,
			Desktop:     img.Desktop,
		}
		if statusMap != nil {
			// 同时尝试模板 ID 和 alias 匹配
			var matchedStatus *mergedStatus
			if s, ok := statusMap[img.ID]; ok {
				matchedStatus = s
			} else if img.Alias != "" {
				if s, ok := statusMap[img.Alias]; ok {
					matchedStatus = s
				}
			}
			if matchedStatus != nil {
				info.Downloaded = matchedStatus.Downloaded
				info.Stage = matchedStatus.Stage
				info.Progress = matchedStatus.Progress
				info.DownloadedBytes = matchedStatus.DownloadedBytes
				info.TotalBytes = matchedStatus.TotalBytes
				info.SpeedBps = matchedStatus.SpeedBps
				info.Error = matchedStatus.Error
			}
		}
		result = append(result, info)
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  result,
		"total": len(result),
	})
}

// ToggleImage 启用/禁用镜像
func ToggleImage(c *gin.Context) {
	imageID := c.Param("id")
	if imageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "镜像 ID 不能为空"})
		return
	}

	var image models.ImageTemplate
	if err := db.DB.Where("id = ?", imageID).First(&image).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "镜像不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	newStatus := !image.Enabled
	if err := db.DB.Model(&image).Update("enabled", newStatus).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      image.ID,
		"enabled": newStatus,
	})
}

// DownloadImage 在指定节点上下载镜像
func DownloadImage(c *gin.Context) {
	imageID := c.Param("id")
	var req struct {
		NodeID string `json:"node_id" binding:"required"`
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

	// 查找镜像模板
	var img models.ImageTemplate
	if err := db.DB.Where("id = ?", imageID).First(&img).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "镜像不存在"})
		return
	}

	if agentMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Agent 管理器未初始化"})
		return
	}

	if !agentMgr.IsNodeConnected(nodeID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "节点未在线"})
		return
	}

	// 构造下载任务
	taskPayload := map[string]string{
		"image_id":   img.ID,
		"image_type": string(img.Type),
		"source":     img.URL,
	}
	payloadBytes, _ := json.Marshal(taskPayload)

	// 先在 DB 中创建 downloading 状态记录（Master 维护权威状态）
	nodeIDStr := nodeID.String()
	var ni models.NodeImage
	if dbErr := db.DB.Where("node_id = ? AND image_id = ?", nodeIDStr, imageID).First(&ni).Error; dbErr != nil {
		db.DB.Create(&models.NodeImage{
			NodeID:  nodeIDStr,
			ImageID: imageID,
			Status:  "downloading",
		})
	} else if ni.Status != "downloaded" {
		db.DB.Model(&ni).Updates(map[string]interface{}{"status": "downloading", "updated_at": gorm.Expr("NOW()")})
	}

	taskID := uuid.New()
	taskMsg := agent.TaskMessage{
		NodeID:  nodeID,
		TaskID:  taskID,
		Type:    "download_image",
		Payload: payloadBytes,
	}
	if err := agentMgr.SendTask(taskMsg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "下发下载任务失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"task_id":  taskID.String(),
		"image_id": imageID,
		"node_id":  req.NodeID,
		"message":  "下载任务已下发",
	})
}

// GetImageProgress 查询单个镜像在指定节点的下载进度
func GetImageProgress(c *gin.Context) {
	imageID := c.Param("id")
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

	// 从 DB 查持久化状态
	var ni models.NodeImage
	dbErr := db.DB.Where("node_id = ? AND image_id = ?", nodeIDStr, imageID).First(&ni).Error
	downloaded := dbErr == nil && ni.Status == "downloaded"

	// 从内存查实时进度
	stage := ""
	progress := 0
	var downloadedBytes, totalBytes, speedBps int64
	var errMsg string
	if agentMgr != nil {
		live := agentMgr.GetSingleImageProgress(nodeID, imageID)
		if live != nil {
			stage = live.Stage
			progress = live.Progress
			downloadedBytes = live.DownloadedBytes
			totalBytes = live.TotalBytes
			speedBps = live.SpeedBps
			errMsg = live.Error
		}
	}

	// 如果 DB 显示已下载但内存无数据，以 DB 为准
	if downloaded && stage == "" {
		stage = "done"
		progress = 100
	}

	c.JSON(http.StatusOK, gin.H{
		"image_id":         imageID,
		"node_id":          nodeIDStr,
		"downloaded":       downloaded,
		"stage":            stage,
		"progress":         progress,
		"downloaded_bytes": downloadedBytes,
		"total_bytes":      totalBytes,
		"speed_bps":        speedBps,
		"error":            errMsg,
	})
}

// CancelImageDownload 取消镜像下载
func CancelImageDownload(c *gin.Context) {
	imageID := c.Param("id")
	var req struct {
		NodeID string `json:"node_id" binding:"required"`
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

	if agentMgr == nil || !agentMgr.IsNodeConnected(nodeID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "节点未在线"})
		return
	}

	taskPayload := map[string]string{"image_id": imageID}
	payloadBytes, _ := json.Marshal(taskPayload)

	taskID := uuid.New()
	cancelMsg := agent.TaskMessage{
		NodeID:  nodeID,
		TaskID:  taskID,
		Type:    "cancel_image_download",
		Payload: payloadBytes,
	}
	if err := agentMgr.SendTask(cancelMsg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "下发取消任务失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "取消任务已下发"})
}
