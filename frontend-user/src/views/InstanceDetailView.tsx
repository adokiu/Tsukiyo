import { useEffect, useState, useRef, useCallback } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import {
  ArrowLeft, Play, Square, RotateCw, Trash2, Terminal, Monitor, Loader2,
} from 'lucide-react'
import { instancesApi, InstanceDetail, InstanceMetricsData, MetricPoint, Snapshot, Category, InstalledImage } from '@/api/instances'
import { useToastStore } from '@/stores/toast'
import { useTranslation } from 'react-i18next'
import { generateRandomPassword } from '@/utils/format'
import { useUserMetricsWs } from '@/contexts/UserMetricsWsContext'
import { OverviewTab } from '@/components/InstanceDetail/OverviewTab'
import { MonitoringTab } from '@/components/InstanceDetail/MonitoringTab'
import { PortMappingTab } from '@/components/InstanceDetail/PortMappingTab'
import { DiskTab } from '@/components/InstanceDetail/DiskTab'
import { SnapshotTab } from '@/components/InstanceDetail/SnapshotTab'
import { ReinstallTab } from '@/components/InstanceDetail/ReinstallTab'
import { Modal } from '@/components/Modal/Modal'
import { Button } from '@/components/Button/Button'

type TabKey = 'overview' | 'monitoring' | 'portMapping' | 'disk' | 'snapshot' | 'reinstall'

export function InstanceDetailView() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { t } = useTranslation()
  const toast = useToastStore()

  const [instance, setInstance] = useState<InstanceDetail | null>(null)
  const [metrics, setMetrics] = useState<InstanceMetricsData | null>(null)
  const [loading, setLoading] = useState(true)
  const [actionLoading, setActionLoading] = useState(false)
  const [showPassword, setShowPassword] = useState(false)
  const [snapshots, setSnapshots] = useState<Snapshot[]>([])
  const [currentTab, setCurrentTab] = useState<TabKey>('overview')

  const [metricHistory, setMetricHistory] = useState<MetricPoint[]>([])
  const [metricPeriod, setMetricPeriod] = useState('1m')
  const [metricLoading, setMetricLoading] = useState(false)
  const { metrics: wsMetricsMap, connected: wsConnected } = useUserMetricsWs()
  const realtimeDataRef = useRef<MetricPoint[]>([])

  const [reinstallCategories, setReinstallCategories] = useState<Category[]>([])
  const [reinstallImages, setReinstallImages] = useState<InstalledImage[]>([])
  const [reinstallCategory, setReinstallCategory] = useState('')
  const [reinstallImage, setReinstallImage] = useState('')
  const [reinstallDialogOpen, setReinstallDialogOpen] = useState(false)
  const [reinstallLoginMode, setReinstallLoginMode] = useState<'auto' | 'password' | 'sshkey'>('password')
  const [reinstallPassword, setReinstallPassword] = useState(() => generateRandomPassword())
  const [reinstallSSHKey, setReinstallSSHKey] = useState('')
  const [reinstallFormatDisks, setReinstallFormatDisks] = useState(false)

  const [resetPwdDialogOpen, setResetPwdDialogOpen] = useState(false)
  const [resetPwdValue, setResetPwdValue] = useState('')

  const fetchInstance = async () => {
    if (!id) return
    try {
      const res = await instancesApi.get(id)
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
      const res = await instancesApi.metrics(id)
      setMetrics(res.data)
    } catch {
      // 忽略
    }
  }

  const fetchMetricHistory = useCallback(async () => {
    if (!id) return
    setMetricLoading(true)
    try {
      const res = await instancesApi.metricsHistory(id, metricPeriod)
      setMetricHistory(res.data.data || [])
    } catch {
      setMetricHistory([])
    } finally {
      setMetricLoading(false)
    }
  }, [id, metricPeriod])

  const fetchSnapshots = async () => {
    if (!id) return
    try {
      const res = await instancesApi.snapshots(id)
      setSnapshots(res.data.data || [])
    } catch {
      setSnapshots([])
    }
  }

  useEffect(() => {
    fetchInstance()
    fetchMetrics()
    fetchSnapshots()
    const interval = setInterval(() => {
      if (!wsConnected) {
        fetchMetrics()
        if (metricPeriod !== '1m') fetchMetricHistory()
      }
    }, 5000)
    return () => clearInterval(interval)
  }, [id, metricPeriod, wsConnected])

  useEffect(() => {
    fetchMetricHistory()
  }, [fetchMetricHistory])

  useEffect(() => {
    if (!id) return
    const item = wsMetricsMap[id]
    if (!item) return

    setMetrics({
      cpu_usage: item.cpu_usage,
      memory_usage: item.memory_used,
      memory_total: item.memory_total,
      memory_used: item.memory_used,
      disk_used: item.disk_used,
      disk_total: item.disk_total,
      network_rx: item.net_in_bps,
      network_tx: item.net_out_bps,
    })

    if (metricPeriod !== '1m') return

    const point: MetricPoint = {
      timestamp: new Date().toISOString(),
      cpu: item.cpu_usage || 0,
      mem_used: item.memory_used || 0,
      mem_total: item.memory_total || 0,
      disk_used: item.disk_used || 0,
      disk_total: item.disk_total || 0,
      disk_read_bps: 0,
      disk_write_bps: 0,
      net_in: item.net_in_bps || 0,
      net_out: item.net_out_bps || 0,
    }
    const newData = [...realtimeDataRef.current, point].slice(-120)
    realtimeDataRef.current = newData
    setMetricHistory(newData)
  }, [id, metricPeriod, wsMetricsMap])

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
        await instancesApi.delete(id)
        toast.success('删除任务已下发')
        navigate('/instances')
        return
      }
      if (action === 'terminal') {
        try {
          const res = await instancesApi.console(id, 'ssh')
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
          const res = await instancesApi.console(id, 'vnc')
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
      await instancesApi[action as 'start' | 'stop' | 'restart'](id)
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
      const body: { password?: string } = {}
      if (resetPwdValue) body.password = resetPwdValue
      await instancesApi.resetPassword(id, body)
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
      await instancesApi.reinstall(id, body)
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

  const handleCreateSnapshot = async () => {
    if (!id) return
    const name = prompt('请输入快照名称：')
    if (!name) return
    try {
      await instancesApi.createSnapshot(id, { name })
      toast.success('快照创建任务已下发')
      fetchSnapshots()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '创建失败')
    }
  }

  const handleRestoreSnapshot = async (snapshotName: string) => {
    if (!id) return
    if (!confirm(`确认恢复到快照 ${snapshotName}？当前数据将被覆盖。`)) return
    try {
      await instancesApi.restoreSnapshot(id, snapshotName)
      toast.success('快照恢复任务已下发')
      fetchInstance()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '恢复失败')
    }
  }

  const handleDeleteSnapshot = async (snapshotID: string) => {
    if (!id) return
    if (!confirm('确认删除该快照？')) return
    const snap = snapshots.find(s => s.id === snapshotID)
    if (!snap) return
    try {
      await instancesApi.deleteSnapshot(id, snap.name)
      toast.success('快照删除成功')
      fetchSnapshots()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '删除失败')
    }
  }

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
    toast.success('已复制到剪贴板')
  }

  if (loading) {
    return (
      <div className="page-container flex justify-center py-12">
        <Loader2 className="animate-spin text-muted-foreground" size={24} />
      </div>
    )
  }

  if (!instance) {
    return (
      <div className="page-container">
        <Link to="/instances" className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-4">
          <ArrowLeft size={16} /> 返回实例列表
        </Link>
        <div className="text-center text-muted-foreground">实例不存在</div>
      </div>
    )
  }

  const isBusy = ['creating', 'starting', 'stopping', 'restarting', 'reinstalling', 'resizing', 'deleting'].includes(instance.status)

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

  const statusColors: Record<string, string> = {
    running: 'text-green-600 bg-green-100',
    stopped: 'text-gray-500 bg-gray-100',
    creating: 'text-blue-600 bg-blue-100',
    error: 'text-red-600 bg-red-100',
    starting: 'text-blue-600 bg-blue-100',
    stopping: 'text-blue-600 bg-blue-100',
    restarting: 'text-blue-600 bg-blue-100',
    reinstalling: 'text-blue-600 bg-blue-100',
  }

  return (
    <div className="page-container">
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Link to="/instances" className="text-muted-foreground hover:text-foreground">
              <ArrowLeft size={20} />
            </Link>
            <div>
              <h1 className="text-xl font-semibold">{instance.name}</h1>
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <span className="font-mono">{instance.incus_name}</span>
                <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${statusColors[instance.status] || ''}`}>
                  {t(`common.${instance.status}`, instance.status)}
                </span>
                <span className="font-mono text-xs">{instance.type === 'container' ? '容器' : '虚拟机'}</span>
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            {instance.status === 'stopped' && (
              <button onClick={() => handleAction('start')} disabled={actionLoading} className="btn btn--primary btn--md disabled:opacity-50">
                {actionLoading ? <Loader2 size={14} className="btn__spinner" /> : <Play size={14} />}
                <span>启动</span>
              </button>
            )}
            {instance.status === 'running' && (
              <button onClick={() => handleAction('stop')} disabled={actionLoading} className="btn btn--secondary btn--md disabled:opacity-50">
                {actionLoading ? <Loader2 size={14} className="btn__spinner" /> : <Square size={14} />}
                <span>停止</span>
              </button>
            )}
            {!isBusy && (
              <button onClick={() => handleAction('restart')} disabled={actionLoading} className="btn btn--secondary btn--md disabled:opacity-50">
                {actionLoading ? <Loader2 size={14} className="btn__spinner" /> : <RotateCw size={14} />}
                <span>重启</span>
              </button>
            )}
            {!isBusy && (
              <button onClick={() => handleAction('terminal')} disabled={actionLoading} className="btn btn--secondary btn--md disabled:opacity-50">
                <Terminal size={14} />
                <span>终端</span>
              </button>
            )}
            {!isBusy && instance.type === 'vm' && (
              <button onClick={() => handleAction('vnc')} disabled={actionLoading} className="btn btn--secondary btn--md disabled:opacity-50">
                <Monitor size={14} />
                <span>VNC</span>
              </button>
            )}
            <button onClick={() => handleAction('delete')} disabled={actionLoading} className="btn btn--danger btn--md disabled:opacity-50">
              {actionLoading ? <Loader2 size={14} className="btn__spinner" /> : <Trash2 size={14} />}
              <span>删除</span>
            </button>
          </div>
        </div>

        <div className="flex flex-wrap gap-2 card p-3">
          {tabs.map(tab => (
            <button
              key={tab.key}
              onClick={() => setCurrentTab(tab.key)}
              className={`px-4 py-2 text-sm font-semibold rounded-full border transition-all ${
                currentTab === tab.key
                  ? 'border-blue-600 bg-blue-600 text-white shadow-sm'
                  : 'border-gray-200 text-muted-foreground hover:text-foreground hover:border-blue-300'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>

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
            isBusy={isBusy}
          />
        )}

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

        {currentTab === 'portMapping' && (
          <PortMappingTab
            portMappings={instance.port_mappings || []}
            portMappingLimit={instance.port_mapping_limit}
            onAdd={() => { toast.info('端口映射功能开发中') }}
            onDelete={() => { toast.info('端口映射功能开发中') }}
          />
        )}

        {currentTab === 'disk' && (
          <DiskTab
            diskMB={instance.disk_mb}
            storagePool={instance.storage_pool}
            dataDisks={instance.data_disks || []}
            metrics={metrics}
          />
        )}

        {currentTab === 'snapshot' && (
          <SnapshotTab
            snapshotLimit={instance.snapshot_limit}
            snapshots={snapshots}
            onCreate={handleCreateSnapshot}
            onRestore={handleRestoreSnapshot}
            onDelete={handleDeleteSnapshot}
          />
        )}

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
            isBusy={isBusy}
          />
        )}

        <Modal
          open={reinstallDialogOpen}
          onClose={() => setReinstallDialogOpen(false)}
          title="确认重装系统"
          confirmMode
          confirmText="确认重装"
          confirmVariant="danger"
          onConfirm={handleReinstall}
          width={440}
        >
          重装系统将删除容器内所有数据，此操作不可撤销！
          {reinstallFormatDisks && ' 所有数据盘也将被格式化。'}
        </Modal>

        <Modal
          open={resetPwdDialogOpen}
          onClose={() => setResetPwdDialogOpen(false)}
          title="重置 SSH 密码"
          width={440}
          footer={
            <>
              <Button variant="ghost" onClick={() => setResetPwdDialogOpen(false)}>取消</Button>
              <Button variant="primary" onClick={handleResetPassword}>确认重置</Button>
            </>
          }
        >
          <div className="space-y-3">
            <p className="text-sm text-muted-foreground">留空则自动生成随机密码。</p>
            <div className="flex gap-2">
              <input
                type="text"
                value={resetPwdValue}
                onChange={(e) => setResetPwdValue(e.target.value)}
                placeholder="输入新密码（留空自动生成）"
                className="flex-1 px-3 py-2 border border-gray-200 rounded-lg text-sm font-mono"
              />
              <button
                onClick={() => setResetPwdValue(generateRandomPassword())}
                className="px-3 py-2 text-xs bg-gray-100 hover:bg-gray-200 rounded-lg"
              >
                随机生成
              </button>
            </div>
          </div>
        </Modal>
      </div>
    </div>
  )
}
