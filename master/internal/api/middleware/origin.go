package middleware

import (
	"net/http"
	"strings"

	"tsukiyo/master/internal/config"
)

// IsAllowedOrigin 校验浏览器跨域 Origin；无 Origin 时允许（非浏览器客户端、同源）。
func IsAllowedOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}

	allowed := config.AppConfig.Security.AllowedOrigins
	if len(allowed) == 0 {
		return false
	}

	for _, o := range allowed {
		if strings.EqualFold(strings.TrimSpace(o), origin) {
			return true
		}
	}
	return false
}
