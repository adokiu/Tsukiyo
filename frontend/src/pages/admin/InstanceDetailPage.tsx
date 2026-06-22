import { useEffect, useState, useRef, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import {
  ArrowLeft, Play, Square, RotateCw, Trash2, Terminal, Lock, Monitor,
  HardDrive, Cpu, MemoryStick, Network, Gauge,
  Calendar, Server, Eye, EyeOff, Copy, Plus,
  AlertTriangle, ChevronRight, Globe
} from 'lucide-react'
import {
  AreaChart, Area, LineChart, Line, XAxis, YAxis,
  CartesianGrid, Tooltip, ResponsiveContainer
} from 'recharts'
import apiClient from '@/api/client'
import { Button } from '@/components/Button/Button'
import { Modal } from '@/components/Modal/Modal'
import { useToastStore } from '@/stores/toast'
import { useTranslation } from 'react-i18next'

type TabKey = 'overview' | 'monitoring' | 'portMapping' | 'disk' | 'snapshot' | 'reinstall'

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
interface Category { id: string; name: string; image_type: string; sort_order: number }

const TIME_RANGES = [
  { key: '1m', label: '实时' },
  { key: '15m', label: '15分钟' },
  { key: '1h', label: '1小时' },
  { key: '1d', label: '1天' },
  { key: '7d', label: '7天' },
]

const tooltipStyle = {
  backgroundColor: 'rgba(255,255,255,0.98)',
  border: '1px solid #e5e7eb',
  borderRadius: '8px',
  fontSize: '12px',
  boxShadow: '0 2px 8px rgba(0,0,0,0.08)',
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(Math.abs(bytes)) / Math.log(k))
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`
}

function formatSpeed(bytes: number): string {
  return `${formatBytes(bytes)}/s`
}

function formatTimeByPeriod(ts: string, period: string): string {
  const d = new Date(ts)
  if (period === '1m' || period === '15m') {
    return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  }
  if (period === '1h') {
    return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  }
  return d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

function generateRandomPassword(length: number = 16): string {
  const chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*'
  let result = ''
  for (let i = 0; i < length; i++) {
    result += chars.charAt(Math.floor(Math.random() * chars.length))
  }
  return result
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
  const [addingPM, setAddingPM] = useState(false)
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
      setAddingPM(false)
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

  // 去掉 IP 地址中的 CIDR 子网掩码后缀
  // IPv4 /32 和 IPv6 /128 是单机地址，去掉前缀；其他前缀保留显示
  const stripCIDR = (ip: string): string => {
    if (!ip) return ''
    const idx = ip.indexOf('/')
    if (idx === -1) return ip
    const prefix = ip.substring(idx + 1)
    if (prefix === '32' || prefix === '128') return ip.substring(0, idx)
    return ip
  }

  const sshAddress = stripCIDR(instance.ipv4_eip || instance.internal_ipv4 || '')
  const sshPort = instance.ssh_port || 22

  // 美化镜像名称显示
  const formatImageName = (templateId: string): string => {
    if (!templateId) return '-'
    // debian/13/cloud -> Debian 13
    // alpine/3.21 -> Alpine 3.21
    // ubuntu/24.04 -> Ubuntu 24.04
    const parts = templateId.split('/')
    if (parts.length >= 2) {
      const distro = parts[0].charAt(0).toUpperCase() + parts[0].slice(1)
      const version = parts[1]
      return `${distro} ${version}`
    }
    return templateId
  }

  // 流量模式美化
  const formatTrafficMode = (mode: string): string => {
    switch (mode) {
      case 'total': return '总计（入站+出站）'
      case 'inbound': return '入站'
      case 'outbound': return '出站'
      case 'max': return '最大值（入站/出站取大）'
      default: return mode || '-'
    }
  }

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
        <div className="space-y-4">
          {/* 资源配置卡片 */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div className="bg-surface rounded-xl border border-surface p-4">
              <div className="flex items-center gap-2 text-tertiary mb-2">
                <Cpu size={16} />
                <span className="text-sm">CPU</span>
              </div>
              <div className="text-2xl font-semibold">{instance.vcpu} <span className="text-sm font-normal text-tertiary">核</span></div>
              {metrics?.cpu_usage !== undefined && (
                <div className="mt-2">
                  <div className="text-xs text-tertiary mb-1">使用率: {cpuPercent.toFixed(1)}%</div>
                  <div className="h-1.5 bg-surface-secondary rounded-full overflow-hidden">
                    <div className="h-full bg-gradient-to-r from-teal-500 to-orange-500 rounded-full" style={{ width: `${Math.min(cpuPercent, 100)}%` }} />
                  </div>
                </div>
              )}
            </div>
            <div className="bg-surface rounded-xl border border-surface p-4">
              <div className="flex items-center gap-2 text-tertiary mb-2">
                <MemoryStick size={16} />
                <span className="text-sm">内存</span>
              </div>
              <div className="text-2xl font-semibold">{instance.memory_mb} <span className="text-sm font-normal text-tertiary">MB</span></div>
              {instance.swap_mb > 0 && (
                <div className="text-xs text-tertiary mt-1">Swap: {instance.swap_mb} MB</div>
              )}
              {metrics?.memory_usage !== undefined && metrics?.memory_total !== undefined && (
                <div className="mt-2">
                  <div className="text-xs text-tertiary mb-1">已用: {formatBytes(metrics.memory_usage)} / {formatBytes(metrics.memory_total)}</div>
                  <div className="h-1.5 bg-surface-secondary rounded-full overflow-hidden">
                    <div className="h-full bg-gradient-to-r from-teal-500 to-orange-500 rounded-full" style={{ width: `${Math.min(memPercent, 100)}%` }} />
                  </div>
                </div>
              )}
            </div>
            <div className="bg-surface rounded-xl border border-surface p-4">
              <div className="flex items-center gap-2 text-tertiary mb-2">
                <HardDrive size={16} />
                <span className="text-sm">系统盘</span>
              </div>
              <div className="text-2xl font-semibold">{(instance.disk_mb / 1024).toFixed(0)} <span className="text-sm font-normal text-tertiary">GB</span></div>
              {metrics?.disk_used !== undefined && (
                <div className="mt-2">
                  <div className="text-xs text-tertiary mb-1">
                    已用: {formatBytes(metrics.disk_used)} / {diskTotalBytes ? formatBytes(diskTotalBytes) : `${(instance.disk_mb / 1024).toFixed(0)} GB`}
                  </div>
                  <div className="h-1.5 bg-surface-secondary rounded-full overflow-hidden">
                    <div className="h-full bg-gradient-to-r from-teal-500 to-orange-500 rounded-full" style={{ width: `${Math.min(diskPercent, 100)}%` }} />
                  </div>
                </div>
              )}
            </div>
            <div className="bg-surface rounded-xl border border-surface p-4">
              <div className="flex items-center gap-2 text-tertiary mb-2">
                <Gauge size={16} />
                <span className="text-sm">流量</span>
              </div>
              {(() => {
                const usedGB = metrics?.traffic_used_gb ?? instance.traffic_used_gb ?? 0
                const totalGB = metrics?.monthly_traffic ?? instance.monthly_traffic ?? 0
                const percent = totalGB > 0 ? (usedGB / totalGB * 100) : 0
                return (
                  <>
                    <div className="text-2xl font-semibold">
                      {usedGB.toFixed(2)} <span className="text-sm font-normal text-tertiary">/ {totalGB} GB</span>
                    </div>
                    <div className="mt-2">
                      <div className="text-xs text-tertiary mb-1">已用 {percent.toFixed(1)}%</div>
                      <div className="h-1.5 bg-surface-secondary rounded-full overflow-hidden">
                        <div className="h-full bg-gradient-to-r from-teal-500 to-orange-500 rounded-full" style={{ width: `${Math.min(percent, 100)}%` }} />
                      </div>
                    </div>
                  </>
                )
              })()}
            </div>
          </div>

          <div className="grid grid-cols-2 gap-6">
            {/* 实例信息 */}
            <div className="bg-surface rounded-xl border border-surface p-5 space-y-4">
              <h3 className="text-sm font-semibold text-primary flex items-center gap-2">
                <Server size={16} /> 实例信息
              </h3>
              <div className="grid grid-cols-2 gap-y-3 text-sm">
                <div className="text-tertiary">宿主机</div>
                <div>{instance.node_name || instance.node_id.slice(0, 8)}</div>
                <div className="text-tertiary">所有者</div>
                <div>{instance.owner_name || '-'}</div>
                <div className="text-tertiary">系统镜像</div>
                <div>{formatImageName(instance.template_id)}</div>
                <div className="text-tertiary">存储池</div>
                <div>{instance.storage_pool || 'default'}</div>
                <div className="text-tertiary">磁盘IO限制</div>
                <div>读 {instance.io_read_iops || 0} IOPS / 写 {instance.io_write_iops || 0} IOPS</div>
                <div className="text-tertiary">创建时间</div>
                <div>{instance.created_at ? new Date(instance.created_at).toLocaleString() : '-'}</div>
                <div className="text-tertiary">到期时间</div>
                <div>{instance.expires_at ? new Date(instance.expires_at).toLocaleString() : '-'}</div>
                <div className="text-tertiary">流量模式</div>
                <div>{formatTrafficMode(instance.traffic_mode)}</div>
                <div className="text-tertiary">月流量</div>
                <div>{instance.monthly_traffic || 0} GB</div>
                <div className="text-tertiary">超限策略</div>
                <div>{instance.over_limit_action === 'throttle' ? `限速 ${instance.throttle_mbps || 1} Mbps` : '直接关机'}</div>
                <div className="text-tertiary">流量状态</div>
                <div>{instance.is_over_limit ? <span className="text-red-500">已超限</span> : <span className="text-green-500">正常</span>}</div>
              </div>
            </div>

            {/* 连接信息 */}
            <div className="bg-surface rounded-xl border border-surface p-5 space-y-4">
              <h3 className="text-sm font-semibold text-primary flex items-center gap-2">
                <Network size={16} /> 连接信息
              </h3>

              {/* SSH 地址 */}
              <div>
                <div className="flex items-center justify-between mb-1">
                  <span className="text-sm text-tertiary">SSH 连接地址</span>
                  <button onClick={() => copyToClipboard(`${sshAddress}:${sshPort}`)} className="text-muted hover:text-tertiary">
                    <Copy size={14} />
                  </button>
                </div>
                <div className="font-mono text-sm bg-surface-secondary px-3 py-2 rounded-lg">{sshAddress ? `${sshAddress}:${sshPort}` : '-'}</div>
              </div>

              {/* 用户名 */}
              <div>
                <div className="flex items-center justify-between mb-1">
                  <span className="text-sm text-tertiary">用户名</span>
                  <button onClick={() => copyToClipboard('root')} className="text-muted hover:text-tertiary">
                    <Copy size={14} />
                  </button>
                </div>
                <div className="font-mono text-sm bg-surface-secondary px-3 py-2 rounded-lg">root</div>
              </div>

              {/* 密码 */}
              <div>
                <div className="flex items-center justify-between mb-1">
                  <span className="text-sm text-tertiary">SSH 密码</span>
                  <div className="flex items-center gap-2">
                    <button onClick={() => setShowPassword(!showPassword)} className="text-muted hover:text-tertiary">
                      {showPassword ? <EyeOff size={14} /> : <Eye size={14} />}
                    </button>
                    {instance.ssh_password && (
                      <button onClick={() => copyToClipboard(instance.ssh_password!)} className="text-muted hover:text-tertiary">
                        <Copy size={14} />
                      </button>
                    )}
                    <button
                      onClick={() => handleAction('reset_password')}
                      disabled={isBanned || isExpired || isBusy}
                      className="text-xs text-blue-600 hover:text-blue-800 disabled:text-muted"
                    >
                      <Lock size={12} className="inline mr-1" />重置
                    </button>
                  </div>
                </div>
                <div className="font-mono text-sm bg-surface-secondary px-3 py-2 rounded-lg">
                  {showPassword ? (instance.ssh_password || '-') : '••••••••'}
                </div>
              </div>

              {/* 网络信息 */}
              <div className="pt-3 border-t border-surface-light">
                <div className="grid grid-cols-2 gap-y-2 text-sm">
                  <div className="text-tertiary">内网 IPv4</div>
                  <div className="font-mono">{stripCIDR(instance.internal_ipv4 || '') || '-'}</div>
                  <div className="text-tertiary">公网 IPv4 (EIP)</div>
                  <div className="font-mono">{stripCIDR(instance.ipv4_eip || '') || '-'}{instance.ipv4_eip_alias ? ` (${instance.ipv4_eip_alias})` : ''}</div>
                  <div className="text-tertiary">公网 IPv6 (EIP)</div>
                  <div className="font-mono">{stripCIDR(instance.ipv6_eip || '') || '-'}{instance.ipv6_eip_alias ? ` (${instance.ipv6_eip_alias})` : ''}</div>
                </div>
              </div>

              {instance.bridge_id && (
                <div className="pt-3 border-t border-surface-light">
                  <div className="text-sm font-medium text-primary mb-2">Bridge 网络</div>
                  <div className="grid grid-cols-2 gap-y-2 text-sm">
                    <div className="text-tertiary">名称</div>
                    <div>{instance.bridge_name || '-'}</div>
                    <div className="text-tertiary">接口</div>
                    <div className="font-mono">{instance.bridge_iface || '-'}</div>
                    <div className="text-tertiary">CIDR</div>
                    <div className="font-mono">{instance.bridge_cidr || '-'}</div>
                    <div className="text-tertiary">网关</div>
                    <div className="font-mono">{instance.bridge_gateway || '-'}</div>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* 监控 Tab */}
      {currentTab === 'monitoring' && (
        <div className="space-y-4">
          {/* 使用率卡片 */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div className="bg-surface rounded-xl border border-surface p-4">
              <div className="text-xs uppercase tracking-widest text-tertiary font-semibold">CPU 使用率</div>
              <div className="text-2xl font-bold mt-2">{cpuPercent.toFixed(1)}%</div>
              <div className="h-1.5 bg-surface-secondary rounded-full overflow-hidden mt-3">
                <div className="h-full bg-gradient-to-r from-teal-500 to-orange-500 rounded-full" style={{ width: `${Math.min(cpuPercent, 100)}%` }} />
              </div>
            </div>
            <div className="bg-surface rounded-xl border border-surface p-4">
              <div className="text-xs uppercase tracking-widest text-tertiary font-semibold">内存使用率</div>
              <div className="text-2xl font-bold mt-2">{memPercent.toFixed(1)}%</div>
              <div className="text-xs text-tertiary mt-1">{formatBytes(metrics?.memory_usage || 0)} / {formatBytes(metrics?.memory_total || 0)}</div>
              <div className="h-1.5 bg-surface-secondary rounded-full overflow-hidden mt-3">
                <div className="h-full bg-gradient-to-r from-teal-500 to-orange-500 rounded-full" style={{ width: `${Math.min(memPercent, 100)}%` }} />
              </div>
            </div>
            <div className="bg-surface rounded-xl border border-surface p-4">
              <div className="text-xs uppercase tracking-widest text-tertiary font-semibold">磁盘 IO</div>
              <div className="flex items-end gap-4 mt-2">
                <div>
                  <div className="text-xs text-tertiary">读</div>
                  <div className="text-lg font-bold">{formatSpeed(metrics?.disk_read_bps || 0)}</div>
                  <div className="text-xs text-tertiary">{metrics?.disk_read_iops || 0} IOPS</div>
                </div>
                <div>
                  <div className="text-xs text-tertiary">写</div>
                  <div className="text-lg font-bold">{formatSpeed(metrics?.disk_write_bps || 0)}</div>
                  <div className="text-xs text-tertiary">{metrics?.disk_write_iops || 0} IOPS</div>
                </div>
              </div>
            </div>
            <div className="bg-surface rounded-xl border border-surface p-4">
              <div className="text-xs uppercase tracking-widest text-tertiary font-semibold">网络 IO</div>
              <div className="text-2xl font-bold mt-2">{formatSpeed(metrics?.network_rx || 0)}</div>
              <div className="text-xs text-tertiary mt-1">上传: {formatSpeed(metrics?.network_tx || 0)}</div>
            </div>
          </div>

          {/* 图表区域 */}
          <div className="bg-surface rounded-xl border border-surface p-5">
            <div className="flex items-center justify-between mb-4">
              <div className="text-sm font-bold text-primary">资源使用图表</div>
              <div className="flex gap-1 bg-surface-secondary rounded-full p-1">
                {TIME_RANGES.map(p => (
                  <button
                    key={p.key}
                    onClick={() => setMetricPeriod(p.key)}
                    className={`px-3 py-1 text-xs font-semibold rounded-full transition-colors ${
                      metricPeriod === p.key
                        ? 'bg-blue-600 text-white'
                        : 'text-tertiary hover:text-secondary'
                    }`}
                  >
                    {p.label}
                  </button>
                ))}
              </div>
            </div>

            {metricLoading && metricHistory.length === 0 ? (
              <div className="h-64 flex items-center justify-center">
                <div className="animate-spin rounded-full h-6 w-6 border-2 border-blue-600 border-t-transparent" />
              </div>
            ) : metricHistory.length === 0 ? (
              <div className="h-64 flex items-center justify-center text-muted text-sm">
                暂无监控数据
              </div>
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {/* CPU 图表 */}
                <div className="bg-surface border border-surface rounded-lg p-4">
                  <div className="text-sm font-semibold text-primary mb-2">CPU 使用率</div>
                  <div className="h-44">
                    <ResponsiveContainer width="100%" height="100%">
                      <AreaChart data={metricHistory} margin={{ top: 5, right: 5, bottom: 0, left: 0 }}>
                        <defs>
                          <linearGradient id="cpuGradient" x1="0" y1="0" x2="0" y2="1">
                            <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.3} />
                            <stop offset="95%" stopColor="#3b82f6" stopOpacity={0} />
                          </linearGradient>
                        </defs>
                        <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
                        <XAxis dataKey="timestamp" tickFormatter={(t) => formatTimeByPeriod(t, metricPeriod)} tick={{ fontSize: 10 }} stroke="#888" />
                        <YAxis domain={[0, 100]} tickFormatter={(v) => `${v}%`} tick={{ fontSize: 10 }} stroke="#888" width={40} />
                        <Tooltip
                          formatter={(value: any) => [`${Number(value).toFixed(1)}%`, 'CPU']}
                          labelFormatter={(label) => new Date(label).toLocaleString('zh-CN')}
                          contentStyle={tooltipStyle}
                        />
                        <Area type="monotone" dataKey="cpu" stroke="#3b82f6" fill="url(#cpuGradient)" strokeWidth={2} />
                      </AreaChart>
                    </ResponsiveContainer>
                  </div>
                </div>

                {/* 内存图表 */}
                <div className="bg-surface border border-surface rounded-lg p-4">
                  <div className="text-sm font-semibold text-primary mb-2">内存使用</div>
                  <div className="h-44">
                    <ResponsiveContainer width="100%" height="100%">
                      <AreaChart data={metricHistory} margin={{ top: 5, right: 5, bottom: 0, left: 0 }}>
                        <defs>
                          <linearGradient id="memGradient" x1="0" y1="0" x2="0" y2="1">
                            <stop offset="5%" stopColor="#8b5cf6" stopOpacity={0.3} />
                            <stop offset="95%" stopColor="#8b5cf6" stopOpacity={0} />
                          </linearGradient>
                        </defs>
                        <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
                        <XAxis dataKey="timestamp" tickFormatter={(t) => formatTimeByPeriod(t, metricPeriod)} tick={{ fontSize: 10 }} stroke="#888" />
                        <YAxis domain={[0, 'auto']} tickFormatter={(v) => formatBytes(v)} tick={{ fontSize: 10 }} stroke="#888" width={50} />
                        <Tooltip
                          formatter={(value: any) => [formatBytes(Number(value)), '内存']}
                          labelFormatter={(label) => new Date(label).toLocaleString('zh-CN')}
                          contentStyle={tooltipStyle}
                        />
                        <Area type="monotone" dataKey="mem_used" stroke="#8b5cf6" fill="url(#memGradient)" strokeWidth={2} />
                      </AreaChart>
                    </ResponsiveContainer>
                  </div>
                </div>

                {/* 磁盘 IO 图表 */}
                <div className="bg-surface border border-surface rounded-lg p-4">
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-sm font-semibold text-primary">磁盘 IO 速度</span>
                    <div className="flex gap-2 text-xs text-tertiary">
                      <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-blue-500" />读</span>
                      <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-orange-500" />写</span>
                    </div>
                  </div>
                  <div className="h-32">
                    <ResponsiveContainer width="100%" height="100%">
                      <LineChart data={metricHistory} margin={{ top: 5, right: 5, bottom: 0, left: 0 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
                        <XAxis dataKey="timestamp" tickFormatter={(t) => formatTimeByPeriod(t, metricPeriod)} tick={{ fontSize: 10 }} stroke="#888" />
                        <YAxis domain={[0, 'auto']} tickFormatter={(v) => formatSpeed(v)} tick={{ fontSize: 10 }} stroke="#888" width={55} />
                        <Tooltip
                          formatter={(value: any, name: any) => [formatSpeed(Number(value)), name === 'disk_read_bps' ? '读速度' : '写速度']}
                          labelFormatter={(label) => new Date(label).toLocaleString('zh-CN')}
                          contentStyle={tooltipStyle}
                        />
                        <Line type="monotone" dataKey="disk_read_bps" stroke="#3b82f6" strokeWidth={2} dot={false} />
                        <Line type="monotone" dataKey="disk_write_bps" stroke="#f97316" strokeWidth={2} dot={false} />
                      </LineChart>
                    </ResponsiveContainer>
                  </div>
                  <div className="flex items-center justify-between mb-2 mt-3">
                    <span className="text-sm font-semibold text-primary">磁盘 IOPS</span>
                    <div className="flex gap-2 text-xs text-tertiary">
                      <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-blue-500" />读</span>
                      <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-orange-500" />写</span>
                    </div>
                  </div>
                  <div className="h-32">
                    <ResponsiveContainer width="100%" height="100%">
                      <LineChart data={metricHistory} margin={{ top: 5, right: 5, bottom: 0, left: 0 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
                        <XAxis dataKey="timestamp" tickFormatter={(t) => formatTimeByPeriod(t, metricPeriod)} tick={{ fontSize: 10 }} stroke="#888" />
                        <YAxis domain={[0, 'auto']} tickFormatter={(v) => `${v} IOPS`} tick={{ fontSize: 10 }} stroke="#888" width={55} />
                        <Tooltip
                          formatter={(value: any, name: any) => [`${Number(value)} IOPS`, name === 'disk_read_iops' ? '读IOPS' : '写IOPS']}
                          labelFormatter={(label) => new Date(label).toLocaleString('zh-CN')}
                          contentStyle={tooltipStyle}
                        />
                        <Line type="monotone" dataKey="disk_read_iops" stroke="#3b82f6" strokeWidth={2} dot={false} />
                        <Line type="monotone" dataKey="disk_write_iops" stroke="#f97316" strokeWidth={2} dot={false} />
                      </LineChart>
                    </ResponsiveContainer>
                  </div>
                </div>

                {/* 网络 IO 图表 */}
                <div className="bg-surface border border-surface rounded-lg p-4">
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-sm font-semibold text-primary">网络 IO</span>
                    <div className="flex gap-2 text-xs text-tertiary">
                      <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-teal-500" />下载</span>
                      <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-orange-500" />上传</span>
                    </div>
                  </div>
                  <div className="h-44">
                    <ResponsiveContainer width="100%" height="100%">
                      <LineChart data={metricHistory} margin={{ top: 5, right: 5, bottom: 0, left: 0 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
                        <XAxis dataKey="timestamp" tickFormatter={(t) => formatTimeByPeriod(t, metricPeriod)} tick={{ fontSize: 10 }} stroke="#888" />
                        <YAxis domain={[0, 'auto']} tickFormatter={(v) => formatSpeed(v)} tick={{ fontSize: 10 }} stroke="#888" width={55} />
                        <Tooltip
                          formatter={(value: any, name: any) => [formatSpeed(Number(value)), name === 'net_in' ? '下载' : '上传']}
                          labelFormatter={(label) => new Date(label).toLocaleString('zh-CN')}
                          contentStyle={tooltipStyle}
                        />
                        <Line type="monotone" dataKey="net_in" stroke="#14b8a6" strokeWidth={2} dot={false} />
                        <Line type="monotone" dataKey="net_out" stroke="#f97316" strokeWidth={2} dot={false} />
                      </LineChart>
                    </ResponsiveContainer>
                  </div>
                </div>
              </div>
            )}
            <div className="text-right text-xs text-muted mt-3">
              {metricPeriod === '1m' ? 'WebSocket 实时刷新（每秒）' : '30秒自动刷新'} · 最后更新: {new Date().toLocaleTimeString()}
            </div>
          </div>
        </div>
      )}

      {/* 端口映射 Tab */}
      {currentTab === 'portMapping' && (
        <div className="space-y-4">
          <div className="bg-surface rounded-xl border border-surface overflow-hidden">
            <div className="px-4 py-3 bg-surface-secondary border-b border-surface flex items-center justify-between">
              <h4 className="text-sm font-semibold text-primary">端口映射列表</h4>
              <span className="text-xs text-tertiary">
                已用 <span className="font-medium text-primary">{instance.port_mappings?.length || 0}</span>
                {instance.port_mapping_limit ? <> / <span className="font-medium text-primary">{instance.port_mapping_limit}</span></> : ''} 个
              </span>
            </div>
            {instance.port_mappings && instance.port_mappings.length > 0 ? (
              <div className="divide-y divide-surface-light">
                {instance.port_mappings.map((pm) => (
                  <div key={pm.id} className="flex items-center gap-3 px-4 py-2.5 bg-surface-hover transition-colors">
                    <span className="px-1.5 py-0.5 bg-blue-50 text-blue-600 text-[11px] font-semibold rounded uppercase flex-shrink-0">{pm.protocol}</span>
                    <div className="flex items-center gap-1.5 font-mono text-sm">
                      <span className="text-primary">{pm.host_port}</span>
                      <ChevronRight size={12} className="text-muted" />
                      <span className="text-primary">{pm.container_port}</span>
                    </div>
                    {pm.description && (
                      <span className="text-xs text-tertiary bg-surface-secondary px-1.5 py-0.5 rounded truncate max-w-[120px]" title={pm.description}>{pm.description}</span>
                    )}
                    <div className="flex-1" />
                    <button
                      onClick={() => handleDeletePortMapping(pm.id)}
                      className="p-1 text-muted hover:text-red-500 hover:bg-red-50 rounded-md transition-colors"
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                ))}
              </div>
            ) : (
              <div className="flex flex-col items-center py-8 text-muted">
                <Globe size={28} className="mb-2 opacity-30" />
                <span className="text-sm">暂无端口映射</span>
              </div>
            )}
          </div>

          {/* 添加端口映射 */}
          <div className="bg-surface rounded-xl border border-surface p-4 space-y-3">
            <div className="flex items-center justify-between">
              <h5 className="text-sm font-semibold text-primary">添加映射</h5>
              <button
                onClick={() => setAddingPM(!addingPM)}
                className="text-xs text-blue-600 hover:text-blue-800"
              >
                {addingPM ? '取消' : '添加'}
              </button>
            </div>
            {addingPM && (
              <PortMappingFormInline onSubmit={handleAddPortMapping} onCancel={() => setAddingPM(false)} />
            )}
          </div>
        </div>
      )}

      {/* 磁盘管理 Tab */}
      {currentTab === 'disk' && (
        <div className="space-y-4">
          {/* 系统盘 */}
          <div className="bg-surface rounded-xl border border-surface p-5">
            <h4 className="text-sm font-semibold text-primary mb-3 flex items-center gap-2">
              <HardDrive size={16} /> 系统盘
            </h4>
            <div className="flex items-center justify-between py-2 px-3 bg-surface-secondary rounded-lg">
              <div className="flex items-center gap-3">
                <HardDrive size={14} className="text-muted" />
                <span className="text-sm">系统盘</span>
                <span className="text-xs text-muted">{(instance.disk_mb / 1024).toFixed(0)} GB</span>
                <span className="text-xs text-muted">存储池: {instance.storage_pool || 'default'}</span>
              </div>
              {metrics?.disk_used !== undefined && metrics?.disk_total !== undefined && (
                <span className="text-xs text-tertiary">{formatBytes(metrics.disk_used)} / {formatBytes(metrics.disk_total)}</span>
              )}
            </div>
          </div>

          {/* 数据盘 */}
          <div className="bg-surface rounded-xl border border-surface p-5">
            <div className="flex items-center justify-between mb-4">
              <h4 className="text-sm font-semibold text-primary flex items-center gap-2">
                <HardDrive size={16} /> 数据盘
              </h4>
              <Button
                icon={<Plus size={14} />}
                size="sm"
                onClick={() => {
                  const name = prompt('请输入数据盘名称：')
                  if (!name) return
                  const sizeStr = prompt('请输入数据盘大小（GB）：')
                  if (!sizeStr) return
                  const size = parseInt(sizeStr)
                  if (!size || size < 1) {
                    toast.error('请输入有效大小')
                    return
                  }
                  apiClient.post(`/instances/${id}/disks`, { name, size_mb: size * 1024 })
                    .then(() => { toast.success('创建任务已下发'); fetchInstance() })
                    .catch((err: any) => toast.error(err.response?.data?.error || '创建失败'))
                }}
              >
                添加数据盘
              </Button>
            </div>
            {instance.data_disks && instance.data_disks.length > 0 ? (
              <div className="space-y-2">
                {instance.data_disks.map((disk) => (
                  <div key={disk.id} className="flex items-center justify-between text-sm py-2 px-3 bg-surface-secondary rounded-lg">
                    <div className="flex items-center gap-3">
                      <HardDrive size={14} className="text-muted" />
                      <span>{disk.name}</span>
                      <span className="text-xs text-muted">{(disk.size_mb / 1024).toFixed(0)} GB</span>
                      <span className="text-xs text-muted">{disk.mount_point || '未挂载'}</span>
                      {disk.status && disk.status !== 'attached' && (
                        <span className="text-xs text-amber-600">{disk.status}</span>
                      )}
                    </div>
                    <div className="flex items-center gap-2">
                      <button className="text-blue-500 hover:text-blue-700 text-xs" onClick={() => {
                        const newSize = prompt(`扩容数据盘 ${disk.name}，当前 ${(disk.size_mb / 1024).toFixed(0)}GB，请输入新大小（GB）：`)
                        if (!newSize) return
                        const size = parseInt(newSize)
                        if (!size || size * 1024 <= disk.size_mb) {
                          toast.error('新大小必须大于当前大小')
                          return
                        }
                        apiClient.put(`/instances/${id}/disks/${disk.id}`, { size_mb: size * 1024 })
                          .then(() => { toast.success('扩容任务已下发'); fetchInstance() })
                          .catch((err: any) => toast.error(err.response?.data?.error || '扩容失败'))
                      }}>扩容</button>
                      <button className="text-red-500 hover:text-red-700 text-xs" onClick={() => {
                        if (!confirm(`确认删除数据盘 ${disk.name}？数据将丢失。`)) return
                        apiClient.delete(`/instances/${id}/disks/${disk.id}`)
                          .then(() => { toast.success('删除任务已下发'); fetchInstance() })
                          .catch((err: any) => toast.error(err.response?.data?.error || '删除失败'))
                      }}>删除</button>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-sm text-muted text-center py-4">暂无数据盘</div>
            )}
          </div>
        </div>
      )}

      {/* 快照管理 Tab */}
      {currentTab === 'snapshot' && (
        <div className="bg-surface rounded-xl border border-surface p-5">
          <div className="flex items-center justify-between mb-4">
            <h4 className="text-sm font-semibold text-primary flex items-center gap-2">
              <Calendar size={16} /> 快照管理
              <span className="text-xs text-muted font-normal">
                (上限 {instance.snapshot_limit} 个)
              </span>
            </h4>
            <Button icon={<Plus size={14} />} onClick={handleCreateSnapshot} size="sm">创建快照</Button>
          </div>
          {snapshots.length > 0 ? (
            <div className="space-y-2">
              {snapshots.map((s) => (
                <div key={s.id} className="flex items-center justify-between text-sm py-2 px-3 bg-surface-secondary rounded-lg">
                  <div className="flex items-center gap-3">
                    <Calendar size={14} className="text-muted" />
                    <span>{s.name}</span>
                    <span className="text-xs text-muted">{new Date(s.created_at).toLocaleString()}</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      className="text-blue-500 hover:text-blue-700 text-xs"
                      onClick={() => handleRestoreSnapshot(s.name)}
                    >
                      恢复
                    </button>
                    <button
                      className="text-red-500 hover:text-red-700 text-xs"
                      onClick={() => handleDeleteSnapshot(s.id)}
                    >
                      删除
                    </button>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="text-sm text-muted text-center py-4">暂无快照</div>
          )}
        </div>
      )}

      {/* 重装系统 Tab */}
      {currentTab === 'reinstall' && (
        <div className="space-y-4">
          {/* 警告 */}
          <div className="bg-red-50 border border-red-200 rounded-xl p-5">
            <div className="flex items-start gap-3">
              <AlertTriangle size={20} className="text-red-500 flex-shrink-0 mt-0.5" />
              <div>
                <h4 className="text-sm font-semibold text-red-700">危险操作</h4>
                <p className="text-sm text-red-600 mt-1">
                  重装系统将删除容器内所有数据，此操作不可撤销！请提前备份重要数据。
                </p>
              </div>
            </div>
          </div>

          {/* 重装配置 */}
          <div className="bg-surface rounded-xl border border-surface p-5 space-y-4">
            {/* 选择镜像 - 两级选择 */}
            <div>
              <h4 className="text-sm font-semibold text-primary mb-2">选择目标系统</h4>
              <p className="text-xs text-tertiary mb-3">当前系统: <span className="font-mono">{instance.template_id || '-'}</span></p>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs text-tertiary mb-1">分类</label>
                  <select
                    value={reinstallCategory}
                    onChange={(e) => { setReinstallCategory(e.target.value); setReinstallImage('') }}
                    className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm"
                  >
                    <option value="">全部分类</option>
                    {reinstallCategories.map(cat => (
                      <option key={cat.id} value={cat.id}>{cat.name}</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="block text-xs text-tertiary mb-1">镜像版本</label>
                  <select
                    value={reinstallImage}
                    onChange={(e) => setReinstallImage(e.target.value)}
                    className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm"
                  >
                    <option value="">选择镜像...</option>
                    {reinstallImages
                      .filter(img => !reinstallCategory || img.category_id === reinstallCategory)
                      .map(img => (
                        <option key={img.id} value={img.alias}>
                          {img.display_name || img.alias} ({img.architecture})
                        </option>
                      ))
                    }
                  </select>
                </div>
              </div>
              {reinstallImages.length === 0 && (
                <p className="text-xs text-muted mt-2">该节点暂无已安装的镜像</p>
              )}
            </div>

            {/* SSH 凭据 */}
            <div className="pt-4 border-t border-surface-light">
              <h4 className="text-sm font-semibold text-primary mb-3">SSH 登录凭据</h4>
              <div className="flex gap-2 mb-3">
                {[
                  { key: 'auto', label: '自动生成' },
                  { key: 'password', label: '随机密码' },
                  { key: 'sshkey', label: '自定义 Key' },
                ].map(mode => (
                  <button
                    key={mode.key}
                    onClick={() => {
                      setReinstallLoginMode(mode.key as any)
                      if (mode.key === 'password') {
                        setReinstallPassword(generateRandomPassword())
                      }
                    }}
                    className={`px-4 py-2 text-sm font-semibold rounded-full border transition-all ${
                      reinstallLoginMode === mode.key
                        ? 'border-blue-600 bg-blue-600 text-white'
                        : 'border-surface text-tertiary hover:text-secondary hover:border-blue-300'
                    }`}
                  >
                    {mode.label}
                  </button>
                ))}
              </div>

              {reinstallLoginMode === 'auto' && (
                <p className="text-xs text-tertiary">系统将自动生成随机密码，重装完成后可在实例详情页查看。</p>
              )}

              {reinstallLoginMode === 'password' && (
                <div className="flex gap-2">
                  <input
                    type="text"
                    value={reinstallPassword}
                    onChange={(e) => setReinstallPassword(e.target.value)}
                    placeholder="密码"
                    className="flex-1 px-3 py-2 border border-surface-strong rounded-lg text-sm font-mono"
                  />
                  <button
                    onClick={() => setReinstallPassword(generateRandomPassword())}
                    className="px-3 py-2 text-xs bg-surface-secondary hover:bg-surface-hover rounded-lg"
                  >
                    重新生成
                  </button>
                </div>
              )}

              {reinstallLoginMode === 'sshkey' && (
                <textarea
                  value={reinstallSSHKey}
                  onChange={(e) => setReinstallSSHKey(e.target.value)}
                  placeholder="粘贴 SSH 公钥 (ssh-ed25519 AAAA... 或 ssh-rsa AAAA...)"
                  rows={4}
                  className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-mono"
                />
              )}
            </div>

            {/* 格式化数据盘 */}
            <div className="pt-4 border-t border-surface-light">
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={reinstallFormatDisks}
                  onChange={(e) => setReinstallFormatDisks(e.target.checked)}
                  className="w-4 h-4 rounded"
                />
                <span className="text-sm text-secondary">同时格式化所有数据盘</span>
              </label>
            </div>

            {/* 确认按钮 */}
            <div className="pt-4 border-t border-surface-light">
              <button
                onClick={() => setReinstallDialogOpen(true)}
                disabled={!reinstallImage || isBanned || isExpired || isBusy}
                className="px-6 py-2.5 rounded-lg bg-red-600 text-white hover:bg-red-700 font-semibold text-sm disabled:opacity-50 disabled:cursor-not-allowed"
              >
                确认重装
              </button>
            </div>
          </div>
        </div>
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

function PortMappingFormInline({ onSubmit, onCancel }: { onSubmit: (cp: number, hp: number | null, proto: string, desc: string) => void; onCancel: () => void }) {
  const [containerPort, setContainerPort] = useState('')
  const [hostPort, setHostPort] = useState('')
  const [protocol, setProtocol] = useState('tcp')
  const [description, setDescription] = useState('')

  return (
    <div className="space-y-2">
      <div className="grid grid-cols-4 gap-2">
        <div>
          <label className="block text-xs text-tertiary mb-1">内部端口</label>
          <input type="number" value={containerPort} onChange={(e) => setContainerPort(e.target.value)} className="w-full px-2 py-1.5 border border-surface-strong rounded text-sm" placeholder="80" />
        </div>
        <div>
          <label className="block text-xs text-tertiary mb-1">外部端口</label>
          <input type="number" value={hostPort} onChange={(e) => setHostPort(e.target.value)} className="w-full px-2 py-1.5 border border-surface-strong rounded text-sm" placeholder="留空自动分配" />
        </div>
        <div>
          <label className="block text-xs text-tertiary mb-1">协议</label>
          <select value={protocol} onChange={(e) => setProtocol(e.target.value)} className="w-full px-2 py-1.5 border border-surface-strong rounded text-sm">
            <option value="tcp">TCP</option>
            <option value="udp">UDP</option>
            <option value="both">TCP/UDP</option>
          </select>
        </div>
        <div>
          <label className="block text-xs text-tertiary mb-1">备注</label>
          <input value={description} onChange={(e) => setDescription(e.target.value)} className="w-full px-2 py-1.5 border border-surface-strong rounded text-sm" placeholder="HTTP" />
        </div>
      </div>
      <div className="flex justify-end gap-2">
        <button className="text-xs text-tertiary hover:text-secondary" onClick={onCancel}>取消</button>
        <button className="text-xs bg-blue-600 text-white px-3 py-1.5 rounded hover:bg-blue-700" onClick={() => {
          const cp = parseInt(containerPort)
          if (!cp || cp < 1 || cp > 65535) return
          const hp = hostPort ? parseInt(hostPort) : null
          if (hp && (hp < 1 || hp > 65535)) return
          onSubmit(cp, hp, protocol, description)
        }}>确认</button>
      </div>
    </div>
  )
}
