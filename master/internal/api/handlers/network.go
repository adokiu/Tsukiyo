package handlers

import (
	"net/http"
	"strconv"

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

// =====================================================================
// Bridge CRUD
// =====================================================================

func ListBridges(c *gin.Context) {
	nodeID := c.Query("node_id")
	q := ParseListQuery(c)
	bridges, total, err := networkService.ListBridges(nodeID, q.Search, q.Filters, q.Page, q.PerPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询网桥失败"})
		return
	}
	c.JSON(http.StatusOK, ListResponse{
		Data:    bridges,
		Total:   total,
		Page:    q.Page,
		PerPage: q.PerPage,
	})
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
	NodeID                string   `json:"node_id" binding:"required"`
	Name                  string   `json:"name" binding:"required"`
	IPv4Enabled           bool     `json:"ipv4_enabled"`
	IPv4CIDR              string   `json:"ipv4_cidr"`
	IPv4Gateway           string   `json:"ipv4_gateway"`
	IPv6Enabled           bool     `json:"ipv6_enabled"`
	IPv6EIPPoolID         string   `json:"ipv6_eip_pool_id"`
	IPv6PrefixLen         int      `json:"ipv6_prefix_len"`
	IPv6SpecificIP        string   `json:"ipv6_specific_ip"`
	DNSServers            []string `json:"dns_servers"`
	PortRangeStart        int      `json:"port_range_start"`
	PortRangeEnd          int      `json:"port_range_end"`
	NATEgressV4PoolID     string   `json:"nat_egress_v4_pool_id"`
	NATEgressV4SpecificIP string   `json:"nat_egress_v4_specific_ip"`
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

	var ipv6EIPPoolID *uuid.UUID
	if req.IPv6EIPPoolID != "" {
		pid, err := uuid.Parse(req.IPv6EIPPoolID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 IPv6 EIP 池 ID"})
			return
		}
		ipv6EIPPoolID = &pid
	}

	bridge, task, err := networkService.CreateBridge(infrastructure.CreateBridgeRequest{
		NodeID:                nodeID,
		Name:                  req.Name,
		IPv4Enabled:           req.IPv4Enabled,
		IPv4CIDR:              req.IPv4CIDR,
		IPv4Gateway:           req.IPv4Gateway,
		IPv6Enabled:           req.IPv6Enabled,
		IPv6EIPPoolID:         ipv6EIPPoolID,
		IPv6PrefixLen:         req.IPv6PrefixLen,
		IPv6SpecificIP:        req.IPv6SpecificIP,
		DNSServers:            req.DNSServers,
		PortRangeStart:        req.PortRangeStart,
		PortRangeEnd:          req.PortRangeEnd,
		NATEgressV4PoolID:     natEgressV4PoolID,
		NATEgressV4SpecificIP: req.NATEgressV4SpecificIP,
		UserID:                userID.(uint),
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
	BroadcastDataRefresh("bridges", req.NodeID)
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
	BroadcastDataRefresh("bridges", "")
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
	BroadcastDataRefresh("bridges", "")
}

// =====================================================================
// Bridge NAT 出口 IP 绑定
// =====================================================================

type bindEgressRequest struct {
	PoolID     string `json:"pool_id" binding:"required"`
	IPVersion  string `json:"ip_version" binding:"required,oneof=ipv4 ipv6"`
	SpecificIP string `json:"specific_ip"`
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
	if err := networkService.BindBridgeEgress(bridgeID, poolID, req.IPVersion, req.SpecificIP, userID.(uint)); err != nil {
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
	BroadcastDataRefresh("bridges", "")
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
	BroadcastDataRefresh("bridges", "")
}

// =====================================================================
// EIP 资源池
// =====================================================================

func ListEIPPools(c *gin.Context) {
	nodeID := c.Query("node_id")
	q := ParseListQuery(c)
	pools, total, err := networkService.ListEIPPools(nodeID, q.Search, q.Filters, q.Page, q.PerPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, ListResponse{
		Data:    pools,
		Total:   total,
		Page:    q.Page,
		PerPage: q.PerPage,
	})
}

// CountAvailableEIP 查询节点可用 EIP 数量
func CountAvailableEIP(c *gin.Context) {
	nodeIDStr := c.Query("node_id")
	if nodeIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 node_id 参数"})
		return
	}
	nodeID, err := uuid.Parse(nodeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 node_id"})
		return
	}
	ipVersion := c.DefaultQuery("ip_version", "ipv4")
	prefixLen := 0
	if plStr := c.Query("prefix_len"); plStr != "" {
		if pl, err := strconv.Atoi(plStr); err == nil {
			prefixLen = pl
		}
	}
	count, err := networkService.CountAvailableEIP(nodeID, ipVersion, prefixLen)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"available": count})
}

// ListAvailableEIPs 列出指定池中可用的 EIP 地址
func ListAvailableEIPs(c *gin.Context) {
	poolIDStr := c.Query("pool_id")
	if poolIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 pool_id 参数"})
		return
	}
	poolID, err := uuid.Parse(poolIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 pool_id"})
		return
	}
	prefixLen := 0
	if plStr := c.Query("prefix_len"); plStr != "" {
		if pl, err := strconv.Atoi(plStr); err == nil {
			prefixLen = pl
		}
	}
	maxCount := 10
	if mcStr := c.Query("max_count"); mcStr != "" {
		if mc, err := strconv.Atoi(mcStr); err == nil && mc > 0 && mc <= 100 {
			maxCount = mc
		}
	}
	addresses, err := networkService.ListAvailableEIPsFromPool(poolID, prefixLen, maxCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"addresses": addresses})
}

// ListAvailableIPv6FromBridge 列出 bridge IPv6 CIDR中可用的子段
func ListAvailableIPv6FromBridge(c *gin.Context) {
	bridgeIDStr := c.Query("bridge_id")
	if bridgeIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 bridge_id 参数"})
		return
	}
	bridgeID, err := uuid.Parse(bridgeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 bridge_id"})
		return
	}
	prefixLen := 128
	if plStr := c.Query("prefix_len"); plStr != "" {
		if pl, err := strconv.Atoi(plStr); err == nil {
			prefixLen = pl
		}
	}
	maxCount := 10
	if mcStr := c.Query("max_count"); mcStr != "" {
		if mc, err := strconv.Atoi(mcStr); err == nil && mc > 0 && mc <= 100 {
			maxCount = mc
		}
	}
	addresses, err := networkService.ListAvailableIPv6FromBridge(bridgeID, prefixLen, maxCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"addresses": addresses})
}

type createEIPPoolRequest struct {
	NodeID        string `json:"node_id" binding:"required"`
	IPVersion     string `json:"ip_version" binding:"required,oneof=ipv4 ipv6"`
	CIDR          string `json:"cidr" binding:"required"`
	Interface     string `json:"interface"`
	Gateway       string `json:"gateway"`
	Alias         string `json:"alias"`
	PoolType      string `json:"pool_type"`
	NetmaskPrefix int    `json:"netmask_prefix"`
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
		NodeID:        nodeID,
		IPVersion:     req.IPVersion,
		CIDR:          req.CIDR,
		Interface:     req.Interface,
		Gateway:       req.Gateway,
		Alias:         req.Alias,
		PoolType:      req.PoolType,
		NetmaskPrefix: req.NetmaskPrefix,
		UserID:        userID.(uint),
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
		if err == service.ErrEIPPoolCIDROverlap {
			c.JSON(http.StatusConflict, gin.H{"error": "CIDR 网段与已有资源池重叠"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, pool)
	BroadcastDataRefresh("eip_pools", req.NodeID)
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
	BroadcastDataRefresh("eip_pools", "")
}

type updateEIPPoolRequest struct {
	Interface     string `json:"interface"`
	Gateway       string `json:"gateway"`
	Alias         string `json:"alias"`
	NetmaskPrefix int    `json:"netmask_prefix"`
	PoolType      string `json:"pool_type"`
	Status        string `json:"status"`
}

func UpdateEIPPool(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的资源池 ID"})
		return
	}
	var req updateEIPPoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	pool, err := networkService.UpdateEIPPool(id, infrastructure.UpdateEIPPoolRequest{
		Interface:     req.Interface,
		Gateway:       req.Gateway,
		Alias:         req.Alias,
		NetmaskPrefix: req.NetmaskPrefix,
		PoolType:      req.PoolType,
		Status:        req.Status,
	})
	if err != nil {
		if err == service.ErrEIPPoolNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "资源池不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, pool)
	BroadcastDataRefresh("eip_pools", "")
}

// =====================================================================
// EIP 分配
// =====================================================================

func ListEIPAllocations(c *gin.Context) {
	nodeID := c.Query("node_id")
	instanceID := c.Query("instance_id")
	q := ParseListQuery(c)
	allocs, total, err := networkService.ListEIPAllocations(nodeID, instanceID, q.Search, q.Filters, q.Page, q.PerPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, ListResponse{
		Data:    allocs,
		Total:   total,
		Page:    q.Page,
		PerPage: q.PerPage,
	})
}

type allocateEIPRequest struct {
	NodeID     string `json:"node_id" binding:"required"`
	IPVersion  string `json:"ip_version" binding:"required,oneof=ipv4 ipv6"`
	PrefixLen  int    `json:"prefix_len"`
	SpecificIP string `json:"specific_ip"`
	BridgeID   string `json:"bridge_id,omitempty"`
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

	var alloc *models.EIPAllocation
	if req.IPVersion == "ipv6" && req.BridgeID != "" {
		bridgeID, err := uuid.Parse(req.BridgeID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的网桥 ID"})
			return
		}
		alloc, err = networkService.AllocateIPv6FromBridge(bridgeID, req.PrefixLen, req.SpecificIP)
		if err != nil {
			if err == service.ErrNoAvailableEIP {
				c.JSON(http.StatusConflict, gin.H{"error": "无可用 IPv6 子段"})
				return
			}
			if err == service.ErrEIPAlreadyAssigned {
				c.JSON(http.StatusConflict, gin.H{"error": "EIP 已分配"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "分配失败"})
			return
		}
	} else {
		alloc, err = networkService.AllocateEIP(nodeID, req.IPVersion, req.PrefixLen, req.SpecificIP)
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
	}
	c.JSON(http.StatusCreated, alloc)
	BroadcastDataRefresh("eip_allocations", req.NodeID)
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
	BroadcastDataRefresh("eip_allocations", "")
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
	BroadcastDataRefresh("eip_allocations", "")
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
	HostPort      int    `json:"host_port"`
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

	// 校验：有公网 IP（EIP）的实例不允许创建端口映射
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "实例不存在"})
		return
	}
	if instance.IPv4EIPAllocationID != nil || instance.IPv6EIPAllocationID != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "该实例已分配公网 IP，无需端口映射"})
		return
	}

	hostPort := req.HostPort
	if hostPort > 0 {
		if hostPort < 1 || hostPort > 65535 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "外部端口范围 1-65535"})
			return
		}
	}
	pm, err := networkService.AddPortMapping(instanceID, req.ContainerPort, hostPort, req.Protocol, req.IPVersion, req.Description)
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
	BroadcastDataRefresh("port_mappings", "")
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
	BroadcastDataRefresh("port_mappings", "")
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
