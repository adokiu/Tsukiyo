import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Plus, ChevronRight, ChevronDown } from 'lucide-react'
import apiClient from '@/api/client'
import { PageLayout } from '@/components/PageLayout/PageLayout'
import { Button } from '@/components/Button/Button'
import { Select } from '@/components/Select/Select'
import { SlidePanel } from '@/components/SlidePanel/SlidePanel'
import { DataTable, type Column } from '@/components/DataTable/DataTable'
import { useFormValidation } from '@/hooks/useFormValidation'
import { useToastStore } from '@/stores/toast'
import { renderTextWithFlags } from '@/utils/emoji'

interface Category {
  id: string
  parent_id: string | null
  name: string
  sort: number
  status: string
  description: string
  children?: Category[]
}

export default function ProductCategoriesPage() {
  const { t } = useTranslation()
  const toast = useToastStore()
  const { validate, hasError, clearError, reset } = useFormValidation()
  const [categories, setCategories] = useState<Category[]>([])
  const [loading, setLoading] = useState(false)
  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set())
  const [panelOpen, setPanelOpen] = useState(false)
  const [editingCategory, setEditingCategory] = useState<Category | null>(null)
  const [parentId, setParentId] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const [formLevel, setFormLevel] = useState<'first' | 'second'>('first')
  const [formData, setFormData] = useState({
    name: '',
    sort: 0,
    description: '',
    status: 'active' as string,
  })

  const fetchCategories = useCallback(async () => {
    setLoading(true)
    try {
      const res = await apiClient.get('/commerce/categories')
      setCategories(res.data.data || [])
    } catch {
      toast.error(t('common.error'))
    } finally {
      setLoading(false)
    }
  }, [t, toast])

  useEffect(() => {
    fetchCategories()
  }, [fetchCategories])

  const toggleExpand = (id: string) => {
    setExpandedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const openCreate = () => {
    setEditingCategory(null)
    setParentId(null)
    setFormLevel('first')
    setFormData({ name: '', sort: 0, description: '', status: 'active' })
    reset()
    setPanelOpen(true)
  }

  const openEdit = (cat: Category) => {
    setEditingCategory(cat)
    setParentId(cat.parent_id)
    setFormData({ name: cat.name, sort: cat.sort, description: cat.description || '', status: cat.status })
    reset()
    setPanelOpen(true)
  }

  const handleSubmit = async () => {
    const result = validate([
      { field: 'name', valid: () => formData.name.trim().length > 0 },
    ])
    if (!result.ok) {
      toast.error(t('common.required'))
      return
    }
    setSaving(true)
    try {
      if (editingCategory) {
        await apiClient.put(`/commerce/categories/${editingCategory.id}`, {
          name: formData.name,
          sort: formData.sort,
          description: formData.description,
          status: formData.status,
        })
        toast.success(t('common.success'))
      } else {
        await apiClient.post('/commerce/categories', {
          parent_id: parentId || undefined,
          name: formData.name,
          sort: formData.sort,
          description: formData.description,
        })
        toast.success(t('common.success'))
      }
      if (!editingCategory && parentId) {
        setExpandedIds((prev) => new Set([...prev, parentId]))
      }
      setPanelOpen(false)
      fetchCategories()
      reset()
    } catch {
      toast.error(t('common.error'))
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm(t('common.confirmDelete'))) return
    try {
      await apiClient.delete(`/commerce/categories/${id}`)
      toast.success(t('common.success'))
      fetchCategories()
    } catch {
      toast.error(t('common.error'))
    }
  }

  const columns: Column<Category>[] = [
    {
      key: 'name',
      title: t('common.name'),
      width: 280,
      render: (row, _idx, ctx) => (
        <div className="flex items-center gap-1.5" style={{ paddingLeft: `${(ctx?.level ?? 0) * 20}px` }}>
          {ctx?.canExpand ? (
            <button onClick={() => ctx?.onToggleExpand?.(row.id)} className="p-0.5 text-tertiary hover:text-primary">
              {ctx?.expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
            </button>
          ) : (
            <span className="w-[18px]" />
          )}
          <span className="text-sm text-primary font-medium">{renderTextWithFlags(row.name)}</span>
        </div>
      ),
    },
    {
      key: 'sort',
      title: 'Sort',
      width: 80,
      render: (row) => <span className="font-number text-sm text-primary">{row.sort}</span>,
    },
    {
      key: 'description',
      title: t('common.description'),
      width: 240,
      render: (row) => <span className="text-sm text-secondary">{row.description || '-'}</span>,
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
      width: 160,
      render: (row) => (
        <div className="flex items-center gap-3">
          <button className="data-table-link-btn" onClick={() => openEdit(row)}>{t('common.edit')}</button>
          <button className="data-table-link-btn" onClick={() => handleDelete(row.id)} style={{ color: 'var(--color-red-500)' }}>{t('common.delete')}</button>
        </div>
      ),
    },
  ]

  return (
    <PageLayout
      rightSlot={
        <Button icon={<Plus size={16} />} onClick={() => openCreate()}>
          {t('common.add')}
        </Button>
      }
    >
      <div className="page-transition__content" style={{ flex: 1, overflow: 'auto' }}>
        <DataTable
          columns={columns}
          data={categories}
          rowKey={(r) => r.id}
          loading={loading}
          emptyText={t('common.noData')}
          expandedKeys={expandedIds}
          onToggleExpand={(key) => toggleExpand(String(key))}
          hasChildren={(row) => !!(row.children && row.children.length > 0)}
          getChildren={(row) => row.children || []}
        />
      </div>

      <SlidePanel
        open={panelOpen}
        onClose={() => setPanelOpen(false)}
        title={editingCategory ? `${t('common.edit')} - ${editingCategory.name}` : t('common.add')}
        width={480}
        footer={
          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setPanelOpen(false)}>{t('common.cancel')}</Button>
            <Button onClick={handleSubmit} loading={saving}>{t('common.confirm')}</Button>
          </div>
        }
      >
        <div className="space-y-5">
          {!editingCategory && (
            <div>
              <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('productCategory.categoryLevel')}</h4>
              <Select
                value={formLevel}
                options={[
                  { label: t('productCategory.firstLevel'), value: 'first' },
                  { label: t('productCategory.secondLevel'), value: 'second' },
                ]}
                onChange={(v) => {
                  setFormLevel(v as 'first' | 'second')
                  if (v === 'first') setParentId(null)
                }}
              />
            </div>
          )}
          {!editingCategory && formLevel === 'second' && (
            <div>
              <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('productCategory.parentCategory')}</h4>
              <Select
                value={parentId || ''}
                options={categories.map((c) => ({ label: renderTextWithFlags(c.name), value: c.id }))}
                onChange={(v) => setParentId(String(v) || null)}
              />
            </div>
          )}
          <div>
            <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('common.name')}</h4>
            <input
              className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('name') ? 'field-error' : ''}`}
              value={formData.name}
              onChange={(e) => { setFormData({ ...formData, name: e.target.value }); clearError('name') }}
              placeholder={t('common.name')}
            />
          </div>
          <div>
            <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">Sort</h4>
            <input
              type="number"
              className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm"
              value={formData.sort}
              onChange={(e) => setFormData({ ...formData, sort: parseInt(e.target.value) || 0 })}
            />
          </div>
          <div>
            <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('common.description')}</h4>
            <textarea
              className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm"
              rows={3}
              value={formData.description}
              onChange={(e) => setFormData({ ...formData, description: e.target.value })}
            />
          </div>
          {editingCategory && (
            <div>
              <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('common.status')}</h4>
              <Select
                value={formData.status}
                options={[
                  { label: t('common.active'), value: 'active' },
                  { label: t('common.disabled'), value: 'disabled' },
                ]}
                onChange={(v) => setFormData({ ...formData, status: String(v) })}
              />
            </div>
          )}
        </div>
      </SlidePanel>
    </PageLayout>
  )
}
