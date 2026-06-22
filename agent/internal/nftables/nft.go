// Package nftables 封装 nftables 规则操作，使用 inet tsukiyo 表统一管理所有 IPv4/IPv6 规则
package nftables

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"go.uber.org/zap"
)

const (
	// tableName nftables 表名
	tableName = "tsukiyo"
	// rulesFile 持久化规则文件路径
	rulesFile = "/etc/tsukiyo/nftables.rules"
)

// chains 所有需要创建的链及其 hook 配置
var chains = []struct {
	Name     string
	Type     string
	Hook     string
	Priority int
}{
	{"prerouting", "nat", "prerouting", -100},
	{"postrouting", "nat", "postrouting", 100},
	{"forward", "filter", "forward", 0},
	{"input", "filter", "input", 0},
	{"output", "filter", "output", 0},
}

// EnsureTable 确保 inet tsukiyo 表和所有链存在（幂等）
func EnsureTable() {
	// 创建表
	exec.Command("nft", "add", "table", "inet", tableName).Run()

	// 创建每条链
	for _, ch := range chains {
		// nft add chain 语法包含花括号和分号，需通过 sh -c 执行
		cmdStr := fmt.Sprintf("nft add chain inet %s %s '{ type %s hook %s priority %d ; }'",
			tableName, ch.Name, ch.Type, ch.Hook, ch.Priority)
		exec.Command("sh", "-c", cmdStr).Run()
	}
}

// AddRule 添加规则到指定链（带 comment 标记）
func AddRule(chain, rule, comment string) error {
	fullRule := fmt.Sprintf("%s comment \"%s\"", rule, comment)
	out, err := exec.Command("nft", "add", "rule", "inet", tableName, chain, fullRule).CombinedOutput()
	if err != nil {
		return fmt.Errorf("添加 nftables 规则失败: chain=%s rule=%s err=%w output=%s", chain, rule, err, string(out))
	}
	SaveRules()
	return nil
}

// AddRuleSilent 添加规则（忽略错误，持久化）
func AddRuleSilent(chain, rule, comment string) {
	fullRule := fmt.Sprintf("%s comment \"%s\"", rule, comment)
	exec.Command("nft", "add", "rule", "inet", tableName, chain, fullRule).Run()
	SaveRules()
}

// DeleteRuleByComment 删除指定链中 comment 完全匹配的 nftables 规则
func DeleteRuleByComment(chain, comment string) {
	deleteRulesByCommentMatch(chain, comment, false)
}

// DeleteRulesByCommentPrefix 删除指定链中 comment 以 prefix 开头的所有 nftables 规则
func DeleteRulesByCommentPrefix(chain, prefix string) {
	deleteRulesByCommentMatch(chain, prefix, true)
}

// deleteRulesByCommentMatch 内部实现：通过解析 nft -a list chain 输出获取 handle 并删除
func deleteRulesByCommentMatch(chain, match string, prefixMatch bool) {
	out, err := exec.Command("nft", "-a", "list", "chain", "inet", tableName, chain).Output()
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	handleRe := regexp.MustCompile(`handle (\d+)`)

	for scanner.Scan() {
		line := scanner.Text()
		// 检查行中是否包含匹配的 comment
		commentMatched := false
		if prefixMatch {
			commentRe := regexp.MustCompile(`comment "` + regexp.QuoteMeta(match))
			commentMatched = commentRe.MatchString(line)
		} else {
			commentMatched = strings.Contains(line, fmt.Sprintf("comment \"%s\"", match))
		}

		if commentMatched {
			m := handleRe.FindStringSubmatch(line)
			if len(m) >= 2 {
				exec.Command("nft", "delete", "rule", "inet", tableName, chain, "handle", m[1]).Run()
			}
		}
	}
	SaveRules()
}

// FlushChain 清空指定链的所有规则
func FlushChain(chain string) {
	exec.Command("nft", "flush", "chain", "inet", tableName, chain).Run()
	SaveRules()
}

// NftRule 表示一条 nftables 规则
type NftRule struct {
	Handle  int
	Comment string
	Rule    string
}

// ListRules 列出链中所有规则（含 handle 和 comment）
func ListRules(chain string) ([]NftRule, error) {
	out, err := exec.Command("nft", "-a", "list", "chain", "inet", tableName, chain).Output()
	if err != nil {
		return nil, fmt.Errorf("列出 nftables 规则失败: chain=%s err=%w", chain, err)
	}

	var rules []NftRule
	handleRe := regexp.MustCompile(`handle (\d+)`)
	commentRe := regexp.MustCompile(`comment "([^"]*)"`)

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "handle") {
			continue
		}
		rule := NftRule{}
		if m := handleRe.FindStringSubmatch(line); len(m) >= 2 {
			fmt.Sscanf(m[1], "%d", &rule.Handle)
		}
		if m := commentRe.FindStringSubmatch(line); len(m) >= 2 {
			rule.Comment = m[1]
		}
		rule.Rule = strings.TrimSpace(line)
		rules = append(rules, rule)
	}
	return rules, nil
}

// RuleExists 检查指定链中是否存在 comment 匹配的规则
func RuleExists(chain, comment string) bool {
	rules, err := ListRules(chain)
	if err != nil {
		return false
	}
	for _, r := range rules {
		if r.Comment == comment {
			return true
		}
	}
	return false
}

// RuleExistsByPrefix 检查指定链中是否存在 comment 以 prefix 开头的规则
func RuleExistsByPrefix(chain, prefix string) bool {
	rules, err := ListRules(chain)
	if err != nil {
		return false
	}
	for _, r := range rules {
		if strings.HasPrefix(r.Comment, prefix) {
			return true
		}
	}
	return false
}

// SaveRules 将当前 inet tsukiyo 表的规则持久化到文件
func SaveRules() {
	// 确保目录存在
	if err := os.MkdirAll("/etc/tsukiyo", 0755); err != nil {
		zap.L().Warn("[Nftables] 创建 /etc/tsukiyo 目录失败", zap.Error(err))
		return
	}

	// 导出 tsukiyo 表规则
	out, err := exec.Command("nft", "list", "table", "inet", tableName).Output()
	if err != nil {
		zap.L().Warn("[Nftables] 导出规则失败", zap.Error(err))
		return
	}

	if err := os.WriteFile(rulesFile, out, 0644); err != nil {
		zap.L().Warn("[Nftables] 写入规则文件失败", zap.Error(err))
		return
	}
	zap.L().Debug("[Nftables] 规则已持久化", zap.String("file", rulesFile))
}

// LoadRules 从持久化文件加载规则
func LoadRules() error {
	if _, err := os.Stat(rulesFile); os.IsNotExist(err) {
		zap.L().Info("[Nftables] 规则文件不存在，跳过加载", zap.String("file", rulesFile))
		return nil
	}

	// 先删除现有表（忽略错误，可能不存在），然后从文件重建
	// nft -f 加载包含 table 定义的文件时，如果表已存在会失败
	exec.Command("nft", "delete", "table", "inet", tableName).Run()

	// 通过 nft -f 加载规则文件
	cmd := exec.Command("nft", "-f", rulesFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		// 加载失败时重新确保表和链存在
		EnsureTable()
		return fmt.Errorf("加载 nftables 规则失败: %w output=%s", err, string(out))
	}
	zap.L().Info("[Nftables] 规则已从文件加载", zap.String("file", rulesFile))
	return nil
}

// EnsureSystemdService 创建 systemd service 确保宿主机重启后规则自动恢复
func EnsureSystemdService() {
	// 动态获取 nft 路径
	nftPath, err := exec.LookPath("nft")
	if err != nil {
		nftPath = "/usr/sbin/nft"
	}

	serviceContent := fmt.Sprintf(`[Unit]
Description=Tsukiyo nftables rules loader
After=network.target

[Service]
Type=oneshot
ExecStart=%s -f /etc/tsukiyo/nftables.rules
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target`, nftPath)

	servicePath := "/etc/systemd/system/tsukiyo-nftables.service"
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		zap.L().Warn("[Nftables] 写入 systemd service 失败", zap.Error(err))
		return
	}

	exec.Command("systemctl", "daemon-reload").Run()
	exec.Command("systemctl", "enable", "tsukiyo-nftables.service").Run()
	zap.L().Info("[Nftables] systemd service 已创建并启用")
}
