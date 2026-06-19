package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"tsukiyo/master/internal/agent"
	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// ImageInfo 镜像信息（含节点下载状态）
// ID 使用 image_key 格式: alias|type|arch
type ImageInfo struct {
	ID              string `json:"id"`    // image_key: alias|type|arch
	Alias           string `json:"alias"` // 镜像别名, e.g. debian/forky/cloud
	Name            string `json:"name"`  // 显示名称
	Type            string `json:"type"`  // container / virtual-machine
	Distro          string `json:"distro,omitempty"`
	Release         string `json:"release,omitempty"`
	Arch            string `json:"arch"`
	Description     string `json:"description,omitempty"`
	Downloaded      bool   `json:"downloaded"`
	Stage           string `json:"stage,omitempty"`
	Progress        int    `json:"progress"`
	DownloadedBytes int64  `json:"downloaded_bytes"`
	TotalBytes      int64  `json:"total_bytes"`
	SpeedBps        int64  `json:"speed_bps"`
	Error           string `json:"download_error,omitempty"`
}

// ListImages 获取镜像列表（从 Incus 官方镜像源获取）
// 查询参数: node_id, type(container/virtual-machine), arch(x86_64/aarch64), distro, downloaded_only(true)
func ListImages(c *gin.Context) {
	nodeIDStr := c.Query("node_id")
	filterType := c.Query("type")
	filterArch := c.Query("arch")
	filterDistro := c.Query("distro")
	downloadedOnly := c.Query("downloaded_only") == "true"

	zap.L().Info("ListImages 请求参数",
		zap.String("node_id", nodeIDStr),
		zap.String("type", filterType),
		zap.String("arch", filterArch),
		zap.String("distro", filterDistro),
		zap.Bool("downloaded_only", downloadedOnly))

	var siteConfig models.SiteConfig
	if err := db.DB.First(&siteConfig).Error; err != nil {
		zap.L().Error("获取站点配置失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取站点配置失败"})
		return
	}

	remote := siteConfig.IncusRemoteURL
	if remote == "" {
		remote = "images:"
	}

	if nodeIDStr == "" {
		c.JSON(http.StatusOK, gin.H{"data": []ImageInfo{}, "total": 0})
		return
	}

	nodeID, err := uuid.Parse(nodeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 node_id"})
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

	// 通过 agent 同步获取远程镜像列表
	respData, err := agentMgr.SendRequest(nodeID, "list_remote_images", map[string]string{"remote": remote}, 30*time.Second)
	if err != nil {
		zap.L().Error("获取远程镜像列表失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取远程镜像列表失败: " + err.Error()})
		return
	}

	var resp struct {
		Images []struct {
			ImageKey     string `json:"image_key"`
			Alias        string `json:"alias"`
			Architecture string `json:"architecture"`
			Description  string `json:"description"`
			OS           string `json:"os"`
			Release      string `json:"release"`
			Type         string `json:"type"`
		} `json:"images"`
		Total int `json:"total"`
	}

	if err := json.Unmarshal(respData, &resp); err != nil {
		zap.L().Error("解析镜像列表响应失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析镜像列表响应失败"})
		return
	}

	// 构建下载状态 map，key = image_key
	type mergedStatus struct {
		Downloaded      bool
		Stage           string
		Progress        int
		DownloadedBytes int64
		TotalBytes      int64
		SpeedBps        int64
		Error           string
	}
	statusMap := make(map[string]*mergedStatus)

	// 1) 从 DB 加载
	var nodeImages []models.NodeImage
	db.DB.Where("node_id = ?", nodeID.String()).Find(&nodeImages)
	for _, ni := range nodeImages {
		switch ni.Status {
		case "downloaded":
			statusMap[ni.ImageID] = &mergedStatus{Downloaded: true, Stage: "done", Progress: 100}
		case "downloading":
			statusMap[ni.ImageID] = &mergedStatus{Stage: "downloading", Progress: 0}
		case "error":
			statusMap[ni.ImageID] = &mergedStatus{Stage: "error", Error: ni.Status}
		}
	}

	// 2) 叠加内存实时进度
	if agentMgr != nil {
		for k, v := range agentMgr.GetImageProgress(nodeID) {
			switch v.Stage {
			case "downloading", "error", "canceled":
				statusMap[k] = &mergedStatus{
					Stage: v.Stage, Progress: v.Progress,
					DownloadedBytes: v.DownloadedBytes, TotalBytes: v.TotalBytes,
					SpeedBps: v.SpeedBps, Error: v.Error,
				}
			case "done":
				statusMap[k] = &mergedStatus{Downloaded: true, Stage: "done", Progress: 100}
			}
		}
	}

	// 合并 + 后端过滤
	result := make([]ImageInfo, 0, len(resp.Images))
	for _, img := range resp.Images {
		// 类型过滤：支持 container / virtual-machine，也兼容前端传 vm
		if filterType != "" {
			ft := filterType
			if ft == "vm" {
				ft = "virtual-machine"
			}
			if img.Type != ft {
				continue
			}
		}
		// 架构过滤
		if filterArch != "" && img.Architecture != filterArch {
			continue
		}
		// 发行版过滤
		if filterDistro != "" && img.OS != filterDistro {
			continue
		}

		info := ImageInfo{
			ID:          img.ImageKey,
			Alias:       img.Alias,
			Name:        img.Description,
			Type:        img.Type,
			Distro:      img.OS,
			Release:     img.Release,
			Arch:        img.Architecture,
			Description: img.Description,
		}
		if s, ok := statusMap[img.ImageKey]; ok {
			info.Downloaded = s.Downloaded
			info.Stage = s.Stage
			info.Progress = s.Progress
			info.DownloadedBytes = s.DownloadedBytes
			info.TotalBytes = s.TotalBytes
			info.SpeedBps = s.SpeedBps
			info.Error = s.Error
		}

		// 仅已下载过滤
		if downloadedOnly && !info.Downloaded {
			continue
		}

		result = append(result, info)
	}

	c.JSON(http.StatusOK, gin.H{"data": result, "total": len(result)})
}

// DownloadImage 在指定节点上下载镜像
// 请求体: { node_id, image_key }
func DownloadImage(c *gin.Context) {
	var req struct {
		NodeID   string `json:"node_id" binding:"required"`
		ImageKey string `json:"image_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id 和 image_key 必填"})
		return
	}

	// 从 image_key 解析 alias, type, arch
	parts := strings.SplitN(req.ImageKey, "|", 3)
	if len(parts) != 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 image_key 格式"})
		return
	}
	alias := parts[0]
	imageType := parts[1]

	nodeID, err := uuid.Parse(req.NodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 node_id"})
		return
	}

	var siteConfig models.SiteConfig
	if err := db.DB.First(&siteConfig).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取站点配置失败"})
		return
	}

	remote := siteConfig.IncusRemoteURL
	if remote == "" {
		remote = "images:"
	}
	if !strings.HasSuffix(remote, ":") {
		remote += ":"
	}

	if agentMgr == nil || !agentMgr.IsNodeConnected(nodeID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "节点未在线"})
		return
	}

	// 构造任务 payload
	taskPayload := map[string]string{
		"image_key":  req.ImageKey,
		"image_type": imageType,
		"source":     remote + alias,
	}
	payloadBytes, _ := json.Marshal(taskPayload)

	// DB 状态记录，key = image_key
	nodeIDStr := nodeID.String()
	var ni models.NodeImage
	if dbErr := db.DB.Where("node_id = ? AND image_id = ?", nodeIDStr, req.ImageKey).First(&ni).Error; dbErr != nil {
		db.DB.Create(&models.NodeImage{NodeID: nodeIDStr, ImageID: req.ImageKey, Status: "downloading"})
	} else if ni.Status != "downloaded" {
		db.DB.Model(&ni).Updates(map[string]interface{}{"status": "downloading", "updated_at": gorm.Expr("NOW()")})
	}

	taskID := uuid.New()
	userID, _ := c.Get("user_id")
	task := models.Task{
		ID: taskID, Type: models.TaskTypeDownloadImage,
		NodeID: nodeID, UserID: userID.(uint),
		Status: models.TaskStatusPending, Payload: payloadBytes,
	}
	if err := db.DB.Create(&task).Error; err != nil {
		zap.L().Error("创建下载镜像任务失败", zap.Error(err))
	}

	if err := agentMgr.SendTask(agent.TaskMessage{
		NodeID: nodeID, TaskID: taskID, Type: "download_image", Payload: payloadBytes,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "下发下载任务失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"task_id": taskID.String(), "image_key": req.ImageKey,
		"node_id": req.NodeID, "message": "下载任务已下发",
	})
}

// GetImageProgress 查询单个镜像在指定节点的下载进度
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

	var ni models.NodeImage
	dbErr := db.DB.Where("node_id = ? AND image_id = ?", nodeIDStr, imageKey).First(&ni).Error
	downloaded := dbErr == nil && ni.Status == "downloaded"

	stage := ""
	progress := 0
	var downloadedBytes, totalBytes, speedBps int64
	var errMsg string
	if agentMgr != nil {
		live := agentMgr.GetSingleImageProgress(nodeID, imageKey)
		if live != nil {
			stage = live.Stage
			progress = live.Progress
			downloadedBytes = live.DownloadedBytes
			totalBytes = live.TotalBytes
			speedBps = live.SpeedBps
			errMsg = live.Error
		}
	}

	if downloaded && stage == "" {
		stage = "done"
		progress = 100
	}

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
	if agentMgr == nil || !agentMgr.IsNodeConnected(nodeID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "节点未在线"})
		return
	}

	taskPayload := map[string]string{"image_key": req.ImageKey}
	payloadBytes, _ := json.Marshal(taskPayload)

	taskID := uuid.New()
	userID, _ := c.Get("user_id")
	task := models.Task{
		ID: taskID, Type: models.TaskTypeCancelImageDownload,
		NodeID: nodeID, UserID: userID.(uint),
		Status: models.TaskStatusPending, Payload: payloadBytes,
	}
	if err := db.DB.Create(&task).Error; err != nil {
		zap.L().Error("创建取消下载任务失败", zap.Error(err))
	}

	if err := agentMgr.SendTask(agent.TaskMessage{
		NodeID: nodeID, TaskID: taskID, Type: "cancel_image_download", Payload: payloadBytes,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "下发取消任务失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "取消任务已下发"})
}

// DeleteImage 删除指定节点上的镜像
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
	if agentMgr == nil || !agentMgr.IsNodeConnected(nodeID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "节点未在线"})
		return
	}

	taskPayload := map[string]string{"image_key": req.ImageKey}
	payloadBytes, _ := json.Marshal(taskPayload)

	taskID := uuid.New()
	userID, _ := c.Get("user_id")
	task := models.Task{
		ID: taskID, Type: models.TaskTypeDeleteImage,
		NodeID: nodeID, UserID: userID.(uint),
		Status: models.TaskStatusPending, Payload: payloadBytes,
	}
	if err := db.DB.Create(&task).Error; err != nil {
		zap.L().Error("创建删除镜像任务失败", zap.Error(err))
	}

	if err := agentMgr.SendTask(agent.TaskMessage{
		NodeID: nodeID, TaskID: taskID, Type: "delete_image", Payload: payloadBytes,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "下发删除任务失败: " + err.Error()})
		return
	}

	// 删除 DB 记录
	db.DB.Where("node_id = ? AND image_id = ?", nodeID.String(), req.ImageKey).Delete(&models.NodeImage{})

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
	if agentMgr == nil || !agentMgr.IsNodeConnected(nodeID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "节点未在线"})
		return
	}

	remote := req.Remote
	if remote == "" {
		remote = "images:"
	}

	payloadBytes, _ := json.Marshal(map[string]string{"remote": remote})

	taskID := uuid.New()
	if err := agentMgr.SendTask(agent.TaskMessage{
		NodeID: nodeID, TaskID: taskID, Type: "list_remote_images", Payload: payloadBytes,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "下发任务失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"task_id": taskID.String(), "message": "获取远程镜像列表任务已下发"})
}
