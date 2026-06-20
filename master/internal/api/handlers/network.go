package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"tsukiyo/master/internal/service"
	"tsukiyo/master/internal/service/infrastructure"
)

var networkService *infrastructure.NetworkService

// InitNetworkService 初始化网络服务
func InitNetworkService(svc *infrastructure.NetworkService) {
	networkService = svc
}

// =====================================================================
// Bridge CRUD
// =====================================================================

func ListBridges(c *gin.Context) {
	nodeID := c.Query("node_id")
	bridges, err := networkService.ListBridges(nodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询网桥失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": bridges})
}

func GetBridge(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的网桥 ID"})
		return
	}
	bridge, err := networkService.GetBridge(id)
	if err != nil {
		if err == service.ErrBridgeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "网桥不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, bridge)
}

type createBridgeRequest struct {
	NodeID            string   `json:"node_id" binding:"required"`
	Name              string   `json:"name" binding:"required"`
	IPv4Enabled       bool     `json:"ipv4_enabled"`
	IPv4CIDR          string   `json:"ipv4_cidr"`
	IPv4Gateway       string   `json:"ipv4_gateway"`
	IPv6Enabled       bool     `json:"ipv6_enabled"`
	IPv6CIDR          string   `json:"ipv6_cidr"`
	IPv6Gateway       string   `json:"ipv6_gateway"`
	DNSServers        []string `json:"dns_servers"`
	PortRangeStart    int      `json:"port_range_start"`
	PortRangeEnd      int      `json:"port_range_end"`
	NATEgressV4PoolID string   `json:"nat_egress_v4_pool_id"`
	NATEgressV6PoolID string   `json:"nat_egress_v6_pool_id"`
}

func CreateBridge(c *gin.Context) {
	var req createBridgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	nodeID, err := uuid.Parse(req.NodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}
	userID, _ := c.Get("user_id")

	var natEgressV4PoolID *uuid.UUID
	if req.NATEgressV4PoolID != "" {
		pid, err := uuid.Parse(req.NATEgressV4PoolID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 IPv4 NAT 出口 EIP 池 ID"})
			return
		}
		natEgressV4PoolID = &pid
	}

	var natEgressV6PoolID *uuid.UUID
	if req.NATEgressV6PoolID != "" {
		pid, err := uuid.Parse(req.NATEgressV6PoolID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 IPv6 NAT 出口 EIP 池 ID"})
			return
		}
		natEgressV6PoolID = &pid
	}

	bridge, task, err := networkService.CreateBridge(infrastructure.CreateBridgeRequest{
		NodeID:            nodeID,
		Name:              req.Name,
		IPv4Enabled:       req.IPv4Enabled,
		IPv4CIDR:          req.IPv4CIDR,
		IPv4Gateway:       req.IPv4Gateway,
		IPv6Enabled:       req.IPv6Enabled,
		IPv6CIDR:          req.IPv6CIDR,
		IPv6Gateway:       req.IPv6Gateway,
		DNSServers:        req.DNSServers,
		PortRangeStart:    req.PortRangeStart,
		PortRangeEnd:      req.PortRangeEnd,
		NATEgressV4PoolID: natEgressV4PoolID,
		NATEgressV6PoolID: natEgressV6PoolID,
		UserID:            userID.(uint),
	})
	if err != nil {
		if err == service.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
			return
		}
		if err == service.ErrInvalidCIDR {
			c.JSON(http.StatusBadRequest, gin.H{"error": "CIDR 格式无效"})
			return
		}
		if err == service.ErrNodeNotConnected {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "节点未在线"})
			return
		}
		if err == service.ErrBridgeCIDROverlap {
			c.JSON(http.StatusConflict, gin.H{"error": "网桥 CIDR 与同节点其他网桥重叠"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp := gin.H{"bridge": bridge}
	if task != nil {
		resp["task_id"] = task.ID.String()
	}
	c.JSON(http.StatusCreated, resp)
}

type updateBridgeRequest struct {
	Name           *string   `json:"name"`
	IPv4Enabled    *bool     `json:"ipv4_enabled"`
	IPv4CIDR       *string   `json:"ipv4_cidr"`
	IPv4Gateway    *string   `json:"ipv4_gateway"`
	IPv6Enabled    *bool     `json:"ipv6_enabled"`
	IPv6CIDR       *string   `json:"ipv6_cidr"`
	IPv6Gateway    *string   `json:"ipv6_gateway"`
	DNSServers     *[]string `json:"dns_servers"`
	PortRangeStart *int      `json:"port_range_start"`
	PortRangeEnd   *int      `json:"port_range_end"`
	Status         *string   `json:"status"`
}

func UpdateBridge(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的网桥 ID"})
		return
	}
	var req updateBridgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	userID, _ := c.Get("user_id")
	err = networkService.UpdateBridge(id, infrastructure.UpdateBridgeRequest{
		Name:           req.Name,
		IPv4Enabled:    req.IPv4Enabled,
		IPv4CIDR:       req.IPv4CIDR,
		IPv4Gateway:    req.IPv4Gateway,
		IPv6Enabled:    req.IPv6Enabled,
		IPv6CIDR:       req.IPv6CIDR,
		IPv6Gateway:    req.IPv6Gateway,
		DNSServers:     req.DNSServers,
		PortRangeStart: req.PortRangeStart,
		PortRangeEnd:   req.PortRangeEnd,
		Status:         req.Status,
		UserID:         userID.(uint),
	})
	if err != nil {
		if err == service.ErrBridgeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "网桥不存在"})
			return
		}
		if err == service.ErrBridgeHasInstances {
			c.JSON(http.StatusConflict, gin.H{"error": "网桥下存在实例，无法修改"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteBridge(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的网桥 ID"})
		return
	}
	userID, _ := c.Get("user_id")
	err = networkService.DeleteBridge(id, userID.(uint))
	if err != nil {
		if err == service.ErrBridgeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "网桥不存在"})
			return
		}
		if err == service.ErrBridgeHasInstances {
			c.JSON(http.StatusConflict, gin.H{"error": "网桥下存在实例，无法删除"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// =====================================================================
// Bridge NAT 出口 IP 绑定
// =====================================================================

type bindEgressRequest struct {
	PoolID    string `json:"pool_id" binding:"required"`
	IPVersion string `json:"ip_version" binding:"required,oneof=ipv4 ipv6"`
}

func BindBridgeEgress(c *gin.Context) {
	bridgeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的网桥 ID"})
		return
	}
	var req bindEgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	poolID, err := uuid.Parse(req.PoolID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 EIP 池 ID"})
		return
	}
	userID, _ := c.Get("user_id")
	if err := networkService.BindBridgeEgress(bridgeID, poolID, req.IPVersion, userID.(uint)); err != nil {
		if err == service.ErrBridgeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "网桥不存在"})
			return
		}
		if err == service.ErrEIPPoolNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "EIP 资源池不存在"})
			return
		}
		if err == service.ErrNoAvailableEIP {
			c.JSON(http.StatusConflict, gin.H{"error": "资源池中无可用 EIP"})
			return
		}
		if err == service.ErrEIPNotAvailable {
			c.JSON(http.StatusBadRequest, gin.H{"error": "EIP 不可用"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "绑定失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "绑定成功"})
}

type unbindEgressRequest struct {
	IPVersion string `json:"ip_version" binding:"required,oneof=ipv4 ipv6"`
}

func UnbindBridgeEgress(c *gin.Context) {
	bridgeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的网桥 ID"})
		return
	}
	var req unbindEgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	userID, _ := c.Get("user_id")
	if err := networkService.UnbindBridgeEgress(bridgeID, req.IPVersion, userID.(uint)); err != nil {
		if err == service.ErrBridgeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "网桥不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解绑失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "解绑成功"})
}

// =====================================================================
// EIP 资源池
// =====================================================================

func ListEIPPools(c *gin.Context) {
	nodeID := c.Query("node_id")
	pools, err := networkService.ListEIPPools(nodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": pools})
}

type createEIPPoolRequest struct {
	NodeID    string `json:"node_id" binding:"required"`
	IPVersion string `json:"ip_version" binding:"required,oneof=ipv4 ipv6"`
	CIDR      string `json:"cidr" binding:"required"`
	Interface string `json:"interface"`
	Gateway   string `json:"gateway"`
	Alias     string `json:"alias"`
	PoolType  string `json:"pool_type"`
}

func CreateEIPPool(c *gin.Context) {
	var req createEIPPoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	nodeID, err := uuid.Parse(req.NodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}
	userID, _ := c.Get("user_id")
	pool, err := networkService.CreateEIPPool(infrastructure.CreateEIPPoolRequest{
		NodeID:    nodeID,
		IPVersion: req.IPVersion,
		CIDR:      req.CIDR,
		Interface: req.Interface,
		Gateway:   req.Gateway,
		Alias:     req.Alias,
		PoolType:  req.PoolType,
		UserID:    userID.(uint),
	})
	if err != nil {
		if err == service.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
			return
		}
		if err == service.ErrInvalidCIDR {
			c.JSON(http.StatusBadRequest, gin.H{"error": "CIDR 格式无效"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}
	c.JSON(http.StatusCreated, pool)
}

func DeleteEIPPool(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的资源池 ID"})
		return
	}
	if err := networkService.DeleteEIPPool(id); err != nil {
		if err == service.ErrEIPPoolHasAllocations {
			c.JSON(http.StatusConflict, gin.H{"error": "资源池中存在已分配的 EIP，无法删除"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// =====================================================================
// EIP 分配
// =====================================================================

func ListEIPAllocations(c *gin.Context) {
	nodeID := c.Query("node_id")
	instanceID := c.Query("instance_id")
	allocs, err := networkService.ListEIPAllocations(nodeID, instanceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": allocs})
}

type allocateEIPRequest struct {
	NodeID     string `json:"node_id" binding:"required"`
	IPVersion  string `json:"ip_version" binding:"required,oneof=ipv4 ipv6"`
	PrefixLen  int    `json:"prefix_len"`
	SpecificIP string `json:"specific_ip"`
}

func AllocateEIP(c *gin.Context) {
	var req allocateEIPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	nodeID, err := uuid.Parse(req.NodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}
	alloc, err := networkService.AllocateEIP(nodeID, req.IPVersion, req.PrefixLen, req.SpecificIP)
	if err != nil {
		if err == service.ErrNoAvailableEIP {
			c.JSON(http.StatusConflict, gin.H{"error": "资源池中无可用 EIP"})
			return
		}
		if err == service.ErrEIPAlreadyAssigned {
			c.JSON(http.StatusConflict, gin.H{"error": "EIP 已分配"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "分配失败"})
		return
	}
	c.JSON(http.StatusCreated, alloc)
}

type assignEIPRequest struct {
	InstanceID string `json:"instance_id" binding:"required"`
}

func AssignEIPToInstance(c *gin.Context) {
	allocID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 EIP 分配 ID"})
		return
	}
	var req assignEIPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	instanceID, err := uuid.Parse(req.InstanceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}
	if err := networkService.AssignEIPToInstance(allocID, instanceID); err != nil {
		if err == service.ErrEIPAllocationNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "EIP 分配记录不存在"})
			return
		}
		if err == service.ErrEIPNotAvailable {
			c.JSON(http.StatusBadRequest, gin.H{"error": "EIP 不可用"})
			return
		}
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		if err == service.ErrInstanceNoBridge {
			c.JSON(http.StatusBadRequest, gin.H{"error": "实例未关联网桥"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "分配失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "分配成功"})
}

func ReleaseEIP(c *gin.Context) {
	allocID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 EIP 分配 ID"})
		return
	}
	if err := networkService.ReleaseInstanceEIP(allocID); err != nil {
		if err == service.ErrEIPAllocationNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "EIP 分配记录不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "释放失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "释放成功"})
}

// =====================================================================
// 端口映射
// =====================================================================

func ListPortMappings(c *gin.Context) {
	instanceID := c.Query("instance_id")
	mappings, err := networkService.ListPortMappings(instanceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": mappings})
}

type addPortMappingRequest struct {
	InstanceID    string `json:"instance_id" binding:"required"`
	ContainerPort int    `json:"container_port" binding:"required,min=1,max=65535"`
	Protocol      string `json:"protocol" binding:"required,oneof=tcp udp"`
	IPVersion     string `json:"ip_version" binding:"required,oneof=ipv4 ipv6"`
	Description   string `json:"description"`
}

func AddPortMapping(c *gin.Context) {
	var req addPortMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	instanceID, err := uuid.Parse(req.InstanceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}
	pm, err := networkService.AddPortMapping(instanceID, req.ContainerPort, req.Protocol, req.IPVersion, req.Description)
	if err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		if err == service.ErrInstanceNoBridge {
			c.JSON(http.StatusBadRequest, gin.H{"error": "实例未关联网桥"})
			return
		}
		if err == service.ErrNoBridgeEgressIP {
			c.JSON(http.StatusBadRequest, gin.H{"error": "网桥未配置 NAT 出口 IP"})
			return
		}
		if err == service.ErrNoAvailablePorts {
			c.JSON(http.StatusConflict, gin.H{"error": "网桥端口范围内无可用端口"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "添加失败"})
		return
	}
	c.JSON(http.StatusCreated, pm)
}

func DeletePortMapping(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的端口映射 ID"})
		return
	}
	if err := networkService.DeletePortMapping(id); err != nil {
		if err == service.ErrPortMappingNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "端口映射不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// =====================================================================
// 防火墙规则
// =====================================================================

func ListFirewallRules(c *gin.Context) {
	instanceID := c.Query("instance_id")
	rules, err := networkService.ListFirewallRules(instanceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rules})
}

type addFirewallRuleRequest struct {
	InstanceID  string `json:"instance_id" binding:"required"`
	Network     string `json:"network" binding:"required,oneof=ipv4 ipv6"`
	Direction   string `json:"direction" binding:"required,oneof=inbound outbound"`
	Protocol    string `json:"protocol"`
	Port        string `json:"port"`
	SourceIP    string `json:"source_ip"`
	Action      string `json:"action" binding:"required,oneof=allow drop"`
	Description string `json:"description"`
	Priority    int    `json:"priority"`
}

func AddFirewallRule(c *gin.Context) {
	var req addFirewallRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	instanceID, err := uuid.Parse(req.InstanceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的实例 ID"})
		return
	}
	rule, err := networkService.AddFirewallRule(instanceID, req.Network, req.Direction, req.Protocol, req.Port, req.SourceIP, req.Action, req.Description, req.Priority)
	if err != nil {
		if err == service.ErrInstanceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "添加失败"})
		return
	}
	c.JSON(http.StatusCreated, rule)
}

func UpdateFirewallRule(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "暂不支持"})
}

func DeleteFirewallRule(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的规则 ID"})
		return
	}
	if err := networkService.DeleteFirewallRule(id); err != nil {
		if err == service.ErrFirewallRuleNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "规则不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// =====================================================================
//  工具方法
// =====================================================================
