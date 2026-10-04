package agent

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"tsukiyo/master/internal/auth"
	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/monitor"
)

// HandleUserMetricsWebSocket 处理用户前台 WebSocket 连接
// 通过 query 参数 token 进行 JWT 鉴权，每 1 秒推送一次用户所有运行中实例的最新指标
func (m *Manager) HandleUserMetricsWebSocket(c *gin.Context) {
	tokenString := c.Query("token")
	if tokenString == "" {
		c.JSON(http.StatusOK, gin.H{"code": 401, "error": "缺少认证信息"})
		return
	}

	if auth.IsTokenRevoked(tokenString) {
		c.JSON(http.StatusOK, gin.H{"code": 401, "error": "Token 已吊销"})
		return
	}

	claims, err := auth.ParseToken(tokenString)
	if err != nil {
		zap.L().Warn("WS Token 解析失败", zap.Error(err))
		c.JSON(http.StatusOK, gin.H{"code": 401, "error": "无效的 Token"})
		return
	}

	var user models.User
	if err := db.DB.Where("id = ?", claims.UserID).First(&user).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 401, "error": "用户不存在"})
		return
	}

	if !user.IsActive() {
		c.JSON(http.StatusOK, gin.H{"code": 403, "error": "用户已被禁用"})
		return
	}

	userID := claims.UserID

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		zap.L().Error("用户前台 WebSocket 升级失败", zap.Error(err))
		return
	}
	defer conn.Close()

	var writeMu sync.Mutex
	writeMetrics := func() error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return m.pushUserInstanceMetrics(conn, userID)
	}

	if err := writeMetrics(); err != nil {
		zap.L().Debug("用户前台 WS 首次推送失败", zap.Error(err))
		return
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		<-ticker.C
		if err := writeMetrics(); err != nil {
			zap.L().Debug("用户前台 WS 推送失败，关闭连接", zap.Error(err))
			return
		}
	}
}

// pushUserInstanceMetrics 推送用户所有运行中实例的最新指标
func (m *Manager) pushUserInstanceMetrics(conn *websocket.Conn, userID uint) error {
	var instances []models.Instance
	if err := db.DB.Where("user_id = ? AND status = ?", userID, models.InstanceStatusRunning).Find(&instances).Error; err != nil {
		return err
	}

	type metricsItem struct {
		InstanceID  uuid.UUID `json:"instance_id"`
		CPUUsage    float64   `json:"cpu_usage"`
		MemoryUsed  int64     `json:"memory_used"`
		MemoryTotal int64     `json:"memory_total"`
		DiskUsed    int64     `json:"disk_used"`
		DiskTotal   int64     `json:"disk_total"`
		NetInBps    int64     `json:"net_in_bps"`
		NetOutBps   int64     `json:"net_out_bps"`
		NetInTotal  int64     `json:"net_in_total"`
		NetOutTotal int64     `json:"net_out_total"`
	}

	items := make([]metricsItem, 0, len(instances))
	for _, inst := range instances {
		mp, err := monitor.GetInstanceLatestMetrics(inst.ID)
		if err != nil || mp == nil {
			items = append(items, metricsItem{InstanceID: inst.ID})
			continue
		}
		items = append(items, metricsItem{
			InstanceID:  inst.ID,
			CPUUsage:    mp.CPU,
			MemoryUsed:  (mp.MemUsed + (int64(inst.MemoryMB) - mp.MemTotal)) * 1024 * 1024,
			MemoryTotal: int64(inst.MemoryMB) * 1024 * 1024,
			DiskUsed:    (mp.DiskUsed + (int64(inst.DiskMB) - mp.DiskTotal)) * 1024 * 1024,
			DiskTotal:   int64(inst.DiskMB) * 1024 * 1024,
			NetInBps:    mp.NetIn,
			NetOutBps:   mp.NetOut,
			NetInTotal:  mp.NetInTotal,
			NetOutTotal: mp.NetOutTotal,
		})
	}

	msg, err := json.Marshal(map[string]interface{}{
		"type":      "instance_metrics_batch",
		"timestamp": time.Now().Unix(),
		"items":     items,
	})
	if err != nil {
		return err
	}

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return conn.WriteMessage(websocket.TextMessage, msg)
}
