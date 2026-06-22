package infrastructure

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

// 默认 Incus 镜像源
const defaultRemoteName = "spiritlhl:"
const defaultStreamsBaseURL = "https://incusimages.spiritlhl.net"

// ImageService 镜像服务
type ImageService struct {
	agentMgr *agent.Manager
}

// NewImageService 创建镜像服务
func NewImageService(agentMgr *agent.Manager) *ImageService {
	return &ImageService{agentMgr: agentMgr}
}

// GetImageSource 获取当前镜像源配置
func (s *ImageService) GetImageSource() (string, error) {
	var siteConfig models.SiteConfig
	if err := db.DB.First(&siteConfig).Error; err != nil {
		return defaultRemoteName, nil
	}
	if siteConfig.IncusRemoteURL != "" {
		return siteConfig.IncusRemoteURL, nil
	}
	return defaultRemoteName, nil
}

// SetImageSource 设置镜像源并刷新缓存
func (s *ImageService) SetImageSource(remoteURL string) error {
	var siteConfig models.SiteConfig
	if err := db.DB.First(&siteConfig).Error; err != nil {
		return err
	}
	siteConfig.IncusRemoteURL = remoteURL
	if err := db.DB.Save(&siteConfig).Error; err != nil {
		return err
	}

	// 清除所有节点的 ImageRemoteURL，使其回退到站点配置
	if err := db.DB.Model(&models.Node{}).Where("image_remote_url IS NOT NULL AND image_remote_url != ''").Update("image_remote_url", "").Error; err != nil {
		zap.L().Error("清除节点镜像源配置失败", zap.Error(err))
	}

	// 刷新缓存
	baseURL := StreamsRemoteToBaseURL(remoteURL)
	return s.RefreshImageCache(baseURL, "")
}

// RefreshImageCacheByNode 根据节点配置刷新镜像缓存
func (s *ImageService) RefreshImageCacheByNode(nodeID uuid.UUID) error {
	var node models.Node
	if err := db.DB.First(&node, "id = ?", nodeID).Error; err != nil {
		return fmt.Errorf("节点不存在: %w", err)
	}
	var siteConfig models.SiteConfig
	if err := db.DB.First(&siteConfig).Error; err != nil {
		return err
	}
	nodeArch := getNodeArch(&node)
	if nodeArch == "" {
		nodeArch = "amd64"
	}
	baseURL := getStreamsBaseURL(&node, &siteConfig)
	return s.RefreshImageCache(baseURL, nodeArch)
}

// ImageInfo 镜像信息（含节点下载状态）- 用于远程镜像下载 tab
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

// InstalledImage 已安装镜像信息（agent上报，合并用户分类和别名）
type InstalledImage struct {
	ID           string  `json:"id"` // fingerprint|type
	Fingerprint  string  `json:"fingerprint"`
	Alias        string  `json:"alias"`        // incus alias
	DisplayName  string  `json:"display_name"` // 用户自定义别名
	Type         string  `json:"type"`         // container / virtual-machine
	Architecture string  `json:"architecture"`
	Size         int64   `json:"size"`
	Description  string  `json:"description"`
	ImageSource  string  `json:"image_source"` // spiritlhl / images / manual
	UploadDate   string  `json:"upload_date"`
	CategoryID   *string `json:"category_id"`
	CategoryName string  `json:"category_name"`
	InstallSSH   bool    `json:"install_ssh"`
}

// CategoryInfo 分类信息
type CategoryInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ImageType string `json:"image_type"`
	SortOrder int    `json:"sort_order"`
}

// streamsAPIResponse Incus streams API 响应结构
type streamsAPIResponse struct {
	Products map[string]streamsProduct `json:"products"`
}

// streamsProduct streams API 中的产品信息
type streamsProduct struct {
	Aliases  string                           `json:"aliases"`
	Arch     string                           `json:"arch"`
	Distro   string                           `json:"distro"`
	OS       string                           `json:"os"`
	Release  string                           `json:"release"`
	Variant  string                           `json:"variant"`
	Versions map[string]streamsProductVersion `json:"versions"`
}

// streamsProductVersion 产品版本信息
type streamsProductVersion struct {
	Items map[string]streamsProductItem `json:"items"`
}

// streamsProductItem 产品文件项
type streamsProductItem struct {
	Ftype string `json:"ftype"`
	Size  int64  `json:"size"`
	Path  string `json:"path"`
}

// getNodeArch 从 system_info 中解析节点架构
// system_info.cpu.architecture 存的是 runtime.GOARCH 值 (amd64/arm64)
// streams API 中 arch 也是 amd64/arm64 格式
// incus image list 返回的是 x86_64/aarch64 格式
func getNodeArch(node *models.Node) string {
	if node.SystemInfo == "" || node.SystemInfo == "{}" {
		return ""
	}
	var sysInfo struct {
		CPU struct {
			Architecture string `json:"architecture"`
		} `json:"cpu"`
	}
	if err := json.Unmarshal([]byte(node.SystemInfo), &sysInfo); err != nil {
		return ""
	}
	return sysInfo.CPU.Architecture
}

// getStreamsBaseURL 根据镜像源配置获取 streams 基础 URL
// 优先使用节点配置，其次使用站点配置，最后使用默认值
func getStreamsBaseURL(node *models.Node, siteConfig *models.SiteConfig) string {
	if node != nil && node.ImageRemoteURL != "" {
		return StreamsRemoteToBaseURL(node.ImageRemoteURL)
	}
	if siteConfig != nil && siteConfig.IncusRemoteURL != "" {
		return StreamsRemoteToBaseURL(siteConfig.IncusRemoteURL)
	}
	return defaultStreamsBaseURL
}

// StreamsRemoteToBaseURL 将 Incus remote 名称或 URL 转换为 streams 基础 URL
func StreamsRemoteToBaseURL(remote string) string {
	remote = strings.TrimSpace(remote)
	remote = strings.TrimSuffix(remote, ":")
	switch remote {
	case "", "spiritlhl":
		return defaultStreamsBaseURL
	case "images":
		return "https://images.linuxcontainers.org"
	default:
		if strings.HasPrefix(remote, "http://") || strings.HasPrefix(remote, "https://") {
			return strings.TrimRight(remote, "/")
		}
		return defaultStreamsBaseURL
	}
}

// getRemoteName 获取 Incus remote 名称，用于 agent 端 incus image copy
// 优先使用节点配置，其次使用站点配置
// 返回的是 Incus remote 名称（如 spiritlhl:、images:、tsukiyo-mirror:），
// agent 端需确保该 remote 已注册
func getRemoteName(node *models.Node, siteConfig *models.SiteConfig) string {
	raw := ""
	if node != nil && node.ImageRemoteURL != "" {
		raw = strings.TrimSpace(node.ImageRemoteURL)
	} else if siteConfig != nil && siteConfig.IncusRemoteURL != "" {
		raw = strings.TrimSpace(siteConfig.IncusRemoteURL)
	} else {
		return defaultRemoteName
	}
	raw = strings.TrimSuffix(raw, ":")
	switch raw {
	case "", "spiritlhl":
		return "spiritlhl:"
	case "images":
		return "images:"
	default:
		if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
			return "tsukiyo-mirror:"
		}
		return defaultRemoteName
	}
}

// getRemoteURL 获取 remote 对应的服务器 URL，用于 agent 端注册 remote
func getRemoteURL(node *models.Node, siteConfig *models.SiteConfig) string {
	return StreamsRemoteToBaseURL(
		func() string {
			if node != nil && node.ImageRemoteURL != "" {
				return node.ImageRemoteURL
			}
			if siteConfig != nil && siteConfig.IncusRemoteURL != "" {
				return siteConfig.IncusRemoteURL
			}
			return ""
		}(),
	)
}

// streamsIndexEntry streams index.json 中的条目
type streamsIndexEntry struct {
	Datatype string `json:"datatype"`
	Path     string `json:"path"`
	Format   string `json:"format"`
}

// streamsIndexResponse streams index.json 响应
type streamsIndexResponse struct {
	Index map[string]streamsIndexEntry `json:"index"`
}

// fetchStreamsAPI 先请求 index.json 发现 products JSON 路径，再请求 products 数据
func fetchStreamsAPI(baseURL string) (*streamsAPIResponse, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	base := strings.TrimRight(baseURL, "/")

	// 第一步：请求 index.json 获取 products JSON 路径
	indexURL := base + "/streams/v1/index.json"
	indexResp, err := client.Get(indexURL)
	if err != nil {
		zap.L().Error("请求 streams index 失败", zap.String("url", indexURL), zap.Error(err))
		return nil, fmt.Errorf("请求 streams index 失败: %w", err)
	}
	defer indexResp.Body.Close()

	var productsPath string
	if indexResp.StatusCode == http.StatusOK {
		indexBody, err := io.ReadAll(indexResp.Body)
		if err == nil {
			var indexData streamsIndexResponse
			if json.Unmarshal(indexBody, &indexData) == nil {
				// 查找 datatype=image-downloads 且 format=products:1.0 的条目
				for _, entry := range indexData.Index {
					if entry.Datatype == "image-downloads" && entry.Format == "products:1.0" {
						productsPath = entry.Path
					}
				}
			} else {
				zap.L().Error("解析 streams index JSON 失败")
			}
		} else {
			zap.L().Error("读取 streams index 响应失败", zap.Error(err))
		}
	} else {
		zap.L().Warn("streams index 状态码非 200", zap.Int("status", indexResp.StatusCode))
	}
	// 如果 index.json 请求失败，回退到默认路径
	if productsPath == "" {
		productsPath = "streams/v1/images.json"
	}

	// 第二步：请求 products JSON
	productsURL := base + "/" + productsPath
	resp, err := client.Get(productsURL)
	if err != nil {
		zap.L().Error("请求 streams products 失败", zap.String("url", productsURL), zap.Error(err))
		return nil, fmt.Errorf("请求 streams products 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		zap.L().Error("streams products 状态码非 200", zap.Int("status", resp.StatusCode), zap.String("url", productsURL))
		return nil, fmt.Errorf("streams products 返回状态码 %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 streams products 响应失败: %w", err)
	}

	var result streamsAPIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		zap.L().Error("解析 streams products 响应失败", zap.Error(err))
		return nil, fmt.Errorf("解析 streams products 响应失败: %w", err)
	}

	return &result, nil
}

// parseStreamsProducts 将 streams API 产品数据解析为镜像列表
// 只返回 cloud 变体的镜像，且只返回指定架构的
func parseStreamsProducts(products map[string]streamsProduct, nodeArch string) []ImageInfo {
	result := make([]ImageInfo, 0)

	for _, product := range products {
		// 只显示 cloud 变体
		if product.Variant != "cloud" {
			continue
		}

		// 架构过滤：只显示与节点架构匹配的镜像
		if nodeArch != "" && product.Arch != nodeArch {
			continue
		}

		// 构造 alias: distro/release/variant (使用小写 distro 名称，与 Incus alias 一致)
		distroName := product.Distro
		if distroName == "" {
			distroName = strings.ToLower(product.OS)
		}
		alias := fmt.Sprintf("%s/%s/%s", distroName, product.Release, product.Variant)

		// 从最新版本中判断镜像类型和大小
		hasContainer := false
		hasVM := false
		var containerSize int64
		var vmSize int64
		for _, version := range product.Versions {
			for _, item := range version.Items {
				switch item.Ftype {
				case "root.tar.xz", "root.squashfs", "rootfs.tar.xz", "rootfs.squashfs", "squashfs":
					hasContainer = true
					if item.Size > containerSize {
						containerSize = item.Size
					}
				case "incus.tar.xz":
					hasContainer = true
				case "disk-kvm.img":
					hasVM = true
					if item.Size > vmSize {
						vmSize = item.Size
					}
				}
			}
		}

		// 构造描述
		description := fmt.Sprintf("%s %s %s (%s)", product.OS, product.Release, product.Arch, product.Variant)

		// 生成容器镜像条目
		if hasContainer {
			imageKey := fmt.Sprintf("%s|container|%s", alias, product.Arch)
			result = append(result, ImageInfo{
				ID:          imageKey,
				Alias:       alias,
				Name:        description,
				Type:        "container",
				Distro:      product.OS,
				Release:     product.Release,
				Arch:        product.Arch,
				Description: description,
				TotalBytes:  containerSize,
			})
		}

		// 生成虚拟机镜像条目
		if hasVM {
			imageKey := fmt.Sprintf("%s|virtual-machine|%s", alias, product.Arch)
			result = append(result, ImageInfo{
				ID:          imageKey,
				Alias:       alias,
				Name:        description,
				Type:        "virtual-machine",
				Distro:      product.OS,
				Release:     product.Release,
				Arch:        product.Arch,
				Description: description,
				TotalBytes:  vmSize,
			})
		}
	}

	return result
}

// RefreshImageCache 从 streams API 拉取镜像列表并写入数据库缓存
func (s *ImageService) RefreshImageCache(baseURL, nodeArch string) error {
	// 请求 streams API
	streamsData, err := fetchStreamsAPI(baseURL)
	if err != nil {
		return fmt.Errorf("获取 streams API 失败: %w", err)
	}

	// 解析产品数据
	images := parseStreamsProducts(streamsData.Products, nodeArch)

	// 先删除该源的旧缓存
	if err := db.DB.Where("source_url = ?", baseURL).Delete(&models.ImageCache{}).Error; err != nil {
		return fmt.Errorf("清除旧缓存失败: %w", err)
	}

	// 写入新缓存
	for _, img := range images {
		cache := models.ImageCache{
			SourceURL:   baseURL,
			ImageKey:    img.ID,
			Alias:       img.Alias,
			Name:        img.Name,
			Type:        img.Type,
			Distro:      img.Distro,
			Release:     img.Release,
			Arch:        img.Arch,
			Description: img.Description,
			TotalBytes:  img.TotalBytes,
		}
		if err := db.DB.Create(&cache).Error; err != nil {
			zap.L().Warn("写入镜像缓存失败", zap.String("image_key", img.ID), zap.Error(err))
		}
	}

	zap.L().Info("镜像缓存刷新成功", zap.String("source_url", baseURL), zap.Int("count", len(images)))
	return nil
}

// ListImages 获取远程镜像列表（用于下载 tab，从数据库缓存读取，支持搜索和分页）
func (s *ImageService) ListImages(nodeID uuid.UUID, search string, filterType, filterArch, filterDistro string, downloadedOnly bool, page, perPage int) ([]ImageInfo, error) {
	// 查询节点信息
	var node models.Node
	if err := db.DB.First(&node, "id = ?", nodeID).Error; err != nil {
		return nil, fmt.Errorf("节点不存在: %w", err)
	}

	// 获取站点配置
	var siteConfig models.SiteConfig
	if err := db.DB.First(&siteConfig).Error; err != nil {
		return nil, err
	}

	// 获取节点架构
	nodeArch := getNodeArch(&node)
	if nodeArch == "" {
		nodeArch = "amd64"
	}

	// 获取镜像源 URL
	baseURL := getStreamsBaseURL(&node, &siteConfig)

	// 从数据库缓存读取镜像列表（按 alias 排序确保顺序稳定）
	var cached []models.ImageCache
	if err := db.DB.Where("source_url = ?", baseURL).Order("alias ASC, type ASC").Find(&cached).Error; err != nil {
		return nil, fmt.Errorf("读取镜像缓存失败: %w", err)
	}

	// 如果缓存为空，自动刷新一次
	if len(cached) == 0 {
		if err := s.RefreshImageCache(baseURL, nodeArch); err != nil {
			return nil, fmt.Errorf("刷新镜像缓存失败: %w", err)
		}
		if err := db.DB.Where("source_url = ?", baseURL).Order("alias ASC, type ASC").Find(&cached).Error; err != nil {
			return nil, fmt.Errorf("读取镜像缓存失败: %w", err)
		}
	}

	// 转换为 ImageInfo，建立 imageKey -> ImageInfo 的 map
	images := make([]ImageInfo, 0, len(cached))
	imageMap := make(map[string]bool, len(cached))
	for _, c := range cached {
		images = append(images, ImageInfo{
			ID:          c.ImageKey,
			Alias:       c.Alias,
			Name:        c.Name,
			Type:        c.Type,
			Distro:      c.Distro,
			Release:     c.Release,
			Arch:        c.Arch,
			Description: c.Description,
			TotalBytes:  c.TotalBytes,
		})
		imageMap[c.ImageKey] = true
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

	// 从 DB 加载已安装镜像（按 image_key 格式匹配）
	var nodeImages []models.NodeImage
	db.DB.Where("node_id = ?", nodeID.String()).Find(&nodeImages)
	for _, ni := range nodeImages {
		imageKey := ni.Alias + "|" + ni.ImageType + "|" + ni.Architecture
		switch ni.Status {
		case "downloaded":
			statusMap[imageKey] = &mergedStatus{Downloaded: true, Stage: "done", Progress: 100}
		case "downloading":
			statusMap[imageKey] = &mergedStatus{Stage: "downloading", Progress: 0}
		case "error":
			statusMap[imageKey] = &mergedStatus{Stage: "error", Error: ni.Status}
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

	// 补充：已下载但不在当前源缓存中的镜像，从其他源的缓存中查找元数据
	for _, ni := range nodeImages {
		if ni.Status != "downloaded" {
			continue
		}
		imageKey := ni.Alias + "|" + ni.ImageType + "|" + ni.Architecture
		if imageMap[imageKey] {
			continue
		}
		// 从所有源的缓存中查找该镜像的元数据
		var otherCache models.ImageCache
		if err := db.DB.Where("image_key = ?", imageKey).First(&otherCache).Error; err == nil {
			images = append(images, ImageInfo{
				ID:          otherCache.ImageKey,
				Alias:       otherCache.Alias,
				Name:        otherCache.Name,
				Type:        otherCache.Type,
				Distro:      otherCache.Distro,
				Release:     otherCache.Release,
				Arch:        otherCache.Arch,
				Description: otherCache.Description,
				TotalBytes:  otherCache.TotalBytes,
			})
			imageMap[imageKey] = true
		}
	}

	// 合并 + 过滤
	result := make([]ImageInfo, 0, len(images))
	for _, img := range images {
		// 搜索：匹配 alias、name、distro、release、description
		if search != "" {
			matched := false
			if strings.Contains(strings.ToLower(img.Alias), strings.ToLower(search)) ||
				strings.Contains(strings.ToLower(img.Name), strings.ToLower(search)) ||
				strings.Contains(strings.ToLower(img.Distro), strings.ToLower(search)) ||
				strings.Contains(strings.ToLower(img.Release), strings.ToLower(search)) ||
				strings.Contains(strings.ToLower(img.Description), strings.ToLower(search)) {
				matched = true
			}
			if !matched {
				continue
			}
		}
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
		// 架构过滤：始终按节点架构过滤
		archFilter := filterArch
		if archFilter == "" {
			archFilter = nodeArch
		}
		if archFilter != "" {
			if img.Arch != archFilter && img.Arch != archAlias(archFilter) {
				continue
			}
		}
		// 发行版过滤
		if filterDistro != "" && img.Distro != filterDistro {
			continue
		}

		info := img
		if st, ok := statusMap[img.ID]; ok {
			info.Downloaded = st.Downloaded
			info.Stage = st.Stage
			info.Progress = st.Progress
			info.DownloadedBytes = st.DownloadedBytes
			info.TotalBytes = st.TotalBytes
			info.SpeedBps = st.SpeedBps
			info.Error = st.Error
		}

		// 仅已下载过滤
		if downloadedOnly && !info.Downloaded {
			continue
		}

		result = append(result, info)
	}

	// 分页（perPage <= 0 时返回全部）
	if perPage > 0 {
		offset := (page - 1) * perPage
		if offset >= len(result) {
			return []ImageInfo{}, nil
		}
		end := offset + perPage
		if end > len(result) {
			end = len(result)
		}
		result = result[offset:end]
	}

	return result, nil
}

// ListInstalledImages 获取节点已安装镜像列表（agent上报，合并用户分类和别名）
func (s *ImageService) ListInstalledImages(nodeID uuid.UUID, imageType string) ([]InstalledImage, error) {
	nodeStr := nodeID.String()

	// 查询 node_images
	var nodeImages []models.NodeImage
	query := db.DB.Where("node_id = ?", nodeStr)
	if imageType != "" {
		query = query.Where("image_type = ?", imageType)
	}
	query.Order("alias ASC").Find(&nodeImages)

	// 查询 node_image_aliases
	var aliases []models.NodeImageAlias
	aliasQuery := db.DB.Where("node_id = ?", nodeStr)
	if imageType != "" {
		aliasQuery = aliasQuery.Where("image_type = ?", imageType)
	}
	aliasQuery.Find(&aliases)

	// 构建 alias map: fingerprint|type -> NodeImageAlias
	aliasMap := make(map[string]*models.NodeImageAlias)
	for i := range aliases {
		key := aliases[i].Fingerprint + "|" + aliases[i].ImageType
		aliasMap[key] = &aliases[i]
	}

	// 查询所有分类
	var categories []models.NodeImageCategory
	catQuery := db.DB.Where("node_id = ?", nodeStr)
	if imageType != "" {
		catQuery = catQuery.Where("image_type = ?", imageType)
	}
	catQuery.Order("sort_order ASC, name ASC").Find(&categories)

	catMap := make(map[string]*models.NodeImageCategory)
	for i := range categories {
		catMap[categories[i].ID.String()] = &categories[i]
	}

	// 合并
	result := make([]InstalledImage, 0, len(nodeImages))
	for _, ni := range nodeImages {
		img := InstalledImage{
			ID:           ni.Fingerprint + "|" + ni.ImageType,
			Fingerprint:  ni.Fingerprint,
			Alias:        ni.Alias,
			DisplayName:  ni.Alias,
			Type:         ni.ImageType,
			Architecture: ni.Architecture,
			Size:         ni.SizeBytes,
			Description:  ni.Description,
			ImageSource:  ni.ImageSource,
			UploadDate:   ni.UploadDate,
		}

		// 合并用户别名
		key := ni.Fingerprint + "|" + ni.ImageType
		if a, ok := aliasMap[key]; ok {
			img.DisplayName = a.DisplayName
			img.InstallSSH = a.InstallSSH
			if a.CategoryID != nil {
				catID := a.CategoryID.String()
				img.CategoryID = &catID
				if cat, ok := catMap[catID]; ok {
					img.CategoryName = cat.Name
				}
			}
		}

		result = append(result, img)
	}

	return result, nil
}

// ListCategories 获取节点镜像分类列表
func (s *ImageService) ListCategories(nodeID uuid.UUID, imageType string) ([]CategoryInfo, error) {
	nodeStr := nodeID.String()

	var categories []models.NodeImageCategory
	query := db.DB.Where("node_id = ?", nodeStr)
	if imageType != "" {
		query = query.Where("image_type = ?", imageType)
	}
	query.Order("sort_order ASC, name ASC").Find(&categories)

	result := make([]CategoryInfo, 0, len(categories))
	for _, c := range categories {
		result = append(result, CategoryInfo{
			ID:        c.ID.String(),
			Name:      c.Name,
			ImageType: c.ImageType,
			SortOrder: c.SortOrder,
		})
	}
	return result, nil
}

// CreateCategory 创建镜像分类
func (s *ImageService) CreateCategory(nodeID uuid.UUID, name, imageType string) (*CategoryInfo, error) {
	nodeStr := nodeID.String()

	cat := models.NodeImageCategory{
		NodeID:    nodeStr,
		Name:      name,
		ImageType: imageType,
	}
	if err := db.DB.Create(&cat).Error; err != nil {
		return nil, fmt.Errorf("创建分类失败: %w", err)
	}

	return &CategoryInfo{
		ID:        cat.ID.String(),
		Name:      cat.Name,
		ImageType: cat.ImageType,
		SortOrder: cat.SortOrder,
	}, nil
}

// UpdateCategory 更新镜像分类
func (s *ImageService) UpdateCategory(categoryID uuid.UUID, name string, sortOrder int) error {
	updates := map[string]interface{}{}
	if name != "" {
		updates["name"] = name
	}
	updates["sort_order"] = sortOrder
	return db.DB.Model(&models.NodeImageCategory{}).Where("id = ?", categoryID).Updates(updates).Error
}

// DeleteCategory 删除镜像分类
func (s *ImageService) DeleteCategory(categoryID uuid.UUID) error {
	return db.DB.Where("id = ?", categoryID).Delete(&models.NodeImageCategory{}).Error
}

// UpdateImageAlias 更新镜像别名（分类、显示名、install_ssh）
// categoryName 非空时按名称查找或创建分类，categoryName 为空字符串时清除分类
func (s *ImageService) UpdateImageAlias(nodeID uuid.UUID, fingerprint, imageType string, categoryID *uuid.UUID, categoryName *string, displayName string, installSSH *bool) error {
	nodeStr := nodeID.String()

	// 如果传了分类名称，查找或创建分类
	if categoryName != nil && *categoryName != "" {
		var cat models.NodeImageCategory
		err := db.DB.Where("node_id = ? AND name = ? AND image_type = ?", nodeStr, *categoryName, imageType).First(&cat).Error
		if err != nil {
			// 创建新分类
			cat = models.NodeImageCategory{
				NodeID:    nodeStr,
				Name:      *categoryName,
				ImageType: imageType,
			}
			if err := db.DB.Create(&cat).Error; err != nil {
				return fmt.Errorf("创建分类失败: %w", err)
			}
		}
		catID := cat.ID
		categoryID = &catID
	} else if categoryName != nil && *categoryName == "" {
		categoryID = nil
	}

	updates := map[string]interface{}{}
	if categoryID != nil {
		updates["category_id"] = *categoryID
	} else {
		updates["category_id"] = nil
	}
	if displayName != "" {
		updates["display_name"] = displayName
	}
	if installSSH != nil {
		updates["install_ssh"] = *installSSH
	}

	result := db.DB.Model(&models.NodeImageAlias{}).Where("node_id = ? AND fingerprint = ? AND image_type = ?", nodeStr, fingerprint, imageType).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		// 记录不存在，创建
		alias := models.NodeImageAlias{
			NodeID:      nodeStr,
			Fingerprint: fingerprint,
			ImageType:   imageType,
			DisplayName: displayName,
		}
		if categoryID != nil {
			alias.CategoryID = categoryID
		}
		if installSSH != nil {
			alias.InstallSSH = *installSSH
		}
		return db.DB.Create(&alias).Error
	}
	return nil
}

// SyncNodeImages 触发 agent 同步镜像列表
func (s *ImageService) SyncNodeImages(nodeID uuid.UUID) error {
	if s.agentMgr == nil || !s.agentMgr.IsNodeConnected(nodeID) {
		return service.ErrNodeNotConnected
	}
	taskID := uuid.New()
	return s.agentMgr.SendTask(agent.TaskMessage{
		NodeID: nodeID,
		TaskID: taskID,
		Type:   "sync_images",
	})
}

// ListReinstallImages 获取重装系统可选镜像列表（按分类分组）
func (s *ImageService) ListReinstallImages(nodeID uuid.UUID, imageType string) (map[string][]InstalledImage, error) {
	images, err := s.ListInstalledImages(nodeID, imageType)
	if err != nil {
		return nil, err
	}

	// 按分类分组
	result := make(map[string][]InstalledImage)
	for _, img := range images {
		catName := img.CategoryName
		if catName == "" {
			catName = "未分类"
		}
		result[catName] = append(result[catName], img)
	}
	return result, nil
}

// archAlias 架构名称转换: x86_64 <-> amd64, aarch64 <-> arm64
func archAlias(arch string) string {
	switch arch {
	case "x86_64":
		return "amd64"
	case "aarch64":
		return "arm64"
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	default:
		return arch
	}
}

// DownloadImage 下载镜像
func (s *ImageService) DownloadImage(nodeID uuid.UUID, imageKey string, userID uint) (*uuid.UUID, error) {
	parts := strings.SplitN(imageKey, "|", 3)
	if len(parts) != 3 {
		return nil, service.ErrInvalidImageKeyFormat
	}
	alias := parts[0]
	imageType := parts[1]
	imageArch := parts[2]

	// 查询节点信息
	var node models.Node
	if err := db.DB.First(&node, "id = ?", nodeID).Error; err != nil {
		return nil, fmt.Errorf("节点不存在: %w", err)
	}

	// 获取节点架构并校验
	nodeArch := getNodeArch(&node)
	if nodeArch == "" {
		nodeArch = "amd64"
	}
	if imageArch != nodeArch {
		return nil, fmt.Errorf("镜像架构 %s 与节点架构 %s 不一致，无法下载", imageArch, nodeArch)
	}

	// 获取站点配置
	var siteConfig models.SiteConfig
	if err := db.DB.First(&siteConfig).Error; err != nil {
		return nil, err
	}

	// 获取 remote 名称和 URL（优先节点配置，其次站点配置）
	remote := getRemoteName(&node, &siteConfig)
	remoteURL := getRemoteURL(&node, &siteConfig)

	if s.agentMgr == nil || !s.agentMgr.IsNodeConnected(nodeID) {
		return nil, service.ErrNodeNotConnected
	}

	// 构造任务 payload
	taskPayload := map[string]string{
		"image_key":   imageKey,
		"image_type":  imageType,
		"source":      remote + alias,
		"remote_name": strings.TrimSuffix(remote, ":"),
		"remote_url":  remoteURL,
	}
	payloadBytes, _ := json.Marshal(taskPayload)

	// DB 状态记录（使用 image_key 作为临时标识，agent 同步后会更新为完整记录）
	nodeIDStr := nodeID.String()
	var ni models.NodeImage
	if dbErr := db.DB.Where("node_id = ? AND alias = ? AND image_type = ?", nodeIDStr, alias, imageType).First(&ni).Error; dbErr != nil {
		db.DB.Create(&models.NodeImage{NodeID: nodeIDStr, Alias: alias, ImageType: imageType, Status: "downloading"})
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

	parts := strings.SplitN(imageKey, "|", 3)
	alias := ""
	if len(parts) > 0 {
		alias = parts[0]
	}
	imgType := ""
	if len(parts) > 1 {
		imgType = parts[1]
	}

	var ni models.NodeImage
	dbErr := db.DB.Where("node_id = ? AND alias = ? AND image_type = ?", nodeIDStr, alias, imgType).First(&ni).Error
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
	parts := strings.SplitN(imageKey, "|", 3)
	alias := ""
	if len(parts) > 0 {
		alias = parts[0]
	}
	imgType := ""
	if len(parts) > 1 {
		imgType = parts[1]
	}
	db.DB.Where("node_id = ? AND alias = ? AND image_type = ?", nodeID.String(), alias, imgType).Delete(&models.NodeImage{})
	db.DB.Where("node_id = ? AND alias = ? AND image_type = ?", nodeID.String(), alias, imgType).Delete(&models.NodeImageAlias{})

	return &taskID, nil
}

// ListRemoteImages 获取远程镜像列表（异步任务）
func (s *ImageService) ListRemoteImages(nodeID uuid.UUID, remote string) (*uuid.UUID, error) {
	if s.agentMgr == nil || !s.agentMgr.IsNodeConnected(nodeID) {
		return nil, service.ErrNodeNotConnected
	}

	if remote == "" {
		remote = defaultRemoteName
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
