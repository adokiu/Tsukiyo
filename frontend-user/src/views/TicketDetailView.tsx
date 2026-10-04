import { useState, useEffect, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { ticketsApi, type Ticket } from '@/api/tickets'
import { Loader2, ArrowLeft, Send, XCircle } from 'lucide-react'
import '@/views/ProductsView.css'

function formatTime(time: string): string {
  const d = new Date(time)
  return d.toLocaleString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

const statusColors: Record<string, string> = {
  open: 'text-blue-500',
  in_progress: 'text-yellow-500',
  waiting: 'text-orange-500',
  resolved: 'text-green-500',
  closed: 'text-gray-500',
}

export function TicketDetailView() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { t } = useTranslation()
  const [ticket, setTicket] = useState<Ticket | null>(null)
  const [loading, setLoading] = useState(true)
  const [replyContent, setReplyContent] = useState('')
  const [replying, setReplying] = useState(false)
  const [closing, setClosing] = useState(false)

  const fetchTicket = useCallback(async () => {
    if (!id) return
    setLoading(true)
    try {
      const res = await ticketsApi.get(id)
      setTicket(res.data)
    } catch {
      setTicket(null)
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => {
    fetchTicket()
  }, [fetchTicket])

  const handleReply = async () => {
    if (!id || !replyContent.trim()) return
    setReplying(true)
    try {
      await ticketsApi.reply(id, replyContent.trim())
      setReplyContent('')
      await fetchTicket()
    } catch {
    } finally {
      setReplying(false)
    }
  }

  const handleClose = async () => {
    if (!id) return
    setClosing(true)
    try {
      await ticketsApi.close(id)
      await fetchTicket()
    } catch {
    } finally {
      setClosing(false)
    }
  }

  if (loading) {
    return (
      <main className="flex-1 overflow-y-auto p-4 sm:p-6 md:p-8">
        <div className="flex justify-center py-20">
          <Loader2 className="animate-spin text-muted-foreground" size={28} />
        </div>
      </main>
    )
  }

  if (!ticket) {
    return (
      <main className="flex-1 overflow-y-auto p-4 sm:p-6 md:p-8">
        <div className="mx-auto max-w-4xl">
          <p className="text-center text-muted-foreground py-20">{t('ticket.notFound')}</p>
        </div>
      </main>
    )
  }

  const isClosed = ticket.status === 'closed'

  return (
    <main className="flex-1 overflow-y-auto overflow-x-hidden p-4 sm:p-6 md:p-8">
      <div className="mx-auto w-full max-w-4xl">
        <div className="space-y-6 animate-in fade-in slide-in-from-bottom-4 duration-700">

          {/* 返回按钮 */}
          <button
            className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors"
            onClick={() => navigate('/tickets')}
          >
            <ArrowLeft size={16} />
            {t('ticket.back')}
          </button>

          {/* 工单信息 */}
          <div className="bg-card border border-border rounded-xl p-6 space-y-4">
            <div className="flex items-start justify-between gap-4">
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 mb-2">
                  <span className="text-xs font-mono text-muted-foreground">{ticket.ticket_no}</span>
                  <span className={`text-xs font-medium ${statusColors[ticket.status] ?? ''}`}>
                    {t(`ticket.status.${ticket.status}`)}
                  </span>
                </div>
                <h2 className="text-xl font-medium">{ticket.subject}</h2>
              </div>
              {!isClosed && (
                <button
                  className="apple-button-secondary text-sm"
                  onClick={handleClose}
                  disabled={closing}
                >
                  {closing ? <Loader2 size={14} className="animate-spin" /> : (
                    <>
                      <XCircle size={14} className="mr-1" />
                      {t('ticket.close')}
                    </>
                  )}
                </button>
              )}
            </div>

            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-sm">
              <div>
                <span className="text-muted-foreground text-xs">{t('ticket.typeLabel')}</span>
                <p>{t(`ticket.type.${ticket.type}`)}</p>
              </div>
              <div>
                <span className="text-muted-foreground text-xs">{t('ticket.priorityLabel')}</span>
                <p>{t(`ticket.priority.${ticket.priority}`)}</p>
              </div>
              {ticket.product && (
                <div>
                  <span className="text-muted-foreground text-xs">{t('ticket.productLabel')}</span>
                  <p>{ticket.product.name}</p>
                </div>
              )}
              {ticket.instance && (
                <div>
                  <span className="text-muted-foreground text-xs">{t('ticket.instanceLabel')}</span>
                  <p>{ticket.instance.name}</p>
                </div>
              )}
              <div>
                <span className="text-muted-foreground text-xs">{t('ticket.createdTime')}</span>
                <p>{formatTime(ticket.created_at)}</p>
              </div>
            </div>

            <div className="pt-3 border-t border-border">
              <p className="text-sm whitespace-pre-wrap">{ticket.content}</p>
            </div>
          </div>

          {/* 回复列表 */}
          {ticket.replies && ticket.replies.length > 0 && (
            <div className="space-y-3">
              {ticket.replies.map((reply) => (
                <div
                  key={reply.id}
                  className={`bg-card border border-border rounded-xl p-4 ${reply.is_staff ? 'border-l-4 border-l-blue-500' : ''}`}
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
                    <span className="text-xs text-muted-foreground">{formatTime(reply.created_at)}</span>
                  </div>
                  <p className="text-sm whitespace-pre-wrap">{reply.content}</p>
                </div>
              ))}
            </div>
          )}

          {/* 回复输入 */}
          {!isClosed && (
            <div className="bg-card border border-border rounded-xl p-4 space-y-3">
              <textarea
                className="apple-input w-full min-h-[80px] resize-y"
                value={replyContent}
                onChange={(e) => setReplyContent(e.target.value)}
                placeholder={t('ticket.replyPlaceholder')}
              />
              <div className="flex justify-end">
                <button
                  className="apple-button"
                  onClick={handleReply}
                  disabled={replying || !replyContent.trim()}
                >
                  {replying ? <Loader2 size={16} className="animate-spin" /> : (
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
            <div className="text-center py-8 text-sm text-muted-foreground">
              {t('ticket.closedNotice')}
            </div>
          )}

        </div>
      </div>
    </main>
  )
}
