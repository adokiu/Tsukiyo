package incus

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Client Incus API 客户端
type Client struct {
	httpClient  *http.Client
	baseURL     string
	socketPath  string
	avail       atomic.Bool  // Incus 是否可用
	consecFail  atomic.Int64 // 连续失败次数
	lastLogTime atomic.Int64 // 上次错误日志时间 (unix nano)
}

// NewClient 创建 Incus 客户端
func NewClient(socketPath string) (*Client, error) {
	if socketPath == "" {
		socketPath = "/var/lib/incus/unix.socket"
	}

	c := &Client{
		socketPath: socketPath,
		baseURL:    "http://unix/1.0",
	}
	c.avail.Store(true) // 初始假设可用，首次失败后自动切换

	transport := &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		},
	}

	c.httpClient = &http.Client{
		Transport: transport,
		Timeout:   90 * time.Second,
	}
	return c, nil
}

// IsAvailable 返回 Incus 是否可用
func (c *Client) IsAvailable() bool {
	return c.avail.Load()
}

// SocketPath 返回 socket 路径
func (c *Client) SocketPath() string {
	return c.socketPath
}

// markSuccess 标记请求成功，重置失败计数
func (c *Client) markSuccess() {
	wasDown := !c.avail.Load()
	c.consecFail.Store(0)
	c.avail.Store(true)
	if wasDown {
		zap.L().Info("Incus 连接已恢复", zap.String("socket", c.socketPath))
	}
}

// markFailure 标记请求失败，返回是否应该记录日志（抑制刷屏）
func (c *Client) markFailure() bool {
	failures := c.consecFail.Add(1)
	c.avail.Store(false)

	// 首次失败立即记录
	if failures == 1 {
		return true
	}

	// 后续失败每 30 秒最多记录一次
	now := time.Now().UnixNano()
	last := c.lastLogTime.Load()
	if now-last > int64(30*time.Second) {
		c.lastLogTime.Store(now)
		return true
	}
	return false
}

// doRequest 执行 HTTP 请求
func (c *Client) doRequest(method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	url := c.baseURL + path
	zap.L().Debug("Incus HTTP 请求", zap.String("method", method), zap.String("path", path), zap.String("url", url))
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if c.markFailure() {
			zap.L().Error("Incus 不可达", zap.String("socket", c.socketPath),
				zap.String("method", method), zap.String("path", path),
				zap.Int64("consecutive_failures", c.consecFail.Load()),
				zap.Error(err))
		}
		return nil, err
	}
	c.markSuccess()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		zap.L().Warn("Incus HTTP 响应非 2xx", zap.String("method", method), zap.String("path", path), zap.Int("status", resp.StatusCode))
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("incus HTTP %d: %s", resp.StatusCode, string(body))
	}
	return resp, nil
}

// parseResponse 解析响应。返回顶层 operation 字段和错误。out 接收 metadata 内容。
func parseResponse(resp *http.Response, out interface{}) (string, error) {
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	// 只在响应体较小时记录 debug，避免刷屏
	if len(data) < 512 {
		zap.L().Debug("Incus 响应体", zap.String("body", string(data)))
	}

	var base struct {
		Type       string          `json:"type"`
		Status     string          `json:"status"`
		StatusCode int             `json:"status_code"`
		Operation  string          `json:"operation"`
		Metadata   json.RawMessage `json:"metadata"`
		Error      string          `json:"error"`
	}
	if err := json.Unmarshal(data, &base); err != nil {
		return "", err
	}

	if base.Type == "error" {
		return "", fmt.Errorf("incus error: %s", base.Error)
	}
	if base.StatusCode >= 400 {
		return "", fmt.Errorf("incus error %d: %s", base.StatusCode, base.Error)
	}

	if out != nil && base.Metadata != nil {
		return base.Operation, json.Unmarshal(base.Metadata, out)
	}
	return base.Operation, nil
}

// parseOperationWaitResponse 解析 /operations/{id}/wait 响应，兼容 metadata 与顶层 status
func parseOperationWaitResponse(resp *http.Response) (status, errMsg string, err error) {
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	var base struct {
		Type       string          `json:"type"`
		Status     string          `json:"status"`
		StatusCode int             `json:"status_code"`
		Error      string          `json:"error"`
		Metadata   json.RawMessage `json:"metadata"`
	}
	if err := json.Unmarshal(data, &base); err != nil {
		return "", "", err
	}
	if base.Type == "error" || base.StatusCode >= 400 {
		msg := base.Error
		if msg == "" {
			msg = string(data)
		}
		return "", "", fmt.Errorf("incus error: %s", msg)
	}
	var meta struct {
		Status string `json:"status"`
		Err    string `json:"err"`
	}
	if base.Metadata != nil {
		_ = json.Unmarshal(base.Metadata, &meta)
	}
	status = meta.Status
	if status == "" {
		status = base.Status
	}
	return status, meta.Err, nil
}

// waitOperation 等待异步操作完成
func (c *Client) waitOperation(opID string) error {
	return c.waitOperationTimeout(opID, 5*time.Minute)
}

// waitOperationTimeout 在限定时间内等待 operation 完成
func (c *Client) waitOperationTimeout(opID string, maxWait time.Duration) error {
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		waitSec := 10
		if remaining < time.Duration(waitSec)*time.Second {
			waitSec = int(remaining.Seconds())
			if waitSec < 1 {
				waitSec = 1
			}
		}
		resp, err := c.doRequest("GET", fmt.Sprintf("/operations/%s/wait?timeout=%d", opID, waitSec), nil)
		if err != nil {
			// 网络错误，短暂等待后重试
			time.Sleep(2 * time.Second)
			continue
		}
		status, errMsg, err := parseOperationWaitResponse(resp)
		if err != nil {
			return err
		}
		zap.L().Debug("operation 等待中", zap.String("op_id", opID), zap.String("status", status))
		switch status {
		case "Success":
			return nil
		case "Failure":
			if errMsg == "" {
				errMsg = "unknown error"
			}
			return fmt.Errorf("operation failed: %s", errMsg)
		case "Cancelled":
			return fmt.Errorf("operation cancelled")
		}
	}
	return fmt.Errorf("operation timeout after %s", maxWait)
}

// GetServerInfo 获取服务器信息
func (c *Client) GetServerInfo() (*ServerInfo, error) {
	resp, err := c.doRequest("GET", "", nil)
	if err != nil {
		return nil, err
	}
	var info ServerInfo
	if _, err := parseResponse(resp, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// CreateInstance 创建实例
func (c *Client) CreateInstance(req CreateInstanceRequest) (string, error) {
	zap.L().Info("CreateInstance 开始", zap.String("name", req.Name), zap.String("template_id", req.TemplateID), zap.String("type", req.Type))
	devices := map[string]map[string]string{
		"root": {
			"path": "/",
			"pool": req.StoragePool,
			"type": "disk",
			"size": fmt.Sprintf("%dGB", req.DiskGB),
		},
	}
	if req.StoragePool == "" {
		devices["root"]["pool"] = "default"
	}

	for _, dd := range req.DataDisks {
		pool := dd.StoragePool
		if pool == "" {
			pool = req.StoragePool
		}
		if pool == "" {
			pool = "default"
		}
		devices[dd.Name] = map[string]string{
			"type": "disk",
			"pool": pool,
			"path": dd.MountPoint,
			"size": fmt.Sprintf("%dGB", dd.SizeGB),
		}
	}

	nicDevice := map[string]string{
		"name":    "eth0",
		"nictype": "bridged",
		"type":    "nic",
	}

	// Bridge 网络配置：使用 nictype=bridged + parent 方式（和 Old 项目一致，确保 limits 生效）
	if req.BridgeName != "" {
		nicDevice["nictype"] = "bridged"
		nicDevice["parent"] = req.BridgeName
		// 通过 Incus 设备属性分配静态 IP
		if req.InternalIPv4 != "" {
			nicDevice["ipv4.address"] = req.InternalIPv4
		}
	} else if req.NetworkBridge != "" {
		nicDevice["parent"] = req.NetworkBridge
	} else {
		nicDevice["parent"] = "incusbr0"
	}

	// 安全过滤
	if req.IPv4Filter {
		nicDevice["security.ipv4_filtering"] = "true"
	}
	if req.MACFilter {
		nicDevice["security.mac_filtering"] = "true"
	}
	// 网络限速（下行=ingress，上行=egress）
	if req.NetworkDown > 0 {
		nicDevice["limits.ingress"] = fmt.Sprintf("%dMbit", req.NetworkDown)
	}
	if req.NetworkUp > 0 {
		nicDevice["limits.egress"] = fmt.Sprintf("%dMbit", req.NetworkUp)
	}
	devices["eth0"] = nicDevice

	configMap := map[string]string{
		"limits.cpu":          strconv.Itoa(int(req.VCPU)),
		"limits.memory":       fmt.Sprintf("%dMB", req.MemoryMB),
		"user.os_information": req.TemplateID,
		"boot.autostart":      "1",
	}
	if req.UserData != "" {
		configMap["user.user-data"] = req.UserData
	}
	if req.NetworkConfig != "" {
		configMap["user.network-config"] = req.NetworkConfig
	}

	body := map[string]interface{}{
		"name":    req.Name,
		"source":  map[string]string{"type": "image", "alias": req.TemplateID},
		"config":  configMap,
		"devices": devices,
	}
	if req.BridgeName != "" {
		body["profiles"] = []string{}
		zap.L().Info("Bridge 模式排除默认 profile", zap.String("name", req.Name))
	}
	if req.Type == "" {
		body["type"] = "container"
	}

	bodyJSON, _ := json.Marshal(body)
	zap.L().Info("CreateInstance 发送 POST /instances", zap.String("name", req.Name), zap.String("body", string(bodyJSON)))
	resp, err := c.doRequest("POST", "/instances", body)
	if err != nil {
		zap.L().Error("CreateInstance POST 请求失败", zap.String("name", req.Name), zap.Error(err))
		return "", err
	}
	zap.L().Info("CreateInstance POST 请求成功", zap.String("name", req.Name))

	opStr, err := parseResponse(resp, nil)
	if err != nil {
		if c.InstanceExists(req.Name) {
			zap.L().Warn("CreateInstance 实例已存在，跳过创建", zap.String("name", req.Name), zap.Error(err))
			return req.Name, nil
		}
		zap.L().Error("CreateInstance 解析响应失败", zap.String("name", req.Name), zap.Error(err))
		return "", err
	}
	zap.L().Info("CreateInstance 获取 operation", zap.String("name", req.Name), zap.String("operation", opStr))

	if opStr == "" {
		zap.L().Info("CreateInstance 同步完成，无需等待 operation", zap.String("name", req.Name))
		return req.Name, nil
	}

	opID := filepath.Base(opStr)
	zap.L().Info("CreateInstance 等待 operation（Incus 创建容器）", zap.String("name", req.Name), zap.String("op_id", opID))
	if err := c.waitOperationTimeout(opID, 120*time.Second); err != nil {
		if c.InstanceExists(req.Name) {
			zap.L().Warn("CreateInstance operation 等待异常但实例已存在，等待存储就绪后继续", zap.String("name", req.Name), zap.Error(err))
			// 等待存储目录就绪
			if err := c.waitForInstanceStorage(req.Name, req.StoragePool, 30*time.Second); err != nil {
				zap.L().Warn("CreateInstance 等待存储就绪失败", zap.String("name", req.Name), zap.Error(err))
			}
		} else {
			zap.L().Error("CreateInstance waitOperation 失败", zap.String("name", req.Name), zap.String("op_id", opID), zap.Error(err))
			return "", err
		}
	}
	zap.L().Info("CreateInstance operation 完成", zap.String("name", req.Name))

	return req.Name, nil
}

// StartInstance 启动实例
func (c *Client) StartInstance(name string) error {
	return c.StartInstanceWithPool(name, "")
}

// StartInstanceWithPool 启动实例，指定存储池用于等待存储就绪
func (c *Client) StartInstanceWithPool(name, pool string) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			zap.L().Info("StartInstance 重试", zap.String("name", name), zap.Int("attempt", attempt+1))
			if err := c.waitForInstanceStorage(name, pool, 30*time.Second); err != nil {
				zap.L().Warn("StartInstance 等待存储就绪失败", zap.String("name", name), zap.Error(err))
			}
		}
		resp, err := c.doRequest("PUT", fmt.Sprintf("/instances/%s/state", name), map[string]string{"action": "start"})
		if err != nil {
			lastErr = err
			time.Sleep(3 * time.Second)
			continue
		}
		opStr, err := parseResponse(resp, nil)
		if err != nil {
			if isInstanceAlreadyRunning(err) {
				return nil
			}
			lastErr = err
			time.Sleep(3 * time.Second)
			continue
		}
		if opStr != "" {
			if err := c.waitOperation(filepath.Base(opStr)); err != nil {
				if isInstanceAlreadyRunning(err) {
					return nil
				}
				lastErr = err
				time.Sleep(3 * time.Second)
				continue
			}
		}
		return nil
	}
	return lastErr
}

func isInstanceAlreadyRunning(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already running") || strings.Contains(msg, "is already running")
}

// StopInstance 停止实例
func (c *Client) StopInstance(name string, force bool) error {
	body := map[string]interface{}{"action": "stop"}
	if force {
		body["force"] = true
	}
	resp, err := c.doRequest("PUT", fmt.Sprintf("/instances/%s/state", name), body)
	if err != nil {
		return err
	}
	opStr, err := parseResponse(resp, nil)
	if err != nil {
		return err
	}
	if opStr != "" {
		return c.waitOperation(filepath.Base(opStr))
	}
	return nil
}

// RestartInstance 重启实例
func (c *Client) RestartInstance(name string) error {
	resp, err := c.doRequest("PUT", fmt.Sprintf("/instances/%s/state", name), map[string]string{"action": "restart"})
	if err != nil {
		return err
	}
	opStr, err := parseResponse(resp, nil)
	if err != nil {
		return err
	}
	if opStr != "" {
		return c.waitOperation(filepath.Base(opStr))
	}
	return nil
}

// DeleteInstance 删除实例（先强制停止再删除）
func (c *Client) DeleteInstance(name string) error {
	info, err := c.GetInstance(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "Not found") {
			zap.L().Info("实例已不存在，删除成功", zap.String("name", name))
			return nil
		}
		return fmt.Errorf("获取实例状态失败: %w", err)
	}

	pool := instanceStoragePool(info)

	if info.Status == "Running" || info.Status == "Frozen" {
		zap.L().Info("实例正在运行，先强制停止", zap.String("name", name), zap.String("status", info.Status))
		stopBody := map[string]interface{}{
			"action":  "stop",
			"timeout": 0,
			"force":   true,
		}
		resp, err := c.doRequest("PUT", fmt.Sprintf("/instances/%s/state", name), stopBody)
		if err != nil {
			return fmt.Errorf("停止实例请求失败: %w", err)
		}
		opStr, err := parseResponse(resp, nil)
		if err != nil {
			return fmt.Errorf("停止实例响应解析失败: %w", err)
		}
		if opStr != "" {
			if err := c.waitOperation(filepath.Base(opStr)); err != nil {
				zap.L().Warn("停止实例等待失败，继续尝试删除", zap.Error(err))
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	if err := c.deleteInstanceAPI(name, true); err == nil {
		return nil
	} else if !isStorageDeleteError(err) {
		return err
	}

	zap.L().Warn("Incus 删除存储卷失败，尝试强制清理后重试",
		zap.String("name", name),
		zap.String("pool", pool),
		zap.Error(err))

	if cleanupErr := c.forceCleanupContainerStorage(pool, name); cleanupErr != nil {
		zap.L().Warn("强制清理容器存储目录失败", zap.Error(cleanupErr))
	}
	_ = c.deleteStorageVolumeForce(pool, "container", name)

	if err := c.deleteInstanceAPI(name, true); err == nil {
		return nil
	} else if !isStorageDeleteError(err) {
		return err
	}

	if err := c.deleteInstanceCLI(name); err == nil {
		return nil
	} else if !c.InstanceExists(name) {
		return nil
	}
	return err
}

func instanceStoragePool(info *InstanceInfo) string {
	if info == nil || info.Devices == nil {
		return "default"
	}
	if root, ok := info.Devices["root"].(map[string]interface{}); ok {
		if pool, ok := root["pool"].(string); ok && pool != "" {
			return pool
		}
	}
	return "default"
}

func isStorageDeleteError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "deleting storage volume") ||
		strings.Contains(msg, "btrfs subvolume") ||
		strings.Contains(msg, "not a btrfs subvolume")
}

func (c *Client) storagePoolsDir() string {
	if info, err := c.GetServerInfo(); err == nil {
		storage := info.Environment.Storage
		if strings.HasPrefix(storage, "/") {
			if strings.HasSuffix(storage, "storage-pools") {
				return storage
			}
			return filepath.Join(storage, "storage-pools")
		}
	}
	return "/var/lib/incus/storage-pools"
}

// waitForInstanceStorage 等待实例存储目录就绪
func (c *Client) waitForInstanceStorage(name, pool string, maxWait time.Duration) error {
	if pool == "" {
		pool = "default"
	}
	base := filepath.Join(c.storagePoolsDir(), pool, "containers", name)
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(base); err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("等待实例存储目录就绪超时: %s", base)
}

func (c *Client) forceCleanupContainerStorage(pool, name string) error {
	base := filepath.Join(c.storagePoolsDir(), pool)
	paths := []string{
		filepath.Join(base, "containers", name),
		filepath.Join(base, "containers-snapshots", name),
		filepath.Join(base, "custom", name),
	}
	var lastErr error
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		zap.L().Warn("强制删除容器存储路径", zap.String("path", p))
		if err := os.RemoveAll(p); err != nil {
			lastErr = err
			zap.L().Warn("删除存储路径失败", zap.String("path", p), zap.Error(err))
		}
	}
	return lastErr
}

func (c *Client) deleteInstanceAPI(name string, force bool) error {
	path := fmt.Sprintf("/instances/%s", name)
	if force {
		path += "?force=1"
	}
	resp, err := c.doRequest("DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("删除实例请求失败: %w", err)
	}
	opStr, err := parseResponse(resp, nil)
	if err != nil {
		return fmt.Errorf("删除实例响应解析失败: %w", err)
	}
	if opStr != "" {
		return c.waitOperation(filepath.Base(opStr))
	}
	return nil
}

func (c *Client) deleteStorageVolumeForce(pool, volType, name string) error {
	resp, err := c.doRequest("DELETE", fmt.Sprintf("/storage-pools/%s/volumes/%s/%s?force=1", pool, volType, name), nil)
	if err != nil {
		return err
	}
	opStr, err := parseResponse(resp, nil)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return err
	}
	if opStr != "" {
		return c.waitOperation(filepath.Base(opStr))
	}
	return nil
}

func (c *Client) deleteInstanceCLI(name string) error {
	cmd := exec.Command("incus", "delete", name, "--force")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "not found") {
			return nil
		}
		return fmt.Errorf("incus delete --force 失败: %w, output: %s", err, string(output))
	}
	return nil
}

// InstanceExists 检查实例是否存在
func (c *Client) InstanceExists(name string) bool {
	resp, err := c.doRequest("GET", fmt.Sprintf("/instances/%s", name), nil)
	if err != nil {
		return false
	}
	var info InstanceInfo
	if _, err := parseResponse(resp, &info); err != nil {
		return false
	}
	return info.Name == name
}

// GetInstance 获取实例详情
func (c *Client) GetInstance(name string) (*InstanceInfo, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/instances/%s", name), nil)
	if err != nil {
		return nil, err
	}
	var info InstanceInfo
	if _, err := parseResponse(resp, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// ListInstances 列出所有实例
func (c *Client) ListInstances() ([]InstanceInfo, error) {
	resp, err := c.doRequest("GET", "/instances?recursion=1", nil)
	if err != nil {
		return nil, err
	}
	var list []InstanceInfo
	if _, err := parseResponse(resp, &list); err != nil {
		return nil, err
	}
	return list, nil
}

// UpdateInstanceConfig 更新实例配置
func (c *Client) UpdateInstanceConfig(name string, config map[string]string) error {
	body := map[string]interface{}{"config": config}
	resp, err := c.doRequest("PATCH", fmt.Sprintf("/instances/%s", name), body)
	if err != nil {
		return err
	}
	_, err = parseResponse(resp, nil)
	return err
}

// SetInstanceConfig 设置实例完整配置 (用于 resize)
func (c *Client) SetInstanceConfig(name string, config map[string]string, devices map[string]map[string]string) error {
	body := map[string]interface{}{}
	if config != nil {
		body["config"] = config
	}
	if devices != nil {
		body["devices"] = devices
	}
	resp, err := c.doRequest("PUT", fmt.Sprintf("/instances/%s", name), body)
	if err != nil {
		return err
	}
	_, err = parseResponse(resp, nil)
	return err
}

// ReinstallInstance 重装实例 (删除后重建)
func (c *Client) ReinstallInstance(name string, templateID string) error {
	if err := c.DeleteInstance(name); err != nil {
		zap.L().Warn("删除旧实例失败，尝试继续重装", zap.Error(err))
	}
	req := CreateInstanceRequest{
		Name:       name,
		TemplateID: templateID,
		Type:       "container",
	}
	_, err := c.CreateInstance(req)
	return err
}

// DeviceExists 检查实例是否已有指定设备
func (c *Client) DeviceExists(instanceName, deviceName string) (bool, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/instances/%s", instanceName), nil)
	if err != nil {
		return false, err
	}
	var result struct {
		Metadata struct {
			Devices map[string]interface{} `json:"devices"`
		} `json:"metadata"`
	}
	if _, err := parseResponse(resp, &result); err != nil {
		return false, err
	}
	_, exists := result.Metadata.Devices[deviceName]
	return exists, nil
}

// AddProxyDevice 为实例添加 Incus 原生 proxy 设备端口映射（使用 incus CLI，和 Old 项目一致）
func (c *Client) AddProxyDevice(instanceName, deviceName, listenAddr, connectAddr string) error {
	// 先尝试通过 HTTP API 添加（带 bind=host，确保在宿主机监听）
	body := map[string]interface{}{
		"devices": map[string]map[string]string{
			deviceName: {
				"type":    "proxy",
				"listen":  listenAddr,
				"connect": connectAddr,
				"bind":    "host",
				"nat":     "true",
			},
		},
	}
	resp, err := c.doRequest("PATCH", fmt.Sprintf("/instances/%s", instanceName), body)
	if err != nil {
		// HTTP API 失败时回退到 incus CLI
		return c.addProxyDeviceCLI(instanceName, deviceName, listenAddr, connectAddr)
	}
	_, err = parseResponse(resp, nil)
	if err != nil {
		return c.addProxyDeviceCLI(instanceName, deviceName, listenAddr, connectAddr)
	}
	return nil
}

// addProxyDeviceCLI 使用 incus CLI 添加 proxy 设备
func (c *Client) addProxyDeviceCLI(instanceName, deviceName, listenAddr, connectAddr string) error {
	cmd := exec.Command("incus", "config", "device", "add", instanceName, deviceName, "proxy",
		"bind=host", "listen="+listenAddr, "connect="+connectAddr, "nat=true")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// 设备已存在，先删再加
		if strings.Contains(string(output), "already exists") {
			_ = exec.Command("incus", "config", "device", "remove", instanceName, deviceName).Run()
			cmd = exec.Command("incus", "config", "device", "add", instanceName, deviceName, "proxy",
				"bind=host", "listen="+listenAddr, "connect="+connectAddr, "nat=true")
			output, err = cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("incus CLI 添加 proxy 设备 %s 失败(重试): %w, output: %s", deviceName, err, string(output))
			}
			return nil
		}
		return fmt.Errorf("incus CLI 添加 proxy 设备 %s 失败: %w, output: %s", deviceName, err, string(output))
	}
	return nil
}

// RemoveProxyDevice 从实例移除 proxy 设备（使用 incus CLI）
func (c *Client) RemoveProxyDevice(instanceName, deviceName string) error {
	cmd := exec.Command("incus", "config", "device", "remove", instanceName, deviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// 设备可能不存在，忽略错误
		if strings.Contains(string(output), "not found") || strings.Contains(string(output), "doesn't exist") {
			return nil
		}
		return fmt.Errorf("incus CLI 移除 proxy 设备 %s 失败: %w, output: %s", deviceName, err, string(output))
	}
	return nil
}

// CreateSnapshot 创建快照
func (c *Client) CreateSnapshot(name, snapshotName string, stateful bool) error {
	body := map[string]interface{}{
		"name":     snapshotName,
		"stateful": stateful,
	}
	resp, err := c.doRequest("POST", fmt.Sprintf("/instances/%s/snapshots", name), body)
	if err != nil {
		return err
	}
	opStr, err := parseResponse(resp, nil)
	if err != nil {
		return err
	}
	if opStr != "" {
		return c.waitOperation(filepath.Base(opStr))
	}
	return nil
}

// RestoreSnapshot 恢复快照
func (c *Client) RestoreSnapshot(name, snapshotName string) error {
	body := map[string]interface{}{
		"restore": snapshotName,
	}
	resp, err := c.doRequest("PUT", fmt.Sprintf("/instances/%s", name), body)
	if err != nil {
		return err
	}
	opStr, err := parseResponse(resp, nil)
	if err != nil {
		return err
	}
	if opStr != "" {
		return c.waitOperation(filepath.Base(opStr))
	}
	return nil
}

// DeleteSnapshot 删除快照
func (c *Client) DeleteSnapshot(name, snapshotName string) error {
	resp, err := c.doRequest("DELETE", fmt.Sprintf("/instances/%s/snapshots/%s", name, snapshotName), nil)
	if err != nil {
		return err
	}
	opStr, err := parseResponse(resp, nil)
	if err != nil {
		return err
	}
	if opStr != "" {
		return c.waitOperation(filepath.Base(opStr))
	}
	return nil
}

// GetInstanceMetrics 获取实例监控指标
func (c *Client) GetInstanceMetrics(name string) (*InstanceMetrics, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/instances/%s/state", name), nil)
	if err != nil {
		return nil, err
	}
	var state struct {
		Status     string `json:"status"`
		StatusCode int    `json:"status_code"`
		Disk       map[string]struct {
			Usage int64 `json:"usage"`
		} `json:"disk"`
		Memory struct {
			Usage int64 `json:"usage"`
			Total int64 `json:"total"`
		} `json:"memory"`
		CPU struct {
			Usage int64 `json:"usage"`
		} `json:"cpu"`
		Network map[string]struct {
			Counters struct {
				BytesReceived   int64 `json:"bytes_received"`
				BytesSent       int64 `json:"bytes_sent"`
				PacketsReceived int64 `json:"packets_received"`
				PacketsSent     int64 `json:"packets_sent"`
			} `json:"counters"`
		} `json:"network"`
		Processes int `json:"processes"`
	}
	if _, err := parseResponse(resp, &state); err != nil {
		return nil, err
	}

	metrics := &InstanceMetrics{
		Status:       state.Status,
		StatusCode:   state.StatusCode,
		MemUsage:     state.Memory.Usage,
		MemTotal:     state.Memory.Total,
		CPUUsage:     state.CPU.Usage,
		Processes:    state.Processes,
		DiskUsage:    make(map[string]int64),
		NetworkStats: make(map[string]NetworkStat),
	}

	for k, v := range state.Disk {
		metrics.DiskUsage[k] = v.Usage
	}
	for k, v := range state.Network {
		metrics.NetworkStats[k] = NetworkStat{
			BytesReceived:   v.Counters.BytesReceived,
			BytesSent:       v.Counters.BytesSent,
			PacketsReceived: v.Counters.PacketsReceived,
			PacketsSent:     v.Counters.PacketsSent,
		}
	}

	return metrics, nil
}

// GetInstanceNetworkInfo 获取实例网络信息，返回 eth0 等接口的 IPv4 地址
func (c *Client) GetInstanceNetworkInfo(name string) ([]string, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/instances/%s/state", name), nil)
	if err != nil {
		return nil, err
	}
	var state struct {
		Network map[string]struct {
			Addresses []struct {
				Family  string `json:"family"`
				Address string `json:"address"`
				Scope   string `json:"scope"`
			} `json:"addresses"`
		} `json:"network"`
	}
	if _, err := parseResponse(resp, &state); err != nil {
		return nil, err
	}

	var ipv4s []string
	for iface, netInfo := range state.Network {
		if iface == "lo" {
			continue
		}
		for _, addr := range netInfo.Addresses {
			if addr.Family == "inet" && addr.Scope == "global" {
				ipv4s = append(ipv4s, addr.Address)
			}
		}
	}
	return ipv4s, nil
}

// ExecCommand 在实例内执行命令
func (c *Client) ExecCommand(instanceName string, cmd []string, env map[string]string, user string, group uint32) (*ExecResult, error) {
	body := map[string]interface{}{
		"command":            cmd,
		"wait-for-websocket": false,
		"record-output":      true,
		"interactive":        false,
	}
	if env != nil {
		var envList []string
		for k, v := range env {
			envList = append(envList, fmt.Sprintf("%s=%s", k, v))
		}
		body["environment"] = envList
	}
	if user != "" {
		body["user"] = user
	}
	if group != 0 {
		body["group"] = group
	}

	resp, err := c.doRequest("POST", fmt.Sprintf("/instances/%s/exec", instanceName), body)
	if err != nil {
		return nil, err
	}
	opStr, err := parseResponse(resp, nil)
	if err != nil {
		return nil, err
	}

	// 等待执行完成
	if opStr != "" {
		if err := c.waitOperation(filepath.Base(opStr)); err != nil {
			return nil, err
		}
	}

	// 获取输出
	return &ExecResult{ExitCode: 0}, nil
}

// SetInstancePassword 设置实例密码 (通过 cloud-init 或 exec)
func (c *Client) SetInstancePassword(name string, password string) error {
	cmd := fmt.Sprintf("echo 'root:%s' | chpasswd", password)
	_, err := c.ExecCommand(name, []string{"sh", "-c", cmd}, nil, "", 0)
	if err != nil {
		cli := exec.Command("incus", "exec", name, "--", "sh", "-c", cmd)
		if output, cliErr := cli.CombinedOutput(); cliErr != nil {
			return fmt.Errorf("设置密码失败: %w, cli output: %s", err, string(output))
		}
	}
	return nil
}

// ImageAliasExists 检查镜像别名是否存在
func (c *Client) ImageAliasExists(alias string) bool {
	resp, err := c.doRequest("GET", fmt.Sprintf("/images/aliases/%s", alias), nil)
	if err != nil {
		return false
	}
	var result struct {
		Name string `json:"name"`
	}
	if _, err := parseResponse(resp, &result); err != nil {
		return false
	}
	return result.Name == alias
}

// RemoteImage 远程镜像信息
// ImageKey 格式: alias|type|arch，作为全局唯一标识
// 例如: debian/forky/cloud|container|x86_64
type RemoteImage struct {
	ImageKey     string `json:"image_key"`
	Alias        string `json:"alias"`
	Architecture string `json:"architecture"`
	Description  string `json:"description"`
	OS           string `json:"os"`
	Release      string `json:"release"`
	Type         string `json:"type"` // container / virtual-machine
}

// normalizeArch 将 Incus 架构名统一为 master 使用的格式
func normalizeArch(arch string) string {
	switch arch {
	case "x86_64":
		return "amd64"
	case "aarch64":
		return "arm64"
	default:
		return arch
	}
}

// BuildImageKey 构建镜像复合键
func BuildImageKey(alias, imageType, arch string) string {
	return alias + "|" + imageType + "|" + normalizeArch(arch)
}

// ParseImageKey 解析镜像复合键，返回 alias, type, arch
func ParseImageKey(key string) (alias, imageType, arch string) {
	parts := strings.SplitN(key, "|", 3)
	if len(parts) == 3 {
		return parts[0], parts[1], parts[2]
	}
	// 兼容旧格式：直接返回原始值
	return key, "container", "x86_64"
}

// ListRemoteImages 获取远程镜像列表（所有架构的 cloud 镜像）
func (c *Client) ListRemoteImages(remote string) ([]RemoteImage, error) {
	cmd := exec.Command("incus", "image", "list", remote, "--format", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		zap.L().Error("incus image list 失败", zap.String("output", string(output)), zap.Error(err))
		return nil, fmt.Errorf("incus image list 失败: %w, output: %s", err, string(output))
	}

	var rawImages []struct {
		Type         string `json:"type"`
		Architecture string `json:"architecture"`
		Properties   struct {
			Description string `json:"description"`
			OS          string `json:"os"`
			Release     string `json:"release"`
			Variant     string `json:"variant"`
		} `json:"properties"`
		Aliases []struct {
			Name string `json:"name"`
		} `json:"aliases"`
	}

	if err := json.Unmarshal(output, &rawImages); err != nil {
		zap.L().Error("解析镜像列表失败", zap.Error(err))
		return nil, fmt.Errorf("解析镜像列表失败: %w", err)
	}

	zap.L().Info("解析后的镜像总数", zap.Int("count", len(rawImages)))

	result := make([]RemoteImage, 0, len(rawImages))
	for _, img := range rawImages {
		if img.Properties.Variant != "cloud" {
			continue
		}
		if len(img.Aliases) == 0 {
			continue
		}

		alias := img.Aliases[0].Name
		imageKey := BuildImageKey(alias, img.Type, img.Architecture)

		result = append(result, RemoteImage{
			ImageKey:     imageKey,
			Alias:        alias,
			Architecture: img.Architecture,
			Description:  img.Properties.Description,
			OS:           img.Properties.OS,
			Release:      img.Properties.Release,
			Type:         img.Type,
		})
	}

	zap.L().Info("过滤后的 cloud 镜像数量", zap.Int("count", len(result)))
	return result, nil
}

// ImportImageFromFile 从 qcow2 文件导入镜像到 Incus
// Incus 的 image import 需要两个参数：metadata tarball 和 rootfs（qcow2）
func (c *Client) ImportImageFromFile(alias string, filePath string) error {
	// 1. 校验 qcow2 文件存在且可读
	fi, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("qcow2 文件不存在: %s", filePath)
		}
		return fmt.Errorf("访问 qcow2 文件失败: %w", err)
	}
	if fi.Size() == 0 {
		return fmt.Errorf("qcow2 文件大小为 0: %s", filePath)
	}
	zap.L().Info("准备导入 VM 镜像", zap.String("alias", alias), zap.String("path", filePath), zap.Int64("size", fi.Size()))

	// 2. 创建临时目录存放 metadata
	metaDir, err := os.MkdirTemp("", "tsukiyo-incus-meta-")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(metaDir)

	// 3. 写入 metadata.yaml（Incus 导入必需）
	metadataYaml := fmt.Sprintf(`architecture: x86_64
creation_date: %d
properties:
  architecture: x86_64
  description: %s VM image
`, time.Now().Unix(), alias)
	metaFile := filepath.Join(metaDir, "metadata.yaml")
	if err := os.WriteFile(metaFile, []byte(metadataYaml), 0644); err != nil {
		return fmt.Errorf("写入 metadata.yaml 失败: %w", err)
	}

	// 4. 将 metadata.yaml 打包为 tar.gz
	metaTar := filepath.Join(metaDir, "metadata.tar.gz")
	cmd := exec.Command("tar", "czf", metaTar, "-C", metaDir, "metadata.yaml")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("打包 metadata tarball 失败: %w, output: %s", err, string(output))
	}
	if _, err := os.Stat(metaTar); err != nil {
		return fmt.Errorf("metadata tarball 未生成: %w", err)
	}

	// 5. 调用 incus image import <metadata> <rootfs> --alias <alias>
	importCmd := exec.Command("incus", "image", "import", metaTar, filePath, "--alias", alias)
	importOutput, importErr := importCmd.CombinedOutput()
	if importErr != nil {
		return fmt.Errorf("incus image import 失败: %w, output: %s", importErr, string(importOutput))
	}

	zap.L().Info("VM 镜像导入 Incus 成功", zap.String("alias", alias), zap.String("output", string(importOutput)))
	return nil
}

// SyncRemote 确保 Incus remote 已注册，如果不存在或 URL 不一致则添加/更新
// name: remote 名称（如 spiritlhl、tsukiyo-mirror）
// serverURL: remote 服务器地址（如 https://incusimages.spiritlhl.net）
func (c *Client) SyncRemote(name, serverURL string) error {
	if name == "" || serverURL == "" {
		return nil
	}
	// images 是 Incus 内置 remote，无需注册
	if name == "images" {
		return nil
	}

	// 查询当前 remote 列表
	cmd := exec.Command("incus", "remote", "list", "--format", "csv")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("查询 remote 列表失败: %w", err)
	}

	zap.L().Info("remote list 原始输出", zap.String("output", string(output)))

	// CSV 格式: name,type,protocol,url,default (Incus 实际输出顺序)
	// 字段可能带引号，用 csv.Reader 解析
	rdr := csv.NewReader(strings.NewReader(string(output)))
	records, err := rdr.ReadAll()
	if err != nil {
		zap.L().Warn("解析 remote list CSV 失败，回退到简单分割", zap.Error(err))
		// 回退到简单分割
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			fields := strings.Split(line, ",")
			if len(fields) >= 4 && strings.TrimSpace(fields[0]) == name {
				// 尝试两种列顺序：name,type,url,default,protocol 或 name,type,protocol,url,default
				existingURL := ""
				for _, f := range fields[2:] {
					f = strings.Trim(strings.TrimSpace(f), "\"")
					if strings.HasPrefix(f, "http://") || strings.HasPrefix(f, "https://") {
						existingURL = f
						break
					}
				}
				if existingURL == serverURL {
					zap.L().Info("remote 已存在且 URL 一致，跳过", zap.String("name", name), zap.String("url", serverURL))
					return nil
				}
				zap.L().Info("remote URL 不一致，移除后重新添加", zap.String("name", name), zap.String("existing", existingURL), zap.String("expected", serverURL))
				if out, err := exec.Command("incus", "remote", "remove", name).CombinedOutput(); err != nil {
					return fmt.Errorf("移除 remote %s 失败: %w, output: %s", name, err, string(out))
				}
				break
			}
		}
	} else {
		for _, record := range records {
			zap.L().Info("remote list 记录", zap.Strings("fields", record))
			if len(record) >= 4 && strings.TrimSpace(record[0]) == name {
				// 在所有字段中查找 URL（以 http 开头）
				existingURL := ""
				for _, f := range record {
					f = strings.TrimSpace(f)
					if strings.HasPrefix(f, "http://") || strings.HasPrefix(f, "https://") {
						existingURL = f
						break
					}
				}
				if existingURL == serverURL {
					zap.L().Info("remote 已存在且 URL 一致，跳过", zap.String("name", name), zap.String("url", serverURL))
					return nil
				}
				zap.L().Info("remote URL 不一致，移除后重新添加", zap.String("name", name), zap.String("existing", existingURL), zap.String("expected", serverURL))
				if out, err := exec.Command("incus", "remote", "remove", name).CombinedOutput(); err != nil {
					return fmt.Errorf("移除 remote %s 失败: %w, output: %s", name, err, string(out))
				}
				break
			}
		}
	}

	// 添加 remote
	cmd = exec.Command("incus", "remote", "add", name, serverURL, "--protocol", "simplestreams", "--public")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("添加 remote %s 失败: %w, output: %s", name, err, string(output))
	}

	zap.L().Info("remote 注册成功", zap.String("name", name), zap.String("url", serverURL))
	return nil
}

// CopyImage 复制镜像到本地 (从 remote 下载)
func (c *Client) CopyImage(alias string, sourceRemote string) error {
	body := map[string]interface{}{
		"source": map[string]string{
			"type":     "image",
			"mode":     "pull",
			"server":   sourceRemote,
			"protocol": "simplestreams",
			"alias":    alias,
		},
	}
	if sourceRemote == "" {
		body["source"] = map[string]string{
			"type":  "image",
			"alias": alias,
		}
	}

	resp, err := c.doRequest("POST", "/images", body)
	if err != nil {
		return err
	}
	opStr, err := parseResponse(resp, nil)
	if err != nil {
		return err
	}
	if opStr != "" {
		return c.waitOperation(filepath.Base(opStr))
	}
	return nil
}

// CreateStoragePool 创建存储池
func (c *Client) CreateStoragePool(name, driver, source string) error {
	config := map[string]string{}
	if source != "" {
		config["source"] = source
	}
	return c.CreateStoragePoolWithConfig(name, driver, config)
}

// CreateStoragePoolWithConfig 使用完整配置创建存储池
func (c *Client) CreateStoragePoolWithConfig(name, driver string, config map[string]string) error {
	if config == nil {
		config = map[string]string{}
	}
	body := map[string]interface{}{
		"config": config,
		"driver": driver,
		"name":   name,
	}
	resp, err := c.doRequest("POST", "/storage-pools", body)
	if err != nil {
		return err
	}
	opStr, err := parseResponse(resp, nil)
	if err != nil {
		return err
	}
	if opStr != "" {
		return c.waitOperation(filepath.Base(opStr))
	}
	return nil
}

// StoragePoolInfo 存储池信息
type StoragePoolInfo struct {
	Name           string `json:"name"`
	Driver         string `json:"driver"`
	Source         string `json:"source"`
	Size           int64  `json:"size"`
	Used           int64  `json:"used"`
	InUse          bool   `json:"in_use"`
	QuotaSupported bool   `json:"quota_supported"`
}

// ListStoragePools 列出所有存储池
func (c *Client) ListStoragePools() ([]StoragePoolInfo, error) {
	resp, err := c.doRequest("GET", "/storage-pools?recursion=1", nil)
	if err != nil {
		return nil, err
	}
	var pools []struct {
		Name        string `json:"name"`
		Driver      string `json:"driver"`
		Description string `json:"description"`
		Config      struct {
			Source string `json:"source"`
			Size   string `json:"size"`
			Zfs    string `json:"zfs.pool_name"`
			LvmVg  string `json:"lvm.vg_name"`
		} `json:"config"`
		Status string   `json:"status"`
		UsedBy []string `json:"used_by"`
	}
	if _, err := parseResponse(resp, &pools); err != nil {
		return nil, err
	}
	var result []StoragePoolInfo
	for _, p := range pools {
		source := p.Config.Source
		if source == "" {
			source = p.Config.LvmVg
		}

		info := StoragePoolInfo{
			Name:   p.Name,
			Driver: p.Driver,
			Source: source,
			InUse:  len(p.UsedBy) > 0,
		}

		// 配额支持：dir 驱动需检查 source 分区是否 ext4/xfs 启用 project quota，其他驱动原生支持
		if p.Driver == "dir" {
			info.QuotaSupported = checkDirQuota(source)
		} else {
			info.QuotaSupported = true
		}

		// 查询存储池资源使用情况
		resResp, err := c.doRequest("GET", fmt.Sprintf("/storage-pools/%s/resources", url.PathEscape(p.Name)), nil)
		if err == nil {
			var res struct {
				Space struct {
					Used  int64 `json:"used"`
					Total int64 `json:"total"`
				} `json:"space"`
			}
			if _, err := parseResponse(resResp, &res); err == nil {
				info.Used = res.Space.Used
				info.Size = res.Space.Total
			}
		}

		result = append(result, info)
	}
	return result, nil
}

// checkDirQuota 检查 dir 存储池的 source 路径是否支持 project quota
// source 是挂载点路径或设备路径，需要检查底层文件系统是否 ext4/xfs 且启用 project quota
func checkDirQuota(source string) bool {
	if source == "" {
		return false
	}

	// source 可能是设备路径（如 /dev/sda1）或挂载点路径
	// 先尝试用 findmnt 查找该路径的文件系统信息
	var mountpoint string
	if strings.HasPrefix(source, "/dev/") {
		// 设备路径，查找其挂载点
		out, err := exec.Command("findmnt", "-no", "TARGET", source).Output()
		if err != nil {
			return false
		}
		mountpoint = strings.TrimSpace(string(out))
	} else {
		mountpoint = source
	}

	if mountpoint == "" {
		return false
	}

	// 获取文件系统类型和挂载选项
	out, err := exec.Command("findmnt", "-no", "FSTYPE,OPTIONS", mountpoint).Output()
	if err != nil {
		return false
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) < 2 {
		return false
	}
	fsType := parts[0]
	options := parts[1]

	switch fsType {
	case "ext4":
		if strings.Contains(options, "prjquota") || strings.Contains(options, "quota") {
			return true
		}
		// 检查 tune2fs 是否启用了 project quota
		deviceOut, err := exec.Command("findmnt", "-no", "SOURCE", mountpoint).Output()
		if err != nil {
			return false
		}
		device := strings.TrimSpace(string(deviceOut))
		tuneOut, err := exec.Command("tune2fs", "-l", device).Output()
		if err == nil {
			return strings.Contains(string(tuneOut), "Project")
		}
		return false
	case "xfs":
		return strings.Contains(options, "prjquota")
	default:
		return false
	}
}

// ImageAlias Incus 镜像别名
type ImageAlias struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ImageInfo 镜像信息
type ImageInfo struct {
	Fingerprint  string       `json:"fingerprint"`
	Aliases      []ImageAlias `json:"aliases"`
	Type         string       `json:"type"`
	Architecture string       `json:"architecture"`
	Public       bool         `json:"public"`
	Size         int64        `json:"size"`
}

// ListImages 列出所有本地镜像，返回 image_key 列表 (alias|type|arch)
func (c *Client) ListImages() ([]string, error) {
	resp, err := c.doRequest("GET", "/images?recursion=1", nil)
	if err != nil {
		return nil, err
	}
	var images []ImageInfo
	if _, err := parseResponse(resp, &images); err != nil {
		return nil, err
	}
	var keys []string
	for _, img := range images {
		for _, alias := range img.Aliases {
			if alias.Name != "" {
				keys = append(keys, BuildImageKey(alias.Name, img.Type, img.Architecture))
			}
		}
	}
	return keys, nil
}

// StoragePoolExists 检查存储池是否存在
func (c *Client) StoragePoolExists(name string) bool {
	resp, err := c.doRequest("GET", fmt.Sprintf("/storage-pools/%s", name), nil)
	if err != nil {
		return false
	}
	var pool struct {
		Name string `json:"name"`
	}
	if _, err := parseResponse(resp, &pool); err != nil {
		return false
	}
	return pool.Name == name
}

// StoragePoolDetail 存储池详情（含 used_by 和状态）
type StoragePoolDetail struct {
	Name   string            `json:"name"`
	Driver string            `json:"driver"`
	Config map[string]string `json:"config"`
	UsedBy []string          `json:"used_by"`
	Status string            `json:"status"`
}

// GetStoragePool 获取单个存储池详情
func (c *Client) GetStoragePool(name string) (*StoragePoolDetail, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/storage-pools/%s", name), nil)
	if err != nil {
		return nil, fmt.Errorf("获取存储池 %s 失败: %w", name, err)
	}
	var detail StoragePoolDetail
	if _, err := parseResponse(resp, &detail); err != nil {
		return nil, fmt.Errorf("解析存储池 %s 详情失败: %w", name, err)
	}
	return &detail, nil
}

// DeleteStoragePool 删除存储池
func (c *Client) DeleteStoragePool(name string) error {
	resp, err := c.doRequest("DELETE", fmt.Sprintf("/storage-pools/%s", name), nil)
	if err != nil {
		return fmt.Errorf("删除存储池 %s 请求失败: %w", name, err)
	}
	opStr, err := parseResponse(resp, nil)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			zap.L().Info("存储池不存在，无需删除", zap.String("name", name))
			return nil
		}
		return fmt.Errorf("删除存储池 %s 失败: %w", name, err)
	}
	if opStr != "" {
		return c.waitOperation(filepath.Base(opStr))
	}
	return nil
}

// StoragePoolResources 存储池空间资源
type StoragePoolResources struct {
	Space struct {
		Total uint64 `json:"total"`
		Used  uint64 `json:"used"`
	} `json:"space"`
	Inodes struct {
		Total uint64 `json:"total"`
		Used  uint64 `json:"used"`
	} `json:"inodes"`
}

// GetStoragePoolResources 获取存储池空间用量
func (c *Client) GetStoragePoolResources(name string) (*StoragePoolResources, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/storage-pools/%s/resources", name), nil)
	if err != nil {
		return nil, fmt.Errorf("获取存储池 %s 资源失败: %w", name, err)
	}
	var res StoragePoolResources
	if _, err := parseResponse(resp, &res); err != nil {
		return nil, fmt.Errorf("解析存储池 %s 资源失败: %w", name, err)
	}
	return &res, nil
}

// VolumeInfo 存储卷信息
type VolumeInfo struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	ContentType string            `json:"content_type"`
	Config      map[string]string `json:"config"`
	UsedBy      []string          `json:"used_by"`
	Location    string            `json:"location"`
	CreatedAt   time.Time         `json:"created_at"`
}

// ListStorageVolumes 列出指定存储池内的所有 volume
func (c *Client) ListStorageVolumes(pool string) ([]VolumeInfo, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/storage-pools/%s/volumes?recursion=1", pool), nil)
	if err != nil {
		return nil, fmt.Errorf("列出存储池 %s 卷失败: %w", pool, err)
	}
	var volumes []VolumeInfo
	if _, err := parseResponse(resp, &volumes); err != nil {
		return nil, fmt.Errorf("解析存储池 %s 卷列表失败: %w", pool, err)
	}
	return volumes, nil
}

// DeleteStorageVolume 删除指定存储池中的 volume
func (c *Client) DeleteStorageVolume(pool, volType, name string) error {
	resp, err := c.doRequest("DELETE", fmt.Sprintf("/storage-pools/%s/volumes/%s/%s", pool, volType, name), nil)
	if err != nil {
		return fmt.Errorf("删除 volume %s/%s/%s 请求失败: %w", pool, volType, name, err)
	}
	opStr, err := parseResponse(resp, nil)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return fmt.Errorf("删除 volume %s/%s/%s 失败: %w", pool, volType, name, err)
	}
	if opStr != "" {
		return c.waitOperation(filepath.Base(opStr))
	}
	return nil
}

// NetworkInfo Incus 网络信息
type NetworkInfo struct {
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Config map[string]string `json:"config"`
}

// ListNetworks 列出所有 Incus 网络
func (c *Client) ListNetworks() ([]NetworkInfo, error) {
	resp, err := c.doRequest("GET", "/networks?recursion=1", nil)
	if err != nil {
		return nil, err
	}
	var networks []NetworkInfo
	if _, err := parseResponse(resp, &networks); err != nil {
		return nil, err
	}
	return networks, nil
}

// NetworkExists 检查网络是否存在
func (c *Client) NetworkExists(name string) bool {
	resp, err := c.doRequest("GET", fmt.Sprintf("/networks/%s", name), nil)
	if err != nil {
		return false
	}
	var net struct {
		Name string `json:"name"`
	}
	if _, err := parseResponse(resp, &net); err != nil {
		return false
	}
	return net.Name == name
}

// ServerInfo Incus 服务器信息
type ServerInfo struct {
	APIVersion  string            `json:"api_version"`
	Auth        string            `json:"auth"`
	Public      bool              `json:"public"`
	Environment ServerEnvironment `json:"environment"`
}

// ServerEnvironment 服务器环境
type ServerEnvironment struct {
	Addresses              []string          `json:"addresses"`
	Architectures          []string          `json:"architectures"`
	Certificate            string            `json:"certificate"`
	CertificateFingerprint string            `json:"certificate_fingerprint"`
	Driver                 string            `json:"driver"`
	DriverVersion          string            `json:"driver_version"`
	Firewall               string            `json:"firewall"`
	Kernel                 string            `json:"kernel"`
	KernelArchitecture     string            `json:"kernel_architecture"`
	KernelFeatures         map[string]string `json:"kernel_features"`
	KernelVersion          string            `json:"kernel_version"`
	OSName                 string            `json:"os_name"`
	OSVersion              string            `json:"os_version"`
	Project                string            `json:"project"`
	Server                 string            `json:"server"`
	ServerPid              int               `json:"server_pid"`
	ServerVersion          string            `json:"server_version"`
	Storage                string            `json:"storage"`
	StorageVersion         string            `json:"storage_version"`
}

// DataDisk 数据盘
type DataDisk struct {
	Name        string `json:"name"`
	SizeGB      int    `json:"size_gb"`
	StoragePool string `json:"storage_pool"`
	MountPoint  string `json:"mount_point"`
}

// CreateInstanceRequest 创建实例请求
type CreateInstanceRequest struct {
	Name          string
	TemplateID    string
	Type          string
	VCPU          float64
	MemoryMB      int
	DiskGB        int
	StoragePool   string
	NetworkBridge string
	IPv4Address   string
	IPv6Address   string
	UserData      string     // cloud-init user-data YAML
	NetworkConfig string     // cloud-init network-config YAML
	DataDisks     []DataDisk // 数据盘
	// Bridge 网络配置
	BridgeName   string // Incus bridge 名称，如 br-xxx
	InternalIPv4 string // 静态内网 IP
	GatewayV4    string // IPv4 网关
	IPv4CIDR     string // IPv4 CIDR，用于生成 network-config
	IPv4Filter   bool   // security.ipv4_filter
	MACFilter    bool   // security.mac_filter
	NetworkDown  int    // 下行限速 Mbit
	NetworkUp    int    // 上行限速 Mbit
}

// InstanceInfo 实例信息
type InstanceInfo struct {
	Name       string                 `json:"name"`
	Status     string                 `json:"status"`
	StatusCode int                    `json:"status_code"`
	Type       string                 `json:"type"`
	Config     map[string]string      `json:"config"`
	Devices    map[string]interface{} `json:"devices"`
	CreatedAt  time.Time              `json:"created_at"`
}

// InstanceMetrics 实例监控指标
type InstanceMetrics struct {
	Status       string
	StatusCode   int
	MemUsage     int64
	MemTotal     int64
	CPUUsage     int64
	Processes    int
	DiskUsage    map[string]int64
	NetworkStats map[string]NetworkStat
}

// NetworkStat 网络统计
type NetworkStat struct {
	BytesReceived   int64
	BytesSent       int64
	PacketsReceived int64
	PacketsSent     int64
}

// ExecResult 执行结果
type ExecResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

// CreateBridgeNetwork 通过 Incus API 创建 bridge 网络
// ipv4CIDR: 如 "10.10.1.0/24"，gatewayV4: 如 "10.10.1.1"
func (c *Client) CreateBridgeNetwork(name, ipv4CIDR, ipv6ULA, ipv6GUA, gatewayV4 string) error {
	// 构造带前缀的网关地址，如 "10.10.1.1/24"
	ipv4Addr := gatewayV4
	if ipv4CIDR != "" {
		_, ipNet, err := net.ParseCIDR(ipv4CIDR)
		if err == nil {
			ones, _ := ipNet.Mask.Size()
			ipv4Addr = fmt.Sprintf("%s/%d", gatewayV4, ones)
		}
	}

	config := map[string]string{
		"ipv4.address": ipv4Addr,
		"ipv4.nat":     "false",
		"ipv4.dhcp":    "true",
	}
	if ipv6ULA != "" {
		config["ipv6.address"] = ipv6ULA
		config["ipv6.nat"] = "false"
		config["ipv6.dhcp"] = "false"
	} else if ipv6GUA != "" {
		config["ipv6.address"] = ipv6GUA
		config["ipv6.nat"] = "false"
		config["ipv6.dhcp"] = "false"
	}

	body := map[string]interface{}{
		"name":   name,
		"type":   "bridge",
		"config": config,
	}

	resp, err := c.doRequest("POST", "/networks", body)
	if err != nil {
		return fmt.Errorf("创建 bridge 网络请求失败: %w", err)
	}

	opID, err := parseResponse(resp, nil)
	if err != nil {
		// 如果网络已存在，不算错误
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "Network already exists") {
			zap.L().Info("bridge 网络已存在", zap.String("name", name))
			return nil
		}
		return fmt.Errorf("创建 bridge 网络失败: %w", err)
	}

	// 如果有异步操作，等待完成
	if opID != "" {
		// opID 可能是 "/1.0/operations/xxx"，提取纯 ID
		pureOpID := opID
		if idx := strings.LastIndex(opID, "/"); idx >= 0 {
			pureOpID = opID[idx+1:]
		}
		zap.L().Info("等待 bridge 创建操作完成", zap.String("name", name), zap.String("operation", pureOpID))
		if err := c.waitOperation(pureOpID); err != nil {
			return fmt.Errorf("等待 bridge 创建操作失败: %w", err)
		}
	}
	return nil
}

// UpdateBridgeNetwork 更新 bridge 网络配置
func (c *Client) UpdateBridgeNetwork(name string, config map[string]string) error {
	body := map[string]interface{}{
		"config": config,
	}
	resp, err := c.doRequest("PATCH", "/networks/"+name, body)
	if err != nil {
		return fmt.Errorf("更新 bridge 网络请求失败: %w", err)
	}
	opID, err := parseResponse(resp, nil)
	if err != nil {
		return fmt.Errorf("更新 bridge 网络响应解析失败: %w", err)
	}
	if opID != "" {
		pureOpID := opID
		if idx := strings.LastIndex(opID, "/"); idx >= 0 {
			pureOpID = opID[idx+1:]
		}
		if err := c.waitOperation(pureOpID); err != nil {
			return fmt.Errorf("等待 bridge 更新操作失败: %w", err)
		}
	}
	return nil
}

// DeleteBridgeNetwork 通过 Incus API 删除 bridge 网络
func (c *Client) DeleteBridgeNetwork(name string) error {
	resp, err := c.doRequest("DELETE", "/networks/"+name, nil)
	if err != nil {
		// 网络不存在不算错误
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "Network not found") || strings.Contains(err.Error(), "not found") {
			zap.L().Info("bridge 网络不存在，无需删除", zap.String("name", name))
			return nil
		}
		return fmt.Errorf("删除 bridge 网络请求失败: %w", err)
	}
	opID, err := parseResponse(resp, nil)
	if err != nil {
		// 如果不存在，不算错误
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "Network not found") {
			zap.L().Info("bridge 网络不存在，无需删除", zap.String("name", name))
			return nil
		}
		return fmt.Errorf("删除 bridge 网络失败: %w", err)
	}
	// 如果有异步操作，等待完成
	if opID != "" {
		pureOpID := opID
		if idx := strings.LastIndex(opID, "/"); idx >= 0 {
			pureOpID = opID[idx+1:]
		}
		if err := c.waitOperation(pureOpID); err != nil {
			return fmt.Errorf("等待 bridge 删除操作失败: %w", err)
		}
	}
	return nil
}
