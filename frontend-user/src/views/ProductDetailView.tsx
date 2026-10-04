import { useQuery } from '@tanstack/react-query'
import { useState, useMemo, useEffect } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { productsApi, type Product, type NodeImage } from '@/api/products'
import { useAuthStore } from '@/stores/auth'
import { useCartStore } from '@/stores/cart'
import { useToastStore } from '@/stores/toast'
import { Slider } from '@/components/Slider/Slider'
import {
  ArrowLeft, Cpu, MemoryStick, HardDrive, Network, Globe,
  Activity, Loader2, Package, Zap, TrafficCone,
  Monitor, Check, ChevronDown, ChevronUp, AlertCircle,
  ShoppingCart,
} from 'lucide-react'
import '@/views/ProductsView.css'
import { Modal } from '@/components/Modal/Modal'

function formatPrice(cents: number): string {
  return (cents / 100).toFixed(2)
}

function formatMB(mb: number): string {
  if (mb >= 1024) return `${(mb / 1024).toFixed(0)} GB`
  return `${mb} MB`
}

function formatMbps(mbps: number): string {
  if (mbps <= 0) return '不限'
  return `${mbps} Mbps`
}

function formatTraffic(gb: number): string {
  if (gb <= 0) return '不限'
  return `${gb} GB`
}

function SpecRow({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <div className="product-detail-spec-row">
      <div className="product-detail-spec-icon">{icon}</div>
      <span className="product-detail-spec-label">{label}</span>
      <span className="product-detail-spec-value font-number">{value}</span>
    </div>
  )
}

function PriceCard({ product }: { product: Product }) {
  const { t } = useTranslation()
  const monthly = product.base_price_cents
  const quarterly = Math.round(monthly * 3 * product.quarterly_discount)
  const halfYearly = Math.round(monthly * 6 * product.half_yearly_discount)
  const yearly = Math.round(monthly * 12 * product.yearly_discount)

  const periods = [
    { key: 'monthly', label: t('product.monthly'), months: 1, price: monthly },
    { key: 'quarterly', label: t('product.quarterly'), months: 3, price: quarterly, discount: product.quarterly_discount },
    { key: 'half_yearly', label: t('product.halfYearly'), months: 6, price: halfYearly, discount: product.half_yearly_discount },
    { key: 'yearly', label: t('product.yearly'), months: 12, price: yearly, discount: product.yearly_discount },
  ]

  return (
    <div className="product-detail-price-card">
      <h3 className="product-detail-price-title">{t('product.pricing')}</h3>
      <div className="product-detail-price-periods">
        {periods.map((p) => (
          <div key={p.key} className="product-detail-price-period">
            <div className="product-detail-price-period-label">
              <span>{p.label}</span>
              {p.discount && p.discount < 1 && (
                <span className="product-detail-price-discount">
                  {(p.discount * 10).toFixed(1)}折
                </span>
              )}
            </div>
            <div className="product-detail-price-period-value">
              <span className="font-number">¥{formatPrice(p.price)}</span>
            </div>
          </div>
        ))}
      </div>
      {product.trial_enabled && (
        <div className="product-detail-price-trial">
          <Zap size={14} />
          <span>{t('product.trialAvailable')}: {product.trial_hours}h / ¥{formatPrice(product.trial_price_cents)}</span>
        </div>
      )}
    </div>
  )
}

export function ProductDetailView() {
  const { t } = useTranslation()
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { user } = useAuthStore()
  const { addToCart } = useCartStore()
  const toast = useToastStore()
  const [submitting, setSubmitting] = useState(false)
  const [addingCart, setAddingCart] = useState(false)
  const [error, setError] = useState('')
  const [showLoginModal, setShowLoginModal] = useState(false)

  // 配置状态
  const [vcpu, setVcpu] = useState(1)
  const [memoryMB, setMemoryMB] = useState(512)
  const [diskGB, setDiskGB] = useState(10)
  const [dataDiskGB, setDataDiskGB] = useState(0)
  const [paymentPeriod, setPaymentPeriod] = useState<string>('monthly')
  const [trial, setTrial] = useState(false)
  const [selectedImage, setSelectedImage] = useState('')
  const [loginMethod, setLoginMethod] = useState<'auto' | 'password' | 'sshkey'>('auto')
  const [sshPassword, setSshPassword] = useState('')
  const [sshPublicKey, setSshPublicKey] = useState('')
  const [advancedOpen, setAdvancedOpen] = useState(false)

  const { data: productData, isLoading } = useQuery({
    queryKey: ['product', id],
    queryFn: () => productsApi.get(id!),
    enabled: !!id,
  })

  const product = productData?.data

  // 商品加载后初始化默认值
  useEffect(() => {
    if (product) {
      setVcpu(product.vcpu_min)
      setMemoryMB(product.memory_min_mb)
      setDiskGB(Math.round(product.disk_min_mb / 1024))
      setPaymentPeriod(product.min_payment_period || 'monthly')
    }
  }, [product])

  // 获取镜像
  const { data: imagesData, isLoading: imagesLoading } = useQuery({
    queryKey: ['product-images', id],
    queryFn: () => productsApi.images(id!),
    enabled: !!id,
  })

  const images = imagesData?.data?.data ?? []
  const nodeId = imagesData?.data?.node_id ?? ''

  // 镜像分类
  const imageCategories = useMemo(() => {
    const cats = new Map<string, NodeImage[]>()
    for (const img of images) {
      const catName = img.category_name || t('product.uncategorized')
      if (!cats.has(catName)) cats.set(catName, [])
      cats.get(catName)!.push(img)
    }
    return Array.from(cats.entries())
  }, [images, t])

  // 生成刻度（最多3个，不含首尾）
  const generateMarks = (min: number, max: number): number[] => {
    const range = max - min
    if (range <= 1) return []
    const marks: number[] = []
    const count = Math.min(3, Math.floor(range))
    for (let i = 1; i <= count; i++) {
      marks.push(Math.round(min + (range * i) / (count + 1)))
    }
    return marks
  }

  // vcpu 配置
  const vcpuTiers: number[] = useMemo(() => {
    if (!product) return []
    if (product.vcpu_custom_mode === 'tiers') {
      return Array.isArray(product.vcpu_tiers) ? product.vcpu_tiers : []
    }
    return []
  }, [product])

  const vcpuMarks = useMemo(() => {
    if (!product) return []
    if (vcpuTiers.length > 0) {
      return generateMarks(0, vcpuTiers.length - 1).map(i => vcpuTiers[i])
    }
    return generateMarks(product.vcpu_min, product.vcpu_max)
  }, [product, vcpuTiers])

  const memoryTiers: number[] = useMemo(() => {
    if (!product) return []
    if (product.memory_custom_mode === 'tiers') {
      return Array.isArray(product.memory_tiers) ? product.memory_tiers : []
    }
    return []
  }, [product])

  // 内存单位: max >= 4096MB 时用 GB, 否则用 MB
  const memUseGB = product ? product.memory_max_mb >= 4096 : false
  const memStep = memUseGB ? 1024 : 32
  const memUnit = memUseGB ? 'GB' : 'MB'
  const memDisplayMin = product ? (memUseGB ? Math.round(product.memory_min_mb / 1024) : product.memory_min_mb) : 0
  const memDisplayMax = product ? (memUseGB ? Math.round(product.memory_max_mb / 1024) : product.memory_max_mb) : 0
  const memDisplayValue = memUseGB ? Math.round(memoryMB / 1024) : memoryMB

  const memDisplayTiers = useMemo(() => {
    if (memoryTiers.length === 0) return undefined
    if (memUseGB) return memoryTiers.map(mb => Math.round(mb / 1024))
    return memoryTiers
  }, [memoryTiers, memUseGB])

  const memoryMarks = useMemo(() => {
    if (!product) return []
    if (memDisplayTiers && memDisplayTiers.length > 0) {
      return generateMarks(0, memDisplayTiers.length - 1).map(i => memDisplayTiers[i])
    }
    return generateMarks(memDisplayMin, memDisplayMax)
  }, [product, memDisplayTiers, memDisplayMin, memDisplayMax])

  const diskTiers: number[] = useMemo(() => {
    if (!product) return []
    if (product.disk_custom_mode === 'tiers') {
      const raw = Array.isArray(product.disk_tiers) ? product.disk_tiers : []
      return raw.map((mb) => Math.round(mb / 1024))
    }
    return []
  }, [product])

  const diskMarks = useMemo(() => {
    if (!product) return []
    const minGB = Math.max(1, Math.round(product.disk_min_mb / 1024))
    const maxGB = Math.round(product.disk_max_mb / 1024)
    if (diskTiers.length > 0) {
      return generateMarks(0, diskTiers.length - 1).map(i => diskTiers[i])
    }
    return generateMarks(minGB, maxGB)
  }, [product, diskTiers])

  // 价格计算
  const totalPrice = useMemo(() => {
    if (!product) return 0
    const monthsMap: Record<string, number> = { monthly: 1, quarterly: 3, half_yearly: 6, yearly: 12 }
    const months = monthsMap[paymentPeriod] || 1

    const base = product.base_price_cents
    const vcpuExtra = Math.max(0, vcpu - product.vcpu_min)
    const cpuExtraCents = vcpuExtra * product.vcpu_unit_price_cents

    const memoryExtra = Math.max(0, memoryMB - product.memory_min_mb)
    let memoryExtraCents = 0
    if (product.memory_unit === 'gb') {
      memoryExtraCents = Math.floor(memoryExtra / 1024) * product.memory_unit_price_cents
    } else {
      memoryExtraCents = memoryExtra * product.memory_unit_price_cents
    }

    const diskMB = diskGB * 1024
    const diskExtra = Math.max(0, diskMB - product.disk_min_mb)
    const diskExtraCents = Math.floor(diskExtra / 1024) * product.disk_unit_price_cents

    const dataDiskMB = dataDiskGB * 1024
    const dataDiskExtraCents = product.data_disk_allow ? Math.floor(dataDiskMB / 1024) * product.data_disk_unit_price_cents : 0

    const monthlyCents = base + cpuExtraCents + memoryExtraCents + diskExtraCents + dataDiskExtraCents

    let discount = 1.0
    if (paymentPeriod === 'quarterly') discount = product.quarterly_discount
    else if (paymentPeriod === 'half_yearly') discount = product.half_yearly_discount
    else if (paymentPeriod === 'yearly') discount = product.yearly_discount

    let total = Math.round(monthlyCents * months * discount)

    if (trial && product.trial_enabled) {
      total = product.trial_price_cents
    }

    return total
  }, [product, vcpu, memoryMB, diskGB, dataDiskGB, paymentPeriod, trial])

  const balanceCents = user?.balance_cents ?? 0
  const canAfford = balanceCents >= totalPrice

  const periods = [
    { key: 'monthly', label: t('product.monthly'), months: 1, discount: 1.0 },
    { key: 'quarterly', label: t('product.quarterly'), months: 3, discount: product?.quarterly_discount ?? 1 },
    { key: 'half_yearly', label: t('product.halfYearly'), months: 6, discount: product?.half_yearly_discount ?? 1 },
    { key: 'yearly', label: t('product.yearly'), months: 12, discount: product?.yearly_discount ?? 1 },
  ]

  const minPeriodIndex = product ? periods.findIndex(p => p.key === product.min_payment_period) : 0
  const allowedPeriods = minPeriodIndex >= 0 ? periods.slice(minPeriodIndex) : periods

  const buildConfig = () => {
    if (!product) return null
    const selectedImg = images.find(img => img.id === selectedImage)
    return {
      vcpu,
      memory_mb: memoryMB,
      disk_mb: diskGB * 1024,
      payment_period: trial ? 'monthly' : paymentPeriod,
      template_id: selectedImg?.alias || selectedImage,
      image_key: selectedImage,
      node_id: nodeId || undefined,
      login_method: loginMethod,
      ssh_password: loginMethod === 'password' ? sshPassword : undefined,
      ssh_public_key: loginMethod === 'sshkey' ? sshPublicKey : undefined,
      data_disk_mb: dataDiskGB > 0 && product.data_disk_allow ? dataDiskGB * 1024 : undefined,
      trial: trial && product.trial_enabled,
    }
  }

  const validateConfig = (): boolean => {
    if (!product) return false
    if (!selectedImage) {
      setError(t('product.errorImageRequired'))
      return false
    }
    return true
  }

  const handleSubmit = async () => {
    if (!product) return
    setError('')
    if (!validateConfig()) return

    setSubmitting(true)
    try {
      const config = buildConfig()!
      navigate('/checkout', {
        state: {
          items: [{
            product_id: product.id,
            config,
            quantity: 1,
          }],
        },
      })
    } finally {
      setSubmitting(false)
    }
  }

  const handleAddToCart = async () => {
    if (!product) return
    setError('')
    if (!validateConfig()) return

    const token = useAuthStore.getState().token
    if (!token) {
      setShowLoginModal(true)
      return
    }

    setAddingCart(true)
    try {
      const config = buildConfig()!
      await addToCart(product.id, config, 1)
      toast.success(t('cart.addedToCart'))
    } catch (err: any) {
      setError(err.response?.data?.error || err.message || t('cart.addToCartFailed'))
    } finally {
      setAddingCart(false)
    }
  }

  if (isLoading) {
    return (
      <main className="flex-1 overflow-y-auto overflow-x-hidden p-4 sm:p-6 md:p-8">
        <div className="flex justify-center py-20">
          <Loader2 className="animate-spin text-muted-foreground" size={28} />
        </div>
      </main>
    )
  }

  if (!product) {
    return (
      <main className="flex-1 overflow-y-auto overflow-x-hidden p-4 sm:p-6 md:p-8">
        <div className="product-empty">
          <Package size={48} className="text-muted-foreground opacity-50" />
          <p className="text-muted-foreground mt-4">{t('product.notFound')}</p>
          <Link to="/products" className="product-detail-back-btn mt-4">{t('common.back')}</Link>
        </div>
      </main>
    )
  }

  const typeLabel = product.type === 'vm' ? t('product.typeVM') : t('product.typeContainer')

  return (
    <main className="flex-1 overflow-y-auto overflow-x-hidden p-4 sm:p-6 md:p-8">
      <div className="mx-auto max-w-7xl w-full">
        <div className="space-y-6 animate-in fade-in slide-in-from-bottom-4 duration-700">

          <button
            onClick={() => navigate('/products')}
            className="product-detail-back-btn"
          >
            <ArrowLeft size={16} />
            {t('common.back')}
          </button>

          <div className="product-detail-header">
            <div className="product-detail-header-info">
              <h2 className="text-2xl font-medium tracking-tight">{product.name}</h2>
              <div className="product-detail-header-tags">
                <span className="product-detail-tag">{typeLabel}</span>
                {product.ipv6_enabled && <span className="product-detail-tag">IPv6</span>}
                {product.ipv4_mode === 'eip' && <span className="product-detail-tag">独立IP</span>}
                {product.ipv4_mode === 'nat' && <span className="product-detail-tag">NAT</span>}
              </div>
            </div>
            <div className="product-detail-header-price">
              <span className="text-3xl font-semibold font-number">¥{formatPrice(product.base_price_cents)}</span>
              <span className="text-sm text-muted-foreground">/{t('product.month')}</span>
            </div>
          </div>

          <div className="product-detail-grid">
            {/* 左侧：配置选择 */}
            <div className="product-detail-main">
              {/* CPU 配置 */}
              <div className="product-detail-section">
                <h3 className="product-detail-section-title">
                  <Cpu size={16} />
                  {t('product.cores')}
                </h3>
                <Slider
                  min={product.vcpu_min}
                  max={product.vcpu_max}
                  step={1}
                  value={vcpu}
                  onChange={setVcpu}
                  unit={t('product.coresUnit')}
                  tiers={vcpuTiers.length > 0 ? vcpuTiers : undefined}
                  marks={vcpuMarks}
                />
              </div>

              {/* 内存配置 */}
              <div className="product-detail-section">
                <h3 className="product-detail-section-title">
                  <MemoryStick size={16} />
                  {t('product.memory')}
                </h3>
                <Slider
                  min={memDisplayMin}
                  max={memDisplayMax}
                  step={memUseGB ? 1 : 32}
                  value={memDisplayValue}
                  onChange={(v) => setMemoryMB(memUseGB ? v * 1024 : v)}
                  unit={memUnit}
                  tiers={memDisplayTiers}
                  marks={memoryMarks}
                />
              </div>

              {/* 系统盘配置 */}
              <div className="product-detail-section">
                <h3 className="product-detail-section-title">
                  <HardDrive size={16} />
                  {t('product.systemDisk')}
                </h3>
                <Slider
                  min={Math.max(1, Math.round(product.disk_min_mb / 1024))}
                  max={Math.round(product.disk_max_mb / 1024)}
                  step={1}
                  value={diskGB}
                  onChange={setDiskGB}
                  unit="GB"
                  tiers={diskTiers.length > 0 ? diskTiers : undefined}
                  marks={diskMarks}
                />
              </div>

              {/* 数据盘 - 不支持则不显示 */}
              {product.data_disk_allow && product.data_disk_max_mb > 0 && (
                <div className="product-detail-section">
                  <h3 className="product-detail-section-title">
                    <HardDrive size={16} />
                    {t('product.dataDisk')} ({t('product.optional')})
                  </h3>
                  <Slider
                    min={0}
                    max={Math.round(product.data_disk_max_mb / 1024)}
                    step={1}
                    value={dataDiskGB}
                    onChange={setDataDiskGB}
                    unit="GB"
                    marks={generateMarks(0, Math.round(product.data_disk_max_mb / 1024))}
                  />
                </div>
              )}

              {/* 系统镜像选择 */}
              <div className="product-detail-section">
                <h3 className="product-detail-section-title">
                  <Monitor size={16} />
                  {t('product.systemImage')} <span className="purchase-required">*</span>
                </h3>
                {imagesLoading ? (
                  <div className="purchase-loading">
                    <Loader2 className="animate-spin" size={16} />
                    <span>{t('common.loading')}</span>
                  </div>
                ) : images.length === 0 ? (
                  <div className="purchase-empty-hint">{t('product.noImages')}</div>
                ) : (
                  <div className="purchase-image-list">
                    {imageCategories.map(([catName, imgs]) => (
                      <div key={catName} className="purchase-image-group">
                        <div className="purchase-image-group-title">{catName}</div>
                        <div className="purchase-image-grid">
                          {imgs.map(img => (
                            <button
                              key={img.id}
                              className={`purchase-image-card ${selectedImage === img.id ? 'selected' : ''}`}
                              onClick={() => setSelectedImage(img.id)}
                            >
                              <Monitor size={18} />
                              <span className="purchase-image-name">{img.display_name || img.alias}</span>
                              {selectedImage === img.id && <Check size={14} className="purchase-image-check" />}
                            </button>
                          ))}
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>

              {/* 网络配置展示 */}
              <div className="product-detail-section">
                <h3 className="product-detail-section-title">
                  <Network size={16} />
                  {t('product.networkConfig')}
                </h3>
                <div className="product-detail-spec-list">
                  <SpecRow icon={<Network size={14} />} label={t('product.downBandwidth')} value={formatMbps(product.network_down_min_mbps)} />
                  <SpecRow icon={<Network size={14} />} label={t('product.upBandwidth')} value={formatMbps(product.network_up_min_mbps)} />
                  <SpecRow icon={<Globe size={14} />} label={t('product.ipv4Mode')} value={product.ipv4_mode === 'eip' ? t('product.eipMode') : t('product.natMode')} />
                  {product.ipv6_enabled && (
                    <SpecRow icon={<Globe size={14} />} label={t('product.ipv6')} value={`${t('product.enabled')} ${Array.isArray(product.ipv6_configs) ? product.ipv6_configs.map(c => `/${c.prefix_len}`).join(' ') : ''}`} />
                  )}
                </div>
              </div>

              {/* 流量配置展示 */}
              <div className="product-detail-section">
                <h3 className="product-detail-section-title">
                  <TrafficCone size={16} />
                  {t('product.trafficConfig')}
                </h3>
                <div className="product-detail-spec-list">
                  <SpecRow icon={<Activity size={14} />} label={t('product.trafficLimit')} value={formatTraffic(product.traffic_min_gb)} />
                  <SpecRow icon={<Activity size={14} />} label={t('product.trafficCalcMode')} value={
                    product.traffic_calc_mode === 'both' ? t('product.trafficBoth') :
                    product.traffic_calc_mode === 'inbound' ? t('product.trafficInbound') :
                    product.traffic_calc_mode === 'outbound' ? t('product.trafficOutbound') :
                    t('product.trafficMax')
                  } />
                </div>
              </div>

              {/* 高级设置 */}
              <div className="product-detail-section">
                <button
                  className="purchase-advanced-toggle"
                  onClick={() => setAdvancedOpen(!advancedOpen)}
                >
                  {t('product.advancedSettings')}
                  {advancedOpen ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
                </button>
                {advancedOpen && (
                  <div className="purchase-advanced-body">
                    <div className="purchase-subsection">
                      <label className="purchase-label">{t('product.loginMethod')}</label>
                      <div className="purchase-tier-row">
                        {(['auto', 'password', 'sshkey'] as const).map(m => (
                          <button
                            key={m}
                            className={`purchase-tier-chip ${loginMethod === m ? 'active' : ''}`}
                            onClick={() => setLoginMethod(m)}
                          >
                            {t(`product.login_${m}`)}
                          </button>
                        ))}
                      </div>
                    </div>
                    {loginMethod === 'password' && (
                      <div className="purchase-subsection">
                        <label className="purchase-label">{t('product.sshPassword')}</label>
                        <input
                          className="purchase-input"
                          type="text"
                          value={sshPassword}
                          onChange={e => setSshPassword(e.target.value)}
                          placeholder={t('product.sshPasswordPlaceholder')}
                        />
                      </div>
                    )}
                    {loginMethod === 'sshkey' && (
                      <div className="purchase-subsection">
                        <label className="purchase-label">{t('product.sshPublicKey')}</label>
                        <textarea
                          className="purchase-input purchase-textarea"
                          value={sshPublicKey}
                          onChange={e => setSshPublicKey(e.target.value)}
                          placeholder={t('product.sshPublicKeyPlaceholder')}
                          rows={3}
                        />
                      </div>
                    )}
                  </div>
                )}
              </div>
            </div>

            {/* 右侧：价格 + 下单 */}
            <div className="product-detail-sidebar">
              <PriceCard product={product} />

              {/* 付款周期 */}
              <div className="purchase-section">
                <label className="purchase-label">{t('product.paymentPeriod')}</label>
                <div className="purchase-tier-row">
                  {allowedPeriods.map(p => (
                    <button
                      key={p.key}
                      className={`purchase-tier-chip ${paymentPeriod === p.key ? 'active' : ''}`}
                      onClick={() => { setPaymentPeriod(p.key); setTrial(false) }}
                      disabled={trial}
                    >
                      {p.label}
                      {p.discount < 1 && (
                        <span className="purchase-discount-tag">{(p.discount * 10).toFixed(1)}折</span>
                      )}
                    </button>
                  ))}
                </div>
              </div>

              {/* 试用 */}
              {product.trial_enabled && (
                <div className="purchase-section">
                  <label className="purchase-checkbox-row">
                    <input
                      type="checkbox"
                      checked={trial}
                      onChange={e => {
                        setTrial(e.target.checked)
                        if (e.target.checked) setPaymentPeriod('monthly')
                      }}
                    />
                    <Zap size={14} />
                    <span>{t('product.trialMode')} ({product.trial_hours}h / ¥{formatPrice(product.trial_price_cents)})</span>
                  </label>
                </div>
              )}

              {error && (
                <div className="purchase-error">
                  <AlertCircle size={14} />
                  <span>{error}</span>
                </div>
              )}

              <div className="purchase-footer-info">
                <div className="purchase-balance">
                  <span className="purchase-balance-label">{t('product.currentBalance')}</span>
                  <span className="purchase-balance-value font-number">¥{formatPrice(balanceCents)}</span>
                </div>
                <div className="purchase-total">
                  <span className="purchase-total-label">{t('product.totalPrice')}</span>
                  <span className="purchase-total-value font-number">¥{formatPrice(totalPrice)}</span>
                </div>
              </div>

              <div className="purchase-action-row">
                <button
                  className="purchase-cart-btn"
                  disabled={addingCart || !selectedImage}
                  onClick={handleAddToCart}
                >
                  {addingCart ? (
                    <><Loader2 className="animate-spin" size={16} />{t('cart.adding')}</>
                  ) : (
                    <><ShoppingCart size={16} />{t('cart.addToCart')}</>
                  )}
                </button>
                <button
                  className="purchase-submit-btn"
                  disabled={submitting || !selectedImage}
                  onClick={handleSubmit}
                >
                  {submitting ? (
                    <><Loader2 className="animate-spin" size={16} />{t('product.ordering')}</>
                  ) : (
                    t('product.buyNow')
                  )}
                </button>
              </div>
            </div>
          </div>

        </div>
      </div>

      <Modal
        open={showLoginModal}
        onClose={() => setShowLoginModal(false)}
        title={t('product.loginRequiredTitle')}
        confirmMode
        confirmText={t('product.goToLogin')}
        cancelText={t('common.cancel')}
        confirmVariant="primary"
        width={360}
        onConfirm={() => {
          setShowLoginModal(false)
          navigate('/login', { state: { redirect: '/products' } })
        }}
      >
        {t('product.loginRequiredMessage')}
      </Modal>
    </main>
  )
}
