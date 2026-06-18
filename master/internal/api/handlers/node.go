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

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// CreateNodeRequest 创建节点请求
type CreateNodeRequest struct {
	Name string `json:"name" binding:"required,max=64"`
}

// CreateNode 创建节点
func CreateNode(c *gin.Context) {
	var req CreateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 生成节点 Token
	token := uuid.New().String() + uuid.New().String()

	node := models.Node{
		ID:     uuid.New(),
		Name:   req.Name,
		Token:  token,
		Status: models.NodeStatusOffline,
	}

	if err := db.DB.Create(&node).Error; err != nil {
		zap.L().Error("创建节点失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建节点失败"})
		return
	}

	// 写审计日志
	userID, _ := c.Get("user_id")
	auditLog := models.AuditLog{
		UserID:    userID.(uint),
		Action:    "node:create",
		Target:    "node",
		Detail:    "创建节点: " + node.Name,
		IPAddress: c.ClientIP(),
		Success:   true,
	}
	db.DB.Create(&auditLog)

	c.JSON(http.StatusCreated, gin.H{
		"id":         node.ID.String(),
		"name":       node.Name,
		"token":      node.Token,
		"status":     node.Status,
		"created_at": node.CreatedAt,
	})
}

// ListNodes 获取节点列表
func ListNodes(c *gin.Context) {
	var nodes []models.Node
	if err := db.DB.Order("created_at DESC").Find(&nodes).Error; err != nil {
		zap.L().Error("查询节点列表失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	result := make([]gin.H, 0, len(nodes))
	for _, node := range nodes {
		isOnline := node.Status == models.NodeStatusOnline
		if node.LastHeartbeat != nil {
			isOnline = isOnline && time.Since(*node.LastHeartbeat) < 60*time.Second
		}

		result = append(result, gin.H{
			"id":                   node.ID.String(),
			"name":                 node.Name,
			"token":                node.Token,
			"hostname":             node.Hostname,
			"ip_address":           node.IPAddress,
			"status":               node.Status,
			"is_online":            isOnline,
			"initialized":          node.Initialized,
			"incus_version":        node.IncusVersion,
			"total_cpu":            node.TotalCPU,
			"total_memory":         node.TotalMemory,
			"total_disk":           node.TotalDisk,
			"used_cpu":             node.UsedCPU,
			"used_memory":          node.UsedMemory,
			"used_disk":            node.UsedDisk,
			"instance_count":       node.InstanceCount,
			"running_count":        node.RunningCount,
			"last_seen_at":         node.LastSeenAt,
			"last_heartbeat":       node.LastHeartbeat,
			"created_at":           node.CreatedAt,
			"incus_socket_path":    node.IncusSocketPath,
			"metrics_interval":     node.MetricsInterval,
			"heartbeat_interval":   node.HeartbeatInterval,
			"network_interface":    node.NetworkInterface,
			"enable_nat":           node.EnableNAT,
			"enable_firewall":      node.EnableFirewall,
			"enable_security_scan": node.EnableSecurityScan,
			"scan_interval":        node.ScanInterval,
			"console_bind_addr":    node.ConsoleBindAddr,
			"default_storage_pool": node.DefaultStoragePool,
			"storage_pool_type":    node.StoragePoolType,
			"storage_pool_source":  node.StoragePoolSource,
			"system_info":          node.SystemInfo,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  result,
		"total": len(result),
	})
}

// GetNode 获取节点详情
func GetNode(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}

	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	isOnline := node.Status == models.NodeStatusOnline
	if node.LastHeartbeat != nil {
		isOnline = isOnline && time.Since(*node.LastHeartbeat) < 60*time.Second
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                   node.ID.String(),
		"name":                 node.Name,
		"token":                node.Token,
		"hostname":             node.Hostname,
		"ip_address":           node.IPAddress,
		"status":               node.Status,
		"is_online":            isOnline,
		"incus_version":        node.IncusVersion,
		"total_cpu":            node.TotalCPU,
		"total_memory":         node.TotalMemory,
		"total_disk":           node.TotalDisk,
		"used_cpu":             node.UsedCPU,
		"used_memory":          node.UsedMemory,
		"used_disk":            node.UsedDisk,
		"instance_count":       node.InstanceCount,
		"running_count":        node.RunningCount,
		"last_seen_at":         node.LastSeenAt,
		"last_heartbeat":       node.LastHeartbeat,
		"created_at":           node.CreatedAt,
		"initialized":          node.Initialized,
		"incus_socket_path":    node.IncusSocketPath,
		"metrics_interval":     node.MetricsInterval,
		"heartbeat_interval":   node.HeartbeatInterval,
		"network_interface":    node.NetworkInterface,
		"enable_nat":           node.EnableNAT,
		"enable_firewall":      node.EnableFirewall,
		"enable_security_scan": node.EnableSecurityScan,
		"scan_interval":        node.ScanInterval,
		"console_bind_addr":    node.ConsoleBindAddr,
		"default_storage_pool": node.DefaultStoragePool,
		"storage_pool_type":    node.StoragePoolType,
		"storage_pool_source":  node.StoragePoolSource,
		"system_info":          node.SystemInfo,
	})
}

// UpdateNodeConfigRequest 更新节点配置请求
type UpdateNodeConfigRequest struct {
	IncusSocketPath    string `json:"incus_socket_path"`
	MetricsInterval    int    `json:"metrics_interval"`
	HeartbeatInterval  int    `json:"heartbeat_interval"`
	NetworkInterface   string `json:"network_interface"`
	EnableNAT          bool   `json:"enable_nat"`
	EnableFirewall     bool   `json:"enable_firewall"`
	EnableSecurityScan bool   `json:"enable_security_scan"`
	ScanInterval       int    `json:"scan_interval"`
	ConsoleBindAddr    string `json:"console_bind_addr"`
	DefaultStoragePool string `json:"default_storage_pool"`
	StoragePoolType    string `json:"storage_pool_type"`
	StoragePoolSource  string `json:"storage_pool_source"`
}

// UpdateNodeConfig 更新节点配置并下发给 Agent
func UpdateNodeConfig(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}

	var req UpdateNodeConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	updates := map[string]interface{}{
		"initialized":          true,
		"incus_socket_path":    req.IncusSocketPath,
		"metrics_interval":     req.MetricsInterval,
		"heartbeat_interval":   req.HeartbeatInterval,
		"network_interface":    req.NetworkInterface,
		"enable_nat":           req.EnableNAT,
		"enable_firewall":      req.EnableFirewall,
		"enable_security_scan": req.EnableSecurityScan,
		"scan_interval":        req.ScanInterval,
		"console_bind_addr":    req.ConsoleBindAddr,
		"default_storage_pool": req.DefaultStoragePool,
		"storage_pool_type":    req.StoragePoolType,
		"storage_pool_source":  req.StoragePoolSource,
		"status":               models.NodeStatusOnline,
	}

	if err := db.DB.Model(&node).Updates(updates).Error; err != nil {
		zap.L().Error("更新节点配置失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置失败"})
		return
	}

	// 下发配置给 Agent
	if agentMgr != nil {
		cfg := map[string]interface{}{
			"incus_socket_path":    req.IncusSocketPath,
			"metrics_interval":     req.MetricsInterval,
			"heartbeat_interval":   req.HeartbeatInterval,
			"network_interface":    req.NetworkInterface,
			"enable_nat":           req.EnableNAT,
			"enable_firewall":      req.EnableFirewall,
			"enable_security_scan": req.EnableSecurityScan,
			"scan_interval":        req.ScanInterval,
			"console_bind_addr":    req.ConsoleBindAddr,
			"default_storage_pool": req.DefaultStoragePool,
			"storage_pool_type":    req.StoragePoolType,
			"storage_pool_source":  req.StoragePoolSource,
		}
		if err := agentMgr.SendConfig(nodeID, cfg); err != nil {
			zap.L().Warn("下发配置给 Agent 失败", zap.String("node_id", nodeID.String()), zap.Error(err))
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "配置更新并下发成功"})
}

// DeleteNode 删除节点
func DeleteNode(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}

	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	// 检查节点下是否有实例
	var count int64
	db.DB.Model(&models.Instance{}).Where("node_id = ?", nodeID).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "节点下存在实例，无法删除"})
		return
	}

	if err := db.DB.Delete(&node).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除节点失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "节点删除成功"})
}

// GetNodeNetworks 获取节点网卡列表（从 Agent 上报的 system_info 解析）
func GetNodeNetworks(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}

	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	type networkInfo struct {
		Name   string   `json:"name"`
		Status string   `json:"status"`
		IPv4   []string `json:"ipv4"`
		IPv6   []string `json:"ipv6"`
		MAC    string   `json:"mac"`
	}

	var sysInfo struct {
		Networks []networkInfo `json:"networks"`
	}
	if err := json.Unmarshal([]byte(node.SystemInfo), &sysInfo); err != nil {
		c.JSON(http.StatusOK, gin.H{"networks": []networkInfo{}})
		return
	}

	// 过滤：排除 VPC bridge 和 loopback
	var filtered []networkInfo
	for _, n := range sysInfo.Networks {
		if n.Name == "lo" || strings.HasPrefix(n.Name, "vpc-") {
			continue
		}
		filtered = append(filtered, n)
	}

	c.JSON(http.StatusOK, gin.H{"networks": filtered})
}
