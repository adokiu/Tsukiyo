import { useState, useEffect } from 'react'
import { Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import apiClient from '@/api/client'
import { DataTable, type Column } from '@/components/DataTable/DataTable'
import { Button } from '@/components/Button/Button'
import { PageLayout } from '@/components/PageLayout/PageLayout'
import { SearchInput } from '@/components/SearchInput/SearchInput'
import { SlidePanel } from '@/components/SlidePanel/SlidePanel'
import { Select } from '@/components/Select/Select'
import { useFormValidation } from '@/hooks/useFormValidation'
import { useToastStore } from '@/stores/toast'

interface ConfigFieldOption {
  label: string
  value: string
}

interface ConfigField {
  key: string
  label: string
  type: string
  required: boolean
  placeholder?: string
  default?: string
  options?: ConfigFieldOption[]
  help?: string
}

interface PaymentMethodDef {
  type: string
  name: string
  icon?: string
  description?: string
}

interface PaymentDriverInfo {
  type: string
  name: string
  description: string
  config_fields: ConfigField[]
  supported_methods: PaymentMethodDef[]
  supported_currencies: string[]
}

interface PaymentChannel {
  id: number
  name: string
  type: string
  config: string
  icon: string
  description: string
  fee_bearer: string
  fee_type: string
  fee_fixed_cents: number
  fee_percent: number
  min_amount: number
  max_amount: number
  settlement_currency: string
  exchange_rate_markup: number
  status: string
  sort_order: number
  created_at: string
}

function formatMoney(cents: number): string {
  if (cents <= 0) return '-'
  return `¥${(cents / 100).toFixed(2)}`
}

function formatFeeDescription(channel: PaymentChannel): string {
  switch (channel.fee_type) {
    case 'fixed':
      return formatMoney(channel.fee_fixed_cents)
    case 'percent':
      return `${channel.fee_percent}%`
    case 'fixed_plus_percent':
      return `${formatMoney(channel.fee_fixed_cents)} + ${channel.fee_percent}%`
    default:
      return '-'
  }
}

export default function FinanceChannelsPage() {
  const { t } = useTranslation()
  const toast = useToastStore()
  const [channels, setChannels] = useState<PaymentChannel[]>([])
  const [drivers, setDrivers] = useState<PaymentDriverInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [editChannel, setEditChannel] = useState<PaymentChannel | null>(null)
  const [editOpen, setEditOpen] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)

  const fetchChannels = () => {
    setLoading(true)
    apiClient.get('/finance/payment-channels').then((res) => setChannels(res.data.data || [])).finally(() => setLoading(false))
  }

  const fetchDrivers = () => {
    apiClient.get('/finance/payment-drivers').then((res) => setDrivers(res.data.data || []))
  }

  useEffect(() => { fetchChannels(); fetchDrivers() }, [])

  const handleDelete = async (channel: PaymentChannel) => {
    if (!confirm(t('common.confirm') + t('common.delete') + `: ${channel.name}?`)) return
    try {
      await apiClient.delete(`/finance/payment-channels/${channel.id}`)
      toast.success(t('common.success'))
      fetchChannels()
    } catch {
      toast.error(t('common.error'))
    }
  }

  const handleEdit = (channel: PaymentChannel) => {
    setEditChannel(channel)
    setEditOpen(true)
  }

  const filtered = channels.filter((c) =>
    !search || c.name.toLowerCase().includes(search.toLowerCase()) || c.type.toLowerCase().includes(search.toLowerCase())
  )

  const getDriverName = (type: string) => {
    const d = drivers.find((d) => d.type === type)
    return d ? d.name : type
  }

  const columns: Column<PaymentChannel>[] = [
    {
      key: 'name',
      title: t('finance.channelName'),
      width: 140,
      render: (row) => (
        <span className="text-sm text-primary font-medium">{row.name}</span>
      ),
    },
    {
      key: 'type',
      title: t('finance.channelType'),
      width: 100,
      render: (row) => (
        <span className="data-table-tag">{getDriverName(row.type)}</span>
      ),
    },
    {
      key: 'fee',
      title: t('finance.feeConfig'),
      width: 120,
      render: (row) => (
        <div className="flex items-center gap-2">
          <span className="text-sm text-secondary font-number">{formatFeeDescription(row)}</span>
          <span className="text-xs text-tertiary">
            {row.fee_bearer === 'user' ? t('finance.feeBearerUser') : t('finance.feeBearerMerchant')}
          </span>
        </div>
      ),
    },
    {
      key: 'settlement_currency',
      title: t('finance.settlementCurrency'),
      width: 90,
      render: (row) => (
        <span className="font-number text-sm text-secondary">{row.settlement_currency || 'CNY'}</span>
      ),
    },
    {
      key: 'exchange_rate_markup',
      title: t('finance.exchangeRateMarkup'),
      width: 90,
      render: (row) => (
        <span className="font-number text-sm text-tertiary">
          {row.exchange_rate_markup > 0 ? `+${row.exchange_rate_markup}%` : '-'}
        </span>
      ),
    },
    {
      key: 'status',
      title: t('common.status'),
      width: 80,
      render: (row) => (
        <span className={row.status === 'enabled' ? 'data-table-tag data-table-tag--online' : 'data-table-tag data-table-tag--disabled'}>
          {row.status === 'enabled' ? t('common.active') : t('common.inactive')}
        </span>
      ),
    },
    {
      key: 'actions',
      title: t('common.actions'),
      width: 100,
      render: (row: PaymentChannel) => (
        <div className="flex items-center gap-3">
          <button className="data-table-link-btn" onClick={() => handleEdit(row)}>{t('common.edit')}</button>
          <button
            className="data-table-link-btn"
            onClick={() => handleDelete(row)}
            style={{ color: 'var(--color-red-500)' }}
          >
            {t('common.delete')}
          </button>
        </div>
      ),
    },
  ]

  return (
    <PageLayout
      leftSlot={
        <SearchInput value={search} placeholder={t('common.search')} onChange={setSearch} />
      }
      rightSlot={
        <Button icon={<Plus size={16} />} onClick={() => setCreateOpen(true)}>
          {t('finance.createChannel')}
        </Button>
      }
    >
      <div className="page-transition__content" style={{ flex: 1, overflow: 'auto' }}>
        <DataTable
          columns={columns}
          data={filtered}
          rowKey={(r) => r.id}
          loading={loading}
          emptyText={t('common.noData')}
        />
      </div>

      <EditChannelPanel
        open={editOpen}
        channel={editChannel}
        drivers={drivers}
        onClose={() => { setEditOpen(false); setEditChannel(null) }}
        onSuccess={() => { fetchChannels(); setEditOpen(false); setEditChannel(null) }}
      />

      <CreateChannelPanel
        open={createOpen}
        drivers={drivers}
        onClose={() => setCreateOpen(false)}
        onSuccess={() => { fetchChannels(); setCreateOpen(false) }}
      />
    </PageLayout>
  )
}

// =================== 渠道编辑面板 ===================

function EditChannelPanel({ open, channel, drivers, onClose, onSuccess }: { open: boolean; channel: PaymentChannel | null; drivers: PaymentDriverInfo[]; onClose: () => void; onSuccess: () => void }) {
  const { t } = useTranslation()
  const toast = useToastStore()
  const { validate, hasError, clearError, reset } = useFormValidation()
  const [name, setName] = useState('')
  const [type, setType] = useState('alipay')
  const [configValues, setConfigValues] = useState<Record<string, string>>({})
  const [icon, setIcon] = useState('')
  const [description, setDescription] = useState('')
  const [feeBearer, setFeeBearer] = useState('user')
  const [feeType, setFeeType] = useState('fixed')
  const [feeFixed, setFeeFixed] = useState('0')
  const [feePercent, setFeePercent] = useState('0')
  const [minAmount, setMinAmount] = useState('0')
  const [maxAmount, setMaxAmount] = useState('0')
  const [settlementCurrency, setSettlementCurrency] = useState('CNY')
  const [exchangeRateMarkup, setExchangeRateMarkup] = useState('0')
  const [status, setStatus] = useState('enabled')
  const [sortOrder, setSortOrder] = useState('0')
  const [saving, setSaving] = useState(false)

  const currentDriver = drivers.find((d) => d.type === type)

  useEffect(() => {
    if (channel) {
      setName(channel.name)
      setType(channel.type)
      let parsed: Record<string, string> = {}
      try { parsed = JSON.parse(channel.config || '{}') } catch { parsed = {} }
      setConfigValues(parsed)
      setIcon(channel.icon || '')
      setDescription(channel.description || '')
      setFeeBearer(channel.fee_bearer || 'user')
      setFeeType(channel.fee_type || 'fixed')
      setFeeFixed(String((channel.fee_fixed_cents || 0) / 100))
      setFeePercent(String(channel.fee_percent || 0))
      setMinAmount(String(channel.min_amount / 100))
      setMaxAmount(String(channel.max_amount / 100))
      setSettlementCurrency(channel.settlement_currency || 'CNY')
      setExchangeRateMarkup(String(channel.exchange_rate_markup))
      setStatus(channel.status)
      setSortOrder(String(channel.sort_order))
    }
  }, [channel])

  const handleSave = async () => {
    if (!channel) return
    const result = validate([
      { field: 'name', valid: () => name.trim().length > 0 },
    ])
    if (!result.ok) {
      toast.error(t('common.required'))
      return
    }
    setSaving(true)
    try {
      const payload: Record<string, any> = {
        name,
        type,
        config: JSON.stringify(configValues),
        icon,
        description,
        fee_bearer: feeBearer,
        fee_type: feeType,
        fee_fixed_cents: Math.round(parseFloat(feeFixed) * 100) || 0,
        fee_percent: parseFloat(feePercent) || 0,
        min_amount: Math.round(parseFloat(minAmount) * 100) || 0,
        max_amount: Math.round(parseFloat(maxAmount) * 100) || 0,
        settlement_currency: settlementCurrency,
        exchange_rate_markup: parseFloat(exchangeRateMarkup) || 0,
        status,
        sort_order: parseInt(sortOrder) || 0,
      }
      await apiClient.put(`/finance/payment-channels/${channel.id}`, payload)
      toast.success(t('common.success'))
      onSuccess()
      reset()
    } catch {
      toast.error(t('common.error'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <SlidePanel
      open={open}
      onClose={onClose}
      title={channel ? `${t('common.edit')} - ${channel.name}` : t('common.edit')}
      width={480}
      footer={
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>{t('common.cancel')}</Button>
          <Button onClick={handleSave} loading={saving}>{t('common.save')}</Button>
        </div>
      }
    >
      <ChannelFormBody
        name={name} setName={(v) => { setName(v); clearError('name') }} nameError={hasError('name')}
        type={type} setType={setType}
        drivers={drivers}
        currentDriver={currentDriver}
        configValues={configValues} setConfigValues={setConfigValues}
        icon={icon} setIcon={setIcon}
        description={description} setDescription={setDescription}
        feeBearer={feeBearer} setFeeBearer={setFeeBearer}
        feeType={feeType} setFeeType={setFeeType}
        feeFixed={feeFixed} setFeeFixed={setFeeFixed}
        feePercent={feePercent} setFeePercent={setFeePercent}
        minAmount={minAmount} setMinAmount={setMinAmount}
        maxAmount={maxAmount} setMaxAmount={setMaxAmount}
        settlementCurrency={settlementCurrency} setSettlementCurrency={setSettlementCurrency}
        exchangeRateMarkup={exchangeRateMarkup} setExchangeRateMarkup={setExchangeRateMarkup}
        status={status} setStatus={setStatus}
        sortOrder={sortOrder} setSortOrder={setSortOrder}
        t={t}
      />
    </SlidePanel>
  )
}

// =================== 渠道创建面板 ===================

function CreateChannelPanel({ open, drivers, onClose, onSuccess }: { open: boolean; drivers: PaymentDriverInfo[]; onClose: () => void; onSuccess: () => void }) {
  const { t } = useTranslation()
  const toast = useToastStore()
  const { validate, hasError, clearError, reset } = useFormValidation()
  const [name, setName] = useState('')
  const [type, setType] = useState(drivers[0]?.type || 'alipay')
  const [configValues, setConfigValues] = useState<Record<string, string>>({})
  const [icon, setIcon] = useState('')
  const [description, setDescription] = useState('')
  const [feeBearer, setFeeBearer] = useState('user')
  const [feeType, setFeeType] = useState('fixed')
  const [feeFixed, setFeeFixed] = useState('0')
  const [feePercent, setFeePercent] = useState('0')
  const [minAmount, setMinAmount] = useState('0')
  const [maxAmount, setMaxAmount] = useState('0')
  const [settlementCurrency, setSettlementCurrency] = useState('CNY')
  const [exchangeRateMarkup, setExchangeRateMarkup] = useState('0')
  const [status, setStatus] = useState('enabled')
  const [sortOrder, setSortOrder] = useState('0')
  const [saving, setSaving] = useState(false)

  const currentDriver = drivers.find((d) => d.type === type)

  useEffect(() => {
    if (!open) {
      setName('')
      setType(drivers[0]?.type || 'alipay')
      setConfigValues({})
      setIcon('')
      setDescription('')
      setFeeBearer('user')
      setFeeType('fixed')
      setFeeFixed('0')
      setFeePercent('0')
      setMinAmount('0')
      setMaxAmount('0')
      setSettlementCurrency('CNY')
      setExchangeRateMarkup('0')
      setStatus('enabled')
      setSortOrder('0')
      reset()
    }
  }, [open])

  const handleSave = async () => {
    const result = validate([
      { field: 'name', valid: () => name.trim().length > 0 },
    ])
    if (!result.ok) {
      toast.error(t('common.required'))
      return
    }
    setSaving(true)
    try {
      const payload: Record<string, any> = {
        name,
        type,
        config: JSON.stringify(configValues),
        icon,
        description,
        fee_bearer: feeBearer,
        fee_type: feeType,
        fee_fixed_cents: Math.round(parseFloat(feeFixed) * 100) || 0,
        fee_percent: parseFloat(feePercent) || 0,
        min_amount: Math.round(parseFloat(minAmount) * 100) || 0,
        max_amount: Math.round(parseFloat(maxAmount) * 100) || 0,
        settlement_currency: settlementCurrency,
        exchange_rate_markup: parseFloat(exchangeRateMarkup) || 0,
        status,
        sort_order: parseInt(sortOrder) || 0,
      }
      await apiClient.post('/finance/payment-channels', payload)
      toast.success(t('common.success'))
      onSuccess()
      reset()
    } catch {
      toast.error(t('common.error'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <SlidePanel
      open={open}
      onClose={onClose}
      title={t('finance.createChannel')}
      width={480}
      footer={
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>{t('common.cancel')}</Button>
          <Button onClick={handleSave} loading={saving}>{t('common.create')}</Button>
        </div>
      }
    >
      <ChannelFormBody
        name={name} setName={(v) => { setName(v); clearError('name') }} nameError={hasError('name')}
        type={type} setType={setType}
        drivers={drivers}
        currentDriver={currentDriver}
        configValues={configValues} setConfigValues={setConfigValues}
        icon={icon} setIcon={setIcon}
        description={description} setDescription={setDescription}
        feeBearer={feeBearer} setFeeBearer={setFeeBearer}
        feeType={feeType} setFeeType={setFeeType}
        feeFixed={feeFixed} setFeeFixed={setFeeFixed}
        feePercent={feePercent} setFeePercent={setFeePercent}
        minAmount={minAmount} setMinAmount={setMinAmount}
        maxAmount={maxAmount} setMaxAmount={setMaxAmount}
        settlementCurrency={settlementCurrency} setSettlementCurrency={setSettlementCurrency}
        exchangeRateMarkup={exchangeRateMarkup} setExchangeRateMarkup={setExchangeRateMarkup}
        status={status} setStatus={setStatus}
        sortOrder={sortOrder} setSortOrder={setSortOrder}
        t={t}
      />
    </SlidePanel>
  )
}

// =================== 渠道表单公共部分 ===================

interface ChannelFormProps {
  name: string; setName: (v: string) => void; nameError?: boolean
  type: string; setType: (v: string) => void
  drivers: PaymentDriverInfo[]
  currentDriver?: PaymentDriverInfo
  configValues: Record<string, string>; setConfigValues: (v: Record<string, string>) => void
  icon: string; setIcon: (v: string) => void
  description: string; setDescription: (v: string) => void
  feeBearer: string; setFeeBearer: (v: string) => void
  feeType: string; setFeeType: (v: string) => void
  feeFixed: string; setFeeFixed: (v: string) => void
  feePercent: string; setFeePercent: (v: string) => void
  minAmount: string; setMinAmount: (v: string) => void
  maxAmount: string; setMaxAmount: (v: string) => void
  settlementCurrency: string; setSettlementCurrency: (v: string) => void
  exchangeRateMarkup: string; setExchangeRateMarkup: (v: string) => void
  status: string; setStatus: (v: string) => void
  sortOrder: string; setSortOrder: (v: string) => void
  t: (k: string) => string
}

function ChannelFormBody(props: ChannelFormProps) {
  const { t } = props
  const currencyOptions = (props.currentDriver?.supported_currencies || ['CNY']).map((c) => ({ label: c, value: c }))

  const handleTypeChange = (v: string) => {
    props.setType(v)
    const driver = props.drivers.find((d) => d.type === v)
    if (driver) {
      const defaults: Record<string, string> = {}
      for (const f of driver.config_fields || []) {
        if (f.default) defaults[f.key] = f.default
      }
      props.setConfigValues(defaults)
      const firstCurrency = (driver.supported_currencies || ['CNY'])[0] || 'CNY'
      props.setSettlementCurrency(firstCurrency)
    }
  }

  const setConfigValue = (key: string, value: string) => {
    props.setConfigValues({ ...props.configValues, [key]: value })
  }

  return (
    <div className="space-y-5">
      <div>
        <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('finance.channelName')}</h4>
        <input className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${props.nameError ? 'field-error' : ''}`} value={props.name} onChange={(e) => props.setName(e.target.value)} />
      </div>
      <div>
        <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('finance.channelType')}</h4>
        <Select
          value={props.type}
          onChange={(v) => handleTypeChange(String(v))}
          options={props.drivers.map((d) => ({ label: d.name, value: d.type }))}
        />
      </div>
      {props.currentDriver && props.currentDriver.description && (
        <div className="px-3 py-2 rounded-lg text-xs text-tertiary" style={{ background: 'var(--color-surface-subtle, #f9fafb)' }}>
          {props.currentDriver.description}
        </div>
      )}
      <div>
        <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('finance.methodIcon')}</h4>
        <input className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={props.icon} onChange={(e) => props.setIcon(e.target.value)} placeholder="URL" />
      </div>
      <div>
        <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('common.description')}</h4>
        <textarea className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" rows={2} value={props.description} onChange={(e) => props.setDescription(e.target.value)} />
      </div>

      <div style={{ borderTop: '1px solid var(--color-border, #e5e7eb)', paddingTop: 16, marginTop: 16 }}>
        <h3 className="text-sm font-semibold text-primary mb-4">{t('finance.feeConfig')}</h3>
        <div className="grid grid-cols-2 gap-4 mb-4">
          <div>
            <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('finance.feeBearer')}</h4>
            <Select
              value={props.feeBearer}
              onChange={(v) => props.setFeeBearer(String(v))}
              options={[
                { label: t('finance.feeBearerUser'), value: 'user' },
                { label: t('finance.feeBearerMerchant'), value: 'merchant' },
              ]}
            />
          </div>
          <div>
            <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('finance.feeType')}</h4>
            <Select
              value={props.feeType}
              onChange={(v) => props.setFeeType(String(v))}
              options={[
                { label: t('finance.feeTypeFixed'), value: 'fixed' },
                { label: t('finance.feeTypePercent'), value: 'percent' },
                { label: t('finance.feeTypeFixedPlusPercent'), value: 'fixed_plus_percent' },
              ]}
            />
          </div>
        </div>
        <div className="grid grid-cols-2 gap-4">
          {(props.feeType === 'fixed' || props.feeType === 'fixed_plus_percent') && (
            <div>
              <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('finance.feeFixed')}</h4>
              <div className="relative">
                <input type="number" step="0.01" min="0" className="w-full px-3 py-2 pr-8 border border-surface-strong rounded-lg text-sm font-number" value={props.feeFixed} onChange={(e) => props.setFeeFixed(e.target.value)} />
                <span className="absolute right-3 top-1/2 -translate-y-1/2 text-sm text-tertiary">¥</span>
              </div>
            </div>
          )}
          {(props.feeType === 'percent' || props.feeType === 'fixed_plus_percent') && (
            <div>
              <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('finance.feePercent')}</h4>
              <div className="relative">
                <input type="number" step="0.01" min="0" className="w-full px-3 py-2 pr-8 border border-surface-strong rounded-lg text-sm font-number" value={props.feePercent} onChange={(e) => props.setFeePercent(e.target.value)} />
                <span className="absolute right-3 top-1/2 -translate-y-1/2 text-sm text-tertiary">%</span>
              </div>
            </div>
          )}
        </div>
        {props.feeType === 'fixed_plus_percent' && (
          <div className="mt-3 px-3 py-2 rounded-lg" style={{ background: 'var(--color-surface-subtle, #f9fafb)' }}>
            <span className="text-xs text-tertiary">{t('finance.feePreview')}: </span>
            <span className="text-xs font-number text-secondary">
              ¥{parseFloat(props.feeFixed || '0').toFixed(2)} + {parseFloat(props.feePercent || '0')}%
            </span>
          </div>
        )}
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('finance.settlementCurrency')}</h4>
          <Select
            value={props.settlementCurrency}
            onChange={(v) => props.setSettlementCurrency(String(v))}
            options={currencyOptions}
          />
        </div>
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('finance.exchangeRateMarkup')}</h4>
          <div className="relative">
            <input type="number" step="0.01" min="0" className="w-full px-3 py-2 pr-8 border border-surface-strong rounded-lg text-sm font-number" value={props.exchangeRateMarkup} onChange={(e) => props.setExchangeRateMarkup(e.target.value)} placeholder="0" />
            <span className="absolute right-3 top-1/2 -translate-y-1/2 text-sm text-tertiary">%</span>
          </div>
        </div>
      </div>
      <div className="grid grid-cols-2 gap-4">
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('finance.minAmount')}</h4>
          <input type="number" step="0.01" min="0" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" value={props.minAmount} onChange={(e) => props.setMinAmount(e.target.value)} />
        </div>
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('finance.maxAmount')}</h4>
          <input type="number" step="0.01" min="0" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" value={props.maxAmount} onChange={(e) => props.setMaxAmount(e.target.value)} />
        </div>
      </div>
      <div className="grid grid-cols-2 gap-4">
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('common.status')}</h4>
          <Select
            value={props.status}
            onChange={(v) => props.setStatus(String(v))}
            options={[
              { label: t('common.active'), value: 'enabled' },
              { label: t('common.inactive'), value: 'disabled' },
            ]}
          />
        </div>
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('finance.sortOrder')}</h4>
          <input type="number" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" value={props.sortOrder} onChange={(e) => props.setSortOrder(e.target.value)} />
        </div>
      </div>

      {props.currentDriver && (props.currentDriver.config_fields || []).length > 0 && (
        <div style={{ borderTop: '1px solid var(--color-border, #e5e7eb)', paddingTop: 16, marginTop: 16 }}>
          <h3 className="text-sm font-semibold text-primary mb-4">{t('finance.driverConfig')}</h3>
          <div className="space-y-4">
            {(props.currentDriver.config_fields || []).map((field) => (
              <div key={field.key}>
                <h4 className="text-xs font-semibold text-tertiary uppercase mb-2">
                  {field.label}{field.required && <span style={{ color: 'var(--color-red-500)' }}> *</span>}
                </h4>
                {field.type === 'select' && field.options ? (
                  <Select
                    value={props.configValues[field.key] || field.default || ''}
                    onChange={(v) => setConfigValue(field.key, String(v))}
                    options={field.options.map((o) => ({ label: o.label, value: o.value }))}
                  />
                ) : field.type === 'boolean' ? (
                  <Select
                    value={props.configValues[field.key] || field.default || 'false'}
                    onChange={(v) => setConfigValue(field.key, String(v))}
                    options={[
                      { label: t('common.true'), value: 'true' },
                      { label: t('common.false'), value: 'false' },
                    ]}
                  />
                ) : (
                  <input
                    type={field.type === 'number' ? 'number' : field.type === 'secret' ? 'password' : 'text'}
                    className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number"
                    value={props.configValues[field.key] || field.default || ''}
                    onChange={(e) => setConfigValue(field.key, e.target.value)}
                    placeholder={field.placeholder || ''}
                  />
                )}
                {field.help && <p className="text-xs text-tertiary mt-1">{field.help}</p>}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
