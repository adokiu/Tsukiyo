import { useQuery } from '@tanstack/react-query'
import { useState, useMemo, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router-dom'
import { productsApi, type Product, type ProductCategory } from '@/api/products'
import { useAppStore } from '@/stores/app'
import { Cpu, MemoryStick, HardDrive, Network, Loader2, Package, ChevronDown, Search, X } from 'lucide-react'
import { ProductConfigPanel } from '@/components/ProductConfigPanel'
import '@/views/ProductsView.css'

function formatPrice(cents: number): string {
  return (cents / 100).toFixed(2)
}

function formatMB(mb: number): string {
  if (mb >= 1024) return `${(mb / 1024).toFixed(0)}GB`
  return `${mb}MB`
}

function ProductCard({ product, onSelect }: { product: Product; onSelect: (id: string) => void }) {
  const { t } = useTranslation()

  const cpuText = product.vcpu_min === product.vcpu_max
    ? `${product.vcpu_min} 核`
    : `${product.vcpu_min}-${product.vcpu_max} 核`

  const memoryText = product.memory_min_mb === product.memory_max_mb
    ? formatMB(product.memory_min_mb)
    : `${formatMB(product.memory_min_mb)}-${formatMB(product.memory_max_mb)}`

  const diskText = product.disk_min_mb === product.disk_max_mb
    ? formatMB(product.disk_min_mb)
    : `${formatMB(product.disk_min_mb)}-${formatMB(product.disk_max_mb)}`

  const trafficText = product.traffic_min_gb <= 0
    ? '无限制'
    : `${product.traffic_min_gb}${product.traffic_max_gb !== product.traffic_min_gb ? `-${product.traffic_max_gb}` : ''} G`

  const upText = product.network_up_min_mbps <= 0 ? '不限' : `${product.network_up_min_mbps} M`
  const downText = product.network_down_min_mbps <= 0 ? '不限' : `${product.network_down_min_mbps} M`

  const dataDiskText = product.data_disk_allow
    ? '可选配'
    : '不可选配'

  const dailyPrice = (product.base_price_cents / 100 / 30).toFixed(2)

  return (
    <div className="product-card-container" onClick={() => onSelect(product.id)}>
      <div className="product-card">
        <div className="card-inner">
          {/* 卡片头部 */}
          <div className="product-card-header">
            <div className="header-title">
              <h5>{product.name}</h5>
              <span className="product-card-type-badge">{product.type === 'vm' ? 'KVM' : '容器'}</span>
            </div>
          </div>

          {/* 卡片内容 - 两列布局 */}
          <div className="product-card-body">
            <div className="product-description">
              <div className="spec-row">
                {/* 左列 - 图标+标签 */}
                <div className="spec-labels">
                  <div className="spec-label">
                    <Cpu size={16} className="spec-icon spec-icon-cpu" />
                    <span>CPU</span>
                  </div>
                  <div className="spec-label">
                    <MemoryStick size={16} className="spec-icon spec-icon-memory" />
                    <span>内存</span>
                  </div>
                  <div className="spec-label">
                    <Network size={16} className="spec-icon spec-icon-traffic" />
                    <span>流量</span>
                  </div>
                  <div className="spec-label">
                    <Network size={16} className="spec-icon spec-icon-traffic" />
                    <span>带宽</span>
                  </div>
                  <div className="spec-label">
                    <HardDrive size={16} className="spec-icon spec-icon-disk" />
                    <span>系统盘</span>
                  </div>
                  <div className="spec-label">
                    <HardDrive size={16} className="spec-icon spec-icon-disk" />
                    <span>数据盘</span>
                  </div>
                </div>
                {/* 右列 - 数值 */}
                <div className="spec-values">
                  <div className="spec-value">
                    <b>{cpuText}</b>
                  </div>
                  <div className="spec-value">
                    <b>{memoryText}</b>
                  </div>
                  <div className="spec-value">
                    <b>{trafficText}</b>
                  </div>
                  <div className="spec-value">
                    <b>{upText}</b> 上传
                    <b>{downText}</b> 下载
                  </div>
                  <div className="spec-value">
                    <b>{diskText}</b>
                  </div>
                  <div className="spec-value">
                    <b>{dataDiskText}</b>
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* 价格区域 */}
          <div className="product-parameters">
            <div className="pricing-info">
              <span className="price">
                <span className="price-currency">¥</span>
                <span className="price-amount">{formatPrice(product.base_price_cents)}</span>
                <span className="price-period">起/ 月</span>
                <span className="price-daily">约 ¥{dailyPrice}/天</span>
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

// 将旗帜 emoji 替换为 flagcdn SVG 图片
// 旗帜 emoji 由两个 regional indicator 字符组成 (U+1F1E6 ~ U+1F1FF)
// 使用 Array.from 正确处理 UTF-16 代理对
function renderCategoryName(name: string): React.ReactNode {
  const regionalIndicatorA = 0x1F1E6
  const chars = Array.from(name)
  const parts: React.ReactNode[] = []
  let key = 0
  let i = 0

  while (i < chars.length) {
    const code = chars[i].codePointAt(0)
    if (code && code >= regionalIndicatorA && code <= regionalIndicatorA + 25) {
      if (i + 1 < chars.length) {
        const nextCode = chars[i + 1].codePointAt(0)
        if (nextCode && nextCode >= regionalIndicatorA && nextCode <= regionalIndicatorA + 25) {
          const cc1 = String.fromCharCode(code - regionalIndicatorA + 0x61)
          const cc2 = String.fromCharCode(nextCode - regionalIndicatorA + 0x61)
          const countryCode = (cc1 + cc2).toLowerCase()
          parts.push(
            <img
              key={`flag-${key++}`}
              src={`https://flagcdn.com/${countryCode}.svg`}
              alt={countryCode}
              className="category-flag"
              loading="lazy"
            />
          )
          i += 2
          continue
        }
      }
      i += 1
      continue
    }
    let text = ''
    while (i < chars.length) {
      const c = chars[i].codePointAt(0)
      if (c && c >= regionalIndicatorA && c <= regionalIndicatorA + 25) break
      text += chars[i]
      i++
    }
    parts.push(<span key={`text-${key++}`}>{text}</span>)
  }

  return <span className="category-name-with-flag">{parts}</span>
}

export function ProductsView() {
  const { t } = useTranslation()
  const { getThemeConfig } = useAppStore()
  const [searchParams, setSearchParams] = useSearchParams()
  const selectedCategory = searchParams.get('category') || ''
  const selectedProductId = searchParams.get('pid') || ''
  const [expandedParent, setExpandedParent] = useState<string>('')
  const [searchQuery, setSearchQuery] = useState('')

  const setSelectedCategory = (id: string) => {
    setSearchParams(prev => {
      if (id) prev.set('category', id)
      else prev.delete('category')
      prev.delete('pid')
      return prev
    }, { replace: true })
  }

  const setSelectedProductId = (id: string) => {
    setSearchParams(prev => {
      if (id) prev.set('pid', id)
      else prev.delete('pid')
      return prev
    }, { replace: true })
  }

  const { data: categoriesData } = useQuery({
    queryKey: ['product-categories'],
    queryFn: () => productsApi.categories(),
  })

  const { data: productsData, isLoading } = useQuery({
    queryKey: ['products', selectedCategory],
    queryFn: () => productsApi.list(selectedCategory || undefined),
  })

  const categories = categoriesData?.data?.data ?? []
  const products = productsData?.data?.data ?? []

  const parentCategories = useMemo(() => {
    return categories.filter(cat => !cat.parent_id)
  }, [categories])

  // 搜索过滤分类
  const filteredParents = useMemo(() => {
    if (!searchQuery.trim()) return parentCategories
    const query = searchQuery.toLowerCase()
    return parentCategories.map(parent => {
      const parentMatch = parent.name.toLowerCase().includes(query)
      const filteredChildren = parent.children?.filter(child =>
        child.name.toLowerCase().includes(query)
      )
      if (parentMatch || (filteredChildren && filteredChildren.length > 0)) {
        return { ...parent, children: filteredChildren || parent.children }
      }
      return null
    }).filter(Boolean) as ProductCategory[]
  }, [parentCategories, searchQuery])

  // 默认展开第一个一级分类并选中第一个二级分类
  useEffect(() => {
    if (parentCategories.length > 0 && !expandedParent) {
      const firstParent = parentCategories[0]
      setExpandedParent(firstParent.id)
      if (!selectedCategory) {
        if (firstParent.children && firstParent.children.length > 0) {
          setSelectedCategory(firstParent.children[0].id)
        } else {
          setSelectedCategory(firstParent.id)
        }
      } else {
        // 展开选中分类所属的一级分类
        const parent = parentCategories.find(p =>
          p.id === selectedCategory || p.children?.some(c => c.id === selectedCategory)
        )
        if (parent) setExpandedParent(parent.id)
      }
    }
  }, [parentCategories, expandedParent, selectedCategory])

  const handleParentClick = (id: string) => {
    if (expandedParent === id) {
      setExpandedParent('')
    } else {
      setExpandedParent(id)
      const parent = categories.find(cat => cat.id === id)
      if (parent?.children && parent.children.length > 0) {
        setSelectedCategory(parent.children[0].id)
      } else {
        setSelectedCategory(id)
      }
    }
  }

  return (
    <>
      <section className="products-section">
        {/* 左侧单栏手风琴菜单 */}
        <aside className="products-sidebar">
          {/* 搜索框 */}
          <div className="products-search">
            <div className="products-search-box">
              <Search size={16} className="products-search-icon" />
              <input
                type="text"
                placeholder="请输入产品名称/关键字查找"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
              />
              {searchQuery && (
                <button type="button" className="products-search-clear" onClick={() => setSearchQuery('')}>
                  <X size={14} />
                </button>
              )}
            </div>
          </div>

          {/* 手风琴菜单 */}
          <div className="products-menu">
            {filteredParents.map((parent) => {
              const isExpanded = expandedParent === parent.id
              const hasChildren = parent.children && parent.children.length > 0
              return (
                <div key={parent.id} className="products-menu-item">
                  <div
                    className={`products-menu-title ${isExpanded ? 'active' : ''}`}
                    onClick={() => handleParentClick(parent.id)}
                  >
                    {renderCategoryName(parent.name)}
                    {hasChildren && (
                      <ChevronDown size={14} className={`products-menu-chevron ${isExpanded ? 'open' : ''}`} />
                    )}
                  </div>
                  {hasChildren && isExpanded && (
                    <div className="products-submenu">
                      {parent.children!.map((child) => (
                        <button
                          key={child.id}
                          type="button"
                          className={`products-submenu-item ${selectedCategory === child.id ? 'active' : ''}`}
                          onClick={() => setSelectedCategory(child.id)}
                        >
                          <div className="products-submenu-item-inner">
                            {renderCategoryName(child.name)}
                          </div>
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        </aside>

        {/* 右侧内容区域 */}
        <div className="products-main">

          {selectedProductId ? (
            <ProductConfigPanel
              productId={selectedProductId}
              onBack={() => setSelectedProductId('')}
            />
          ) : isLoading ? (
            <div className="products-loading">
              <Loader2 className="animate-spin text-muted-foreground" size={28} />
            </div>
          ) : products.length === 0 ? (
            <div className="product-empty">
              <Package size={48} className="text-muted-foreground opacity-50" />
              <p className="text-muted-foreground mt-4">{t('product.noProducts')}</p>
            </div>
          ) : (
            <div className="product-grid">
              {products.map((product) => (
                <ProductCard key={product.id} product={product} onSelect={setSelectedProductId} />
              ))}
            </div>
          )}
        </div>
      </section>
    </>
  )
}
