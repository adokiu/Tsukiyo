package system

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// runCommandOutput 执行命令并返回输出（带超时）
func runCommandOutput(timeout time.Duration, name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil && len(out) == 0 {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// commandExists 检查命令是否存在
func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// fileExists 检查文件是否存在
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// readFirstExistingFile 读取第一个存在的文件内容
func readFirstExistingFile(paths ...string) string {
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data)
		}
	}
	return ""
}

// readUintFile 读取文件内容并解析为 uint64
func readUintFile(path string) uint64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	value, _ := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	return value
}

// shellQuoteSimple 简单的 shell 引号转义
func shellQuoteSimple(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

// firstUintText 提取字符串中第一段连续数字
func firstUintText(value string) string {
	start := -1
	for i, ch := range value {
		if ch >= '0' && ch <= '9' {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			return value[start:i]
		}
	}
	if start >= 0 {
		return value[start:]
	}
	return ""
}

// formatBytesText 格式化字节数为人类可读
func formatBytesText(value uint64) string {
	if value == 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	next := float64(value)
	index := 0
	for next >= 1024 && index < len(units)-1 {
		next /= 1024
		index++
	}
	if index == 0 {
		return strconv.FormatUint(value, 10) + " " + units[index]
	}
	return strconv.FormatFloat(next, 'f', 1, 64) + " " + units[index]
}

// boolDetail 返回布尔值的文本描述
func boolDetail(ok bool) string {
	if ok {
		return "available"
	}
	return "missing"
}

// firstNonEmpty 返回第一个非空、非 inactive、非 unknown 的字符串
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		v := strings.TrimSpace(value)
		if v != "" && v != "inactive" && v != "unknown" {
			return v
		}
	}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
