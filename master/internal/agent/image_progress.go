package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// BroadcastImageProgress 向前端广播镜像下载进度
func (m *Manager) BroadcastImageProgress(nodeID uuid.UUID, payload ImageProgressPayload) {
	data, err := json.Marshal(map[string]interface{}{
		"type":    "image_progress",
		"node_id": nodeID.String(),
		"payload": payload,
	})
	if err != nil {
		return
	}

	m.frontendMu.Lock()
	defer m.frontendMu.Unlock()
	alive := make([]*FrontendConn, 0, len(m.frontendConns))
	for _, fc := range m.frontendConns {
		select {
		case fc.SendCh <- data:
			alive = append(alive, fc)
		default:
			// 发送缓冲区满，丢弃该连接
			zap.L().Warn("发送缓冲区满，丢弃连接")
		}
	}
	m.frontendConns = alive
}

// BroadcastInstanceProgress 向前端广播实例创建进度
func (m *Manager) BroadcastInstanceProgress(nodeID uuid.UUID, payload InstanceProgressPayload) {
	data, err := json.Marshal(map[string]interface{}{
		"type":    "instance_progress",
		"node_id": nodeID.String(),
		"payload": payload,
	})
	if err != nil {
		return
	}

	m.frontendMu.Lock()
	defer m.frontendMu.Unlock()
	zap.L().Info("广播实例进度", zap.String("node_id", nodeID.String()), zap.String("instance_id", payload.InstanceID), zap.Int("前端连接数", len(m.frontendConns)))
	alive := make([]*FrontendConn, 0, len(m.frontendConns))
	for _, fc := range m.frontendConns {
		select {
		case fc.SendCh <- data:
			alive = append(alive, fc)
		default:
			zap.L().Warn("发送缓冲区满，丢弃连接")
		}
	}
	m.frontendConns = alive
}

// handleImageProgress 处理 Agent 上报的镜像下载进度或本地镜像列表
func (m *Manager) handleImageProgress(nodeID uuid.UUID, payload json.RawMessage) {
	// 先尝试解析为镜像列表上报（Agent 启动时 / 定期同步）
	// agent 现在上报完整镜像信息结构体列表
	var imageList struct {
		Images []AgentLocalImage `json:"images"`
	}
	if err := json.Unmarshal(payload, &imageList); err == nil && imageList.Images != nil {
		m.handleImageListSync(nodeID, imageList.Images)
		return
	}

	// 单条下载进度
	var p ImageProgressPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		zap.L().Warn("解析镜像进度失败", zap.String("node_id", nodeID.String()), zap.Error(err))
		return
	}

	now := time.Now()

	m.imageMu.Lock()
	nodeMap, ok := m.imageProgress[nodeID]
	if !ok {
		nodeMap = make(map[string]*NodeImageStatus)
		m.imageProgress[nodeID] = nodeMap
	}
	oldStatus, existed := nodeMap[p.ImageID]
	nodeMap[p.ImageID] = &NodeImageStatus{
		ImageID:         p.ImageID,
		Stage:           p.Stage,
		Progress:        p.Progress,
		DownloadedBytes: p.DownloadedBytes,
		TotalBytes:      p.TotalBytes,
		SpeedBps:        p.SpeedBps,
		Error:           p.Error,
		UpdatedAt:       now,
	}
	m.imageMu.Unlock()

	// 限制广播频率：每 0.5 秒最多一次（或状态变化时立即广播）
	shouldBroadcast := true
	if existed && oldStatus.Stage == p.Stage && p.Stage == "downloading" {
		if now.Sub(oldStatus.UpdatedAt) < 500*time.Millisecond {
			shouldBroadcast = false
		}
	}
	if shouldBroadcast {
		m.BroadcastImageProgress(nodeID, p)
	}

	// 下载完成 → 清除 Redis 中的中间进度（NodeImage 持久化由 handleImageListSync 处理）
	if p.Stage == "done" {
		nodeStr := nodeID.String()
		ctx := context.Background()
		progressKey := fmt.Sprintf("image_progress:%s:%s", nodeStr, p.ImageID)
		db.RedisClient.Del(ctx, progressKey)
	} else if p.Stage == "downloading" {
		// 中间进度写入 Redis，防止 Master 重启丢失
		nodeStr := nodeID.String()
		ctx := context.Background()
		progressKey := fmt.Sprintf("image_progress:%s:%s", nodeStr, p.ImageID)
		progressData, _ := json.Marshal(map[string]interface{}{
			"stage":            p.Stage,
			"progress":         p.Progress,
			"downloaded_bytes": p.DownloadedBytes,
			"total_bytes":      p.TotalBytes,
			"speed_bps":        p.SpeedBps,
			"updated_at":       now.Unix(),
		})
		db.RedisClient.Set(ctx, progressKey, progressData, 10*time.Minute)
	}
}

// handleInstanceProgress 处理 Agent 上报的实例创建进度
func (m *Manager) handleInstanceProgress(nodeID uuid.UUID, payload json.RawMessage) {
	var p InstanceProgressPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		zap.L().Warn("解析实例进度失败", zap.String("node_id", nodeID.String()), zap.Error(err))
		return
	}

	// 直接广播给前端
	m.BroadcastInstanceProgress(nodeID, p)
}

// AgentLocalImage agent 上报的本地镜像完整信息
type AgentLocalImage struct {
	Fingerprint  string   `json:"fingerprint"`
	Aliases      []string `json:"aliases"`
	Type         string   `json:"type"`
	Architecture string   `json:"architecture"`
	Size         int64    `json:"size"`
	Description  string   `json:"description"`
	UploadDate   string   `json:"upload_date"`
	ImageSource  string   `json:"image_source"`
}

// handleImageListSync 处理 Agent 全量镜像列表上报，执行增量同步 + 清理已删除
// agent 上报完整镜像信息，master 按 (node_id, fingerprint, image_type) 做增量同步
func (m *Manager) handleImageListSync(nodeID uuid.UUID, images []AgentLocalImage) {
	now := time.Now()
	nodeStr := nodeID.String()

	zap.L().Info("Agent 上报镜像列表", zap.String("node_id", nodeStr), zap.Int("count", len(images)))

	// 构建上报集合（fingerprint|image_type 作为唯一键）
	reported := make(map[string]struct{}, len(images))
	for _, img := range images {
		key := img.Fingerprint + "|" + img.Type
		reported[key] = struct{}{}
	}

	// Upsert 上报的镜像
	for _, img := range images {
		// 取第一个 alias 作为主别名
		alias := ""
		if len(img.Aliases) > 0 {
			alias = img.Aliases[0]
		}

		// upsert node_images
		var ni models.NodeImage
		if err := db.DB.Where("node_id = ? AND fingerprint = ? AND image_type = ?", nodeStr, img.Fingerprint, img.Type).First(&ni).Error; err != nil {
			// 新镜像，创建记录
			db.DB.Create(&models.NodeImage{
				NodeID:       nodeStr,
				Fingerprint:  img.Fingerprint,
				Alias:        alias,
				ImageType:    img.Type,
				Architecture: img.Architecture,
				SizeBytes:    img.Size,
				Description:  img.Description,
				UploadDate:   img.UploadDate,
				ImageSource:  img.ImageSource,
				Status:       "downloaded",
				UpdatedAt:    now,
			})
		} else {
			// 更新已有记录
			updates := map[string]interface{}{
				"alias":        alias,
				"architecture": img.Architecture,
				"size_bytes":   img.Size,
				"description":  img.Description,
				"upload_date":  img.UploadDate,
				"status":       "downloaded",
				"updated_at":   now,
			}
			// 仅在上报的 image_source 非 manual 时才更新，避免属性丢失后覆盖原值
			if img.ImageSource != "" && img.ImageSource != "manual" {
				updates["image_source"] = img.ImageSource
			}
			db.DB.Model(&ni).Updates(updates)
		}

		// upsert node_image_aliases（ON CONFLICT 时不覆盖用户已设置的 display_name 和 install_ssh）
		installSSH := defaultInstallSSH(img.ImageSource, img.Type)
		db.DB.Exec(`INSERT INTO node_image_aliases (node_id, fingerprint, image_type, category_id, display_name, install_ssh)
			VALUES (?, ?, ?, NULL, ?, ?)
			ON CONFLICT (node_id, fingerprint, image_type) DO NOTHING`,
			nodeStr, img.Fingerprint, img.Type, alias, installSSH)
	}

	// 清理 DB 中该节点已不存在的镜像
	var existing []models.NodeImage
	db.DB.Where("node_id = ?", nodeStr).Find(&existing)
	cleaned := 0
	for _, ni := range existing {
		key := ni.Fingerprint + "|" + ni.ImageType
		if _, ok := reported[key]; !ok {
			zap.L().Info("删除节点镜像记录", zap.String("node_id", nodeStr), zap.String("fingerprint", ni.Fingerprint), zap.String("image_type", ni.ImageType))
			db.DB.Where("node_id = ? AND fingerprint = ? AND image_type = ?", nodeStr, ni.Fingerprint, ni.ImageType).Delete(&models.NodeImage{})
			// 同时删除别名映射
			db.DB.Where("node_id = ? AND fingerprint = ? AND image_type = ?", nodeStr, ni.Fingerprint, ni.ImageType).Delete(&models.NodeImageAlias{})
			m.BroadcastImageProgress(nodeID, ImageProgressPayload{
				ImageID: ni.Alias, Stage: "deleted", Progress: 0,
			})
			cleaned++
		}
	}

	// 同步更新内存缓存（用于下载进度追踪）
	m.imageMu.Lock()
	nodeMap := make(map[string]*NodeImageStatus, len(images))
	for _, img := range images {
		imageKey := ""
		if len(img.Aliases) > 0 {
			imageKey = img.Aliases[0] + "|" + img.Type + "|" + img.Architecture
		}
		if imageKey != "" {
			nodeMap[imageKey] = &NodeImageStatus{
				ImageID: imageKey, Stage: "done", Progress: 100, UpdatedAt: now,
			}
		}
	}
	// 保留正在下载中的条目
	if oldMap, ok := m.imageProgress[nodeID]; ok {
		for k, v := range oldMap {
			if v.Stage == "downloading" {
				nodeMap[k] = v
			}
		}
	}
	m.imageProgress[nodeID] = nodeMap
	m.imageMu.Unlock()

	zap.L().Info("节点镜像列表已同步",
		zap.String("node_id", nodeStr),
		zap.Int("reported", len(images)),
		zap.Int("cleaned", cleaned))

	// 广播镜像列表更新给前端
	m.BroadcastImageProgress(nodeID, ImageProgressPayload{
		ImageID:  "",
		Stage:    "sync",
		Progress: 100,
	})
}

// defaultInstallSSH 根据镜像来源和类型判断默认是否安装 SSH
func defaultInstallSSH(imageSource, imageType string) bool {
	if imageType == "virtual-machine" {
		return false
	}
	// 容器类型
	if imageSource == "images" {
		return true
	}
	// spiritlhl 和 manual 默认不安装
	return false
}

// GetImageProgress 获取指定节点的所有镜像下载状态
func (m *Manager) GetImageProgress(nodeID uuid.UUID) map[string]*NodeImageStatus {
	m.imageMu.RLock()
	nodeMap, ok := m.imageProgress[nodeID]
	m.imageMu.RUnlock()

	if !ok || len(nodeMap) == 0 {
		// 内存无数据，从数据库加载
		var nodeImages []models.NodeImage
		if err := db.DB.Where("node_id = ?", nodeID.String()).Find(&nodeImages).Error; err == nil && len(nodeImages) > 0 {
			m.imageMu.Lock()
			nodeMap = make(map[string]*NodeImageStatus)
			m.imageProgress[nodeID] = nodeMap
			for _, ni := range nodeImages {
				imageKey := ni.Alias + "|" + ni.ImageType + "|" + ni.Architecture
				nodeMap[imageKey] = &NodeImageStatus{
					ImageID:   imageKey,
					Stage:     "done",
					Progress:  100,
					UpdatedAt: ni.UpdatedAt,
				}
			}
			m.imageMu.Unlock()
		} else {
			return nil
		}
	}

	// 返回副本
	m.imageMu.RLock()
	defer m.imageMu.RUnlock()
	result := make(map[string]*NodeImageStatus, len(nodeMap))
	for k, v := range nodeMap {
		cp := *v
		result[k] = &cp
	}
	return result
}

// GetSingleImageProgress 获取指定节点上指定镜像的下载状态
func (m *Manager) GetSingleImageProgress(nodeID uuid.UUID, imageID string) *NodeImageStatus {
	m.imageMu.RLock()
	defer m.imageMu.RUnlock()

	if nodeMap, ok := m.imageProgress[nodeID]; ok {
		if s, ok := nodeMap[imageID]; ok {
			cp := *s
			return &cp
		}
	}
	return nil
}

// getEIPAllocCIDR 查询 EIP 分配记录的 CIDR
func getEIPAllocCIDR(allocID *uuid.UUID) string {
	if allocID == nil {
		return ""
	}
	var alloc models.EIPAllocation
	if err := db.DB.Where("id = ?", *allocID).First(&alloc).Error; err != nil {
		return ""
	}
	return alloc.CIDR
}
