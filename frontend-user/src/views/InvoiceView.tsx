import { useState, useEffect } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { billsApi, type InvoiceData } from '@/api/bills'
import { Loader2, ArrowLeft, Printer, FileText } from 'lucide-react'
import '@/views/ProductsView.css'

function formatPrice(cents: number): string {
  return (cents / 100).toFixed(2)
}

function formatPeriod(period: string, t: (k: string) => string): string {
  const map: Record<string, string> = {
    monthly: t('invoice.periodMonthly'),
    quarterly: t('invoice.periodQuarterly'),
    half_yearly: t('invoice.periodHalfYearly'),
    yearly: t('invoice.periodYearly'),
  }
  return map[period] || period
}

function formatLoginMethod(method: string, t: (k: string) => string): string {
  const map: Record<string, string> = {
    auto: t('invoice.loginAuto'),
    password: t('invoice.loginPassword'),
    sshkey: t('invoice.loginSSHKey'),
  }
  return map[method] || method
}

function formatIPMode(mode: string, t: (k: string) => string): string {
  const map: Record<string, string> = {
    nat: t('invoice.ipNat'),
    eip: t('invoice.ipEIP'),
    none: t('invoice.ipNone'),
  }
  return map[mode] || mode
}

const statusColors: Record<string, string> = {
  pending: 'text-yellow-500',
  paid: 'text-green-500',
  completed: 'text-green-500',
  cancelled: 'text-gray-500',
  failed: 'text-red-500',
  refunded: 'text-blue-500',
}

export function InvoiceView() {
  const { t } = useTranslation()
  const { id: orderId } = useParams<{ id: string }>()
  const [invoice, setInvoice] = useState<InvoiceData | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!orderId) return
    billsApi.getInvoice(orderId).then(res => {
      setInvoice(res.data)
      setLoading(false)
    }).catch(() => {
      setLoading(false)
    })
  }, [orderId])

  const handlePrint = () => {
    window.print()
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

  if (!invoice) {
    return (
      <main className="flex-1 overflow-y-auto overflow-x-hidden p-4 sm:p-6 md:p-8">
        <div className="product-empty">
          <FileText size={48} className="text-muted-foreground opacity-50" />
          <p className="text-muted-foreground mt-4">{t('invoice.notFound')}</p>
          <Link to="/bills" className="product-detail-back-btn mt-4">{t('common.back')}</Link>
        </div>
      </main>
    )
  }

  return (
    <main className="flex-1 overflow-y-auto overflow-x-hidden p-4 sm:p-6 md:p-8">
      <div className="mx-auto max-w-4xl w-full" style={{ animation: 'page-enter 0.25s ease' }}>
        {/* 工具栏 - 打印时隐藏 */}
        <div className="flex items-center justify-between mb-6 no-print">
          <Link to="/bills" className="product-detail-back-btn">
            <ArrowLeft size={16} />
            {t('common.back')}
          </Link>
          <button className="btn btn--primary btn--md" onClick={handlePrint}>
            <Printer size={16} />
            {t('invoice.print')}
          </button>
        </div>

        {/* 发票正文 */}
        <div className="invoice-document">
          {/* 发票头部 */}
          <div className="invoice-header">
            <div className="invoice-header-left">
              <h1 className="invoice-site-name">{invoice.site_name}</h1>
              {invoice.site_url && <p className="invoice-site-url">{invoice.site_url}</p>}
              {invoice.contact_email && <p className="invoice-contact">{invoice.contact_email}</p>}
            </div>
            <div className="invoice-header-right">
              <h2 className="invoice-title">{t('invoice.title')}</h2>
              <p className="invoice-order-no font-mono">{invoice.order_no}</p>
            </div>
          </div>

          {/* 客户信息 */}
          <div className="invoice-section">
            <div className="invoice-info-grid">
              <div className="invoice-info-item">
                <span className="invoice-info-label">{t('invoice.customer')}</span>
                <span className="invoice-info-value">{invoice.username}</span>
              </div>
              <div className="invoice-info-item">
                <span className="invoice-info-label">{t('invoice.email')}</span>
                <span className="invoice-info-value">{invoice.email}</span>
              </div>
              <div className="invoice-info-item">
                <span className="invoice-info-label">{t('invoice.orderTime')}</span>
                <span className="invoice-info-value">{invoice.created_at}</span>
              </div>
              {invoice.paid_at && (
                <div className="invoice-info-item">
                  <span className="invoice-info-label">{t('invoice.paidTime')}</span>
                  <span className="invoice-info-value">{invoice.paid_at}</span>
                </div>
              )}
              <div className="invoice-info-item">
                <span className="invoice-info-label">{t('invoice.status')}</span>
                <span className={`invoice-info-value font-medium ${statusColors[invoice.status] || ''}`}>
                  {t(`invoice.statusLabel.${invoice.status}`)}
                </span>
              </div>
              {invoice.payment_channel_name && (
                <div className="invoice-info-item">
                  <span className="invoice-info-label">{t('invoice.paymentMethod')}</span>
                  <span className="invoice-info-value">{invoice.payment_channel_name}</span>
                </div>
              )}
            </div>
          </div>

          {/* 商品明细 */}
          <div className="invoice-section">
            <h3 className="invoice-section-title">{t('invoice.itemsTitle')}</h3>
            <div className="invoice-items">
              <table className="invoice-table">
                <thead>
                  <tr>
                    <th className="invoice-th">{t('invoice.colProduct')}</th>
                    <th className="invoice-th">{t('invoice.colConfig')}</th>
                    <th className="invoice-th">{t('invoice.colInstance')}</th>
                    <th className="invoice-th text-right">{t('invoice.colUnitPrice')}</th>
                    <th className="invoice-th text-center">{t('invoice.colQty')}</th>
                    <th className="invoice-th text-right">{t('invoice.colSubtotal')}</th>
                  </tr>
                </thead>
                <tbody>
                  {invoice.items.map((item, idx) => (
                    <tr key={idx}>
                      <td className="invoice-td">
                        <div className="invoice-product-name">{item.product_name}</div>
                        <div className="invoice-product-sub">
                          {formatPeriod(item.payment_period, t)}
                        </div>
                      </td>
                      <td className="invoice-td">
                        <div className="invoice-config-list">
                          <div className="invoice-config-row">
                            <span className="invoice-config-key">CPU</span>
                            <span className="invoice-config-val font-number">{item.vcpu} {t('invoice.cores')}</span>
                          </div>
                          <div className="invoice-config-row">
                            <span className="invoice-config-key">{t('invoice.memory')}</span>
                            <span className="invoice-config-val font-number">{item.memory_mb} MB</span>
                          </div>
                          <div className="invoice-config-row">
                            <span className="invoice-config-key">{t('invoice.disk')}</span>
                            <span className="invoice-config-val font-number">{(item.disk_mb / 1024).toFixed(1)} GB</span>
                          </div>
                          {item.data_disk_mb > 0 && (
                            <div className="invoice-config-row">
                              <span className="invoice-config-key">{t('invoice.dataDisk')}</span>
                              <span className="invoice-config-val font-number">{(item.data_disk_mb / 1024).toFixed(1)} GB</span>
                            </div>
                          )}
                          <div className="invoice-config-row">
                            <span className="invoice-config-key">{t('invoice.bandwidth')}</span>
                            <span className="invoice-config-val font-number">{item.network_down_mbps}/{item.network_up_mbps} Mbps</span>
                          </div>
                          {item.monthly_traffic_gb > 0 && (
                            <div className="invoice-config-row">
                              <span className="invoice-config-key">{t('invoice.traffic')}</span>
                              <span className="invoice-config-val font-number">{item.monthly_traffic_gb} GB/{t('invoice.month')}</span>
                            </div>
                          )}
                          <div className="invoice-config-row">
                            <span className="invoice-config-key">{t('invoice.image')}</span>
                            <span className="invoice-config-val font-mono text-xs">{item.template_id || item.image_key || '-'}</span>
                          </div>
                          <div className="invoice-config-row">
                            <span className="invoice-config-key">{t('invoice.loginMethod')}</span>
                            <span className="invoice-config-val">{formatLoginMethod(item.login_method, t)}</span>
                          </div>
                        </div>
                      </td>
                      <td className="invoice-td">
                        {item.instance_id ? (
                          <div className="invoice-instance-info">
                            <div className="invoice-instance-name">{item.instance_name}</div>
                            {item.internal_ipv4 && <div className="invoice-instance-ip font-mono text-xs">IPv4: {item.internal_ipv4}</div>}
                            {item.internal_ipv6 && <div className="invoice-instance-ip font-mono text-xs">IPv6: {item.internal_ipv6}</div>}
                            {item.ipv4_mode && item.ipv4_mode !== 'none' && (
                              <div className="invoice-instance-ip text-xs">{t('invoice.ipv4Mode')}: {formatIPMode(item.ipv4_mode, t)}</div>
                            )}
                            {item.ipv6_mode && item.ipv6_mode !== 'none' && (
                              <div className="invoice-instance-ip text-xs">{t('invoice.ipv6Mode')}: {formatIPMode(item.ipv6_mode, t)}</div>
                            )}
                            {item.expires_at && <div className="invoice-instance-ip text-xs">{t('invoice.expiresAt')}: {item.expires_at}</div>}
                          </div>
                        ) : (
                          <span className="text-tertiary">-</span>
                        )}
                      </td>
                      <td className="invoice-td text-right font-number">¥{formatPrice(item.unit_price_cents)}</td>
                      <td className="invoice-td text-center font-number">{item.quantity}</td>
                      <td className="invoice-td text-right font-number font-medium">¥{formatPrice(item.subtotal_cents)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          {/* 金额汇总 */}
          <div className="invoice-summary-section">
            <div className="invoice-summary">
              <div className="invoice-summary-row">
                <span>{t('invoice.subtotal')}</span>
                <span className="font-number">¥{formatPrice(invoice.subtotal_cents)}</span>
              </div>
              {invoice.discount_cents > 0 && (
                <div className="invoice-summary-row invoice-discount">
                  <span>{t('invoice.discount')}</span>
                  <span className="font-number">-¥{formatPrice(invoice.discount_cents)}</span>
                </div>
              )}
              <div className="invoice-summary-row invoice-total">
                <span>{t('invoice.total')}</span>
                <span className="font-number">¥{formatPrice(invoice.total_cents)}</span>
              </div>
            </div>
          </div>

          {/* 账单信息 */}
          {invoice.bill_no && (
            <div className="invoice-bill-info">
              <div className="invoice-bill-row">
                <span className="invoice-bill-label">{t('invoice.billNo')}</span>
                <span className="font-mono text-xs">{invoice.bill_no}</span>
              </div>
              <div className="invoice-bill-row">
                <span className="invoice-bill-label">{t('invoice.billStatus')}</span>
                <span className={statusColors[invoice.bill_status || ''] || ''}>{t(`invoice.statusLabel.${invoice.bill_status || 'unknown'}`)}</span>
              </div>
            </div>
          )}

          {/* 页脚 */}
          <div className="invoice-footer">
            <p>{t('invoice.footerNote')}</p>
            <p className="invoice-footer-time">{t('invoice.generatedAt')}: {new Date().toLocaleString()}</p>
          </div>
        </div>
      </div>
    </main>
  )
}
