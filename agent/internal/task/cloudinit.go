package task

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// buildCloudInitNetworkConfig 生成 cloud-init network-config YAML，配置静态 IP
func buildCloudInitNetworkConfig(internalIPv4, gatewayV4, ipv4CIDR string) string {
	mask := "24"
	if ipv4CIDR != "" {
		if _, ipNet, err := net.ParseCIDR(ipv4CIDR); err == nil {
			ones, _ := ipNet.Mask.Size()
			mask = strconv.Itoa(ones)
		}
	}
	var lines []string
	lines = append(lines, "version: 2")
	lines = append(lines, "ethernets:")
	lines = append(lines, "  eth0:")
	lines = append(lines, "    addresses:")
	lines = append(lines, fmt.Sprintf("      - %s/%s", internalIPv4, mask))
	lines = append(lines, fmt.Sprintf("    routes:"))
	lines = append(lines, "      - to: default")
	lines = append(lines, fmt.Sprintf("        via: %s", gatewayV4))
	lines = append(lines, "    dhcp4: false")
	lines = append(lines, "    dhcp6: false")
	return strings.Join(lines, "\n")
}

// buildCloudInitUserData 生成 cloud-init user-data YAML，预配置 root 密码、SSH 公钥和网络
func buildCloudInitUserData(password, publicKey, loginMethod, internalIPv4, gatewayV4, ipv4CIDR string) string {
	_ = loginMethod
	_, _, _ = internalIPv4, gatewayV4, ipv4CIDR
	var lines []string
	lines = append(lines, "#cloud-config")
	lines = append(lines, "users:")
	lines = append(lines, "  - name: root")
	lines = append(lines, "    lock_passwd: false")
	if publicKey != "" {
		lines = append(lines, "    ssh_authorized_keys:")
		lines = append(lines, fmt.Sprintf("      - %s", publicKey))
	}
	if password != "" {
		lines = append(lines, "chpasswd:")
		lines = append(lines, "  list: |")
		lines = append(lines, fmt.Sprintf("    root:%s", password))
		lines = append(lines, "  expire: false")
	}
	lines = append(lines, "ssh_pwauth: true")
	lines = append(lines, "disable_root: false")

	// 清除所有旧 motd（bootcmd 在 write_files 之前执行，兼容所有发行版）
	lines = append(lines, "bootcmd:")
	lines = append(lines, "  - rm -f /etc/motd")
	lines = append(lines, "  - touch /etc/motd")
	lines = append(lines, "  - rm -rf /etc/update-motd.d/* 2>/dev/null || true")
	lines = append(lines, "  - rm -f /etc/profile.d/*motd* 2>/dev/null || true")

	lines = append(lines, "write_files:")
	lines = append(lines, "  - path: /etc/profile.d/tsukiyo-motd.sh")
	lines = append(lines, "    permissions: '0755'")
	lines = append(lines, "    content: |")
	lines = append(lines, "      #!/bin/sh")
	lines = append(lines, "      case \"$-\" in *i*) ;; *) return 0 ;; esac")
	lines = append(lines, "      [ -n \"$SSH_CONNECTION\" ] || return 0")
	lines = append(lines, "      [ -n \"$TSUKIYO_MOTD_SHOWN\" ] && return 0")
	lines = append(lines, "      export TSUKIYO_MOTD_SHOWN=1")
	lines = append(lines, "      echo")
	lines = append(lines, "      printf \"\\033[38;5;196m        ,----,                                                              \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;202m      ,/   .\\`|                                                              \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;208m    ,\\`   .'  :                              ,-.                             \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;214m  ;    ;     /                          ,--/ /|   ,--,                      \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;220m.'___,/    ,'                    ,--, ,--. :/ | ,--.'|              ,---.   \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;226m|    :     |  .--.--.          ,'_ /| :  : ' /  |  |,              '   ,'\\\\  \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;154m;    |.';  ; /  /    '    .--. |  | : |  '  /   \\`--'_        .--, /   /   | \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;118m\\`----'  |  ||  :  /\\`./  ,'_ /| :  . | '  |   \\\\  '  | |  , ' , ' :'   | |: | \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;82m    '   :  ;|  :  ;_    |  ' | |  . . |  |   \\\\  '  : | /___/ \\: |'   | .; : \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;46m    |   |  ' \\\\  \\\\    \\`. |  | : ;  ; | |  | ' \\\\ \\`  : |__.  \\\\  ' ||   :    | \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;47m    '   :  |  \\`----.   \\\\:  | : ;  ; | |  | |. \\\\ |  | '.'|\\\\  ;   : \\\\   \\\\  /  \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;48m    ;   |.'  /  /\\`--'  /'  :  \\`--'   \\`'  : |--' |  |    ; \\\\  \\\\  ;  \\`----'   \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;49m    '---'   '--'.     / :  ,      .-./;  |,'    ;  :    ; \\\\  \\\\  :  \\`----'    \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;50m              \\`--'---'   \\`--\\`----'    '--'      |  ,   /   :  \\\\  \\\\          \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;51m                                                 ---\\`-'     \\\\  ' ;          \\033[0m\\n\"")
	lines = append(lines, "      printf \"\\033[38;5;87m                                                             \\`--\\`           \\033[0m\\n\"")
	lines = append(lines, "      echo")
	lines = append(lines, "      echo \"Tsukiyo Virtualization System By aDokiu\"")
	lines = append(lines, "      echo \"Github       : https://github.com/adokiu/Tsukiyo\"")
	lines = append(lines, "      echo")
	lines = append(lines, "      echo \"Distribution : $(cat /etc/os-release 2>/dev/null | grep PRETTY_NAME | cut -d= -f2 | tr -d '\"' || echo 'Linux')\"")
	lines = append(lines, "      echo \"Kernel       : $(uname -r)\"")
	lines = append(lines, "      echo")

	return strings.Join(lines, "\n")
}

// buildMotdScript 生成清除旧 motd 并注入 Tsukiyo motd 的 shell 脚本
func buildMotdScript() string {
	return "cat > /etc/profile.d/tsukiyo-motd.sh << 'MOTDEOF'\n" +
		"#!/bin/sh\n" +
		"case \"$-\" in *i*) ;; *) return 0 ;; esac\n" +
		"[ -n \"$SSH_CONNECTION\" ] || return 0\n" +
		"[ -n \"$TSUKIYO_MOTD_SHOWN\" ] && return 0\n" +
		"export TSUKIYO_MOTD_SHOWN=1\n" +
		"echo\n" +
		"printf \"\\033[38;5;196m        ,----,                                                              \\033[0m\\n\"\n" +
		"printf \"\\033[38;5;202m      ,/   .`|                                                              \\033[0m\\n\"\n" +
		"printf \"\\033[38;5;208m    ,`   .'  :                              ,-.                             \\033[0m\\n\"\n" +
		"printf \"\\033[38;5;214m  ;    ;     /                          ,--/ /|   ,--,                      \\033[0m\\n\"\n" +
		"printf \"\\033[38;5;220m.'___,/    ,'                    ,--, ,--. :/ | ,--.'|              ,---.   \\033[0m\\n\"\n" +
		"printf \"\\033[38;5;226m|    :     |  .--.--.          ,'_ /| :  : ' /  |  |,              '   ,'\\   \\033[0m\\n\"\n" +
		"printf \"\\033[38;5;154m;    |.';  ; /  /    '    .--. |  | : |  '  /   `--'_        .--, /   /   | \\033[0m\\n\"\n" +
		"printf \"\\033[38;5;118m`----'  |  ||  :  /`./  ,'_ /| :  . | '  |   \\\\  '  | |  , ' , ' :'   | |: | \\033[0m\\n\"\n" +
		"printf \"\\033[38;5;82m    '   :  ;|  :  ;_    |  ' | |  . . |  |   \\\\  '  : | /___/ \\: |'   | .; : \\033[0m\\n\"\n" +
		"printf \"\\033[38;5;46m    |   |  ' \\\\  \\\\    `. |  | : ;  ; | |  | ' \\\\ `  : |__.  \\\\  ' ||   :    | \\033[0m\\n\"\n" +
		"printf \"\\033[38;5;47m    '   :  |  `----.   \\:  | : ;  ; | |  | |. \\\\ |  | '.'|\\\\  ;   : \\\\   \\\\  /  \\033[0m\\n\"\n" +
		"printf \"\\033[38;5;48m    ;   |.'  /  /`--'  /'  :  `--'   `'  : |--' |  |    ; \\\\  \\\\  ;  `----'   \\033[0m\\n\"\n" +
		"printf \"\\033[38;5;49m    '---'   '--'.     / :  ,      .-./;  |,'    ;  :    ; \\\\  \\\\  :  `----'    \\033[0m\\n\"\n" +
		"printf \"\\033[38;5;50m              `--'---'   `--`----'    '--'      |  ,   /   :  \\\\  \\\\          \\033[0m\\n\"\n" +
		"printf \"\\033[38;5;51m                                                 ---`-'     \\\\  ' ;          \\033[0m\\n\"\n" +
		"printf \"\\033[38;5;87m                                                             `--`           \\033[0m\\n\"\n" +
		"echo\n" +
		"echo \"Tsukiyo Virtualization System By aDokiu\"\n" +
		"echo \"Github       : https://github.com/adokiu/Tsukiyo\"\n" +
		"echo\n" +
		"echo \"Distribution : $(cat /etc/os-release 2>/dev/null | grep PRETTY_NAME | cut -d= -f2 | tr -d '\"' || echo 'Linux')\"\n" +
		"echo \"Kernel       : $(uname -r)\"\n" +
		"echo\n" +
		"MOTDEOF\n" +
		"chmod 755 /etc/profile.d/tsukiyo-motd.sh\n"
}

// isSpiritlhlSource 判断镜像源是否为 spiritlhl（预装 SSH）
// imageSource 为空时默认为 spiritlhl（因为默认源已改为 spiritlhl）
func isSpiritlhlSource(imageSource string) bool {
	s := strings.TrimSpace(imageSource)
	s = strings.TrimSuffix(s, ":")
	if s == "" || s == "spiritlhl" {
		return true
	}
	return false
}
