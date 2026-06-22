package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// routePermissionMap 路由到权限的映射
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
	"GET /api/v1/instances/:id/console":                  "instance:console",
	"GET /api/v1/instances/:id/snapshots":                "instance:snapshot",
	"POST /api/v1/instances/:id/snapshots":               "instance:snapshot",
	"POST /api/v1/instances/:id/snapshots/:name/restore": "instance:snapshot",
	"DELETE /api/v1/instances/:id/snapshots/:name":       "instance:snapshot",
	"GET /api/v1/nodes":                                  "node:read",
	"POST /api/v1/nodes":                                 "node:create",
	"GET /api/v1/nodes/:id":                              "node:read",
	"DELETE /api/v1/nodes/:id":                           "node:delete",
	"GET /api/v1/nodes/:id/disks":                        "node:read",
	"POST /api/v1/nodes/:id/disks/format":                "node:update",
	"GET /api/v1/nodes/:id/storages":                     "node:read",
	"POST /api/v1/nodes/:id/storages/init":               "node:update",
	"GET /api/v1/users":                                  "user:read",
	"POST /api/v1/users":                                 "user:create",
	"GET /api/v1/users/:id":                              "user:read",
	"PUT /api/v1/users/:id":                              "user:update",
	"DELETE /api/v1/users/:id":                           "user:delete",
	"GET /api/v1/user-groups":                            "user:group_manage",
	"POST /api/v1/user-groups":                           "user:group_manage",
	"PUT /api/v1/user-groups/:id":                        "user:group_manage",
	"DELETE /api/v1/user-groups/:id":                     "user:group_manage",
	"GET /api/v1/images":                                 "image:read",
	"POST /api/v1/images":                                "image:create",
	"POST /api/v1/images/:id/toggle":                     "image:update",
	"GET /api/v1/images/installed":                       "image:read",
	"GET /api/v1/images/categories":                      "image:read",
	"POST /api/v1/images/categories":                     "image:update",
	"PUT /api/v1/images/categories/:id":                  "image:update",
	"DELETE /api/v1/images/categories/:id":               "image:update",
	"PUT /api/v1/images/alias":                           "image:update",
	"POST /api/v1/images/sync":                           "image:read",
	"GET /api/v1/images/reinstall":                       "image:read",
	"GET /api/v1/network/pools":                          "network:manage",
	"POST /api/v1/network/pools":                         "network:ip_allocate",
	"DELETE /api/v1/network/pools/:id":                   "network:manage",
	"GET /api/v1/network/prefixes":                       "network:manage",
	"GET /api/v1/network/port-mappings":                  "network:port_forward",
	"POST /api/v1/network/port-mappings":                 "network:port_forward",
	"DELETE /api/v1/network/port-mappings/:id":           "network:port_forward",
	"GET /api/v1/network/firewall":                       "network:manage",
	"POST /api/v1/network/firewall":                      "network:manage",
	"PUT /api/v1/network/firewall/:id":                   "network:manage",
	"DELETE /api/v1/network/firewall/:id":                "network:manage",
	"POST /api/v1/batch/create":                          "instance:create",
	"POST /api/v1/batch/action":                          "instance:update",
	"GET /api/v1/security/alerts":                        "system:config",
	"GET /api/v1/security/summary":                       "system:config",
	"GET /api/v1/audit-logs":                             "audit:read",
	"GET /api/v1/dashboard":                              "system:config",
	"GET /api/v1/tasks":                                  "system:config",
	"GET /api/v1/tasks/:id":                              "system:config",
}

// RBACMiddleware RBAC 权限中间件
func RBACMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetUser(c)
		if user.ID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
			c.Abort()
			return
		}

		// 获取当前请求需要的权限
		requiredPerm := getRequiredPermission(c)
		if requiredPerm == "" {
			// 未映射的接口默认放行
			c.Next()
			return
		}

		// 加载用户权限
		perms := loadUserPermissions(user.ID)

		// 检查是否拥有所需权限
		permInfo, hasPerm := perms[requiredPerm]
		if !hasPerm {
			zap.L().Warn("权限不足",
				zap.Uint("user_id", user.ID),
				zap.String("perm", requiredPerm),
				zap.String("path", c.Request.URL.Path))
			c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
			c.Abort()
			return
		}

		// 检查 scope (all / own / group / node)
		if !checkScope(c, user.ID, permInfo.Scope) {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权操作此资源"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// PermissionInfo 权限信息
type PermissionInfo struct {
	PermissionID string
	Scope        string
}

// getRequiredPermission 获取当前路由需要的权限
func getRequiredPermission(c *gin.Context) string {
	method := c.Request.Method
	path := c.Request.URL.Path

	// 尝试精确匹配
	key := method + " " + path
	if perm, ok := routePermissionMap[key]; ok {
		return perm
	}

	// 尝试通配匹配 (替换路径参数为 :id)
	for route, perm := range routePermissionMap {
		if matchRoute(method+" "+path, route) {
			return perm
		}
	}

	return ""
}

// matchRoute 路由匹配
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

// loadUserPermissions 加载用户所有权限
func loadUserPermissions(userID uint) map[string]PermissionInfo {
	result := make(map[string]PermissionInfo)

	// 查询用户所属组的所有权限
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

// checkScope 检查 scope 权限
func checkScope(c *gin.Context, userID uint, scope string) bool {
	switch scope {
	case "all":
		return true
	case "own":
		// 检查资源是否属于当前用户
		return checkOwnResource(c, userID)
	case "group":
		// 简化处理，同 own
		return checkOwnResource(c, userID)
	default:
		return true
	}
}

// checkOwnResource 检查资源是否属于当前用户
func checkOwnResource(c *gin.Context, userID uint) bool {
	path := c.Request.URL.Path

	// 从路径中提取 instance_id 或 user_id
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
				}
			}
		}
	}

	// 默认放行 (针对列表接口等)
	return true
}

// RequirePermission 单独检查某个权限 (用于特殊逻辑)
func RequirePermission(userID uint, permID string) bool {
	perms := loadUserPermissions(userID)
	_, ok := perms[permID]
	return ok
}
