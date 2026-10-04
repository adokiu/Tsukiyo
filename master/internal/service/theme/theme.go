package theme

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

const (
	ThemeDir       = "./data/theme"
	DefaultThemeID = "default"
)

// ThemeService 主题管理服务
type ThemeService struct{}

// NewThemeService 创建主题服务
func NewThemeService() *ThemeService {
	return &ThemeService{}
}

// ListThemes 列出所有已安装主题
func (s *ThemeService) ListThemes() ([]models.Theme, error) {
	var themes []models.Theme

	// 默认主题从 embed.FS 读取（由 handler 层处理）
	// 这里读取 ./data/theme/ 目录下的自定义主题
	entries, err := os.ReadDir(ThemeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return themes, nil
		}
		return nil, fmt.Errorf("读取主题目录失败: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		themeConfigPath := filepath.Join(ThemeDir, entry.Name(), "tsukiyo-theme.json")
		theme, err := s.loadThemeConfig(themeConfigPath)
		if err != nil {
			zap.L().Warn("读取主题配置失败", zap.String("path", themeConfigPath), zap.Error(err))
			continue
		}
		themes = append(themes, theme)
	}

	return themes, nil
}

// loadThemeConfig 从文件加载主题配置
func (s *ThemeService) loadThemeConfig(path string) (models.Theme, error) {
	var theme models.Theme
	data, err := os.ReadFile(path)
	if err != nil {
		return theme, err
	}
	if err := json.Unmarshal(data, &theme); err != nil {
		return theme, fmt.Errorf("解析主题配置失败: %w", err)
	}
	return theme, nil
}

// ExtractAndValidateTheme 解压并验证主题 ZIP 包
func (s *ThemeService) ExtractAndValidateTheme(zipPath string) (models.Theme, error) {
	var themeInfo models.Theme

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return themeInfo, fmt.Errorf("无法打开ZIP文件: %v", err)
	}
	defer r.Close()

	// 查找 tsukiyo-theme.json 文件
	var themeConfigFile *zip.File
	for _, f := range r.File {
		if f.Name == "tsukiyo-theme.json" {
			themeConfigFile = f
			break
		}
	}

	if themeConfigFile == nil {
		return themeInfo, fmt.Errorf("主题配置文件 tsukiyo-theme.json 不存在")
	}

	rc, err := themeConfigFile.Open()
	if err != nil {
		return themeInfo, fmt.Errorf("无法读取主题配置文件: %v", err)
	}
	defer rc.Close()

	configData, err := io.ReadAll(rc)
	if err != nil {
		return themeInfo, fmt.Errorf("读取主题配置失败: %v", err)
	}

	if err := json.Unmarshal(configData, &themeInfo); err != nil {
		return themeInfo, fmt.Errorf("主题配置格式错误: %v", err)
	}

	// 验证必填字段
	if themeInfo.Name == "" || themeInfo.Short == "" {
		return themeInfo, fmt.Errorf("主题配置缺少必填字段（name、short）")
	}

	// 验证 Short 字段格式
	if !isValidThemeShort(themeInfo.Short) {
		return themeInfo, fmt.Errorf("主题short字段格式无效，只允许字母、数字、下划线和连字符")
	}

	if err := themeInfo.ValidateConfiguration(); err != nil {
		return themeInfo, err
	}

	// 创建主题目录
	themeDir := filepath.Join(ThemeDir, themeInfo.Short)
	if _, err := os.Stat(themeDir); err == nil {
		if err := os.RemoveAll(themeDir); err != nil {
			return themeInfo, fmt.Errorf("删除原有主题失败: %v", err)
		}
	}

	if err := os.MkdirAll(themeDir, 0755); err != nil {
		return themeInfo, fmt.Errorf("创建主题目录失败: %v", err)
	}

	// 解压文件到主题目录
	for _, f := range r.File {
		path := filepath.Join(themeDir, f.Name)

		// 安全检查，防止路径遍历攻击
		if !strings.HasPrefix(path, filepath.Clean(themeDir)+string(os.PathSeparator)) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.FileInfo().Mode())
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return themeInfo, fmt.Errorf("创建目录失败: %v", err)
		}

		rc, err := f.Open()
		if err != nil {
			return themeInfo, fmt.Errorf("打开压缩文件失败: %v", err)
		}

		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.FileInfo().Mode())
		if err != nil {
			rc.Close()
			return themeInfo, fmt.Errorf("创建文件失败: %v", err)
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return themeInfo, fmt.Errorf("解压文件失败: %v", err)
		}
	}

	return themeInfo, nil
}

// PeekThemeFromZip 仅读取 ZIP 中的主题配置，不解压
func (s *ThemeService) PeekThemeFromZip(zipPath string) (models.Theme, error) {
	var themeInfo models.Theme

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return themeInfo, fmt.Errorf("无法打开ZIP文件: %v", err)
	}
	defer r.Close()

	var themeConfigFile *zip.File
	for _, f := range r.File {
		if f.Name == "tsukiyo-theme.json" {
			themeConfigFile = f
			break
		}
	}

	if themeConfigFile == nil {
		return themeInfo, fmt.Errorf("主题配置文件 tsukiyo-theme.json 不存在")
	}

	rc, err := themeConfigFile.Open()
	if err != nil {
		return themeInfo, fmt.Errorf("无法读取主题配置文件: %v", err)
	}
	defer rc.Close()

	configData, err := io.ReadAll(rc)
	if err != nil {
		return themeInfo, fmt.Errorf("读取主题配置失败: %v", err)
	}

	if err := json.Unmarshal(configData, &themeInfo); err != nil {
		return themeInfo, fmt.Errorf("主题配置格式错误: %v", err)
	}

	return themeInfo, nil
}

// DeleteTheme 删除主题
func (s *ThemeService) DeleteTheme(short string) error {
	if short == DefaultThemeID {
		return fmt.Errorf("默认主题不能删除")
	}

	themeDir := filepath.Join(ThemeDir, short)
	if _, err := os.Stat(themeDir); os.IsNotExist(err) {
		return fmt.Errorf("主题不存在")
	}

	return os.RemoveAll(themeDir)
}

// SetTheme 设置当前主题
func (s *ThemeService) SetTheme(short string) error {
	if short != DefaultThemeID {
		themeConfigPath := filepath.Join(ThemeDir, short, "tsukiyo-theme.json")
		if _, err := os.Stat(themeConfigPath); os.IsNotExist(err) {
			return fmt.Errorf("主题不存在")
		}
	}

	// 更新 site_configs 表中的 theme 字段
	return db.DB.Model(&models.SiteConfig{}).Where("1=1").Update("theme", short).Error
}

// UpdateThemeSettings 更新主题配置
func (s *ThemeService) UpdateThemeSettings(short string, settings map[string]interface{}) error {
	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	var tc models.ThemeConfiguration
	result := db.DB.Where("short = ?", short).Assign(models.ThemeConfiguration{
		Short: short,
		Data:  string(data),
	}).FirstOrCreate(&tc)

	return result.Error
}

// GetThemeSettings 获取主题配置
func (s *ThemeService) GetThemeSettings(short string) (map[string]interface{}, error) {
	var tc models.ThemeConfiguration
	if err := db.DB.Where("short = ?", short).First(&tc).Error; err != nil {
		return make(map[string]interface{}), nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(tc.Data), &result); err != nil {
		return make(map[string]interface{}), nil
	}

	return result, nil
}

// GetThemeConfigWithDefaults 获取主题配置并合并默认值
func (s *ThemeService) GetThemeConfigWithDefaults(short string, defaultConfigJSON []byte) (map[string]interface{}, error) {
	// 1. 从数据库读取已保存的配置
	savedSettings, err := s.GetThemeSettings(short)
	if err != nil {
		savedSettings = make(map[string]interface{})
	}

	// 2. 从主题声明文件读取默认值
	if defaultConfigJSON != nil {
		// 先尝试 pages 格式
		var themeDeclPages struct {
			Configuration struct {
				Type  string                   `json:"type"`
				Pages []models.ThemeConfigPage `json:"pages"`
			} `json:"configuration"`
		}
		if err := json.Unmarshal(defaultConfigJSON, &themeDeclPages); err == nil {
			if (themeDeclPages.Configuration.Type == "managed" || themeDeclPages.Configuration.Type == "") && len(themeDeclPages.Configuration.Pages) > 0 {
				for _, page := range themeDeclPages.Configuration.Pages {
					for _, item := range page.Items {
						if item.Key == "" {
							continue
						}
						if _, exists := savedSettings[item.Key]; !exists {
							savedSettings[item.Key] = getDefaultForItem(item)
						}
					}
				}
				return savedSettings, nil
			}
		}

		// 兼容旧 data 格式
		var themeDeclData struct {
			Configuration struct {
				Type string                          `json:"type"`
				Data []models.ManagedThemeConfigItem `json:"data"`
			} `json:"configuration"`
		}
		if err := json.Unmarshal(defaultConfigJSON, &themeDeclData); err == nil {
			if themeDeclData.Configuration.Type == "managed" || themeDeclData.Configuration.Type == "" {
				for _, item := range themeDeclData.Configuration.Data {
					if item.Key == "" {
						continue
					}
					if _, exists := savedSettings[item.Key]; !exists {
						savedSettings[item.Key] = getDefaultForItem(item)
					}
				}
			}
		}
	}

	return savedSettings, nil
}

// getDefaultForItem 获取配置项的默认值
func getDefaultForItem(item models.ManagedThemeConfigItem) any {
	var def any = item.Default
	if item.Type == "select" {
		if def == nil || def == "" {
			if item.Options != "" {
				opts := strings.Split(item.Options, ",")
				if len(opts) > 0 {
					def = strings.TrimSpace(opts[0])
				}
			}
		}
	}
	if def == nil {
		switch item.Type {
		case "number":
			def = 0
		case "switch":
			def = false
		default:
			def = ""
		}
	}
	return def
}

// isValidThemeShort 验证主题 short 字段
func isValidThemeShort(short string) bool {
	for _, c := range short {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' || c == '-') {
			return false
		}
	}
	return len(short) > 0
}
