package image

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// DownloadStatus 下载状态
type DownloadStatus string

const (
	StatusPending   DownloadStatus = "pending"
	StatusRunning   DownloadStatus = "running"
	StatusPaused    DownloadStatus = "paused"
	StatusCompleted DownloadStatus = "completed"
	StatusFailed    DownloadStatus = "failed"
	StatusCancelled DownloadStatus = "cancelled"
)

// DownloadTask 下载任务
type DownloadTask struct {
	ID          string         `json:"id"`
	Alias       string         `json:"alias"`
	Remote      string         `json:"remote"`
	TargetPath  string         `json:"target_path"`
	URL         string         `json:"url,omitempty"`
	TotalSize   int64          `json:"total_size"`
	Downloaded  int64          `json:"downloaded"`
	Status      DownloadStatus `json:"status"`
	Error       string         `json:"error,omitempty"`
	Concurrency int            `json:"concurrency"`
	RetryCount  int            `json:"retry_count"`
	MaxRetries  int            `json:"max_retries"`
	CreatedAt   time.Time      `json:"created_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	Checksum    string         `json:"checksum,omitempty"`
	HashType    string         `json:"hash_type,omitempty"`

	mu         sync.RWMutex
	cancelFunc context.CancelFunc
	partStates []*PartState
	progressCb ProgressCallback
	lastReport time.Time
}

// PartState 分片状态
type PartState struct {
	Index      int    `json:"index"`
	Start      int64  `json:"start"`
	End        int64  `json:"end"`
	Downloaded int64  `json:"downloaded"`
	Completed  bool   `json:"completed"`
	ETag       string `json:"etag,omitempty"`
}

// ProgressCallback 进度回调
type ProgressCallback func(taskID string, downloaded, total int64, status DownloadStatus)

// DefaultConcurrency 默认并发数
const DefaultConcurrency = 8

// DefaultPartSize 默认分片大小 (10MB)
const DefaultPartSize = 10 * 1024 * 1024

// DefaultMaxRetries 默认最大重试次数
const DefaultMaxRetries = 5

// DefaultTimeout 默认超时
const DefaultTimeout = 30 * time.Minute

// NewDownloadTask 创建下载任务
func NewDownloadTask(alias, remote, targetPath string) *DownloadTask {
	return &DownloadTask{
		ID:          fmt.Sprintf("dl_%d", time.Now().UnixNano()),
		Alias:       alias,
		Remote:      remote,
		TargetPath:  targetPath,
		Status:      StatusPending,
		Concurrency: DefaultConcurrency,
		MaxRetries:  DefaultMaxRetries,
		CreatedAt:   time.Now(),
		partStates:  make([]*PartState, 0),
	}
}

// SetProgressCallback 设置进度回调
func (t *DownloadTask) SetProgressCallback(cb ProgressCallback) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.progressCb = cb
}

// GetStatus 获取状态
func (t *DownloadTask) GetStatus() DownloadStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Status
}

// GetProgress 获取进度
func (t *DownloadTask) GetProgress() (downloaded, total int64) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Downloaded, t.TotalSize
}

// Start 开始下载
func (t *DownloadTask) Start(ctx context.Context) error {
	t.mu.Lock()
	if t.Status == StatusRunning {
		t.mu.Unlock()
		return fmt.Errorf("任务已在运行中")
	}
	t.Status = StatusRunning
	t.Error = ""
	t.mu.Unlock()

	dlCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	t.cancelFunc = cancel

	// 尝试恢复之前的下载
	if err := t.loadState(); err == nil && len(t.partStates) > 0 {
		zap.L().Info("恢复下载", zap.String("task_id", t.ID), zap.Int64("downloaded", t.Downloaded))
	}

	// 获取文件信息
	if err := t.fetchFileInfo(dlCtx); err != nil {
		t.setFailed(err)
		return err
	}

	// 如果没有保存过分片状态，创建新的
	if len(t.partStates) == 0 {
		t.createParts()
	}

	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(t.TargetPath), 0755); err != nil {
		t.setFailed(err)
		return err
	}

	// 创建/打开目标文件
	file, err := os.OpenFile(t.TargetPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.setFailed(err)
		return err
	}
	defer file.Close()

	// 预分配文件大小
	if t.TotalSize > 0 {
		if err := file.Truncate(t.TotalSize); err != nil {
			zap.L().Warn("预分配文件失败", zap.Error(err))
		}
	}

	// 并发下载
	if err := t.downloadParts(dlCtx, file); err != nil {
		t.setFailed(err)
		return err
	}

	// 校验文件
	if t.Checksum != "" {
		if err := t.verifyChecksum(); err != nil {
			t.setFailed(err)
			return err
		}
	}

	// 清理临时状态文件
	t.cleanState()

	t.setCompleted()
	return nil
}

// Pause 暂停下载
func (t *DownloadTask) Pause() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.Status != StatusRunning {
		return fmt.Errorf("任务未在运行")
	}
	if t.cancelFunc != nil {
		t.cancelFunc()
	}
	t.Status = StatusPaused
	t.saveState()
	return nil
}

// Cancel 取消下载
func (t *DownloadTask) Cancel() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cancelFunc != nil {
		t.cancelFunc()
	}
	t.Status = StatusCancelled
	t.cleanState()
	// 删除未完成的文件
	if _, err := os.Stat(t.TargetPath); err == nil {
		os.Remove(t.TargetPath)
	}
	return nil
}

// fetchFileInfo 获取文件信息
func (t *DownloadTask) fetchFileInfo(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", t.Remote, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// 允许最多 10 次重定向
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("获取文件信息失败: %d", resp.StatusCode)
	}

	// 获取文件大小
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		size, err := strconv.ParseInt(cl, 10, 64)
		if err == nil {
			t.TotalSize = size
		}
	}

	// 暂时禁用分片下载，改为单线程下载避免文件损坏
	zap.L().Info("使用单线程下载", zap.String("url", t.Remote))
	t.Concurrency = 1

	// 获取 ETag
	if etag := resp.Header.Get("ETag"); etag != "" {
		// 用于校验
	}

	// 获取文件名
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if idx := strings.Index(cd, "filename="); idx >= 0 {
			fname := cd[idx+9:]
			fname = strings.Trim(fname, `"`)
			if fname != "" {
				// 可以更新目标文件名
			}
		}
	}

	return nil
}

// createParts 创建分片
func (t *DownloadTask) createParts() {
	// 强制使用单线程下载，只创建一个分片
	t.Concurrency = 1
	t.partStates = []*PartState{
		{Index: 0, Start: 0, End: t.TotalSize - 1, Downloaded: 0, Completed: false},
	}

	zap.L().Info("创建分片",
		zap.String("task_id", t.ID),
		zap.Int("parts", 1),
		zap.Int64("size", t.TotalSize),
		zap.Int("concurrency", 1))
}

// downloadParts 并发下载分片
func (t *DownloadTask) downloadParts(ctx context.Context, file *os.File) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(t.partStates))
	sem := make(chan struct{}, t.Concurrency)

	for _, part := range t.partStates {
		if part.Completed {
			atomic.AddInt64(&t.Downloaded, part.Downloaded)
			continue
		}

		wg.Add(1)
		go func(p *PartState) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := t.downloadPart(ctx, file, p); err != nil {
				select {
				case errCh <- fmt.Errorf("分片 %d 下载失败: %w", p.Index, err):
				default:
				}
			}
		}(part)
	}

	// 等待完成
	wg.Wait()
	close(errCh)

	// 检查是否有错误
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errs[0]
	}

	return nil
}

// downloadPart 下载单个分片 (带重试)
func (t *DownloadTask) downloadPart(ctx context.Context, file *os.File, part *PartState) error {
	for attempt := 0; attempt <= t.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * time.Second
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			zap.L().Info("分片下载重试",
				zap.String("task_id", t.ID),
				zap.Int("part", part.Index),
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff))
			time.Sleep(backoff)
		}

		err := t.downloadPartOnce(ctx, file, part)
		if err == nil {
			return nil
		}

		// 检查上下文是否被取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 不可重试的错误
		if isNonRetryableError(err) {
			return err
		}
	}

	return fmt.Errorf("分片 %d 下载失败，已重试 %d 次", part.Index, t.MaxRetries)
}

// downloadPartOnce 单次下载分片
func (t *DownloadTask) downloadPartOnce(ctx context.Context, file *os.File, part *PartState) error {
	req, err := http.NewRequestWithContext(ctx, "GET", t.Remote, nil)
	if err != nil {
		return err
	}

	// 设置 Range 头
	if t.TotalSize > 0 && t.Concurrency > 1 {
		start := part.Start + part.Downloaded
		end := part.End
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	} else if part.Downloaded > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", part.Start+part.Downloaded))
	}

	client := &http.Client{
		Timeout: 5 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// 允许最多 10 次重定向
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 简单单线程下载，不使用 Range
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP 错误: %d", resp.StatusCode)
	}

	// 直接写入文件，从 0 开始
	buffer := make([]byte, 32*1024)
	written := int64(0)
	lastProgressTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if _, werr := file.WriteAt(buffer[:n], written); werr != nil {
				return fmt.Errorf("写入文件失败: %w", werr)
			}
			written += int64(n)

			t.mu.Lock()
			t.Downloaded = written
			t.mu.Unlock()

			// 每 0.5 秒推送一次进度
			if time.Since(lastProgressTime) >= 500*time.Millisecond {
				t.reportProgress()
				lastProgressTime = time.Now()
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("读取响应失败: %w", err)
		}
	}

	part.Completed = true
	part.Downloaded = written
	return nil
}

// verifyChecksum 校验文件哈希
func (t *DownloadTask) verifyChecksum() error {
	if t.Checksum == "" {
		return nil
	}

	file, err := os.Open(t.TargetPath)
	if err != nil {
		return err
	}
	defer file.Close()

	var hasher hash.Hash
	switch strings.ToLower(t.HashType) {
	case "sha256":
		hasher = sha256.New()
	default:
		hasher = sha256.New()
	}

	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}

	sum := hex.EncodeToString(hasher.Sum(nil))
	if sum != t.Checksum {
		return fmt.Errorf("校验和不匹配: 期望 %s, 实际 %s", t.Checksum, sum)
	}

	zap.L().Info("文件校验通过", zap.String("task_id", t.ID))
	return nil
}

// reportProgress 报告进度 (每 0.5 秒最多一次)
func (t *DownloadTask) reportProgress() {
	t.mu.RLock()
	cb := t.progressCb
	dl := t.Downloaded
	tot := t.TotalSize
	st := t.Status
	t.mu.RUnlock()

	if cb != nil {
		cb(t.ID, dl, tot, st)
	}
}

// setFailed 设置失败状态
func (t *DownloadTask) setFailed(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = StatusFailed
	t.Error = err.Error()
	now := time.Now()
	t.CompletedAt = &now
	t.RetryCount++
	t.saveState()
}

// setCompleted 设置完成状态
func (t *DownloadTask) setCompleted() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = StatusCompleted
	t.Error = ""
	now := time.Now()
	t.CompletedAt = &now
	if t.TotalSize > 0 {
		t.Downloaded = t.TotalSize
	}
	zap.L().Info("下载完成",
		zap.String("task_id", t.ID),
		zap.String("alias", t.Alias),
		zap.Int64("size", t.TotalSize))
}

// stateFilePath 状态文件路径
func (t *DownloadTask) stateFilePath() string {
	return t.TargetPath + ".dlstate"
}

// saveState 保存下载状态
func (t *DownloadTask) saveState() {
	data, err := json.Marshal(t.partStates)
	if err != nil {
		zap.L().Warn("保存下载状态失败", zap.Error(err))
		return
	}
	if err := os.WriteFile(t.stateFilePath(), data, 0644); err != nil {
		zap.L().Warn("写入状态文件失败", zap.Error(err))
	}
}

// loadState 加载下载状态
func (t *DownloadTask) loadState() error {
	data, err := os.ReadFile(t.stateFilePath())
	if err != nil {
		return err
	}
	var parts []*PartState
	if err := json.Unmarshal(data, &parts); err != nil {
		return err
	}

	t.partStates = parts
	var downloaded int64
	for _, p := range parts {
		if p.Completed {
			downloaded += p.End - p.Start + 1
		} else {
			downloaded += p.Downloaded
		}
	}
	t.Downloaded = downloaded
	return nil
}

// cleanState 清理状态文件
func (t *DownloadTask) cleanState() {
	os.Remove(t.stateFilePath())
}

// isNonRetryableError 判断是否不可重试的错误
func isNonRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// 客户端错误通常不需要重试
	if strings.Contains(errStr, "403") ||
		strings.Contains(errStr, "404") ||
		strings.Contains(errStr, "401") ||
		strings.Contains(errStr, "checksum") ||
		strings.Contains(errStr, "校验和") {
		return true
	}
	return false
}

// DownloadManager 下载管理器
type DownloadManager struct {
	mu      sync.RWMutex
	tasks   map[string]*DownloadTask
	workDir string
	client  *http.Client
}

// NewDownloadManager 创建下载管理器
func NewDownloadManager(workDir string) *DownloadManager {
	if workDir == "" {
		workDir = "/var/cache/tsukiyo/images"
	}
	os.MkdirAll(workDir, 0755)

	return &DownloadManager{
		tasks:   make(map[string]*DownloadTask),
		workDir: workDir,
		client: &http.Client{
			Timeout: 5 * time.Minute,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// 允许最多 10 次重定向
				if len(via) >= 10 {
					return fmt.Errorf("stopped after 10 redirects")
				}
				return nil
			},
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// DownloadImage 下载镜像 (支持多线程、断点续传、重试)
func (dm *DownloadManager) DownloadImage(ctx context.Context, alias, remote string, progressCb ...ProgressCallback) (*DownloadTask, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// 检查是否已在下载
	if task, exists := dm.tasks[alias]; exists {
		status := task.GetStatus()
		if status == StatusRunning || status == StatusPending {
			return task, fmt.Errorf("镜像 %s 已在下载中", alias)
		}
		if status == StatusCompleted {
			// 已下载完成，返回任务对象让调用方继续执行导入逻辑
			return task, nil
		}
	}

	// 根据 URL 推断文件扩展名，VM 镜像可能是 .img/.qcow2，容器镜像默认 .tar.gz
	ext := ".tar.gz"
	if remote != "" {
		base := filepath.Base(remote)
		if idx := strings.LastIndex(base, "."); idx != -1 && idx < len(base)-1 {
			urlExt := base[idx:]
			// 只允许已知镜像扩展名
			switch urlExt {
			case ".img", ".qcow2", ".qcow", ".raw", ".tar", ".tar.gz", ".tar.xz":
				ext = urlExt
			}
		}
	}
	if remote == "" {
		// 从 GitHub Release 下载镜像
		remote = fmt.Sprintf("https://github.com/adokiu/Tsukiyo-images/releases/download/images/%s_amd64.tar.gz", alias)
	}

	targetPath := filepath.Join(dm.workDir, alias+ext)
	task := NewDownloadTask(alias, remote, targetPath)
	task.Remote = remote
	if len(progressCb) > 0 && progressCb[0] != nil {
		task.progressCb = progressCb[0]
	}
	dm.tasks[alias] = task

	go func() {
		if err := task.Start(ctx); err != nil {
			zap.L().Error("镜像下载失败",
				zap.String("alias", alias),
				zap.Error(err))
		}
	}()

	return task, nil
}

// GetTask 获取任务
func (dm *DownloadManager) GetTask(alias string) *DownloadTask {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.tasks[alias]
}

// ListTasks 列出所有任务
func (dm *DownloadManager) ListTasks() []*DownloadTask {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	tasks := make([]*DownloadTask, 0, len(dm.tasks))
	for _, t := range dm.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// RemoveTask 移除任务
func (dm *DownloadManager) RemoveTask(alias string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	if task, exists := dm.tasks[alias]; exists {
		task.Cancel()
		delete(dm.tasks, alias)
	}
}
