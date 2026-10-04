import { useState, useEffect, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import apiClient from '@/api/client'
import { Select, type SelectOption } from '@/components/Select/Select'
import { ArrowLeft, Send } from 'lucide-react'

interface TicketReply {
  id: string
  ticket_id: string
  user_id: number
  user?: { id: number; username: string; email: string }
  content: string
  is_staff: boolean
  created_at: string
}

interface Ticket {
  id: string
  ticket_no: string
  user_id: number
  user?: { id: number; username: string; email: string }
  subject: string
  content: string
  type: string
  priority: string
  status: string
  product_id?: string
  product?: { id: string; name: string }
  instance_id?: string
  instance?: { id: string; name: string }
  assignee_id?: number
  assignee?: { id: number; username: string }
  replies?: TicketReply[]
  closed_at?: string
  created_at: string
  updated_at: string
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

export default function TicketDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { t } = useTranslation()
  const [ticket, setTicket] = useState<Ticket | null>(null)
  const [loading, setLoading] = useState(true)
  const [replyContent, setReplyContent] = useState('')
  const [replying, setReplying] = useState(false)
  const [updating, setUpdating] = useState(false)

  const fetchTicket = useCallback(async () => {
    if (!id) return
    setLoading(true)
    try {
      const res = await apiClient.get(`/tickets/${id}`)
      setTicket(res.data)
    } catch {
      setTicket(null)
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => { fetchTicket() }, [fetchTicket])

  const handleReply = async () => {
    if (!id || !replyContent.trim()) return
    setReplying(true)
    try {
      await apiClient.post(`/tickets/${id}/reply`, { content: replyContent.trim() })
      setReplyContent('')
      await fetchTicket()
    } catch {
    } finally {
      setReplying(false)
    }
  }

  const handleUpdateStatus = async (status: string) => {
    if (!id) return
    setUpdating(true)
    try {
      await apiClient.put(`/tickets/${id}/status`, { status })
      await fetchTicket()
    } catch {
    } finally {
      setUpdating(false)
    }
  }

  const handleUpdatePriority = async (priority: string) => {
    if (!id) return
    setUpdating(true)
    try {
      await apiClient.put(`/tickets/${id}/priority`, { priority })
      await fetchTicket()
    } catch {
    } finally {
      setUpdating(false)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-[#087ed1] border-t-transparent" />
      </div>
    )
  }

  if (!ticket) {
    return (
      <div className="flex flex-col items-center justify-center h-full">
        <p className="text-tertiary">{t('ticket.notFound')}</p>
        <button className="mt-4 text-primary text-sm" onClick={() => navigate('/admin/ticketManagement/tickets')}>
          {t('ticket.back')}
        </button>
      </div>
    )
  }

  const isClosed = ticket.status === 'closed'

  return (
    <div className="flex flex-col h-full p-6 overflow-y-auto">
      <div className="max-w-4xl mx-auto w-full space-y-6">
        {/* 返回 */}
        <button
          className="flex items-center gap-1 text-sm text-tertiary hover:text-primary transition-colors"
          onClick={() => navigate('/admin/ticketManagement/tickets')}
        >
          <ArrowLeft size={16} />
          {t('ticket.back')}
        </button>

        {/* 工单信息 */}
        <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6 space-y-4">
          <div className="flex items-start justify-between gap-4">
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 mb-2">
                <span className="text-xs font-mono text-tertiary">{ticket.ticket_no}</span>
              </div>
              <h2 className="text-xl font-medium">{ticket.subject}</h2>
            </div>
          </div>

          {/* 管理操作 */}
          <div className="flex flex-wrap gap-3 pt-3 border-t border-gray-200 dark:border-gray-700">
            <div className="flex items-center gap-2">
              <label className="text-xs text-tertiary">{t('common.status')}</label>
              <Select
                value={ticket.status}
                options={[
                  { label: t('ticket.status.open'), value: 'open' },
                  { label: t('ticket.status.inProgress'), value: 'in_progress' },
                  { label: t('ticket.status.waiting'), value: 'waiting' },
                  { label: t('ticket.status.resolved'), value: 'resolved' },
                  { label: t('ticket.status.closed'), value: 'closed' },
                ] as SelectOption[]}
                onChange={(v) => handleUpdateStatus(String(v))}
                disabled={updating}
              />
            </div>
            <div className="flex items-center gap-2">
              <label className="text-xs text-tertiary">{t('ticket.priorityLabel')}</label>
              <Select
                value={ticket.priority}
                options={[
                  { label: t('ticket.priority.low'), value: 'low' },
                  { label: t('ticket.priority.normal'), value: 'normal' },
                  { label: t('ticket.priority.high'), value: 'high' },
                  { label: t('ticket.priority.urgent'), value: 'urgent' },
                ] as SelectOption[]}
                onChange={(v) => handleUpdatePriority(String(v))}
                disabled={updating}
              />
            </div>
          </div>

          {/* 详细信息 */}
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-sm pt-3 border-t border-gray-200 dark:border-gray-700">
            <div>
              <span className="text-tertiary text-xs">{t('common.username')}</span>
              <p>{ticket.user?.username || `#${ticket.user_id}`}</p>
            </div>
            <div>
              <span className="text-tertiary text-xs">{t('ticket.typeLabel')}</span>
              <p>{t(`ticket.type.${ticket.type}`)}</p>
            </div>
            {ticket.product && (
              <div>
                <span className="text-tertiary text-xs">{t('ticket.productLabel')}</span>
                <p>{ticket.product.name}</p>
              </div>
            )}
            {ticket.instance && (
              <div>
                <span className="text-tertiary text-xs">{t('ticket.instanceLabel')}</span>
                <p>{ticket.instance.name}</p>
              </div>
            )}
            <div>
              <span className="text-tertiary text-xs">{t('ticket.assignee')}</span>
              <p>{ticket.assignee?.username || '-'}</p>
            </div>
            <div>
              <span className="text-tertiary text-xs">{t('common.createdAt')}</span>
              <p className="font-number">{formatDateTime(ticket.created_at)}</p>
            </div>
          </div>

          {/* 工单内容 */}
          <div className="pt-3 border-t border-gray-200 dark:border-gray-700">
            <p className="text-sm whitespace-pre-wrap">{ticket.content}</p>
          </div>
        </div>

        {/* 回复列表 */}
        {ticket.replies && ticket.replies.length > 0 && (
          <div className="space-y-3">
            {ticket.replies.map((reply) => (
              <div
                key={reply.id}
                className={`bg-white dark:bg-gray-800 rounded-xl border p-4 ${
                  reply.is_staff ? 'border-l-4 border-l-blue-500 border-gray-200 dark:border-gray-700' : 'border-gray-200 dark:border-gray-700'
                }`}
              >
                <div className="flex items-center gap-2 mb-2">
                  <span className="text-sm font-medium">
                    {reply.user?.username ?? `User#${reply.user_id}`}
                  </span>
                  {reply.is_staff && (
                    <span className="text-xs px-1.5 py-0.5 rounded bg-blue-500/10 text-blue-500">
                      {t('ticket.staff')}
                    </span>
                  )}
                  <span className="text-xs text-tertiary font-number">{formatDateTime(reply.created_at)}</span>
                </div>
                <p className="text-sm whitespace-pre-wrap">{reply.content}</p>
              </div>
            ))}
          </div>
        )}

        {/* 回复输入 */}
        {!isClosed && (
          <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 space-y-3">
            <textarea
              className="w-full min-h-[80px] resize-y rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/20"
              value={replyContent}
              onChange={(e) => setReplyContent(e.target.value)}
              placeholder={t('ticket.replyPlaceholder')}
            />
            <div className="flex justify-end">
              <button
                className="inline-flex items-center justify-center rounded-lg px-4 py-2 text-sm font-medium bg-black text-white hover:opacity-90 disabled:opacity-50"
                onClick={handleReply}
                disabled={replying || !replyContent.trim()}
              >
                {replying ? (
                  <div className="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
                ) : (
                  <>
                    <Send size={14} className="mr-1" />
                    {t('ticket.reply')}
                  </>
                )}
              </button>
            </div>
          </div>
        )}

        {isClosed && (
          <div className="text-center py-8 text-sm text-tertiary">
            {t('ticket.closedNotice')}
          </div>
        )}
      </div>
    </div>
  )
}
