import { useState, useRef, useEffect, useCallback } from 'react'
import { createPortal } from 'react-dom'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { Plus, Play, Square, RotateCw, Terminal, Ban, Monitor } from 'lucide-react'
import apiClient from '@/api/client'
import { DataTable, type Column } from '@/components/DataTable/DataTable'
import { Button } from '@/components/Button/Button'
import { useToastStore } from '@/stores/toast'
import { PageLayout } from '@/components/PageLayout/PageLayout'
import { SearchInput } from '@/components/SearchInput/SearchInput'
import { FilterBar, type FilterField } from '@/components/FilterBar/FilterBar'
import { useListQuery } from '@/hooks/useListQuery'
import { useWebSocket } from '@/hooks/useWebSocket'
import { getStatusLabel } from '@/utils/format'
import CreateInstanceModal from './CreateInstanceModal'
import { EditInstancePanel } from '@/components/InstancesPage/EditInstancePanel'
import '@/components/PageTransition/PageTransition.css'

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let val = bytes
  let idx = 0
  while (val >= 1024 && idx < units.length - 1) {
    val /= 1024
    idx++
  }
  return `${val.toFixed(idx === 0 ? 0 : 1)} ${units[idx]}`
}

function formatNetSpeed(bytesPerSec: number): string {
  if (!bytesPerSec || bytesPerSec <= 0) return '0 B/s'
  const units = ['B/s', 'KB/s', 'MB/s', 'GB/s']
  let val = bytesPerSec
  let idx = 0
  while (val >= 1024 && idx < units.length - 1) {
    val /= 1024
    idx++
  }
  return `${val.toFixed(1)} ${units[idx]}`
}

function formatDateTime(dateStr?: string): string {
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

export interface InstanceMetrics {
  cpu_usage?: number
  memory_usage?: number
  memory_total?: number
  disk_used?: number
  disk_total?: number
  disk_read_bps?: number
  disk_write_bps?: number
  disk_read_iops?: number
  disk_write_iops?: number
  network_rx?: number
  network_tx?: number
}

export interface Instance {
  id: string
  name: string
  type: string
  status: string
  node_id: string
  node_name?: string
  user_id: number
  owner_name?: string
  incus_name: string
  vcpu: number
  memory_mb: number
  swap_mb: number
  disk_mb: number
  internal_ipv4?: string
  internal_ipv6?: string
  ipv4_eip?: string
  ipv6_eip?: string
  ssh_port?: number
  bridge_id?: string
  network_down?: number
  network_up?: number
  io_read_iops?: number
  io_write_iops?: number
  monthly_traffic?: number
  traffic_used_gb?: number
  traffic_mode?: string
  over_limit_action?: string
  throttle_mbps?: number
  is_over_limit?: boolean
  snapshot_limit?: number
  port_mapping_limit?: number
  expires_at?: string
  created_at: string
  has_eip?: boolean
  metrics?: InstanceMetrics
  data_disks?: DataDisk[]
  port_mappings?: PortMapping[]
  ipv4_eip_allocation_id?: string
  ipv6_eip_allocation_id?: string
  bridge_name?: string
  ipv4_mode?: string
  ipv6_mode?: string
}

export interface DataDisk {
  id: string
  name: string
  size_mb: number
  storage_pool: string
  mount_point?: string
  status: string
}

export interface PortMapping {
  id: string
  protocol: string
  external_port: number
  internal_port: number
}

interface Props {
  instanceType?: 'vm' | 'container'
}

export default function InstancesPage({ instanceType }: Props) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const toast = useToastStore()
  const [modalOpen, setModalOpen] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const [editInstance, setEditInstance] = useState<Instance | null>(null)
  const [actionMenuId, setActionMenuId] = useState<string | null>(null)
  const [actionMenuPos, setActionMenuPos] = useState<{ top: number; right: number } | null>(null)
  const actionMenuRef = useRef<HTMLDivElement>(null)
  const actionMenuPortalRef = useRef<HTMLDivElement>(null)
  const [instancesState, setInstancesState] = useState<Instance[]>([])

  const typeFilter = instanceType === 'vm'
    ? 'vm'
    : instanceType === 'container'
      ? 'container'
      : undefined

  const { data: instances, total, loading, page, perPage, search, filters, setPage, setPerPage, setSearch, setFilter, refresh } = useListQuery<Instance>(
    '/instances',
    typeFilter ? { type: typeFilter } : {},
    {
      wsUrl: '/ws/instances',
      wsType: 'instance_status',
      wsUpdate: (prev: any[], msg: any) => {
        if (msg.status === 'deleted') {
          return prev.filter((inst: any) => inst.id !== msg.instance_id)
        }
        return prev.map((inst: any) =>
          inst.id === msg.instance_id
            ? { ...inst, status: msg.status }
            : inst
        )
      },
      wsRefreshTypes: ['data_refresh'],
    }
  )

  // 同步 instances 到本地 state
  useEffect(() => {
    setInstancesState(instances)
  }, [instances])

  // 监听 instance_metrics WebSocket 消息，实时更新指标
  const handleWsMessage = useCallback((msg: any) => {
    if (msg.type === 'instance_metrics' && msg.instance_id && msg.data) {
      setInstancesState((prev) =>
        prev.map((inst) =>
          inst.id === msg.instance_id
            ? { ...inst, metrics: msg.data as InstanceMetrics }
            : inst
        )
      )
    }
  }, [])

  useWebSocket({
    url: '/ws/instances',
    onMessage: handleWsMessage,
    enabled: true,
  })

  // 点击外部关闭操作菜单
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      const target = e.target as Node
      if (actionMenuRef.current && actionMenuRef.current.contains(target)) return
      if (actionMenuPortalRef.current && actionMenuPortalRef.current.contains(target)) return
      setActionMenuId(null)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  // 打开操作菜单时计算位置
  const openActionMenu = (e: React.MouseEvent, rowId: string) => {
    if (actionMenuId === rowId) {
      setActionMenuId(null)
      return
    }
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
    setActionMenuPos({ top: rect.bottom + 4, right: window.innerWidth - rect.right })
    setActionMenuId(rowId)
  }

  const handleAction = async (id: string, action: string) => {
    setActionMenuId(null)
    try {
      if (action === 'terminal') {
        const res = await apiClient.get(`/instances/${id}/console?type=ssh`)
        if (res.data.token) {
          window.open(`/console?token=${res.data.token}`, '_blank')
        }
      } else if (action === 'vnc') {
        const res = await apiClient.get(`/instances/${id}/console?type=vnc`)
        if (res.data.token) {
          window.open(`/vnc?token=${res.data.token}`, '_blank')
        }
      } else if (action === 'ban') {
        await apiClient.post(`/instances/${id}/ban`)
        toast.success('封禁已下发')
        refresh()
      } else if (action === 'unban') {
        await apiClient.post(`/instances/${id}/unban`)
        toast.success('解封已下发')
        refresh()
      } else {
        await apiClient.post(`/instances/${id}/${action}`)
        toast.success(`操作 ${action} 已下发`)
        refresh()
      }
    } catch (err: any) {
      toast.error(err.response?.data?.error || '操作失败')
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('确认删除该实例？')) return
    try {
      await apiClient.delete(`/instances/${id}`)
      toast.success('删除任务已下发')
      refresh()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '删除任务下发失败')
    }
  }

  const handleEdit = async (row: Instance) => {
    try {
      const res = await apiClient.get(`/instances/${row.id}`)
      setEditInstance(res.data as Instance)
      setEditOpen(true)
    } catch (err: any) {
      toast.error(err.response?.data?.error || '获取实例详情失败')
    }
  }

  const getProgressColor = (percent: number): string => {
    if (percent >= 90) return 'metric-progress__fill--red'
    if (percent >= 70) return 'metric-progress__fill--yellow'
    return 'metric-progress__fill--green'
  }

  const getStatusTagClass = (status: string) => {
    switch (status) {
      case 'running': return 'data-table-tag data-table-tag--online'
      case 'stopped': return 'data-table-tag data-table-tag--disabled'
      case 'creating': return 'data-table-tag'
      case 'error': return 'data-table-tag data-table-tag--offline'
      case 'banned': return 'data-table-tag data-table-tag--offline'
      case 'expired': return 'data-table-tag data-table-tag--disabled'
      case 'offline': return 'data-table-tag data-table-tag--offline'
      case 'missing': return 'data-table-tag data-table-tag--offline'
      default: return 'data-table-tag'
    }
  }

  const columns: Column<Instance>[] = [
    {
      key: 'name',
      title: '名称',
      width: 140,
      render: (row) => (
        <button className="data-table-link-btn" onClick={() => navigate(`/admin/instanceManagement/instances/${row.id}`)}>
          {row.name}
        </button>
      ),
    },
    {
      key: 'type',
      title: '类型',
      width: 70,
      render: (row) => (
        <span className="data-table-tag">{row.type === 'vm' ? 'VM' : '容器'}</span>
      ),
    },
    {
      key: 'status',
      title: '状态',
      width: 80,
      render: (row) => (
        <span className={getStatusTagClass(row.status)}>{getStatusLabel(row.status, t)}</span>
      ),
    },
    {
      key: 'node_name',
      title: '宿主机',
      width: 120,
      render: (row) => (
        <span title={row.node_id}>{row.node_name}</span>
      ),
    },
    {
      key: 'owner_name',
      title: '用户',
      width: 100,
      render: (row) => (
        <span className="text-sm text-secondary">{row.owner_name || row.user_id}</span>
      ),
    },
    {
      key: 'eip',
      title: 'EIP',
      width: 160,
      render: (row) => (
        <div className="ip-cell">
          {row.ipv4_eip ? (
            <div className="ip-cell__line">
              <span className="font-number">{row.ipv4_eip}</span>
            </div>
          ) : null}
          {row.ipv6_eip ? (
            <div className="ip-cell__line">
              <span className="font-number">{row.ipv6_eip}</span>
            </div>
          ) : null}
          {!row.ipv4_eip && !row.ipv6_eip ? (
            <span className="font-number" style={{ color: 'var(--color-gray-400)' }}>{row.internal_ipv4 || '-'}</span>
          ) : null}
        </div>
      ),
    },
    {
      key: 'cpu',
      title: 'CPU',
      width: 160,
      render: (row) => {
        const usage = row.metrics?.cpu_usage ?? 0
        const total = row.vcpu ?? 0
        return (
          <div className="metric-cell">
            <div className="metric-cell__header">
              <span className="metric-cell__percent font-number">{usage.toFixed(1)}%</span>
              <span className="metric-cell__detail font-number">/ {total}核</span>
            </div>
            <div className="metric-progress">
              <div className={`metric-progress__fill ${getProgressColor(usage)}`} style={{ width: `${Math.min(usage, 100)}%` }} />
            </div>
          </div>
        )
      },
    },
    {
      key: 'memory',
      title: '内存',
      width: 160,
      render: (row) => {
        const used = row.metrics?.memory_usage ?? 0
        const total = row.metrics?.memory_total ?? row.memory_mb * 1024 * 1024
        const percent = total > 0 ? (used / total) * 100 : 0
        return (
          <div className="metric-cell">
            <div className="metric-cell__header">
              <span className="metric-cell__percent font-number">{percent.toFixed(1)}%</span>
              <span className="metric-cell__detail font-number">{formatBytes(used)} / {formatBytes(total)}</span>
            </div>
            <div className="metric-progress">
              <div className={`metric-progress__fill ${getProgressColor(percent)}`} style={{ width: `${Math.min(percent, 100)}%` }} />
            </div>
          </div>
        )
      },
    },
    {
      key: 'disk',
      title: '磁盘',
      width: 160,
      render: (row) => {
        const used = row.metrics?.disk_used ?? 0
        const total = row.metrics?.disk_total ?? row.disk_mb * 1024 * 1024
        const percent = total > 0 ? (used / total) * 100 : 0
        return (
          <div className="metric-cell">
            <div className="metric-cell__header">
              <span className="metric-cell__percent font-number">{percent.toFixed(1)}%</span>
              <span className="metric-cell__detail font-number">{formatBytes(used)} / {formatBytes(total)}</span>
            </div>
            <div className="metric-progress">
              <div className={`metric-progress__fill ${getProgressColor(percent)}`} style={{ width: `${Math.min(percent, 100)}%` }} />
            </div>
          </div>
        )
      },
    },
    {
      key: 'traffic',
      title: '流量',
      width: 160,
      render: (row) => {
        const used = row.traffic_used_gb ?? 0
        const total = row.monthly_traffic ?? 0
        if (!total || total <= 0) {
          return <span className="text-sm" style={{ color: 'var(--color-gray-400)' }}>不限</span>
        }
        const percent = (used / total) * 100
        return (
          <div className="metric-cell">
            <div className="metric-cell__header">
              <span className="metric-cell__percent font-number">{percent.toFixed(1)}%</span>
              <span className="metric-cell__detail font-number">{used.toFixed(1)} / {total.toFixed(0)} GB</span>
            </div>
            <div className="metric-progress">
              <div className={`metric-progress__fill ${getProgressColor(percent)}`} style={{ width: `${Math.min(percent, 100)}%` }} />
            </div>
          </div>
        )
      },
    },
    {
      key: 'net_io',
      title: '网络IO',
      width: 120,
      render: (row) => {
        const rx = row.metrics?.network_rx ?? 0
        const tx = row.metrics?.network_tx ?? 0
        return (
          <div className="net-cell font-number">
            <span className="net-cell__up">↑{formatNetSpeed(tx)}</span>
            <span className="net-cell__down">↓{formatNetSpeed(rx)}</span>
          </div>
        )
      },
    },
    {
      key: 'expires_at',
      title: '到期时间',
      width: 160,
      render: (row) => (
        <span className="text-sm text-tertiary font-number">{formatDateTime(row.expires_at)}</span>
      ),
    },
    {
      key: 'actions',
      title: '操作',
      width: 160,
      render: (row: Instance) => (
        <div className="flex items-center gap-3 relative">
          <button className="data-table-link-btn" onClick={() => handleEdit(row)}>编辑</button>
          <div className="relative" ref={actionMenuId === row.id ? actionMenuRef : null}>
            <button
              className="data-table-link-btn"
              onClick={(e) => openActionMenu(e, row.id)}
            >
              操作
            </button>
            {actionMenuId === row.id && actionMenuPos && createPortal(
              <div ref={actionMenuPortalRef} className="action-menu fixed z-[99999] bg-surface border border-surface rounded-lg shadow-lg py-1 min-w-[140px]" style={{ top: actionMenuPos.top, right: actionMenuPos.right }}>
                {row.status === 'stopped' && (
                  <button className="w-full text-left px-3 py-1.5 text-xs hover:bg-surface-secondary flex items-center gap-2" onClick={() => handleAction(row.id, 'start')}>
                    <Play size={12} /> 启动
                  </button>
                )}
                {row.status === 'running' && (
                  <button className="w-full text-left px-3 py-1.5 text-xs hover:bg-surface-secondary flex items-center gap-2" onClick={() => handleAction(row.id, 'stop')}>
                    <Square size={12} /> 停止
                  </button>
                )}
                {row.status !== 'offline' && (
                  <button className="w-full text-left px-3 py-1.5 text-xs hover:bg-surface-secondary flex items-center gap-2" onClick={() => handleAction(row.id, 'restart')}>
                    <RotateCw size={12} /> 重启
                  </button>
                )}
                {row.status !== 'offline' && (
                  <button className="w-full text-left px-3 py-1.5 text-xs hover:bg-surface-secondary flex items-center gap-2" onClick={() => handleAction(row.id, 'terminal')}>
                    <Terminal size={12} /> 终端
                  </button>
                )}
                {row.status !== 'offline' && row.type === 'vm' && (
                  <button className="w-full text-left px-3 py-1.5 text-xs hover:bg-surface-secondary flex items-center gap-2" onClick={() => handleAction(row.id, 'vnc')}>
                    <Monitor size={12} /> VNC
                  </button>
                )}
                {row.status !== 'banned' && row.status !== 'offline' ? (
                  <button className="w-full text-left px-3 py-1.5 text-xs hover:bg-surface-secondary text-red-600 flex items-center gap-2" onClick={() => handleAction(row.id, 'ban')}>
                    <Ban size={12} /> 封禁
                  </button>
                ) : row.status === 'banned' ? (
                  <button className="w-full text-left px-3 py-1.5 text-xs hover:bg-surface-secondary text-green-600 flex items-center gap-2" onClick={() => handleAction(row.id, 'unban')}>
                    <Ban size={12} /> 解封
                  </button>
                ) : null}
              </div>,
              document.body
            )}
          </div>
          <button className="data-table-link-btn" onClick={() => handleDelete(row.id)} style={{ color: 'var(--color-red-500)' }}>删除</button>
        </div>
      ),
    },
  ]

  const statusOptions = [
    { label: t('common.running'), value: 'running' },
    { label: t('common.stopped'), value: 'stopped' },
    { label: t('common.creating'), value: 'creating' },
    { label: t('common.error'), value: 'error' },
    { label: t('common.banned'), value: 'banned' },
    { label: t('common.expired'), value: 'expired' },
    { label: t('common.nodeOffline'), value: 'offline' },
    { label: t('common.missing'), value: 'missing' },
  ]

  return (
    <PageLayout
      leftSlot={
        <>
          <SearchInput value={search} placeholder="搜索实例名称" onChange={setSearch} />
          <FilterBar
            fields={[
              { key: 'status', label: '状态', options: statusOptions },
            ] as FilterField[]}
            values={filters}
            onChange={setFilter}
          />
        </>
      }
      rightSlot={
        <Button icon={<Plus size={16} />} onClick={() => setModalOpen(true)}>
          {t('instance.createInstance')}
        </Button>
      }
    >
      <div className="page-transition__content" style={{ flex: 1, overflow: 'auto' }}>
        <DataTable
          columns={columns}
          data={instancesState}
          rowKey={(r) => r.id}
          loading={loading}
          emptyText="暂无实例"
          pagination={{ page, size: perPage, total }}
          onPageChange={setPage}
          onSizeChange={setPerPage}
        />
      </div>
      <CreateInstanceModal open={modalOpen} onClose={() => setModalOpen(false)} onSuccess={refresh} />
      <EditInstancePanel
        open={editOpen}
        instance={editInstance}
        onClose={() => { setEditOpen(false); setEditInstance(null) }}
        onSuccess={refresh}
        onInstanceUpdate={setEditInstance}
      />
    </PageLayout>
  )
}
