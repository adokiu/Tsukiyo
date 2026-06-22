import { useEffect, useState, useRef, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import {
  ArrowLeft, Play, Square, RotateCw, Trash2, Terminal, Monitor,
} from 'lucide-react'
import apiClient from '@/api/client'
import { Button } from '@/components/Button/Button'
import { Modal } from '@/components/Modal/Modal'
import { useToastStore } from '@/stores/toast'
import { useTranslation } from 'react-i18next'
import { OverviewTab } from '@/components/InstanceDetail/OverviewTab'
import { MonitoringTab } from '@/components/InstanceDetail/MonitoringTab'
import { PortMappingTab } from '@/components/InstanceDetail/PortMappingTab'
import { DiskTab } from '@/components/InstanceDetail/DiskTab'
import { SnapshotTab } from '@/components/InstanceDetail/SnapshotTab'
import { ReinstallTab } from '@/components/InstanceDetail/ReinstallTab'
import { getStatusLabel, generateRandomPassword } from '@/utils/format'

type TabKey = 'overview' | 'monitoring' | 'portMapping' | 'disk' | 'snapshot' | 'reinstall'

interface Instance {
  id: string
  name: string
  type: string
  status: string
  node_id: string
  node_name?: string
  user_id: number
  owner_name?: string
  owner_email?: string
  incus_name: string
  template_id: string
  vcpu: number
  memory_mb: number
  swap_mb: number
  disk_mb: number
  storage_pool: string
  internal_ipv4?: string
  internal_ipv6?: string
  login_method: string
  ssh_port?: number
  ssh_password?: string
  ssh_public_key?: string
  network_down?: number
  network_up?: number
  io_read_iops?: number
  io_write_iops?: number
  monthly_traffic?: number
  traffic_used_gb?: number
  traffic_mode: string
  over_limit_action?: string
  throttle_mbps?: number
  is_over_limit?: boolean
  snapshot_limit: number
  port_mapping_limit?: number
  bridge_id?: string
  bridge_name?: string
  bridge_iface?: string
  bridge_cidr?: string
  bridge_gateway?: string
  ipv4_eip?: string
  ipv4_eip_alias?: string
  ipv6_eip?: string
  ipv6_eip_alias?: string
  has_eip?: boolean
  data_disks?: DataDisk[]
  port_mappings?: PortMapping[]
  expired_at?: string
  created_at: string
  expires_at?: string
}

interface DataDisk {
  id: string
  name: string
  size_mb: number
  mount_point: string
  storage_pool?: string
  status?: string
  updated_at?: string
}

interface PortMapping {
  id: string
  host_port: number
  container_port: number
  protocol: string
  description?: string
}

interface Snapshot {
  id: string
  name: string
  created_at: string
  size?: number
}

interface Metrics {
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
  traffic_used_gb?: number
  monthly_traffic?: number
}

interface MetricPoint {
  timestamp: string
  cpu: number
  cpu_max?: number
  cpu_min?: number
  mem_used: number
  mem_used_max?: number
  mem_used_min?: number
  mem_total: number
  disk_used: number
  disk_used_max?: number
  disk_used_min?: number
  disk_total: number
  disk_read_bps: number
  disk_read_max?: number
  disk_write_bps: number
  disk_write_max?: number
  disk_read_iops?: number
  disk_write_iops?: number
  net_in: number
  net_in_max?: number
  net_in_min?: number
  net_out: number
  net_out_max?: number
  net_out_min?: number
}

interface InstalledImage {
  id: string
  fingerprint: string
  alias: string
  display_name: string
  type: string
  architecture: string
  size: number
  description: string
  image_source: string
  upload_date: string
  category_id?: string | null
  category_name?: string
  install_ssh: boolean
}

interface Category {
  id: string
  name: string
  image_type: string
  sort_order: number
}

export default function InstanceDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { t } = useTranslation()
  const toast = useToastStore()

  const [instance, setInstance] = useState<Instance | null>(null)
  const [metrics, setMetrics] = useState<Metrics | null>(null)
  const [loading, setLoading] = useState(true)
  const [actionLoading, setActionLoading] = useState(false)
  const [showPassword, setShowPassword] = useState(false)
  const [snapshots, setSnapshots] = useState<Snapshot[]>([])
  const [currentTab, setCurrentTab] = useState<TabKey>('overview')

  // 监控图表状态
  const [metricHistory, setMetricHistory] = useState<MetricPoint[]>([])
  const [metricPeriod, setMetricPeriod] = useState('1m')
  const [metricLoading, setMetricLoading] = useState(false)
  const wsRef = useRef<WebSocket | null>(null)
  const realtimeDataRef = useRef<MetricPoint[]>([])
  const wsFailCountRef = useRef(0)

  // 重装系统状态
  const [reinstallCategories, setReinstallCategories] = useState<Category[]>([])
  const [reinstallImages, setReinstallImages] = useState<InstalledImage[]>([])
  const [reinstallCategory, setReinstallCategory] = useState('')
  const [reinstallImage, setReinstallImage] = useState('')
  const [reinstallDialogOpen, setReinstallDialogOpen] = useState(false)
  const [reinstallLoginMode, setReinstallLoginMode] = useState<'auto' | 'password' | 'sshkey'>('auto')
  const [reinstallPassword, setReinstallPassword] = useState('')
  const [reinstallSSHKey, setReinstallSSHKey] = useState('')
  const [reinstallFormatDisks, setReinstallFormatDisks] = useState(false)

  // 重置密码弹窗
  const [resetPwdDialogOpen, setResetPwdDialogOpen] = useState(false)
  const [resetPwdValue, setResetPwdValue] = useState('')

  const fetchInstance = async () => {
    if (!id) return
    try {
      const res = await apiClient.get(`/instances/${id}`)
      setInstance(res.data)
    } catch (err: any) {
      toast.error(err.response?.data?.error || '获取实例详情失败')
    } finally {
      setLoading(false)
    }
  }

  const fetchMetrics = async () => {
    if (!id) return
    try {
      const res = await apiClient.get(`/instances/${id}/metrics`)
      setMetrics(res.data)
    } catch {
      // 忽略监控获取失败
    }
  }

  const fetchMetricHistory = useCallback(async () => {
    if (!id) return
    setMetricLoading(true)
    try {
      const res = await apiClient.get(`/instances/${id}/metrics/history?period=${metricPeriod}`)
      const points: MetricPoint[] = res.data.data || []
      setMetricHistory(points)
    } catch {
      setMetricHistory([])
    } finally {
      setMetricLoading(false)
    }
  }, [id, metricPeriod])

  const fetchSnapshots = async () => {
    if (!id) return
    try {
      const res = await apiClient.get(`/instances/${id}/snapshots`)
      setSnapshots(res.data.data || [])
    } catch {
      setSnapshots([])
    }
  }

  const fetchReinstallImages = async () => {
    if (!instance?.node_id) return
    const imageType = instance.type === 'vm' ? 'virtual-machine' : 'container'
    try {
      // 获取分类
      const catRes = await apiClient.get('/images/categories', { params: { node_id: instance.node_id, type: imageType } })
      setReinstallCategories(catRes.data.data || [])
      // 获取已安装镜像
      const imgRes = await apiClient.get('/images/installed', { params: { node_id: instance.node_id, type: imageType } })
      setReinstallImages(imgRes.data.data || [])
    } catch {
      setReinstallCategories([])
      setReinstallImages([])
    }
  }

  useEffect(() => {
    fetchInstance()
    fetchMetrics()
    fetchSnapshots()
    // 轮询作为 fallback，WS 不可用时每5秒刷新
    const interval = setInterval(() => {
      if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
        fetchMetrics()
        if (metricPeriod !== '1m') fetchMetricHistory()
      }
    }, 5000)
    return () => clearInterval(interval)
  }, [id, metricPeriod])

  useEffect(() => {
    fetchMetricHistory()
  }, [fetchMetricHistory])

  // WebSocket 实时监控数据推送（带自动重连）
  useEffect(() => {
    if (!id) return
    let reconnectTimer: ReturnType<typeof setTimeout> | undefined
    let manualClose = false

    const connect = () => {
      const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const ws = new WebSocket(`${proto}//${window.location.host}/ws/instances`)
      wsRef.current = ws

      ws.onopen = () => {
        wsFailCountRef.current = 0
      }

      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data)
          if (msg.type === 'instance_metrics' && msg.instance_id === id) {
            const d = msg.data
            // 更新概览卡片的实时 metrics
            setMetrics({
              cpu_usage: d.cpu_usage,
              memory_usage: d.memory_usage,
              memory_total: d.memory_total,
              disk_used: d.disk_used,
              disk_total: d.disk_total,
              disk_read_bps: d.disk_read_bps,
              disk_write_bps: d.disk_write_bps,
              disk_read_iops: d.disk_read_iops,
              disk_write_iops: d.disk_write_iops,
              network_rx: d.network_rx,
              network_tx: d.network_tx,
              traffic_used_gb: d.traffic_used_gb,
              monthly_traffic: d.monthly_traffic,
            })
            // 实时模式（1m）时追加到历史数据
            if (metricPeriod === '1m') {
              const point: MetricPoint = {
                timestamp: new Date(d.timestamp * 1000).toISOString(),
                cpu: d.cpu_usage || 0,
                mem_used: d.memory_usage || 0,
                mem_total: d.memory_total || 0,
                disk_used: d.disk_used || 0,
                disk_total: d.disk_total || 0,
                disk_read_bps: d.disk_read_bps || 0,
                disk_write_bps: d.disk_write_bps || 0,
                disk_read_iops: d.disk_read_iops || 0,
                disk_write_iops: d.disk_write_iops || 0,
                net_in: d.network_rx || 0,
                net_out: d.network_tx || 0,
              }
              const newData = [...realtimeDataRef.current, point].slice(-120)
              realtimeDataRef.current = newData
              setMetricHistory(newData)
            }
          }
        } catch {
          // 忽略解析失败
        }
      }

      ws.onclose = () => {
        wsRef.current = null
        wsFailCountRef.current++
        if (!manualClose) {
          reconnectTimer = setTimeout(connect, 3000)
        }
      }

      ws.onerror = () => {
        ws.close()
      }
    }

    connect()

    return () => {
      manualClose = true
      if (reconnectTimer) clearTimeout(reconnectTimer)
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
    }
  }, [id, metricPeriod])

  // 非1m模式定时刷新历史数据
  useEffect(() => {
    if (metricPeriod === '1m') {
      realtimeDataRef.current = []
      setMetricHistory([])
      return
    }
    const interval = setInterval(fetchMetricHistory, 30000)
    return () => clearInterval(interval)
  }, [metricPeriod, fetchMetricHistory])

  const handleAction = async (action: string) => {
    if (!id) return
    setActionLoading(true)
    try {
      if (action === 'delete') {
        if (!confirm('确认删除该实例？此操作不可恢复。')) return
        await apiClient.delete(`/instances/${id}`)
        toast.success('删除任务已下发')
        navigate('/admin/instanceManagement/container')
        return
      }
      if (action === 'terminal') {
        try {
          const res = await apiClient.get(`/instances/${id}/console?type=ssh`)
          if (res.data.token) {
            window.open(`/console?token=${res.data.token}`, '_blank')
          }
        } catch (err: any) {
          if (err.response?.status === 503) {
            toast.error('节点离线，无法连接终端')
          } else {
            toast.error(err.response?.data?.error || '获取终端信息失败')
          }
        }
        return
      }
      if (action === 'vnc') {
        try {
          const res = await apiClient.get(`/instances/${id}/console?type=vnc`)
          if (res.data.token) {
            window.open(`/vnc?token=${res.data.token}`, '_blank')
          }
        } catch (err: any) {
          if (err.response?.status === 503) {
            toast.error('节点离线，无法连接 VNC')
          } else {
            toast.error(err.response?.data?.error || '获取 VNC 信息失败')
          }
        }
        return
      }
      if (action === 'reset_password') {
        setResetPwdValue('')
        setResetPwdDialogOpen(true)
        return
      }
      if (action === 'ban') {
        if (!confirm('确认封禁该实例？封禁后实例将被强制停止。')) return
        try {
          await apiClient.post(`/instances/${id}/ban`)
          toast.success('实例已封禁')
          fetchInstance()
        } catch (err: any) {
          toast.error(err.response?.data?.error || '封禁失败')
        }
        return
      }
      if (action === 'unban') {
        try {
          await apiClient.post(`/instances/${id}/unban`)
          toast.success('实例已解封')
          fetchInstance()
        } catch (err: any) {
          toast.error(err.response?.data?.error || '解封失败')
        }
        return
      }
      await apiClient.post(`/instances/${id}/${action}`)
      toast.success(`操作 ${action} 已下发`)
      fetchInstance()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '操作失败')
    } finally {
      setActionLoading(false)
    }
  }

  const handleResetPassword = async () => {
    if (!id) return
    try {
      const body: Record<string, string> = {}
      if (resetPwdValue) body.password = resetPwdValue
      await apiClient.post(`/instances/${id}/reset-password`, body)
      toast.success('重置密码任务已下发')
      setResetPwdDialogOpen(false)
      fetchInstance()
    } catch (err: any) {
      if (err.response?.status === 409) {
        toast.error('实例正在执行其他操作')
      } else if (err.response?.status === 403) {
        toast.error(err.response?.data?.error || '实例已封禁或过期')
      } else {
        toast.error(err.response?.data?.error || '创建重置密码任务失败')
      }
    }
  }

  const handleReinstall = async () => {
    if (!id) return
    try {
      const body: Record<string, any> = {
        template_id: reinstallImage || instance?.template_id || '',
        login_method: reinstallLoginMode,
        format_data_disks: reinstallFormatDisks,
      }
      if (reinstallLoginMode === 'password' && reinstallPassword) {
        body.password = reinstallPassword
      }
      if (reinstallLoginMode === 'sshkey' && reinstallSSHKey) {
        body.ssh_key = reinstallSSHKey
      }
      await apiClient.post(`/instances/${id}/reinstall`, body)
      toast.success('重装任务已下发')
      setReinstallDialogOpen(false)
      fetchInstance()
    } catch (err: any) {
      if (err.response?.status === 409) {
        toast.error('实例正在执行其他操作，请稍后重试')
      } else {
        toast.error(err.response?.data?.error || '重装失败')
      }
    }
  }

  const handleAddPortMapping = async (containerPort: number, hostPort: number | null, protocol: string, description: string) => {
    if (!id) return
    try {
      const body: any = {
        instance_id: id,
        container_port: containerPort,
        protocol,
        ip_version: 'ipv4',
        description,
      }
      if (hostPort) body.host_port = hostPort
      await apiClient.post('/network/port-mappings', body)
      toast.success('端口映射添加成功')
      fetchInstance()
    } catch (err: any) {
      const status = err.response?.status
      if (status === 503) {
        toast.error('节点离线，无法添加端口映射')
      } else if (status === 502) {
        toast.error('Agent 执行失败')
      } else {
        toast.error(err.response?.data?.error || '添加失败')
      }
    }
  }

  const handleDeletePortMapping = async (pmID: string) => {
    if (!confirm('确认删除该端口映射？')) return
    try {
      await apiClient.delete(`/network/port-mappings/${pmID}`)
      toast.success('端口映射已删除')
      fetchInstance()
    } catch (err: any) {
      const status = err.response?.status
      if (status === 503) {
        toast.error('节点离线，无法删除端口映射')
      } else if (status === 502) {
        toast.error('Agent 执行失败')
      } else {
        toast.error(err.response?.data?.error || '删除失败')
      }
    }
  }

  const handleDeleteSnapshot = async (snapshotID: string) => {
    if (!confirm('确认删除该快照？')) return
    try {
      await apiClient.delete(`/instances/${id}/snapshots/${snapshotID}`)
      toast.success('快照删除成功')
      fetchSnapshots()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '删除失败')
    }
  }

  const handleCreateSnapshot = async () => {
    const name = prompt('请输入快照名称：')
    if (!name) return
    try {
      await apiClient.post(`/instances/${id}/snapshots`, { name })
      toast.success('快照创建任务已下发')
      fetchSnapshots()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '创建失败')
    }
  }

  const handleRestoreSnapshot = async (snapshotName: string) => {
    if (!confirm(`确认恢复到快照 ${snapshotName}？当前数据将被覆盖。`)) return
    try {
      await apiClient.post(`/instances/${id}/snapshots/${snapshotName}/restore`)
      toast.success('快照恢复任务已下发')
      fetchInstance()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '恢复失败')
    }
  }

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
    toast.success('已复制到剪贴板')
  }

  if (loading) {
    return (
      <div className="p-6 flex items-center justify-center min-h-[400px]">
        <div className="animate-spin rounded-full h-8 w-8 border-2 border-apple-blue border-t-transparent" />
      </div>
    )
  }

  if (!instance) {
    return (
      <div className="p-6">
        <button onClick={() => navigate('/admin/instanceManagement/container')} className="flex items-center gap-1 text-sm text-tertiary hover:text-secondary mb-4">
          <ArrowLeft size={16} /> 返回实例列表
        </button>
        <div className="text-center text-tertiary">实例不存在</div>
      </div>
    )
  }

  const statusColor = instance.status === 'running' ? 'text-green-600' : instance.status === 'stopped' ? 'text-tertiary' : instance.status === 'banned' ? 'text-red-600' : instance.status === 'expired' ? 'text-orange-600' : instance.status === 'offline' ? 'text-red-600' : 'text-amber-600'
  const statusBg = instance.status === 'running' ? 'bg-green-100' : instance.status === 'stopped' ? 'bg-surface-secondary' : instance.status === 'banned' ? 'bg-red-100' : instance.status === 'expired' ? 'bg-orange-100' : instance.status === 'offline' ? 'bg-red-100' : 'bg-amber-100'
  const isBusy = ['creating', 'starting', 'stopping', 'restarting', 'reinstalling', 'resizing', 'deleting'].includes(instance.status)
  const isBanned = instance.status === 'banned'
  const isExpired = instance.status === 'expired'
  const isOffline = instance.status === 'offline'

  const tabs: { key: TabKey; label: string }[] = [
    { key: 'overview', label: '概览' },
    { key: 'monitoring', label: '监控' },
    ...(instance.has_eip ? [] : [{ key: 'portMapping' as TabKey, label: '端口映射' }]),
    { key: 'disk', label: '磁盘管理' },
    { key: 'snapshot', label: '快照管理' },
    { key: 'reinstall', label: '重装系统' },
  ]

  const cpuPercent = metrics?.cpu_usage ?? 0
  const memPercent = metrics?.memory_total ? ((metrics.memory_usage || 0) / metrics.memory_total) * 100 : 0
  const diskTotalBytes = metrics?.disk_total || (instance.disk_mb * 1024 * 1024)
  const diskPercent = diskTotalBytes ? ((metrics?.disk_used || 0) / diskTotalBytes) * 100 : 0

  return (
    <div className="p-6 space-y-6">
      {/* 顶部导航 */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <button onClick={() => navigate('/admin/instanceManagement/container')} className="text-tertiary hover:text-secondary">
            <ArrowLeft size={20} />
          </button>
          <div>
            <h1 className="text-xl font-semibold text-primary">{instance.name}</h1>
            <div className="flex items-center gap-2 text-sm text-tertiary">
              <span className="font-mono">{instance.incus_name}</span>
              <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${statusBg} ${statusColor}`}>{getStatusLabel(instance.status, t)}</span>
              <span className="font-mono text-xs">{instance.type}</span>
            </div>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {instance.status === 'stopped' && !isBanned && !isExpired && !isOffline && (
            <Button icon={<Play size={14} />} onClick={() => handleAction('start')} loading={actionLoading}>启动</Button>
          )}
          {instance.status === 'running' && !isBanned && !isExpired && !isOffline && (
            <Button icon={<Square size={14} />} variant="ghost" onClick={() => handleAction('stop')} loading={actionLoading}>停止</Button>
          )}
          {!isBanned && !isExpired && !isBusy && !isOffline && (
            <Button icon={<RotateCw size={14} />} variant="ghost" onClick={() => handleAction('restart')} loading={actionLoading}>重启</Button>
          )}
          {!isBanned && !isExpired && !isBusy && !isOffline && (
            <Button icon={<Terminal size={14} />} variant="ghost" onClick={() => handleAction('terminal')} loading={actionLoading}>终端</Button>
          )}
          {!isBanned && !isExpired && !isBusy && !isOffline && instance.type === 'vm' && (
            <Button icon={<Monitor size={14} />} variant="ghost" onClick={() => handleAction('vnc')} loading={actionLoading}>VNC</Button>
          )}
          {isBanned && (
            <Button variant="ghost" onClick={() => handleAction('unban')} loading={actionLoading}>解封</Button>
          )}
          {!isBanned && !isExpired && !isBusy && !isOffline && (
            <Button variant="ghost" className="text-orange-600 hover:text-orange-700" onClick={() => handleAction('ban')} loading={actionLoading}>封禁</Button>
          )}
          <Button icon={<Trash2 size={14} />} variant="ghost" className="text-red-500 hover:text-red-700" onClick={() => handleAction('delete')} loading={actionLoading}>删除</Button>
        </div>
      </div>

      {/* Tab 导航 */}
      <div className="flex flex-wrap gap-2 bg-surface rounded-xl border border-surface p-3">
        {tabs.map((tab) => (
          <button
            key={tab.key}
            onClick={() => {
              setCurrentTab(tab.key)
              if (tab.key === 'reinstall') fetchReinstallImages()
            }}
            className={`px-4 py-2 text-sm font-semibold rounded-full border transition-all ${
              currentTab === tab.key
                ? 'border-blue-600 bg-blue-600 text-white shadow-sm'
                : 'border-surface text-tertiary hover:text-secondary hover:border-blue-300'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* 概览 Tab */}
      {currentTab === 'overview' && (
        <OverviewTab
          instance={instance}
          metrics={metrics}
          cpuPercent={cpuPercent}
          memPercent={memPercent}
          diskTotalBytes={diskTotalBytes}
          diskPercent={diskPercent}
          showPassword={showPassword}
          setShowPassword={setShowPassword}
          copyToClipboard={copyToClipboard}
          onResetPassword={() => setResetPwdDialogOpen(true)}
          isBanned={isBanned}
          isExpired={isExpired}
          isBusy={isBusy}
        />
      )}

      {/* 监控 Tab */}
      {currentTab === 'monitoring' && (
        <MonitoringTab
          metrics={metrics}
          metricHistory={metricHistory}
          metricPeriod={metricPeriod}
          setMetricPeriod={setMetricPeriod}
          metricLoading={metricLoading}
          cpuPercent={cpuPercent}
          memPercent={memPercent}
        />
      )}

      {/* 端口映射 Tab */}
      {currentTab === 'portMapping' && (
        <PortMappingTab
          portMappings={instance.port_mappings || []}
          portMappingLimit={instance.port_mapping_limit}
          onAdd={handleAddPortMapping}
          onDelete={handleDeletePortMapping}
        />
      )}

      {/* 磁盘管理 Tab */}
      {currentTab === 'disk' && (
        <DiskTab
          instanceId={id!}
          diskMB={instance.disk_mb}
          storagePool={instance.storage_pool}
          dataDisks={instance.data_disks || []}
          metrics={metrics}
          onRefresh={fetchInstance}
          toast={toast}
        />
      )}

      {/* 快照管理 Tab */}
      {currentTab === 'snapshot' && (
        <SnapshotTab
          snapshotLimit={instance.snapshot_limit}
          snapshots={snapshots}
          onCreate={handleCreateSnapshot}
          onRestore={handleRestoreSnapshot}
          onDelete={handleDeleteSnapshot}
        />
      )}

      {/* 重装系统 Tab */}
      {currentTab === 'reinstall' && (
        <ReinstallTab
          templateId={instance.template_id}
          categories={reinstallCategories}
          images={reinstallImages}
          category={reinstallCategory}
          setCategory={setReinstallCategory}
          image={reinstallImage}
          setImage={setReinstallImage}
          loginMode={reinstallLoginMode}
          setLoginMode={setReinstallLoginMode}
          password={reinstallPassword}
          setPassword={setReinstallPassword}
          sshKey={reinstallSSHKey}
          setSSHKey={setReinstallSSHKey}
          formatDisks={reinstallFormatDisks}
          setFormatDisks={setReinstallFormatDisks}
          onConfirm={() => setReinstallDialogOpen(true)}
          isBanned={isBanned}
          isExpired={isExpired}
          isBusy={isBusy}
        />
      )}

      {/* 重装确认弹窗 */}
      <Modal
        open={reinstallDialogOpen}
        onClose={() => setReinstallDialogOpen(false)}
        title="确认重装系统"
        confirmMode
        confirmText="确认重装"
        confirmVariant="danger"
        onConfirm={handleReinstall}
      >
        重装系统将删除容器内所有数据，此操作不可撤销！
        {reinstallFormatDisks && ' 所有数据盘也将被格式化。'}
      </Modal>

      {/* 重置密码弹窗 */}
      <Modal
        open={resetPwdDialogOpen}
        onClose={() => setResetPwdDialogOpen(false)}
        title="重置 SSH 密码"
        confirmMode
        confirmText="确认重置"
        confirmVariant="primary"
        onConfirm={handleResetPassword}
      >
        <div className="space-y-3">
          <p className="text-sm text-tertiary">留空则自动生成随机密码。</p>
          <div className="flex gap-2">
            <input
              type="text"
              value={resetPwdValue}
              onChange={(e) => setResetPwdValue(e.target.value)}
              placeholder="输入新密码（留空自动生成）"
              className="flex-1 px-3 py-2 border border-surface-strong rounded-lg text-sm font-mono"
            />
            <button
              onClick={() => setResetPwdValue(generateRandomPassword())}
              className="px-3 py-2 text-xs bg-surface-secondary hover:bg-surface-hover rounded-lg"
            >
              随机生成
            </button>
          </div>
        </div>
      </Modal>
    </div>
  )
}