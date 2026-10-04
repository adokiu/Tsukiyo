package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"tsukiyo/master/internal/db"
)

// RateLimitMiddleware API 限流；scope 区分不同接口，strict 为 true 时 Redis 故障拒绝请求。
func RateLimitMiddleware(scope string, requests int, window time.Duration, strict bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		key := fmt.Sprintf("rate_limit:%s:%s", scope, clientIP)

		ctx := c.Request.Context()

		count, err := db.RedisClient.Incr(ctx, key).Result()
		if err != nil {
			zap.L().Warn("限流计数失败", zap.Error(err), zap.String("scope", scope))
			if strict {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "服务繁忙，请稍后重试"})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		if count == 1 {
			db.RedisClient.Expire(ctx, key, window)
		}

		if count > int64(requests) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "请求过于频繁，请稍后重试",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
