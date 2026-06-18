package console

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"tsukiyo/agent/internal/config"
)

// Proxy 控制台代理
type Proxy struct {
	cfg        *config.Config
	upgrader   websocket.Upgrader
	sshUpgrader websocket.Upgrader
}

// NewProxy 创建控制台代理
func NewProxy(cfg *config.Config) *Proxy {
	return &Proxy{
		cfg: cfg,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		sshUpgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
			Subprotocols: []string{"binary"},
		},
	}
}

// ServeHTTP 启动 HTTP 服务
func (p *Proxy) ServeHTTP() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/console/ssh", p.handleWebSSH)
	mux.HandleFunc("/console/vnc", p.handleWebVNC)

	addr := p.cfg.ConsoleBindAddr()
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	zap.L().Info("控制台代理启动", zap.String("addr", addr))
	return http.ListenAndServe(addr, mux)
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

	// 通过 incus exec 启动 shell
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

	// 转发数据
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			stdin.Write(data)
		}
	}()

	// 合并 stdout 和 stderr
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

	// VNC 通常通过 noVNC 或类似工具实现
	// 这里提供基础代理：连接到本地 VNC 端口
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

	// 连接到本地 VNC 服务器
	vncAddr := fmt.Sprintf("127.0.0.1:%d", vncPort)
	vncConn, err := net.Dial("tcp", vncAddr)
	if err != nil {
		zap.L().Error("连接 VNC 失败", zap.Error(err))
		return
	}
	defer vncConn.Close()

	// 双向转发
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

// wsReader WebSocket 读取器
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

// wsWriter WebSocket 写入器
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

// GetConsoleAddr 获取控制台监听地址
func GetConsoleAddr(cfg *config.Config) string {
	addr := cfg.ConsoleBindAddr()
	// 如果 bind addr 是 127.0.0.1:0，需要获取实际端口
	if strings.HasSuffix(addr, ":0") {
		// 由 ServeHTTP 动态分配
		return addr
	}
	return addr
}
