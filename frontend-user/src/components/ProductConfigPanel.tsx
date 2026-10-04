import { useQuery } from '@tanstack/react-query'
import { useState, useMemo, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { productsApi, type Product, type NodeImage } from '@/api/products'
import { useAuthStore } from '@/stores/auth'
import { useCartStore } from '@/stores/cart'
import { Slider } from '@/components/Slider/Slider'
import { Select } from '@/components/Select/Select'
import {
  ArrowLeft,
  ChevronDown, ChevronUp,
  Eye, EyeOff,
  Loader2, Package,
} from 'lucide-react'
import { getOSImage } from '@/utils/osImageHelper'
import { Modal } from '@/components/Modal/Modal'
import '@/views/ProductsView.css'

function formatPrice(cents: number): string {
  return (cents / 100).toFixed(2)
}

function formatMbps(mbps: number): string {
  if (mbps <= 0) return '不限'
  return `${mbps} Mbps`
}

function formatTraffic(gb: number): string {
  if (gb <= 0) return '不限'
  return `${gb} GB`
}

function generateMarks(min: number, max: number): number[] {
  const range = max - min
  if (range <= 1) return []
  const marks: number[] = []
  const count = Math.min(3, Math.floor(range))
  for (let i = 1; i <= count; i++) {
    marks.push(Math.round(min + (range * i) / (count + 1)))
  }
  return marks
}

function randomPassword(len: number = 18): string {
  const chars = 'abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789'
  let pwd = ''
  for (let i = 0; i < len; i++) {
    pwd += chars[Math.floor(Math.random() * chars.length)]
  }
  return pwd
}

function SpecRow({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <div className="config-spec-row">
      <div className="config-spec-icon">{icon}</div>
      <span className="config-spec-label">{label}</span>
      <span className="config-spec-value font-number">{value}</span>
    </div>
  )
}

export function ProductConfigPanel({ productId, onBack }: { productId: string; onBack: () => void }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { addToCart } = useCartStore()
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [showPriceDetail, setShowPriceDetail] = useState(false)
  const [showLoginModal, setShowLoginModal] = useState(false)

  const [vcpu, setVcpu] = useState(1)
  const [memoryMB, setMemoryMB] = useState(512)
  const [diskGB, setDiskGB] = useState(10)
  const [dataDiskGB, setDataDiskGB] = useState(0)
  const [netDownMbps, setNetDownMbps] = useState(0)
  const [netUpMbps, setNetUpMbps] = useState(0)
  const [natPortCount, setNATPortCount] = useState(0)
  const [ipv4EIPCount, setIPv4EIPCount] = useState(0)
  const [ipv6Items, setIPv6Items] = useState<{ prefix_len: number; count: number }[]>([])
  const [paymentPeriod, setPaymentPeriod] = useState<string>('monthly')
  const [trial, setTrial] = useState(false)
  const [selectedImage, setSelectedImage] = useState('')
  const [selectedCategory, setSelectedCategory] = useState('')
  const [loginMethod, setLoginMethod] = useState<'password' | 'sshkey'>('password')
  const [sshPassword, setSshPassword] = useState('')
  const [showPwd, setShowPwd] = useState(false)
  const [sshPublicKey, setSshPublicKey] = useState('')

  const { data: productData, isLoading } = useQuery({
    queryKey: ['product', productId],
    queryFn: () => productsApi.get(productId),
  })

  const product = productData?.data

  useEffect(() => {
    if (product) {
      setVcpu(product.vcpu_min)
      setMemoryMB(product.memory_min_mb)
      setDiskGB(Math.round(product.disk_min_mb / 1024))
      setNetDownMbps(product.network_down_min_mbps)
      setNetUpMbps(product.network_up_min_mbps)
      setNATPortCount(product.nat_port_min)
      setIPv4EIPCount(product.ipv4_eip_min || 1)
      const configs = Array.isArray(product.ipv6_configs) ? product.ipv6_configs : []
      setIPv6Items(configs.map(c => ({ prefix_len: c.prefix_len, count: c.min })))
      setPaymentPeriod(product.min_payment_period || 'monthly')
    }
  }, [product])

  useEffect(() => {
    if (loginMethod === 'password' && !sshPassword) {
      setSshPassword(randomPassword(18))
    }
  }, [loginMethod])

  const { data: imagesData, isLoading: imagesLoading } = useQuery({
    queryKey: ['product-images', productId],
    queryFn: () => productsApi.images(productId),
  })

  const images = imagesData?.data?.data ?? []
  const nodeId = imagesData?.data?.node_id ?? ''

  const imageCategories = useMemo(() => {
    const cats = new Map<string, NodeImage[]>()
    for (const img of images) {
      const catName = img.category_name || t('product.uncategorized')
      if (!cats.has(catName)) cats.set(catName, [])
      cats.get(catName)!.push(img)
    }
    return Array.from(cats.entries())
  }, [images, t])

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

  const memUseGB = product ? product.memory_max_mb >= 4096 : false
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

  // 下行带宽 tiers
  const netDownTiers: number[] = useMemo(() => {
    if (!product) return []
    if (product.network_down_custom_mode === 'tiers') {
      return Array.isArray(product.network_down_tiers) ? product.network_down_tiers : []
    }
    return []
  }, [product])

  const netDownMarks = useMemo(() => {
    if (!product) return []
    if (netDownTiers.length > 0) {
      return generateMarks(0, netDownTiers.length - 1).map(i => netDownTiers[i])
    }
    return generateMarks(product.network_down_min_mbps, product.network_down_max_mbps)
  }, [product, netDownTiers])

  // 上行带宽 tiers
  const netUpTiers: number[] = useMemo(() => {
    if (!product) return []
    if (product.network_up_custom_mode === 'tiers') {
      return Array.isArray(product.network_up_tiers) ? product.network_up_tiers : []
    }
    return []
  }, [product])

  const netUpMarks = useMemo(() => {
    if (!product) return []
    if (netUpTiers.length > 0) {
      return generateMarks(0, netUpTiers.length - 1).map(i => netUpTiers[i])
    }
    return generateMarks(product.network_up_min_mbps, product.network_up_max_mbps)
  }, [product, netUpTiers])

  // NAT 端口 tiers
  const natPortTiers: number[] = useMemo(() => {
    if (!product) return []
    if (product.nat_port_custom_mode === 'tiers') {
      return Array.isArray(product.nat_port_tiers) ? product.nat_port_tiers : []
    }
    return []
  }, [product])

  const natPortMarks = useMemo(() => {
    if (!product) return []
    if (natPortTiers.length > 0) {
      return generateMarks(0, natPortTiers.length - 1).map(i => natPortTiers[i])
    }
    return generateMarks(product.nat_port_min, product.nat_port_max)
  }, [product, natPortTiers])

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

    const netDownExtra = Math.max(0, netDownMbps - product.network_down_min_mbps)
    const netDownExtraCents = netDownExtra * product.network_down_unit_price_cents

    const netUpExtra = Math.max(0, netUpMbps - product.network_up_min_mbps)
    const netUpExtraCents = netUpExtra * product.network_up_unit_price_cents

    const natPortExtra = product.ipv4_mode === 'nat' ? Math.max(0, natPortCount - product.nat_port_min) : 0
    const natPortExtraCents = natPortExtra * product.nat_port_unit_price_cents

    // IPv4 独立IP增配差价
    const ipv4EIPExtra = product.ipv4_mode === 'eip' ? Math.max(0, ipv4EIPCount - product.ipv4_eip_min) : 0
    const ipv4EIPExtraCents = ipv4EIPExtra * product.ipv4_eip_unit_price_cents

    // IPv6 增配差价
    let ipv6ExtraCents = 0
    if (product.ipv6_enabled && Array.isArray(product.ipv6_configs)) {
      for (const item of ipv6Items) {
        const cfg = product.ipv6_configs.find(c => c.prefix_len === item.prefix_len)
        if (cfg) {
          const extra = Math.max(0, item.count - cfg.min)
          ipv6ExtraCents += extra * cfg.unit_price_cents
        }
      }
    }

    const monthlyCents = base + cpuExtraCents + memoryExtraCents + diskExtraCents + dataDiskExtraCents + netDownExtraCents + netUpExtraCents + natPortExtraCents + ipv4EIPExtraCents + ipv6ExtraCents

    let discount = 1.0
    if (paymentPeriod === 'quarterly') discount = product.quarterly_discount
    else if (paymentPeriod === 'half_yearly') discount = product.half_yearly_discount
    else if (paymentPeriod === 'yearly') discount = product.yearly_discount

    let total = Math.round(monthlyCents * months * discount)

    if (trial && product.trial_enabled) {
      total = product.trial_price_cents
    }

    return total
  }, [product, vcpu, memoryMB, diskGB, dataDiskGB, netDownMbps, netUpMbps, natPortCount, ipv4EIPCount, ipv6Items, paymentPeriod, trial])

  interface PriceDetailItem {
    name: string
    spec: string
    originalCents: number
    discountCents: number
    finalCents: number
  }

  const priceDetails = useMemo((): PriceDetailItem[] => {
    if (!product) return []
    const monthsMap: Record<string, number> = { monthly: 1, quarterly: 3, half_yearly: 6, yearly: 12 }
    const months = trial ? 1 : (monthsMap[paymentPeriod] || 1)
    let discount = 1.0
    if (paymentPeriod === 'quarterly') discount = product.quarterly_discount
    else if (paymentPeriod === 'half_yearly') discount = product.half_yearly_discount
    else if (paymentPeriod === 'yearly') discount = product.yearly_discount

    const calc = (monthlyCents: number) => {
      const original = Math.round(monthlyCents * months)
      const final = Math.round(monthlyCents * months * discount)
      return { originalCents: original, discountCents: original - final, finalCents: final }
    }

    const items: PriceDetailItem[] = []

    // 产品名称
    items.push({ name: t('product.detailProductName'), spec: product.name, ...calc(product.base_price_cents) })

    // CPU
    const cpuExtra = Math.max(0, vcpu - product.vcpu_min)
    items.push({ name: t('product.cpu'), spec: `${vcpu}${t('product.coresUnit')}`, ...calc(cpuExtra * product.vcpu_unit_price_cents) })

    // 内存
    const memSpec = product.memory_unit === 'gb' ? `${Math.round(memoryMB / 1024)}GB` : `${memoryMB}MB`
    const memExtra = Math.max(0, memoryMB - product.memory_min_mb)
    const memExtraCents = product.memory_unit === 'gb' ? Math.floor(memExtra / 1024) * product.memory_unit_price_cents : memExtra * product.memory_unit_price_cents
    items.push({ name: t('product.memory'), spec: memSpec, ...calc(memExtraCents) })

    // 系统盘
    const diskMB = diskGB * 1024
    const diskExtra = Math.max(0, diskMB - product.disk_min_mb)
    items.push({ name: t('product.systemDisk'), spec: `${diskGB}GB`, ...calc(Math.floor(diskExtra / 1024) * product.disk_unit_price_cents) })

    // 数据盘
    if (dataDiskGB > 0 && product.data_disk_allow) {
      items.push({ name: t('product.dataDisk'), spec: `${dataDiskGB}GB`, ...calc(Math.floor((dataDiskGB * 1024) / 1024) * product.data_disk_unit_price_cents) })
    }

    // 下行带宽
    const netDownExtra = Math.max(0, netDownMbps - product.network_down_min_mbps)
    items.push({ name: t('product.downBandwidth'), spec: `${netDownMbps}Mbps`, ...calc(netDownExtra * product.network_down_unit_price_cents) })

    // 上行带宽
    const netUpExtra = Math.max(0, netUpMbps - product.network_up_min_mbps)
    items.push({ name: t('product.upBandwidth'), spec: `${netUpMbps}Mbps`, ...calc(netUpExtra * product.network_up_unit_price_cents) })

    // NAT端口
    if (product.ipv4_mode === 'nat') {
      const natExtra = Math.max(0, natPortCount - product.nat_port_min)
      items.push({ name: t('product.natPortCount'), spec: `${natPortCount}`, ...calc(natExtra * product.nat_port_unit_price_cents) })
    }

    // IPv4 独立IP
    if (product.ipv4_mode === 'eip') {
      const eipExtra = Math.max(0, ipv4EIPCount - product.ipv4_eip_min)
      items.push({ name: t('product.ipCount'), spec: `${ipv4EIPCount}`, ...calc(eipExtra * product.ipv4_eip_unit_price_cents) })
    }

    // IPv6
    if (product.ipv6_enabled && Array.isArray(product.ipv6_configs)) {
      for (const item of ipv6Items) {
        const cfg = product.ipv6_configs.find(c => c.prefix_len === item.prefix_len)
        if (cfg) {
          const extra = Math.max(0, item.count - cfg.min)
          items.push({ name: `IPv6 /${cfg.prefix_len}`, spec: `${item.count}`, ...calc(extra * cfg.unit_price_cents) })
        }
      }
    }

    return items
  }, [product, vcpu, memoryMB, diskGB, dataDiskGB, netDownMbps, netUpMbps, natPortCount, ipv4EIPCount, ipv6Items, paymentPeriod, trial, t])

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
      network_down_mbps: netDownMbps,
      network_up_mbps: netUpMbps,
      nat_port_count: product.ipv4_mode === 'nat' ? natPortCount : undefined,
      ipv4_eip_count: product.ipv4_mode === 'eip' ? ipv4EIPCount : undefined,
      ipv6_items: product.ipv6_enabled && ipv6Items.length > 0 ? ipv6Items : undefined,
      trial: trial && product.trial_enabled,
    }
  }

  const validateConfig = (): boolean => {
    if (!product) return false
    if (!selectedImage) {
      setError(t('product.errorImageRequired'))
      return false
    }
    if (loginMethod === 'sshkey' && !sshPublicKey.trim()) {
      setError(t('product.errorSshKeyRequired'))
      return false
    }
    return true
  }

  const handleSubmit = async () => {
    if (!product) return
    setError('')
    if (!validateConfig()) return

    const token = useAuthStore.getState().token
    if (!token) {
      setShowLoginModal(true)
      return
    }

    setSubmitting(true)
    try {
      const config = buildConfig()!
      await addToCart(product.id, config, 1)
      navigate('/checkout')
    } catch (err: any) {
      setError(err.response?.data?.error || err.message || t('cart.addToCartFailed'))
    } finally {
      setSubmitting(false)
    }
  }

  if (isLoading) {
    return (
      <div className="config-loading">
        <Loader2 className="animate-spin" size={28} />
      </div>
    )
  }

  if (!product) {
    return (
      <div className="config-empty">
        <Package size={48} className="opacity-40" />
        <p className="config-empty-text">{t('product.notFound')}</p>
      </div>
    )
  }

  const ipv4Text = product.ipv4_mode === 'eip' ? t('product.eipMode') : t('product.natMode')
  const ipv6Text = product.ipv6_enabled
    ? `${t('product.enabled')} ${Array.isArray(product.ipv6_configs) ? product.ipv6_configs.map(c => `/${c.prefix_len}`).join(' ') : ''}`
    : t('product.notSupported')

  return (
    <div className="config-card">
      <div className="config-card-header">
        <div className="config-card-title-wrap">
          <h1 className="config-card-title">{product.name}</h1>
          <span className="config-card-tag tag-type">{product.type === 'vm' ? t('product.typeVM') : t('product.typeContainer')}</span>
          <span className="config-card-tag tag-traffic">{t('product.traffic')}{
            product.traffic_calc_mode === 'both' ? t('product.trafficBoth') :
            product.traffic_calc_mode === 'inbound' ? t('product.trafficInbound') :
            product.traffic_calc_mode === 'outbound' ? t('product.trafficOutbound') :
            t('product.trafficMax')
          }</span>
        </div>
        <button onClick={onBack} className="config-card-back">
          <ArrowLeft size={14} />
          {t('common.back')}
        </button>
      </div>

      <div className="config-card-body">
        {/* 操作系统 */}
        <div className="config-form-group">
          <label className="config-form-label">{t('product.systemImage')} <span className="config-required">*</span></label>
          <div className="config-form-content">
            {imagesLoading ? (
              <div className="config-loading-inline">
                <Loader2 className="animate-spin" size={16} />
                <span>{t('common.loading')}</span>
              </div>
            ) : images.length === 0 ? (
              <div className="config-empty-hint">{t('product.noImages')}</div>
            ) : (
              <div className="os-container">
                {imageCategories.map(([catName, imgs]) => {
                  const isCatActive = selectedCategory === catName
                  return (
                    <div
                      key={catName}
                      className={`os-card ${isCatActive ? 'active' : ''}`}
                      onClick={() => setSelectedCategory(catName)}
                    >
                      <div className="os-header">
                        <img className="img_os" src={getOSImage(catName)} alt={catName} height={20} />
                        <div className="os-name">{catName}</div>
                      </div>
                      <div className="os-version-select" onClick={(e) => e.stopPropagation()}>
                        <Select
                          value={isCatActive ? (selectedImage || undefined) : undefined}
                          options={imgs.map(img => ({ label: img.display_name || img.alias, value: img.id }))}
                          placeholder={t('product.selectVersion')}
                          disabled={!isCatActive}
                          onChange={(v) => setSelectedImage(String(v))}
                        />
                      </div>
                    </div>
                  )
                })}
              </div>
            )}
          </div>
        </div>

        {/* CPU */}
        <div className="config-form-group">
          <label className="config-form-label">{t('product.cores')}</label>
          <div className="config-form-content">
            {product.vcpu_min === product.vcpu_max ? (
              <div className="config-info-display">
                <span>{product.vcpu_min}{t('product.coresUnit')}</span>
              </div>
            ) : vcpuTiers.length > 0 ? (
              <div className="config-btn-group">
                {vcpuTiers.map(tier => (
                  <button
                    key={tier}
                    className={`config-btn ${vcpu === tier ? 'active' : ''}`}
                    onClick={() => setVcpu(tier)}
                  >
                    {tier}{t('product.coresUnit')}
                  </button>
                ))}
              </div>
            ) : (
              <Slider
                min={product.vcpu_min}
                max={product.vcpu_max}
                step={1}
                value={vcpu}
                onChange={setVcpu}
                unit={t('product.coresUnit')}
                marks={vcpuMarks}
              />
            )}
          </div>
        </div>

        {/* 内存 */}
        <div className="config-form-group">
          <label className="config-form-label">{t('product.memory')}</label>
          <div className="config-form-content">
            {product.memory_min_mb === product.memory_max_mb ? (
              <div className="config-info-display">
                <span>{memDisplayMin}{memUnit}</span>
              </div>
            ) : memDisplayTiers && memDisplayTiers.length > 0 ? (
              <div className="config-btn-group">
                {memDisplayTiers.map(tier => (
                  <button
                    key={tier}
                    className={`config-btn ${memDisplayValue === tier ? 'active' : ''}`}
                    onClick={() => setMemoryMB(memUseGB ? tier * 1024 : tier)}
                  >
                    {tier}{memUnit}
                  </button>
                ))}
              </div>
            ) : (
              <Slider
                min={memDisplayMin}
                max={memDisplayMax}
                step={memUseGB ? 1 : 32}
                value={memDisplayValue}
                onChange={(v) => setMemoryMB(memUseGB ? v * 1024 : v)}
                unit={memUnit}
                marks={memoryMarks}
              />
            )}
          </div>
        </div>

        {/* 下行带宽 */}
        <div className="config-form-group">
          <label className="config-form-label">{t('product.downBandwidth')}</label>
          <div className="config-form-content">
            {product.network_down_max_mbps > product.network_down_min_mbps ? (
              netDownTiers.length > 0 ? (
                <div className="config-btn-group">
                  {netDownTiers.map(tier => (
                    <button
                      key={tier}
                      className={`config-btn ${netDownMbps === tier ? 'active' : ''}`}
                      onClick={() => setNetDownMbps(tier)}
                    >
                      {tier}Mbps
                    </button>
                  ))}
                </div>
              ) : (
                <Slider
                  min={product.network_down_min_mbps}
                  max={product.network_down_max_mbps}
                  step={1}
                  value={netDownMbps}
                  onChange={setNetDownMbps}
                  unit="Mbps"
                  marks={netDownMarks}
                />
              )
            ) : (
              <div className="config-info-display">
                <span>{formatMbps(product.network_down_min_mbps)}</span>
              </div>
            )}
          </div>
        </div>

        {/* 上行带宽 */}
        <div className="config-form-group">
          <label className="config-form-label">{t('product.upBandwidth')}</label>
          <div className="config-form-content">
            {product.network_up_max_mbps > product.network_up_min_mbps ? (
              netUpTiers.length > 0 ? (
                <div className="config-btn-group">
                  {netUpTiers.map(tier => (
                    <button
                      key={tier}
                      className={`config-btn ${netUpMbps === tier ? 'active' : ''}`}
                      onClick={() => setNetUpMbps(tier)}
                    >
                      {tier}Mbps
                    </button>
                  ))}
                </div>
              ) : (
                <Slider
                  min={product.network_up_min_mbps}
                  max={product.network_up_max_mbps}
                  step={1}
                  value={netUpMbps}
                  onChange={setNetUpMbps}
                  unit="Mbps"
                  marks={netUpMarks}
                />
              )
            ) : (
              <div className="config-info-display">
                <span>{formatMbps(product.network_up_min_mbps)}</span>
              </div>
            )}
          </div>
        </div>

        {/* 流量限制 */}
        <div className="config-form-group">
          <label className="config-form-label">{t('product.trafficLimit')}</label>
          <div className="config-form-content">
            <div className="config-info-display">
              <span>{formatTraffic(product.traffic_min_gb)}</span>
            </div>
          </div>
        </div>

        {/* IPv4 地址 */}
        <div className="config-form-group">
          <label className="config-form-label">IPv4 {t('product.address')}</label>
          <div className="config-form-content">
            <div className="config-info-display">
              <span>{ipv4Text}</span>
            </div>
          </div>
        </div>

        {/* IPv4 独立IP增配 */}
        {product.ipv4_mode === 'eip' && product.ipv4_eip_max > product.ipv4_eip_min && (
          <div className="config-form-group">
            <label className="config-form-label">{t('product.ipCount')}</label>
            <div className="config-form-content">
              {product.ipv4_eip_custom_mode === 'tiers' && Array.isArray(product.ipv4_eip_tiers) && product.ipv4_eip_tiers.length > 0 ? (
                <div className="config-btn-group">
                  {product.ipv4_eip_tiers.map(tier => (
                    <button
                      key={tier}
                      className={`config-btn ${ipv4EIPCount === tier ? 'active' : ''}`}
                      onClick={() => setIPv4EIPCount(tier)}
                    >
                      {tier}
                    </button>
                  ))}
                </div>
              ) : (
                <Slider
                  min={product.ipv4_eip_min}
                  max={product.ipv4_eip_max}
                  step={1}
                  value={ipv4EIPCount}
                  onChange={setIPv4EIPCount}
                  unit=""
                />
              )}
            </div>
          </div>
        )}

        {/* NAT 端口映射 */}
        {product.ipv4_mode === 'nat' && product.nat_port_max > product.nat_port_min && (
          <div className="config-form-group">
            <label className="config-form-label">{t('product.natPortCount')}</label>
            <div className="config-form-content">
              {natPortTiers.length > 0 ? (
                <div className="config-btn-group">
                  {natPortTiers.map(tier => (
                    <button
                      key={tier}
                      className={`config-btn ${natPortCount === tier ? 'active' : ''}`}
                      onClick={() => setNATPortCount(tier)}
                    >
                      {tier}
                    </button>
                  ))}
                </div>
              ) : (
                <Slider
                  min={product.nat_port_min}
                  max={product.nat_port_max}
                  step={1}
                  value={natPortCount}
                  onChange={setNATPortCount}
                  unit=""
                  marks={natPortMarks}
                />
              )}
            </div>
          </div>
        )}

        {/* IPv6 地址 */}
        <div className="config-form-group">
          <label className="config-form-label">IPv6 {t('product.address')}</label>
          <div className="config-form-content">
            <div className="config-info-display">
              <span>{ipv6Text}</span>
            </div>
          </div>
        </div>

        {/* IPv6 多前缀增配 */}
        {product.ipv6_enabled && Array.isArray(product.ipv6_configs) && product.ipv6_configs.map((cfg, idx) => {
          if (cfg.max <= cfg.min) return null
          const currentItem = ipv6Items.find(i => i.prefix_len === cfg.prefix_len)
          const currentCount = currentItem ? currentItem.count : cfg.min
          const updateCount = (count: number) => {
            setIPv6Items(prev => {
              const existing = prev.findIndex(i => i.prefix_len === cfg.prefix_len)
              if (existing >= 0) {
                const next = [...prev]
                next[existing] = { prefix_len: cfg.prefix_len, count }
                return next
              }
              return [...prev, { prefix_len: cfg.prefix_len, count }]
            })
          }
          return (
            <div key={idx} className="config-form-group">
              <label className="config-form-label">/{cfg.prefix_len} {t('product.count')}</label>
              <div className="config-form-content">
                {cfg.custom_mode === 'tiers' && Array.isArray(cfg.tiers) && cfg.tiers.length > 0 ? (
                  <div className="config-btn-group">
                    {cfg.tiers.map(tier => (
                      <button
                        key={tier}
                        className={`config-btn ${currentCount === tier ? 'active' : ''}`}
                        onClick={() => updateCount(tier)}
                      >
                        {tier}
                      </button>
                    ))}
                  </div>
                ) : (
                  <Slider
                    min={cfg.min}
                    max={cfg.max}
                    step={1}
                    value={currentCount}
                    onChange={updateCount}
                    unit=""
                  />
                )}
              </div>
            </div>
          )
        })}

        {/* 系统盘 */}
        <div className="config-form-group">
          <label className="config-form-label">{t('product.systemDisk')}</label>
          <div className="config-form-content">
            {product.disk_min_mb === product.disk_max_mb ? (
              <div className="config-info-display">
                <span>{Math.round(product.disk_min_mb / 1024)}GB</span>
              </div>
            ) : diskTiers.length > 0 ? (
              <div className="config-btn-group">
                {diskTiers.map(tier => (
                  <button
                    key={tier}
                    className={`config-btn ${diskGB === tier ? 'active' : ''}`}
                    onClick={() => setDiskGB(tier)}
                  >
                    {tier}GB
                  </button>
                ))}
              </div>
            ) : (
              <Slider
                min={Math.max(1, Math.round(product.disk_min_mb / 1024))}
                max={Math.round(product.disk_max_mb / 1024)}
                step={1}
                value={diskGB}
                onChange={setDiskGB}
                unit="GB"
                marks={diskMarks}
              />
            )}
          </div>
        </div>

        {/* 数据盘 */}
        {product.data_disk_allow && product.data_disk_max_mb > 0 && (
          <div className="config-form-group">
            <label className="config-form-label">{t('product.dataDisk')} ({t('product.optional')})</label>
            <div className="config-form-content">
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
          </div>
        )}

        {/* 登录方式 */}
        <div className="config-form-group">
          <label className="config-form-label">{t('product.loginMethod')}</label>
          <div className="config-form-content">
            <div className="config-tier-row">
              <button
                className={`config-tier-chip ${loginMethod === 'password' ? 'active' : ''}`}
                onClick={() => setLoginMethod('password')}
              >
                {t('product.login_password')}
              </button>
              <button
                className={`config-tier-chip ${loginMethod === 'sshkey' ? 'active' : ''}`}
                onClick={() => setLoginMethod('sshkey')}
              >
                {t('product.login_sshkey')}
              </button>
            </div>

            {loginMethod === 'password' && (
              <div className="config-password-row">
                <div className="config-password-input-wrap">
                  <input
                    className="config-password-input"
                    type={showPwd ? 'text' : 'password'}
                    value={sshPassword}
                    onChange={e => setSshPassword(e.target.value)}
                  />
                  <button
                    type="button"
                    className="config-password-toggle"
                    onClick={() => setShowPwd(!showPwd)}
                  >
                    {showPwd ? <EyeOff size={16} /> : <Eye size={16} />}
                  </button>
                </div>
                <button
                  type="button"
                  className="config-password-random"
                  onClick={() => setSshPassword(randomPassword(18))}
                >
                  {t('product.randomGenerate')}
                </button>
              </div>
            )}

            {loginMethod === 'sshkey' && (
              <div className="config-sshkey-row">
                <textarea
                  className="config-textarea"
                  value={sshPublicKey}
                  onChange={e => setSshPublicKey(e.target.value)}
                  placeholder={t('product.sshPublicKeyPlaceholder')}
                  rows={4}
                />
              </div>
            )}
          </div>
        </div>

        {/* 付款周期 */}
        <div className="config-form-group">
          <label className="config-form-label">{t('product.paymentPeriod')}</label>
          <div className="config-form-content">
            <div className="config-tier-row">
              {allowedPeriods.map(p => (
                <button
                  key={p.key}
                  className={`config-tier-chip ${paymentPeriod === p.key ? 'active' : ''}`}
                  onClick={() => { setPaymentPeriod(p.key); setTrial(false) }}
                  disabled={trial}
                >
                  {p.label}
                  {p.discount < 1 && (
                    <span className="config-discount-tag">{(p.discount * 10).toFixed(1)}折</span>
                  )}
                </button>
              ))}
            </div>
          </div>
        </div>

        {/* 试用 */}
        {product.trial_enabled && (
          <div className="config-form-group">
            <label className="config-form-label">{t('product.trialMode')}</label>
            <div className="config-form-content">
              <label className="config-checkbox-row">
                <input
                  type="checkbox"
                  checked={trial}
                  onChange={e => {
                    setTrial(e.target.checked)
                    if (e.target.checked) setPaymentPeriod('monthly')
                  }}
                />
                <span>{product.trial_hours}h / ¥{formatPrice(product.trial_price_cents)}</span>
              </label>
            </div>
          </div>
        )}
      </div>

      {/* 底部结算栏 */}
      <div className="config-card-footer">
        <div className="config-footer-left">
          <div className="config-footer-price-row">
            <span className="config-footer-label">{t('product.totalPrice')}</span>
            <span className="config-footer-currency">¥</span>
            <span className="config-footer-price font-number">{formatPrice(totalPrice)}</span>
          </div>
        </div>
        <div className="config-footer-right">
          {error && (
            <div className="config-error">
              <span>{error}</span>
            </div>
          )}
          <div className="config-footer-actions">
            <div className="config-price-detail-trigger-wrap">
              <button
                type="button"
                className="config-price-detail-trigger"
                onClick={() => setShowPriceDetail(!showPriceDetail)}
              >
                {t('product.priceDetail')}
                {showPriceDetail ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
              </button>
              {showPriceDetail && product && (
                <div className="config-price-detail-tooltip">
                  <div className="config-price-detail-header">
                    <span className="config-price-detail-col-name">{t('product.detailItem')}</span>
                    <span className="config-price-detail-col-spec">{t('product.detailSpec')}</span>
                    <span className="config-price-detail-col-price">{t('product.detailOriginal')}</span>
                    <span className="config-price-detail-col-discount">{t('product.detailDiscount')}</span>
                    <span className="config-price-detail-col-final">{t('product.detailFinal')}</span>
                  </div>
                  {priceDetails.map((item, idx) => (
                    <div key={idx} className="config-price-detail-row">
                      <span className="config-price-detail-col-name">{item.name}</span>
                      <span className="config-price-detail-col-spec">{item.spec}</span>
                      <span className="config-price-detail-col-price font-number">¥{formatPrice(item.originalCents)}</span>
                      <span className="config-price-detail-col-discount font-number">¥{formatPrice(item.discountCents)}</span>
                      <span className="config-price-detail-col-final font-number">¥{formatPrice(item.finalCents)}</span>
                    </div>
                  ))}
                  <div className="config-price-detail-total">
                    <span className="config-price-detail-total-label">{t('product.totalPrice')}</span>
                    <span className="config-price-detail-total-value font-number">¥{formatPrice(totalPrice)}</span>
                  </div>
                </div>
              )}
            </div>
            <button
              className="config-submit-btn"
              disabled={submitting || !selectedImage}
              onClick={handleSubmit}
            >
              {submitting ? t('product.ordering') : t('product.buyNow')}
            </button>
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
    </div>
  )
}
