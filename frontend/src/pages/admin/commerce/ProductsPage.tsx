import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Plus, Check } from 'lucide-react'
import apiClient from '@/api/client'
import { PageLayout } from '@/components/PageLayout/PageLayout'
import { Button } from '@/components/Button/Button'
import { Select } from '@/components/Select/Select'
import { SlidePanel } from '@/components/SlidePanel/SlidePanel'
import { SearchInput } from '@/components/SearchInput/SearchInput'
import { DataTable, type Column } from '@/components/DataTable/DataTable'
import { useToastStore } from '@/stores/toast'
import { renderTextWithFlags } from '@/utils/emoji'
import { useFormValidation, type FieldRule } from '@/hooks/useFormValidation'

interface Category {
  id: string
  parent_id: string | null
  name: string
  children?: Category[]
}

interface Product {
  id: string
  category_id: string | null
  name: string
  type: string
  status: string
  sort: number
  node_ids: string[]
  vcpu_min: number
  vcpu_max: number
  vcpu_custom_mode: string
  vcpu_tiers: number[]
  vcpu_unit_price_cents: number
  memory_min_mb: number
  memory_max_mb: number
  memory_custom_mode: string
  memory_tiers: number[]
  memory_unit_price_cents: number
  memory_unit: string
  disk_min_mb: number
  disk_max_mb: number
  disk_custom_mode: string
  disk_tiers: number[]
  disk_unit_price_cents: number
  disk_storage_pools: Record<string, string>
  data_disk_allow: boolean
  data_disk_min_mb: number
  data_disk_max_mb: number
  data_disk_custom_mode: string
  data_disk_tiers: number[]
  data_disk_unit_price_cents: number
  data_disk_storage_pools: Record<string, string>
  network_down_min_mbps: number
  network_down_max_mbps: number
  network_down_custom_mode: string
  network_down_tiers: number[]
  network_up_min_mbps: number
  network_up_max_mbps: number
  network_up_custom_mode: string
  network_up_tiers: number[]
  network_down_unit_price_cents: number
  network_up_unit_price_cents: number
  traffic_min_gb: number
  traffic_max_gb: number
  traffic_calc_mode: string
  traffic_custom_mode: string
  traffic_tiers: number[]
  traffic_unit_price_cents: number
  node_bridges: Record<string, string>
  ipv4_mode: string
  ipv4_eip_pools: Record<string, string[]>
  ipv4_eip_min: number
  ipv4_eip_max: number
  ipv4_eip_custom_mode: string
  ipv4_eip_tiers: number[]
  ipv4_eip_unit_price_cents: number
  ipv4_eip_allow_change: boolean
  ipv4_eip_change_price_cents: number
  nat_port_min: number
  nat_port_max: number
  nat_port_custom_mode: string
  nat_port_tiers: number[]
  nat_port_unit_price_cents: number
  ipv6_enabled: boolean
  ipv6_configs: IPv6ConfigItem[]
  ipv6_eip_pools: Record<string, string[]>
  stock: number
  base_price_cents: number
  min_payment_period: string
  trial_enabled: boolean
  trial_hours: number
  trial_price_cents: number
  quarterly_discount: number
  half_yearly_discount: number
  yearly_discount: number
}

interface IPv6ConfigItem {
  prefix_len: number
  min: number
  max: number
  custom_mode: string
  tiers: number[]
  unit_price_cents: number
}

interface Node {
  id: string
  name: string
}

const defaultForm: Product = {
  id: '',
  category_id: null,
  name: '',
  type: 'container',
  status: 'active',
  sort: 0,
  node_ids: [],
  vcpu_min: 1,
  vcpu_max: 1,
  vcpu_custom_mode: 'unlimited',
  vcpu_tiers: [],
  vcpu_unit_price_cents: 0,
  memory_min_mb: 1024,
  memory_max_mb: 1024,
  memory_custom_mode: 'unlimited',
  memory_tiers: [],
  memory_unit_price_cents: 0,
  memory_unit: 'mb',
  disk_min_mb: 10240,
  disk_max_mb: 10240,
  disk_custom_mode: 'unlimited',
  disk_tiers: [],
  disk_unit_price_cents: 0,
  disk_storage_pools: {},
  data_disk_allow: true,
  data_disk_min_mb: 0,
  data_disk_max_mb: 0,
  data_disk_custom_mode: 'unlimited',
  data_disk_tiers: [],
  data_disk_unit_price_cents: 0,
  data_disk_storage_pools: {},
  network_down_min_mbps: 100,
  network_down_max_mbps: 100,
  network_down_custom_mode: 'unlimited',
  network_down_tiers: [],
  network_up_min_mbps: 100,
  network_up_max_mbps: 100,
  network_up_custom_mode: 'unlimited',
  network_up_tiers: [],
  network_down_unit_price_cents: 0,
  network_up_unit_price_cents: 0,
  traffic_min_gb: 0,
  traffic_max_gb: 0,
  traffic_calc_mode: 'both',
  traffic_custom_mode: 'unlimited',
  traffic_tiers: [],
  traffic_unit_price_cents: 0,
  node_bridges: {},
  ipv4_mode: 'nat',
  ipv4_eip_pools: {},
  ipv4_eip_min: 0,
  ipv4_eip_max: 0,
  ipv4_eip_custom_mode: 'unlimited',
  ipv4_eip_tiers: [],
  ipv4_eip_unit_price_cents: 0,
  ipv4_eip_allow_change: false,
  ipv4_eip_change_price_cents: 0,
  nat_port_min: 0,
  nat_port_max: 0,
  nat_port_custom_mode: 'unlimited',
  nat_port_tiers: [],
  nat_port_unit_price_cents: 0,
  ipv6_enabled: false,
  ipv6_configs: [],
  ipv6_eip_pools: {},
  stock: 0,
  base_price_cents: 0,
  min_payment_period: 'monthly',
  trial_enabled: false,
  trial_hours: 0,
  trial_price_cents: 0,
  quarterly_discount: 1.0,
  half_yearly_discount: 1.0,
  yearly_discount: 1.0,
}

const centsToYuan = (cents: number) => (cents / 100).toFixed(2)

export default function ProductsPage() {
  const { t } = useTranslation()
  const toast = useToastStore()
  const [products, setProducts] = useState<Product[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize] = useState(20)
  const [search, setSearch] = useState('')
  const [loading, setLoading] = useState(false)
  const [categories, setCategories] = useState<Category[]>([])
  const [nodes, setNodes] = useState<Node[]>([])
  const [panelOpen, setPanelOpen] = useState(false)
  const [editing, setEditing] = useState(false)
  const [saving, setSaving] = useState(false)
  const [form, setForm] = useState<Product>(defaultForm)
  const [step, setStep] = useState(0)
  const [nodeStorages, setNodeStorages] = useState<Record<string, { name: string; driver: string }[]>>({})
  const [nodeBridges, setNodeBridges] = useState<Record<string, { id: string; name: string; bridge_name: string; ipv4_enabled: boolean; ipv6_enabled: boolean }[]>>({})
  const { validate, clearError, reset: resetErrors, hasError } = useFormValidation()
  const [tiersText, setTiersText] = useState<Record<string, string>>({})

  const fetchProducts = useCallback(async () => {
    setLoading(true)
    try {
      const params: Record<string, string | number> = { page, per_page: pageSize }
      if (search) params.search = search
      const res = await apiClient.get('/commerce/products', { params })
      setProducts(res.data.data || [])
      setTotal(res.data.total || 0)
    } catch {
      toast.error(t('common.error'))
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, search, t, toast])

  const fetchCategories = useCallback(async () => {
    try {
      const res = await apiClient.get('/commerce/categories')
      setCategories(res.data.data || [])
    } catch {
      // ignore
    }
  }, [])

  const fetchNodes = useCallback(async () => {
    try {
      const res = await apiClient.get('/nodes')
      const data = res.data.data || res.data || []
      setNodes(data.map((n: any) => ({ id: n.id, name: n.name || n.hostname || n.id })))
    } catch {
      // ignore
    }
  }, [])

  useEffect(() => {
    fetchProducts()
  }, [fetchProducts])

  useEffect(() => {
    fetchCategories()
    fetchNodes()
  }, [fetchCategories, fetchNodes])

  useEffect(() => {
    if (!panelOpen) return
    for (const nid of form.node_ids || []) {
      if (nodeStorages[nid]) continue
      apiClient.get(`/nodes/${nid}/storages`).then((res) => {
        const data = res.data.data || res.data || []
        setNodeStorages((prev) => ({ ...prev, [nid]: data.map((s: any) => ({ name: s.name, driver: s.driver })) }))
      }).catch(() => {
        setNodeStorages((prev) => ({ ...prev, [nid]: [] }))
      })
      if (nodeBridges[nid]) continue
      apiClient.get(`/nodes/${nid}/bridges`).then((res) => {
        const data = res.data.data || []
        setNodeBridges((prev) => ({ ...prev, [nid]: data.map((b: any) => ({ id: b.id, name: b.name, bridge_name: b.bridge_name, ipv4_enabled: b.ipv4_enabled, ipv6_enabled: b.ipv6_enabled })) }))
      }).catch(() => {
        setNodeBridges((prev) => ({ ...prev, [nid]: [] }))
      })
    }
  }, [panelOpen, form.node_ids])

  const flattenCategories = (cats: Category[], prefix: string = ''): { id: string; name: string }[] => {
    let result: { id: string; name: string }[] = []
    for (const cat of cats) {
      const name = prefix ? `${prefix} / ${cat.name}` : cat.name
      result.push({ id: cat.id, name })
      if (cat.children) {
        result = result.concat(flattenCategories(cat.children, name))
      }
    }
    return result
  }

  const flatCategories = flattenCategories(categories)

  const openCreate = () => {
    setForm(defaultForm)
    setTiersText({})
    setEditing(false)
    setStep(0)
    resetErrors()
    setPanelOpen(true)
  }

  const openEdit = (p: Product) => {
    const merged = { ...p, node_ids: p.node_ids || [], vcpu_tiers: p.vcpu_tiers || [], memory_tiers: p.memory_tiers || [], disk_tiers: p.disk_tiers || [], data_disk_tiers: p.data_disk_tiers || [], network_down_tiers: p.network_down_tiers || [], network_up_tiers: p.network_up_tiers || [], traffic_tiers: p.traffic_tiers || [], nat_port_tiers: p.nat_port_tiers || [], ipv4_eip_tiers: p.ipv4_eip_tiers || [], ipv6_configs: p.ipv6_configs || [], disk_storage_pools: p.disk_storage_pools || {}, data_disk_storage_pools: p.data_disk_storage_pools || {}, node_bridges: p.node_bridges || {}, ipv4_eip_pools: p.ipv4_eip_pools || {}, ipv6_eip_pools: p.ipv6_eip_pools || {} }
    setForm(merged)
    setTiersText({
      vcpu_tiers: (merged.vcpu_tiers || []).join(','),
      memory_tiers: (merged.memory_tiers || []).join(','),
      disk_tiers: (merged.disk_tiers || []).join(','),
      data_disk_tiers: (merged.data_disk_tiers || []).join(','),
      network_down_tiers: (merged.network_down_tiers || []).join(','),
      network_up_tiers: (merged.network_up_tiers || []).join(','),
      traffic_tiers: (merged.traffic_tiers || []).join(','),
      nat_port_tiers: (merged.nat_port_tiers || []).join(','),
    })
    setEditing(true)
    setStep(0)
    resetErrors()
    setPanelOpen(true)
  }

  // 构建商品表单校验规则（按步骤组织，保存时统一校验）
  const buildRules = (): FieldRule[] => {
    const rules: FieldRule[] = []
    // 步骤1 基本信息
    rules.push({ field: 'name', step: 0, valid: () => form.name.trim().length > 0 })
    // 步骤2 承载宿主机：至少选一个
    rules.push({ field: 'node_ids', step: 1, valid: () => (form.node_ids || []).length > 0 })
    // 步骤3 计算资源：min<=max，挡位模式需有挡位
    rules.push({ field: 'vcpu_range', step: 2, valid: () => form.vcpu_min > 0 && form.vcpu_max >= form.vcpu_min })
    rules.push({ field: 'vcpu_tiers', step: 2, valid: () => form.vcpu_custom_mode !== 'tiers' || form.vcpu_tiers.length > 0 })
    rules.push({ field: 'memory_range', step: 2, valid: () => form.memory_min_mb > 0 && form.memory_max_mb >= form.memory_min_mb })
    rules.push({ field: 'memory_tiers', step: 2, valid: () => form.memory_custom_mode !== 'tiers' || form.memory_tiers.length > 0 })
    // 步骤4 存储：系统盘 min<=max，挡位模式需有挡位，每个选中节点需选存储池
    rules.push({ field: 'disk_range', step: 3, valid: () => form.disk_min_mb > 0 && form.disk_max_mb >= form.disk_min_mb })
    rules.push({ field: 'disk_tiers', step: 3, valid: () => form.disk_custom_mode !== 'tiers' || form.disk_tiers.length > 0 })
    rules.push({ field: 'disk_storage_pools', step: 3, valid: () => (form.node_ids || []).every((nid) => !!(form.disk_storage_pools[nid] || '').trim()) })
    if (form.data_disk_allow) {
      rules.push({ field: 'data_disk_range', step: 3, valid: () => form.data_disk_max_mb >= form.data_disk_min_mb })
      rules.push({ field: 'data_disk_tiers', step: 3, valid: () => form.data_disk_custom_mode !== 'tiers' || form.data_disk_tiers.length > 0 })
      rules.push({ field: 'data_disk_storage_pools', step: 3, valid: () => (form.node_ids || []).every((nid) => !!(form.data_disk_storage_pools[nid] || '').trim()) })
    }
    // 步骤5 网络：每个选中节点需选网桥，带宽 min<=max，流量 min<=max
    rules.push({ field: 'node_bridges', step: 4, valid: () => (form.node_ids || []).every((nid) => !!(form.node_bridges[nid] || '').trim()) })
    rules.push({ field: 'network_down_range', step: 4, valid: () => form.network_down_min_mbps > 0 && form.network_down_max_mbps >= form.network_down_min_mbps })
    rules.push({ field: 'network_up_range', step: 4, valid: () => form.network_up_min_mbps >= 0 && form.network_up_max_mbps >= form.network_up_min_mbps })
    rules.push({ field: 'traffic_range', step: 4, valid: () => form.traffic_min_gb >= 0 && form.traffic_max_gb >= form.traffic_min_gb })
    // 步骤6 价格：基础价格需大于0
    rules.push({ field: 'base_price', step: 5, valid: () => form.base_price_cents > 0 })
    return rules
  }

  const handleSubmit = async () => {
    // 保存时统一校验，失败跳到第一个错误步骤并标红
    const result = validate(buildRules())
    if (!result.ok) {
      if (result.firstErrorStep !== undefined) setStep(result.firstErrorStep)
      toast.error(t('common.required'))
      return
    }
    setSaving(true)
    try {
      if (editing) {
        await apiClient.put(`/commerce/products/${form.id}`, form)
        toast.success(t('common.success'))
      } else {
        await apiClient.post('/commerce/products', form)
        toast.success(t('common.success'))
      }
      setPanelOpen(false)
      fetchProducts()
    } catch {
      toast.error(t('common.error'))
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm(t('common.confirmDelete'))) return
    try {
      await apiClient.delete(`/commerce/products/${id}`)
      toast.success(t('common.success'))
      fetchProducts()
    } catch {
      toast.error(t('common.error'))
    }
  }

  const toggleNodeId = (nodeId: string) => {
    setForm((f) => {
      const ids = f.node_ids || []
      return {
        ...f,
        node_ids: ids.includes(nodeId) ? ids.filter((id) => id !== nodeId) : [...ids, nodeId],
      }
    })
  }

  const update = (field: keyof Product, value: any) => {
    setForm((f) => ({ ...f, [field]: value }))
  }

  const updateTiersText = (field: string, text: string) => {
    setTiersText((prev) => ({ ...prev, [field]: text }))
  }

  const commitTiers = (field: keyof Product) => {
    const text = tiersText[field as string] || ''
    const parsed = text.split(',').map(s => parseInt(s.trim())).filter(n => !isNaN(n))
    update(field, parsed)
  }

  const errorSteps = new Set<number>()
  for (const rule of buildRules()) {
    if (hasError(rule.field) && rule.step !== undefined) {
      errorSteps.add(rule.step)
    }
  }

  const columns: Column<Product>[] = [
    {
      key: 'name',
      title: t('common.name'),
      width: 200,
      render: (row) => <span className="text-sm text-primary font-medium">{row.name}</span>,
    },
    {
      key: 'type',
      title: t('common.type'),
      width: 100,
      render: (row) => (
        <span className="data-table-tag data-table-tag--disabled">
          {row.type === 'vm' ? 'VM' : t('common.container')}
        </span>
      ),
    },
    {
      key: 'vcpu',
      title: 'CPU',
      width: 80,
      render: (row) => <span className="font-number text-sm text-primary">{row.vcpu_min === row.vcpu_max ? `${row.vcpu_min}C` : `${row.vcpu_min}-${row.vcpu_max}C`}</span>,
    },
    {
      key: 'memory_mb',
      title: t('common.memory'),
      width: 100,
      render: (row) => {
        const fmt = (mb: number) => mb >= 1024 ? `${(mb / 1024).toFixed(0)}GB` : `${mb}MB`
        return <span className="font-number text-sm text-primary">{row.memory_min_mb === row.memory_max_mb ? fmt(row.memory_min_mb) : `${fmt(row.memory_min_mb)}-${fmt(row.memory_max_mb)}`}</span>
      },
    },
    {
      key: 'ipv4_mode',
      title: 'IPv4',
      width: 80,
      render: (row) => (
        <span className="data-table-tag data-table-tag--disabled">
          {row.ipv4_mode === 'nat' ? 'NAT' : 'EIP'}
        </span>
      ),
    },
    {
      key: 'base_price_cents',
      title: t('common.price'),
      width: 100,
      render: (row) => <span className="font-number text-sm text-primary">{centsToYuan(row.base_price_cents)}/mo</span>,
    },
    {
      key: 'status',
      title: t('common.status'),
      width: 90,
      render: (row) => (
        <span className={row.status === 'active' ? 'data-table-tag data-table-tag--online' : 'data-table-tag data-table-tag--disabled'}>
          {row.status === 'active' ? t('common.active') : t('common.disabled')}
        </span>
      ),
    },
    {
      key: 'actions',
      title: t('common.actions'),
      width: 120,
      render: (row: Product) => (
        <div className="flex items-center gap-3">
          <button className="data-table-link-btn" onClick={() => openEdit(row)}>{t('common.edit')}</button>
          <button className="data-table-link-btn" onClick={() => handleDelete(row.id)} style={{ color: 'var(--color-red-500)' }}>{t('common.delete')}</button>
        </div>
      ),
    },
  ]

  return (
    <PageLayout
      leftSlot={<SearchInput value={search} placeholder={t('common.search')} onChange={(v) => { setSearch(v); setPage(1) }} />}
      rightSlot={<Button icon={<Plus size={16} />} onClick={openCreate}>{t('common.add')}</Button>}
    >
      <div className="page-transition__content" style={{ flex: 1, overflow: 'auto' }}>
        <DataTable
          columns={columns}
          data={products}
          rowKey={(r) => r.id}
          loading={loading}
          emptyText={t('common.noData')}
          pagination={{ page, size: pageSize, total }}
          onPageChange={setPage}
        />
      </div>

      <SlidePanel
        open={panelOpen}
        onClose={() => setPanelOpen(false)}
        title={editing ? `${t('common.edit')} - ${form.name}` : t('common.add')}
        width={720}
        footer={
          <div className="flex items-center justify-between w-full">
            <div className="flex items-center gap-2">
              {step > 0 && (
                <Button variant="ghost" onClick={() => setStep(step - 1)}>
                  {t('product.prevStep')}
                </Button>
              )}
            </div>
            <div className="flex items-center gap-2">
              <Button variant="ghost" onClick={() => setPanelOpen(false)}>{t('common.cancel')}</Button>
              {step < 5 ? (
                <Button onClick={() => setStep(step + 1)}>{t('product.nextStep')}</Button>
              ) : (
                <Button onClick={handleSubmit} loading={saving}>{t('common.confirm')}</Button>
              )}
            </div>
          </div>
        }
      >
        <div className="slide-steps">
          {[0, 1, 2, 3, 4, 5].map((s, idx, arr) => (
            <div key={s} className="flex items-center" style={{ flexShrink: 0 }}>
              <button
                className={`slide-step ${step === s ? 'slide-step--active' : ''} ${step > s ? 'slide-step--done' : ''} ${errorSteps.has(s) ? 'slide-step--error' : ''}`}
                onClick={() => setStep(s)}
              >
                <span className="slide-step__num">
                  {step > s ? <Check size={12} /> : s + 1}
                </span>
                {t(`product.step${['BasicInfo', 'HostNodes', 'Compute', 'Storage', 'Network', 'Pricing'][s]}`)}
              </button>
              {idx < arr.length - 1 && <span className="slide-step__divider" />}
            </div>
          ))}
        </div>

        <div className="slide-step-content">
          {/* 步骤1: 基本信息 */}
          {step === 0 && (
            <>
              <div className="slide-step-section">
                <div className="slide-step-section__title">{t('product.stepBasicInfo')}</div>
                <div className="slide-form-grid slide-form-grid--2">
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('common.name')}</span>
                    <input className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('name') ? 'field-error' : ''}`} value={form.name} onChange={(e) => { update('name', e.target.value); clearError('name') }} />
                  </div>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.category')}</span>
                    <Select
                      value={form.category_id || ''}
                      options={[{ label: '-', value: '' }, ...flatCategories.map((c) => ({ label: renderTextWithFlags(c.name), value: c.id }))]}
                      onChange={(v) => update('category_id', v || null)}
                    />
                  </div>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('common.type')}</span>
                    <Select
                      value={form.type}
                      options={[
                        { label: t('common.container'), value: 'container' },
                        { label: 'VM', value: 'vm' },
                      ]}
                      onChange={(v) => update('type', v)}
                    />
                  </div>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">Sort</span>
                    <input type="number" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={form.sort} onChange={(e) => update('sort', parseInt(e.target.value) || 0)} />
                  </div>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.stock')}</span>
                    <input type="number" min="0" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={form.stock} onChange={(e) => update('stock', parseInt(e.target.value) || 0)} placeholder="0" />
                  </div>
                </div>
                {editing && (
                  <div className="slide-form-item" style={{ maxWidth: 200 }}>
                    <span className="slide-form-item__label">{t('common.status')}</span>
                    <Select
                      value={form.status}
                      options={[
                        { label: t('common.active'), value: 'active' },
                        { label: t('common.disabled'), value: 'disabled' },
                      ]}
                      onChange={(v) => update('status', v)}
                    />
                  </div>
                )}
              </div>
            </>
          )}

          {/* 步骤2: 承载宿主机 */}
          {step === 1 && (
            <div className="slide-step-section">
              <div className="slide-step-section__title">{t('product.stepHostNodes')}</div>
              <div className={`slide-node-chips ${hasError('node_ids') ? 'field-error' : ''}`} style={hasError('node_ids') ? { borderColor: 'var(--color-red-500)', borderRadius: '8px', padding: '8px' } : {}}>
                {nodes.map((n) => (
                  <label
                    key={n.id}
                    className={`slide-node-chip ${(form.node_ids || []).includes(n.id) ? 'slide-node-chip--active' : ''}`}
                  >
                    <input type="checkbox" checked={(form.node_ids || []).includes(n.id)} onChange={() => { toggleNodeId(n.id); clearError('node_ids') }} className="hidden" />
                    {n.name}
                  </label>
                ))}
                {nodes.length === 0 && <span className="text-xs text-tertiary">{t('common.noData')}</span>}
              </div>
            </div>
          )}

          {/* 步骤3: 计算资源 (CPU + 内存) */}
          {step === 2 && (
            <>
              <div className="slide-step-section">
                <div className="slide-step-section__title">CPU</div>
                <div className="slide-form-grid slide-form-grid--4">
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.minCores')}</span>
                    <input type="number" min="1" className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('vcpu_range') ? 'field-error' : ''}`} value={form.vcpu_min} onChange={(e) => { update('vcpu_min', parseInt(e.target.value) || 1); clearError('vcpu_range') }} />
                  </div>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.maxCores')}</span>
                    <input type="number" min="1" className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('vcpu_range') ? 'field-error' : ''}`} value={form.vcpu_max} onChange={(e) => { update('vcpu_max', parseInt(e.target.value) || 1); clearError('vcpu_range') }} />
                  </div>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.customMode')}</span>
                    <Select
                      value={form.vcpu_custom_mode}
                      options={[
                        { label: t('product.customUnlimited'), value: 'unlimited' },
                        { label: t('product.customTiers'), value: 'tiers' },
                      ]}
                      onChange={(v) => update('vcpu_custom_mode', v)}
                    />
                  </div>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.unitPrice')}({t('common.yuan')})</span>
                    <input type="number" step="0.01" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={form.vcpu_unit_price_cents / 100} onChange={(e) => update('vcpu_unit_price_cents', Math.round(parseFloat(e.target.value) * 100))} />
                  </div>
                </div>
                {form.vcpu_custom_mode === 'tiers' && (
                  <div className="slide-form-grid slide-form-grid--1">
                    <div className="slide-form-item">
                      <span className="slide-form-item__label">{t('product.tiers')}</span>
                      <input
                        className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('vcpu_tiers') ? 'field-error' : ''}`}
                        placeholder={t('product.tiersHint')}
                        value={tiersText['vcpu_tiers'] ?? ''}
                        onChange={(e) => updateTiersText('vcpu_tiers', e.target.value)}
                        onBlur={() => { commitTiers('vcpu_tiers'); clearError('vcpu_tiers') }}
                      />
                    </div>
                  </div>
                )}
              </div>

              <div className="slide-step-section">
                <div className="slide-step-section__title">{t('common.memory')}</div>
                <div className="slide-form-grid slide-form-grid--4">
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.minMemory')} (MB)</span>
                    <input type="number" className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('memory_range') ? 'field-error' : ''}`} value={form.memory_min_mb} onChange={(e) => { update('memory_min_mb', parseInt(e.target.value) || 0); clearError('memory_range') }} />
                  </div>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.maxMemory')} (MB)</span>
                    <input type="number" className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('memory_range') ? 'field-error' : ''}`} value={form.memory_max_mb} onChange={(e) => { update('memory_max_mb', parseInt(e.target.value) || 0); clearError('memory_range') }} />
                  </div>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.memoryUnit')}</span>
                    <Select
                      value={form.memory_unit}
                      options={[
                        { label: t('product.memoryUnitMB'), value: 'mb' },
                        { label: t('product.memoryUnitGB'), value: 'gb' },
                      ]}
                      onChange={(v) => update('memory_unit', v)}
                    />
                  </div>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.unitPrice')}({t('common.yuan')}/{form.memory_unit === 'gb' ? 'GB' : 'MB'})</span>
                    <input type="number" step="0.01" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={form.memory_unit_price_cents / 100} onChange={(e) => update('memory_unit_price_cents', Math.round(parseFloat(e.target.value) * 100))} />
                  </div>
                </div>
                <div className="slide-form-grid slide-form-grid--2">
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.customMode')}</span>
                    <Select
                      value={form.memory_custom_mode}
                      options={[
                        { label: t('product.customUnlimited'), value: 'unlimited' },
                        { label: t('product.customTiers'), value: 'tiers' },
                      ]}
                      onChange={(v) => update('memory_custom_mode', v)}
                    />
                  </div>
                  {form.memory_custom_mode === 'tiers' && (
                    <div className="slide-form-item">
                      <span className="slide-form-item__label">{t('product.tiers')} (MB)</span>
                      <input
                        className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('memory_tiers') ? 'field-error' : ''}`}
                        placeholder={t('product.tiersHint')}
                        value={tiersText['memory_tiers'] ?? ''}
                        onChange={(e) => updateTiersText('memory_tiers', e.target.value)}
                        onBlur={() => { commitTiers('memory_tiers'); clearError('memory_tiers') }}
                      />
                    </div>
                  )}
                </div>
              </div>
            </>
          )}

          {/* 步骤4: 存储配置 (系统盘 + 数据盘) */}
          {step === 3 && (
            <>
              <div className="slide-step-section">
                <div className="slide-step-section__title">{t('product.systemDisk')}</div>
                <div className="slide-form-grid slide-form-grid--4">
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.minDiskSize')} (MB)</span>
                    <input type="number" className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('disk_range') ? 'field-error' : ''}`} value={form.disk_min_mb} onChange={(e) => { update('disk_min_mb', parseInt(e.target.value) || 0); clearError('disk_range') }} />
                  </div>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.maxDiskSize')} (MB)</span>
                    <input type="number" className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('disk_range') ? 'field-error' : ''}`} value={form.disk_max_mb} onChange={(e) => { update('disk_max_mb', parseInt(e.target.value) || 0); clearError('disk_range') }} />
                  </div>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.customMode')}</span>
                    <Select
                      value={form.disk_custom_mode}
                      options={[
                        { label: t('product.customUnlimited'), value: 'unlimited' },
                        { label: t('product.customTiers'), value: 'tiers' },
                      ]}
                      onChange={(v) => update('disk_custom_mode', v)}
                    />
                  </div>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.unitPrice')}({t('common.yuan')})</span>
                    <input type="number" step="0.01" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={form.disk_unit_price_cents / 100} onChange={(e) => update('disk_unit_price_cents', Math.round(parseFloat(e.target.value) * 100))} />
                  </div>
                </div>
                {form.disk_custom_mode === 'tiers' && (
                  <div className="slide-form-grid slide-form-grid--1">
                    <div className="slide-form-item">
                      <span className="slide-form-item__label">{t('product.tiers')} (MB)</span>
                      <input
                        className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('disk_tiers') ? 'field-error' : ''}`}
                        placeholder={t('product.tiersHint')}
                        value={tiersText['disk_tiers'] ?? ''}
                        onChange={(e) => updateTiersText('disk_tiers', e.target.value)}
                        onBlur={() => { commitTiers('disk_tiers'); clearError('disk_tiers') }}
                      />
                    </div>
                  </div>
                )}
                <div>
                  <span className="slide-form-item__label" style={{ display: 'block', marginBottom: 8 }}>{t('product.storagePoolPerNode')}</span>
                  <div className="space-y-2">
                    {(form.node_ids || []).map((nid) => {
                      const node = nodes.find((n) => n.id === nid)
                      const pools = nodeStorages[nid] || []
                      return (
                        <div key={nid} className="slide-storage-pool-row">
                          <span className="slide-storage-pool-row__name">{node?.name || nid}</span>
                          <Select
                            value={form.disk_storage_pools[nid] || ''}
                            error={hasError('disk_storage_pools')}
                            options={[{ label: '-', value: '' }, ...pools.map((p) => ({ label: `${p.name} (${p.driver})`, value: p.name }))]}
                            onChange={(v) => { update('disk_storage_pools', { ...form.disk_storage_pools, [nid]: v }); clearError('disk_storage_pools') }}
                          />
                        </div>
                      )
                    })}
                  </div>
                </div>
              </div>

              <div className="slide-step-section">
                <div className="slide-step-section__title">{t('product.dataDisk')}</div>
                <div className="slide-form-grid slide-form-grid--4">
                  <div className="slide-checkbox-row">
                    <label>
                      <input type="checkbox" checked={form.data_disk_allow} onChange={(e) => update('data_disk_allow', e.target.checked)} />
                      {t('product.allowDataDisk')}
                    </label>
                  </div>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.minDiskSize')} (MB)</span>
                    <input type="number" className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('data_disk_range') ? 'field-error' : ''}`} value={form.data_disk_min_mb} onChange={(e) => { update('data_disk_min_mb', parseInt(e.target.value) || 0); clearError('data_disk_range') }} />
                  </div>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.maxDiskSize')} (MB)</span>
                    <input type="number" className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('data_disk_range') ? 'field-error' : ''}`} value={form.data_disk_max_mb} onChange={(e) => { update('data_disk_max_mb', parseInt(e.target.value) || 0); clearError('data_disk_range') }} />
                  </div>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.customMode')}</span>
                    <Select
                      value={form.data_disk_custom_mode}
                      options={[
                        { label: t('product.customUnlimited'), value: 'unlimited' },
                        { label: t('product.customTiers'), value: 'tiers' },
                      ]}
                      onChange={(v) => update('data_disk_custom_mode', v)}
                    />
                  </div>
                </div>
                <div className="slide-form-grid slide-form-grid--2">
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.unitPrice')}({t('common.yuan')})</span>
                    <input type="number" step="0.01" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={form.data_disk_unit_price_cents / 100} onChange={(e) => update('data_disk_unit_price_cents', Math.round(parseFloat(e.target.value) * 100))} />
                  </div>
                  {form.data_disk_custom_mode === 'tiers' && (
                    <div className="slide-form-item">
                      <span className="slide-form-item__label">{t('product.tiers')} (MB)</span>
                      <input
                        className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('data_disk_tiers') ? 'field-error' : ''}`}
                        placeholder={t('product.tiersHint')}
                        value={tiersText['data_disk_tiers'] ?? ''}
                        onChange={(e) => updateTiersText('data_disk_tiers', e.target.value)}
                        onBlur={() => { commitTiers('data_disk_tiers'); clearError('data_disk_tiers') }}
                      />
                    </div>
                  )}
                </div>
                {form.data_disk_allow && (
                  <div>
                    <span className="slide-form-item__label" style={{ display: 'block', marginBottom: 8 }}>{t('product.storagePoolPerNode')}</span>
                    <div className="space-y-2">
                      {(form.node_ids || []).map((nid) => {
                        const node = nodes.find((n) => n.id === nid)
                        const pools = nodeStorages[nid] || []
                        return (
                          <div key={nid} className="slide-storage-pool-row">
                            <span className="slide-storage-pool-row__name">{node?.name || nid}</span>
                            <Select
                              value={form.data_disk_storage_pools[nid] || ''}
                              error={hasError('data_disk_storage_pools')}
                              options={[{ label: '-', value: '' }, ...pools.map((p) => ({ label: `${p.name} (${p.driver})`, value: p.name }))]}
                              onChange={(v) => { update('data_disk_storage_pools', { ...form.data_disk_storage_pools, [nid]: v }); clearError('data_disk_storage_pools') }}
                            />
                          </div>
                        )
                      })}
                    </div>
                  </div>
                )}
              </div>
            </>
          )}

          {/* 步骤5: 网络配置 (网桥 + 带宽 + 流量 + IPv4 + IPv6) */}
          {step === 4 && (
            <>
              {/* 网桥选择（按宿主机分别选择） */}
              <div className="slide-step-section">
                <div className="slide-step-section__title">{t('product.bridgePerNode')}</div>
                <div className="space-y-2">
                  {(form.node_ids || []).map((nid) => {
                    const node = nodes.find((n) => n.id === nid)
                    const bridges = nodeBridges[nid] || []
                    return (
                      <div key={nid} className="slide-storage-pool-row">
                        <span className="slide-storage-pool-row__name">{node?.name || nid}</span>
                        <Select
                          value={form.node_bridges[nid] || ''}
                          error={hasError('node_bridges')}
                          options={[{ label: '-', value: '' }, ...bridges.map((b) => ({ label: `${b.name} (${b.bridge_name})`, value: b.id }))]}
                          onChange={(v) => { update('node_bridges', { ...form.node_bridges, [nid]: v }); clearError('node_bridges') }}
                        />
                      </div>
                    )
                  })}
                </div>
              </div>

              {/* 带宽配置 */}
              <div className="slide-step-section">
                <div className="slide-step-section__title">{t('product.bandwidth')} (Mbps)</div>
                {/* 下行带宽 */}
                <div className="slide-form-grid slide-form-grid--4">
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.minDownMbps')}</span>
                    <input type="number" className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('network_down_range') ? 'field-error' : ''}`} value={form.network_down_min_mbps} onChange={(e) => { update('network_down_min_mbps', parseInt(e.target.value) || 0); clearError('network_down_range') }} />
                  </div>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.maxDownMbps')}</span>
                    <input type="number" className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('network_down_range') ? 'field-error' : ''}`} value={form.network_down_max_mbps} onChange={(e) => { update('network_down_max_mbps', parseInt(e.target.value) || 0); clearError('network_down_range') }} />
                  </div>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.customMode')}</span>
                    <Select
                      value={form.network_down_custom_mode}
                      options={[
                        { label: t('product.customUnlimited'), value: 'unlimited' },
                        { label: t('product.customTiers'), value: 'tiers' },
                      ]}
                      onChange={(v) => update('network_down_custom_mode', v)}
                    />
                  </div>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.downUnitPrice')}({t('common.yuan')})</span>
                    <input type="number" step="0.01" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={form.network_down_unit_price_cents / 100} onChange={(e) => update('network_down_unit_price_cents', Math.round(parseFloat(e.target.value) * 100))} />
                  </div>
                </div>
                {form.network_down_custom_mode === 'tiers' && (
                  <div className="slide-form-grid slide-form-grid--1">
                    <div className="slide-form-item">
                      <span className="slide-form-item__label">{t('product.tiers')} (Mbps)</span>
                      <input
                        className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm"
                        placeholder={t('product.tiersHint')}
                        value={tiersText['network_down_tiers'] ?? ''}
                        onChange={(e) => updateTiersText('network_down_tiers', e.target.value)}
                        onBlur={() => commitTiers('network_down_tiers')}
                      />
                    </div>
                  </div>
                )}
                {/* 上行带宽 */}
                <div className="slide-form-grid slide-form-grid--4" style={{ marginTop: 12 }}>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.minUpMbps')}</span>
                    <input type="number" className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('network_up_range') ? 'field-error' : ''}`} value={form.network_up_min_mbps} onChange={(e) => { update('network_up_min_mbps', parseInt(e.target.value) || 0); clearError('network_up_range') }} />
                  </div>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.maxUpMbps')}</span>
                    <input type="number" className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('network_up_range') ? 'field-error' : ''}`} value={form.network_up_max_mbps} onChange={(e) => { update('network_up_max_mbps', parseInt(e.target.value) || 0); clearError('network_up_range') }} />
                  </div>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.customMode')}</span>
                    <Select
                      value={form.network_up_custom_mode}
                      options={[
                        { label: t('product.customUnlimited'), value: 'unlimited' },
                        { label: t('product.customTiers'), value: 'tiers' },
                      ]}
                      onChange={(v) => update('network_up_custom_mode', v)}
                    />
                  </div>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.upUnitPrice')}({t('common.yuan')})</span>
                    <input type="number" step="0.01" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={form.network_up_unit_price_cents / 100} onChange={(e) => update('network_up_unit_price_cents', Math.round(parseFloat(e.target.value) * 100))} />
                  </div>
                </div>
                {form.network_up_custom_mode === 'tiers' && (
                  <div className="slide-form-grid slide-form-grid--1">
                    <div className="slide-form-item">
                      <span className="slide-form-item__label">{t('product.tiers')} (Mbps)</span>
                      <input
                        className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm"
                        placeholder={t('product.tiersHint')}
                        value={tiersText['network_up_tiers'] ?? ''}
                        onChange={(e) => updateTiersText('network_up_tiers', e.target.value)}
                        onBlur={() => commitTiers('network_up_tiers')}
                      />
                    </div>
                  </div>
                )}
              </div>

              {/* 流量配置 */}
              <div className="slide-step-section">
                <div className="slide-step-section__title">{t('product.traffic')}</div>
                <div className="slide-form-grid slide-form-grid--4">
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.minTraffic')}(GB)</span>
                    <input type="number" className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('traffic_range') ? 'field-error' : ''}`} value={form.traffic_min_gb} onChange={(e) => { update('traffic_min_gb', parseInt(e.target.value) || 0); clearError('traffic_range') }} />
                  </div>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.maxTraffic')}(GB)</span>
                    <input type="number" className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('traffic_range') ? 'field-error' : ''}`} value={form.traffic_max_gb} onChange={(e) => { update('traffic_max_gb', parseInt(e.target.value) || 0); clearError('traffic_range') }} />
                  </div>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.customMode')}</span>
                    <Select
                      value={form.traffic_custom_mode}
                      options={[
                        { label: t('product.customUnlimited'), value: 'unlimited' },
                        { label: t('product.customTiers'), value: 'tiers' },
                      ]}
                      onChange={(v) => update('traffic_custom_mode', v)}
                    />
                  </div>
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.trafficUnitPrice')}({t('common.yuan')}/GB)</span>
                    <input type="number" step="0.01" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={form.traffic_unit_price_cents / 100} onChange={(e) => update('traffic_unit_price_cents', Math.round(parseFloat(e.target.value) * 100))} />
                  </div>
                </div>
                <div className="slide-form-grid slide-form-grid--2">
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.calcMode')}</span>
                    <Select
                      value={form.traffic_calc_mode}
                      options={[
                        { label: t('product.trafficBoth'), value: 'both' },
                        { label: t('product.trafficInbound'), value: 'inbound' },
                        { label: t('product.trafficOutbound'), value: 'outbound' },
                        { label: t('product.trafficMax'), value: 'max' },
                      ]}
                      onChange={(v) => update('traffic_calc_mode', v)}
                    />
                  </div>
                  {form.traffic_custom_mode === 'tiers' && (
                    <div className="slide-form-item">
                      <span className="slide-form-item__label">{t('product.tiers')} (GB)</span>
                      <input
                        className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm"
                        placeholder={t('product.tiersHint')}
                        value={tiersText['traffic_tiers'] ?? ''}
                        onChange={(e) => updateTiersText('traffic_tiers', e.target.value)}
                        onBlur={() => commitTiers('traffic_tiers')}
                      />
                    </div>
                  )}
                </div>
              </div>

              {/* IPv4 配置 */}
              <div className="slide-step-section">
                <div className="slide-step-section__title">IPv4</div>
                <div className="slide-form-grid slide-form-grid--3">
                  <div className="slide-form-item">
                    <span className="slide-form-item__label">{t('product.ipv4Mode')}</span>
                    <Select
                      value={form.ipv4_mode}
                      options={[
                        { label: 'NAT', value: 'nat' },
                        { label: t('product.eipMode'), value: 'eip' },
                      ]}
                      onChange={(v) => update('ipv4_mode', v)}
                    />
                  </div>
                  {form.ipv4_mode === 'eip' && (
                    <>
                      <div className="slide-form-item">
                        <span className="slide-form-item__label">{t('product.allowChangeIP')}</span>
                        <Select
                          value={form.ipv4_eip_allow_change ? 'yes' : 'no'}
                          options={[{ label: t('common.yes'), value: 'yes' }, { label: t('common.no'), value: 'no' }]}
                          onChange={(v) => update('ipv4_eip_allow_change', v === 'yes')}
                        />
                      </div>
                    </>
                  )}
                </div>
                {form.ipv4_mode === 'eip' && (
                  <>
                    <div className="slide-form-grid slide-form-grid--4" style={{ marginTop: 12 }}>
                      <div className="slide-form-item">
                        <span className="slide-form-item__label">{t('product.minIPCount')}</span>
                        <input type="number" min="0" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={form.ipv4_eip_min} onChange={(e) => update('ipv4_eip_min', parseInt(e.target.value) || 0)} />
                      </div>
                      <div className="slide-form-item">
                        <span className="slide-form-item__label">{t('product.maxIPCount')}</span>
                        <input type="number" min="0" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={form.ipv4_eip_max} onChange={(e) => update('ipv4_eip_max', parseInt(e.target.value) || 0)} />
                      </div>
                      <div className="slide-form-item">
                        <span className="slide-form-item__label">{t('product.customMode')}</span>
                        <Select
                          value={form.ipv4_eip_custom_mode}
                          options={[
                            { label: t('product.customUnlimited'), value: 'unlimited' },
                            { label: t('product.customTiers'), value: 'tiers' },
                          ]}
                          onChange={(v) => update('ipv4_eip_custom_mode', v)}
                        />
                      </div>
                      <div className="slide-form-item">
                        <span className="slide-form-item__label">{t('product.eipUnitPrice')}({t('common.yuan')})</span>
                        <input type="number" step="0.01" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={form.ipv4_eip_unit_price_cents / 100} onChange={(e) => update('ipv4_eip_unit_price_cents', Math.round(parseFloat(e.target.value) * 100))} />
                      </div>
                    </div>
                    {form.ipv4_eip_custom_mode === 'tiers' && (
                      <div className="slide-form-grid slide-form-grid--1">
                        <div className="slide-form-item">
                          <span className="slide-form-item__label">{t('product.tiers')}</span>
                          <input
                            className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm"
                            placeholder={t('product.tiersHint')}
                            value={tiersText['ipv4_eip_tiers'] ?? ''}
                            onChange={(e) => updateTiersText('ipv4_eip_tiers', e.target.value)}
                            onBlur={() => commitTiers('ipv4_eip_tiers')}
                          />
                        </div>
                      </div>
                    )}
                    {form.ipv4_eip_allow_change && (
                      <div className="slide-form-grid slide-form-grid--3" style={{ marginTop: 12 }}>
                        <div className="slide-form-item">
                          <span className="slide-form-item__label">{t('product.changeIPPrice')}({t('common.yuan')})</span>
                          <input type="number" step="0.01" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={form.ipv4_eip_change_price_cents / 100} onChange={(e) => update('ipv4_eip_change_price_cents', Math.round(parseFloat(e.target.value) * 100))} />
                        </div>
                      </div>
                    )}
                  </>
                )}
                {form.ipv4_mode === 'nat' && (
                  <>
                    <div className="slide-form-grid slide-form-grid--4" style={{ marginTop: 12 }}>
                      <div className="slide-form-item">
                        <span className="slide-form-item__label">{t('product.natPortMin')}</span>
                        <input type="number" min="0" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={form.nat_port_min} onChange={(e) => update('nat_port_min', parseInt(e.target.value) || 0)} />
                      </div>
                      <div className="slide-form-item">
                        <span className="slide-form-item__label">{t('product.natPortMax')}</span>
                        <input type="number" min="0" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={form.nat_port_max} onChange={(e) => update('nat_port_max', parseInt(e.target.value) || 0)} />
                      </div>
                      <div className="slide-form-item">
                        <span className="slide-form-item__label">{t('product.customMode')}</span>
                        <Select
                          value={form.nat_port_custom_mode}
                          options={[
                            { label: t('product.customUnlimited'), value: 'unlimited' },
                            { label: t('product.customTiers'), value: 'tiers' },
                          ]}
                          onChange={(v) => update('nat_port_custom_mode', v)}
                        />
                      </div>
                      <div className="slide-form-item">
                        <span className="slide-form-item__label">{t('product.natPortUnitPrice')}({t('common.yuan')})</span>
                        <input type="number" step="0.01" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={form.nat_port_unit_price_cents / 100} onChange={(e) => update('nat_port_unit_price_cents', Math.round(parseFloat(e.target.value) * 100))} />
                      </div>
                    </div>
                    {form.nat_port_custom_mode === 'tiers' && (
                      <div className="slide-form-grid slide-form-grid--1">
                        <div className="slide-form-item">
                          <span className="slide-form-item__label">{t('product.tiers')}</span>
                          <input
                            className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm"
                            placeholder={t('product.tiersHint')}
                            value={tiersText['nat_port_tiers'] ?? ''}
                            onChange={(e) => updateTiersText('nat_port_tiers', e.target.value)}
                            onBlur={() => commitTiers('nat_port_tiers')}
                          />
                        </div>
                      </div>
                    )}
                  </>
                )}
              </div>

              {/* IPv6 配置 */}
              <div className="slide-step-section">
                <div className="slide-step-section__title">IPv6</div>
                <div className="slide-form-grid slide-form-grid--3">
                  <div className="slide-checkbox-row">
                    <label>
                      <input type="checkbox" checked={form.ipv6_enabled} onChange={(e) => update('ipv6_enabled', e.target.checked)} />
                      {t('product.enableIPv6')}
                    </label>
                  </div>
                </div>
                {form.ipv6_enabled && (
                  <div style={{ marginTop: 12 }}>
                    {form.ipv6_configs.map((cfg, idx) => (
                      <div key={idx} className="slide-form-grid slide-form-grid--5" style={{ marginBottom: 8 }}>
                        <div className="slide-form-item">
                          <span className="slide-form-item__label">{t('product.prefixLen')}</span>
                          <input type="number" min="1" max="128" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={cfg.prefix_len} onChange={(e) => {
                            const configs = [...form.ipv6_configs]
                            configs[idx] = { ...cfg, prefix_len: parseInt(e.target.value) || 128 }
                            update('ipv6_configs', configs)
                          }} />
                        </div>
                        <div className="slide-form-item">
                          <span className="slide-form-item__label">{t('product.minCount')}</span>
                          <input type="number" min="0" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={cfg.min} onChange={(e) => {
                            const configs = [...form.ipv6_configs]
                            configs[idx] = { ...cfg, min: parseInt(e.target.value) || 0 }
                            update('ipv6_configs', configs)
                          }} />
                        </div>
                        <div className="slide-form-item">
                          <span className="slide-form-item__label">{t('product.maxCount')}</span>
                          <input type="number" min="0" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={cfg.max} onChange={(e) => {
                            const configs = [...form.ipv6_configs]
                            configs[idx] = { ...cfg, max: parseInt(e.target.value) || 0 }
                            update('ipv6_configs', configs)
                          }} />
                        </div>
                        <div className="slide-form-item">
                          <span className="slide-form-item__label">{t('product.customMode')}</span>
                          <Select
                            value={cfg.custom_mode}
                            options={[
                              { label: t('product.customUnlimited'), value: 'unlimited' },
                              { label: t('product.customTiers'), value: 'tiers' },
                            ]}
                            onChange={(v) => {
                              const configs = [...form.ipv6_configs]
                              configs[idx] = { ...cfg, custom_mode: String(v) }
                              update('ipv6_configs', configs)
                            }}
                          />
                        </div>
                        <div className="slide-form-item">
                          <span className="slide-form-item__label">{t('product.unitPrice')}({t('common.yuan')})</span>
                          <input type="number" step="0.01" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={cfg.unit_price_cents / 100} onChange={(e) => {
                            const configs = [...form.ipv6_configs]
                            configs[idx] = { ...cfg, unit_price_cents: Math.round(parseFloat(e.target.value) * 100) }
                            update('ipv6_configs', configs)
                          }} />
                        </div>
                        {cfg.custom_mode === 'tiers' && (
                          <div className="slide-form-grid slide-form-grid--1" style={{ gridColumn: '1 / -1' }}>
                            <div className="slide-form-item">
                              <span className="slide-form-item__label">{t('product.tiers')}</span>
                              <input
                                className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm"
                                placeholder={t('product.tiersHint')}
                                value={tiersText[`ipv6_configs.${idx}.tiers`] ?? ''}
                                onChange={(e) => updateTiersText(`ipv6_configs.${idx}.tiers`, e.target.value)}
                                onBlur={() => {
                                  const text = tiersText[`ipv6_configs.${idx}.tiers`] || ''
                                  const parsed = text.split(',').map(s => parseInt(s.trim())).filter(n => !isNaN(n))
                                  const configs = [...form.ipv6_configs]
                                  configs[idx] = { ...cfg, tiers: parsed }
                                  update('ipv6_configs', configs)
                                }}
                              />
                            </div>
                          </div>
                        )}
                        <div style={{ gridColumn: '1 / -1' }}>
                          <button
                            className="px-3 py-1 text-sm text-red-500 border border-red-300 rounded-lg hover:bg-red-50"
                            onClick={() => {
                              const configs = form.ipv6_configs.filter((_, i) => i !== idx)
                              update('ipv6_configs', configs)
                            }}
                          >
                            {t('common.delete')}
                          </button>
                        </div>
                      </div>
                    ))}
                    <button
                      className="px-3 py-1.5 text-sm border border-surface-strong rounded-lg hover:bg-surface-hover"
                      onClick={() => {
                        const configs = [...form.ipv6_configs, { prefix_len: 128, min: 1, max: 10, custom_mode: 'unlimited', tiers: [], unit_price_cents: 0 }]
                        update('ipv6_configs', configs)
                      }}
                    >
                      + {t('common.add')}
                    </button>
                  </div>
                )}
              </div>
            </>
          )}

          {/* 步骤6: 价格配置 */}
          {step === 5 && (
            <div className="slide-step-section">
              <div className="slide-step-section__title">{t('product.pricing')}</div>
              <div className="slide-form-grid slide-form-grid--3">
                <div className="slide-form-item">
                  <span className="slide-form-item__label">{t('product.basePrice')}({t('common.yuan')}/mo)</span>
                  <input type="number" step="0.01" className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('base_price') ? 'field-error' : ''}`} value={form.base_price_cents / 100} onChange={(e) => { update('base_price_cents', Math.round(parseFloat(e.target.value) * 100)); clearError('base_price') }} />
                </div>
                <div className="slide-form-item">
                  <span className="slide-form-item__label">{t('product.minPaymentPeriod')}</span>
                  <Select
                    value={form.min_payment_period}
                    options={[
                      { label: t('product.monthly'), value: 'monthly' },
                      { label: t('product.quarterly'), value: 'quarterly' },
                      { label: t('product.halfYearly'), value: 'half_yearly' },
                      { label: t('product.yearly'), value: 'yearly' },
                    ]}
                    onChange={(v) => update('min_payment_period', v)}
                  />
                </div>
              </div>
              <div className="slide-form-grid slide-form-grid--3">
                <div className="slide-form-item">
                  <span className="slide-form-item__label">{t('product.quarterlyDiscount')}</span>
                  <input type="number" step="0.01" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={form.quarterly_discount} onChange={(e) => update('quarterly_discount', parseFloat(e.target.value) || 1)} />
                </div>
                <div className="slide-form-item">
                  <span className="slide-form-item__label">{t('product.halfYearlyDiscount')}</span>
                  <input type="number" step="0.01" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={form.half_yearly_discount} onChange={(e) => update('half_yearly_discount', parseFloat(e.target.value) || 1)} />
                </div>
                <div className="slide-form-item">
                  <span className="slide-form-item__label">{t('product.yearlyDiscount')}</span>
                  <input type="number" step="0.01" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={form.yearly_discount} onChange={(e) => update('yearly_discount', parseFloat(e.target.value) || 1)} />
                </div>
              </div>
              <div className="slide-form-grid slide-form-grid--3">
                <div className="slide-checkbox-row">
                  <label>
                    <input type="checkbox" checked={form.trial_enabled} onChange={(e) => update('trial_enabled', e.target.checked)} />
                    {t('product.enableTrial')}
                  </label>
                </div>
                {form.trial_enabled && (
                  <>
                    <div className="slide-form-item">
                      <span className="slide-form-item__label">{t('product.trialHours')}</span>
                      <input type="number" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={form.trial_hours} onChange={(e) => update('trial_hours', parseInt(e.target.value) || 0)} />
                    </div>
                    <div className="slide-form-item">
                      <span className="slide-form-item__label">{t('product.trialPrice')}({t('common.yuan')})</span>
                      <input type="number" step="0.01" className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" value={form.trial_price_cents / 100} onChange={(e) => update('trial_price_cents', Math.round(parseFloat(e.target.value) * 100))} />
                    </div>
                  </>
                )}
              </div>
            </div>
          )}
        </div>
      </SlidePanel>
    </PageLayout>
  )
}
