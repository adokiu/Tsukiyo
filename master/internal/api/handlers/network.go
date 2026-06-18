package handlers

import (
	"encoding/json"
	"net"
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

// IPPoolInfo IP 池信息
type IPPoolInfo struct {
	ID         string     `json:"id"`
	NodeID     string     `json:"node_id"`
	Address    string     `json:"address"`
	Gateway    string     `json:"gateway,omitempty"`
	PrefixLen  int        `json:"prefix_len"`
	Interface  string     `json:"interface,omitempty"`
	Status     string     `json:"status"`
	InstanceID *string    `json:"instance_id,omitempty"`
	AssignedAt *time.Time `json:"assigned_at,omitempty"`
}

// ListIPPools 获取 IP 池列表
func ListIPPools(c *gin.Context) {
	nodeID := c.Query("node_id")
	status := c.Query("status")

	query := db.DB.Order("address ASC")
	if nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var ips []models.PublicIPPool
	if err := query.Find(&ips).Error; err != nil {
		zap.L().Error("查询 IP 池失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	result := make([]IPPoolInfo, 0, len(ips))
	for _, ip := range ips {
		info := IPPoolInfo{
			ID:        ip.ID.String(),
			NodeID:    ip.NodeID.String(),
			Address:   ip.Address,
			Gateway:   ip.Gateway,
			PrefixLen: ip.PrefixLen,
			Interface: ip.Interface,
			Status:    string(ip.Status),
		}
		if ip.InstanceID != uuid.Nil {
			s := ip.InstanceID.String()
			info.InstanceID = &s
		}
		if ip.AssignedAt != nil {
			info.AssignedAt = ip.AssignedAt
		}
		result = append(result, info)
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  result,
		"total": len(result),
	})
}

// AddIPPoolRequest 添加 IP 请求
type AddIPPoolRequest struct {
	NodeID    string `json:"node_id" binding:"required,uuid"`
	Address   string `json:"address" binding:"required,ip"`
	Gateway   string `json:"gateway,omitempty"`
	PrefixLen int    `json:"prefix_len,omitempty"`
	Interface string `json:"interface,omitempty"`
}

// AddIPPool 添加 IP 到池
func AddIPPool(c *gin.Context) {
	var req AddIPPoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	nodeID, _ := uuid.Parse(req.NodeID)

	// 检查 IP 是否已存在
	var existing models.PublicIPPool
	if err := db.DB.Where("address = ? AND node_id = ?", req.Address, nodeID).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "该 IP 已在池中"})
		return
	}

	prefixLen := req.PrefixLen
	if prefixLen <= 0 || prefixLen > 32 {
		prefixLen = 32
	}

	ip := models.PublicIPPool{
		ID:        uuid.New(),
		NodeID:    nodeID,
		Address:   req.Address,
		Gateway:   req.Gateway,
		PrefixLen: prefixLen,
		Interface: req.Interface,
		Status:    models.IPStatusFree,
	}

	if err := db.DB.Create(&ip).Error; err != nil {
		zap.L().Error("添加 IP 失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "添加 IP 失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":      ip.ID.String(),
		"address": ip.Address,
		"node_id": ip.NodeID.String(),
		"status":  ip.Status,
	})
}

// DeleteIPPool 从池中删除 IP
func DeleteIPPool(c *gin.Context) {
	ipID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 IP ID"})
		return
	}

	var ip models.PublicIPPool
	if err := db.DB.Where("id = ?", ipID).First(&ip).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "IP 不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	if ip.Status == models.IPStatusAssigned {
		c.JSON(http.StatusConflict, gin.H{"error": "该 IP 已被分配，无法删除"})
		return
	}

	if err := db.DB.Delete(&ip).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ListIPv6Prefixes 获取 IPv6 前缀列表
func ListIPv6Prefixes(c *gin.Context) {
	nodeID := c.Query("node_id")

	query := db.DB.Order("prefix ASC")
	if nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}

	var prefixes []models.IPv6Prefix
	if err := query.Find(&prefixes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	result := make([]gin.H, 0, len(prefixes))
	for _, p := range prefixes {
		result = append(result, gin.H{
			"id":         p.ID.String(),
			"node_id":    p.NodeID.String(),
			"prefix":     p.Prefix,
			"prefix_len": p.PrefixLen,
			"interface":  p.Interface,
			"gateway":    p.Gateway,
			"status":     p.Status,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  result,
		"total": len(result),
	})
}

// PortMappingInfo 端口映射信息
type PortMappingInfo struct {
	ID            string `json:"id"`
	InstanceID    string `json:"instance_id"`
	NodeID        string `json:"node_id"`
	ContainerPort int    `json:"container_port"`
	HostPort      int    `json:"host_port"`
	Protocol      string `json:"protocol"`
	HostIP        string `json:"host_ip,omitempty"`
	Description   string `json:"description,omitempty"`
}

// ListPortMappings 获取端口映射列表
func ListPortMappings(c *gin.Context) {
	instanceID := c.Query("instance_id")
	nodeID := c.Query("node_id")

	query := db.DB.Order("created_at DESC")
	if instanceID != "" {
		query = query.Where("instance_id = ?", instanceID)
	}
	if nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}

	var mappings []models.PortMapping
	if err := query.Find(&mappings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	result := make([]PortMappingInfo, 0, len(mappings))
	for _, pm := range mappings {
		result = append(result, PortMappingInfo{
			ID:            pm.ID.String(),
			InstanceID:    pm.InstanceID.String(),
			NodeID:        pm.NodeID.String(),
			ContainerPort: pm.ContainerPort,
			HostPort:      pm.HostPort,
			Protocol:      pm.Protocol,
			HostIP:        pm.HostIP,
			Description:   pm.Description,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  result,
		"total": len(result),
	})
}

// AddPortMappingRequest 添加端口映射请求
type AddPortMappingRequest struct {
	InstanceID    string `json:"instance_id" binding:"required,uuid"`
	ContainerPort int    `json:"container_port" binding:"required,min=1,max=65535"`
	HostPort      int    `json:"host_port,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
	HostIP        string `json:"host_ip,omitempty"`
	Description   string `json:"description,omitempty"`
}

// AddPortMapping 添加端口映射
func AddPortMapping(c *gin.Context) {
	var req AddPortMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	instanceID, _ := uuid.Parse(req.InstanceID)

	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}

	// 检查端口映射配额
	var mappingCount int64
	db.DB.Model(&models.PortMapping{}).Where("instance_id = ?", instanceID).Count(&mappingCount)
	if int(mappingCount) >= instance.PortMappingLimit {
		c.JSON(http.StatusForbidden, gin.H{"error": "端口映射数量已达上限"})
		return
	}

	hostPort := req.HostPort
	if hostPort == 0 {
		// 自动分配一个随机端口 (10000-65535)
		hostPort = 10000 + int(mappingCount)*10 + 1
	}

	// 获取实例所属 VPC 的 SNAT 出口 IP 作为对外监听地址
	hostIP := req.HostIP
	if hostIP == "" && instance.VPCID != nil {
		var vpc models.VPCNetwork
		if err := db.DB.Where("id = ?", *instance.VPCID).First(&vpc).Error; err == nil && vpc.EgressV4Primary != "" {
			hostIP = vpc.EgressV4Primary
			if idx := strings.Index(hostIP, "/"); idx > 0 {
				hostIP = hostIP[:idx]
			}
		}
	}

	protocols := []string{"tcp"}
	if req.Protocol == "udp" {
		protocols = []string{"udp"}
	} else if req.Protocol == "both" {
		protocols = []string{"tcp", "udp"}
	}

	var results []models.PortMapping
	var tasks []models.Task
	userID, _ := c.Get("user_id")

	for _, protocol := range protocols {
		pm := models.PortMapping{
			ID:            uuid.New(),
			InstanceID:    instanceID,
			NodeID:        instance.NodeID,
			ContainerPort: req.ContainerPort,
			HostPort:      hostPort,
			Protocol:      protocol,
			HostIP:        hostIP,
			Description:   req.Description,
		}

		if err := db.DB.Create(&pm).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建端口映射失败: " + err.Error()})
			return
		}
		results = append(results, pm)

		// 下发网络配置任务
		payload := map[string]interface{}{
			"instance_id":    instance.IncusName,
			"action":         "add_port",
			"container_port": pm.ContainerPort,
			"host_port":      pm.HostPort,
			"protocol":       pm.Protocol,
			"host_ip":        pm.HostIP,
		}
		payloadBytes, _ := json.Marshal(payload)
		tasks = append(tasks, models.Task{
			ID:         uuid.New(),
			Type:       models.TaskTypeApplyNetwork,
			NodeID:     instance.NodeID,
			InstanceID: &instance.ID,
			UserID:     userID.(uint),
			Status:     models.TaskStatusPending,
			Payload:    payloadBytes,
		})
	}

	for _, t := range tasks {
		db.DB.Create(&t)
	}

	c.JSON(http.StatusCreated, gin.H{
		"ids":       results[0].ID.String(),
		"host_port": hostPort,
		"task_ids":  tasks[0].ID.String(),
	})
}

// DeletePortMapping 删除端口映射
func DeletePortMapping(c *gin.Context) {
	pmID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的端口映射 ID"})
		return
	}

	var pm models.PortMapping
	if err := db.DB.Where("id = ?", pmID).First(&pm).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "端口映射不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	var instance models.Instance
	db.DB.Where("id = ?", pm.InstanceID).First(&instance)

	// 下发删除任务
	payload := map[string]interface{}{
		"instance_id": instance.IncusName,
		"action":      "del_port",
		"host_port":   pm.HostPort,
		"protocol":    pm.Protocol,
		"host_ip":     pm.HostIP,
	}
	payloadBytes, _ := json.Marshal(payload)

	userID, _ := c.Get("user_id")
	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeApplyNetwork,
		NodeID:     pm.NodeID,
		InstanceID: &pm.InstanceID,
		UserID:     userID.(uint),
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}
	db.DB.Create(&task)

	// 删除数据库记录
	db.DB.Delete(&pm)

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// FirewallRuleInfo 防火墙规则信息
type FirewallRuleInfo struct {
	ID          string `json:"id"`
	InstanceID  string `json:"instance_id"`
	NodeID      string `json:"node_id"`
	Network     string `json:"network"`
	Direction   string `json:"direction"`
	Protocol    string `json:"protocol"`
	Port        string `json:"port,omitempty"`
	SourceIP    string `json:"source_ip,omitempty"`
	Action      string `json:"action"`
	Enabled     bool   `json:"enabled"`
	Priority    int    `json:"priority"`
	Description string `json:"description,omitempty"`
}

// ListFirewallRules 获取防火墙规则列表
func ListFirewallRules(c *gin.Context) {
	instanceID := c.Query("instance_id")

	query := db.DB.Order("priority ASC, created_at DESC")
	if instanceID != "" {
		query = query.Where("instance_id = ?", instanceID)
	}

	var rules []models.FirewallRule
	if err := query.Find(&rules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	result := make([]FirewallRuleInfo, 0, len(rules))
	for _, rule := range rules {
		result = append(result, FirewallRuleInfo{
			ID:          rule.ID.String(),
			InstanceID:  rule.InstanceID.String(),
			NodeID:      rule.NodeID.String(),
			Network:     rule.Network,
			Direction:   rule.Direction,
			Protocol:    rule.Protocol,
			Port:        rule.Port,
			SourceIP:    rule.SourceIP,
			Action:      rule.Action,
			Enabled:     rule.Enabled,
			Priority:    rule.Priority,
			Description: rule.Description,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  result,
		"total": len(result),
	})
}

// AddFirewallRuleRequest 添加防火墙规则请求
type AddFirewallRuleRequest struct {
	InstanceID  string `json:"instance_id" binding:"required,uuid"`
	Network     string `json:"network,omitempty"`
	Direction   string `json:"direction" binding:"required,oneof=in out"`
	Protocol    string `json:"protocol,omitempty"`
	Port        string `json:"port,omitempty"`
	SourceIP    string `json:"source_ip,omitempty"`
	Action      string `json:"action" binding:"required,oneof=ACCEPT DROP"`
	Description string `json:"description,omitempty"`
	Priority    int    `json:"priority,omitempty"`
}

// AddFirewallRule 添加防火墙规则
func AddFirewallRule(c *gin.Context) {
	var req AddFirewallRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	instanceID, _ := uuid.Parse(req.InstanceID)

	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}

	network := req.Network
	if network == "" {
		network = "ipv4"
	}
	protocol := req.Protocol
	if protocol == "" {
		protocol = "all"
	}
	priority := req.Priority
	if priority == 0 {
		priority = 100
	}

	rule := models.FirewallRule{
		ID:          uuid.New(),
		InstanceID:  instanceID,
		NodeID:      instance.NodeID,
		Network:     network,
		Direction:   req.Direction,
		Protocol:    protocol,
		Port:        req.Port,
		SourceIP:    req.SourceIP,
		Action:      req.Action,
		Description: req.Description,
		Enabled:     true,
		Priority:    priority,
	}

	if err := db.DB.Create(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建规则失败"})
		return
	}

	// 下发防火墙任务
	payload := map[string]interface{}{
		"instance_id": instance.IncusName,
		"action":      "apply",
		"rules": []map[string]interface{}{
			{
				"network":   rule.Network,
				"direction": rule.Direction,
				"protocol":  rule.Protocol,
				"port":      rule.Port,
				"source_ip": rule.SourceIP,
				"action":    rule.Action,
				"priority":  rule.Priority,
			},
		},
	}
	payloadBytes, _ := json.Marshal(payload)

	userID, _ := c.Get("user_id")
	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeApplyFirewall,
		NodeID:     instance.NodeID,
		InstanceID: &instance.ID,
		UserID:     userID.(uint),
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}
	db.DB.Create(&task)

	c.JSON(http.StatusCreated, gin.H{
		"id":      rule.ID.String(),
		"task_id": task.ID.String(),
	})
}

// UpdateFirewallRuleRequest 更新防火墙规则请求
type UpdateFirewallRuleRequest struct {
	Protocol    string `json:"protocol,omitempty"`
	Port        string `json:"port,omitempty"`
	SourceIP    string `json:"source_ip,omitempty"`
	Action      string `json:"action,omitempty"`
	Enabled     *bool  `json:"enabled,omitempty"`
	Priority    int    `json:"priority,omitempty"`
	Description string `json:"description,omitempty"`
}

// UpdateFirewallRule 更新防火墙规则
func UpdateFirewallRule(c *gin.Context) {
	ruleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的规则 ID"})
		return
	}

	var req UpdateFirewallRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	var rule models.FirewallRule
	if err := db.DB.Where("id = ?", ruleID).First(&rule).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "规则不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	updates := make(map[string]interface{})
	if req.Protocol != "" {
		updates["protocol"] = req.Protocol
	}
	if req.Port != "" {
		updates["port"] = req.Port
	}
	if req.SourceIP != "" {
		updates["source_ip"] = req.SourceIP
	}
	if req.Action != "" {
		updates["action"] = req.Action
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.Priority > 0 {
		updates["priority"] = req.Priority
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}

	if err := db.DB.Model(&rule).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// DeleteFirewallRule 删除防火墙规则
func DeleteFirewallRule(c *gin.Context) {
	ruleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的规则 ID"})
		return
	}

	var rule models.FirewallRule
	if err := db.DB.Where("id = ?", ruleID).First(&rule).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "规则不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	var instance models.Instance
	db.DB.Where("id = ?", rule.InstanceID).First(&instance)

	// 下发删除规则任务
	payload := map[string]interface{}{
		"instance_id": instance.IncusName,
		"action":      "delete",
		"rule_ids":    []string{rule.ID.String()},
	}
	payloadBytes, _ := json.Marshal(payload)

	userID, _ := c.Get("user_id")
	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeApplyFirewall,
		NodeID:     rule.NodeID,
		InstanceID: &rule.InstanceID,
		UserID:     userID.(uint),
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}
	db.DB.Create(&task)

	db.DB.Delete(&rule)

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ============= VPC 网络管理 =============

// CreateVPCRequest 创建 VPC 请求
type CreateVPCRequest struct {
	NodeID           string   `json:"node_id" binding:"required,uuid"`
	Name             string   `json:"name" binding:"required"`
	IPv4CIDR         string   `json:"ipv4_cidr" binding:"required"`
	IPv6ULACIDR      string   `json:"ipv6_ula_cidr,omitempty"`
	IPv6GUACIDR      string   `json:"ipv6_gua_cidr,omitempty"`
	DefaultGatewayV4 string   `json:"default_gateway_v4,omitempty"`
	DefaultGatewayV6 string   `json:"default_gateway_v6,omitempty"`
	EgressV4Primary  string   `json:"egress_v4_primary,omitempty"`
	EgressV4Extra    []string `json:"egress_v4_extra,omitempty"` // 独立公网 IP 地址池
	PortRangeStart   int      `json:"port_range_start,omitempty"`
	PortRangeEnd     int      `json:"port_range_end,omitempty"`
	ParentIface      string   `json:"parent_iface,omitempty"`
}

// ListVPCs 获取 VPC 列表
func ListVPCs(c *gin.Context) {
	nodeID := c.Query("node_id")

	query := db.DB.Order("created_at DESC")
	if nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}

	var vpcs []models.VPCNetwork
	if err := query.Find(&vpcs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	result := make([]gin.H, 0, len(vpcs))
	for _, v := range vpcs {
		var extraArr []string
		if v.EgressV4Extra != "" {
			_ = json.Unmarshal([]byte(v.EgressV4Extra), &extraArr)
		}
		result = append(result, gin.H{
			"id":                 v.ID.String(),
			"node_id":            v.NodeID.String(),
			"name":               v.Name,
			"ipv4_cidr":          v.IPv4CIDR,
			"ipv6_ula_cidr":      v.IPv6ULACIDR,
			"ipv6_gua_cidr":      v.IPv6GUACIDR,
			"default_gateway_v4": v.DefaultGatewayV4,
			"default_gateway_v6": v.DefaultGatewayV6,
			"egress_v4_primary":  v.EgressV4Primary,
			"egress_v4_extra":    extraArr,
			"port_range_start":   v.PortRangeStart,
			"port_range_end":     v.PortRangeEnd,
			"parent_iface":       v.ParentIface,
			"status":             v.Status,
			"bridge_name":        v.GetBridgeName(),
			"snat_enabled":       v.SNATEnabled,
			"ipv4_filter":        v.IPv4Filter,
			"ipv6_filter":        v.IPv6Filter,
			"mac_filter":         v.MACFilter,
			"created_at":         v.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  result,
		"total": len(result),
	})
}

// CreateVPC 创建 VPC
func CreateVPC(c *gin.Context) {
	var req CreateVPCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	nodeID, _ := uuid.Parse(req.NodeID)

	// 检查节点是否存在
	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
		return
	}

	// 检查 CIDR 格式
	if _, _, err := net.ParseCIDR(req.IPv4CIDR); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IPv4 CIDR 格式无效"})
		return
	}

	// 生成网关地址（如果未指定）
	gatewayV4 := req.DefaultGatewayV4
	if gatewayV4 == "" {
		// 取 CIDR 的第一个可用地址 .1
		ip, ipNet, _ := net.ParseCIDR(req.IPv4CIDR)
		ip = ip.Mask(ipNet.Mask)
		ip[len(ip)-1] = 1
		gatewayV4 = ip.String()
	}

	portRangeStart := req.PortRangeStart
	if portRangeStart <= 0 {
		portRangeStart = 10000
	}
	portRangeEnd := req.PortRangeEnd
	if portRangeEnd <= 0 {
		portRangeEnd = 65535
	}

	extraJSON := ""
	if len(req.EgressV4Extra) > 0 {
		b, _ := json.Marshal(req.EgressV4Extra)
		extraJSON = string(b)
	}

	vpc := models.VPCNetwork{
		ID:               uuid.New(),
		NodeID:           nodeID,
		Name:             req.Name,
		IPv4CIDR:         req.IPv4CIDR,
		IPv6ULACIDR:      req.IPv6ULACIDR,
		IPv6GUACIDR:      req.IPv6GUACIDR,
		DefaultGatewayV4: gatewayV4,
		DefaultGatewayV6: req.DefaultGatewayV6,
		EgressV4Primary:  req.EgressV4Primary,
		EgressV4Extra:    extraJSON,
		PortRangeStart:   portRangeStart,
		PortRangeEnd:     portRangeEnd,
		ParentIface:      req.ParentIface,
		Status:           "active",
		SNATEnabled:      true,
		IPv4Filter:       true,
		IPv6Filter:       true,
		MACFilter:        true,
	}

	if err := db.DB.Create(&vpc).Error; err != nil {
		zap.L().Error("创建 VPC 失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建 VPC 失败"})
		return
	}

	// 下发 VPC 网络配置任务到 Agent
	taskPayload := map[string]interface{}{
		"vpc_id":             vpc.ID.String(),
		"action":             "create",
		"bridge_name":        vpc.GetBridgeName(),
		"ipv4_cidr":          vpc.IPv4CIDR,
		"ipv6_ula_cidr":      vpc.IPv6ULACIDR,
		"ipv6_gua_cidr":      vpc.IPv6GUACIDR,
		"default_gateway_v4": vpc.DefaultGatewayV4,
		"default_gateway_v6": vpc.DefaultGatewayV6,
		"egress_v4_primary":  vpc.EgressV4Primary,
		"parent_iface":       vpc.ParentIface,
		"snat_enabled":       vpc.SNATEnabled,
		"ipv4_filter":        vpc.IPv4Filter,
		"mac_filter":         vpc.MACFilter,
	}
	payloadBytes, _ := json.Marshal(taskPayload)

	userID, _ := c.Get("user_id")
	task := models.Task{
		ID:      uuid.New(),
		Type:    models.TaskTypeVPCNetwork,
		NodeID:  nodeID,
		UserID:  userID.(uint),
		Status:  models.TaskStatusPending,
		Payload: payloadBytes,
	}
	if err := db.DB.Create(&task).Error; err != nil {
		zap.L().Error("创建 VPC 配置任务失败", zap.Error(err), zap.String("vpc_id", vpc.ID.String()))
	} else {
		zap.L().Info("VPC 配置任务已创建",
			zap.String("task_id", task.ID.String()),
			zap.String("vpc_id", vpc.ID.String()),
			zap.String("node_id", nodeID.String()),
			zap.String("type", string(models.TaskTypeVPCNetwork)))
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":                 vpc.ID.String(),
		"name":               vpc.Name,
		"bridge_name":        vpc.GetBridgeName(),
		"ipv4_cidr":          vpc.IPv4CIDR,
		"default_gateway_v4": vpc.DefaultGatewayV4,
		"task_id":            task.ID.String(),
	})
}

// GetVPC 获取 VPC 详情
func GetVPC(c *gin.Context) {
	vpcID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 VPC ID"})
		return
	}

	var vpc models.VPCNetwork
	if err := db.DB.Where("id = ?", vpcID).First(&vpc).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "VPC 不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	var extraArr []string
	if vpc.EgressV4Extra != "" {
		_ = json.Unmarshal([]byte(vpc.EgressV4Extra), &extraArr)
	}
	var instanceCount int64
	db.DB.Model(&models.Instance{}).Where("vpc_id = ?", vpcID).Count(&instanceCount)

	c.JSON(http.StatusOK, gin.H{
		"id":                 vpc.ID.String(),
		"node_id":            vpc.NodeID.String(),
		"name":               vpc.Name,
		"ipv4_cidr":          vpc.IPv4CIDR,
		"ipv6_ula_cidr":      vpc.IPv6ULACIDR,
		"ipv6_gua_cidr":      vpc.IPv6GUACIDR,
		"default_gateway_v4": vpc.DefaultGatewayV4,
		"default_gateway_v6": vpc.DefaultGatewayV6,
		"egress_v4_primary":  vpc.EgressV4Primary,
		"egress_v4_extra":    extraArr,
		"port_range_start":   vpc.PortRangeStart,
		"port_range_end":     vpc.PortRangeEnd,
		"parent_iface":       vpc.ParentIface,
		"status":             vpc.Status,
		"bridge_name":        vpc.GetBridgeName(),
		"snat_enabled":       vpc.SNATEnabled,
		"ipv4_filter":        vpc.IPv4Filter,
		"ipv6_filter":        vpc.IPv6Filter,
		"mac_filter":         vpc.MACFilter,
		"instance_count":     instanceCount,
	})
}

// UpdateVPC 更新 VPC
func UpdateVPC(c *gin.Context) {
	vpcID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 VPC ID"})
		return
	}

	var vpc models.VPCNetwork
	if err := db.DB.Where("id = ?", vpcID).First(&vpc).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "VPC 不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	var req struct {
		Name             string   `json:"name,omitempty"`
		IPv4CIDR         string   `json:"ipv4_cidr,omitempty"`
		IPv6ULACIDR      string   `json:"ipv6_ula_cidr,omitempty"`
		IPv6GUACIDR      string   `json:"ipv6_gua_cidr,omitempty"`
		DefaultGatewayV4 string   `json:"default_gateway_v4,omitempty"`
		DefaultGatewayV6 string   `json:"default_gateway_v6,omitempty"`
		EgressV4Primary  string   `json:"egress_v4_primary,omitempty"`
		EgressV4Extra    []string `json:"egress_v4_extra,omitempty"`
		PortRangeStart   int      `json:"port_range_start,omitempty"`
		PortRangeEnd     int      `json:"port_range_end,omitempty"`
		ParentIface      string   `json:"parent_iface,omitempty"`
		Status           string   `json:"status,omitempty"`
		SNATEnabled      *bool    `json:"snat_enabled,omitempty"`
		IPv4Filter       *bool    `json:"ipv4_filter,omitempty"`
		MACFilter        *bool    `json:"mac_filter,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	// 检查 VPC 下是否有实例
	var instanceCount int64
	db.DB.Model(&models.Instance{}).Where("vpc_id = ?", vpcID).Count(&instanceCount)
	hasInstances := instanceCount > 0

	updates := map[string]interface{}{}

	// 名称始终可编辑
	if req.Name != "" {
		updates["name"] = req.Name
	}

	// CIDR 和网关：有实例时不可编辑（只有值真正改变时才拒绝）
	if req.IPv4CIDR != "" && req.IPv4CIDR != vpc.IPv4CIDR {
		if hasInstances {
			c.JSON(http.StatusConflict, gin.H{"error": "VPC 下存在实例，IPv4 CIDR 不可修改"})
			return
		}
		if _, _, err := net.ParseCIDR(req.IPv4CIDR); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "IPv4 CIDR 格式无效"})
			return
		}
		updates["ipv4_cidr"] = req.IPv4CIDR
	}
	if req.IPv6ULACIDR != "" && req.IPv6ULACIDR != vpc.IPv6ULACIDR {
		if hasInstances {
			c.JSON(http.StatusConflict, gin.H{"error": "VPC 下存在实例，IPv6 ULA CIDR 不可修改"})
			return
		}
		updates["ipv6_ula_cidr"] = req.IPv6ULACIDR
	}
	if req.IPv6GUACIDR != "" && req.IPv6GUACIDR != vpc.IPv6GUACIDR {
		if hasInstances {
			c.JSON(http.StatusConflict, gin.H{"error": "VPC 下存在实例，IPv6 GUA CIDR 不可修改"})
			return
		}
		updates["ipv6_gua_cidr"] = req.IPv6GUACIDR
	}
	if req.DefaultGatewayV4 != "" && req.DefaultGatewayV4 != vpc.DefaultGatewayV4 {
		if hasInstances {
			c.JSON(http.StatusConflict, gin.H{"error": "VPC 下存在实例，IPv4 网关不可修改"})
			return
		}
		updates["default_gateway_v4"] = req.DefaultGatewayV4
	}
	if req.DefaultGatewayV6 != "" && req.DefaultGatewayV6 != vpc.DefaultGatewayV6 {
		if hasInstances {
			c.JSON(http.StatusConflict, gin.H{"error": "VPC 下存在实例，IPv6 网关不可修改"})
			return
		}
		updates["default_gateway_v6"] = req.DefaultGatewayV6
	}

	// 以下字段始终可编辑
	if req.EgressV4Primary != "" {
		updates["egress_v4_primary"] = req.EgressV4Primary
	}
	if len(req.EgressV4Extra) > 0 {
		b, _ := json.Marshal(req.EgressV4Extra)
		updates["egress_v4_extra"] = string(b)
	}
	if req.PortRangeStart > 0 {
		updates["port_range_start"] = req.PortRangeStart
	}
	if req.PortRangeEnd > 0 {
		updates["port_range_end"] = req.PortRangeEnd
	}
	if req.ParentIface != "" {
		updates["parent_iface"] = req.ParentIface
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.SNATEnabled != nil {
		updates["snat_enabled"] = *req.SNATEnabled
	}
	if req.IPv4Filter != nil {
		updates["ipv4_filter"] = *req.IPv4Filter
	}
	if req.MACFilter != nil {
		updates["mac_filter"] = *req.MACFilter
	}

	if len(updates) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "无需更新"})
		return
	}

	if err := db.DB.Model(&vpc).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	// 下发更新任务到 Agent
	taskPayload := map[string]interface{}{
		"vpc_id":             vpc.ID.String(),
		"action":             "update",
		"bridge_name":        vpc.GetBridgeName(),
		"ipv4_cidr":          vpc.IPv4CIDR,
		"parent_iface":       vpc.ParentIface,
		"egress_v4_primary":  req.EgressV4Primary,
		"snat_enabled":       req.SNATEnabled,
		"default_gateway_v4": req.DefaultGatewayV4,
		"default_gateway_v6": req.DefaultGatewayV6,
	}
	payloadBytes, _ := json.Marshal(taskPayload)

	userID, _ := c.Get("user_id")
	task := models.Task{
		ID:      uuid.New(),
		Type:    models.TaskTypeVPCNetwork,
		NodeID:  vpc.NodeID,
		UserID:  userID.(uint),
		Status:  models.TaskStatusPending,
		Payload: payloadBytes,
	}
	db.DB.Create(&task)

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// DeleteVPC 删除 VPC
func DeleteVPC(c *gin.Context) {
	vpcID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 VPC ID"})
		return
	}

	var vpc models.VPCNetwork
	if err := db.DB.Where("id = ?", vpcID).First(&vpc).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "VPC 不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	// 检查是否有实例在使用该 VPC
	var count int64
	db.DB.Model(&models.Instance{}).Where("vpc_id = ?", vpcID).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "该 VPC 下存在实例，无法删除"})
		return
	}

	// 下发删除任务到 Agent
	taskPayload := map[string]interface{}{
		"vpc_id":      vpc.ID.String(),
		"action":      "delete",
		"bridge_name": vpc.GetBridgeName(),
		"ipv4_cidr":   vpc.IPv4CIDR,
	}
	payloadBytes, _ := json.Marshal(taskPayload)

	userID, _ := c.Get("user_id")
	task := models.Task{
		ID:      uuid.New(),
		Type:    models.TaskTypeVPCNetwork,
		NodeID:  vpc.NodeID,
		UserID:  userID.(uint),
		Status:  models.TaskStatusPending,
		Payload: payloadBytes,
	}
	db.DB.Create(&task)

	db.DB.Delete(&vpc)

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}
