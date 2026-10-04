import { useState, useEffect } from 'react'
import { Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import apiClient from '@/api/client'
import { DataTable, type Column } from '@/components/DataTable/DataTable'
import { Button } from '@/components/Button/Button'
import { PageLayout } from '@/components/PageLayout/PageLayout'
import { SearchInput } from '@/components/SearchInput/SearchInput'
import { SlidePanel } from '@/components/SlidePanel/SlidePanel'
import { useFormValidation } from '@/hooks/useFormValidation'
import { useToastStore } from '@/stores/toast'

interface UserGroup {
  id: string
  name: string
  description: string
  is_builtin: boolean
  user_count: number
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

export default function UserGroupsPage() {
  const { t } = useTranslation()
  const toast = useToastStore()
  const [groups, setGroups] = useState<UserGroup[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [editGroup, setEditGroup] = useState<UserGroup | null>(null)
  const [editOpen, setEditOpen] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)

  const fetchGroups = () => {
    setLoading(true)
    apiClient.get('/user-groups').then((res) => setGroups(res.data.data || [])).finally(() => setLoading(false))
  }

  useEffect(() => { fetchGroups() }, [])

  const handleDelete = async (group: UserGroup) => {
    if (!confirm(t('common.confirm') + t('common.delete') + `: ${group.name}?`)) return
    try {
      await apiClient.delete(`/user-groups/${group.id}`)
      toast.success(t('common.success'))
      fetchGroups()
    } catch {
      toast.error(t('common.error'))
    }
  }

  const handleEdit = (group: UserGroup) => {
    setEditGroup(group)
    setEditOpen(true)
  }

  const filtered = groups.filter((g) =>
    !search || g.name.toLowerCase().includes(search.toLowerCase()) || g.description.toLowerCase().includes(search.toLowerCase())
  )

  const columns: Column<UserGroup>[] = [
    {
      key: 'name',
      title: t('userGroups.groupName'),
      width: 160,
      render: (row) => (
        <span className="text-sm text-primary font-medium">{row.name}</span>
      ),
    },
    {
      key: 'description',
      title: t('common.description'),
      width: 240,
      render: (row) => (
        <span className="text-sm text-secondary">{row.description || '-'}</span>
      ),
    },
    {
      key: 'user_count',
      title: t('userGroups.userCount'),
      width: 100,
      render: (row) => (
        <span className="font-number text-sm text-primary">{row.user_count}</span>
      ),
    },
    {
      key: 'is_builtin',
      title: t('userGroups.type'),
      width: 90,
      render: (row) => (
        <span className={row.is_builtin ? 'data-table-tag data-table-tag--disabled' : 'data-table-tag data-table-tag--online'}>
          {row.is_builtin ? t('userGroups.builtin') : t('userGroups.custom')}
        </span>
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
    {
      key: 'actions',
      title: t('common.actions'),
      width: 120,
      render: (row: UserGroup) => (
        <div className="flex items-center gap-3">
          <button className="data-table-link-btn" onClick={() => handleEdit(row)}>{t('common.edit')}</button>
          <button
            className="data-table-link-btn"
            onClick={() => handleDelete(row)}
            disabled={row.is_builtin}
            style={row.is_builtin ? { opacity: 0.3, cursor: 'not-allowed' } : { color: 'var(--color-red-500)' }}
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
          {t('userGroups.createGroup')}
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

      <EditGroupPanel
        open={editOpen}
        group={editGroup}
        onClose={() => { setEditOpen(false); setEditGroup(null) }}
        onSuccess={() => { fetchGroups(); setEditOpen(false); setEditGroup(null) }}
      />

      <CreateGroupPanel
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onSuccess={() => { fetchGroups(); setCreateOpen(false) }}
      />
    </PageLayout>
  )
}

function EditGroupPanel({ open, group, onClose, onSuccess }: { open: boolean; group: UserGroup | null; onClose: () => void; onSuccess: () => void }) {
  const { t } = useTranslation()
  const toast = useToastStore()
  const { validate, hasError, clearError, reset } = useFormValidation()
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (group) {
      setName(group.name)
      setDescription(group.description)
    }
  }, [group])

  const handleSave = async () => {
    if (!group) return
    const result = validate([
      { field: 'name', valid: () => name.trim().length > 0 },
    ])
    if (!result.ok) {
      toast.error(t('common.required'))
      return
    }
    setSaving(true)
    try {
      await apiClient.put(`/user-groups/${group.id}`, { name, description })
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
      title={group ? `${t('common.edit')} - ${group.name}` : t('common.edit')}
      width={480}
      footer={
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>{t('common.cancel')}</Button>
          <Button onClick={handleSave} loading={saving}>{t('common.save')}</Button>
        </div>
      }
    >
      <div className="space-y-5">
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('userGroups.groupName')}</h4>
          <input
            className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('name') ? 'field-error' : ''}`}
            value={name}
            onChange={(e) => { setName(e.target.value); clearError('name') }}
            disabled={group?.is_builtin}
          />
        </div>
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('common.description')}</h4>
          <textarea
            className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm"
            rows={4}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
        </div>
        {group && (
          <div>
            <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('userGroups.userCount')}</h4>
            <span className="font-number text-sm text-primary">{group.user_count}</span>
          </div>
        )}
      </div>
    </SlidePanel>
  )
}

function CreateGroupPanel({ open, onClose, onSuccess }: { open: boolean; onClose: () => void; onSuccess: () => void }) {
  const { t } = useTranslation()
  const toast = useToastStore()
  const { validate, hasError, clearError, reset } = useFormValidation()
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) {
      setName('')
      setDescription('')
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
      await apiClient.post('/user-groups', { name, description })
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
      title={t('userGroups.createGroup')}
      width={480}
      footer={
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>{t('common.cancel')}</Button>
          <Button onClick={handleSave} loading={saving}>{t('common.create')}</Button>
        </div>
      }
    >
      <div className="space-y-5">
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('userGroups.groupName')}</h4>
          <input
            className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('name') ? 'field-error' : ''}`}
            value={name}
            onChange={(e) => { setName(e.target.value); clearError('name') }}
            placeholder={t('userGroups.groupNamePlaceholder')}
          />
        </div>
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('common.description')}</h4>
          <textarea
            className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm"
            rows={4}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
        </div>
      </div>
    </SlidePanel>
  )
}
