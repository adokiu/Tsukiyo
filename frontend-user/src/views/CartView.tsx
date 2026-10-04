import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useCartStore } from '@/stores/cart'
import { type CartItem } from '@/api/cart'
import {
  ShoppingCart, Trash2, Plus, Minus, ArrowLeft, Loader2, Package,
} from 'lucide-react'
import '@/views/ProductsView.css'

function formatPrice(cents: number): string {
  return (cents / 100).toFixed(2)
}

function formatMB(mb: number): string {
  if (mb >= 1024) return `${(mb / 1024).toFixed(0)} GB`
  return `${mb} MB`
}

function formatConfig(config: CartItem['config']): string {
  const parts: string[] = []
  if (config.vcpu) parts.push(`${config.vcpu} 核`)
  if (config.memory_mb) parts.push(formatMB(config.memory_mb))
  if (config.disk_mb) parts.push(`系统盘 ${formatMB(config.disk_mb)}`)
  if (config.data_disk_mb) parts.push(`数据盘 ${formatMB(config.data_disk_mb)}`)
  const periodMap: Record<string, string> = { monthly: '月付', quarterly: '季付', half_yearly: '半年付', yearly: '年付' }
  if (config.payment_period) parts.push(periodMap[config.payment_period] || config.payment_period)
  if (config.trial) parts.push('试用')
  return parts.join(' / ')
}

export function CartView() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { items, loading, fetchCart, updateQuantity, removeItem, clearCart } = useCartStore()
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [clearing, setClearing] = useState(false)

  useEffect(() => {
    fetchCart()
  }, [fetchCart])

  const toggleSelect = (id: string) => {
    setSelectedIds(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const toggleSelectAll = () => {
    if (selectedIds.size === items.length) {
      setSelectedIds(new Set())
    } else {
      setSelectedIds(new Set(items.map(i => i.id)))
    }
  }

  const selectedItems = items.filter(i => selectedIds.has(i.id))
  const totalCents = selectedItems.reduce((sum, i) => sum + i.unit_price_cents * i.quantity, 0)

  const handleCheckout = () => {
    if (selectedItems.length === 0) return
    const checkoutItems = selectedItems.map(i => ({
      product_id: i.product_id,
      config: i.config,
      quantity: i.quantity,
    }))
    navigate('/checkout', { state: { items: checkoutItems } })
  }

  const handleClearCart = async () => {
    setClearing(true)
    await clearCart()
    setSelectedIds(new Set())
    setClearing(false)
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

  return (
    <main className="flex-1 overflow-y-auto overflow-x-hidden p-4 sm:p-6 md:p-8">
      <div className="product-detail-container">
        <div className="product-detail-header">
          <Link to="/products" className="product-detail-back-btn">
            <ArrowLeft size={16} />
            {t('common.back')}
          </Link>
          <h1 className="product-detail-title">
            <ShoppingCart size={24} />
            {t('cart.title')}
          </h1>
        </div>

        {items.length === 0 ? (
          <div className="product-empty">
            <ShoppingCart size={48} className="text-muted-foreground opacity-50" />
            <p className="text-muted-foreground mt-4">{t('cart.empty')}</p>
            <Link to="/products" className="product-detail-back-btn mt-4">{t('cart.goShopping')}</Link>
          </div>
        ) : (
          <>
            <div className="cart-toolbar">
              <label className="cart-select-all">
                <input
                  type="checkbox"
                  checked={selectedIds.size === items.length && items.length > 0}
                  onChange={toggleSelectAll}
                />
                <span>{t('cart.selectAll')}</span>
              </label>
              <button
                className="cart-clear-btn"
                onClick={handleClearCart}
                disabled={clearing}
              >
                <Trash2 size={14} />
                {t('cart.clearAll')}
              </button>
            </div>

            <div className="cart-list">
              {items.map(item => (
                <div key={item.id} className={`cart-item ${selectedIds.has(item.id) ? 'selected' : ''}`}>
                  <label className="cart-item-check">
                    <input
                      type="checkbox"
                      checked={selectedIds.has(item.id)}
                      onChange={() => toggleSelect(item.id)}
                    />
                  </label>
                  <div className="cart-item-info">
                    <div className="cart-item-name">
                      <Package size={16} />
                      {item.product?.name || t('cart.unknownProduct')}
                    </div>
                    <div className="cart-item-config">{formatConfig(item.config)}</div>
                    {item.config.name && (
                      <div className="cart-item-instance-name">{t('product.instanceName')}: {item.config.name}</div>
                    )}
                  </div>
                  <div className="cart-item-price">
                    <span className="cart-item-unit-price font-number">¥{formatPrice(item.unit_price_cents)}</span>
                    <div className="cart-item-quantity">
                      <button
                        onClick={() => updateQuantity(item.id, Math.max(1, item.quantity - 1))}
                        disabled={item.quantity <= 1}
                      >
                        <Minus size={14} />
                      </button>
                      <span className="font-number">{item.quantity}</span>
                      <button onClick={() => updateQuantity(item.id, item.quantity + 1)}>
                        <Plus size={14} />
                      </button>
                    </div>
                  </div>
                  <div className="cart-item-subtotal">
                    <span className="font-number">¥{formatPrice(item.unit_price_cents * item.quantity)}</span>
                  </div>
                  <button
                    className="cart-item-remove"
                    onClick={() => removeItem(item.id)}
                  >
                    <Trash2 size={16} />
                  </button>
                </div>
              ))}
            </div>

            <div className="cart-footer">
              <div className="cart-footer-summary">
                <span>{t('cart.selectedCount', { count: selectedItems.length })}</span>
                <span className="cart-footer-total">
                  {t('product.totalPrice')}: <span className="font-number">¥{formatPrice(totalCents)}</span>
                </span>
              </div>
              <button
                className="purchase-submit-btn"
                disabled={selectedItems.length === 0}
                onClick={handleCheckout}
              >
                {t('cart.checkout')}
              </button>
            </div>
          </>
        )}
      </div>
    </main>
  )
}
