package task

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/creack/pty"
	"go.uber.org/zap"

	"tsukiyo/agent/internal/image"
	"tsukiyo/agent/internal/incus"
	"tsukiyo/agent/internal/ws"
)

// handleDownloadImage 处理镜像下载任务
// image_key 格式: alias|type|arch
func (e *Executor) handleDownloadImage(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ImageKey   string `json:"image_key"`
		ImageType  string `json:"image_type"`  // container / vm
		Source     string `json:"source"`      // incus remote source, e.g. "spiritlhl:debian/12/cloud"
		RemoteName string `json:"remote_name"` // remote 名称, e.g. spiritlhl
		RemoteURL  string `json:"remote_url"`  // remote 服务器 URL, e.g. https://incusimages.spiritlhl.net
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("解析下载参数失败: %w", err)
	}

	// 从 image_key 解析出 alias, type, arch
	alias, imageType, arch := incus.ParseImageKey(req.ImageKey)

	// 兼容旧字段：如果 image_type 有值，优先使用
	if req.ImageType != "" {
		imageType = req.ImageType
	}

	zap.L().Info("开始下载镜像",
		zap.String("image_key", req.ImageKey),
		zap.String("alias", alias),
		zap.String("type", imageType),
		zap.String("arch", arch),
		zap.String("source", req.Source),
		zap.String("remote_name", req.RemoteName),
		zap.String("remote_url", req.RemoteURL))

	// 确保 remote 已注册到 Incus
	if req.RemoteName != "" && req.RemoteURL != "" {
		if err := e.incusClient.SyncRemote(req.RemoteName, req.RemoteURL); err != nil {
			zap.L().Warn("确保 remote 注册失败，继续尝试下载", zap.String("remote", req.RemoteName), zap.Error(err))
		}
	}

	// Incus 远程镜像：使用 incus image copy
	if strings.HasPrefix(req.Source, "images:") || strings.Contains(req.Source, ":") {
		return e.downloadIncusImage(req.ImageKey, alias, req.Source, imageType)
	}

	// 自定义 URL 下载
	switch imageType {
	case "container":
		return e.downloadContainerImage(req.ImageKey, alias, req.Source)
	case "vm", "virtual-machine":
		return e.downloadVMImage(req.ImageKey, alias, req.Source)
	default:
		return nil, fmt.Errorf("未知镜像类型: %s", imageType)
	}
}

// downloadIncusImage 使用 incus image copy 命令下载 Incus 官方镜像
func (e *Executor) downloadIncusImage(imageKey, alias, source, imageType string) (json.RawMessage, error) {
	e.wsClient.SendImageProgress(ws.ImageProgressPayload{
		ImageID:  imageKey,
		Stage:    "downloading",
		Progress: 0,
	})

	// incus image copy <source> local: --alias <alias>
	// 如果是虚拟机类型需要 --vm
	args := []string{"image", "copy", source, "local:", "--alias", alias, "--verbose"}
	if imageType == "vm" || imageType == "virtual-machine" {
		args = append(args, "--vm")
	}

	zap.L().Info("执行 incus image copy", zap.Strings("args", args))

	// 先删除可能已存在的 alias，避免冲突
	exec.Command("incus", "image", "alias", "delete", alias).Run()

	cmd := exec.Command("incus", args...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	// 使用 pty 模拟终端，使 Incus CLI 输出进度
	ptmx, err := pty.Start(cmd)
	if err != nil {
		errMsg := err.Error()
		zap.L().Error("incus image copy pty 启动失败", zap.Error(err))
		e.wsClient.SendImageProgress(ws.ImageProgressPayload{
			ImageID: imageKey,
			Stage:   "error",
			Error:   "incus image copy 启动失败: " + errMsg,
		})
		return nil, fmt.Errorf("incus image copy pty 启动失败: %w", err)
	}
	defer ptmx.Close()

	// 设置 pty 窗口大小，使 CLI 进度条正常输出
	pty.Setsize(ptmx, &pty.Winsize{
		Rows: 24,
		Cols: 80,
	})

	// 实时读取 pty 输出并解析进度
	// Incus CLI 进度格式: "Copying the image: rootfs: 28% (42.93MB/s)"
	// 进度用 \r 回车覆盖同一行
	progressDone := make(chan struct{})
	var outputBuf strings.Builder
	lastSendTime := time.Time{}
	go func() {
		defer close(progressDone)
		buf := make([]byte, 4096)
		for {
			n, readErr := ptmx.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				outputBuf.WriteString(chunk)
				// 按 \r 和 \n 分割，逐行解析
				for _, line := range strings.FieldsFunc(chunk, func(r rune) bool { return r == '\r' || r == '\n' }) {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}

					// 解析进度百分比
					if pct, speed, _, _, ok := parseIncusProgress(line); ok {
						// 限制上报频率，最少间隔 0.5 秒
						now := time.Now()
						if now.Sub(lastSendTime) >= 500*time.Millisecond || pct >= 100 {
							lastSendTime = now
							e.wsClient.SendImageProgress(ws.ImageProgressPayload{
								ImageID:  imageKey,
								Stage:    "downloading",
								Progress: pct,
								SpeedBps: speed,
							})
						}
					}
				}
			}
			if readErr != nil {
				break
			}
		}
	}()

	waitErr := cmd.Wait()
	<-progressDone

	if waitErr != nil {
		errMsg := strings.TrimSpace(outputBuf.String())
		zap.L().Error("incus image copy 失败", zap.String("image_key", imageKey), zap.String("output", errMsg), zap.Error(waitErr))
		e.wsClient.SendImageProgress(ws.ImageProgressPayload{
			ImageID: imageKey,
			Stage:   "error",
			Error:   "incus image copy 失败: " + errMsg,
		})
		return nil, fmt.Errorf("incus image copy 失败: %w, output: %s", waitErr, errMsg)
	}

	zap.L().Info("incus image copy 成功", zap.String("image_key", imageKey), zap.String("alias", alias))

	e.wsClient.SendImageProgress(ws.ImageProgressPayload{
		ImageID:  imageKey,
		Stage:    "done",
		Progress: 100,
	})

	return json.Marshal(map[string]interface{}{
		"image_key": imageKey,
		"alias":     alias,
		"status":    "success",
	})
}

// parseIncusProgress 解析 Incus CLI 进度输出行
// 格式: "Transfer: 45% (10.5MB/s)" 或 "Transfer: 45% (100MB/200MB) 10.5MB/s"
// 返回: 百分比, 速度(bytes/s), 已下载(bytes), 总大小(bytes), 是否解析成功
func parseIncusProgress(line string) (progress int, speedBps int64, downloadedBytes int64, totalBytes int64, ok bool) {
	// 查找 "Transfer:" 或 "Image export:" 等进度前缀
	if !strings.Contains(line, "%") {
		return 0, 0, 0, 0, false
	}

	// 提取百分比
	pctIdx := strings.Index(line, "%")
	if pctIdx < 0 {
		return 0, 0, 0, 0, false
	}
	// 向前找数字
	numStart := pctIdx - 1
	for numStart >= 0 && (line[numStart] >= '0' && line[numStart] <= '9') {
		numStart--
	}
	numStart++
	if numStart >= pctIdx {
		return 0, 0, 0, 0, false
	}
	pctStr := line[numStart:pctIdx]
	pct, err := strconv.Atoi(pctStr)
	if err != nil || pct < 0 || pct > 100 {
		return 0, 0, 0, 0, false
	}
	progress = pct
	ok = true

	// 提取速度 (括号内)
	if parenIdx := strings.Index(line, "("); parenIdx >= 0 {
		parenEnd := strings.Index(line[parenIdx:], ")")
		if parenEnd > 0 {
			inner := line[parenIdx+1 : parenIdx+parenEnd]
			// 格式: "12.68MB/s" 或 "100MB/200MB 12.68MB/s"
			parts := strings.Fields(inner)
			if len(parts) >= 2 {
				// 格式: "100MB/200MB 12.68MB/s"
				if sz, err := parseSizeToBytes(strings.Split(parts[0], "/")[0]); err == nil {
					downloadedBytes = sz
				}
				if sz, err := parseSizeToBytes(parts[len(parts)-1]); err == nil {
					speedBps = sz
				}
			} else if len(parts) == 1 {
				// 格式: "12.68MB/s"
				if sz, err := parseSizeToBytes(parts[0]); err == nil {
					speedBps = sz
				}
			}
		}
	}

	return
}

// parseSizeToBytes 将 "10.5MB/s"、"100MB"、"1.2GB" 等转换为字节
func parseSizeToBytes(s string) (int64, error) {
	s = strings.TrimSpace(s)
	// 去掉 /s 后缀
	s = strings.TrimSuffix(s, "/s")
	s = strings.TrimSuffix(s, "/s")

	var multiplier float64 = 1
	if strings.HasSuffix(s, "GB") {
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "GB")
	} else if strings.HasSuffix(s, "MB") {
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(s, "MB")
	} else if strings.HasSuffix(s, "KB") {
		multiplier = 1024
		s = strings.TrimSuffix(s, "KB")
	} else if strings.HasSuffix(s, "B") {
		multiplier = 1
		s = strings.TrimSuffix(s, "B")
	}

	val, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, err
	}
	return int64(val * multiplier), nil
}

// downloadContainerImage 使用 downloader 模块下载容器镜像（自定义 URL）
func (e *Executor) downloadContainerImage(imageKey, alias, source string) (json.RawMessage, error) {
	cacheDir := "/var/cache/tsukiyo/images"
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("创建缓存目录失败: %w", err)
	}

	e.wsClient.SendImageProgress(ws.ImageProgressPayload{
		ImageID:  imageKey,
		Stage:    "downloading",
		Progress: 0,
	})

	dm := image.NewDownloadManager(cacheDir)
	downloadTask, err := dm.DownloadImage(context.Background(), alias, source, func(taskID string, downloaded, total int64, status image.DownloadStatus) {
		progress := 0
		if total > 0 {
			progress = int(float64(downloaded) / float64(total) * 100)
		}
		speedBps := int64(0)
		if downloaded > 0 {
			speedBps = downloaded * 2
		}
		e.wsClient.SendImageProgress(ws.ImageProgressPayload{
			ImageID:         imageKey,
			Stage:           "downloading",
			Progress:        progress,
			DownloadedBytes: downloaded,
			TotalBytes:      total,
			SpeedBps:        speedBps,
		})
	})
	if err != nil {
		e.wsClient.SendImageProgress(ws.ImageProgressPayload{
			ImageID: imageKey,
			Stage:   "error",
			Error:   "创建下载任务失败: " + err.Error(),
		})
		return nil, fmt.Errorf("创建下载任务失败: %w", err)
	}

	target := downloadTask.TargetPath

	if downloadTask.GetStatus() == image.StatusCompleted {
		zap.L().Info("镜像已下载完成，跳过等待", zap.String("image_key", imageKey))
	} else {
		for {
			time.Sleep(500 * time.Millisecond)
			status := downloadTask.GetStatus()
			if status == image.StatusCompleted {
				break
			}
			if status == image.StatusFailed {
				e.wsClient.SendImageProgress(ws.ImageProgressPayload{
					ImageID: imageKey,
					Stage:   "error",
					Error:   downloadTask.Error,
				})
				return nil, fmt.Errorf("下载失败: %s", downloadTask.Error)
			}
		}
	}

	e.wsClient.SendImageProgress(ws.ImageProgressPayload{
		ImageID:  imageKey,
		Stage:    "importing",
		Progress: 100,
	})

	importCmd := exec.Command("incus", "image", "import", target, "--alias", alias)
	importOutput, importErr := importCmd.CombinedOutput()
	if importErr != nil {
		_ = os.Remove(target)
		e.wsClient.SendImageProgress(ws.ImageProgressPayload{
			ImageID: imageKey,
			Stage:   "error",
			Error:   "导入失败: " + string(importOutput),
		})
		return nil, fmt.Errorf("导入 Incus 失败: %w, output: %s", importErr, string(importOutput))
	}
	zap.L().Info("容器镜像导入成功", zap.String("image_key", imageKey))

	_ = os.Remove(target)

	e.wsClient.SendImageProgress(ws.ImageProgressPayload{
		ImageID:  imageKey,
		Stage:    "done",
		Progress: 100,
	})

	go func() {
		aliases, err := e.incusClient.ListImages()
		if err == nil {
			e.wsClient.SendLocalImages(aliases)
		}
	}()

	return json.Marshal(map[string]string{"status": "completed"})
}

// downloadVMImage 下载 VM 镜像（自定义 URL）
func (e *Executor) downloadVMImage(imageKey, alias, url string) (json.RawMessage, error) {
	if url == "" {
		return nil, fmt.Errorf("VM 镜像 %s 无下载地址", alias)
	}

	cacheDir := "/var/cache/tsukiyo/images"
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("创建缓存目录失败: %w", err)
	}

	// 用 alias 中的斜杠替换成下划线作为文件名
	safeAlias := strings.ReplaceAll(alias, "/", "_")
	target := filepath.Join(cacheDir, safeAlias+".qcow2")

	if _, err := os.Stat(target); err == nil {
		e.wsClient.SendImageProgress(ws.ImageProgressPayload{
			ImageID:  imageKey,
			Stage:    "done",
			Progress: 100,
		})
		return json.Marshal(map[string]string{"status": "already_exists"})
	}

	e.wsClient.SendImageProgress(ws.ImageProgressPayload{
		ImageID:  imageKey,
		Stage:    "downloading",
		Progress: 0,
	})

	dm := image.NewDownloadManager(cacheDir)
	task, err := dm.DownloadImage(context.Background(), alias, url)
	if err != nil {
		e.wsClient.SendImageProgress(ws.ImageProgressPayload{
			ImageID: imageKey,
			Stage:   "error",
			Error:   "创建下载任务失败: " + err.Error(),
		})
		return nil, fmt.Errorf("创建下载任务失败: %w", err)
	}

	task.SetProgressCallback(func(taskID string, downloaded, total int64, status image.DownloadStatus) {
		progress := 0
		if total > 0 {
			progress = int(float64(downloaded) / float64(total) * 100)
		}
		speedBps := int64(0)
		if task.Downloaded > 0 && !task.CreatedAt.IsZero() {
			elapsed := time.Since(task.CreatedAt).Seconds()
			if elapsed > 0 {
				speedBps = int64(float64(downloaded) / elapsed)
			}
		}
		e.wsClient.SendImageProgress(ws.ImageProgressPayload{
			ImageID:         imageKey,
			Stage:           "downloading",
			Progress:        progress,
			DownloadedBytes: downloaded,
			TotalBytes:      total,
			SpeedBps:        speedBps,
		})
	})

	for {
		time.Sleep(500 * time.Millisecond)
		status := task.GetStatus()
		if status == image.StatusCompleted {
			break
		}
		if status == image.StatusFailed {
			e.wsClient.SendImageProgress(ws.ImageProgressPayload{
				ImageID: imageKey,
				Stage:   "error",
				Error:   task.Error,
			})
			return nil, fmt.Errorf("下载失败: %s", task.Error)
		}
	}

	e.wsClient.SendImageProgress(ws.ImageProgressPayload{
		ImageID:  imageKey,
		Stage:    "converting",
		Progress: 100,
	})

	downloadedFile := task.TargetPath
	if err := normalizeQCOW2(downloadedFile, target); err != nil {
		_ = os.Remove(downloadedFile)
		_ = os.Remove(target)
		e.wsClient.SendImageProgress(ws.ImageProgressPayload{
			ImageID: imageKey,
			Stage:   "error",
			Error:   "转换失败: " + err.Error(),
		})
		return nil, fmt.Errorf("转换 qcow2 失败: %w", err)
	}
	_ = os.Remove(downloadedFile)

	e.wsClient.SendImageProgress(ws.ImageProgressPayload{
		ImageID:  imageKey,
		Stage:    "importing",
		Progress: 100,
	})
	if err := e.incusClient.ImportImageFromFile(alias, target); err != nil {
		_ = os.Remove(target)
		e.wsClient.SendImageProgress(ws.ImageProgressPayload{
			ImageID: imageKey,
			Stage:   "error",
			Error:   "导入失败: " + err.Error(),
		})
		return nil, fmt.Errorf("导入 Incus 失败: %w", err)
	}
	zap.L().Info("VM 镜像导入成功", zap.String("image_key", imageKey))

	_ = os.Remove(target)

	e.wsClient.SendImageProgress(ws.ImageProgressPayload{
		ImageID:  imageKey,
		Stage:    "done",
		Progress: 100,
	})

	go func() {
		aliases, err := e.incusClient.ListImages()
		if err == nil {
			e.wsClient.SendLocalImages(aliases)
		}
	}()

	return json.Marshal(map[string]string{"status": "completed"})
}

// handleCancelImageDownload 取消镜像下载
func (e *Executor) handleCancelImageDownload(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ImageKey string `json:"image_key"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}

	alias, _, _ := incus.ParseImageKey(req.ImageKey)

	zap.L().Info("取消镜像下载", zap.String("image_key", req.ImageKey))

	if task := e.downloadManager.GetTask(alias); task != nil {
		task.Cancel()
		e.downloadManager.RemoveTask(alias)
	}

	e.wsClient.SendImageProgress(ws.ImageProgressPayload{
		ImageID: req.ImageKey,
		Stage:   "canceled",
	})

	return json.Marshal(map[string]string{"status": "canceled", "image_key": req.ImageKey})
}

// handleCheckImage 检查镜像是否已下载
func (e *Executor) handleCheckImage(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ImageKey string `json:"image_key"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}

	alias, _, _ := incus.ParseImageKey(req.ImageKey)
	exists := e.incusClient.ImageAliasExists(alias)

	zap.L().Info("检查镜像", zap.String("image_key", req.ImageKey), zap.Bool("exists", exists))

	return json.Marshal(map[string]interface{}{
		"image_key":  req.ImageKey,
		"downloaded": exists,
	})
}

// handleListRemoteImages 获取远程镜像列表
func (e *Executor) handleListRemoteImages(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		Remote string `json:"remote"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}

	if req.Remote == "" {
		req.Remote = "images:"
	}

	zap.L().Info("获取远程镜像列表", zap.String("remote", req.Remote))

	images, err := e.incusClient.ListRemoteImages(req.Remote)
	if err != nil {
		return nil, err
	}

	zap.L().Info("获取远程镜像列表成功", zap.Int("count", len(images)))
	return json.Marshal(map[string]interface{}{
		"images": images,
		"total":  len(images),
	})
}

// handleDeleteImage 删除已下载的镜像
func (e *Executor) handleDeleteImage(payload json.RawMessage) (json.RawMessage, error) {
	var req struct {
		ImageKey string `json:"image_key"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}

	alias, _, _ := incus.ParseImageKey(req.ImageKey)

	zap.L().Info("删除镜像", zap.String("image_key", req.ImageKey), zap.String("alias", alias))

	// 通过 incus image delete 删除
	cmd := exec.Command("incus", "image", "delete", alias)
	if output, err := cmd.CombinedOutput(); err != nil {
		if !strings.Contains(string(output), "Image not found") {
			zap.L().Warn("删除 incus 镜像失败", zap.String("alias", alias), zap.Error(err), zap.String("output", string(output)))
		}
	} else {
		zap.L().Info("删除 incus 镜像成功", zap.String("alias", alias))
	}

	// 删除本地缓存
	safeAlias := strings.ReplaceAll(alias, "/", "_")
	cacheDir := "/var/cache/tsukiyo/images"
	for _, ext := range []string{".tar.gz", ".qcow2"} {
		f := filepath.Join(cacheDir, safeAlias+ext)
		if _, err := os.Stat(f); err == nil {
			_ = os.Remove(f)
		}
	}

	if e.downloadManager != nil {
		e.downloadManager.RemoveTask(alias)
	}

	go func() {
		aliases, err := e.incusClient.ListImages()
		if err == nil {
			e.wsClient.SendLocalImages(aliases)
		}
	}()

	return json.Marshal(map[string]string{"status": "deleted", "image_key": req.ImageKey})
}

// normalizeQCOW2 使用 qemu-img convert 转换镜像为标准 qcow2
func normalizeQCOW2(src, target string) error {
	cmd := exec.Command("qemu-img", "convert", "-O", "qcow2", src, target)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("qemu-img convert 失败: %v, output: %s", err, string(output))
	}
	return nil
}
