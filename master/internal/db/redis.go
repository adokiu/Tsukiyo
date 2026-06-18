package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"tsukiyo/master/internal/config"
)

var RedisClient *redis.Client

// InitRedis 初始化 Redis 连接
func InitRedis(cfg *config.RedisConfig) error {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := RedisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("连接 Redis 失败: %w", err)
	}

	zap.L().Info("Redis 连接成功",
		zap.String("addr", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)),
		zap.Int("db", cfg.DB),
	)

	return nil
}

// CacheGet 缓存读取
func CacheGet(ctx context.Context, key string) (string, error) {
	if RedisClient == nil {
		return "", fmt.Errorf("Redis 未初始化")
	}
	return RedisClient.Get(ctx, key).Result()
}

// CacheSet 缓存写入
func CacheSet(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if RedisClient == nil {
		return fmt.Errorf("Redis 未初始化")
	}
	return RedisClient.Set(ctx, key, value, ttl).Err()
}

// CacheDelete 缓存删除
func CacheDelete(ctx context.Context, key string) error {
	if RedisClient == nil {
		return fmt.Errorf("Redis 未初始化")
	}
	return RedisClient.Del(ctx, key).Err()
}

// CacheDeletePattern 按模式删除缓存
func CacheDeletePattern(ctx context.Context, pattern string) error {
	if RedisClient == nil {
		return fmt.Errorf("Redis 未初始化")
	}
	iter := RedisClient.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		if err := RedisClient.Del(ctx, iter.Val()).Err(); err != nil {
			zap.L().Warn("删除缓存失败", zap.String("key", iter.Val()), zap.Error(err))
		}
	}
	return iter.Err()
}

// CacheGetJSON 获取 JSON 缓存并解析
func CacheGetJSON(ctx context.Context, key string, dest interface{}) error {
	data, err := CacheGet(ctx, key)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(data), dest)
}

// CacheSetJSON 设置 JSON 缓存
func CacheSetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return CacheSet(ctx, key, data, ttl)
}

// CacheExists 检查 key 是否存在
func CacheExists(ctx context.Context, key string) (bool, error) {
	if RedisClient == nil {
		return false, fmt.Errorf("Redis 未初始化")
	}
	n, err := RedisClient.Exists(ctx, key).Result()
	return n > 0, err
}

// CacheTTL 获取 key 剩余 TTL
func CacheTTL(ctx context.Context, key string) (time.Duration, error) {
	if RedisClient == nil {
		return 0, fmt.Errorf("Redis 未初始化")
	}
	return RedisClient.TTL(ctx, key).Result()
}

// CacheIncr 原子递增
func CacheIncr(ctx context.Context, key string) (int64, error) {
	if RedisClient == nil {
		return 0, fmt.Errorf("Redis 未初始化")
	}
	return RedisClient.Incr(ctx, key).Result()
}

// CacheExpire 设置过期时间
func CacheExpire(ctx context.Context, key string, ttl time.Duration) error {
	if RedisClient == nil {
		return fmt.Errorf("Redis 未初始化")
	}
	return RedisClient.Expire(ctx, key, ttl).Err()
}
