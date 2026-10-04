package middleware

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// routePermissionMap 管理端 API 路由到权限的映射（/api/v1/user/* 不在此组内）
var routePermissionMap = map[string]string{
	"POST /api/v1/instances":                             "instance:create",
	"GET /api/v1/instances":                              "instance:read",
	"GET /api/v1/instances/:id":                          "instance:read",
	"PUT /api/v1/instances/:id":                          "instance:update",
	"DELETE /api/v1/instances/:id":                       "instance:delete",
	"POST /api/v1/instances/:id/start":                   "instance:start",
	"POST /api/v1/instances/:id/stop":                    "instance:stop",
	"POST /api/v1/instances/:id/restart":                 "instance:restart",
	"POST /api/v1/instances/:id/reinstall":               "instance:reinstall",
	"POST /api/v1/instances/:id/resize":                  "instance:update",
	"POST /api/v1/instances/:id/reset-password":          "instance:update",
	"POST /api/v1/instances/:id/status":                  "instance:update",
	"POST /api/v1/instances/:id/ban":                     "instance:update",
	"POST /api/v1/instances/:id/unban":                   "instance:update",
	"POST /api/v1/instances/:id/renew":                   "instance:update",
	"POST /api/v1/instances/:id/network":                 "instance:update",
	"GET /api/v1/instances/:id/console":                  "instance:console",
	"GET /api/v1/instances/:id/metrics":                  "instance:read",
	"GET /api/v1/instances/:id/metrics/history":          "instance:read",
	"POST /api/v1/instances/:id/disks":                   "instance:update",
	"DELETE /api/v1/instances/:id/disks/:disk_id":        "instance:update",
	"PUT /api/v1/instances/:id/disks/:disk_id":           "instance:update",
	"GET /api/v1/instances/:id/snapshots":                "instance:snapshot",
	"POST /api/v1/instances/:id/snapshots":               "instance:snapshot",
	"POST /api/v1/instances/:id/snapshots/:name/restore": "instance:snapshot",
	"DELETE /api/v1/instances/:id/snapshots/:name":       "instance:snapshot",

	"GET /api/v1/nodes":                                        "node:read",
	"POST /api/v1/nodes":                                       "node:create",
	"GET /api/v1/nodes/:id":                                    "node:read",
	"PUT /api/v1/nodes/:id/config":                             "node:update",
	"DELETE /api/v1/nodes/:id":                                 "node:delete",
	"GET /api/v1/nodes/:id/disks":                              "node:read",
	"POST /api/v1/nodes/:id/disks/format":                      "node:update",
	"POST /api/v1/nodes/:id/disks/partitions":                  "node:update",
	"DELETE /api/v1/nodes/:id/disks/partitions/:device":        "node:update",
	"GET /api/v1/nodes/:id/storages":                           "node:read",
	"POST /api/v1/nodes/:id/storages/init":                     "node:update",
	"DELETE /api/v1/nodes/:id/storages/:name":                  "node:update",
	"GET /api/v1/nodes/:id/storages/:name/volumes":             "node:read",
	"GET /api/v1/nodes/:id/storages/:name/resources":           "node:read",
	"GET /api/v1/nodes/:id/networks":                           "node:read",
	"GET /api/v1/nodes/:id/bridges":                            "node:read",
	"GET /api/v1/nodes/:id/tasks":                              "node:read",
	"GET /api/v1/nodes/:id/security-alerts":                    "node:read",

	"GET /api/v1/users":        "user:read",
	"GET /api/v1/users/:id":    "user:read",
	"PUT /api/v1/users/:id":    "user:update",
	"DELETE /api/v1/users/:id": "user:delete",

	"GET /api/v1/user-groups":        "user:group_manage",
	"POST /api/v1/user-groups":       "user:group_manage",
	"PUT /api/v1/user-groups/:id":    "user:group_manage",
	"DELETE /api/v1/user-groups/:id": "user:group_manage",

	"GET /api/v1/finance/payment-drivers":      "system:config",
	"GET /api/v1/finance/overview":             "system:config",
	"GET /api/v1/finance/bills":                "system:config",
	"GET /api/v1/finance/bills/:id":            "system:config",
	"GET /api/v1/finance/payment-channels":     "system:config",
	"POST /api/v1/finance/payment-channels":    "system:config",
	"PUT /api/v1/finance/payment-channels/:id": "system:config",
	"DELETE /api/v1/finance/payment-channels/:id": "system:config",

	"GET /api/v1/images":                         "image:read",
	"POST /api/v1/images/remote/list":            "image:read",
	"POST /api/v1/images/download":               "image:create",
	"GET /api/v1/images/progress":                "image:read",
	"POST /api/v1/images/cancel":                 "image:update",
	"DELETE /api/v1/images":                      "image:update",
	"GET /api/v1/images/source":                  "image:read",
	"PUT /api/v1/images/source":                  "image:update",
	"POST /api/v1/images/refresh":                "image:update",
	"GET /api/v1/images/installed":               "image:read",
	"POST /api/v1/images/sync":                   "image:update",
	"GET /api/v1/images/categories":              "image:read",
	"POST /api/v1/images/categories":             "image:update",
	"PUT /api/v1/images/categories/:id":          "image:update",
	"DELETE /api/v1/images/categories/:id":       "image:update",
	"PUT /api/v1/images/alias":                   "image:update",
	"GET /api/v1/images/reinstall":               "image:read",

	"GET /api/v1/network/bridges":                              "network:manage",
	"POST /api/v1/network/bridges":                             "network:manage",
	"GET /api/v1/network/bridges/:id":                          "network:manage",
	"PUT /api/v1/network/bridges/:id":                          "network:manage",
	"DELETE /api/v1/network/bridges/:id":                       "network:manage",
	"POST /api/v1/network/bridges/:id/bind-egress":             "network:manage",
	"POST /api/v1/network/bridges/:id/unbind-egress":           "network:manage",
	"GET /api/v1/network/eip-pools":                            "network:manage",
	"POST /api/v1/network/eip-pools":                           "network:manage",
	"DELETE /api/v1/network/eip-pools/:id":                     "network:manage",
	"PUT /api/v1/network/eip-pools/:id":                        "network:manage",
	"GET /api/v1/network/eip-available":                        "network:manage",
	"GET /api/v1/network/eip-available-list":                   "network:manage",
	"GET /api/v1/network/bridge-ipv6-available":                "network:manage",
	"GET /api/v1/network/eip-allocations":                      "network:manage",
	"POST /api/v1/network/eip-allocations/allocate":            "network:ip_allocate",
	"POST /api/v1/network/eip-allocations/:id/assign":          "network:ip_allocate",
	"POST /api/v1/network/eip-allocations/:id/release":         "network:ip_allocate",
	"GET /api/v1/network/port-mappings":                        "network:port_forward",
	"POST /api/v1/network/port-mappings":                       "network:port_forward",
	"DELETE /api/v1/network/port-mappings/:id":                 "network:port_forward",
	"GET /api/v1/network/firewall":                             "network:manage",
	"POST /api/v1/network/firewall":                            "network:manage",
	"PUT /api/v1/network/firewall/:id":                         "network:manage",
	"DELETE /api/v1/network/firewall/:id":                      "network:manage",

	"POST /api/v1/batch/create": "instance:create",
	"POST /api/v1/batch/action": "instance:update",

	"GET /api/v1/security/alerts":              "system:config",
	"GET /api/v1/security/summary":             "system:config",
	"POST /api/v1/security/alerts/:id/resolve": "system:config",
	"POST /api/v1/security/alerts/:id/ignore":  "system:config",

	"GET /api/v1/audit-logs": "audit:read",

	"GET /api/v1/tasks":          "system:config",
	"GET /api/v1/tasks/:id":      "system:config",
	"GET /api/v1/tasks/:id/logs": "system:config",

	"GET /api/v1/dashboard": "system:config",

	"GET /api/v1/settings/site":        "system:config",
	"PUT /api/v1/settings/site":        "system:config",
	"GET /api/v1/settings/smtp":        "system:config",
	"PUT /api/v1/settings/smtp":        "system:config",
	"POST /api/v1/settings/smtp/test":  "system:config",

	"GET /api/v1/console/credentials": "instance:console",

	"PUT /api/v1/theme/upload":       "system:config",
	"GET /api/v1/theme/list":         "system:config",
	"GET /api/v1/theme/set":          "system:config",
	"POST /api/v1/theme/delete":      "system:config",
	"POST /api/v1/theme/import":      "system:config",
	"POST /api/v1/theme/settings":    "system:config",
	"GET /api/v1/theme/settings":     "system:config",

	"GET /api/v1/admin/entry-path":                 "system:config",
	"POST /api/v1/admin/entry-path/regenerate":     "system:config",

	"GET /api/v1/commerce/categories":        "system:config",
	"POST /api/v1/commerce/categories":       "system:config",
	"PUT /api/v1/commerce/categories/:id":    "system:config",
	"DELETE /api/v1/commerce/categories/:id": "system:config",
	"GET /api/v1/commerce/products":          "system:config",
	"POST /api/v1/commerce/products":         "system:config",
	"GET /api/v1/commerce/products/:id":      "system:config",
	"PUT /api/v1/commerce/products/:id":      "system:config",
	"DELETE /api/v1/commerce/products/:id":   "system:config",

	"GET /api/v1/tickets":                    "system:config",
	"GET /api/v1/tickets/:id":                "system:config",
	"POST /api/v1/tickets/:id/reply":         "system:config",
	"PUT /api/v1/tickets/:id/status":         "system:config",
	"PUT /api/v1/tickets/:id/assign":         "system:config",
	"PUT /api/v1/tickets/:id/priority":       "system:config",
}

var collectionGETPaths = []*regexp.Regexp{
	regexp.MustCompile(`^/api/v1/instances$`),
	regexp.MustCompile(`^/api/v1/nodes$`),
	regexp.MustCompile(`^/api/v1/users$`),
	regexp.MustCompile(`^/api/v1/user-groups$`),
	regexp.MustCompile(`^/api/v1/finance/bills$`),
	regexp.MustCompile(`^/api/v1/finance/payment-channels$`),
	regexp.MustCompile(`^/api/v1/images$`),
	regexp.MustCompile(`^/api/v1/network/bridges$`),
	regexp.MustCompile(`^/api/v1/network/eip-pools$`),
	regexp.MustCompile(`^/api/v1/network/eip-allocations$`),
	regexp.MustCompile(`^/api/v1/network/port-mappings$`),
	regexp.MustCompile(`^/api/v1/network/firewall$`),
	regexp.MustCompile(`^/api/v1/security/alerts$`),
	regexp.MustCompile(`^/api/v1/audit-logs$`),
	regexp.MustCompile(`^/api/v1/tasks$`),
	regexp.MustCompile(`^/api/v1/commerce/categories$`),
	regexp.MustCompile(`^/api/v1/commerce/products$`),
	regexp.MustCompile(`^/api/v1/tickets$`),
}

// RBACMiddleware RBAC 权限中间件（管理端 API）
func RBACMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetUser(c)
		if user.ID == 0 {
			c.JSON(http.StatusOK, gin.H{"code": 401, "error": "未认证"})
			c.Abort()
			return
		}

		if isAuthenticatedOnlyRoute(c.Request.Method, c.Request.URL.Path) {
			c.Next()
			return
		}

		if !UserHasAdminPanelAccess(user.ID) {
			zap.L().Warn("非管理端用户访问管理 API",
				zap.Uint("user_id", user.ID),
				zap.String("path", c.Request.URL.Path))
			c.JSON(http.StatusOK, gin.H{"code": 403, "error": "无权访问管理接口"})
			c.Abort()
			return
		}

		requiredPerm := getRequiredPermission(c)
		if requiredPerm == "" {
			zap.L().Warn("管理 API 未配置权限映射",
				zap.String("method", c.Request.Method),
				zap.String("path", c.Request.URL.Path))
			c.JSON(http.StatusOK, gin.H{"code": 403, "error": "接口未授权"})
			c.Abort()
			return
		}

		perms := loadUserPermissions(user.ID)

		permInfo, hasPerm := perms[requiredPerm]
		if !hasPerm {
			zap.L().Warn("权限不足",
				zap.Uint("user_id", user.ID),
				zap.String("perm", requiredPerm),
				zap.String("path", c.Request.URL.Path))
			c.JSON(http.StatusOK, gin.H{"code": 403, "error": "权限不足"})
			c.Abort()
			return
		}

		if !checkScope(c, user.ID, permInfo.Scope) {
			c.JSON(http.StatusOK, gin.H{"code": 403, "error": "无权操作此资源"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func isAuthenticatedOnlyRoute(method, path string) bool {
	key := method + " " + path
	return key == "POST /api/v1/auth/logout" || key == "POST /api/v1/auth/change-password"
}

// PermissionInfo 权限信息
type PermissionInfo struct {
	PermissionID string
	Scope        string
}

func getRequiredPermission(c *gin.Context) string {
	method := c.Request.Method
	path := c.Request.URL.Path

	key := method + " " + path
	if perm, ok := routePermissionMap[key]; ok {
		return perm
	}

	for route, perm := range routePermissionMap {
		if matchRoute(method+" "+path, route) {
			return perm
		}
	}

	return ""
}

func matchRoute(actual, pattern string) bool {
	actualParts := strings.Split(actual, "/")
	patternParts := strings.Split(pattern, "/")
	if len(actualParts) != len(patternParts) {
		return false
	}
	for i := range actualParts {
		if patternParts[i] != actualParts[i] && !strings.HasPrefix(patternParts[i], ":") {
			return false
		}
	}
	return true
}

func loadUserPermissions(userID uint) map[string]PermissionInfo {
	result := make(map[string]PermissionInfo)

	var perms []struct {
		PermissionID string
		Scope        string
	}

	db.DB.Raw(`
		SELECT p.permission_id, p.scope
		FROM group_permissions p
		INNER JOIN user_group_members m ON m.group_id = p.group_id
		WHERE m.user_id = ?
	`, userID).Scan(&perms)

	for _, p := range perms {
		result[p.PermissionID] = PermissionInfo{
			PermissionID: p.PermissionID,
			Scope:        p.Scope,
		}
	}

	return result
}

func checkScope(c *gin.Context, userID uint, scope string) bool {
	switch scope {
	case "all":
		return true
	case "own", "group":
		if c.Request.Method == http.MethodGet && isCollectionGET(c.Request.URL.Path) {
			return false
		}
		return checkOwnResource(c, userID)
	default:
		return false
	}
}

func isCollectionGET(path string) bool {
	for _, re := range collectionGETPaths {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}

func checkOwnResource(c *gin.Context, userID uint) bool {
	path := c.Request.URL.Path

	if strings.Contains(path, "/instances/") {
		parts := strings.Split(path, "/")
		for i, part := range parts {
			if part == "instances" && i+1 < len(parts) {
				instanceIDStr := parts[i+1]
				if instanceIDStr != "" && !strings.HasPrefix(instanceIDStr, ":") {
					var instance models.Instance
					if err := db.DB.Where("id = ?", instanceIDStr).First(&instance).Error; err == nil {
						return instance.UserID == userID
					}
					return false
				}
			}
		}
	}

	if strings.Contains(path, "/users/") {
		parts := strings.Split(path, "/")
		for i, part := range parts {
			if part == "users" && i+1 < len(parts) {
				targetID, err := strconv.ParseUint(parts[i+1], 10, 64)
				if err != nil {
					return false
				}
				return uint(targetID) == userID
			}
		}
	}

	return false
}

// RequirePermission 单独检查某个权限 (用于特殊逻辑)
func RequirePermission(userID uint, permID string) bool {
	perms := loadUserPermissions(userID)
	_, ok := perms[permID]
	return ok
}
