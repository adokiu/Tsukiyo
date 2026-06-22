package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"tsukiyo/master/internal/service"
)

// serviceErrorStatusCode 将 ServiceError 映射到 HTTP 状态码
var serviceErrorStatusCode = map[*service.ServiceError]int{
	// 404 Not Found
	service.ErrInstanceNotFound:        http.StatusNotFound,
	service.ErrNodeNotFound:            http.StatusNotFound,
	service.ErrUserNotFound:            http.StatusNotFound,
	service.ErrBridgeNotFound:          http.StatusNotFound,
	service.ErrEIPPoolNotFound:         http.StatusNotFound,
	service.ErrEIPAllocationNotFound:   http.StatusNotFound,
	service.ErrDiskNotFound:            http.StatusNotFound,
	service.ErrStoragePoolNotFound:     http.StatusNotFound,
	service.ErrPortMappingNotFound:     http.StatusNotFound,
	service.ErrFirewallRuleNotFound:    http.StatusNotFound,

	// 400 Bad Request
	service.ErrInvalidNodeID:           http.StatusBadRequest,
	service.ErrInvalidBridgeID:         http.StatusBadRequest,
	service.ErrInvalidUserID:           http.StatusBadRequest,
	service.ErrInvalidCIDR:             http.StatusBadRequest,
	service.ErrInvalidImageKeyFormat:   http.StatusBadRequest,
	service.ErrInvalidResizeConfig:     http.StatusBadRequest,
	service.ErrNoValidUpdateFields:     http.StatusBadRequest,
	service.ErrDiskShrinkNotSupported:  http.StatusBadRequest,
	service.ErrPortOutOfRange:          http.StatusBadRequest,
	service.ErrPortAlreadyUsed:         http.StatusBadRequest,

	// 409 Conflict
	service.ErrInstanceNameExists:      http.StatusConflict,
	service.ErrEIPAlreadyAssigned:      http.StatusConflict,
	service.ErrEIPPoolHasAllocations:   http.StatusConflict,
	service.ErrBridgeHasInstances:      http.StatusConflict,
	service.ErrNodeHasInstances:        http.StatusConflict,
	service.ErrDiskNameExists:          http.StatusConflict,
	service.ErrEIPPoolCIDROverlap:      http.StatusConflict,
	service.ErrBridgeCIDROverlap:       http.StatusConflict,
	service.ErrInstanceBusy:            http.StatusConflict,
	service.ErrInstanceBanned:          http.StatusConflict,
	service.ErrInstanceExpired:         http.StatusConflict,
	service.ErrVMResizeRequiresStop:    http.StatusConflict,

	// 503 Service Unavailable
	service.ErrNodeOffline:               http.StatusServiceUnavailable,
	service.ErrNodeNotConnected:          http.StatusServiceUnavailable,
	service.ErrAgentManagerNotInitialized: http.StatusServiceUnavailable,
	service.ErrNoAvailableEIP:            http.StatusServiceUnavailable,
	service.ErrNoAvailablePorts:          http.StatusServiceUnavailable,
	service.ErrNoBridgeEgressIP:          http.StatusServiceUnavailable,
	service.ErrEIPNotAvailable:           http.StatusServiceUnavailable,

	// 504 Gateway Timeout
	service.ErrOperationTimeout: http.StatusGatewayTimeout,

	// 500 Internal Server Error
	service.ErrAgentFailed:         http.StatusInternalServerError,
	service.ErrInstanceBridgeMismatch: http.StatusBadRequest,
	service.ErrInstanceNoBridge:    http.StatusBadRequest,
	service.ErrStoragePoolInUse:    http.StatusConflict,
}

// HandleServiceError 统一处理 service 层错误，自动映射 HTTP 状态码。
// 返回 true 表示错误已处理（调用方应 return），false 表示错误不是 ServiceError（调用方需自行处理）。
func HandleServiceError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if serviceErr, ok := err.(*service.ServiceError); ok {
		statusCode, exists := serviceErrorStatusCode[serviceErr]
		if !exists {
			statusCode = http.StatusInternalServerError
		}
		c.JSON(statusCode, gin.H{"error": serviceErr.Message})
		return true
	}
	return false
}

// HandleServiceErrorWithFallback 统一处理 service 层错误，未匹配时使用 fallbackStatusCode 和 fallbackMessage。
func HandleServiceErrorWithFallback(c *gin.Context, err error, fallbackStatusCode int, fallbackMessage string) {
	if HandleServiceError(c, err) {
		return
	}
	c.JSON(fallbackStatusCode, gin.H{"error": fallbackMessage})
}

// BindErrorResponse 请求参数绑定失败的统一响应
func BindErrorResponse(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
}
