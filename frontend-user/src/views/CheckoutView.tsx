import { useEffect, useState, useMemo } from 'react'
import { useLocation, useNavigate, Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { orderApi, couponApi, type CheckoutItem, type CouponValidateResult } from '@/api/cart'
import { useAuthStore } from '@/stores/auth'
import { useCartStore } from '@/stores/cart'
import {
  ArrowLeft, Loader2, Tag, Check, AlertCircle, ShoppingBag, X,
} from 'lucide-react'
import '@/views/ProductsView.css'

function formatPrice(cents: number): string {
  return (cents / 100).toFixed(2)
}

function formatMB(mb: number): string {
  if (mb >= 1024) return `${(mb / 1024).toFixed(0)} GB`
  return `${mb} MB`
}

function formatConfig(config: Record<string, unknown>): string {
  const parts: string[] = []
  if (config.vcpu) parts.push(`${config.vcpu} 核`)
  if (config.memory_mb) parts.push(formatMB(config.memory_mb as number))
  if (config.disk_mb) parts.push(`系统盘 ${formatMB(config.disk_mb as number)}`)
  if (config.data_disk_mb) parts.push(`数据盘 ${formatMB(config.data_disk_mb as number)}`)
  const periodMap: Record<string, string> = { monthly: '月付', quarterly: '季付', half_yearly: '半年付', yearly: '年付' }
  if (config.payment_period) parts.push(periodMap[config.payment_period as string] || config.payment_period as string)
  if (config.trial) parts.push('试用')
  return parts.join(' / ')
}

export function CheckoutView() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const { user } = useAuthStore()
  const { items: cartItems, clearCart } = useCartStore()

  const [checkoutItems, setCheckoutItems] = useState<CheckoutItem[]>([])
  const [itemDetails, setItemDetails] = useState<{ product_id: string; product_name: string; unit_price_cents: number; config: Record<string, unknown>; quantity: number }[]>([])
  const [couponCode, setCouponCode] = useState('')
  const [couponResult, setCouponResult] = useState<CouponValidateResult | null>(null)
  const [couponLoading, setCouponLoading] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    const state = location.state as { items?: CheckoutItem[] } | null
    if (state?.items && state.items.length > 0) {
      setCheckoutItems(state.items)
    } else if (cartItems.length > 0) {
      setCheckoutItems(cartItems.map(i => ({
        product_id: i.product_id,
        config: i.config as Record<string, unknown>,
        quantity: i.quantity,
      })))
    } else {
      navigate('/cart')
    }
  }, [location.state, cartItems, navigate])

  useEffect(() => {
    if (checkoutItems.length === 0) return
    const details = checkoutItems.map(item => {
      const cartItem = cartItems.find(ci => ci.product_id === item.product_id)
      return {
        product_id: item.product_id,
        product_name: cartItem?.product?.name || t('cart.unknownProduct'),
        unit_price_cents: cartItem?.unit_price_cents || 0,
        config: item.config,
        quantity: item.quantity,
      }
    })
    setItemDetails(details)
  }, [checkoutItems, cartItems, t])

  const subtotalCents = useMemo(() => {
    return itemDetails.reduce((sum, i) => sum + i.unit_price_cents * i.quantity, 0)
  }, [itemDetails])

  const discountCents = couponResult?.valid ? couponResult.discount_cents : 0
  const totalCents = Math.max(0, subtotalCents - discountCents)
  const balanceCents = user?.balance_cents ?? 0

  const handleValidateCoupon = async () => {
    if (!couponCode.trim()) return
    setCouponLoading(true)
    setError('')
    try {
      const res = await couponApi.validate({ code: couponCode.trim(), amount_cents: subtotalCents })
      setCouponResult(res.data)
      if (!res.data.valid) {
        setError(res.data.message || t('checkout.couponInvalid'))
      }
    } catch (err: any) {
      setCouponResult(null)
      setError(err.response?.data?.error || t('checkout.couponInvalid'))
    } finally {
      setCouponLoading(false)
    }
  }

  const handleRemoveCoupon = () => {
    setCouponCode('')
    setCouponResult(null)
    setError('')
  }

  const handleCheckout = async () => {
    setSubmitting(true)
    setError('')
    try {
      const res = await orderApi.checkout({
        items: checkoutItems,
        coupon_code: couponResult?.valid ? couponCode.trim() : undefined,
      })
      const orderId = res.data.order_id
      // 清空购物车中已结算的商品
      await clearCart()
      navigate(`/payment/${orderId}`)
    } catch (err: any) {
      setError(err.response?.data?.error || t('checkout.failed'))
    } finally {
      setSubmitting(false)
    }
  }

  if (itemDetails.length === 0) {
    return (
      <main className="flex-1 overflow-y-auto overflow-x-hidden p-4 sm:p-6 md:p-8">
        <div className="flex justify-center py-20">
          <Loader2 className="animate-spin text-muted-foreground" size={28} />
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
            <ShoppingBag size={24} />
            {t('checkout.title')}
          </h1>
        </div>

        {/* 商品列表 */}
        <div className="checkout-section">
          <h3 className="checkout-section-title">{t('checkout.items')}</h3>
          <div className="checkout-items">
            {itemDetails.map((item, idx) => (
              <div key={idx} className="checkout-item">
                <div className="checkout-item-info">
                  <div className="checkout-item-name">{item.product_name}</div>
                  <div className="checkout-item-config">{formatConfig(item.config)}</div>
                </div>
                <div className="checkout-item-meta">
                  <span className="checkout-item-qty">x{item.quantity}</span>
                  <span className="checkout-item-price font-number">¥{formatPrice(item.unit_price_cents * item.quantity)}</span>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* 优惠码 */}
        <div className="checkout-section">
          <h3 className="checkout-section-title">
            <Tag size={16} />
            {t('checkout.coupon')}
          </h3>
          {couponResult?.valid ? (
            <div className="checkout-coupon-applied">
              <div className="checkout-coupon-info">
                <Check size={16} />
                <span>{couponResult.coupon_name || couponCode}</span>
                <span className="checkout-coupon-discount font-number">
                  -¥{formatPrice(couponResult.discount_cents)}
                </span>
              </div>
              <button className="checkout-coupon-remove" onClick={handleRemoveCoupon}>
                <X size={16} />
              </button>
            </div>
          ) : (
            <div className="checkout-coupon-input-row">
              <input
                className="purchase-input checkout-coupon-input"
                value={couponCode}
                onChange={e => setCouponCode(e.target.value)}
                placeholder={t('checkout.couponPlaceholder')}
                onKeyDown={e => e.key === 'Enter' && handleValidateCoupon()}
              />
              <button
                className="checkout-coupon-btn"
                onClick={handleValidateCoupon}
                disabled={couponLoading || !couponCode.trim()}
              >
                {couponLoading ? <Loader2 className="animate-spin" size={14} /> : t('checkout.applyCoupon')}
              </button>
            </div>
          )}
        </div>

        {/* 价格汇总 */}
        <div className="checkout-summary">
          <div className="checkout-summary-row">
            <span>{t('checkout.subtotal')}</span>
            <span className="font-number">¥{formatPrice(subtotalCents)}</span>
          </div>
          {discountCents > 0 && (
            <div className="checkout-summary-row checkout-discount">
              <span>{t('checkout.discount')}</span>
              <span className="font-number">-¥{formatPrice(discountCents)}</span>
            </div>
          )}
          <div className="checkout-summary-row checkout-total">
            <span>{t('checkout.total')}</span>
            <span className="font-number">¥{formatPrice(totalCents)}</span>
          </div>
          <div className="checkout-summary-row checkout-balance">
            <span>{t('product.currentBalance')}</span>
            <span className="font-number">¥{formatPrice(balanceCents)}</span>
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
          disabled={submitting}
          onClick={handleCheckout}
        >
          {submitting ? (
            <><Loader2 className="animate-spin" size={16} />{t('checkout.creatingOrder')}</>
          ) : (
            t('checkout.goToPay')
          )}
        </button>
      </div>
    </main>
  )
}
