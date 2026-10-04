import { useEffect, useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { orderApi, type Order } from '@/api/cart'
import { paymentApi } from '@/api/payment'
import { useAuthStore } from '@/stores/auth'
import {
  ArrowLeft, Loader2, Wallet, CreditCard, Check, AlertCircle,
  ExternalLink, CheckCircle2, XCircle, Clock,
} from 'lucide-react'
import '@/views/ProductsView.css'

function formatPrice(cents: number): string {
  return (cents / 100).toFixed(2)
}

export function PaymentView() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { id: orderId } = useParams<{ id: string }>()
  const { user } = useAuthStore()

  const [order, setOrder] = useState<Order | null>(null)
  const [loading, setLoading] = useState(true)
  const [paying, setPaying] = useState(false)
  const [error, setError] = useState('')
  const [payMethod, setPayMethod] = useState<'balance' | 'channel'>('balance')
  const [channels, setChannels] = useState<{ id: number; name: string; type: string; icon?: string; description?: string }[]>([])
  const [selectedChannel, setSelectedChannel] = useState<number | null>(null)
  const [payResult, setPayResult] = useState<{ status: string; pay_url?: string } | null>(null)
  const [countdown, setCountdown] = useState('')
  const [expired, setExpired] = useState(false)

  useEffect(() => {
    if (!orderId) return
    orderApi.get(orderId).then(res => {
      setOrder(res.data)
      setLoading(false)
      if (res.data.status === 'paid' || res.data.status === 'completed') {
        setPayResult({ status: res.data.status })
      } else if (res.data.status === 'cancelled') {
        setExpired(true)
      }
    }).catch(() => {
      setLoading(false)
    })
  }, [orderId])

  // 倒计时
  useEffect(() => {
    if (!order?.expires_at || expired || payResult) return
    const updateCountdown = () => {
      const expires = new Date(order.expires_at!).getTime()
      const remaining = expires - Date.now()
      if (remaining <= 0) {
        setExpired(true)
        setCountdown('00:00')
        return
      }
      const minutes = Math.floor(remaining / 60000)
      const seconds = Math.floor((remaining % 60000) / 1000)
      setCountdown(`${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`)
    }
    updateCountdown()
    const timer = setInterval(updateCountdown, 1000)
    return () => clearInterval(timer)
  }, [order?.expires_at, expired, payResult])

  useEffect(() => {
    paymentApi.listChannels().then(res => {
      setChannels(res.data?.data || [])
    }).catch(() => {})
  }, [])

  const balanceCents = user?.balance_cents ?? 0
  const totalCents = order?.total_cents ?? 0
  const canPayByBalance = balanceCents >= totalCents

  const handlePay = async () => {
    if (!orderId) return
    setPaying(true)
    setError('')
    try {
      const res = await orderApi.pay({
        order_id: orderId,
        method: payMethod,
        channel_id: payMethod === 'channel' ? selectedChannel || undefined : undefined,
      })
      const result = res.data
      if (result.status === 'paid' || result.status === 'completed') {
        setPayResult({ status: result.status })
      } else if (result.pay_url) {
        setPayResult({ status: 'pending', pay_url: result.pay_url })
        window.open(result.pay_url, '_blank')
      }
    } catch (err: any) {
      setError(err.response?.data?.error || t('payment.failed'))
    } finally {
      setPaying(false)
    }
  }

  if (loading) {
    return (
      <main className="flex-1 overflow-y-auto overflow-x-hidden p-4 sm:p-6 md:p-8">
        <div className="flex justify-center py-20">
          <Loader2 className="animate-spin text-muted-foreground" size={28} />
        </div>
      </main>
    )
  }

  if (!order) {
    return (
      <main className="flex-1 overflow-y-auto overflow-x-hidden p-4 sm:p-6 md:p-8">
        <div className="product-empty">
          <AlertCircle size={48} className="text-muted-foreground opacity-50" />
          <p className="text-muted-foreground mt-4">{t('payment.orderNotFound')}</p>
          <Link to="/cart" className="product-detail-back-btn mt-4">{t('common.back')}</Link>
        </div>
      </main>
    )
  }

  // 支付成功页面
  if (payResult?.status === 'paid' || payResult?.status === 'completed') {
    return (
      <main className="flex-1 overflow-y-auto overflow-x-hidden p-4 sm:p-6 md:p-8">
        <div className="product-detail-container">
          <div className="payment-result-card">
            <CheckCircle2 size={64} className="payment-result-icon success" />
            <h2 className="payment-result-title">{t('payment.success')}</h2>
            <p className="payment-result-desc">{t('payment.successDesc')}</p>
            <div className="payment-result-order">
              <span>{t('payment.orderNo')}: {order.order_no}</span>
              <span className="font-number">¥{formatPrice(order.total_cents)}</span>
            </div>
            <div className="payment-result-actions">
              <Link to="/instances" className="purchase-submit-btn" style={{ textDecoration: 'none' }}>
                {t('payment.viewInstances')}
              </Link>
              <Link to="/products" className="product-detail-back-btn">
                {t('payment.continueShopping')}
              </Link>
            </div>
          </div>
        </div>
      </main>
    )
  }

  // 等待外部支付
  if (payResult?.status === 'pending' && payResult.pay_url) {
    return (
      <main className="flex-1 overflow-y-auto overflow-x-hidden p-4 sm:p-6 md:p-8">
        <div className="product-detail-container">
          <div className="payment-result-card">
            {expired ? (
              <>
                <XCircle size={64} className="payment-result-icon failed" />
                <h2 className="payment-result-title">{t('payment.expired')}</h2>
                <p className="payment-result-desc">{t('payment.expiredDesc')}</p>
                <Link to="/products" className="product-detail-back-btn">
                  {t('payment.continueShopping')}
                </Link>
              </>
            ) : (
              <>
                <Loader2 size={64} className="payment-result-icon pending" />
                <h2 className="payment-result-title">{t('payment.waiting')}</h2>
                <p className="payment-result-desc">{t('payment.waitingDesc')}</p>
                {countdown && (
                  <div className="payment-countdown">
                    <Clock size={16} />
                    <span className="font-number">{countdown}</span>
                  </div>
                )}
                <a href={payResult.pay_url} target="_blank" rel="noopener noreferrer" className="purchase-submit-btn" style={{ textDecoration: 'none', display: 'inline-flex' }}>
                  <ExternalLink size={16} />
                  {t('payment.openPayPage')}
                </a>
                <button className="product-detail-back-btn" onClick={() => navigate('/instances')}>
                  {t('payment.viewInstances')}
                </button>
              </>
            )}
          </div>
        </div>
      </main>
    )
  }

  // 订单已过期
  if (expired) {
    return (
      <main className="flex-1 overflow-y-auto overflow-x-hidden p-4 sm:p-6 md:p-8">
        <div className="product-detail-container">
          <div className="payment-result-card">
            <XCircle size={64} className="payment-result-icon failed" />
            <h2 className="payment-result-title">{t('payment.expired')}</h2>
            <p className="payment-result-desc">{t('payment.expiredDesc')}</p>
            {order && (
              <div className="payment-result-order">
                <span>{t('payment.orderNo')}: {order.order_no}</span>
                <span className="font-number">¥{formatPrice(order.total_cents)}</span>
              </div>
            )}
            <div className="payment-result-actions">
              <Link to="/products" className="purchase-submit-btn" style={{ textDecoration: 'none' }}>
                {t('payment.continueShopping')}
              </Link>
            </div>
          </div>
        </div>
      </main>
    )
  }

  return (
    <main className="flex-1 overflow-y-auto overflow-x-hidden p-4 sm:p-6 md:p-8">
      <div className="product-detail-container">
        <div className="product-detail-header">
          <Link to="/cart" className="product-detail-back-btn">
            <ArrowLeft size={16} />
            {t('common.back')}
          </Link>
          <h1 className="product-detail-title">
            <CreditCard size={24} />
            {t('payment.title')}
          </h1>
        </div>

        {/* 订单信息 */}
        <div className="checkout-section">
          <h3 className="checkout-section-title">{t('payment.orderInfo')}</h3>
          <div className="checkout-summary">
            <div className="checkout-summary-row">
              <span>{t('payment.orderNo')}</span>
              <span>{order.order_no}</span>
            </div>
            <div className="checkout-summary-row">
              <span>{t('checkout.subtotal')}</span>
              <span className="font-number">¥{formatPrice(order.subtotal_cents)}</span>
            </div>
            {order.discount_cents > 0 && (
              <div className="checkout-summary-row checkout-discount">
                <span>{t('checkout.discount')}</span>
                <span className="font-number">-¥{formatPrice(order.discount_cents)}</span>
              </div>
            )}
            <div className="checkout-summary-row checkout-total">
              <span>{t('checkout.total')}</span>
              <span className="font-number">¥{formatPrice(order.total_cents)}</span>
            </div>
            {countdown && (
              <div className="checkout-summary-row payment-countdown-row">
                <span><Clock size={14} style={{ display: 'inline', verticalAlign: 'middle' }} /> {t('payment.countdown')}</span>
                <span className="font-number payment-countdown-text">{countdown}</span>
              </div>
            )}
          </div>
        </div>

        {/* 支付方式 */}
        <div className="checkout-section">
          <h3 className="checkout-section-title">{t('payment.method')}</h3>
          <div className="payment-methods">
            <button
              className={`payment-method-card ${payMethod === 'balance' ? 'selected' : ''}`}
              onClick={() => setPayMethod('balance')}
              disabled={!canPayByBalance}
            >
              <Wallet size={20} />
              <div className="payment-method-info">
                <span className="payment-method-name">{t('payment.balance')}</span>
                <span className="payment-method-desc font-number">¥{formatPrice(balanceCents)}</span>
              </div>
              {payMethod === 'balance' && <Check size={18} className="payment-method-check" />}
              {!canPayByBalance && <span className="payment-method-insufficient">{t('payment.insufficient')}</span>}
            </button>

            {channels.map(ch => (
              <button
                key={ch.id}
                className={`payment-method-card ${payMethod === 'channel' && selectedChannel === ch.id ? 'selected' : ''}`}
                onClick={() => { setPayMethod('channel'); setSelectedChannel(ch.id) }}
              >
                <CreditCard size={20} />
                <div className="payment-method-info">
                  <span className="payment-method-name">{ch.name}</span>
                  {ch.description && <span className="payment-method-desc">{ch.description}</span>}
                </div>
                {payMethod === 'channel' && selectedChannel === ch.id && <Check size={18} className="payment-method-check" />}
              </button>
            ))}
          </div>
        </div>

        {error && (
          <div className="purchase-error">
            <AlertCircle size={14} />
            <span>{error}</span>
          </div>
        )}

        <button
          className="purchase-submit-btn"
          disabled={paying || (payMethod === 'balance' && !canPayByBalance) || (payMethod === 'channel' && !selectedChannel)}
          onClick={handlePay}
        >
          {paying ? (
            <><Loader2 className="animate-spin" size={16} />{t('payment.paying')}</>
          ) : (
            t('payment.confirmPay', { amount: formatPrice(totalCents) })
          )}
        </button>
      </div>
    </main>
  )
}
