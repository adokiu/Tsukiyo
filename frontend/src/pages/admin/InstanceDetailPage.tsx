import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import {
  ArrowLeft, Play, Square, RotateCw, Trash2, Terminal, Lock,
  RefreshCw, HardDrive, Cpu, MemoryStick, Network, Gauge,
  Calendar, Server, Eye, EyeOff, Copy, Plus
} from 'lucide-react'
import apiClient from '@/api/client'
import { Button } from '@/components/Button/Button'
import { useToastStore } from '@/stores/toast'

interface Instance {
  id: string
  name: string
  type: string
  status: string
  node_id: string
  incus_name: string
  template_id: string
  vcpu: number
  memory_mb: number
  disk_gb: number
  storage_pool: string
  internal_ipv4?: string
  ipv4_address?: string
  ipv6_address?: string
  login_method: string
  ssh_port?: number
  ssh_password?: string
  ssh_public_key?: string
  network_down?: number
  network_up?: number
  io_read?: number
  io_write?: number
  monthly_traffic?: number
  traffic_mode: string
  snapshot_limit: number
  bridge_id?: string
  bridge_name?: string
  bridge_cidr?: string
  bridge_gateway?: string
  internal_ipv6?: string
  ipv4_eip?: string
  ipv6_eip?: string
  created_at: string
  expires_at?: string
  port_mappings?: PortMapping[]
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
  disk_usage?: number
  disk_total?: number
  network_rx?: number
  network_tx?: number
}

function PortMappingForm({ onSubmit, onCancel }: { onSubmit: (cp: number, hp: number | null, proto: string, desc: string) => void; onCancel: () => void }) {
  const [containerPort, setContainerPort] = useState('')
  const [hostPort, setHostPort] = useState('')
  const [protocol, setProtocol] = useState('tcp')
  const [description, setDescription] = useState('')

  return (
    <div className="space-y-2 mb-3 p-3 bg-gray-50 rounded-lg">
      <div className="grid grid-cols-4 gap-2">
        <div>
          <label className="block text-xs text-gray-500 mb-1">内部端口</label>
          <input type="number" value={containerPort} onChange={(e) => setContainerPort(e.target.value)} className="w-full px-2 py-1.5 border border-gray-300 rounded text-sm" placeholder="80" />
        </div>
        <div>
          <label className="block text-xs text-gray-500 mb-1">外部端口</label>
          <input type="number" value={hostPort} onChange={(e) => setHostPort(e.target.value)} className="w-full px-2 py-1.5 border border-gray-300 rounded text-sm" placeholder="留空自动分配" />
        </div>
        <div>
          <label className="block text-xs text-gray-500 mb-1">协议</label>
          <select value={protocol} onChange={(e) => setProtocol(e.target.value)} className="w-full px-2 py-1.5 border border-gray-300 rounded text-sm">
            <option value="tcp">TCP</option>
            <option value="udp">UDP</option>
            <option value="both">TCP/UDP</option>
          </select>
        </div>
        <div>
          <label className="block text-xs text-gray-500 mb-1">备注</label>
          <input value={description} onChange={(e) => setDescription(e.target.value)} className="w-full px-2 py-1.5 border border-gray-300 rounded text-sm" placeholder="HTTP" />
        </div>
      </div>
      <div className="flex justify-end gap-2">
        <button className="text-xs text-gray-500 hover:text-gray-700" onClick={onCancel}>取消</button>
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

export default function InstanceDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const toast = useToastStore()

  const [instance, setInstance] = useState<Instance | null>(null)
  const [metrics, setMetrics] = useState<Metrics | null>(null)
  const [loading, setLoading] = useState(true)
  const [actionLoading, setActionLoading] = useState(false)
  const [showPassword, setShowPassword] = useState(false)
  const [snapshots, setSnapshots] = useState<Snapshot[]>([])
  const [addingPM, setAddingPM] = useState(false)

  const fetchInstance = async () => {
    if (!id) return
    setLoading(true)
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

  const fetchSnapshots = async () => {
    if (!id) return
    try {
      const res = await apiClient.get(`/instances/${id}/snapshots`)
      setSnapshots(res.data.data || [])
    } catch {
      setSnapshots([])
    }
  }

  useEffect(() => {
    fetchInstance()
    fetchMetrics()
    fetchSnapshots()
    const interval = setInterval(fetchMetrics, 5000)
    return () => clearInterval(interval)
  }, [id])

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
      if (action === 'console') {
        try {
          const res = await apiClient.get(`/instances/${id}/console?type=ssh`)
          if (res.data.agent_url) {
            window.open(res.data.agent_url, '_blank')
          } else if (res.data.token) {
            window.open(`/console?token=${res.data.token}`, '_blank')
          }
        } catch (err: any) {
          if (err.response?.status === 503) {
            toast.error('节点离线，无法连接控制台')
          } else {
            toast.error(err.response?.data?.error || '获取控制台信息失败')
          }
        }
        return
      }
      if (action === 'reset_password') {
        const newPwd = prompt('请输入新密码（留空则自动生成）：')
        if (newPwd === null) return
        try {
          const body: Record<string, string> = {}
          if (newPwd) body.password = newPwd
          await apiClient.post(`/instances/${id}/reset-password`, body)
          toast.success('密码重置成功')
          fetchInstance()
        } catch (err: any) {
          if (err.response?.status === 503) {
            toast.error('节点离线，无法重置密码')
          } else if (err.response?.status === 504) {
            toast.error('操作超时')
          } else if (err.response?.status === 409) {
            toast.error('实例正在执行其他操作')
          } else {
            toast.error(err.response?.data?.error || '密码重置失败')
          }
        }
        return
      }
      if (action === 'reinstall') {
        if (!confirm('确认重装系统？所有数据将丢失。')) return
        try {
          await apiClient.post(`/instances/${id}/reinstall`, {
            template_id: instance?.template_id || '',
          })
          toast.success('重装任务已下发')
          fetchInstance()
        } catch (err: any) {
          if (err.response?.status === 409) {
            toast.error('实例正在执行其他操作，请稍后重试')
          } else {
            toast.error(err.response?.data?.error || '重装失败')
          }
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

  const handleAddPortMapping = async (containerPort: number, hostPort: number | null, protocol: string, description: string) => {
    if (!id) return
    try {
      const body: any = {
        instance_id: id,
        container_port: containerPort,
        protocol,
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
        <button onClick={() => navigate('/admin/instanceManagement/container')} className="flex items-center gap-1 text-sm text-gray-500 hover:text-gray-700 mb-4">
          <ArrowLeft size={16} /> 返回实例列表
        </button>
        <div className="text-center text-gray-500">实例不存在</div>
      </div>
    )
  }

  const statusColor = instance.status === 'running' ? 'text-green-600' : instance.status === 'stopped' ? 'text-gray-500' : 'text-amber-600'
  const statusBg = instance.status === 'running' ? 'bg-green-100' : instance.status === 'stopped' ? 'bg-gray-100' : 'bg-amber-100'

  return (
    <div className="p-6 space-y-6">
      {/* 顶部导航 */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <button onClick={() => navigate('/admin/instanceManagement/container')} className="text-gray-500 hover:text-gray-700">
            <ArrowLeft size={20} />
          </button>
          <div>
            <h1 className="text-xl font-semibold text-black">{instance.name}</h1>
            <div className="flex items-center gap-2 text-sm text-gray-500">
              <span className="font-mono">{instance.incus_name}</span>
              <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${statusBg} ${statusColor}`}>{instance.status}</span>
              <span className="font-mono text-xs">{instance.type}</span>
            </div>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {instance.status === 'stopped' && (
            <Button icon={<Play size={14} />} onClick={() => handleAction('start')} loading={actionLoading}>启动</Button>
          )}
          {instance.status === 'running' && (
            <Button icon={<Square size={14} />} variant="ghost" onClick={() => handleAction('stop')} loading={actionLoading}>停止</Button>
          )}
          <Button icon={<RotateCw size={14} />} variant="ghost" onClick={() => handleAction('restart')} loading={actionLoading}>重启</Button>
          <Button icon={<Terminal size={14} />} variant="ghost" onClick={() => handleAction('console')} loading={actionLoading}>控制台</Button>
          <Button icon={<Lock size={14} />} variant="ghost" onClick={() => handleAction('reset_password')} loading={actionLoading}>重置密码</Button>
          <Button icon={<RefreshCw size={14} />} variant="ghost" onClick={() => handleAction('reinstall')} loading={actionLoading}>重装系统</Button>
          <Button icon={<Trash2 size={14} />} variant="ghost" className="text-red-500 hover:text-red-700" onClick={() => handleAction('delete')} loading={actionLoading}>删除</Button>
        </div>
      </div>

      {/* 概览卡片 */}
      <div className="grid grid-cols-4 gap-4">
        <div className="bg-white rounded-xl border border-gray-200 p-4">
          <div className="flex items-center gap-2 text-gray-500 mb-2">
            <Cpu size={16} />
            <span className="text-sm">CPU</span>
          </div>
          <div className="text-2xl font-semibold">{instance.vcpu} <span className="text-sm font-normal text-gray-500">核</span></div>
          {metrics?.cpu_usage !== undefined && <div className="text-xs text-gray-500 mt-1">使用率: {metrics.cpu_usage.toFixed(1)}%</div>}
        </div>
        <div className="bg-white rounded-xl border border-gray-200 p-4">
          <div className="flex items-center gap-2 text-gray-500 mb-2">
            <MemoryStick size={16} />
            <span className="text-sm">内存</span>
          </div>
          <div className="text-2xl font-semibold">{instance.memory_mb} <span className="text-sm font-normal text-gray-500">MB</span></div>
          {metrics?.memory_usage !== undefined && metrics.memory_total !== undefined && (
            <div className="text-xs text-gray-500 mt-1">已用: {(metrics.memory_usage / 1024 / 1024).toFixed(0)}MB / {(metrics.memory_total / 1024 / 1024).toFixed(0)}MB</div>
          )}
        </div>
        <div className="bg-white rounded-xl border border-gray-200 p-4">
          <div className="flex items-center gap-2 text-gray-500 mb-2">
            <HardDrive size={16} />
            <span className="text-sm">磁盘</span>
          </div>
          <div className="text-2xl font-semibold">{instance.disk_gb} <span className="text-sm font-normal text-gray-500">GB</span></div>
          {metrics?.disk_usage !== undefined && metrics.disk_total !== undefined && (
            <div className="text-xs text-gray-500 mt-1">已用: {(metrics.disk_usage / 1024 / 1024 / 1024).toFixed(1)}GB</div>
          )}
        </div>
        <div className="bg-white rounded-xl border border-gray-200 p-4">
          <div className="flex items-center gap-2 text-gray-500 mb-2">
            <Gauge size={16} />
            <span className="text-sm">带宽限制</span>
          </div>
          <div className="text-2xl font-semibold">{instance.network_down || 0} <span className="text-sm font-normal text-gray-500">Mbps</span></div>
          <div className="text-xs text-gray-500 mt-1">上行: {instance.network_up || 0} Mbps</div>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-6">
        {/* 实例信息 */}
        <div className="bg-white rounded-xl border border-gray-200 p-5 space-y-4">
          <h3 className="text-sm font-semibold text-gray-900 flex items-center gap-2">
            <Server size={16} /> 实例信息
          </h3>
          <div className="grid grid-cols-2 gap-y-3 text-sm">
            <div className="text-gray-500">节点</div>
            <div className="font-mono">{instance.node_id.slice(0, 8)}</div>
            <div className="text-gray-500">模板</div>
            <div className="font-mono">{instance.template_id?.slice(0, 8) || '-'}</div>
            <div className="text-gray-500">存储池</div>
            <div>{instance.storage_pool || 'default'}</div>
            <div className="text-gray-500">登录方式</div>
            <div>{instance.login_method}</div>
            <div className="text-gray-500">创建时间</div>
            <div>{instance.created_at ? new Date(instance.created_at).toLocaleString() : '-'}</div>
            <div className="text-gray-500">到期时间</div>
            <div>{instance.expires_at ? new Date(instance.expires_at).toLocaleString() : '-'}</div>
            <div className="text-gray-500">流量模式</div>
            <div>{instance.traffic_mode || '-'}</div>
            <div className="text-gray-500">月流量</div>
            <div>{instance.monthly_traffic || 0} GB</div>
          </div>

          {instance.ssh_password && (
            <div className="pt-3 border-t border-gray-100">
              <div className="flex items-center justify-between mb-1">
                <span className="text-sm text-gray-500">root 密码</span>
                <div className="flex items-center gap-1">
                  <button onClick={() => setShowPassword(!showPassword)} className="text-gray-400 hover:text-gray-600">
                    {showPassword ? <EyeOff size={14} /> : <Eye size={14} />}
                  </button>
                  <button onClick={() => copyToClipboard(instance.ssh_password!)} className="text-gray-400 hover:text-gray-600">
                    <Copy size={14} />
                  </button>
                </div>
              </div>
              <div className="font-mono text-sm bg-gray-50 px-3 py-2 rounded-lg">{showPassword ? instance.ssh_password : '••••••••'}</div>
            </div>
          )}

          {instance.ssh_port && (
            <div className="pt-3 border-t border-gray-100">
              <div className="text-sm text-gray-500 mb-1">SSH 连接</div>
              <div className="flex items-center gap-2">
                <code className="font-mono text-sm bg-gray-50 px-3 py-2 rounded-lg flex-1">ssh root@{instance.ipv4_address || instance.internal_ipv4 || '?'}</code>
                <button onClick={() => copyToClipboard(`ssh root@${instance.ipv4_address || instance.internal_ipv4 || ''}`)} className="text-gray-400 hover:text-gray-600">
                  <Copy size={14} />
                </button>
              </div>
              <div className="text-xs text-gray-400 mt-1">端口: {instance.ssh_port}</div>
            </div>
          )}
        </div>

        {/* 网络信息 */}
        <div className="bg-white rounded-xl border border-gray-200 p-5 space-y-4">
          <h3 className="text-sm font-semibold text-gray-900 flex items-center gap-2">
            <Network size={16} /> 网络信息
          </h3>
          <div className="grid grid-cols-2 gap-y-3 text-sm">
            <div className="text-gray-500">内网 IP</div>
            <div className="font-mono">{instance.internal_ipv4 || '-'}</div>
            <div className="text-gray-500">公网 IPv4</div>
            <div className="font-mono">{instance.ipv4_address || '-'}</div>
            <div className="text-gray-500">公网 IPv6</div>
            <div className="font-mono">{instance.ipv6_address || '-'}</div>
            <div className="text-gray-500">SSH 端口</div>
            <div>{instance.ssh_port || '-'}</div>
          </div>

          {instance.bridge_id && (
            <div className="pt-3 border-t border-gray-100">
              <div className="text-sm font-medium text-gray-900 mb-2">Bridge 网络</div>
              <div className="grid grid-cols-2 gap-y-2 text-sm">
                <div className="text-gray-500">Bridge 名称</div>
                <div>{instance.bridge_name || '-'}</div>
                <div className="text-gray-500">CIDR</div>
                <div className="font-mono">{instance.bridge_cidr || '-'}</div>
                <div className="text-gray-500">网关</div>
                <div className="font-mono">{instance.bridge_gateway || '-'}</div>
                <div className="text-gray-500">内网 IPv4</div>
                <div className="font-mono">{instance.internal_ipv4 || '-'}</div>
                <div className="text-gray-500">EIP IPv4</div>
                <div className="font-mono">{instance.ipv4_eip || '-'}</div>
              </div>
            </div>
          )}

          <div className="pt-3 border-t border-gray-100">
            <div className="flex items-center justify-between mb-2">
              <div className="text-sm font-medium text-gray-900">端口映射</div>
              <button className="text-xs text-blue-600 hover:text-blue-800" onClick={() => setAddingPM(!addingPM)}>
                {addingPM ? '取消' : '添加'}
              </button>
            </div>
            {addingPM && (
              <PortMappingForm onSubmit={handleAddPortMapping} onCancel={() => setAddingPM(false)} />
            )}
            {instance.port_mappings && instance.port_mappings.length > 0 ? (
              <div className="space-y-1">
                {instance.port_mappings.map((pm) => (
                  <div key={pm.id} className="flex items-center justify-between text-sm py-1 px-2 bg-gray-50 rounded">
                    <span>{pm.description || `映射-${pm.host_port}`}</span>
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-gray-600">{pm.host_port}:{pm.container_port}/{pm.protocol}</span>
                      <button className="text-red-500 hover:text-red-700 text-xs" onClick={() => handleDeletePortMapping(pm.id)}>删除</button>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-sm text-gray-400">暂无端口映射</div>
            )}
          </div>
        </div>
      </div>

      {/* 快照管理 */}
      <div className="bg-white rounded-xl border border-gray-200 p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-sm font-semibold text-gray-900 flex items-center gap-2">
            <Calendar size={16} /> 快照管理
          </h3>
          <Button icon={<Plus size={14} />} onClick={handleCreateSnapshot} size="sm">创建快照</Button>
        </div>
        {snapshots.length > 0 ? (
          <div className="space-y-2">
            {snapshots.map((s) => (
              <div key={s.id} className="flex items-center justify-between text-sm py-2 px-3 bg-gray-50 rounded-lg">
                <div className="flex items-center gap-3">
                  <Calendar size={14} className="text-gray-400" />
                  <span>{s.name}</span>
                  <span className="text-xs text-gray-400">{new Date(s.created_at).toLocaleString()}</span>
                </div>
                <button className="text-red-500 hover:text-red-700 text-xs" onClick={() => handleDeleteSnapshot(s.id)}>删除</button>
              </div>
            ))}
          </div>
        ) : (
          <div className="text-sm text-gray-400 text-center py-4">暂无快照</div>
        )}
      </div>
    </div>
  )
}
