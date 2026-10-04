import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { ticketsApi, type Ticket } from '@/api/tickets'
import { productsApi, type Product } from '@/api/products'
import { instancesApi, type InstanceItem } from '@/api/instances'
import { Select, type SelectOption } from '@/components/Select/Select'
import { Loader2, TicketIcon, Plus, MessageSquare, ChevronLeft, ChevronRight } from 'lucide-react'
import '@/views/ProductsView.css'

function formatTime(time: string): string {
  const d = new Date(time)
  return d.toLocaleString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

const statusColors: Record<string, string> = {
  open: 'text-blue-500',
  in_progress: 'text-yellow-500',
  waiting: 'text-orange-500',
  resolved: 'text-green-500',
  closed: 'text-gray-500',
}

const priorityColors: Record<string, string> = {
  low: 'text-gray-400',
  normal: 'text-blue-400',
  high: 'text-orange-500',
  urgent: 'text-red-500',
}

export function TicketsView() {
  const { t } = useTranslation()
  const [tickets, setTickets] = useState<Ticket[]>([])
  const [loading, setLoading] = useState(true)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [statusFilter, setStatusFilter] = useState('')
  const [showCreate, setShowCreate] = useState(false)
  const pageSize = 20

  const fetchTickets = useCallback(async () => {
    setLoading(true)
    try {
      const res = await ticketsApi.list({ page, page_size: pageSize, status: statusFilter || undefined })
      setTickets(res.data.data ?? [])
      setTotal(res.data.pagination?.total ?? 0)
    } catch {
      setTickets([])
    } finally {
      setLoading(false)
    }
  }, [page, statusFilter])

  useEffect(() => {
    fetchTickets()
  }, [fetchTickets])

  const totalPages = Math.ceil(total / pageSize)

  return (
    <main className="flex-1 overflow-y-auto overflow-x-hidden p-4 sm:p-6 md:p-8">
      <div className="mx-auto w-full max-w-[1600px]">
        <div className="space-y-6 animate-in fade-in slide-in-from-bottom-4 duration-700">

          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-2xl font-medium tracking-tight">{t('ticket.title')}</h2>
              <p className="text-sm text-muted-foreground mt-1">{t('ticket.subtitle')}</p>
            </div>
            <button
              className="apple-button"
              onClick={() => setShowCreate(true)}
            >
              <Plus size={16} className="mr-1" />
              {t('ticket.create')}
            </button>
          </div>

          {/* 状态筛选 */}
          <div className="flex gap-2 flex-wrap">
            {['', 'open', 'in_progress', 'waiting', 'resolved', 'closed'].map((s) => (
              <button
                key={s}
                className={`product-category-chip ${statusFilter === s ? 'active' : ''}`}
                onClick={() => { setStatusFilter(s); setPage(1) }}
              >
                {s === '' ? t('ticket.allStatus') : t(`ticket.status.${s}`)}
              </button>
            ))}
          </div>

          {/* 工单列表 */}
          {loading ? (
            <div className="flex justify-center py-20">
              <Loader2 className="animate-spin text-muted-foreground" size={28} />
            </div>
          ) : tickets.length === 0 ? (
            <div className="product-empty">
              <TicketIcon size={48} className="text-muted-foreground opacity-50" />
              <p className="text-muted-foreground mt-4">{t('ticket.empty')}</p>
            </div>
          ) : (
            <>
              <div className="space-y-3">
                {tickets.map((ticket) => (
                  <Link
                    key={ticket.id}
                    to={`/tickets/${ticket.id}`}
                    className="block p-4 rounded-xl border border-border bg-card hover:bg-secondary/50 transition-colors"
                  >
                    <div className="flex items-start justify-between gap-4">
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 mb-1">
                          <span className="text-xs font-mono text-muted-foreground">{ticket.ticket_no}</span>
                          <span className={`text-xs font-medium ${statusColors[ticket.status] ?? ''}`}>
                            {t(`ticket.status.${ticket.status}`)}
                          </span>
                          <span className={`text-xs ${priorityColors[ticket.priority] ?? ''}`}>
                            {t(`ticket.priority.${ticket.priority}`)}
                          </span>
                        </div>
                        <h3 className="text-sm font-medium truncate">{ticket.subject}</h3>
                        <div className="flex items-center gap-3 mt-1 text-xs text-muted-foreground">
                          <span>{t(`ticket.type.${ticket.type}`)}</span>
                          {ticket.product && <span>{ticket.product.name}</span>}
                          {ticket.instance && <span>{ticket.instance.name}</span>}
                          <span>{formatTime(ticket.created_at)}</span>
                        </div>
                      </div>
                      {ticket.replies && ticket.replies.length > 0 && (
                        <div className="flex items-center gap-1 text-xs text-muted-foreground">
                          <MessageSquare size={12} />
                          {ticket.replies.length}
                        </div>
                      )}
                    </div>
                  </Link>
                ))}
              </div>

              {/* 分页 */}
              {totalPages > 1 && (
                <div className="flex items-center justify-center gap-4">
                  <button
                    className="p-2 rounded-lg hover:bg-secondary disabled:opacity-30"
                    disabled={page <= 1}
                    onClick={() => setPage(page - 1)}
                  >
                    <ChevronLeft size={18} />
                  </button>
                  <span className="text-sm text-muted-foreground">{page} / {totalPages}</span>
                  <button
                    className="p-2 rounded-lg hover:bg-secondary disabled:opacity-30"
                    disabled={page >= totalPages}
                    onClick={() => setPage(page + 1)}
                  >
                    <ChevronRight size={18} />
                  </button>
                </div>
              )}
            </>
          )}

          {/* 创建工单弹窗 */}
          {showCreate && (
            <CreateTicketModal
              onClose={() => setShowCreate(false)}
              onCreated={() => { setShowCreate(false); fetchTickets() }}
            />
          )}

        </div>
      </div>
    </main>
  )
}

function CreateTicketModal({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const { t } = useTranslation()
  const [subject, setSubject] = useState('')
  const [content, setContent] = useState('')
  const [type, setType] = useState('tech')
  const [priority, setPriority] = useState('normal')
  const [productId, setProductId] = useState<string | undefined>(undefined)
  const [instanceId, setInstanceId] = useState<string | undefined>(undefined)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [products, setProducts] = useState<Product[]>([])
  const [instances, setInstances] = useState<InstanceItem[]>([])

  useEffect(() => {
    productsApi.list().then((res) => {
      setProducts(res.data.data ?? [])
    }).catch(() => {})
    instancesApi.list({ per_page: 100 }).then((res) => {
      setInstances(res.data.data ?? [])
    }).catch(() => {})
  }, [])

  const typeOptions: SelectOption[] = [
    { label: t('ticket.type.fault'), value: 'fault' },
    { label: t('ticket.type.finance'), value: 'finance' },
    { label: t('ticket.type.sales'), value: 'sales' },
    { label: t('ticket.type.tech'), value: 'tech' },
    { label: t('ticket.type.complaint'), value: 'complaint' },
    { label: t('ticket.type.other'), value: 'other' },
  ]

  const priorityOptions: SelectOption[] = [
    { label: t('ticket.priority.low'), value: 'low' },
    { label: t('ticket.priority.normal'), value: 'normal' },
    { label: t('ticket.priority.high'), value: 'high' },
    { label: t('ticket.priority.urgent'), value: 'urgent' },
  ]

  const productOptions: SelectOption[] = [
    { label: t('ticket.none'), value: '' },
    ...products.map((p) => ({ label: p.name, value: p.id })),
  ]

  const instanceOptions: SelectOption[] = [
    { label: t('ticket.none'), value: '' },
    ...instances.map((i) => ({ label: i.name, value: i.id })),
  ]

  const handleSubmit = async () => {
    if (!subject.trim() || !content.trim()) {
      setError(t('ticket.errorRequired'))
      return
    }
    setSubmitting(true)
    setError('')
    try {
      await ticketsApi.create({
        subject: subject.trim(),
        content: content.trim(),
        type,
        priority,
        product_id: productId || undefined,
        instance_id: instanceId || undefined,
      })
      onCreated()
    } catch (e: any) {
      setError(e?.response?.data?.error || t('ticket.errorCreate'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose}>
      <div
        className="bg-card border border-border rounded-2xl shadow-xl w-full max-w-2xl mx-4 max-h-[90vh] overflow-y-auto"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="p-6 space-y-4">
          <h3 className="text-lg font-medium">{t('ticket.createTitle')}</h3>

          <div>
            <label className="apple-label">{t('ticket.subject')}</label>
            <input
              className="apple-input w-full"
              value={subject}
              onChange={(e) => setSubject(e.target.value)}
              placeholder={t('ticket.subjectPlaceholder')}
              maxLength={256}
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="apple-label">{t('ticket.typeLabel')}</label>
              <Select
                value={type}
                options={typeOptions}
                onChange={(v) => setType(String(v))}
              />
            </div>
            <div>
              <label className="apple-label">{t('ticket.priorityLabel')}</label>
              <Select
                value={priority}
                options={priorityOptions}
                onChange={(v) => setPriority(String(v))}
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="apple-label">{t('ticket.productLabel')}</label>
              <Select
                value={productId ?? ''}
                options={productOptions}
                placeholder={t('ticket.productPlaceholder')}
                onChange={(v) => setProductId(String(v) || undefined)}
              />
            </div>
            <div>
              <label className="apple-label">{t('ticket.instanceLabel')}</label>
              <Select
                value={instanceId ?? ''}
                options={instanceOptions}
                placeholder={t('ticket.instancePlaceholder')}
                onChange={(v) => setInstanceId(String(v) || undefined)}
              />
            </div>
          </div>

          <div>
            <label className="apple-label">{t('ticket.content')}</label>
            <textarea
              className="apple-input w-full min-h-[120px] resize-y"
              value={content}
              onChange={(e) => setContent(e.target.value)}
              placeholder={t('ticket.contentPlaceholder')}
            />
          </div>

          {error && <p className="text-sm text-red-500">{error}</p>}

          <div className="flex justify-end gap-3 pt-2">
            <button className="apple-button-secondary" onClick={onClose}>
              {t('ticket.cancel')}
            </button>
            <button className="apple-button" onClick={handleSubmit} disabled={submitting}>
              {submitting ? <Loader2 size={16} className="animate-spin" /> : t('ticket.submit')}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
