import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import apiClient from '@/api/client'
import { DataTable, type Column } from '@/components/DataTable/DataTable'
import { PageLayout } from '@/components/PageLayout/PageLayout'
import { FilterBar, type FilterField } from '@/components/FilterBar/FilterBar'

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
  return `${Y}/${M}/${D} ${h}:${m}`
}

export default function TicketsPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [tickets, setTickets] = useState<Ticket[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [page, setPage] = useState(1)
  const [perPage, setPerPage] = useState(20)
  const [filters, setFilters] = useState<Record<string, string>>({})

  const fetchTickets = () => {
    setLoading(true)
    const params: Record<string, any> = { page, page_size: perPage }
    if (filters.status) params.status = filters.status
    if (filters.type) params.type = filters.type
    if (filters.priority) params.priority = filters.priority
    apiClient.get('/tickets', { params }).then((res) => {
      setTickets(res.data.data || [])
      setTotal(res.data.pagination?.total || 0)
    }).finally(() => setLoading(false))
  }

  useEffect(() => { fetchTickets() }, [page, perPage, filters])

  const getTypeLabel = (type: string) => {
    const map: Record<string, string> = {
      fault: t('ticket.type.fault'),
      finance: t('ticket.type.finance'),
      sales: t('ticket.type.sales'),
      tech: t('ticket.type.tech'),
      complaint: t('ticket.type.complaint'),
      other: t('ticket.type.other'),
    }
    return map[type] || type
  }

  const getStatusLabel = (status: string) => {
    const map: Record<string, string> = {
      open: t('ticket.status.open'),
      in_progress: t('ticket.status.inProgress'),
      waiting: t('ticket.status.waiting'),
      resolved: t('ticket.status.resolved'),
      closed: t('ticket.status.closed'),
    }
    return map[status] || status
  }

  const getStatusTagClass = (status: string) => {
    switch (status) {
      case 'open': return 'data-table-tag data-table-tag--online'
      case 'in_progress': return 'data-table-tag'
      case 'waiting': return 'data-table-tag data-table-tag--offline'
      case 'resolved': return 'data-table-tag data-table-tag--online'
      case 'closed': return 'data-table-tag data-table-tag--disabled'
      default: return 'data-table-tag'
    }
  }

  const getPriorityLabel = (priority: string) => {
    const map: Record<string, string> = {
      low: t('ticket.priority.low'),
      normal: t('ticket.priority.normal'),
      high: t('ticket.priority.high'),
      urgent: t('ticket.priority.urgent'),
    }
    return map[priority] || priority
  }

  const getPriorityTagClass = (priority: string) => {
    switch (priority) {
      case 'low': return 'data-table-tag data-table-tag--disabled'
      case 'normal': return 'data-table-tag'
      case 'high': return 'data-table-tag data-table-tag--offline'
      case 'urgent': return 'data-table-tag data-table-tag--offline'
      default: return 'data-table-tag'
    }
  }

  const columns: Column<Ticket>[] = [
    {
      key: 'ticket_no',
      title: t('ticket.ticketNo'),
      width: 160,
      render: (row) => (
        <span className="text-sm text-primary font-number cursor-pointer" onClick={() => navigate(`/admin/ticketManagement/tickets/${row.id}`)}>
          {row.ticket_no}
        </span>
      ),
    },
    {
      key: 'subject',
      title: t('ticket.subject'),
      width: 240,
      render: (row) => (
        <span className="text-sm text-primary cursor-pointer" onClick={() => navigate(`/admin/ticketManagement/tickets/${row.id}`)}>
          {row.subject}
        </span>
      ),
    },
    {
      key: 'user',
      title: t('common.username'),
      width: 120,
      render: (row) => (
        <span className="text-sm text-secondary">{row.user?.username || `#${row.user_id}`}</span>
      ),
    },
    {
      key: 'type',
      title: t('ticket.typeLabel'),
      width: 100,
      render: (row) => (
        <span className="data-table-tag">{getTypeLabel(row.type)}</span>
      ),
    },
    {
      key: 'priority',
      title: t('ticket.priorityLabel'),
      width: 80,
      render: (row) => (
        <span className={getPriorityTagClass(row.priority)}>{getPriorityLabel(row.priority)}</span>
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
      key: 'assignee',
      title: t('ticket.assignee'),
      width: 100,
      render: (row) => (
        <span className="text-sm text-tertiary">{row.assignee?.username || '-'}</span>
      ),
    },
    {
      key: 'created_at',
      title: t('common.createdAt'),
      width: 140,
      render: (row) => (
        <span className="text-sm text-tertiary font-number">{formatDateTime(row.created_at)}</span>
      ),
    },
  ]

  const statusOptions = [
    { label: t('ticket.status.open'), value: 'open' },
    { label: t('ticket.status.inProgress'), value: 'in_progress' },
    { label: t('ticket.status.waiting'), value: 'waiting' },
    { label: t('ticket.status.resolved'), value: 'resolved' },
    { label: t('ticket.status.closed'), value: 'closed' },
  ]

  const typeOptions = [
    { label: t('ticket.type.fault'), value: 'fault' },
    { label: t('ticket.type.finance'), value: 'finance' },
    { label: t('ticket.type.sales'), value: 'sales' },
    { label: t('ticket.type.tech'), value: 'tech' },
    { label: t('ticket.type.complaint'), value: 'complaint' },
    { label: t('ticket.type.other'), value: 'other' },
  ]

  const priorityOptions = [
    { label: t('ticket.priority.low'), value: 'low' },
    { label: t('ticket.priority.normal'), value: 'normal' },
    { label: t('ticket.priority.high'), value: 'high' },
    { label: t('ticket.priority.urgent'), value: 'urgent' },
  ]

  return (
    <PageLayout
      leftSlot={
        <FilterBar
          fields={[
            { key: 'status', label: t('common.status'), options: statusOptions },
            { key: 'type', label: t('ticket.typeLabel'), options: typeOptions },
            { key: 'priority', label: t('ticket.priorityLabel'), options: priorityOptions },
          ] as FilterField[]}
          values={filters}
          onChange={(key, value) => {
            setFilters((prev) => ({ ...prev, [key]: value }))
            setPage(1)
          }}
        />
      }
    >
      <div className="page-transition__content" style={{ flex: 1, overflow: 'auto' }}>
        <DataTable
          columns={columns}
          data={tickets}
          rowKey={(r) => r.id}
          loading={loading}
          emptyText={t('common.noData')}
          pagination={{ page, size: perPage, total }}
          onPageChange={setPage}
          onSizeChange={setPerPage}
        />
      </div>
    </PageLayout>
  )
}
