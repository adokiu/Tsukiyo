import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Wallet, TrendingUp, TrendingDown, DollarSign, Loader2, CreditCard, Clock } from 'lucide-react'
import apiClient from '@/api/client'

interface FinanceOverview {
  total_recharge_cents: number
  total_consume_cents: number
  total_refund_cents: number
  month_recharge_cents: number
  month_consume_cents: number
  today_recharge_cents: number
  today_consume_cents: number
  pending_bills_count: number
  total_balance_cents: number
  active_channels_count: number
}

interface DailyStat {
  date: string
  recharge_cents: number
  consume_cents: number
}

function formatMoney(cents: number): string {
  return `¥${(cents / 100).toFixed(2)}`
}

export default function FinanceOverviewPage() {
  const { t } = useTranslation()
  const [overview, setOverview] = useState<FinanceOverview | null>(null)
  const [dailyStats, setDailyStats] = useState<DailyStat[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    apiClient.get('/finance/overview').then((res) => {
      setOverview(res.data.overview)
      setDailyStats(res.data.daily_stats || [])
    }).finally(() => setLoading(false))
  }, [])

  const stats = [
    { label: t('finance.totalRecharge'), value: overview ? formatMoney(overview.total_recharge_cents) : '-', icon: TrendingUp, color: '#16a34a' },
    { label: t('finance.totalConsume'), value: overview ? formatMoney(overview.total_consume_cents) : '-', icon: TrendingDown, color: '#dc2626' },
    { label: t('finance.totalBalance'), value: overview ? formatMoney(overview.total_balance_cents) : '-', icon: Wallet, color: '#087ed1' },
    { label: t('finance.totalRefund'), value: overview ? formatMoney(overview.total_refund_cents) : '-', icon: DollarSign, color: '#f59e0b' },
  ]

  const monthStats = [
    { label: t('finance.monthRecharge'), value: overview ? formatMoney(overview.month_recharge_cents) : '-' },
    { label: t('finance.monthConsume'), value: overview ? formatMoney(overview.month_consume_cents) : '-' },
    { label: t('finance.todayRecharge'), value: overview ? formatMoney(overview.today_recharge_cents) : '-' },
    { label: t('finance.todayConsume'), value: overview ? formatMoney(overview.today_consume_cents) : '-' },
  ]

  const miscStats = [
    { label: t('finance.pendingBills'), value: overview?.pending_bills_count ?? 0, icon: Clock },
    { label: t('finance.activeChannels'), value: overview?.active_channels_count ?? 0, icon: CreditCard },
  ]

  const maxRecharge = Math.max(...dailyStats.map((d) => d.recharge_cents), 1)
  const maxConsume = Math.max(...dailyStats.map((d) => d.consume_cents), 1)
  const maxVal = Math.max(maxRecharge, maxConsume)

  return (
    <div className="page-container">
      <div className="page-header">
        <h1 className="page-title flex items-center gap-2">
          <Wallet size={20} />
          {t('nav.financeOverview')}
        </h1>
      </div>

      {loading && (
        <div className="flex items-center gap-2 text-muted-foreground mb-4">
          <Loader2 size={18} className="animate-spin" />
          <span className="text-sm">{t('common.loading')}</span>
        </div>
      )}

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-5">
        {stats.map((s) => (
          <div key={s.label} className="page-card p-5">
            <div className="flex items-center justify-between mb-3">
              <div className="flex h-9 w-9 items-center justify-center rounded-lg" style={{ background: `${s.color}1a`, color: s.color }}>
                <s.icon size={18} />
              </div>
            </div>
            <p className="text-2xl font-semibold font-number">{s.value}</p>
            <p className="text-sm text-[#8597ab] mt-1">{s.label}</p>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 mb-5">
        <div className="page-card p-5">
          <h3 className="text-base font-semibold mb-4">{t('finance.recentStats')}</h3>
          <div className="grid grid-cols-2 gap-4">
            {monthStats.map((s) => (
              <div key={s.label} className="flex flex-col gap-1">
                <span className="text-sm text-[#8597ab]">{s.label}</span>
                <span className="text-xl font-semibold font-number">{s.value}</span>
              </div>
            ))}
          </div>
        </div>

        <div className="page-card p-5">
          <h3 className="text-base font-semibold mb-4">{t('finance.miscStats')}</h3>
          <div className="grid grid-cols-2 gap-4">
            {miscStats.map((s) => (
              <div key={s.label} className="flex flex-col gap-1">
                <div className="flex items-center gap-2">
                  <s.icon size={16} className="text-[#8597ab]" />
                  <span className="text-sm text-[#8597ab]">{s.label}</span>
                </div>
                <span className="text-xl font-semibold font-number">{s.value}</span>
              </div>
            ))}
          </div>
        </div>
      </div>

      <div className="page-card p-5">
        <h3 className="text-base font-semibold mb-4">{t('finance.dailyStats')}</h3>
        {dailyStats.length > 0 ? (
          <div style={{ display: 'flex', alignItems: 'flex-end', gap: 2, height: 200, overflowX: 'auto' }}>
            {dailyStats.map((d) => (
              <div key={d.date} style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', minWidth: 24, flex: 1 }}>
                <div style={{ display: 'flex', gap: 2, alignItems: 'flex-end', height: 160 }}>
                  <div
                    style={{
                      width: 6,
                      height: maxVal > 0 ? Math.max(2, (d.recharge_cents / maxVal) * 160) : 2,
                      background: '#16a34a',
                      borderRadius: 2,
                    }}
                    title={`${t('finance.recharge')}: ${formatMoney(d.recharge_cents)}`}
                  />
                  <div
                    style={{
                      width: 6,
                      height: maxVal > 0 ? Math.max(2, (d.consume_cents / maxVal) * 160) : 2,
                      background: '#dc2626',
                      borderRadius: 2,
                    }}
                    title={`${t('finance.consume')}: ${formatMoney(d.consume_cents)}`}
                  />
                </div>
                <span style={{ fontSize: 9, color: '#8597ab', marginTop: 4, whiteSpace: 'nowrap' }}>
                  {d.date.slice(5)}
                </span>
              </div>
            ))}
          </div>
        ) : (
          <div className="h-48 flex items-center justify-center text-muted-foreground text-sm">{t('common.noData')}</div>
        )}
        <div className="flex items-center gap-4 mt-4">
          <div className="flex items-center gap-2">
            <div style={{ width: 10, height: 10, background: '#16a34a', borderRadius: 2 }} />
            <span className="text-sm text-[#8597ab]">{t('finance.recharge')}</span>
          </div>
          <div className="flex items-center gap-2">
            <div style={{ width: 10, height: 10, background: '#dc2626', borderRadius: 2 }} />
            <span className="text-sm text-[#8597ab]">{t('finance.consume')}</span>
          </div>
        </div>
      </div>
    </div>
  )
}
