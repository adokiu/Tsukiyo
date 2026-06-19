package errors

// AgentError Agent层错误
type AgentError struct {
	Message string
}

func (e *AgentError) Error() string {
	return e.Message
}

// 统一错误定义
var (
	ErrConfigLoadFailed       = &AgentError{Message: "配置加载失败"}
	ErrConfigInvalid          = &AgentError{Message: "配置无效"}
	ErrMasterConnectFailed    = &AgentError{Message: "连接Master失败"}
	ErrMasterAuthFailed       = &AgentError{Message: "Master认证失败"}
	ErrIncusConnectFailed     = &AgentError{Message: "连接Incus失败"}
	ErrIncusOperationFailed   = &AgentError{Message: "Incus操作失败"}
	ErrInstanceNotFound       = &AgentError{Message: "实例不存在"}
	ErrInstanceCreateFailed   = &AgentError{Message: "创建实例失败"}
	ErrInstanceDeleteFailed   = &AgentError{Message: "删除实例失败"}
	ErrImageNotFound          = &AgentError{Message: "镜像不存在"}
	ErrImageDownloadFailed    = &AgentError{Message: "镜像下载失败"}
	ErrNetworkConfigFailed    = &AgentError{Message: "网络配置失败"}
	ErrTaskExecutionFailed    = &AgentError{Message: "任务执行失败"}
	ErrTaskUnknown           = &AgentError{Message: "未知任务类型"}
	ErrStoragePoolNotFound    = &AgentError{Message: "存储池不存在"}
	ErrStoragePoolCreateFailed = &AgentError{Message: "创建存储池失败"}
	ErrNetworkNotFound       = &AgentError{Message: "网络不存在"}
	ErrNetworkCreateFailed   = &AgentError{Message: "创建网络失败"}
	ErrSnapshotNotFound      = &AgentError{Message: "快照不存在"}
	ErrSnapshotCreateFailed  = &AgentError{Message: "创建快照失败"}
	ErrConsoleProxyFailed    = &AgentError{Message: "控制台代理失败"}
	ErrMonitorCollectionFailed = &AgentError{Message: "监控采集失败"}
	ErrSecurityScanFailed    = &AgentError{Message: "安全扫描失败"}
	ErrReconcileFailed       = &AgentError{Message: "状态对齐失败"}
)
