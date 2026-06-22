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
	ErrInstanceNotFound           = &ServiceError{Message: "实例不存在"}
	ErrPortMappingLimitReached    = &ServiceError{Message: "端口映射数量已达上限"}
	ErrPortMappingNotFound        = &ServiceError{Message: "端口映射不存在"}
	ErrFirewallRuleNotFound       = &ServiceError{Message: "规则不存在"}
	ErrInvalidCIDR                = &ServiceError{Message: "CIDR 格式无效"}
	ErrBridgeNotFound             = &ServiceError{Message: "网桥不存在"}
	ErrBridgeHasInstances         = &ServiceError{Message: "网桥下存在实例"}
	ErrInvalidBridgeID            = &ServiceError{Message: "无效的网桥 ID"}
	ErrInvalidNodeID              = &ServiceError{Message: "无效的节点 ID"}
	ErrNodeOffline                = &ServiceError{Message: "节点离线"}
	ErrImageNotDownloaded         = &ServiceError{Message: "镜像未下载，请先下载镜像"}
	ErrInstanceNameExists         = &ServiceError{Message: "该节点上已存在同名实例"}
	ErrEIPPoolNotFound            = &ServiceError{Message: "EIP 资源池不存在"}
	ErrEIPAllocationNotFound      = &ServiceError{Message: "EIP 分配记录不存在"}
	ErrEIPNotAvailable            = &ServiceError{Message: "EIP 不可用或已被分配"}
	ErrNoAvailableEIP             = &ServiceError{Message: "资源池中无可用 EIP"}
	ErrEIPAlreadyAssigned         = &ServiceError{Message: "EIP 已分配，无法重复分配"}
	ErrEIPPoolHasAllocations      = &ServiceError{Message: "资源池中存在已分配的 EIP，无法删除"}
	ErrAgentManagerNotInitialized = &ServiceError{Message: "Agent 管理器未初始化"}
	ErrNodeNotConnected           = &ServiceError{Message: "节点未在线"}
	ErrInvalidImageKeyFormat      = &ServiceError{Message: "无效的 image_key 格式"}
	ErrInstanceBridgeMismatch     = &ServiceError{Message: "实例不属于该网桥"}
	ErrInstanceNoBridge           = &ServiceError{Message: "实例未关联网桥"}
	ErrNoBridgeEgressIP           = &ServiceError{Message: "网桥未配置 NAT 出口 IP"}
	ErrPortOutOfRange             = &ServiceError{Message: "端口超出网桥允许的范围"}
	ErrPortAlreadyUsed            = &ServiceError{Message: "该端口已被占用"}
	ErrNoAvailablePorts           = &ServiceError{Message: "网桥端口范围内无可用端口"}
	ErrOperationTimeout           = &ServiceError{Message: "操作超时，Agent 未在规定时间内响应"}
	ErrAgentFailed                = &ServiceError{Message: "Agent 执行操作失败"}
	ErrInstanceBusy               = &ServiceError{Message: "实例正在执行其他操作，请稍后重试"}
	ErrStoragePoolInUse           = &ServiceError{Message: "存储池仍有资源在使用，无法删除"}
	ErrStoragePoolNotFound        = &ServiceError{Message: "存储池不存在"}
	ErrBridgeCIDROverlap          = &ServiceError{Message: "网桥 CIDR 与同节点其他网桥重叠"}
	ErrEIPPoolCIDROverlap         = &ServiceError{Message: "EIP 池 CIDR 与同节点同 IP 版本其他池重叠"}
	ErrDiskNotFound               = &ServiceError{Message: "数据盘不存在"}
	ErrDiskShrinkNotSupported     = &ServiceError{Message: "磁盘不支持缩容"}
	ErrInvalidResizeConfig        = &ServiceError{Message: "无效的配置调整参数"}
	ErrDiskNameExists             = &ServiceError{Message: "实例下已存在同名数据盘"}
	ErrInstanceBanned             = &ServiceError{Message: "实例已被封禁"}
	ErrInstanceExpired            = &ServiceError{Message: "实例已过期"}
	ErrVMResizeRequiresStop       = &ServiceError{Message: "虚拟机运行时无法调整内存，请先关机再操作"}
)
