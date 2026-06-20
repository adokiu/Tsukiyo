package console

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"tsukiyo/agent/internal/config"
	"tsukiyo/agent/internal/ws"
)

// Proxy 控制台代理（含健康检查和 Token 鉴权）
type Proxy struct {
	cfg         *config.Config
	wsClient    *ws.Client
	upgrader    websocket.Upgrader
	sshUpgrader websocket.Upgrader
	tokenCache  sync.Map
}

type cachedToken struct {
	valid     bool
	expiresAt time.Time
}

// NewProxy 创建控制台代理
func NewProxy(cfg *config.Config, wsClient *ws.Client) *Proxy {
	return &Proxy{
		cfg:      cfg,
		wsClient: wsClient,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		sshUpgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
			Subprotocols: []string{"binary"},
		},
	}
}

// ServeHTTP 启动统一 HTTP 服务（健康检查 + 控制台代理）
func (p *Proxy) ServeHTTP() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", p.handleHealth)
	mux.HandleFunc("/ready", p.handleReady)

	mux.HandleFunc("/console/ssh", p.withTokenAuth(p.handleWebSSH))
	mux.HandleFunc("/console/vnc", p.withTokenAuth(p.handleWebVNC))

	addr := p.cfg.ConsoleBindAddr()
	if addr == "" {
		addr = "0.0.0.0:9090"
	}
	zap.L().Info("统一 HTTP 服务启动（健康检查 + 控制台代理）", zap.String("addr", addr))
	return http.ListenAndServe(addr, mux)
}

// handleHealth 健康检查
func (p *Proxy) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status":    "healthy",
		"connected": p.wsClient.IsConnected(),
		"timestamp": time.Now().Unix(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleReady 就绪检查
func (p *Proxy) handleReady(w http.ResponseWriter, r *http.Request) {
	if p.wsClient.IsConnected() {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ready":true}`))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"ready":false}`))
	}
}

// withTokenAuth Token 鉴权中间件
func (p *Proxy) withTokenAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			token = r.Header.Get("X-Console-Token")
		}
		if token == "" {
			http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized)
			return
		}

		if cached, ok := p.tokenCache.Load(token); ok {
			ct := cached.(*cachedToken)
			if time.Now().Before(ct.expiresAt) {
				if ct.valid {
					next(w, r)
					return
				}
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}
			p.tokenCache.Delete(token)
		}

		valid, err := p.verifyTokenViaMaster(token)
		if err != nil {
			zap.L().Error("Token 验证失败", zap.Error(err))
			http.Error(w, `{"error":"token verification failed"}`, http.StatusInternalServerError)
			return
		}

		p.tokenCache.Store(token, &cachedToken{
			valid:     valid,
			expiresAt: time.Now().Add(30 * time.Second),
		})

		if !valid {
			http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// verifyTokenViaMaster 通过 Master WebSocket 验证 Token
func (p *Proxy) verifyTokenViaMaster(token string) (bool, error) {
	resp, err := p.wsClient.SendRequest("verify_console_token", map[string]string{
		"token": token,
	})
	if err != nil {
		return false, fmt.Errorf("发送验证请求失败: %w", err)
	}
	var result struct {
		Valid bool `json:"valid"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return false, fmt.Errorf("解析验证响应失败: %w", err)
	}
	return result.Valid, nil
}

// handleWebSSH 处理 WebSSH 连接
func (p *Proxy) handleWebSSH(w http.ResponseWriter, r *http.Request) {
	container := r.URL.Query().Get("container")
	if container == "" {
		http.Error(w, "缺少 container 参数", http.StatusBadRequest)
		return
	}

	conn, err := p.sshUpgrader.Upgrade(w, r, nil)
	if err != nil {
		zap.L().Error("WebSocket 升级失败", zap.Error(err))
		return
	}
	defer conn.Close()

	cmd := exec.Command("incus", "exec", container, "--", "/bin/bash")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		zap.L().Error("创建 stdin pipe 失败", zap.Error(err))
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		zap.L().Error("创建 stdout pipe 失败", zap.Error(err))
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		zap.L().Error("创建 stderr pipe 失败", zap.Error(err))
		return
	}

	if err := cmd.Start(); err != nil {
		zap.L().Error("启动 shell 失败", zap.Error(err))
		return
	}
	defer cmd.Process.Kill()

	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			stdin.Write(data)
		}
	}()

	go io.Copy(stdin, &wsReader{conn: conn})
	go io.Copy(&wsWriter{conn: conn, typ: websocket.TextMessage}, stdout)
	go io.Copy(&wsWriter{conn: conn, typ: websocket.TextMessage}, stderr)

	cmd.Wait()
}

// handleWebVNC 处理 WebVNC 连接
func (p *Proxy) handleWebVNC(w http.ResponseWriter, r *http.Request) {
	container := r.URL.Query().Get("container")
	if container == "" {
		http.Error(w, "缺少 container 参数", http.StatusBadRequest)
		return
	}

	vncPortStr := r.URL.Query().Get("port")
	if vncPortStr == "" {
		vncPortStr = "5900"
	}
	vncPort, _ := strconv.Atoi(vncPortStr)

	conn, err := p.upgrader.Upgrade(w, r, nil)
	if err != nil {
		zap.L().Error("WebSocket 升级失败", zap.Error(err))
		return
	}
	defer conn.Close()

	vncAddr := fmt.Sprintf("127.0.0.1:%d", vncPort)
	vncConn, err := net.Dial("tcp", vncAddr)
	if err != nil {
		zap.L().Error("连接 VNC 失败", zap.Error(err))
		return
	}
	defer vncConn.Close()

	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			vncConn.Write(data)
		}
	}()

	buf := make([]byte, 4096)
	for {
		n, err := vncConn.Read(buf)
		if err != nil {
			return
		}
		if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
			return
		}
	}
}

type wsReader struct {
	conn *websocket.Conn
}

func (r *wsReader) Read(p []byte) (n int, err error) {
	_, data, err := r.conn.ReadMessage()
	if err != nil {
		return 0, err
	}
	return copy(p, data), nil
}

type wsWriter struct {
	conn *websocket.Conn
	typ  int
}

func (w *wsWriter) Write(p []byte) (n int, err error) {
	if err := w.conn.WriteMessage(w.typ, p); err != nil {
		return 0, err
	}
	return len(p), nil
}
