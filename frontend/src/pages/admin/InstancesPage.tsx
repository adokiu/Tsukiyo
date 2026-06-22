import { useState, useRef, useEffect, useCallback, useLayoutEffect } from 'react'
import { createPortal } from 'react-dom'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { Plus, Play, Square, RotateCw, Terminal, Ban, Monitor } from 'lucide-react'
import apiClient from '@/api/client'
import { DataTable, type Column } from '@/components/DataTable/DataTable'
import { Button } from '@/components/Button/Button'
import { Select } from '@/components/Select/Select'
import { SlidePanel } from '@/components/SlidePanel/SlidePanel'
import { useToastStore } from '@/stores/toast'
import { PageLayout } from '@/components/PageLayout/PageLayout'
import { SearchInput } from '@/components/SearchInput/SearchInput'
import { FilterBar, type FilterField } from '@/components/FilterBar/FilterBar'
import { useListQuery } from '@/hooks/useListQuery'
import { useWebSocket } from '@/hooks/useWebSocket'
import CreateInstanceModal from './CreateInstanceModal'
import '@/components/PageTransition/PageTransition.css'

interface InstanceMetrics {
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

interface Instance {
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

interface DataDisk {
  id: string
  name: string
  size_mb: number
  storage_pool: string
  mount_point?: string
  status: string
}

interface PortMapping {
  id: string
  protocol: string
  external_port: number
  internal_port: number
}

interface Props {
  instanceType?: 'vm' | 'container'
}

const STATUS_LABEL_KEYS: Record<string, string> = {
  running: 'common.running',
  stopped: 'common.stopped',
  creating: 'common.creating',
  error: 'common.error',
  banned: 'common.banned',
  expired: 'common.expired',
  starting: 'common.starting',
  stopping: 'common.stopping',
  deleting: 'common.deleting',
  offline: 'common.nodeOffline',
  missing: 'common.missing',
  restarting: 'common.restarting',
  reinstalling: 'common.reinstalling',
  resizing: 'common.resizing',
}

function getStatusLabel(status: string, t: (key: string) => string): string {
  const key = STATUS_LABEL_KEYS[status]
  return key ? t(key) : status
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

  const formatBytes = (bytes: number): string => {
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

  const formatNetSpeed = (bytesPerSec: number): string => {
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

  const formatDateTime = (dateStr?: string): string => {
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

// 编辑实例 SlidePanel
function EditInstancePanel({ open, instance, onClose, onSuccess, onInstanceUpdate }: {
  open: boolean
  instance: Instance | null
  onClose: () => void
  onSuccess: () => void
  onInstanceUpdate: (inst: Instance) => void
}) {
  const { t } = useTranslation()
  const toast = useToastStore()
  const [saving, setSaving] = useState(false)
  const [vcpu, setVcpu] = useState(0)
  const [memoryMb, setMemoryMb] = useState(0)
  const [swapMb, setSwapMb] = useState(0)
  const [expiresAt, setExpiresAt] = useState('')
  const [networkDown, setNetworkDown] = useState(0)
  const [networkUp, setNetworkUp] = useState(0)
  const [monthlyTraffic, setMonthlyTraffic] = useState(0)
  const [overLimitAction, setOverLimitAction] = useState('shutdown')
  const [throttleMbps, setThrottleMbps] = useState(1)
  const [portMappingLimit, setPortMappingLimit] = useState(2)
  const [snapshotLimit, setSnapshotLimit] = useState(3)
  const [ioReadIops, setIoReadIops] = useState(0)
  const [ioWriteIops, setIoWriteIops] = useState(0)
  const [statusOverride, setStatusOverride] = useState('')
  const [eipPools, setEipPools] = useState<any[]>([])
  const [eipv4PoolId, setEipv4PoolId] = useState('')
  const [eipv4AddrList, setEipv4AddrList] = useState<string[]>([])
  const [eipv4SelectedIP, setEipv4SelectedIP] = useState('')
  const [eipv6AddrList, setEipv6AddrList] = useState<string[]>([])
  const [eipv6SelectedIP, setEipv6SelectedIP] = useState('')
  const [eipv6PrefixLen, setEipv6PrefixLen] = useState(128)
  const [newDiskName, setNewDiskName] = useState('')
  const [newDiskSize, setNewDiskSize] = useState(10)
  const [newDiskPool, setNewDiskPool] = useState('default')
  const [newDiskMount, setNewDiskMount] = useState('')
  const [resizeDiskId, setResizeDiskId] = useState('')
  const [resizeDiskSize, setResizeDiskSize] = useState(0)
  const [sysDiskResize, setSysDiskResize] = useState(0)
  const [sysDiskUnit, setSysDiskUnit] = useState<'MB' | 'GB' | 'TB'>('GB')
  const [diskLoading, setDiskLoading] = useState(false)
  const [eipLoading, setEipLoading] = useState(false)

  useEffect(() => {
    if (instance) {
      setVcpu(instance.vcpu)
      setMemoryMb(instance.memory_mb)
      setSwapMb(instance.swap_mb || 0)
      setExpiresAt(instance.expires_at ? instance.expires_at.slice(0, 16) : '')
      setNetworkDown(instance.network_down || 0)
      setNetworkUp(instance.network_up || 0)
      setMonthlyTraffic(instance.monthly_traffic || 0)
      setOverLimitAction(instance.over_limit_action || 'shutdown')
      setThrottleMbps(instance.throttle_mbps || 1)
      setPortMappingLimit(instance.port_mapping_limit || 2)
      setSnapshotLimit(instance.snapshot_limit || 3)
      setIoReadIops(instance.io_read_iops || 0)
      setIoWriteIops(instance.io_write_iops || 0)
      setStatusOverride(instance.status)
      setSysDiskResize(Math.round(instance.disk_mb / 1024))
      setSysDiskUnit('GB')
      // 加载 EIP 池列表
      if (instance.node_id) {
        apiClient.get('/network/eip-pools', { params: { node_id: instance.node_id } }).then(res => {
          setEipPools((res.data.data || []).filter((p: any) => p.status === 'active' && p.pool_type === 'eip'))
        }).catch(() => setEipPools([]))
      }
      // 加载 IPv6 可用子段（从 bridge）
      if (instance.bridge_id) {
        apiClient.get('/network/bridge-ipv6-available', { params: { bridge_id: instance.bridge_id, prefix_len: eipv6PrefixLen, max_count: 10 } }).then(res => {
          setEipv6AddrList(res.data.addresses || [])
        }).catch(() => setEipv6AddrList([]))
      }
    }
  }, [instance])

  // IPv4 池变化时查询可用地址
  useEffect(() => {
    if (!eipv4PoolId) { setEipv4AddrList([]); return }
    apiClient.get('/network/eip-available-list', { params: { pool_id: eipv4PoolId, prefix_len: 32, max_count: 10 } }).then(res => {
      setEipv4AddrList(res.data.addresses || [])
    }).catch(() => setEipv4AddrList([]))
  }, [eipv4PoolId])

  // IPv6 前缀长度变化时重新查询
  useEffect(() => {
    if (!instance?.bridge_id) return
    apiClient.get('/network/bridge-ipv6-available', { params: { bridge_id: instance.bridge_id, prefix_len: eipv6PrefixLen, max_count: 10 } }).then(res => {
      setEipv6AddrList(res.data.addresses || [])
    }).catch(() => setEipv6AddrList([]))
  }, [eipv6PrefixLen, instance?.bridge_id])

  const handleSave = async () => {
    if (!instance) return
    setSaving(true)
    try {
      const payload: Record<string, any> = {
        vcpu,
        memory_mb: memoryMb,
        swap_mb: swapMb,
        network_down_mbps: networkDown,
        network_up_mbps: networkUp,
        monthly_traffic_gb: monthlyTraffic,
        over_limit_action: overLimitAction,
        throttle_mbps: throttleMbps,
        port_mapping_limit: portMappingLimit,
        snapshot_limit: snapshotLimit,
        io_read_iops: ioReadIops,
        io_write_iops: ioWriteIops,
      }
      if (expiresAt) {
        payload.expires_at = expiresAt
      }
      await apiClient.put(`/instances/${instance.id}`, payload)
      if (statusOverride !== instance.status) {
        await apiClient.post(`/instances/${instance.id}/status`, { status: statusOverride })
      }
      toast.success('实例配置已更新')
      onSuccess()
      onClose()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '更新失败')
    } finally {
      setSaving(false)
    }
  }

  const reloadInstance = async () => {
    const res = await apiClient.get(`/instances/${instance!.id}`)
    onInstanceUpdate(res.data as Instance)
  }

  // 绑定 IPv4 EIP: 先从池分配 EIP，再 assign 给实例
  const handleBindEIPv4 = async () => {
    if (!instance || !eipv4PoolId) return
    setEipLoading(true)
    try {
      const specificIP = eipv4SelectedIP || undefined
      const allocRes = await apiClient.post('/network/eip-allocations/allocate', {
        node_id: instance.node_id,
        ip_version: 'ipv4',
        prefix_len: 32,
        specific_ip: specificIP,
      })
      const allocId = allocRes.data.id
      await apiClient.post(`/network/eip-allocations/${allocId}/assign`, { instance_id: instance.id })
      toast.success('IPv4 EIP 绑定成功')
      setEipv4SelectedIP('')
      onSuccess()
      await reloadInstance()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '绑定失败')
    } finally {
      setEipLoading(false)
    }
  }

  // 绑定 IPv6 EIP: 先从 bridge 分配子段，再 assign 给实例
  const handleBindEIPv6 = async () => {
    if (!instance || !eipv6SelectedIP) return
    setEipLoading(true)
    try {
      // 从 bridge 分配 IPv6 子段
      const allocRes = await apiClient.post('/network/eip-allocations/allocate', {
        node_id: instance.node_id,
        ip_version: 'ipv6',
        prefix_len: eipv6PrefixLen,
        specific_ip: eipv6SelectedIP,
        bridge_id: instance.bridge_id,
      })
      const allocId = allocRes.data.id
      await apiClient.post(`/network/eip-allocations/${allocId}/assign`, { instance_id: instance.id })
      toast.success('IPv6 EIP 绑定成功')
      setEipv6SelectedIP('')
      onSuccess()
      await reloadInstance()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '绑定失败')
    } finally {
      setEipLoading(false)
    }
  }

  const handleUnbindEIP = async (allocId: string) => {
    if (!instance) return
    setEipLoading(true)
    try {
      await apiClient.post(`/network/eip-allocations/${allocId}/release`)
      toast.success('EIP 解绑成功')
      onSuccess()
      await reloadInstance()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '解绑失败')
    } finally {
      setEipLoading(false)
    }
  }

  // 系统盘扩容（通过 UpdateInstance API 的 disk_mb 字段）
  const handleResizeSysDisk = async () => {
    const targetMb = sysDiskUnit === 'MB' ? sysDiskResize : sysDiskUnit === 'GB' ? sysDiskResize * 1024 : sysDiskResize * 1024 * 1024
    if (!instance || targetMb <= instance.disk_mb) return
    setDiskLoading(true)
    try {
      await apiClient.put(`/instances/${instance.id}`, { disk_mb: targetMb })
      toast.success('系统盘扩容任务已创建')
      onSuccess()
      await reloadInstance()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '扩容失败')
    } finally {
      setDiskLoading(false)
    }
  }

  const handleAddDisk = async () => {
    if (!instance || !newDiskName.trim()) return
    setDiskLoading(true)
    try {
      await apiClient.post(`/instances/${instance.id}/disks`, {
        name: newDiskName.trim(),
        size_mb: newDiskSize * 1024,
        storage_pool: newDiskPool || undefined,
        mount_point: newDiskMount || undefined,
      })
      toast.success('添加数据盘任务已创建')
      setNewDiskName('')
      setNewDiskSize(10)
      setNewDiskMount('')
      onSuccess()
      const res = await apiClient.get(`/instances/${instance.id}`)
      onInstanceUpdate(res.data as Instance)
    } catch (err: any) {
      toast.error(err.response?.data?.error || '添加失败')
    } finally {
      setDiskLoading(false)
    }
  }

  const handleDeleteDisk = async (diskId: string) => {
    if (!instance) return
    if (!confirm('确认删除该数据盘？')) return
    setDiskLoading(true)
    try {
      await apiClient.delete(`/instances/${instance.id}/disks/${diskId}`)
      toast.success('删除数据盘任务已创建')
      onSuccess()
      const res = await apiClient.get(`/instances/${instance.id}`)
      onInstanceUpdate(res.data as Instance)
    } catch (err: any) {
      toast.error(err.response?.data?.error || '删除失败')
    } finally {
      setDiskLoading(false)
    }
  }

  const handleResizeDisk = async () => {
    if (!instance || !resizeDiskId || resizeDiskSize <= 0) return
    setDiskLoading(true)
    try {
      await apiClient.put(`/instances/${instance.id}/disks/${resizeDiskId}`, { size_mb: resizeDiskSize * 1024 })
      toast.success('扩容任务已创建')
      setResizeDiskId('')
      setResizeDiskSize(0)
      onSuccess()
      const res = await apiClient.get(`/instances/${instance.id}`)
      onInstanceUpdate(res.data as Instance)
    } catch (err: any) {
      toast.error(err.response?.data?.error || '扩容失败')
    } finally {
      setDiskLoading(false)
    }
  }

  if (!instance) return null

  return (
    <SlidePanel
      open={open}
      onClose={onClose}
      title={`编辑实例 - ${instance.name}`}
      width={520}
      footer={
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>取消</Button>
          <Button onClick={handleSave} loading={saving}>保存</Button>
        </div>
      }
    >
      <div className="space-y-5">
        {/* 基本信息 */}
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">基本信息</h4>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">CPU (核)</label>
              <input type="number" min={0.1} step={0.1} value={vcpu} onChange={(e) => setVcpu(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" />
            </div>
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">内存 (MB)</label>
              <input type="number" min={64} value={memoryMb} onChange={(e) => setMemoryMb(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" />
            </div>
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">Swap (MB)</label>
              <input type="number" min={0} step={128} value={swapMb} onChange={(e) => setSwapMb(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" placeholder="0 = 不限" />
            </div>
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">到期时间</label>
              <input type="datetime-local" value={expiresAt} onChange={(e) => setExpiresAt(e.target.value)} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" />
            </div>
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">状态</label>
              <Select
                value={statusOverride}
                onChange={(v) => setStatusOverride(String(v))}
                options={[
                  { label: t('common.running'), value: 'running' },
                  { label: t('common.stopped'), value: 'stopped' },
                  { label: t('common.error'), value: 'error' },
                  { label: t('common.expired'), value: 'expired' },
                  { label: t('common.banned'), value: 'banned' },
                  { label: t('common.nodeOffline'), value: 'offline' },
                  { label: t('common.missing'), value: 'missing' },
                ]}
              />
            </div>
          </div>
        </div>

        {/* 网络限速 */}
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">网络限速</h4>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">下行限速 (Mbps)</label>
              <input type="number" min={0} value={networkDown} onChange={(e) => setNetworkDown(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" placeholder="0 = 不限" />
            </div>
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">上行限速 (Mbps)</label>
              <input type="number" min={0} value={networkUp} onChange={(e) => setNetworkUp(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" placeholder="0 = 不限" />
            </div>
          </div>
        </div>

        {/* IO 限制 */}
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">IO 限制</h4>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">读 IOPS</label>
              <input type="number" min={0} value={ioReadIops} onChange={(e) => setIoReadIops(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" placeholder="0 = 不限" />
            </div>
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">写 IOPS</label>
              <input type="number" min={0} value={ioWriteIops} onChange={(e) => setIoWriteIops(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" placeholder="0 = 不限" />
            </div>
          </div>
        </div>

        {/* 流量配额 */}
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">流量配额</h4>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">月度流量 (GB)</label>
              <input type="number" min={0} value={monthlyTraffic} onChange={(e) => setMonthlyTraffic(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" placeholder="0 = 不限" />
            </div>
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">超限策略</label>
              <Select
                value={overLimitAction}
                options={[{ label: '直接关机', value: 'shutdown' }, { label: '限速', value: 'throttle' }]}
                onChange={(v) => setOverLimitAction(v as string)}
              />
            </div>
            {overLimitAction === 'throttle' && (
              <div>
                <label className="block text-sm font-medium text-secondary mb-1">限速值 (Mbps)</label>
                <input type="number" min={1} value={throttleMbps} onChange={(e) => setThrottleMbps(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" />
              </div>
            )}
          </div>
        </div>

        {/* 限额配置 */}
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">限额配置</h4>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">端口映射限额</label>
              <input type="number" min={0} value={portMappingLimit} onChange={(e) => setPortMappingLimit(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" />
            </div>
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">快照限额</label>
              <input type="number" min={0} value={snapshotLimit} onChange={(e) => setSnapshotLimit(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" />
            </div>
          </div>
        </div>

        {/* 网卡与 EIP */}
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">网卡与 EIP</h4>
          <div className="space-y-3">
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-secondary mb-1">内网 IPv4</label>
                <div className="px-3 py-2 border border-surface rounded-lg text-sm text-tertiary bg-surface-secondary font-number">{instance.internal_ipv4 || '-'}</div>
              </div>
              <div>
                <label className="block text-sm font-medium text-secondary mb-1">内网 IPv6</label>
                <div className="px-3 py-2 border border-surface rounded-lg text-sm text-tertiary bg-surface-secondary font-number">{instance.internal_ipv6 || '-'}</div>
              </div>
              <div>
                <label className="block text-sm font-medium text-secondary mb-1">IPv4 模式</label>
                <div className="px-3 py-2 border border-surface rounded-lg text-sm text-tertiary bg-surface-secondary">{instance.ipv4_mode || 'nat'}</div>
              </div>
              <div>
                <label className="block text-sm font-medium text-secondary mb-1">IPv6 模式</label>
                <div className="px-3 py-2 border border-surface rounded-lg text-sm text-tertiary bg-surface-secondary">{instance.ipv6_mode || 'none'}</div>
              </div>
              <div>
                <label className="block text-sm font-medium text-secondary mb-1">网桥</label>
                <div className="px-3 py-2 border border-surface rounded-lg text-sm text-tertiary bg-surface-secondary">{instance.bridge_name || '-'}</div>
              </div>
              <div>
                <label className="block text-sm font-medium text-secondary mb-1">SSH 端口</label>
                <div className="px-3 py-2 border border-surface rounded-lg text-sm text-tertiary bg-surface-secondary font-number">{instance.ssh_port || '-'}</div>
              </div>
            </div>

            {/* EIP 管理区 */}
            <div className="border-t pt-3">
              <div className="flex items-center justify-between mb-2">
                <span className="text-sm font-medium text-secondary">EIP 绑定</span>
              </div>
              {/* 已绑定的 EIP */}
              <div className="space-y-2 mb-3">
                {instance.ipv4_eip && (
                  <div key="v4" className="flex items-center justify-between px-3 py-2 bg-surface-secondary rounded-lg">
                    <div className="flex items-center gap-2">
                      <span className="text-xs px-1.5 py-0.5 rounded bg-blue-100 text-blue-700">IPv4</span>
                      <span className="font-number text-sm">{instance.ipv4_eip}</span>
                    </div>
                    <button className="data-table-link-btn" onClick={() => instance.ipv4_eip_allocation_id && handleUnbindEIP(instance.ipv4_eip_allocation_id)} disabled={eipLoading} style={{ color: 'var(--color-red-500)' }}>解绑</button>
                  </div>
                )}
                {instance.ipv6_eip && (
                  <div key="v6" className="flex items-center justify-between px-3 py-2 bg-surface-secondary rounded-lg">
                    <div className="flex items-center gap-2">
                      <span className="text-xs px-1.5 py-0.5 rounded bg-blue-100 text-blue-700">IPv6</span>
                      <span className="font-number text-sm">{instance.ipv6_eip}</span>
                    </div>
                    <button className="data-table-link-btn" onClick={() => instance.ipv6_eip_allocation_id && handleUnbindEIP(instance.ipv6_eip_allocation_id)} disabled={eipLoading} style={{ color: 'var(--color-red-500)' }}>解绑</button>
                  </div>
                )}
                {!instance.ipv4_eip && !instance.ipv6_eip && (
                  <div className="text-sm text-muted px-3 py-2">暂无绑定的 EIP</div>
                )}
              </div>

              {/* 绑定 IPv4 EIP */}
              {!instance.ipv4_eip && (
                <div className="space-y-2 mb-3 p-3 border border-surface-light rounded-lg">
                  <div className="text-xs font-medium text-tertiary">绑定 IPv4 EIP</div>
                  <Select
                    value={eipv4PoolId}
                    placeholder="选择 IPv4 EIP 池"
                    options={eipPools.filter(p => p.ip_version === 'ipv4').map(p => ({ label: `${p.cidr} (${p.interface || '无网卡'})`, value: p.id }))}
                    onChange={(v) => { setEipv4PoolId(String(v)); setEipv4SelectedIP('') }}
                  />
                  {eipv4PoolId && eipv4AddrList.length > 0 && (
                    <div>
                      <label className="block text-xs text-tertiary mb-1">选择地址（留空自动分配）</label>
                      <Select
                        value={eipv4SelectedIP}
                        placeholder="自动分配"
                        options={[{ label: '自动分配', value: '' }, ...eipv4AddrList.map(addr => ({ label: addr.split('/')[0], value: addr }))]}
                        onChange={(v) => setEipv4SelectedIP(v as string)}
                      />
                    </div>
                  )}
                  <button className="data-table-link-btn" onClick={handleBindEIPv4} disabled={eipLoading || !eipv4PoolId}>绑定 IPv4 EIP</button>
                </div>
              )}

              {/* 绑定 IPv6 EIP */}
              {!instance.ipv6_eip && instance.bridge_id && (
                <div className="space-y-2 p-3 border border-surface-light rounded-lg">
                  <div className="text-xs font-medium text-tertiary">绑定 IPv6 EIP</div>
                  <div>
                    <label className="block text-xs text-tertiary mb-1">前缀长度</label>
                    <input type="number" min={64} max={128} value={eipv6PrefixLen} onChange={(e) => setEipv6PrefixLen(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" />
                  </div>
                  {eipv6AddrList.length > 0 && (
                    <div>
                      <label className="block text-xs text-tertiary mb-1">选择子段</label>
                      <Select
                        value={eipv6SelectedIP}
                        placeholder="选择 IPv6 子段"
                        options={eipv6AddrList.map(addr => ({ label: addr, value: addr }))}
                        onChange={(v) => setEipv6SelectedIP(v as string)}
                      />
                    </div>
                  )}
                  {eipv6AddrList.length === 0 && (
                    <div className="text-xs text-muted">无可用 IPv6 子段</div>
                  )}
                  <button className="data-table-link-btn" onClick={handleBindEIPv6} disabled={eipLoading || !eipv6SelectedIP}>绑定 IPv6 EIP</button>
                </div>
              )}
            </div>
          </div>
        </div>

        {/* 磁盘管理 */}
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">磁盘管理</h4>
          <div className="space-y-3">
            {/* 系统盘（可扩容） */}
            <div className="flex items-center justify-between px-3 py-2 bg-surface-secondary rounded-lg">
              <div className="flex items-center gap-2">
                <span className="text-xs px-1.5 py-0.5 rounded bg-surface-secondary">系统盘</span>
                <span className="font-number text-sm">{(instance.disk_mb / 1024).toFixed(0)} GB</span>
              </div>
              <div className="flex items-center gap-2">
                <input type="number" min={1} value={sysDiskResize} onChange={(e) => setSysDiskResize(Number(e.target.value))} className="w-20 px-2 py-1 border border-surface-strong rounded text-sm font-number" />
                <select value={sysDiskUnit} onChange={(e) => setSysDiskUnit(e.target.value as 'MB' | 'GB' | 'TB')} className="px-1 py-1 border border-surface-strong rounded text-xs bg-surface">
                  <option value="MB">MB</option>
                  <option value="GB">GB</option>
                  <option value="TB">TB</option>
                </select>
                <button className="data-table-link-btn" onClick={handleResizeSysDisk} disabled={diskLoading || (sysDiskUnit === 'MB' ? sysDiskResize : sysDiskUnit === 'GB' ? sysDiskResize * 1024 : sysDiskResize * 1024 * 1024) <= instance.disk_mb}>扩容</button>
              </div>
            </div>
            {/* 数据盘列表 */}
            {instance.data_disks && instance.data_disks.length > 0 && (
              <div className="space-y-2">
                {instance.data_disks.map(disk => (
                  <div key={disk.id} className="flex items-center justify-between px-3 py-2 border border-surface-light rounded-lg">
                    <div className="flex items-center gap-2">
                      <span className="text-xs px-1.5 py-0.5 rounded bg-surface-secondary">数据盘</span>
                      <span className="text-sm">{disk.name}</span>
                      <span className="font-number text-sm text-tertiary">{(disk.size_mb / 1024).toFixed(0)} GB</span>
                      {disk.mount_point && <span className="text-xs text-muted">{disk.mount_point}</span>}
                    </div>
                    <div className="flex items-center gap-2">
                      <button className="data-table-link-btn" onClick={() => { setResizeDiskId(disk.id); setResizeDiskSize(Math.round(disk.size_mb / 1024)) }}>扩容</button>
                      <button className="data-table-link-btn" onClick={() => handleDeleteDisk(disk.id)} disabled={diskLoading} style={{ color: 'var(--color-red-500)' }}>删除</button>
                    </div>
                  </div>
                ))}
              </div>
            )}
            {/* 扩容输入区 */}
            {resizeDiskId && (
              <div className="flex items-center gap-2 px-3 py-2 border border-blue-200 rounded-lg bg-blue-50">
                <span className="text-sm text-tertiary">扩容至</span>
                <input type="number" min={1} value={resizeDiskSize} onChange={(e) => setResizeDiskSize(Number(e.target.value))} className="w-24 px-2 py-1 border border-surface-strong rounded text-sm font-number" />
                <span className="text-sm text-tertiary">GB</span>
                <button className="data-table-link-btn" onClick={handleResizeDisk} disabled={diskLoading}>确认</button>
                <button className="data-table-link-btn" onClick={() => { setResizeDiskId(''); setResizeDiskSize(0) }}>取消</button>
              </div>
            )}
            {/* 添加数据盘 */}
            <div className="border-t pt-3 space-y-2">
              <div className="text-xs text-tertiary mb-1">添加数据盘</div>
              <div className="grid grid-cols-2 gap-3">
                <input type="text" value={newDiskName} onChange={(e) => setNewDiskName(e.target.value)} className="px-3 py-2 border border-surface-strong rounded-lg text-sm" placeholder="磁盘名称" />
                <input type="number" min={1} value={newDiskSize} onChange={(e) => setNewDiskSize(Number(e.target.value))} className="px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" placeholder="大小 GB" />
                <input type="text" value={newDiskPool} onChange={(e) => setNewDiskPool(e.target.value)} className="px-3 py-2 border border-surface-strong rounded-lg text-sm" placeholder="存储池 (默认 default)" />
                <input type="text" value={newDiskMount} onChange={(e) => setNewDiskMount(e.target.value)} className="px-3 py-2 border border-surface-strong rounded-lg text-sm" placeholder="挂载点 (可选)" />
              </div>
              <button className="data-table-link-btn" onClick={handleAddDisk} disabled={diskLoading || !newDiskName.trim()}>+ 添加数据盘</button>
            </div>
          </div>
        </div>
      </div>
    </SlidePanel>
  )
}
