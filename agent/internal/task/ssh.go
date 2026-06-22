package task

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// injectMotdAndSSHConfig 配置 SSH 允许密码/root 登录并注入 motd，不安装 SSH。用于 VM/cloud 镜像（已预装 SSH）
func injectMotdAndSSHConfig(instanceName, loginMethod string) error {
	zap.L().Info("等待容器 exec 可用", zap.String("instance", instanceName))
	for i := 0; i < 60; i++ {
		if exec.Command("incus", "exec", instanceName, "--", "true").Run() == nil {
			zap.L().Info("容器 exec 已可用", zap.String("instance", instanceName), zap.Int("wait_seconds", i))
			break
		}
		if i == 59 {
			return fmt.Errorf("容器 exec 60 秒内不可用")
		}
		if i == 0 || i%5 == 0 {
			zap.L().Info("容器 exec 尚未就绪", zap.String("instance", instanceName), zap.Int("wait_seconds", i))
		}
		time.Sleep(1 * time.Second)
	}

	pwAuth := "no"
	if loginMethod == "password" || loginMethod == "auto" || loginMethod == "" {
		pwAuth = "yes"
	}

	script := `set -eu
log() { echo "[tsukiyo-ssh-config] $*"; }

log "配置 sshd 允许 root 登录和密码登录"
if [ -f /etc/ssh/sshd_config ]; then
    sed -i '/^PasswordAuthentication/d' /etc/ssh/sshd_config
    sed -i '/^PermitRootLogin/d' /etc/ssh/sshd_config
    sed -i '/^PubkeyAuthentication/d' /etc/ssh/sshd_config
    sed -i '/^UsePAM/d' /etc/ssh/sshd_config
    echo 'PermitRootLogin yes' >> /etc/ssh/sshd_config
    echo 'PasswordAuthentication ` + pwAuth + `' >> /etc/ssh/sshd_config
    echo 'PubkeyAuthentication yes' >> /etc/ssh/sshd_config
    echo 'UsePAM yes' >> /etc/ssh/sshd_config
    log "sshd_config 已更新"
else
    log "WARNING: /etc/ssh/sshd_config 不存在，跳过 SSH 配置"
fi

log "重启 sshd 服务"
if command -v systemctl >/dev/null 2>&1; then
    systemctl restart ssh 2>/dev/null || systemctl restart sshd 2>/dev/null || true
fi
service ssh restart 2>/dev/null || service sshd restart 2>/dev/null || true

log "清除旧 motd 并注入 Tsukiyo motd"
rm -f /etc/motd
touch /etc/motd
rm -rf /etc/update-motd.d/* 2>/dev/null || true
rm -f /etc/profile.d/*motd* 2>/dev/null || true
` + buildMotdScript() + `
log "motd 注入完成"
`
	if err := runIncusExecScript(instanceName, script); err != nil {
		zap.L().Error("SSH 配置和 motd 注入失败", zap.String("instance", instanceName), zap.Error(err))
		return err
	}
	zap.L().Info("SSH 配置和 motd 注入完成", zap.String("instance", instanceName))
	return nil
}

// ensureSSHInContainer 通过 incus exec 安装、配置、启动 sshd
func ensureSSHInContainer(instanceName, loginMethod, gatewayV4 string) error {
	gw := gatewayV4
	if idx := strings.Index(gw, "/"); idx > 0 {
		gw = gw[:idx]
	}
	zap.L().Info("等待容器 exec 可用", zap.String("instance", instanceName))
	for i := 0; i < 60; i++ {
		if exec.Command("incus", "exec", instanceName, "--", "true").Run() == nil {
			zap.L().Info("容器 exec 已可用", zap.String("instance", instanceName), zap.Int("wait_seconds", i))
			break
		}
		if i == 59 {
			return fmt.Errorf("容器 exec 60 秒内不可用")
		}
		if i == 0 || i%5 == 0 {
			zap.L().Info("容器 exec 尚未就绪", zap.String("instance", instanceName), zap.Int("wait_seconds", i))
		}
		time.Sleep(1 * time.Second)
	}

	script := buildSSHSetupScript(loginMethod, gw)
	zap.L().Info("开始执行 SSH 安装脚本", zap.String("instance", instanceName), zap.String("gateway", gw))
	if err := runIncusExecScript(instanceName, script); err != nil {
		zap.L().Error("SSH 安装脚本失败", zap.String("instance", instanceName), zap.Error(err))
		return err
	}
	zap.L().Info("SSH 安装脚本完成", zap.String("instance", instanceName))
	return nil
}

// runIncusExecScript 执行 incus exec 脚本并实时输出日志
func runIncusExecScript(instanceName, script string) error {
	cmd := exec.Command("incus", "exec", instanceName, "--", "sh", "-c", script)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("获取 stdout 失败: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("获取 stderr 失败: %w", err)
	}

	var wg sync.WaitGroup
	logReader := func(r io.Reader, stream string) {
		defer wg.Done()
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			line := sc.Text()
			if stream == "stderr" {
				zap.L().Warn("exec stderr", zap.String("instance", instanceName), zap.String("line", line))
			} else {
				zap.L().Info("exec stdout", zap.String("instance", instanceName), zap.String("line", line))
			}
		}
	}
	wg.Add(2)
	go logReader(stdout, "stdout")
	go logReader(stderr, "stderr")

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 exec 失败: %w", err)
	}
	zap.L().Info("exec 进程已启动", zap.String("instance", instanceName))

	err = cmd.Wait()
	wg.Wait()
	if err != nil {
		return fmt.Errorf("exec 退出码非零: %w", err)
	}
	return nil
}

func buildSSHSetupScript(loginMethod, gatewayV4 string) string {
	pwAuth := "no"
	if loginMethod == "password" || loginMethod == "auto" || loginMethod == "" {
		pwAuth = "yes"
	}
	gw := gatewayV4
	if idx := strings.Index(gw, "/"); idx > 0 {
		gw = gw[:idx]
	}

	return `set -eu
GW="` + gw + `"
log() { echo "[tsukiyo-ssh] $*"; }

log "step 1/5: 修复 DNS, gateway=$GW"
if [ -L /etc/resolv.conf ] 2>/dev/null; then rm -f /etc/resolv.conf; fi
: > /etc/resolv.conf
[ -n "$GW" ] && echo "nameserver $GW" >> /etc/resolv.conf
echo "nameserver 8.8.8.8" >> /etc/resolv.conf
echo "nameserver 1.1.1.1" >> /etc/resolv.conf
cat /etc/resolv.conf

log "step 2/5: 等待网络就绪"
for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
    if ping -c1 -W2 8.8.8.8 >/dev/null 2>&1; then log "网络就绪 (ping 8.8.8.8)"; break; fi
    if [ -n "$GW" ] && ping -c1 -W2 "$GW" >/dev/null 2>&1; then log "网络就绪 (ping gateway)"; break; fi
    log "等待网络... ($i/15)"
    sleep 2
done

has_sshd() { command -v sshd >/dev/null 2>&1 || [ -x /usr/sbin/sshd ]; }

log "step 3/5: 安装 openssh"
if has_sshd; then
    log "sshd 已存在，跳过安装"
else
    if command -v apt-get >/dev/null 2>&1; then
        log "使用 apt-get 安装(不 update)"; DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends openssh-server
    elif command -v apk >/dev/null 2>&1; then
        log "使用 apk 安装(不 update)"; apk add --no-cache --no-network openssh 2>/dev/null || apk add --no-cache openssh
    elif command -v pacman >/dev/null 2>&1; then
        log "使用 pacman 安装(不 sync)"; pacman -S --noconfirm openssh
    elif command -v dnf >/dev/null 2>&1; then
        log "使用 dnf 安装(不 update)"; dnf install -y -C openssh-server
    elif command -v yum >/dev/null 2>&1; then
        log "使用 yum 安装(不 update)"; yum install -y -C openssh-server
    elif command -v zypper >/dev/null 2>&1; then
        log "使用 zypper 安装(不 refresh)"; zypper install -y --no-refresh openssh
    else
        log "ERROR: 不支持的发行版"; exit 1
    fi
fi

log "step 4/5: 配置并启动 sshd"
mkdir -p /run/sshd /var/run/sshd
ssh-keygen -A 2>/dev/null || true

sed -i '/^PasswordAuthentication/d' /etc/ssh/sshd_config
sed -i '/^PermitRootLogin/d' /etc/ssh/sshd_config
sed -i '/^PubkeyAuthentication/d' /etc/ssh/sshd_config
sed -i '/^UsePAM/d' /etc/ssh/sshd_config
echo 'PermitRootLogin yes' >> /etc/ssh/sshd_config
echo 'PasswordAuthentication ` + pwAuth + `' >> /etc/ssh/sshd_config
echo 'PubkeyAuthentication yes' >> /etc/ssh/sshd_config
echo 'UsePAM yes' >> /etc/ssh/sshd_config

if command -v rc-update >/dev/null 2>&1; then rc-update add sshd default 2>/dev/null || true; fi
if command -v systemctl >/dev/null 2>&1; then
    systemctl stop ssh.socket 2>/dev/null || true
    systemctl disable ssh.socket 2>/dev/null || true
    systemctl enable --now ssh 2>/dev/null || systemctl enable --now sshd 2>/dev/null || true
fi
service ssh restart 2>/dev/null || service sshd restart 2>/dev/null || rc-service sshd restart 2>/dev/null || true

log "step 5/5: 验证 22 端口"
for i in 1 2 3 4 5 6 7 8 9 10; do
    if (ss -ltn 2>/dev/null || netstat -tln 2>/dev/null) | grep -q ':22[[:space:]]'; then
        log "sshd 已在 22 端口监听"; break
    fi
    if pgrep -x sshd >/dev/null 2>&1; then
        log "sshd 进程已运行"; break
    fi
    log "等待 sshd 启动... ($i/10)"
    sleep 1
done
pgrep -x sshd >/dev/null 2>&1 || { log "尝试直接启动 sshd"; /usr/sbin/sshd -f /etc/ssh/sshd_config 2>/dev/null || sshd -f /etc/ssh/sshd_config; sleep 1; }
pgrep -x sshd >/dev/null 2>&1 || { log "ERROR: sshd 未运行"; exit 1; }
log "sshd 启动成功"

log "step 6/6: 清除旧 motd 并注入 Tsukiyo motd"
rm -f /etc/motd
touch /etc/motd
rm -rf /etc/update-motd.d/* 2>/dev/null || true
rm -f /etc/profile.d/*motd* 2>/dev/null || true
` + buildMotdScript() + `
log "motd 注入完成"
`
}
