package task

import (
	"strconv"
	"time"

	"go.uber.org/zap"

	"tsukiyo/agent/internal/incus"
)

// waitForRunning 等待实例变为 running 状态
func waitForRunning(client *incus.Client, name string, timeoutSec int) {
	for i := 0; i < timeoutSec; i++ {
		if info, err := client.GetInstance(name); err == nil && info.Status == "Running" {
			zap.L().Info("实例已进入 Running 状态", zap.String("name", name), zap.Int("wait_seconds", i))
			return
		}
		time.Sleep(1 * time.Second)
	}
	zap.L().Warn("等待 Running 超时", zap.String("name", name), zap.Int("timeout_sec", timeoutSec))
}

func getMapString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getMapInt(m map[string]interface{}, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	if v, ok := m[key].(int); ok {
		return v
	}
	return 0
}

func parseInt(v interface{}) int {
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case string:
		i, _ := strconv.Atoi(val)
		return i
	}
	return 0
}
