package balance

import (
	"errors"

	"tsukiyo/master/internal/service/payment"
)

// BalanceDriver 余额支付驱动 (系统内置, 无需外部配置)
type BalanceDriver struct{}

func (d *BalanceDriver) Type() string        { return "balance" }
func (d *BalanceDriver) Name() string        { return "Balance" }
func (d *BalanceDriver) Description() string { return "用户余额支付, 直接从用户钱包余额扣款" }

func (d *BalanceDriver) ConfigFields() []payment.ConfigField {
	return nil
}

func (d *BalanceDriver) SupportedMethods() []payment.PaymentMethodDef {
	return []payment.PaymentMethodDef{
		{Type: "balance", Name: "余额支付", Icon: "", Description: "使用用户钱包余额支付"},
	}
}

func (d *BalanceDriver) SupportedCurrencies() []string {
	return []string{"CNY"}
}

// Pay 余额支付直接扣款, 不需要跳转第三方
func (d *BalanceDriver) Pay(config map[string]string, order payment.PayOrder) (*payment.PayResult, error) {
	return nil, errors.New("余额支付无需跳转, 直接在服务层扣款")
}

// Notify 余额支付无回调
func (d *BalanceDriver) Notify(config map[string]string, params map[string]string) (*payment.NotifyResult, error) {
	return nil, errors.New("余额支付无回调")
}

func init() {
	payment.Register(&BalanceDriver{})
}
