package service

// ServiceError 服务层错误
type ServiceError struct {
	Message string
}

func (e *ServiceError) Error() string {
	return e.Message
}

// 统一错误定义
var (
	ErrNodeNotFound               = &ServiceError{Message: "节点不存在"}
	ErrNodeHasInstances           = &ServiceError{Message: "节点下存在实例，无法删除"}
	ErrUserNotFound               = &ServiceError{Message: "用户不存在"}
	ErrInvalidUserID              = &ServiceError{Message: "无效的用户 ID"}
	ErrNoValidUpdateFields        = &ServiceError{Message: "无有效更新字段"}
	ErrIPAlreadyExists            = &ServiceError{Message: "该 IP 已在池中"}
	ErrIPNotFound                 = &ServiceError{Message: "IP 不存在"}
	ErrIPAssigned                 = &ServiceError{Message: "该 IP 已被分配，无法删除"}
	ErrInstanceNotFound           = &ServiceError{Message: "实例不存在"}
	ErrPortMappingLimitReached    = &ServiceError{Message: "端口映射数量已达上限"}
	ErrPortMappingNotFound        = &ServiceError{Message: "端口映射不存在"}
	ErrFirewallRuleNotFound       = &ServiceError{Message: "规则不存在"}
	ErrInvalidCIDR                = &ServiceError{Message: "IPv4 CIDR 格式无效"}
	ErrVPCNotFound                = &ServiceError{Message: "VPC 不存在"}
	ErrVPCHasInstances            = &ServiceError{Message: "VPC 下存在实例"}
	ErrInvalidNodeID              = &ServiceError{Message: "无效的节点 ID"}
	ErrNodeOffline                = &ServiceError{Message: "节点离线"}
	ErrImageNotDownloaded         = &ServiceError{Message: "镜像未下载，请先下载镜像"}
	ErrInvalidVPCID               = &ServiceError{Message: "无效的 VPC ID"}
	ErrInstanceNameExists         = &ServiceError{Message: "该节点上已存在同名实例"}
	ErrInvalidIPv4ID              = &ServiceError{Message: "无效的公网 IPv4 ID"}
	ErrIPv4NotAvailable           = &ServiceError{Message: "公网 IPv4 不可用或已被分配"}
	ErrInvalidIPv6ID              = &ServiceError{Message: "无效的 IPv6 前缀 ID"}
	ErrIPv6NotFound               = &ServiceError{Message: "IPv6 前缀不存在"}
	ErrAgentManagerNotInitialized = &ServiceError{Message: "Agent 管理器未初始化"}
	ErrNodeNotConnected           = &ServiceError{Message: "节点未在线"}
	ErrInvalidImageKeyFormat      = &ServiceError{Message: "无效的 image_key 格式"}
)
