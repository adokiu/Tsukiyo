import { useState, useEffect, useMemo } from 'react'
import { Plus, Columns3 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import apiClient from '@/api/client'
import { DataTable, type Column } from '@/components/DataTable/DataTable'
import { Button } from '@/components/Button/Button'
import { PageLayout } from '@/components/PageLayout/PageLayout'
import { SearchInput } from '@/components/SearchInput/SearchInput'
import { FilterBar, type FilterField } from '@/components/FilterBar/FilterBar'
import { SlidePanel } from '@/components/SlidePanel/SlidePanel'
import { Select } from '@/components/Select/Select'
import { useListQuery } from '@/hooks/useListQuery'
import { useFormValidation } from '@/hooks/useFormValidation'
import { useToastStore } from '@/stores/toast'

interface User {
  id: number
  username: string
  email: string
  status: string
  balance_cents: number
  phone: string
  qq: string
  real_name_status: string
  real_name: string
  id_card: string
  groups: string[]
  created_at: string
  last_login_at: string | null
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

export default function UsersPage() {
  const { t } = useTranslation()
  const toast = useToastStore()
  const [editUser, setEditUser] = useState<User | null>(null)
  const [editOpen, setEditOpen] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)
  const [columnMenuOpen, setColumnMenuOpen] = useState(false)

  const allColumnKeys = ['id', 'username', 'status', 'groups', 'email', 'balance_cents', 'phone', 'qq', 'real_name_status', 'created_at', 'last_login_at'] as const
  const [visibleCols, setVisibleCols] = useState<string[]>(['id', 'username', 'status', 'groups', 'email', 'balance_cents', 'created_at', 'last_login_at'])

  const { data: users, total, loading, page, perPage, search, filters, setPage, setPerPage, setSearch, setFilter, refresh } = useListQuery<User>('/users')

  const getStatusTagClass = (status: string) => {
    switch (status) {
      case 'active': return 'data-table-tag data-table-tag--online'
      case 'suspended': return 'data-table-tag data-table-tag--offline'
      case 'banned': return 'data-table-tag data-table-tag--offline'
      case 'deleted': return 'data-table-tag data-table-tag--disabled'
      default: return 'data-table-tag'
    }
  }

  const getStatusLabel = (status: string) => {
    switch (status) {
      case 'active': return t('common.active')
      case 'suspended': return t('common.inactive')
      case 'banned': return t('common.banned')
      case 'deleted': return t('common.deleted')
      default: return status
    }
  }

  const getRealNameStatusTagClass = (status: string) => {
    switch (status) {
      case 'approved': return 'data-table-tag data-table-tag--online'
      case 'pending': return 'data-table-tag'
      case 'rejected': return 'data-table-tag data-table-tag--offline'
      default: return 'data-table-tag data-table-tag--disabled'
    }
  }

  const getRealNameStatusLabel = (status: string) => {
    switch (status) {
      case 'approved': return t('user.realNameApproved')
      case 'pending': return t('user.realNamePending')
      case 'rejected': return t('user.realNameRejected')
      default: return t('user.realNameNone')
    }
  }

  const handleDelete = async (id: number) => {
    if (!confirm(t('common.confirm') + t('common.delete') + '?')) return
    try {
      await apiClient.delete(`/users/${id}`)
      toast.success(t('common.success'))
      refresh()
    } catch {
      toast.error(t('common.error'))
    }
  }

  const handleEdit = (user: User) => {
    setEditUser(user)
    setEditOpen(true)
  }

  const allColumns: Column<User>[] = [
    {
      key: 'id',
      title: 'ID',
      width: 70,
      render: (row) => (
        <span className="font-number text-sm text-tertiary">{row.id}</span>
      ),
    },
    {
      key: 'username',
      title: t('common.username'),
      width: 130,
      render: (row) => (
        <span className="text-sm text-primary">{row.username}</span>
      ),
    },
    {
      key: 'status',
      title: t('common.status'),
      width: 90,
      render: (row) => (
        <span className={getStatusTagClass(row.status)}>{getStatusLabel(row.status)}</span>
      ),
    },
    {
      key: 'groups',
      title: t('nav.userGroups'),
      width: 110,
      render: (row) => (
        <div className="flex items-center gap-1 flex-wrap">
          {row.groups && row.groups.length > 0 ? (
            row.groups.map((g) => (
              <span key={g} className="data-table-tag">{g}</span>
            ))
          ) : (
            <span className="text-sm text-tertiary">-</span>
          )}
        </div>
      ),
    },
    {
      key: 'email',
      title: t('common.email'),
      width: 200,
      render: (row) => (
        <span className="text-sm text-secondary">{row.email}</span>
      ),
    },
    {
      key: 'balance_cents',
      title: t('common.balance'),
      width: 100,
      render: (row) => (
        <span className="font-number text-sm text-primary">{`¥${(row.balance_cents / 100).toFixed(2)}`}</span>
      ),
    },
    {
      key: 'phone',
      title: t('common.phone'),
      width: 130,
      render: (row) => (
        <span className="text-sm text-secondary font-number">{row.phone || '-'}</span>
      ),
    },
    {
      key: 'qq',
      title: 'QQ',
      width: 110,
      render: (row) => (
        <span className="text-sm text-secondary font-number">{row.qq || '-'}</span>
      ),
    },
    {
      key: 'real_name_status',
      title: t('user.realNameStatus'),
      width: 100,
      render: (row) => (
        <span className={getRealNameStatusTagClass(row.real_name_status)}>{getRealNameStatusLabel(row.real_name_status)}</span>
      ),
    },
    {
      key: 'created_at',
      title: t('common.createdAt'),
      width: 150,
      render: (row) => (
        <span className="text-sm text-tertiary font-number">{formatDateTime(row.created_at)}</span>
      ),
    },
    {
      key: 'last_login_at',
      title: t('common.lastLoginAt'),
      width: 150,
      render: (row) => (
        <span className="text-sm text-tertiary font-number">{formatDateTime(row.last_login_at)}</span>
      ),
    },
    {
      key: 'actions',
      title: t('common.actions'),
      width: 110,
      render: (row: User) => (
        <div className="flex items-center gap-3">
          <button className="data-table-link-btn" onClick={() => handleEdit(row)}>{t('common.edit')}</button>
          <button className="data-table-link-btn" onClick={() => handleDelete(row.id)} style={{ color: 'var(--color-red-500)' }}>{t('common.delete')}</button>
        </div>
      ),
    },
  ]

  const columns = useMemo(() => {
    return allColumns.filter((col) => col.key === 'actions' || visibleCols.includes(col.key))
  }, [visibleCols, t])

  const columnLabels: Record<string, string> = {
    id: 'ID',
    username: t('common.username'),
    status: t('common.status'),
    groups: t('nav.userGroups'),
    email: t('common.email'),
    balance_cents: t('common.balance'),
    phone: t('common.phone'),
    qq: 'QQ',
    real_name_status: t('user.realNameStatus'),
    created_at: t('common.createdAt'),
    last_login_at: t('common.lastLoginAt'),
  }

  const statusOptions = [
    { label: t('common.active'), value: 'active' },
    { label: t('common.inactive'), value: 'suspended' },
    { label: t('common.banned'), value: 'banned' },
    { label: t('common.deleted'), value: 'deleted' },
  ]

  return (
    <PageLayout
      leftSlot={
        <>
          <SearchInput value={search} placeholder={t('common.search')} onChange={setSearch} />
          <FilterBar
            fields={[
              { key: 'status', label: t('common.status'), options: statusOptions },
            ] as FilterField[]}
            values={filters}
            onChange={setFilter}
          />
        </>
      }
      rightSlot={
        <div className="flex items-center gap-2">
          <div style={{ position: 'relative' }}>
            <Button variant="ghost" icon={<Columns3 size={16} />} onClick={() => setColumnMenuOpen(!columnMenuOpen)}>
              {t('common.columns')}
            </Button>
            {columnMenuOpen && (
              <>
                <div style={{ position: 'fixed', inset: 0, zIndex: 9998 }} onClick={() => setColumnMenuOpen(false)} />
                <div
                  style={{
                    position: 'absolute',
                    top: '100%',
                    right: 0,
                    marginTop: 4,
                    background: 'var(--color-white)',
                    border: '1px solid var(--color-gray-200)',
                    borderRadius: 8,
                    boxShadow: '0 4px 16px rgba(0,0,0,0.08)',
                    padding: '8px 0',
                    zIndex: 9999,
                    minWidth: 180,
                    maxHeight: 360,
                    overflowY: 'auto',
                  }}
                >
                  {allColumnKeys.map((key) => (
                    <label
                      key={key}
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: 8,
                        padding: '6px 16px',
                        cursor: 'pointer',
                        fontSize: 13,
                        color: 'var(--color-gray-700)',
                      }}
                      onMouseEnter={(e) => (e.currentTarget.style.background = 'var(--color-gray-50)')}
                      onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
                    >
                      <input
                        type="checkbox"
                        checked={visibleCols.includes(key)}
                        onChange={(e) => {
                          if (e.target.checked) {
                            setVisibleCols([...visibleCols, key])
                          } else {
                            setVisibleCols(visibleCols.filter((k) => k !== key))
                          }
                        }}
                      />
                      {columnLabels[key]}
                    </label>
                  ))}
                </div>
              </>
            )}
          </div>
          <Button icon={<Plus size={16} />} onClick={() => setCreateOpen(true)}>
            {t('user.createUser')}
          </Button>
        </div>
      }
    >
      <div className="page-transition__content" style={{ flex: 1, overflow: 'auto' }}>
        <DataTable
          columns={columns}
          data={users}
          rowKey={(r) => r.id}
          loading={loading}
          emptyText={t('common.noData')}
          pagination={{ page, size: perPage, total }}
          onPageChange={setPage}
          onSizeChange={setPerPage}
        />
      </div>

      <EditUserPanel
        open={editOpen}
        user={editUser}
        onClose={() => { setEditOpen(false); setEditUser(null) }}
        onSuccess={() => { refresh(); setEditOpen(false); setEditUser(null) }}
      />

      <CreateUserPanel
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onSuccess={() => { refresh(); setCreateOpen(false) }}
      />
    </PageLayout>
  )
}

function EditUserPanel({ open, user, onClose, onSuccess }: { open: boolean; user: User | null; onClose: () => void; onSuccess: () => void }) {
  const { t } = useTranslation()
  const toast = useToastStore()
  const { validate, hasError, clearError, reset } = useFormValidation()
  const [email, setEmail] = useState('')
  const [status, setStatus] = useState('active')
  const [balanceCents, setBalanceCents] = useState(0)
  const [balanceInput, setBalanceInput] = useState('')
  const [password, setPassword] = useState('')
  const [phone, setPhone] = useState('')
  const [qq, setQQ] = useState('')
  const [realNameStatus, setRealNameStatus] = useState('none')
  const [realName, setRealName] = useState('')
  const [idCard, setIDCard] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (user) {
      setEmail(user.email)
      setStatus(user.status)
      setBalanceCents(user.balance_cents)
      setBalanceInput((user.balance_cents / 100).toFixed(2))
      setPassword('')
      setPhone(user.phone || '')
      setQQ(user.qq || '')
      setRealNameStatus(user.real_name_status || 'none')
      setRealName(user.real_name || '')
      setIDCard(user.id_card || '')
    }
  }, [user])

  const handleSave = async () => {
    if (!user) return
    const result = validate([
      { field: 'email', valid: () => email.trim().length > 0 },
    ])
    if (!result.ok) {
      toast.error(t('common.required'))
      return
    }
    setSaving(true)
    try {
      const payload: Record<string, any> = { email, status, phone, qq, real_name_status: realNameStatus, real_name: realName, id_card: idCard }
      const parsedCents = Math.round(parseFloat(balanceInput) * 100)
      if (!isNaN(parsedCents) && parsedCents !== balanceCents) {
        payload.balance_cents = parsedCents
      }
      if (password) {
        payload.password = password
      }
      await apiClient.put(`/users/${user.id}`, payload)
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
      title={user ? `${t('common.edit')} - ${user.username}` : t('common.edit')}
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
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('common.username')}</h4>
          <input
            className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm"
            value={user?.username || ''}
            disabled
          />
        </div>
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('common.email')}</h4>
          <input
            className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('email') ? 'field-error' : ''}`}
            value={email}
            onChange={(e) => { setEmail(e.target.value); clearError('email') }}
          />
        </div>
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('common.status')}</h4>
          <Select
            value={status}
            onChange={(v) => setStatus(String(v))}
            options={[
              { label: t('common.active'), value: 'active' },
              { label: t('common.inactive'), value: 'suspended' },
              { label: t('common.banned'), value: 'banned' },
              { label: t('common.deleted'), value: 'deleted' },
            ]}
          />
        </div>
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('common.balance')}</h4>
          <div className="flex items-center gap-2">
            <span className="text-sm text-tertiary">¥</span>
            <input
              type="number"
              step="0.01"
              min="0"
              className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number"
              value={balanceInput}
              onChange={(e) => setBalanceInput(e.target.value)}
            />
          </div>
        </div>
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('common.phone')}</h4>
          <input
            className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
          />
        </div>
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">QQ</h4>
          <input
            className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number"
            value={qq}
            onChange={(e) => setQQ(e.target.value)}
          />
        </div>
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('user.realNameStatus')}</h4>
          <Select
            value={realNameStatus}
            onChange={(v) => setRealNameStatus(String(v))}
            options={[
              { label: t('user.realNameNone'), value: 'none' },
              { label: t('user.realNamePending'), value: 'pending' },
              { label: t('user.realNameApproved'), value: 'approved' },
              { label: t('user.realNameRejected'), value: 'rejected' },
            ]}
          />
        </div>
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('user.realName')}</h4>
          <input
            className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm"
            value={realName}
            onChange={(e) => setRealName(e.target.value)}
          />
        </div>
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('user.idCard')}</h4>
          <input
            className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number"
            value={idCard}
            onChange={(e) => setIDCard(e.target.value)}
          />
        </div>
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('auth.password')}</h4>
          <input
            type="password"
            className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder={t('user.passwordPlaceholder')}
          />
        </div>
      </div>
    </SlidePanel>
  )
}

function CreateUserPanel({ open, onClose, onSuccess }: { open: boolean; onClose: () => void; onSuccess: () => void }) {
  const { t } = useTranslation()
  const toast = useToastStore()
  const { validate, hasError, clearError, reset } = useFormValidation()
  const [username, setUsername] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [balanceInput, setBalanceInput] = useState('0.00')
  const [phone, setPhone] = useState('')
  const [qq, setQQ] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) {
      setUsername('')
      setEmail('')
      setPassword('')
      setBalanceInput('0.00')
      setPhone('')
      setQQ('')
      reset()
    }
  }, [open])

  const handleSave = async () => {
    const result = validate([
      { field: 'username', valid: () => username.trim().length > 0 },
      { field: 'email', valid: () => email.trim().length > 0 },
      { field: 'password', valid: () => password.trim().length > 0 },
    ])
    if (!result.ok) {
      toast.error(t('common.required'))
      return
    }
    setSaving(true)
    try {
      const payload: Record<string, any> = { username, email, password, phone, qq }
      const parsedCents = Math.round(parseFloat(balanceInput) * 100)
      if (!isNaN(parsedCents) && parsedCents > 0) {
        payload.balance_cents = parsedCents
      }
      await apiClient.post('/auth/register', payload)
      toast.success(t('common.success'))
      onSuccess()
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
      title={t('user.createUser')}
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
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('common.username')}</h4>
          <input
            className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('username') ? 'field-error' : ''}`}
            value={username}
            onChange={(e) => { setUsername(e.target.value); clearError('username') }}
          />
        </div>
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('common.email')}</h4>
          <input
            className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('email') ? 'field-error' : ''}`}
            value={email}
            onChange={(e) => { setEmail(e.target.value); clearError('email') }}
          />
        </div>
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('auth.password')}</h4>
          <input
            className={`w-full px-3 py-2 border border-surface-strong rounded-lg text-sm ${hasError('password') ? 'field-error' : ''}`}
            type="password"
            value={password}
            onChange={(e) => { setPassword(e.target.value); clearError('password') }}
          />
        </div>
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('common.balance')}</h4>
          <div className="flex items-center gap-2">
            <span className="text-sm text-tertiary">¥</span>
            <input
              type="number"
              step="0.01"
              min="0"
              className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number"
              value={balanceInput}
              onChange={(e) => setBalanceInput(e.target.value)}
            />
          </div>
        </div>
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">{t('common.phone')}</h4>
          <input
            className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
          />
        </div>
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">QQ</h4>
          <input
            className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number"
            value={qq}
            onChange={(e) => setQQ(e.target.value)}
          />
        </div>
      </div>
    </SlidePanel>
  )
}
