import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { billsApi, type Bill, type WalletTransaction } from '@/api/bills'
import { useAuthStore } from '@/stores/auth'
import { Loader2, Receipt, Wallet, ArrowDownCircle, ArrowUpCircle, FileText, ChevronLeft, ChevronRight } from 'lucide-react'
import '@/views/ProductsView.css'

function formatPrice(cents: number): string {
  return (cents / 100).toFixed(2)
}

function formatTime(time: string): string {
  const d = new Date(time)
  return d.toLocaleString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

const statusColors: Record<string, string> = {
  pending: 'text-yellow-500',
  paid: 'text-green-500',
  failed: 'text-red-500',
  cancelled: 'text-gray-500',
  refunded: 'text-blue-500',
}

export function BillsView() {
  const { t } = useTranslation()
  const { user } = useAuthStore()
  const [tab, setTab] = useState<'bills' | 'transactions'>('bills')
  const [bills, setBills] = useState<Bill[]>([])
  const [transactions, setTransactions] = useState<WalletTransaction[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const pageSize = 20

  useEffect(() => {
    fetchData()
  }, [tab, page])

  const fetchData = async () => {
    setLoading(true)
    try {
      if (tab === 'bills') {
        const res = await billsApi.listBills(page, pageSize)
        setBills(res.data.data || [])
        setTotal(res.data.total || 0)
      } else {
        const res = await billsApi.listTransactions(page, pageSize)
        setTransactions(res.data.data || [])
        setTotal(res.data.total || 0)
      }
    } catch {
      // 静默失败
    } finally {
      setLoading(false)
    }
  }

  const totalPages = Math.ceil(total / pageSize)

  return (
    <main className="flex-1 overflow-y-auto overflow-x-hidden p-4 sm:p-6 md:p-8">
      <div className="mx-auto max-w-5xl w-full space-y-6" style={{ animation: 'page-enter 0.25s ease' }}>
        {/* 页头 */}
        <div className="page-header">
          <h1 className="page-title">{t('bills.title')}</h1>
        </div>

        {/* 余额卡片 */}
        <div className="card p-6">
          <div className="flex items-center gap-4">
            <div className="w-12 h-12 rounded-xl bg-surface-secondary flex items-center justify-center">
              <Wallet size={24} className="text-tertiary" />
            </div>
            <div>
              <p className="text-sm text-tertiary">{t('wallet.currentBalance')}</p>
              <p className="text-3xl font-semibold text-primary font-number">
                ¥{formatPrice(user?.balance_cents || 0)}
              </p>
            </div>
          </div>
        </div>

        {/* Tab 切换 */}
        <div className="flex gap-2 border-b border-border">
          <button
            className={`px-4 py-2 text-sm font-medium transition-colors ${tab === 'bills' ? 'text-primary border-b-2 border-primary' : 'text-tertiary hover:text-secondary'}`}
            onClick={() => { setTab('bills'); setPage(1) }}
          >
            <Receipt size={16} style={{ display: 'inline', verticalAlign: 'middle', marginRight: '4px' }} />
            {t('bills.tabBills')}
          </button>
          <button
            className={`px-4 py-2 text-sm font-medium transition-colors ${tab === 'transactions' ? 'text-primary border-b-2 border-primary' : 'text-tertiary hover:text-secondary'}`}
            onClick={() => { setTab('transactions'); setPage(1) }}
          >
            <ArrowDownCircle size={16} style={{ display: 'inline', verticalAlign: 'middle', marginRight: '4px' }} />
            {t('bills.tabTransactions')}
          </button>
        </div>

        {/* 内容区 */}
        {loading ? (
          <div className="flex justify-center py-20">
            <Loader2 className="animate-spin text-muted-foreground" size={28} />
          </div>
        ) : tab === 'bills' ? (
          bills.length === 0 ? (
            <div className="card p-12 text-center">
              <Receipt size={48} className="mx-auto text-tertiary opacity-50" />
              <p className="text-tertiary mt-4">{t('bills.empty')}</p>
            </div>
          ) : (
            <div className="card overflow-hidden">
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border bg-surface-secondary">
                      <th className="px-4 py-3 text-left font-medium text-tertiary">{t('bills.colBillNo')}</th>
                      <th className="px-4 py-3 text-left font-medium text-tertiary">{t('bills.colType')}</th>
                      <th className="px-4 py-3 text-right font-medium text-tertiary">{t('bills.colAmount')}</th>
                      <th className="px-4 py-3 text-left font-medium text-tertiary">{t('bills.colStatus')}</th>
                      <th className="px-4 py-3 text-left font-medium text-tertiary">{t('bills.colChannel')}</th>
                      <th className="px-4 py-3 text-left font-medium text-tertiary">{t('bills.colDescription')}</th>
                      <th className="px-4 py-3 text-left font-medium text-tertiary">{t('bills.colTime')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {bills.map(bill => (
                      <tr key={bill.id} className="border-b border-border last:border-0 hover:bg-surface-secondary/50 transition-colors">
                        <td className="px-4 py-3 font-mono text-xs text-secondary">{bill.bill_no}</td>
                        <td className="px-4 py-3">
                          <span className={`inline-flex items-center gap-1 ${bill.type === 'recharge' ? 'text-green-500' : 'text-orange-500'}`}>
                            {bill.type === 'recharge' ? <ArrowDownCircle size={14} /> : <ArrowUpCircle size={14} />}
                            {bill.type === 'recharge' ? t('bills.typeRecharge') : t('bills.typeConsume')}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-right font-number font-medium">
                          <span className={bill.type === 'recharge' ? 'text-green-500' : 'text-orange-500'}>
                            {bill.type === 'recharge' ? '+' : '-'}¥{formatPrice(bill.amount_cents)}
                          </span>
                        </td>
                        <td className="px-4 py-3">
                          <span className={`font-medium ${statusColors[bill.status] || 'text-tertiary'}`}>
                            {t(`bills.status.${bill.status}`)}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-tertiary">{bill.payment_channel_name || '-'}</td>
                        <td className="px-4 py-3 text-tertiary max-w-xs truncate">{bill.description || '-'}</td>
                        <td className="px-4 py-3 text-tertiary text-xs whitespace-nowrap">{formatTime(bill.created_at)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )
        ) : (
          transactions.length === 0 ? (
            <div className="card p-12 text-center">
              <ArrowDownCircle size={48} className="mx-auto text-tertiary opacity-50" />
              <p className="text-tertiary mt-4">{t('bills.emptyTransactions')}</p>
            </div>
          ) : (
            <div className="card overflow-hidden">
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border bg-surface-secondary">
                      <th className="px-4 py-3 text-left font-medium text-tertiary">{t('bills.colType')}</th>
                      <th className="px-4 py-3 text-right font-medium text-tertiary">{t('bills.colAmount')}</th>
                      <th className="px-4 py-3 text-left font-medium text-tertiary">{t('bills.colDescription')}</th>
                      <th className="px-4 py-3 text-left font-medium text-tertiary">{t('bills.colTime')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {transactions.map(tx => (
                      <tr key={tx.id} className="border-b border-border last:border-0 hover:bg-surface-secondary/50 transition-colors">
                        <td className="px-4 py-3">
                          <span className={`inline-flex items-center gap-1 ${tx.type === 'recharge' ? 'text-green-500' : 'text-orange-500'}`}>
                            {tx.type === 'recharge' ? <ArrowDownCircle size={14} /> : <ArrowUpCircle size={14} />}
                            {tx.type === 'recharge' ? t('bills.typeRecharge') : t('bills.typeConsume')}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-right font-number font-medium">
                          <span className={tx.type === 'recharge' ? 'text-green-500' : 'text-orange-500'}>
                            {tx.type === 'recharge' ? '+' : '-'}¥{formatPrice(tx.amount_cents)}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-tertiary max-w-xs truncate">{tx.description || '-'}</td>
                        <td className="px-4 py-3 text-tertiary text-xs whitespace-nowrap">{formatTime(tx.created_at)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )
        )}

        {/* 分页 */}
        {totalPages > 1 && (
          <div className="flex items-center justify-center gap-2">
            <button
              className="btn btn--ghost btn--sm"
              disabled={page <= 1}
              onClick={() => setPage(p => p - 1)}
            >
              <ChevronLeft size={16} />
            </button>
            <span className="text-sm text-tertiary font-number">{page} / {totalPages}</span>
            <button
              className="btn btn--ghost btn--sm"
              disabled={page >= totalPages}
              onClick={() => setPage(p => p + 1)}
            >
              <ChevronRight size={16} />
            </button>
          </div>
        )}
      </div>
    </main>
  )
}
