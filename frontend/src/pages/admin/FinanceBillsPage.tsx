import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import apiClient from '@/api/client'
import { DataTable, type Column } from '@/components/DataTable/DataTable'
import { PageLayout } from '@/components/PageLayout/PageLayout'
import { SearchInput } from '@/components/SearchInput/SearchInput'
import { FilterBar, type FilterField } from '@/components/FilterBar/FilterBar'

interface Bill {
  id: number
  bill_no: string
  user_id: number
  username: string
  type: string
  status: string
  amount_cents: number
  balance_before: number
  balance_after: number
  payment_channel_name: string
  transaction_no: string
  description: string
  created_at: string
}

function formatDateTime(dateStr?: string | null): string {
  if (!dateStr) return '-'
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return '-'
  const Y = d.getFullYear()
  const M = String(d.getMonth() + 1).padStart(2, '0')
  const D = String(d.getDate()).padStart(2, '0')
  const h = String(d.getHours()).padStart(2, '0')
  const m = String(d.getMinutes()).padStart(2, '0')
  const s = String(d.getSeconds()).padStart(2, '0')
  return `${Y}/${M}/${D} ${h}:${m}:${s}`
}

function formatMoney(cents: number): string {
  return `¥${(cents / 100).toFixed(2)}`
}

export default function FinanceBillsPage() {
  const { t } = useTranslation()
  const [bills, setBills] = useState<Bill[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [page, setPage] = useState(1)
  const [perPage, setPerPage] = useState(20)
  const [search, setSearch] = useState('')
  const [filters, setFilters] = useState<Record<string, string>>({})

  const fetchBills = () => {
    setLoading(true)
    const params: Record<string, any> = { page, per_page: perPage }
    if (search) params.search = search
    if (filters.type) params.filter_type = filters.type
    if (filters.status) params.filter_status = filters.status
    apiClient.get('/finance/bills', { params }).then((res) => {
      setBills(res.data.data || [])
      setTotal(res.data.total || 0)
    }).finally(() => setLoading(false))
  }

  useEffect(() => { fetchBills() }, [page, perPage, search, filters])

  const getBillTypeLabel = (type: string) => {
    switch (type) {
      case 'recharge': return t('finance.billTypeRecharge')
      case 'consume': return t('finance.billTypeConsume')
      case 'refund': return t('finance.billTypeRefund')
      case 'adjustment': return t('finance.billTypeAdjustment')
      case 'subscription': return t('finance.billTypeSubscription')
      default: return type
    }
  }

  const getBillTypeTagClass = (type: string) => {
    switch (type) {
      case 'recharge': return 'data-table-tag data-table-tag--online'
      case 'consume': return 'data-table-tag data-table-tag--offline'
      case 'refund': return 'data-table-tag data-table-tag--disabled'
      case 'adjustment': return 'data-table-tag'
      case 'subscription': return 'data-table-tag data-table-tag--online'
      default: return 'data-table-tag'
    }
  }

  const getBillStatusLabel = (status: string) => {
    switch (status) {
      case 'pending': return t('finance.billStatusPending')
      case 'paid': return t('finance.billStatusPaid')
      case 'failed': return t('finance.billStatusFailed')
      case 'refunded': return t('finance.billStatusRefunded')
      case 'cancelled': return t('finance.billStatusCancelled')
      default: return status
    }
  }

  const getBillStatusTagClass = (status: string) => {
    switch (status) {
      case 'paid': return 'data-table-tag data-table-tag--online'
      case 'pending': return 'data-table-tag'
      case 'failed': return 'data-table-tag data-table-tag--offline'
      case 'refunded': return 'data-table-tag data-table-tag--disabled'
      case 'cancelled': return 'data-table-tag data-table-tag--disabled'
      default: return 'data-table-tag'
    }
  }

  const columns: Column<Bill>[] = [
    {
      key: 'bill_no',
      title: t('finance.billNo'),
      width: 180,
      render: (row) => (
        <span className="text-sm text-primary font-number">{row.bill_no}</span>
      ),
    },
    {
      key: 'username',
      title: t('common.username'),
      width: 120,
      render: (row) => (
        <span className="text-sm text-secondary">{row.username || '-'}</span>
      ),
    },
    {
      key: 'type',
      title: t('finance.billType'),
      width: 90,
      render: (row) => (
        <span className={getBillTypeTagClass(row.type)}>{getBillTypeLabel(row.type)}</span>
      ),
    },
    {
      key: 'amount_cents',
      title: t('finance.amount'),
      width: 110,
      render: (row) => (
        <span className={`font-number text-sm ${row.type === 'recharge' || row.type === 'refund' ? 'text-[#16a34a]' : 'text-[#dc2626]'}`}>
          {row.type === 'recharge' || row.type === 'refund' ? '+' : '-'}{formatMoney(row.amount_cents)}
        </span>
      ),
    },
    {
      key: 'balance_after',
      title: t('finance.balanceAfter'),
      width: 110,
      render: (row) => (
        <span className="font-number text-sm text-primary">{formatMoney(row.balance_after)}</span>
      ),
    },
    {
      key: 'status',
      title: t('common.status'),
      width: 90,
      render: (row) => (
        <span className={getBillStatusTagClass(row.status)}>{getBillStatusLabel(row.status)}</span>
      ),
    },
    {
      key: 'payment_channel_name',
      title: t('finance.paymentChannel'),
      width: 120,
      render: (row) => (
        <span className="text-sm text-secondary">{row.payment_channel_name || '-'}</span>
      ),
    },
    {
      key: 'description',
      title: t('common.description'),
      width: 200,
      render: (row) => (
        <span className="text-sm text-tertiary">{row.description || '-'}</span>
      ),
    },
    {
      key: 'created_at',
      title: t('common.createdAt'),
      width: 160,
      render: (row) => (
        <span className="text-sm text-tertiary font-number">{formatDateTime(row.created_at)}</span>
      ),
    },
  ]

  const typeOptions = [
    { label: t('finance.billTypeRecharge'), value: 'recharge' },
    { label: t('finance.billTypeConsume'), value: 'consume' },
    { label: t('finance.billTypeRefund'), value: 'refund' },
    { label: t('finance.billTypeAdjustment'), value: 'adjustment' },
    { label: t('finance.billTypeSubscription'), value: 'subscription' },
  ]

  const statusOptions = [
    { label: t('finance.billStatusPending'), value: 'pending' },
    { label: t('finance.billStatusPaid'), value: 'paid' },
    { label: t('finance.billStatusFailed'), value: 'failed' },
    { label: t('finance.billStatusRefunded'), value: 'refunded' },
    { label: t('finance.billStatusCancelled'), value: 'cancelled' },
  ]

  return (
    <PageLayout
      leftSlot={
        <>
          <SearchInput value={search} placeholder={t('common.search')} onChange={setSearch} />
          <FilterBar
            fields={[
              { key: 'type', label: t('finance.billType'), options: typeOptions },
              { key: 'status', label: t('common.status'), options: statusOptions },
            ] as FilterField[]}
            values={filters}
            onChange={(key, value) => setFilters((prev) => ({ ...prev, [key]: value }))}
          />
        </>
      }
    >
      <div className="page-transition__content" style={{ flex: 1, overflow: 'auto' }}>
        <DataTable
          columns={columns}
          data={bills}
          rowKey={(r) => r.id}
          loading={loading}
          emptyText={t('common.noData')}
          pagination={{ page, size: perPage, total }}
          onPageChange={setPage}
          onSizeChange={setPerPage}
        />
      </div>
    </PageLayout>
  )
}
