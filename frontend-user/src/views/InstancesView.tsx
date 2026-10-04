import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { instancesApi, InstanceItem, InstanceMetrics } from '@/api/instances'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useState } from 'react'
import { Search, Play, Square, RotateCw, Loader2, Plus, Server } from 'lucide-react'
import { useToastStore } from '@/stores/toast'
import { useInstanceMetrics } from '@/hooks/useInstanceMetrics'
import { DataTable, type Column } from '@/components/DataTable/DataTable'
import { Tooltip } from '@/components/Tooltip/Tooltip'

type TabKey = 'running' | 'stopped' | 'creating' | 'error' | 'all'

export function InstancesView() {
  const { t } = useTranslation()
  const toast = useToastStore()
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const [page, setPage] = useState(1)
  const [activeTab, setActiveTab] = useState<TabKey>('all')
  const perPage = 20

  const { metrics: wsMetrics } = useInstanceMetrics()

  const { data, isLoading } = useQuery({
    queryKey: ['instances', search, page, activeTab],
    queryFn: () => instancesApi.list({
      search,
      page,
      per_page: perPage,
      filter_status: activeTab === 'all' ? undefined : activeTab,
    }),
  })

  const startMutation = useMutation({
    mutationFn: (id: string) => instancesApi.start(id),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['instances'] }); toast.success(t('instance.start') + ' ' + t('common.success')) },
    onError: () => toast.error(t('instance.start') + ' ' + t('common.failed')),
  })
  const stopMutation = useMutation({
    mutationFn: (id: string) => instancesApi.stop(id),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['instances'] }); toast.success(t('instance.stop') + ' ' + t('common.success')) },
    onError: () => toast.error(t('instance.stop') + ' ' + t('common.failed')),
  })
  const restartMutation = useMutation({
    mutationFn: (id: string) => instancesApi.restart(id),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['instances'] }); toast.success(t('instance.restart') + ' ' + t('common.success')) },
    onError: () => toast.error(t('instance.restart') + ' ' + t('common.failed')),
  })

  const items = data?.data.data ?? []
  const total = data?.data.total ?? 0
  const counts = data?.data.counts

  const tabs: { key: TabKey; label: string; count: number }[] = [
    { key: 'all', label: t('instance.tabAll'), count: counts?.all ?? 0 },
    { key: 'running', label: t('instance.tabRunning'), count: counts?.running ?? 0 },
    { key: 'stopped', label: t('instance.tabStopped'), count: counts?.stopped ?? 0 },
    { key: 'creating', label: t('instance.tabCreating'), count: counts?.creating ?? 0 },
    { key: 'error', label: t('instance.tabError'), count: counts?.error ?? 0 },
  ]

  const formatDate = (dateStr: string | null) => {
    if (!dateStr) return null
    const d = new Date(dateStr)
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
  }

  const getExpiryStyle = (dateStr: string | null): { text: string; color: string } => {
    if (!dateStr) return { text: t('instance.longTerm'), color: 'var(--color-gray-400)' }
    const d = new Date(dateStr)
    const now = new Date()
    const diffMs = d.getTime() - now.getTime()
    const diffDays = diffMs / (1000 * 60 * 60 * 24)
    if (diffDays < 0) return { text: formatDate(dateStr)!, color: '#dc2626' }
    if (diffDays < 3) return { text: formatDate(dateStr)!, color: '#dc2626' }
    if (diffDays < 7) return { text: formatDate(dateStr)!, color: '#eab308' }
    return { text: formatDate(dateStr)!, color: 'var(--color-gray-400)' }
  }

  const getMetrics = (inst: { id: string; status: string; metrics?: InstanceMetrics | null }): InstanceMetrics | null => {
    const wsM = wsMetrics[inst.id]
    if (wsM) {
      return {
        cpu_usage: wsM.cpu_usage,
        memory_used: wsM.memory_used,
        memory_total: wsM.memory_total,
        disk_used: wsM.disk_used,
        disk_total: wsM.disk_total,
        net_in_bps: wsM.net_in_bps,
        net_out_bps: wsM.net_out_bps,
        net_in_total: wsM.net_in_total,
        net_out_total: wsM.net_out_total,
      }
    }
    return inst.metrics ?? null
  }

  const getCpuDisplay = (inst: InstanceItem) => {
    const m = getMetrics(inst)
    const usage = m?.cpu_usage ?? 0
    const cores = inst.vcpu ?? 0
    return { usage, cores }
  }

  const getMemDisplay = (inst: InstanceItem) => {
    const m = getMetrics(inst)
    const used = m?.memory_used ?? 0
    const total = m?.memory_total ?? inst.memory_mb * 1024 * 1024
    const percent = total > 0 ? (used / total) * 100 : 0
    return { used, total, percent }
  }

  const getDiskDisplay = (inst: InstanceItem) => {
    const m = getMetrics(inst)
    const used = m?.disk_used ?? 0
    const total = m?.disk_total ?? inst.disk_mb * 1024 * 1024
    const percent = total > 0 ? (used / total) * 100 : 0
    return { used, total, percent }
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

  const getProgressColor = (percent: number): string => {
    if (percent >= 90) return 'metric-progress__fill--red'
    if (percent >= 70) return 'metric-progress__fill--yellow'
    return 'metric-progress__fill--green'
  }

  const columns: Column<InstanceItem>[] = [
    {
      key: 'name',
      title: t('instance.instanceName'),
      render: (inst) => (
        <Link to={`/instances/${inst.id}`} className="font-medium text-foreground hover:text-primary transition-colors">
          {inst.name}
        </Link>
      ),
    },
    {
      key: 'type',
      title: t('instance.type'),
      render: (inst) => (
        <span className="text-muted-foreground">
          {t(`instance.type${inst.type.charAt(0).toUpperCase()}${inst.type.slice(1)}`, inst.type)}
        </span>
      ),
    },
    {
      key: 'ip',
      title: 'IP',
      render: (inst) => (
        <div className="ip-cell">
          <span className="ip-cell__line font-number">{inst.ipv4_addr || '--'}</span>
          {inst.ipv6_addr && <span className="ip-cell__line font-number text-xs text-muted-foreground/70">{inst.ipv6_addr}</span>}
        </div>
      ),
    },
    {
      key: 'cpu',
      title: 'CPU',
      render: (inst) => {
        const { usage, cores } = getCpuDisplay(inst)
        return (
          <div className="metric-cell">
            <div className="metric-cell__header">
              <span className="metric-cell__percent font-number">{usage.toFixed(1)}%</span>
              <span className="metric-cell__detail font-number">/ {cores}核</span>
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
      title: t('instance.memory'),
      render: (inst) => {
        const { used, total, percent } = getMemDisplay(inst)
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
      title: t('instance.disk'),
      render: (inst) => {
        const { used, total, percent } = getDiskDisplay(inst)
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
      title: t('instance.traffic'),
      render: (inst) => {
        const used = inst.traffic_used_gb ?? 0
        const total = inst.monthly_traffic_gb ?? 0
        if (!total || total <= 0) {
          return <span className="text-sm" style={{ color: 'var(--color-gray-400)' }}>{t('instance.unlimited')}</span>
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
      title: t('instance.netIO'),
      render: (inst) => {
        const m = getMetrics(inst)
        const rx = m?.net_in_bps ?? 0
        const tx = m?.net_out_bps ?? 0
        return (
          <div className="net-cell font-number">
            <span className="net-cell__up">↑{formatNetSpeed(tx)}</span>
            <span className="net-cell__down">↓{formatNetSpeed(rx)}</span>
          </div>
        )
      },
    },
    {
      key: 'status',
      title: t('instance.status'),
      render: (inst) => {
        const tagClass = (() => {
          switch (inst.status) {
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
        })()
        return (
          <span className={tagClass}>
            {t(`common.${inst.status}`, inst.status)}
          </span>
        )
      },
    },
    {
      key: 'expires_at',
      title: t('instance.expiresAt'),
      render: (inst) => {
        const expiry = getExpiryStyle(inst.expires_at)
        return <span className="font-number text-sm" style={{ color: expiry.color }}>{expiry.text}</span>
      },
    },
    {
      key: 'action',
      title: t('common.actions'),
      render: (inst) => (
        <div className="instance-actions">
          <Tooltip content={t('instance.start')} placement="top">
            <button
              className="instance-action-btn"
              onClick={() => startMutation.mutate(inst.id)}
              disabled={inst.status === 'running' || startMutation.isPending}
            >
              <Play size={15} />
            </button>
          </Tooltip>
          <Tooltip content={t('instance.stop')} placement="top">
            <button
              className="instance-action-btn"
              onClick={() => stopMutation.mutate(inst.id)}
              disabled={inst.status === 'stopped' || stopMutation.isPending}
            >
              <Square size={15} />
            </button>
          </Tooltip>
          <Tooltip content={t('instance.restart')} placement="top">
            <button
              className="instance-action-btn"
              onClick={() => restartMutation.mutate(inst.id)}
              disabled={restartMutation.isPending}
            >
              <RotateCw size={15} />
            </button>
          </Tooltip>
        </div>
      ),
    },
  ]

  return (
    <main className="flex-1 overflow-y-auto overflow-x-hidden p-4 sm:p-6 md:p-8">
      <div className="mx-auto max-w-[1600px] w-full">
        <div className="space-y-6 animate-in fade-in slide-in-from-bottom-4 duration-700">

          <div className="flex flex-col md:flex-row md:items-end justify-between gap-4">
            <div>
              <h2 className="text-2xl font-medium tracking-tight">{t('instance.title')}</h2>
              <p className="text-sm text-muted-foreground mt-1">{t('instance.subtitle')}</p>
            </div>
            <Link
              to="/instances"
              className="inline-flex items-center gap-1.5 bg-primary text-primary-foreground hover:bg-primary/90 px-4 py-2 rounded-md text-sm font-medium transition-colors shadow-sm"
            >
              <Plus className="w-4 h-4" />
              {t('instance.createInstance')}
            </Link>
          </div>

          <div className="instance-tabs">
            {tabs.map(tab => (
              <button
                key={tab.key}
                className={`instance-tab ${activeTab === tab.key ? 'active' : ''}`}
                onClick={() => { setActiveTab(tab.key); setPage(1) }}
              >
                <span>{tab.label}</span>
                <span className="instance-tab-count font-number">{tab.count}</span>
              </button>
            ))}
          </div>

          <div className="instance-toolbar">
            <div className="instance-toolbar-search">
              <Search size={16} />
              <input
                type="text"
                value={search}
                onChange={(e) => { setSearch(e.target.value); setPage(1) }}
                placeholder={t('instance.searchPlaceholder')}
              />
            </div>
            <span className="instance-toolbar-count">{t('instance.totalCount', { count: total })}</span>
          </div>

          {isLoading ? (
            <div className="flex justify-center py-12">
              <Loader2 className="animate-spin text-muted-foreground" size={24} />
            </div>
          ) : items.length === 0 ? (
            <div className="data-table-wrapper">
              <div className="data-table__empty">
                <div className="flex flex-col items-center gap-3">
                  <Server size={48} className="text-muted-foreground opacity-50" />
                  <h3 className="text-base font-medium">{t('instance.noInstances')}</h3>
                  <p className="text-sm text-muted-foreground">{t('instance.noInstancesDesc')}</p>
                  <Link
                    to="/instances"
                    className="inline-flex items-center gap-1.5 bg-primary text-primary-foreground hover:bg-primary/90 px-4 py-2 rounded-md text-sm font-medium transition-colors"
                  >
                    <Plus className="w-4 h-4" />
                    {t('instance.createInstance')}
                  </Link>
                </div>
              </div>
            </div>
          ) : (
            <DataTable<InstanceItem>
              columns={columns}
              data={items}
              rowKey={(row) => row.id}
              loading={isLoading}
              pagination={{ page, size: perPage, total }}
              onPageChange={setPage}
              emptyText={t('instance.noInstances')}
            />
          )}

        </div>
      </div>
    </main>
  )
}
