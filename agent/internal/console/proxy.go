package console

import (
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
	"go.uber.org/zap"
)

// Session 控制台会话（通过 WS 流式转发）
type Session struct {
	id        string
	container string
	cmd       *exec.Cmd
	ptyFile   *os.File
	sendFunc  func(msgType string, data []byte)
	done      chan struct{}
}

// Handler 控制台处理器（通过 WS 消息处理，不监听任何端口）
type Handler struct {
	sessions sync.Map // sessionID -> *Session
}

// NewHandler 创建控制台处理器
func NewHandler() *Handler {
	return &Handler{}
}

// findShell 检测容器内可用的 shell（bash 优先，fallback 到 sh）
func findShell(container string) string {
	for _, sh := range []string{"/bin/bash", "/bin/sh"} {
		check := exec.Command("incus", "exec", container, "--", "test", "-f", sh)
		if check.Run() == nil {
			return sh
		}
	}
	return "/bin/sh"
}

// StartSSH 启动 SSH 控制台会话
func (h *Handler) StartSSH(sessionID, container string, sendFunc func(msgType string, data []byte)) error {
	shell := findShell(container)
	cmd := exec.Command("incus", "exec", "-t", container, "--", shell)
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")

	// 使用 creack/pty 创建 PTY，使 incus exec 的 stdin/stdout 连接到 PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return err
	}

	sess := &Session{
		id:        sessionID,
		container: container,
		cmd:       cmd,
		ptyFile:   ptmx,
		sendFunc:  sendFunc,
		done:      make(chan struct{}),
	}
	h.sessions.Store(sessionID, sess)

	// 转发 PTY 输出
	go func() {
		buf := make([]byte, 8192)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				h.sendConsoleData(sessionID, "stdout", buf[:n])
			}
			if err != nil {
				break
			}
		}
	}()

	// 等待进程结束
	go func() {
		cmd.Wait()
		h.sendConsoleData(sessionID, "exit", nil)
		h.RemoveSession(sessionID)
	}()

	zap.L().Info("控制台会话已启动", zap.String("session_id", sessionID), zap.String("container", container))
	return nil
}

// WriteInput 向控制台会话写入输入数据
func (h *Handler) WriteInput(sessionID string, data []byte) error {
	val, ok := h.sessions.Load(sessionID)
	if !ok {
		return nil
	}
	sess := val.(*Session)
	_, err := sess.ptyFile.Write(data)
	return err
}

// ResizeSession 调整控制台会话的 PTY 窗口大小
func (h *Handler) ResizeSession(sessionID string, cols, rows int) error {
	val, ok := h.sessions.Load(sessionID)
	if !ok {
		return nil
	}
	sess := val.(*Session)
	return pty.Setsize(sess.ptyFile, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}

// RemoveSession 移除并关闭控制台会话
func (h *Handler) RemoveSession(sessionID string) {
	val, ok := h.sessions.LoadAndDelete(sessionID)
	if !ok {
		return
	}
	sess := val.(*Session)
	if sess.ptyFile != nil {
		sess.ptyFile.Close()
	}
	if sess.cmd != nil && sess.cmd.Process != nil {
		sess.cmd.Process.Kill()
	}
	close(sess.done)
}

// sendConsoleData 发送控制台数据消息
func (h *Handler) sendConsoleData(sessionID, stream string, data []byte) {
	val, ok := h.sessions.Load(sessionID)
	if !ok {
		return
	}
	sess := val.(*Session)
	sess.sendFunc(stream, data)
}
