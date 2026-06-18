package auth

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	"tsukiyo/master/internal/config"
	"tsukiyo/master/internal/db"
)

var (
	ErrInvalidToken = errors.New("无效的 Token")
	ErrTokenExpired = errors.New("Token 已过期")
	ErrTokenRevoked = errors.New("Token 已吊销")
)

// Claims JWT Claims
type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// GenerateToken 生成 JWT Token
func GenerateToken(userID uint, username string) (string, error) {
	cfg := config.AppConfig.JWT
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(cfg.ExpiresIn)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    cfg.Issuer,
			Subject:   strconv.FormatUint(uint64(userID), 10),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Secret))
}

// ParseToken 解析 JWT Token
func ParseToken(tokenString string) (*Claims, error) {
	cfg := config.AppConfig.JWT
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("不支持的签名方法: %v", token.Header["alg"])
		}
		return []byte(cfg.Secret), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// RevokeToken 吊销 Token
func RevokeToken(userID uint, tokenString string, expiresIn time.Duration) error {
	key := fmt.Sprintf("token:blacklist:%s", tokenString)
	if err := cacheSet(key, "1", expiresIn); err != nil {
		zap.L().Error("吊销 Token 失败", zap.Error(err), zap.Uint("user_id", userID))
		return err
	}
	return nil
}

// IsTokenRevoked 检查 Token 是否被吊销
func IsTokenRevoked(tokenString string) bool {
	key := fmt.Sprintf("token:blacklist:%s", tokenString)
	exists, _ := cacheExists(key)
	return exists
}

// cacheSet 写入缓存
func cacheSet(key string, value interface{}, ttl time.Duration) error {
	return db.RedisClient.Set(context.Background(), key, value, ttl).Err()
}

// cacheExists 检查缓存
func cacheExists(key string) (bool, error) {
	ctx := context.Background()
	n, err := db.RedisClient.Exists(ctx, key).Result()
	return n > 0, err
}
