package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/service"
	"tsukiyo/master/internal/service/infrastructure"
)

var networkService *infrastructure.NetworkService

// InitNetworkService 初始化网络服务
func InitNetworkService(svc *infrastructure.NetworkService) {
	networkService = svc
}

// IPPoolInfo IP 池信息
type IPPoolInfo = infrastructure.IPPoolInfo

// ListIPPools 获取 IP 池列表
func ListIPPools(c *gin.Context) {
	nodeID := c.Query("node_id")
	status := c.Query("status")

	result, err := networkService.ListIPPools(nodeID, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
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

	ip, err := networkService.AddIPPool(nodeID, req.Address, req.Gateway, req.PrefixLen, req.Interface)
	if err != nil {
		if err == service.ErrIPAlreadyExists {
			c.JSON(http.StatusConflict, gin.H{"error": "该 IP 已在池中"})
			return
		}
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

	if err := networkService.DeleteIPPool(ipID); err != nil {
		if err == service.ErrIPNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "IP 不存在"})
			return
		}
		if err == service.ErrIPAssigned {
			c.JSON(http.StatusConflict, gin.H{"error": "该 IP 已被分配，无法删除"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ListIPv6Prefixes 获取 IPv6 前缀列表
func ListIPv6Prefixes(c *gin.Context) {
	nodeID := c.Query("node_id")

	prefixes, err := networkService.ListIPv6Prefixes(nodeID)
	if err != nil {
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
type PortMappingInfo = infrastructure.PortMappingInfo

// ListPortMappings 获取端口映射列表
func ListPortMappings(c *gin.Context) {
	instanceID := c.Query("instance_id")
	nodeID := c.Query("node_id")

	result, err := networkService.ListPortMappings(instanceID, nodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
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
	userID, _ := c.Get("user_id")

	results, tasks, err := networkService.AddPortMapping(instanceID, req.ContainerPort, req.HostPort, req.Protocol, req.HostIP, req.Description, userID.(uint))
	if err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		if err == service.ErrPortMappingLimitReached {
			c.JSON(http.StatusForbidden, gin.H{"error": "端口映射数量已达上限"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建端口映射失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"ids":       results[0].ID.String(),
		"host_port": results[0].HostPort,
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

	userID, _ := c.Get("user_id")
	task, err := networkService.DeletePortMapping(pmID, userID.(uint))
	if err != nil {
		if err == service.ErrPortMappingNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "端口映射不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功", "task_id": task.ID.String()})
}

// FirewallRuleInfo 防火墙规则信息
type FirewallRuleInfo = infrastructure.FirewallRuleInfo

// ListFirewallRules 获取防火墙规则列表
func ListFirewallRules(c *gin.Context) {
	instanceID := c.Query("instance_id")

	result, err := networkService.ListFirewallRules(instanceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
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
	userID, _ := c.Get("user_id")

	rule, task, err := networkService.AddFirewallRule(instanceID, req.Network, req.Direction, req.Protocol, req.Port, req.SourceIP, req.Action, req.Description, req.Priority, userID.(uint))
	if err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建规则失败"})
		return
	}

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

	if err := networkService.UpdateFirewallRule(ruleID, req.Protocol, req.Port, req.SourceIP, req.Action, req.Enabled, req.Priority, req.Description); err != nil {
		if err == service.ErrFirewallRuleNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "规则不存在"})
			return
		}
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

	userID, _ := c.Get("user_id")
	task, err := networkService.DeleteFirewallRule(ruleID, userID.(uint))
	if err != nil {
		if err == service.ErrFirewallRuleNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "规则不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功", "task_id": task.ID.String()})
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

	vpcs, err := networkService.ListVPCs(nodeID)
	if err != nil {
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
	userID, _ := c.Get("user_id")

	vpc, task, err := networkService.CreateVPC(nodeID, req.Name, req.IPv4CIDR, req.IPv6ULACIDR, req.IPv6GUACIDR, req.DefaultGatewayV4, req.DefaultGatewayV6, req.EgressV4Primary, req.EgressV4Extra, req.PortRangeStart, req.PortRangeEnd, req.ParentIface, userID.(uint))
	if err != nil {
		if err == service.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
			return
		}
		if err == service.ErrInvalidCIDR {
			c.JSON(http.StatusBadRequest, gin.H{"error": "IPv4 CIDR 格式无效"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建 VPC 失败"})
		return
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

	vpc, err := networkService.GetVPC(vpcID)
	if err != nil {
		if err == service.ErrVPCNotFound {
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

	userID, _ := c.Get("user_id")
	task, err := networkService.UpdateVPC(vpcID, req.Name, req.IPv4CIDR, req.IPv6ULACIDR, req.IPv6GUACIDR, req.DefaultGatewayV4, req.DefaultGatewayV6, req.EgressV4Primary, req.EgressV4Extra, req.PortRangeStart, req.PortRangeEnd, req.ParentIface, req.Status, req.SNATEnabled, req.IPv4Filter, req.MACFilter, userID.(uint))
	if err != nil {
		if err == service.ErrVPCNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "VPC 不存在"})
			return
		}
		if err == service.ErrVPCHasInstances {
			c.JSON(http.StatusConflict, gin.H{"error": "VPC 下存在实例，无法修改"})
			return
		}
		if err == service.ErrInvalidCIDR {
			c.JSON(http.StatusBadRequest, gin.H{"error": "IPv4 CIDR 格式无效"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	if task == nil {
		c.JSON(http.StatusOK, gin.H{"message": "无需更新"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功", "task_id": task.ID.String()})
}

// DeleteVPC 删除 VPC
func DeleteVPC(c *gin.Context) {
	vpcID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 VPC ID"})
		return
	}

	userID, _ := c.Get("user_id")
	task, err := networkService.DeleteVPC(vpcID, userID.(uint))
	if err != nil {
		if err == service.ErrVPCNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "VPC 不存在"})
			return
		}
		if err == service.ErrVPCHasInstances {
			c.JSON(http.StatusConflict, gin.H{"error": "该 VPC 下存在实例，无法删除"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功", "task_id": task.ID.String()})
}
