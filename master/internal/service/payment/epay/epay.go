package epay

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"tsukiyo/master/internal/service/payment"
)

// EpayDriver 易支付驱动
// 兼容标准易支付协议, 支持支付宝/微信/QQ等多种支付方式
type EpayDriver struct{}

func (d *EpayDriver) Type() string        { return "epay" }
func (d *EpayDriver) Name() string        { return "EPay" }
func (d *EpayDriver) Description() string { return "易支付, 兼容标准易支付协议, 可对接支付宝/微信/QQ等多种支付方式" }

func (d *EpayDriver) ConfigFields() []payment.ConfigField {
	return []payment.ConfigField{
		{
			Key:         "url",
			Label:       "支付网关地址",
			Type:        payment.ConfigFieldTypeString,
			Required:    true,
			Placeholder: "https://pay.example.com",
			Help:        "请填写完整的支付网关地址, 包括协议(http或https)",
		},
		{
			Key:         "pid",
			Label:       "商户ID",
			Type:        payment.ConfigFieldTypeString,
			Required:    true,
			Placeholder: "商户ID",
		},
		{
			Key:         "key",
			Label:       "通信密钥",
			Type:        payment.ConfigFieldTypeSecret,
			Required:    true,
			Placeholder: "通信密钥",
		},
		{
			Key:         "type",
			Label:       "支付类型",
			Type:        payment.ConfigFieldTypeSelect,
			Required:    false,
			Default:     "alipay",
			Help:        "支付类型, 如: alipay, wxpay, qqpay 等",
			Options: []payment.ConfigFieldOption{
				{Label: "支付宝", Value: "alipay"},
				{Label: "微信支付", Value: "wxpay"},
				{Label: "QQ支付", Value: "qqpay"},
				{Label: "云闪付", Value: "unionpay"},
			},
		},
	}
}

func (d *EpayDriver) SupportedMethods() []payment.PaymentMethodDef {
	return []payment.PaymentMethodDef{
		{Type: "alipay", Name: "支付宝", Icon: "", Description: "通过易支付使用支付宝支付"},
		{Type: "wxpay", Name: "微信支付", Icon: "", Description: "通过易支付使用微信支付"},
		{Type: "qqpay", Name: "QQ支付", Icon: "", Description: "通过易支付使用QQ支付"},
		{Type: "unionpay", Name: "云闪付", Icon: "", Description: "通过易支付使用云闪付支付"},
	}
}

func (d *EpayDriver) SupportedCurrencies() []string {
	return []string{"CNY"}
}

// Pay 发起支付
// 照抄 Xboard Epay Plugin::pay() 逻辑:
// 1. 组装参数 (money, name, notify_url, return_url, out_trade_no, pid, type)
// 2. 按key排序后拼接签名: ksort + urldecode(http_build_query) + key
// 3. md5 签名
// 4. 返回跳转URL: {url}/submit.php?{params}&sign={sign}&sign_type=MD5
func (d *EpayDriver) Pay(config map[string]string, order payment.PayOrder) (*payment.PayResult, error) {
	gatewayURL := config["url"]
	if gatewayURL == "" {
		return nil, fmt.Errorf("支付网关地址未配置")
	}
	key := config["key"]
	if key == "" {
		return nil, fmt.Errorf("通信密钥未配置")
	}
	pid := config["pid"]
	if pid == "" {
		return nil, fmt.Errorf("商户ID未配置")
	}

	// 组装参数
	params := map[string]string{
		"money":        fmt.Sprintf("%.2f", float64(order.TotalAmount)/100),
		"name":         order.TradeNo,
		"notify_url":   order.NotifyURL,
		"return_url":   order.ReturnURL,
		"out_trade_no": order.TradeNo,
		"pid":          pid,
	}

	// 可选支付类型
	if payType := config["type"]; payType != "" {
		params["type"] = payType
	}

	// 按key排序
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 拼接签名字符串: key1=value1&key2=value2... + key
	var signParts []string
	queryValues := url.Values{}
	for _, k := range keys {
		queryValues.Set(k, params[k])
		signParts = append(signParts, fmt.Sprintf("%s=%s", k, params[k]))
	}
	signStr := strings.Join(signParts, "&") + key

	// md5 签名
	hash := md5.Sum([]byte(signStr))
	sign := hex.EncodeToString(hash[:])

	// 构建跳转URL
	payURL := fmt.Sprintf("%s/submit.php?%s&sign=%s&sign_type=MD5",
		strings.TrimRight(gatewayURL, "/"),
		queryValues.Encode(),
		sign,
	)

	return &payment.PayResult{
		Type: 1, // 跳转URL
		Data: payURL,
	}, nil
}

// Notify 回调验证
// 照抄 Xboard Epay Plugin::notify() 逻辑:
// 1. 取出 sign, 移除 sign 和 sign_type
// 2. 按key排序后拼接签名: ksort + urldecode(http_build_query) + key
// 3. md5 验证签名
// 4. 返回 out_trade_no(商户订单号) 和 trade_no(第三方流水号)
func (d *EpayDriver) Notify(config map[string]string, params map[string]string) (*payment.NotifyResult, error) {
	key := config["key"]
	if key == "" {
		return nil, fmt.Errorf("通信密钥未配置")
	}

	sign, ok := params["sign"]
	if !ok {
		return nil, fmt.Errorf("回调缺少sign参数")
	}

	// 移除 sign 和 sign_type
	notifyParams := make(map[string]string)
	for k, v := range params {
		if k == "sign" || k == "sign_type" {
			continue
		}
		notifyParams[k] = v
	}

	// 按key排序
	keys := make([]string, 0, len(notifyParams))
	for k := range notifyParams {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 拼接签名字符串
	var signParts []string
	for _, k := range keys {
		signParts = append(signParts, fmt.Sprintf("%s=%s", k, notifyParams[k]))
	}
	signStr := strings.Join(signParts, "&") + key

	// md5 验证
	hash := md5.Sum([]byte(signStr))
	expectedSign := hex.EncodeToString(hash[:])

	if sign != expectedSign {
		return nil, fmt.Errorf("签名验证失败")
	}

	tradeNo := notifyParams["out_trade_no"]
	callbackNo := notifyParams["trade_no"]
	if tradeNo == "" {
		return nil, fmt.Errorf("回调缺少out_trade_no参数")
	}

	return &payment.NotifyResult{
		TradeNo:    tradeNo,
		CallbackNo: callbackNo,
	}, nil
}

func init() {
	payment.Register(&EpayDriver{})
}
