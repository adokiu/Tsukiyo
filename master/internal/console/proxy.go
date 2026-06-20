package console

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"tsukiyo/master/internal/db"
)

// ConsoleSession 控制台会话信息（存储在 Redis 中，供 Agent 验证）
type ConsoleSession struct {
	InstanceID string `json:"instance_id"`
	NodeID     string `json:"node_id"`
	Type       string `json:"type"`
	IncusName  string `json:"incus_name"`
	UserID     uint   `json:"user_id"`
}

// GenerateConsoleToken 生成控制台 Token 并存入 Redis
func GenerateConsoleToken(session ConsoleSession) (string, error) {
	token := uuid.New().String()
	ctx := context.Background()
	key := "console_token:" + token
	data, _ := json.Marshal(session)
	if err := db.RedisClient.Set(ctx, key, data, 5*time.Minute).Err(); err != nil {
		return "", err
	}
	return token, nil
}

// ValidateConsoleToken 验证控制台 Token（Agent 通过 WS 调用 Master 验证时使用）
func ValidateConsoleToken(token string) (*ConsoleSession, error) {
	ctx := context.Background()
	key := "console_token:" + token
	data, err := db.RedisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var session ConsoleSession
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, err
	}

	return &session, nil
}

// ConsumeConsoleToken 验证并消费（删除）控制台 Token，防止重放
func ConsumeConsoleToken(token string) (*ConsoleSession, error) {
	session, err := ValidateConsoleToken(token)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	db.RedisClient.Del(ctx, "console_token:"+token)
	return session, nil
}
