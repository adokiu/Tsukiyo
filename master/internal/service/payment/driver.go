package payment

import (
	"sort"
	"sync"
)

// ConfigFieldType 配置字段类型
type ConfigFieldType string

const (
	ConfigFieldTypeString  ConfigFieldType = "string"
	ConfigFieldTypeNumber  ConfigFieldType = "number"
	ConfigFieldTypeBoolean ConfigFieldType = "boolean"
	ConfigFieldTypeSelect  ConfigFieldType = "select"
	ConfigFieldTypeSecret  ConfigFieldType = "secret"
)

// ConfigFieldOption select 类型的选项
type ConfigFieldOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// ConfigField 驱动配置字段定义
type ConfigField struct {
	Key         string              `json:"key"`
	Label       string              `json:"label"`
	Type        ConfigFieldType     `json:"type"`
	Required    bool                `json:"required"`
	Placeholder string              `json:"placeholder,omitempty"`
	Default     string              `json:"default,omitempty"`
	Options     []ConfigFieldOption `json:"options,omitempty"`
	Help        string              `json:"help,omitempty"`
}

// PaymentMethodDef 驱动支持的支付方式定义
type PaymentMethodDef struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Icon        string `json:"icon,omitempty"`
	Description string `json:"description,omitempty"`
}

// PayOrder 支付订单参数
type PayOrder struct {
	TradeNo     string `json:"trade_no"`     // 商户订单号
	TotalAmount int64  `json:"total_amount"` // 金额 (分)
	NotifyURL   string `json:"notify_url"`   // 异步通知地址
	ReturnURL   string `json:"return_url"`   // 同步跳转地址
	UserID      uint   `json:"user_id"`      // 用户ID
}

// PayResult 支付结果
type PayResult struct {
	Type int    `json:"type"` // 1=跳转URL, 2=表单HTML, 3=二维码
	Data string `json:"data"` // URL / HTML / 二维码内容
}

// NotifyResult 回调验证结果
type NotifyResult struct {
	TradeNo    string `json:"trade_no"`    // 商户订单号
	CallbackNo string `json:"callback_no"` // 第三方流水号
}

// PaymentDriver 支付驱动接口 (trait)
// 每个支付驱动模块实现此接口, 并在 init() 中自注册
type PaymentDriver interface {
	// Type 驱动唯一标识 (如 "epay", "balance")
	Type() string
	// Name 驱动显示名称
	Name() string
	// Description 驱动描述
	Description() string
	// ConfigFields 驱动需要的配置字段定义
	ConfigFields() []ConfigField
	// SupportedMethods 该驱动支持的支付方式列表
	SupportedMethods() []PaymentMethodDef
	// SupportedCurrencies 该驱动支持的结算货币
	SupportedCurrencies() []string
	// Pay 发起支付, config 为渠道配置 (JSON 解析后的 map), 返回支付结果
	Pay(config map[string]string, order PayOrder) (*PayResult, error)
	// Notify 回调验证, config 为渠道配置, params 为回调参数, 验证成功返回订单信息
	Notify(config map[string]string, params map[string]string) (*NotifyResult, error)
}

// =================== 注册中心 ===================

var (
	registry     = make(map[string]PaymentDriver)
	registryLock sync.RWMutex
)

// Register 注册支付驱动 (由各驱动的 init() 调用)
func Register(d PaymentDriver) {
	registryLock.Lock()
	defer registryLock.Unlock()
	registry[d.Type()] = d
}

// GetDriver 获取指定类型的驱动
func GetDriver(driverType string) (PaymentDriver, bool) {
	registryLock.RLock()
	defer registryLock.RUnlock()
	d, ok := registry[driverType]
	return d, ok
}

// ListDrivers 列出所有已注册驱动 (按 Type 排序)
func ListDrivers() []PaymentDriver {
	registryLock.RLock()
	defer registryLock.RUnlock()
	drivers := make([]PaymentDriver, 0, len(registry))
	for _, d := range registry {
		drivers = append(drivers, d)
	}
	sort.Slice(drivers, func(i, j int) bool {
		return drivers[i].Type() < drivers[j].Type()
	})
	return drivers
}

// DriverInfo 驱动信息 (用于 API 返回)
type DriverInfo struct {
	Type                string             `json:"type"`
	Name                string             `json:"name"`
	Description         string             `json:"description"`
	ConfigFields        []ConfigField      `json:"config_fields"`
	SupportedMethods    []PaymentMethodDef `json:"supported_methods"`
	SupportedCurrencies []string           `json:"supported_currencies"`
}

// ListDriverInfo 列出所有已注册驱动的信息
func ListDriverInfo() []DriverInfo {
	drivers := ListDrivers()
	result := make([]DriverInfo, 0, len(drivers))
	for _, d := range drivers {
		result = append(result, DriverInfo{
			Type:                d.Type(),
			Name:                d.Name(),
			Description:         d.Description(),
			ConfigFields:        d.ConfigFields(),
			SupportedMethods:    d.SupportedMethods(),
			SupportedCurrencies: d.SupportedCurrencies(),
		})
	}
	return result
}
