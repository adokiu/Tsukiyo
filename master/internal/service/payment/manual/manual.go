package manual

import (
	"errors"

	"tsukiyo/master/internal/service/payment"
)

// ManualDriver 人工入账驱动 (管理员手动操作, 无需外部配置)
type ManualDriver struct{}

func (d *ManualDriver) Type() string        { return "manual" }
func (d *ManualDriver) Name() string        { return "Manual" }
func (d *ManualDriver) Description() string { return "人工入账, 管理员手动确认到账" }

func (d *ManualDriver) ConfigFields() []payment.ConfigField {
	return nil
}

func (d *ManualDriver) SupportedMethods() []payment.PaymentMethodDef {
	return []payment.PaymentMethodDef{
		{Type: "bank_transfer", Name: "银行转账", Icon: "", Description: "用户通过银行转账后管理员手动确认"},
		{Type: "manual_entry", Name: "手动入账", Icon: "", Description: "管理员直接为用户充值"},
	}
}

func (d *ManualDriver) SupportedCurrencies() []string {
	return []string{"CNY", "USD", "EUR", "GBP", "JPY", "HKD", "SGD", "TWD"}
}

// Pay 人工入账不需要发起支付, 管理员在后台手动确认
func (d *ManualDriver) Pay(config map[string]string, order payment.PayOrder) (*payment.PayResult, error) {
	return nil, errors.New("人工入账无需发起支付, 管理员手动确认")
}

// Notify 人工入账无自动回调
func (d *ManualDriver) Notify(config map[string]string, params map[string]string) (*payment.NotifyResult, error) {
	return nil, errors.New("人工入账无自动回调")
}

func init() {
	payment.Register(&ManualDriver{})
}
