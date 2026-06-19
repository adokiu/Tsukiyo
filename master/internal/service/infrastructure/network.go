package infrastructure

import (
	"encoding/json"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"tsukiyo/master/internal/agent"
	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/service"
)

// NetworkService 网络服务
type NetworkService struct {
	agentMgr *agent.Manager
}

// NewNetworkService 创建网络服务
func NewNetworkService(agentMgr *agent.Manager) *NetworkService {
	return &NetworkService{agentMgr: agentMgr}
}

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
func (s *NetworkService) ListIPPools(nodeID, status string) ([]IPPoolInfo, error) {
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
		return nil, err
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

	return result, nil
}

// AddIPPool 添加 IP 到池
func (s *NetworkService) AddIPPool(nodeID uuid.UUID, address, gateway string, prefixLen int, iface string) (*models.PublicIPPool, error) {
	// 检查 IP 是否已存在
	var existing models.PublicIPPool
	if err := db.DB.Where("address = ? AND node_id = ?", address, nodeID).First(&existing).Error; err == nil {
		return nil, service.ErrIPAlreadyExists
	}

	if prefixLen <= 0 || prefixLen > 32 {
		prefixLen = 32
	}

	ip := models.PublicIPPool{
		ID:        uuid.New(),
		NodeID:    nodeID,
		Address:   address,
		Gateway:   gateway,
		PrefixLen: prefixLen,
		Interface: iface,
		Status:    models.IPStatusFree,
	}

	if err := db.DB.Create(&ip).Error; err != nil {
		zap.L().Error("添加 IP 失败", zap.Error(err))
		return nil, err
	}

	return &ip, nil
}

// DeleteIPPool 从池中删除 IP
func (s *NetworkService) DeleteIPPool(ipID uuid.UUID) error {
	var ip models.PublicIPPool
	if err := db.DB.Where("id = ?", ipID).First(&ip).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return service.ErrIPNotFound
		}
		return err
	}

	if ip.Status == models.IPStatusAssigned {
		return service.ErrIPAssigned
	}

	if err := db.DB.Delete(&ip).Error; err != nil {
		return err
	}

	return nil
}

// ListIPv6Prefixes 获取 IPv6 前缀列表
func (s *NetworkService) ListIPv6Prefixes(nodeID string) ([]models.IPv6Prefix, error) {
	query := db.DB.Order("prefix ASC")
	if nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}

	var prefixes []models.IPv6Prefix
	if err := query.Find(&prefixes).Error; err != nil {
		return nil, err
	}

	return prefixes, nil
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
func (s *NetworkService) ListPortMappings(instanceID, nodeID string) ([]PortMappingInfo, error) {
	query := db.DB.Order("created_at DESC")
	if instanceID != "" {
		query = query.Where("instance_id = ?", instanceID)
	}
	if nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}

	var mappings []models.PortMapping
	if err := query.Find(&mappings).Error; err != nil {
		return nil, err
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

	return result, nil
}

// AddPortMapping 添加端口映射
func (s *NetworkService) AddPortMapping(instanceID uuid.UUID, containerPort, hostPort int, protocol, hostIP, description string, userID uint) ([]models.PortMapping, []models.Task, error) {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		return nil, nil, service.ErrInstanceNotFound
	}

	// 检查端口映射配额
	var mappingCount int64
	db.DB.Model(&models.PortMapping{}).Where("instance_id = ?", instanceID).Count(&mappingCount)
	if int(mappingCount) >= instance.PortMappingLimit {
		return nil, nil, service.ErrPortMappingLimitReached
	}

	if hostPort == 0 {
		hostPort = 10000 + int(mappingCount)*10 + 1
	}

	// 获取实例所属 VPC 的 SNAT 出口 IP
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
	if protocol == "udp" {
		protocols = []string{"udp"}
	} else if protocol == "both" {
		protocols = []string{"tcp", "udp"}
	}

	var results []models.PortMapping
	var tasks []models.Task

	for _, proto := range protocols {
		pm := models.PortMapping{
			ID:            uuid.New(),
			InstanceID:    instanceID,
			NodeID:        instance.NodeID,
			ContainerPort: containerPort,
			HostPort:      hostPort,
			Protocol:      proto,
			HostIP:        hostIP,
			Description:   description,
		}

		if err := db.DB.Create(&pm).Error; err != nil {
			return nil, nil, err
		}
		results = append(results, pm)

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
			UserID:     userID,
			Status:     models.TaskStatusPending,
			Payload:    payloadBytes,
		})
	}

	for _, t := range tasks {
		if err := db.DB.Create(&t).Error; err != nil {
			zap.L().Error("创建端口映射任务失败", zap.Error(err))
		}
	}

	return results, tasks, nil
}

// DeletePortMapping 删除端口映射
func (s *NetworkService) DeletePortMapping(pmID uuid.UUID, userID uint) (*models.Task, error) {
	var pm models.PortMapping
	if err := db.DB.Where("id = ?", pmID).First(&pm).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, service.ErrPortMappingNotFound
		}
		return nil, err
	}

	var instance models.Instance
	db.DB.Where("id = ?", pm.InstanceID).First(&instance)

	payload := map[string]interface{}{
		"instance_id": instance.IncusName,
		"action":      "del_port",
		"host_port":   pm.HostPort,
		"protocol":    pm.Protocol,
		"host_ip":     pm.HostIP,
	}
	payloadBytes, _ := json.Marshal(payload)

	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeApplyNetwork,
		NodeID:     pm.NodeID,
		InstanceID: &pm.InstanceID,
		UserID:     userID,
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}
	if err := db.DB.Create(&task).Error; err != nil {
		zap.L().Error("创建删除端口映射任务失败", zap.Error(err))
	}

	db.DB.Delete(&pm)

	return &task, nil
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
func (s *NetworkService) ListFirewallRules(instanceID string) ([]FirewallRuleInfo, error) {
	query := db.DB.Order("priority ASC, created_at DESC")
	if instanceID != "" {
		query = query.Where("instance_id = ?", instanceID)
	}

	var rules []models.FirewallRule
	if err := query.Find(&rules).Error; err != nil {
		return nil, err
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

	return result, nil
}

// AddFirewallRule 添加防火墙规则
func (s *NetworkService) AddFirewallRule(instanceID uuid.UUID, network, direction, protocol, port, sourceIP, action, description string, priority int, userID uint) (*models.FirewallRule, *models.Task, error) {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		return nil, nil, service.ErrInstanceNotFound
	}

	if network == "" {
		network = "ipv4"
	}
	if protocol == "" {
		protocol = "all"
	}
	if priority == 0 {
		priority = 100
	}

	rule := models.FirewallRule{
		ID:          uuid.New(),
		InstanceID:  instanceID,
		NodeID:      instance.NodeID,
		Network:     network,
		Direction:   direction,
		Protocol:    protocol,
		Port:        port,
		SourceIP:    sourceIP,
		Action:      action,
		Description: description,
		Enabled:     true,
		Priority:    priority,
	}

	if err := db.DB.Create(&rule).Error; err != nil {
		return nil, nil, err
	}

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

	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeApplyFirewall,
		NodeID:     instance.NodeID,
		InstanceID: &instance.ID,
		UserID:     userID,
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}
	if err := db.DB.Create(&task).Error; err != nil {
		zap.L().Error("创建防火墙规则任务失败", zap.Error(err))
	}

	return &rule, &task, nil
}

// UpdateFirewallRule 更新防火墙规则
func (s *NetworkService) UpdateFirewallRule(ruleID uuid.UUID, protocol, port, sourceIP, action string, enabled *bool, priority int, description string) error {
	var rule models.FirewallRule
	if err := db.DB.Where("id = ?", ruleID).First(&rule).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return service.ErrFirewallRuleNotFound
		}
		return err
	}

	updates := make(map[string]interface{})
	if protocol != "" {
		updates["protocol"] = protocol
	}
	if port != "" {
		updates["port"] = port
	}
	if sourceIP != "" {
		updates["source_ip"] = sourceIP
	}
	if action != "" {
		updates["action"] = action
	}
	if enabled != nil {
		updates["enabled"] = *enabled
	}
	if priority > 0 {
		updates["priority"] = priority
	}
	if description != "" {
		updates["description"] = description
	}

	if err := db.DB.Model(&rule).Updates(updates).Error; err != nil {
		return err
	}

	return nil
}

// DeleteFirewallRule 删除防火墙规则
func (s *NetworkService) DeleteFirewallRule(ruleID uuid.UUID, userID uint) (*models.Task, error) {
	var rule models.FirewallRule
	if err := db.DB.Where("id = ?", ruleID).First(&rule).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, service.ErrFirewallRuleNotFound
		}
		return nil, err
	}

	var instance models.Instance
	db.DB.Where("id = ?", rule.InstanceID).First(&instance)

	payload := map[string]interface{}{
		"instance_id": instance.IncusName,
		"action":      "delete",
		"rule_ids":    []string{rule.ID.String()},
	}
	payloadBytes, _ := json.Marshal(payload)

	task := models.Task{
		ID:         uuid.New(),
		Type:       models.TaskTypeApplyFirewall,
		NodeID:     rule.NodeID,
		InstanceID: &rule.InstanceID,
		UserID:     userID,
		Status:     models.TaskStatusPending,
		Payload:    payloadBytes,
	}
	if err := db.DB.Create(&task).Error; err != nil {
		zap.L().Error("创建删除防火墙规则任务失败", zap.Error(err))
	}

	db.DB.Delete(&rule)

	return &task, nil
}

// ListVPCs 获取 VPC 列表
func (s *NetworkService) ListVPCs(nodeID string) ([]models.VPCNetwork, error) {
	query := db.DB.Order("created_at DESC")
	if nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}

	var vpcs []models.VPCNetwork
	if err := query.Find(&vpcs).Error; err != nil {
		return nil, err
	}

	return vpcs, nil
}

// CreateVPC 创建 VPC
func (s *NetworkService) CreateVPC(nodeID uuid.UUID, name, ipv4CIDR, ipv6ULACIDR, ipv6GUACIDR, defaultGatewayV4, defaultGatewayV6, egressV4Primary string, egressV4Extra []string, portRangeStart, portRangeEnd int, parentIface string, userID uint) (*models.VPCNetwork, *models.Task, error) {
	// 检查节点是否存在
	var node models.Node
	if err := db.DB.Where("id = ?", nodeID).First(&node).Error; err != nil {
		return nil, nil, service.ErrNodeNotFound
	}

	// 检查 CIDR 格式
	if _, _, err := net.ParseCIDR(ipv4CIDR); err != nil {
		return nil, nil, service.ErrInvalidCIDR
	}

	// 生成网关地址
	gatewayV4 := defaultGatewayV4
	if gatewayV4 == "" {
		ip, ipNet, _ := net.ParseCIDR(ipv4CIDR)
		ip = ip.Mask(ipNet.Mask)
		ip[len(ip)-1] = 1
		gatewayV4 = ip.String()
	}

	if portRangeStart <= 0 {
		portRangeStart = 10000
	}
	if portRangeEnd <= 0 {
		portRangeEnd = 65535
	}

	extraJSON := ""
	if len(egressV4Extra) > 0 {
		b, _ := json.Marshal(egressV4Extra)
		extraJSON = string(b)
	}

	vpc := models.VPCNetwork{
		ID:               uuid.New(),
		NodeID:           nodeID,
		Name:             name,
		IPv4CIDR:         ipv4CIDR,
		IPv6ULACIDR:      ipv6ULACIDR,
		IPv6GUACIDR:      ipv6GUACIDR,
		DefaultGatewayV4: gatewayV4,
		DefaultGatewayV6: defaultGatewayV6,
		EgressV4Primary:  egressV4Primary,
		EgressV4Extra:    extraJSON,
		PortRangeStart:   portRangeStart,
		PortRangeEnd:     portRangeEnd,
		ParentIface:      parentIface,
		Status:           "active",
		SNATEnabled:      true,
		IPv4Filter:       true,
		IPv6Filter:       true,
		MACFilter:        true,
	}

	if err := db.DB.Create(&vpc).Error; err != nil {
		zap.L().Error("创建 VPC 失败", zap.Error(err))
		return nil, nil, err
	}

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

	task := models.Task{
		ID:      uuid.New(),
		Type:    models.TaskTypeVPCNetwork,
		NodeID:  nodeID,
		UserID:  userID,
		Status:  models.TaskStatusPending,
		Payload: payloadBytes,
	}
	if err := db.DB.Create(&task).Error; err != nil {
		zap.L().Error("创建 VPC 配置任务失败", zap.Error(err))
	}

	return &vpc, &task, nil
}

// GetVPC 获取 VPC 详情
func (s *NetworkService) GetVPC(vpcID uuid.UUID) (*models.VPCNetwork, error) {
	var vpc models.VPCNetwork
	if err := db.DB.Where("id = ?", vpcID).First(&vpc).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, service.ErrVPCNotFound
		}
		return nil, err
	}
	return &vpc, nil
}

// UpdateVPC 更新 VPC
func (s *NetworkService) UpdateVPC(vpcID uuid.UUID, name, ipv4CIDR, ipv6ULACIDR, ipv6GUACIDR, defaultGatewayV4, defaultGatewayV6, egressV4Primary string, egressV4Extra []string, portRangeStart, portRangeEnd int, parentIface, status string, snatEnabled, ipv4Filter, macFilter *bool, userID uint) (*models.Task, error) {
	var vpc models.VPCNetwork
	if err := db.DB.Where("id = ?", vpcID).First(&vpc).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, service.ErrVPCNotFound
		}
		return nil, err
	}

	// 检查 VPC 下是否有实例
	var instanceCount int64
	db.DB.Model(&models.Instance{}).Where("vpc_id = ?", vpcID).Count(&instanceCount)
	hasInstances := instanceCount > 0

	updates := map[string]interface{}{}

	if name != "" {
		updates["name"] = name
	}

	if ipv4CIDR != "" && ipv4CIDR != vpc.IPv4CIDR {
		if hasInstances {
			return nil, service.ErrVPCHasInstances
		}
		if _, _, err := net.ParseCIDR(ipv4CIDR); err != nil {
			return nil, service.ErrInvalidCIDR
		}
		updates["ipv4_cidr"] = ipv4CIDR
	}
	if ipv6ULACIDR != "" && ipv6ULACIDR != vpc.IPv6ULACIDR {
		if hasInstances {
			return nil, service.ErrVPCHasInstances
		}
		updates["ipv6_ula_cidr"] = ipv6ULACIDR
	}
	if ipv6GUACIDR != "" && ipv6GUACIDR != vpc.IPv6GUACIDR {
		if hasInstances {
			return nil, service.ErrVPCHasInstances
		}
		updates["ipv6_gua_cidr"] = ipv6GUACIDR
	}
	if defaultGatewayV4 != "" && defaultGatewayV4 != vpc.DefaultGatewayV4 {
		if hasInstances {
			return nil, service.ErrVPCHasInstances
		}
		updates["default_gateway_v4"] = defaultGatewayV4
	}
	if defaultGatewayV6 != "" && defaultGatewayV6 != vpc.DefaultGatewayV6 {
		if hasInstances {
			return nil, service.ErrVPCHasInstances
		}
		updates["default_gateway_v6"] = defaultGatewayV6
	}

	if egressV4Primary != "" {
		updates["egress_v4_primary"] = egressV4Primary
	}
	if len(egressV4Extra) > 0 {
		b, _ := json.Marshal(egressV4Extra)
		updates["egress_v4_extra"] = string(b)
	}
	if portRangeStart > 0 {
		updates["port_range_start"] = portRangeStart
	}
	if portRangeEnd > 0 {
		updates["port_range_end"] = portRangeEnd
	}
	if parentIface != "" {
		updates["parent_iface"] = parentIface
	}
	if status != "" {
		updates["status"] = status
	}
	if snatEnabled != nil {
		updates["snat_enabled"] = *snatEnabled
	}
	if ipv4Filter != nil {
		updates["ipv4_filter"] = *ipv4Filter
	}
	if macFilter != nil {
		updates["mac_filter"] = *macFilter
	}

	if len(updates) == 0 {
		return nil, nil
	}

	if err := db.DB.Model(&vpc).Updates(updates).Error; err != nil {
		return nil, err
	}

	taskPayload := map[string]interface{}{
		"vpc_id":             vpc.ID.String(),
		"action":             "update",
		"bridge_name":        vpc.GetBridgeName(),
		"ipv4_cidr":          vpc.IPv4CIDR,
		"parent_iface":       vpc.ParentIface,
		"egress_v4_primary":  vpc.EgressV4Primary,
		"snat_enabled":       vpc.SNATEnabled,
		"default_gateway_v4": vpc.DefaultGatewayV4,
		"default_gateway_v6": vpc.DefaultGatewayV6,
	}
	payloadBytes, _ := json.Marshal(taskPayload)

	task := models.Task{
		ID:      uuid.New(),
		Type:    models.TaskTypeVPCNetwork,
		NodeID:  vpc.NodeID,
		UserID:  userID,
		Status:  models.TaskStatusPending,
		Payload: payloadBytes,
	}
	if err := db.DB.Create(&task).Error; err != nil {
		zap.L().Error("创建 VPC 更新任务失败", zap.Error(err))
	}

	return &task, nil
}

// DeleteVPC 删除 VPC
func (s *NetworkService) DeleteVPC(vpcID uuid.UUID, userID uint) (*models.Task, error) {
	var vpc models.VPCNetwork
	if err := db.DB.Where("id = ?", vpcID).First(&vpc).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, service.ErrVPCNotFound
		}
		return nil, err
	}

	// 检查是否有实例在使用该 VPC
	var count int64
	db.DB.Model(&models.Instance{}).Where("vpc_id = ?", vpcID).Count(&count)
	if count > 0 {
		return nil, service.ErrVPCHasInstances
	}

	taskPayload := map[string]interface{}{
		"vpc_id":      vpc.ID.String(),
		"action":      "delete",
		"bridge_name": vpc.GetBridgeName(),
		"ipv4_cidr":   vpc.IPv4CIDR,
	}
	payloadBytes, _ := json.Marshal(taskPayload)

	task := models.Task{
		ID:      uuid.New(),
		Type:    models.TaskTypeVPCNetwork,
		NodeID:  vpc.NodeID,
		UserID:  userID,
		Status:  models.TaskStatusPending,
		Payload: payloadBytes,
	}
	if err := db.DB.Create(&task).Error; err != nil {
		zap.L().Error("创建 VPC 删除任务失败", zap.Error(err))
	}

	db.DB.Delete(&vpc)

	return &task, nil
}
