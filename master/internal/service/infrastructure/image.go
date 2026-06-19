package infrastructure

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"tsukiyo/master/internal/agent"
	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/service"
)

// ImageService 镜像服务
type ImageService struct {
	agentMgr *agent.Manager
}

// NewImageService 创建镜像服务
func NewImageService(agentMgr *agent.Manager) *ImageService {
	return &ImageService{agentMgr: agentMgr}
}

// ImageInfo 镜像信息（含节点下载状态）
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

// ListImages 获取镜像列表
func (s *ImageService) ListImages(nodeID uuid.UUID, filterType, filterArch, filterDistro string, downloadedOnly bool) ([]ImageInfo, error) {
	var siteConfig models.SiteConfig
	if err := db.DB.First(&siteConfig).Error; err != nil {
		zap.L().Error("获取站点配置失败", zap.Error(err))
		return nil, err
	}

	remote := siteConfig.IncusRemoteURL
	if remote == "" {
		remote = "images:"
	}

	if s.agentMgr == nil {
		return nil, service.ErrAgentManagerNotInitialized
	}

	if !s.agentMgr.IsNodeConnected(nodeID) {
		return nil, service.ErrNodeNotConnected
	}

	// 通过 agent 同步获取远程镜像列表
	respData, err := s.agentMgr.SendRequest(nodeID, "list_remote_images", map[string]string{"remote": remote}, 30*time.Second)
	if err != nil {
		zap.L().Error("获取远程镜像列表失败", zap.Error(err))
		return nil, err
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
		return nil, err
	}

	// 构建下载状态 map
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

	// 从 DB 加载
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

	// 叠加内存实时进度
	if s.agentMgr != nil {
		for k, v := range s.agentMgr.GetImageProgress(nodeID) {
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

	// 合并 + 过滤
	result := make([]ImageInfo, 0, len(resp.Images))
	for _, img := range resp.Images {
		// 类型过滤
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

	return result, nil
}

// DownloadImage 下载镜像
func (s *ImageService) DownloadImage(nodeID uuid.UUID, imageKey string, userID uint) (*uuid.UUID, error) {
	parts := strings.SplitN(imageKey, "|", 3)
	if len(parts) != 3 {
		return nil, service.ErrInvalidImageKeyFormat
	}
	alias := parts[0]
	imageType := parts[1]

	var siteConfig models.SiteConfig
	if err := db.DB.First(&siteConfig).Error; err != nil {
		return nil, err
	}

	remote := siteConfig.IncusRemoteURL
	if remote == "" {
		remote = "images:"
	}
	if !strings.HasSuffix(remote, ":") {
		remote += ":"
	}

	if s.agentMgr == nil || !s.agentMgr.IsNodeConnected(nodeID) {
		return nil, service.ErrNodeNotConnected
	}

	// 构造任务 payload
	taskPayload := map[string]string{
		"image_key":  imageKey,
		"image_type": imageType,
		"source":     remote + alias,
	}
	payloadBytes, _ := json.Marshal(taskPayload)

	// DB 状态记录
	nodeIDStr := nodeID.String()
	var ni models.NodeImage
	if dbErr := db.DB.Where("node_id = ? AND image_id = ?", nodeIDStr, imageKey).First(&ni).Error; dbErr != nil {
		db.DB.Create(&models.NodeImage{NodeID: nodeIDStr, ImageID: imageKey, Status: "downloading"})
	} else if ni.Status != "downloaded" {
		db.DB.Model(&ni).Updates(map[string]interface{}{"status": "downloading", "updated_at": gorm.Expr("NOW()")})
	}

	taskID := uuid.New()
	task := models.Task{
		ID: taskID, Type: models.TaskTypeDownloadImage,
		NodeID: nodeID, UserID: userID,
		Status: models.TaskStatusPending, Payload: payloadBytes,
	}
	if err := db.DB.Create(&task).Error; err != nil {
		zap.L().Error("创建下载镜像任务失败", zap.Error(err))
	}

	if err := s.agentMgr.SendTask(agent.TaskMessage{
		NodeID: nodeID, TaskID: taskID, Type: "download_image", Payload: payloadBytes,
	}); err != nil {
		return nil, err
	}

	return &taskID, nil
}

// GetImageProgress 查询镜像下载进度
func (s *ImageService) GetImageProgress(nodeID uuid.UUID, imageKey string) (downloaded bool, stage string, progress int, downloadedBytes, totalBytes, speedBps int64, errMsg string) {
	nodeIDStr := nodeID.String()

	var ni models.NodeImage
	dbErr := db.DB.Where("node_id = ? AND image_id = ?", nodeIDStr, imageKey).First(&ni).Error
	downloaded = dbErr == nil && ni.Status == "downloaded"

	if s.agentMgr != nil {
		live := s.agentMgr.GetSingleImageProgress(nodeID, imageKey)
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

	return
}

// CancelImageDownload 取消镜像下载
func (s *ImageService) CancelImageDownload(nodeID uuid.UUID, imageKey string, userID uint) (*uuid.UUID, error) {
	if s.agentMgr == nil || !s.agentMgr.IsNodeConnected(nodeID) {
		return nil, service.ErrNodeNotConnected
	}

	taskPayload := map[string]string{"image_key": imageKey}
	payloadBytes, _ := json.Marshal(taskPayload)

	taskID := uuid.New()
	task := models.Task{
		ID: taskID, Type: models.TaskTypeCancelImageDownload,
		NodeID: nodeID, UserID: userID,
		Status: models.TaskStatusPending, Payload: payloadBytes,
	}
	if err := db.DB.Create(&task).Error; err != nil {
		zap.L().Error("创建取消下载任务失败", zap.Error(err))
	}

	if err := s.agentMgr.SendTask(agent.TaskMessage{
		NodeID: nodeID, TaskID: taskID, Type: "cancel_image_download", Payload: payloadBytes,
	}); err != nil {
		return nil, err
	}

	return &taskID, nil
}

// DeleteImage 删除镜像
func (s *ImageService) DeleteImage(nodeID uuid.UUID, imageKey string, userID uint) (*uuid.UUID, error) {
	if s.agentMgr == nil || !s.agentMgr.IsNodeConnected(nodeID) {
		return nil, service.ErrNodeNotConnected
	}

	taskPayload := map[string]string{"image_key": imageKey}
	payloadBytes, _ := json.Marshal(taskPayload)

	taskID := uuid.New()
	task := models.Task{
		ID: taskID, Type: models.TaskTypeDeleteImage,
		NodeID: nodeID, UserID: userID,
		Status: models.TaskStatusPending, Payload: payloadBytes,
	}
	if err := db.DB.Create(&task).Error; err != nil {
		zap.L().Error("创建删除镜像任务失败", zap.Error(err))
	}

	if err := s.agentMgr.SendTask(agent.TaskMessage{
		NodeID: nodeID, TaskID: taskID, Type: "delete_image", Payload: payloadBytes,
	}); err != nil {
		return nil, err
	}

	// 删除 DB 记录
	db.DB.Where("node_id = ? AND image_id = ?", nodeID.String(), imageKey).Delete(&models.NodeImage{})

	return &taskID, nil
}

// ListRemoteImages 获取远程镜像列表（异步任务）
func (s *ImageService) ListRemoteImages(nodeID uuid.UUID, remote string) (*uuid.UUID, error) {
	if s.agentMgr == nil || !s.agentMgr.IsNodeConnected(nodeID) {
		return nil, service.ErrNodeNotConnected
	}

	if remote == "" {
		remote = "images:"
	}

	payloadBytes, _ := json.Marshal(map[string]string{"remote": remote})

	taskID := uuid.New()
	if err := s.agentMgr.SendTask(agent.TaskMessage{
		NodeID: nodeID, TaskID: taskID, Type: "list_remote_images", Payload: payloadBytes,
	}); err != nil {
		return nil, err
	}

	return &taskID, nil
}
