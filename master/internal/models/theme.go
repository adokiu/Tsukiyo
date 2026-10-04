package models

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

const (
	ThemeConfigurationManaged  = "managed"
	ThemeConfigurationRaw      = "raw"
	ThemeConfigurationRedirect = "redirect"
)

// Theme 主题信息
type Theme struct {
	Name          string        `json:"name"`
	Short         string        `json:"short"`
	Description   string        `json:"description"`
	Version       string        `json:"version"`
	Author        string        `json:"author"`
	URL           string        `json:"url"`
	Preview       string        `json:"preview"`
	Configuration Configuration `json:"configuration"`
}

// Configuration 主题配置声明
type Configuration struct {
	Type  string            `json:"type"`
	Icon  string            `json:"icon"`
	Name  any               `json:"name"`
	Data  any               `json:"data"`
	Pages []ThemeConfigPage `json:"pages"`
}

// ManagedThemeConfigItem 托管模式配置项
type ManagedThemeConfigItem struct {
	Key      string `json:"key"`
	Name     any    `json:"name"`
	Required bool   `json:"required"`
	Type     string `json:"type"`
	Options  string `json:"options"`
	Default  any    `json:"default"`
	Help     any    `json:"help"`
}

// ThemeConfigPage 主题配置页面分组
type ThemeConfigPage struct {
	Key   string                   `json:"key"`
	Name  any                      `json:"name"`
	Icon  string                   `json:"icon"`
	Items []ManagedThemeConfigItem `json:"items"`
}

// ThemeConfiguration 主题配置数据（存储在数据库中）
type ThemeConfiguration struct {
	Short string `json:"short" gorm:"primaryKey;unique;not null"`
	Data  string `json:"data" gorm:"type:text;default:'{}'"`
}

func (ThemeConfiguration) TableName() string {
	return "theme_configurations"
}

func (t Theme) ConfigurationType() string {
	typ := strings.ToLower(strings.TrimSpace(t.Configuration.Type))
	if typ == "" {
		return ThemeConfigurationManaged
	}
	return typ
}

func (t Theme) RawHTML() (string, bool) {
	if t.ConfigurationType() != ThemeConfigurationRaw {
		return "", false
	}
	return configurationDataString(t.Configuration.Data)
}

func (t Theme) RedirectTarget() (string, bool) {
	if t.ConfigurationType() != ThemeConfigurationRedirect {
		return "", false
	}
	return NormalizeThemeRedirectTarget(t.Configuration.Data)
}

func (t Theme) ValidateConfiguration() error {
	switch t.ConfigurationType() {
	case ThemeConfigurationManaged:
		return nil
	case ThemeConfigurationRaw:
		html, ok := t.RawHTML()
		if !ok || strings.TrimSpace(html) == "" {
			return fmt.Errorf("raw 类型主题需要在 configuration.data 中提供 HTML 字符串")
		}
		return nil
	case ThemeConfigurationRedirect:
		if _, ok := t.RedirectTarget(); !ok {
			return fmt.Errorf("redirect 类型主题需要在 configuration.data 中提供站内相对路径")
		}
		return nil
	default:
		return fmt.Errorf("不支持的主题类型: %s", t.Configuration.Type)
	}
}

func configurationDataString(data any) (string, bool) {
	if data == nil {
		return "", false
	}
	switch v := data.(type) {
	case string:
		return v, true
	case fmt.Stringer:
		return v.String(), true
	default:
		return fmt.Sprintf("%v", v), true
	}
}

func NormalizeThemeRedirectTarget(data any) (string, bool) {
	target, ok := configurationDataString(data)
	if !ok {
		return "", false
	}

	target = strings.TrimSpace(target)
	if target == "" || strings.Contains(target, "\\") || strings.HasPrefix(target, "//") {
		return "", false
	}

	parsed, err := url.Parse(target)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return "", false
	}

	cleanInputPath := parsed.Path
	if strings.HasPrefix(cleanInputPath, "/") {
		cleanInputPath = strings.TrimLeft(cleanInputPath, "/")
	} else {
		for strings.HasPrefix(cleanInputPath, "../") {
			cleanInputPath = strings.TrimPrefix(cleanInputPath, "../")
		}
	}

	for _, segment := range strings.Split(cleanInputPath, "/") {
		if segment == ".." {
			return "", false
		}
	}

	cleanPath := cleanInputPath
	if cleanPath == "" {
		cleanPath = "/"
	} else {
		cleanPath = path.Clean(cleanPath)
		if cleanPath == "." {
			cleanPath = "/"
		} else {
			cleanPath = "/" + strings.TrimPrefix(cleanPath, "/")
		}
	}

	normalized := url.URL{
		Path:     cleanPath,
		RawQuery: parsed.RawQuery,
		Fragment: parsed.Fragment,
	}
	return normalized.String(), true
}
