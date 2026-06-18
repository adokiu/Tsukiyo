package incus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Client Incus API 客户端
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient 创建 Incus 客户端
func NewClient(socketPath string) (*Client, error) {
	if socketPath == "" {
		socketPath = "/var/lib/incus/unix.socket"
	}

	transport := &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		},
	}

	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   60 * time.Second,
		},
		baseURL: "http://unix/1.0",
	}, nil
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
		zap.L().Error("Incus HTTP 请求失败", zap.String("method", method), zap.String("path", path), zap.Error(err))
		return nil, err
	}
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

// waitOperation 等待异步操作完成
func (c *Client) waitOperation(opID string) error {
	for i := 0; i < 300; i++ {
		resp, err := c.doRequest("GET", "/operations/"+opID+"/wait?timeout=30", nil)
		if err != nil {
			return err
		}
		// parseResponse 已将外层 metadata 剥离，直接解析 operation 对象
		var result struct {
			Status     string `json:"status"`
			Err        string `json:"err"`
			StatusCode int    `json:"status_code"`
		}
		if _, err := parseResponse(resp, &result); err != nil {
			return err
		}
		if result.Status == "Success" {
			return nil
		}
		if result.Status == "Failure" {
			return fmt.Errorf("operation failed: %s", result.Err)
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("operation timeout")
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
		"parent":  "incusbr0",
		"type":    "nic",
	}
	// VPC 网络配置优先
	if req.BridgeName != "" {
		nicDevice["parent"] = req.BridgeName
	} else if req.NetworkBridge != "" {
		nicDevice["parent"] = req.NetworkBridge
	}
	if req.InternalIPv4 != "" {
		nicDevice["ipv4.address"] = req.InternalIPv4
		// Incus bridge 禁用 DHCP 时，静态 IP 必须同时开启 ipv4_filtering
		nicDevice["security.ipv4_filtering"] = "true"
	} else if req.IPv4Address != "" {
		nicDevice["ipv4.address"] = req.IPv4Address
	}
	if req.IPv6Address != "" {
		nicDevice["ipv6.address"] = req.IPv6Address
	}
	if req.IPv4Filter {
		nicDevice["security.ipv4_filtering"] = "true"
	}
	if req.MACFilter {
		nicDevice["security.mac_filtering"] = "true"
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
	// VPC 静态内网 IP：通过 cloud-init network-config 配置容器内部 eth0
	if req.InternalIPv4 != "" && req.GatewayV4 != "" {
		prefixLen := 24
		if req.IPv4CIDR != "" {
			_, ipNet, err := net.ParseCIDR(req.IPv4CIDR)
			if err == nil {
				ones, _ := ipNet.Mask.Size()
				prefixLen = ones
			}
		}
		networkConfig := fmt.Sprintf("version: 1\nconfig:\n  - type: physical\n    name: eth0\n    subnets:\n      - type: static\n        control: auto\n        address: %s/%d\n        gateway: %s\n",
			req.InternalIPv4, prefixLen, req.GatewayV4)
		configMap["cloud-init.network-config"] = networkConfig
		zap.L().Info("已生成 cloud-init network-config", zap.String("address", req.InternalIPv4), zap.Int("prefix", prefixLen), zap.String("gateway", req.GatewayV4))
	}

	body := map[string]interface{}{
		"name":    req.Name,
		"source":  map[string]string{"type": "image", "alias": req.TemplateID},
		"config":  configMap,
		"devices": devices,
		"type":    req.Type,
	}
	if req.Type == "" {
		body["type"] = "container"
	}

	zap.L().Info("CreateInstance 发送 POST /instances", zap.String("name", req.Name))
	resp, err := c.doRequest("POST", "/instances", body)
	if err != nil {
		zap.L().Error("CreateInstance POST 请求失败", zap.String("name", req.Name), zap.Error(err))
		return "", err
	}
	zap.L().Info("CreateInstance POST 请求成功", zap.String("name", req.Name))

	opStr, err := parseResponse(resp, nil)
	if err != nil {
		zap.L().Error("CreateInstance 解析响应失败", zap.String("name", req.Name), zap.Error(err))
		return "", err
	}
	zap.L().Info("CreateInstance 获取 operation", zap.String("name", req.Name), zap.String("operation", opStr))

	if opStr == "" {
		zap.L().Info("CreateInstance 同步完成，无需等待 operation", zap.String("name", req.Name))
		return req.Name, nil
	}

	opID := filepath.Base(opStr)
	zap.L().Info("CreateInstance 等待 operation", zap.String("name", req.Name), zap.String("op_id", opID))
	if err := c.waitOperation(opID); err != nil {
		zap.L().Error("CreateInstance waitOperation 失败", zap.String("name", req.Name), zap.String("op_id", opID), zap.Error(err))
		return "", err
	}
	zap.L().Info("CreateInstance operation 完成", zap.String("name", req.Name))

	return req.Name, nil
}

// StartInstance 启动实例
func (c *Client) StartInstance(name string) error {
	resp, err := c.doRequest("PUT", fmt.Sprintf("/instances/%s/state", name), map[string]string{"action": "start"})
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
	// 1. 获取实例状态
	info, err := c.GetInstance(name)
	if err != nil {
		// 实例已不存在，视为成功
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "Not found") {
			zap.L().Info("实例已不存在，删除成功", zap.String("name", name))
			return nil
		}
		return fmt.Errorf("获取实例状态失败: %w", err)
	}

	// 2. 如果正在运行，先强制停止
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
		// 给 Incus 一点时间完成状态切换
		time.Sleep(500 * time.Millisecond)
	}

	// 3. 删除实例
	resp, err := c.doRequest("DELETE", fmt.Sprintf("/instances/%s", name), nil)
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
		"bind=host", "listen="+listenAddr, "connect="+connectAddr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// 设备已存在，先删再加
		if strings.Contains(string(output), "already exists") {
			_ = exec.Command("incus", "config", "device", "remove", instanceName, deviceName).Run()
			cmd = exec.Command("incus", "config", "device", "add", instanceName, deviceName, "proxy",
				"bind=host", "listen="+listenAddr, "connect="+connectAddr)
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
func (c *Client) ExecCommand(instanceName string, cmd []string, env map[string]string, user string, group string) (*ExecResult, error) {
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
	if group != "" {
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
	// 尝试使用 incus exec 设置密码
	cmd := []string{"/bin/sh", "-c", fmt.Sprintf("echo 'root:%s' | chpasswd", password)}
	_, err := c.ExecCommand(name, cmd, nil, "", "")
	return err
}

// EnsureSSHInstalled 在容器内安装并启动 openssh-server（支持 apt/apk/dnf/yum）
func (c *Client) EnsureSSHInstalled(name string) error {
	script := `set -e
if command -v sshd >/dev/null 2>&1 || [ -x /usr/sbin/sshd ]; then
  # 已有 sshd，尝试启动服务
  if command -v systemctl >/dev/null 2>&1; then
    systemctl start sshd 2>/dev/null || systemctl start ssh 2>/dev/null || true
  elif command -v rc-service >/dev/null 2>&1; then
    rc-service sshd start 2>/dev/null || true
  elif command -v service >/dev/null 2>&1; then
    service ssh start 2>/dev/null || service sshd start 2>/dev/null || true
  fi
  exit 0
fi

export DEBIAN_FRONTEND=noninteractive
if command -v apt-get >/dev/null 2>&1; then
  apt-get update -qq >/dev/null 2>&1 || true
  apt-get install -y -qq openssh-server >/dev/null 2>&1 || true
elif command -v apk >/dev/null 2>&1; then
  apk add --no-cache openssh >/dev/null 2>&1 || true
elif command -v dnf >/dev/null 2>&1; then
  dnf install -y openssh-server >/dev/null 2>&1 || true
elif command -v yum >/dev/null 2>&1; then
  yum install -y openssh-server >/dev/null 2>&1 || true
fi

if command -v systemctl >/dev/null 2>&1; then
  systemctl enable sshd 2>/dev/null || systemctl enable ssh 2>/dev/null || true
  systemctl start sshd 2>/dev/null || systemctl start ssh 2>/dev/null || true
elif command -v rc-service >/dev/null 2>&1; then
  rc-update add sshd default 2>/dev/null || true
  rc-service sshd start 2>/dev/null || true
elif command -v service >/dev/null 2>&1; then
  service ssh start 2>/dev/null || service sshd start 2>/dev/null || true
fi
`
	_, err := c.ExecCommand(name, []string{"/bin/sh", "-c", script}, nil, "", "")
	return err
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
	body := map[string]interface{}{
		"config": map[string]string{},
		"driver": driver,
		"name":   name,
	}
	if source != "" {
		body["config"] = map[string]string{
			"source": source,
		}
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
	Name   string `json:"name"`
	Driver string `json:"driver"`
	Source string `json:"source"`
	Size   int64  `json:"size"`
	Used   int64  `json:"used"`
	InUse  bool   `json:"in_use"`
}

// ListStoragePools 列出所有存储池
func (c *Client) ListStoragePools() ([]StoragePoolInfo, error) {
	resp, err := c.doRequest("GET", "/storage-pools?recursion=1", nil)
	if err != nil {
		return nil, err
	}
	var pools []struct {
		Name   string `json:"name"`
		Driver string `json:"driver"`
		Config struct {
			Source string `json:"source,omitempty"`
		} `json:"config"`
	}
	if _, err := parseResponse(resp, &pools); err != nil {
		return nil, err
	}
	var result []StoragePoolInfo
	for _, p := range pools {
		result = append(result, StoragePoolInfo{
			Name:   p.Name,
			Driver: p.Driver,
			Source: p.Config.Source,
			InUse:  true,
		})
	}
	return result, nil
}

// ImageAlias Incus 镜像别名
type ImageAlias struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ImageInfo 镜像信息
type ImageInfo struct {
	Fingerprint string       `json:"fingerprint"`
	Aliases     []ImageAlias `json:"aliases"`
	Type        string       `json:"type"`
	Public      bool         `json:"public"`
	Size        int64        `json:"size"`
}

// ListImages 列出所有本地镜像，返回别名列表
func (c *Client) ListImages() ([]string, error) {
	resp, err := c.doRequest("GET", "/images?recursion=1", nil)
	if err != nil {
		return nil, err
	}
	var images []ImageInfo
	if _, err := parseResponse(resp, &images); err != nil {
		return nil, err
	}
	var aliases []string
	for _, img := range images {
		for _, alias := range img.Aliases {
			if alias.Name != "" {
				aliases = append(aliases, alias.Name)
			}
		}
	}
	return aliases, nil
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
	DataDisks     []DataDisk // 数据盘
	// VPC 网络配置
	BridgeName   string // Incus bridge 名称，如 vpc-xxx
	InternalIPv4 string // 静态内网 IP
	GatewayV4    string // IPv4 网关
	IPv4CIDR     string // IPv4 CIDR，用于生成 network-config
	IPv4Filter   bool   // security.ipv4_filter
	MACFilter    bool   // security.mac_filter
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
		"ipv4.dhcp":    "false",
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

// DeleteBridgeNetwork 通过 Incus API 删除 bridge 网络
func (c *Client) DeleteBridgeNetwork(name string) error {
	resp, err := c.doRequest("DELETE", "/networks/"+name, nil)
	if err != nil {
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
