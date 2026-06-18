package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"tsukiyo/master/internal/db"
)

// RateLimitMiddleware API 限流中间件
func RateLimitMiddleware(requests int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		key := fmt.Sprintf("rate_limit:%s", clientIP)

		ctx := c.Request.Context()

		// 尝试递增计数
		count, err := db.RedisClient.Incr(ctx, key).Result()
		if err != nil {
			zap.L().Warn("限流计数失败", zap.Error(err))
			c.Next()
			return
		}

		// 首次请求设置过期时间
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
