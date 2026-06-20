package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/service"
	"tsukiyo/master/internal/service/infrastructure"
)

var nodeService *infrastructure.NodeService

// InitNodeService 初始化节点服务
func InitNodeService(svc *infrastructure.NodeService) {
	nodeService = svc
}

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

	node, err := nodeService.CreateNode(req.Name)
	if err != nil {
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
	nodes, err := nodeService.ListNodes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	result := make([]gin.H, 0, len(nodes))
	for _, node := range nodes {
		isOnline := nodeService.IsNodeOnline(&node)

		result = append(result, gin.H{
			"id":                   node.ID.String(),
			"name":                 node.Name,
			"token":                node.Token,
			"hostname":             node.Hostname,
			"ip_address":           node.IPAddress,
			"ipv6_address":         node.IPv6Address,
			"country_code":         node.CountryCode,
			"status":               node.Status,
			"is_online":            isOnline,
			"incus_version":        node.IncusVersion,
			"total_cpu":            node.TotalCPU,
			"total_memory":         node.TotalMemory,
			"total_disk":           node.TotalDisk,
			"used_cpu":             node.UsedCPU,
			"used_memory":          node.UsedMemory,
			"used_disk":            node.UsedDisk,
			"net_in":               node.NetIn,
			"net_out":              node.NetOut,
			"uptime":               node.Uptime,
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
			"agent_url":            node.AgentURL,
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

	node, err := nodeService.GetNode(nodeID)
	if err != nil {
		if err == service.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	isOnline := nodeService.IsNodeOnline(node)

	c.JSON(http.StatusOK, gin.H{
		"id":                   node.ID.String(),
		"name":                 node.Name,
		"token":                node.Token,
		"hostname":             node.Hostname,
		"ip_address":           node.IPAddress,
		"ipv6_address":         node.IPv6Address,
		"country_code":         node.CountryCode,
		"status":               node.Status,
		"is_online":            isOnline,
		"incus_version":        node.IncusVersion,
		"total_cpu":            node.TotalCPU,
		"total_memory":         node.TotalMemory,
		"total_disk":           node.TotalDisk,
		"used_cpu":             node.UsedCPU,
		"used_memory":          node.UsedMemory,
		"used_disk":            node.UsedDisk,
		"net_in":               node.NetIn,
		"net_out":              node.NetOut,
		"uptime":               node.Uptime,
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
		"agent_url":            node.AgentURL,
		"image_remote_url":     node.ImageRemoteURL,
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
	AgentURL           string `json:"agent_url"`
	ImageRemoteURL     string `json:"image_remote_url"`
	StoragePoolType    string `json:"storage_pool_type"`
	StoragePoolSource  string `json:"storage_pool_source"`
}

// UpdateNodeConfig 更新节点配置
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

	config := map[string]interface{}{
		"incus_socket_path":    req.IncusSocketPath,
		"metrics_interval":     req.MetricsInterval,
		"heartbeat_interval":   req.HeartbeatInterval,
		"network_interface":    req.NetworkInterface,
		"enable_nat":           req.EnableNAT,
		"enable_firewall":      req.EnableFirewall,
		"enable_security_scan": req.EnableSecurityScan,
		"scan_interval":        req.ScanInterval,
		"console_bind_addr":    req.ConsoleBindAddr,
		"agent_url":            req.AgentURL,
		"image_remote_url":     req.ImageRemoteURL,
		"storage_pool_type":    req.StoragePoolType,
		"storage_pool_source":  req.StoragePoolSource,
	}

	if err := nodeService.UpdateNodeConfig(nodeID, config); err != nil {
		if err == service.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置失败"})
		return
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

	if err := nodeService.DeleteNode(nodeID); err != nil {
		if err == service.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
			return
		}
		if err == service.ErrNodeHasInstances {
			c.JSON(http.StatusConflict, gin.H{"error": "节点下存在实例，无法删除"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除节点失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "节点删除成功"})
}

// GetNodeNetworks 获取节点网卡列表
func GetNodeNetworks(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}

	networks, err := nodeService.GetNodeNetworks(nodeID)
	if err != nil {
		if err == service.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"networks": networks})
}

// GetNodeBridges 获取节点的网桥列表
func GetNodeBridges(c *gin.Context) {
	nodeID := c.Param("id")
	if _, err := uuid.Parse(nodeID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}

	var bridges []models.Bridge
	if err := db.DB.Where("node_id = ?", nodeID).Order("created_at DESC").Find(&bridges).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询网桥失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": bridges})
}

// GetNodeTasks 获取节点的任务历史
func GetNodeTasks(c *gin.Context) {
	nodeID := c.Param("id")
	if _, err := uuid.Parse(nodeID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}

	var tasks []models.Task
	query := db.DB.Where("node_id = ?", nodeID).Order("created_at DESC")

	limit := 50
	if l := c.Query("limit"); l != "" {
		if v, err := parseIntParam(l); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}
	query = query.Limit(limit)

	if err := query.Find(&tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询任务失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": tasks})
}

// GetNodeSecurityAlerts 获取节点的安全告警
func GetNodeSecurityAlerts(c *gin.Context) {
	nodeID := c.Param("id")
	if _, err := uuid.Parse(nodeID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}

	var alerts []models.SecurityAlert
	query := db.DB.Where("node_id = ?", nodeID).Order("detected_at DESC")

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	limit := 50
	if l := c.Query("limit"); l != "" {
		if v, err := parseIntParam(l); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}
	query = query.Limit(limit)

	if err := query.Find(&alerts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询安全告警失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": alerts})
}

func parseIntParam(s string) (int, error) {
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}
