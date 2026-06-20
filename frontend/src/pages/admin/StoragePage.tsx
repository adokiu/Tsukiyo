import { useEffect, useState, useCallback } from 'react'
import { HardDrive, Plus } from 'lucide-react'
import apiClient from '@/api/client'
import { DataTable, type Column } from '@/components/DataTable/DataTable'
import { Button } from '@/components/Button/Button'
import { Select } from '@/components/Select/Select'
import { SlidePanel } from '@/components/SlidePanel/SlidePanel'
import { TaskProgressModal } from '@/components/TaskProgressModal/TaskProgressModal'
import { useToastStore } from '@/stores/toast'

// ============== 类型定义 ==============
interface Node { id: string; name: string; status: string }

interface PartitionInfo {
  device: string
  name: string
  size: number
  used?: number
  type?: string
  filesystem?: string
  mount_point?: string
  is_system: boolean
}

interface DiskInfo {
  device: string
  size: number
  used?: number
  model?: string
  serial?: string
  type?: string
  filesystem?: string
  is_mounted: boolean
  mount_point?: string
  is_system: boolean
  is_in_use: boolean
  partitions?: PartitionInfo[]
}

interface StoragePool {
  name: string
  driver: string
  source?: string
  size?: number
  used?: number
  in_use?: boolean
  quota_supported?: boolean
}

// ============== 工具函数 ==============
function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(2)} ${units[i]}`
}

// ============== 创建分区侧边栏 ==============
function CreatePartitionPanel({ open, device, nodeId, onClose, onSuccess, onTaskCreated }: {
  open: boolean
  device: string
  nodeId: string
  onClose: () => void
  onSuccess: () => void
  onTaskCreated: (taskId: string, taskType: string) => void
}) {
  const toast = useToastStore()
  const [loading, setLoading] = useState(false)
  const [sizeGB, setSizeGB] = useState(10)

  useEffect(() => {
    if (open) setSizeGB(10)
  }, [open])

  const handleSubmit = async () => {
    if (!nodeId || !device || sizeGB <= 0) return
    setLoading(true)
    try {
      const res = await apiClient.post(`/nodes/${nodeId}/disks/partitions`, {
        device,
        size_gb: sizeGB,
      })
      toast.success('创建分区任务已提交')
      onTaskCreated(res.data.task_id, 'create_partition')
      onSuccess()
      onClose()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '操作失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <SlidePanel
      open={open}
      onClose={onClose}
      title="创建分区"
      width={480}
      footer={
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>取消</Button>
          <Button loading={loading} onClick={handleSubmit}>创建</Button>
        </div>
      }
    >
      <div className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">设备</label>
          <div className="text-sm font-number px-3 py-2 border border-gray-200 rounded-lg bg-gray-50">{device}</div>
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">分区大小 (GB) <span className="text-red-500">*</span></label>
          <input
            type="number"
            min={1}
            value={sizeGB}
            onChange={(e) => setSizeGB(Number(e.target.value))}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm"
            placeholder="10"
          />
        </div>
      </div>
    </SlidePanel>
  )
}

// ============== 格式化磁盘侧边栏 ==============
function FormatDiskPanel({ open, device, nodeId, onClose, onSuccess, onTaskCreated }: {
  open: boolean
  device: string
  nodeId: string
  onClose: () => void
  onSuccess: () => void
  onTaskCreated: (taskId: string, taskType: string) => void
}) {
  const toast = useToastStore()
  const [loading, setLoading] = useState(false)
  const [formatType, setFormatType] = useState('btrfs')

  useEffect(() => {
    if (open) setFormatType('btrfs')
  }, [open])

  const handleSubmit = async () => {
    if (!nodeId || !device || !formatType) return
    if (!confirm(`确认格式化 ${device} 为 ${formatType}？此操作将清除所有数据！`)) return
    setLoading(true)
    try {
      const res = await apiClient.post(`/nodes/${nodeId}/disks/format`, {
        device,
        type: formatType,
      })
      toast.success('格式化任务已提交')
      onTaskCreated(res.data.task_id, 'format_disk')
      onSuccess()
      onClose()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '操作失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <SlidePanel
      open={open}
      onClose={onClose}
      title="格式化磁盘"
      width={480}
      footer={
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>取消</Button>
          <Button variant="danger" loading={loading} onClick={handleSubmit}>确认格式化</Button>
        </div>
      }
    >
      <div className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">设备</label>
          <div className="text-sm font-number px-3 py-2 border border-gray-200 rounded-lg bg-gray-50">{device}</div>
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">格式化类型 <span className="text-red-500">*</span></label>
          <Select
            value={formatType}
            options={[
              { label: 'btrfs', value: 'btrfs' },
              { label: 'zfs', value: 'zfs' },
              { label: 'lvm', value: 'lvm' },
              { label: 'lvm-thin', value: 'lvm-thin' },
              { label: 'ext4', value: 'ext4' },
            ]}
            onChange={(v) => setFormatType(v as string)}
          />
        </div>
        <div className="px-3 py-2 rounded-lg bg-amber-50 text-amber-700 text-sm">
          警告：格式化将清除该设备上的所有数据，且不可恢复。
        </div>
      </div>
    </SlidePanel>
  )
}

// ============== 创建存储池侧边栏 ==============
function CreatePoolPanel({ open, nodeId, disks, onClose, onSuccess, onTaskCreated }: {
  open: boolean
  nodeId: string
  disks: DiskInfo[]
  onClose: () => void
  onSuccess: () => void
  onTaskCreated: (taskId: string, taskType: string) => void
}) {
  const toast = useToastStore()
  const [loading, setLoading] = useState(false)
  const [name, setName] = useState('')
  const [driver, setDriver] = useState('dir')
  const [source, setSource] = useState('')
  const [loopback, setLoopback] = useState(false)
  const [loopbackSize, setLoopbackSize] = useState(20)
  const [thinpoolName, setThinpoolName] = useState('')
  const [zfsPoolName, setZfsPoolName] = useState('')

  useEffect(() => {
    if (open) {
      setName('')
      setDriver('dir')
      setSource('')
      setLoopback(false)
      setLoopbackSize(20)
      setThinpoolName('')
      setZfsPoolName('')
    }
  }, [open])

  // 所有可用源设备（非系统磁盘和分区）
  const sourceOptions: { label: string; value: string }[] = []
  for (const d of disks) {
    if (!d.is_system) {
      sourceOptions.push({ label: `${d.device} (${formatSize(d.size)})`, value: d.device })
    }
    for (const p of d.partitions || []) {
      if (!p.is_system) {
        sourceOptions.push({ label: `${p.device} (${formatSize(p.size)})`, value: p.device })
      }
    }
  }

  const handleSubmit = async () => {
    if (!nodeId || !name || !driver) return
    if (!loopback && !source && driver !== 'dir') {
      toast.error('请选择源设备')
      return
    }
    setLoading(true)
    try {
      const payload: Record<string, any> = {
        name,
        driver: driver === 'lvm-thin' ? 'lvm' : driver,
        source: loopback ? '' : source,
      }
      if (loopback) {
        payload.size = `${loopbackSize}GiB`
      }
      if (driver === 'lvm-thin' && thinpoolName) {
        payload.thinpool_name = thinpoolName
      }
      if (driver === 'zfs' && zfsPoolName) {
        payload.zfs_pool_name = zfsPoolName
      }
      const res = await apiClient.post(`/nodes/${nodeId}/storages/init`, payload)
      toast.success('存储池初始化任务已提交')
      onTaskCreated(res.data.task_id, 'init_storage')
      onSuccess()
      onClose()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '操作失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <SlidePanel
      open={open}
      onClose={onClose}
      title="创建存储池"
      width={560}
      footer={
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>取消</Button>
          <Button loading={loading} onClick={handleSubmit}>创建</Button>
        </div>
      }
    >
      <div className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">存储池名称 <span className="text-red-500">*</span></label>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm"
            placeholder="如：data-pool-01"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">驱动类型 <span className="text-red-500">*</span></label>
          <Select
            value={driver}
            options={[
              { label: 'dir', value: 'dir' },
              { label: 'btrfs', value: 'btrfs' },
              { label: 'zfs', value: 'zfs' },
              { label: 'lvm', value: 'lvm' },
              { label: 'lvm-thin', value: 'lvm-thin' },
            ]}
            onChange={(v) => {
              const d = v as string
              setDriver(d)
              if (d === 'dir') setLoopback(false)
            }}
          />
        </div>

        {/* dir 驱动：源路径输入 */}
        {driver === 'dir' && (
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">源路径</label>
            <input
              value={source}
              onChange={(e) => setSource(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm font-number"
              placeholder="/var/lib/incus/dir-pool"
            />
            <p className="text-xs text-gray-400 mt-1">留空则使用默认路径</p>
          </div>
        )}

        {/* 非 dir 驱动：源选择或 loop-backed */}
        {driver !== 'dir' && (
          <>
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                id="loopback"
                checked={loopback}
                onChange={(e) => setLoopback(e.target.checked)}
                className="w-4 h-4"
              />
              <label htmlFor="loopback" className="text-sm font-medium text-gray-700">Loop-backed 模式（在根分区创建镜像文件）</label>
            </div>

            {loopback ? (
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">镜像文件大小 (GB)</label>
                <input
                  type="number"
                  min={1}
                  value={loopbackSize}
                  onChange={(e) => setLoopbackSize(Number(e.target.value))}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm"
                />
              </div>
            ) : (
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">源设备</label>
                <Select
                  value={source}
                  options={sourceOptions}
                  placeholder="选择设备或分区"
                  onChange={(v) => setSource(v as string)}
                />
              </div>
            )}
          </>
        )}

        {/* lvm-thin 额外配置 */}
        {driver === 'lvm-thin' && (
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Thin Pool 名称</label>
            <input
              value={thinpoolName}
              onChange={(e) => setThinpoolName(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm"
              placeholder="thin"
            />
          </div>
        )}

        {/* zfs 额外配置 */}
        {driver === 'zfs' && (
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">ZFS Pool 名称</label>
            <input
              value={zfsPoolName}
              onChange={(e) => setZfsPoolName(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm"
              placeholder="zpool-data"
            />
          </div>
        )}

      </div>
    </SlidePanel>
  )
}

// ============== 主页面 ==============
export default function StoragePage() {
  const toast = useToastStore()
  const [tab, setTab] = useState<'disks' | 'pools'>('disks')

  // 节点选择
  const [nodes, setNodes] = useState<Node[]>([])
  const [selectedNodeId, setSelectedNodeId] = useState('')

  // 磁盘数据
  const [disks, setDisks] = useState<DiskInfo[]>([])
  const [disksLoading, setDisksLoading] = useState(false)

  // 存储池数据
  const [pools, setPools] = useState<StoragePool[]>([])
  const [poolsLoading, setPoolsLoading] = useState(false)

  // 侧边栏状态
  const [createPartPanelOpen, setCreatePartPanelOpen] = useState(false)
  const [formatPanelOpen, setFormatPanelOpen] = useState(false)
  const [createPoolPanelOpen, setCreatePoolPanelOpen] = useState(false)
  const [targetDevice, setTargetDevice] = useState('')

  // 任务进度弹窗
  const [activeTaskId, setActiveTaskId] = useState<string | null>(null)
  const [activeTaskType, setActiveTaskType] = useState('')

  const handleTaskCreated = (taskId: string, taskType: string) => {
    setActiveTaskId(taskId)
    setActiveTaskType(taskType)
  }

  useEffect(() => {
    apiClient.get('/nodes').then((r) => {
      const list: Node[] = r.data.data || []
      setNodes(list)
      if (list.length > 0 && !selectedNodeId) {
        setSelectedNodeId(list[0].id)
      }
    })
  }, [])

  const fetchDisks = useCallback(() => {
    if (!selectedNodeId) return
    setDisksLoading(true)
    apiClient.get(`/nodes/${selectedNodeId}/disks`).then((res) => setDisks(res.data.data || [])).finally(() => setDisksLoading(false))
  }, [selectedNodeId])

  const fetchPools = useCallback(() => {
    if (!selectedNodeId) return
    setPoolsLoading(true)
    apiClient.get(`/nodes/${selectedNodeId}/storages`).then((res) => setPools(res.data.data || [])).finally(() => setPoolsLoading(false))
  }, [selectedNodeId])

  useEffect(() => {
    if (selectedNodeId) {
      fetchDisks()
      fetchPools()
    } else {
      setDisks([])
      setPools([])
    }
  }, [selectedNodeId, fetchDisks, fetchPools])

  const handleDeletePartition = async (device: string) => {
    if (!confirm(`确认删除分区 ${device}？`)) return
    try {
      const encoded = encodeURIComponent(device)
      const res = await apiClient.delete(`/nodes/${selectedNodeId}/disks/partitions/${encoded}`)
      toast.success('删除分区任务已提交')
      handleTaskCreated(res.data.task_id, 'delete_partition')
      fetchDisks()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '操作失败')
    }
  }

  const handleDeletePool = async (name: string) => {
    if (!confirm(`确认删除存储池 ${name}？`)) return
    try {
      const res = await apiClient.delete(`/nodes/${selectedNodeId}/storages/${name}`)
      toast.success('删除存储池任务已提交')
      handleTaskCreated(res.data.task_id, 'delete_storage')
      fetchPools()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '删除失败')
    }
  }

  const diskColumns: Column<DiskInfo>[] = [
    { key: 'device', title: '设备', width: 140, render: (row) => <span className="text-sm font-number">{row.device}</span> },
    { key: 'size', title: '大小', width: 160, render: (row) => {
      const used = row.used || 0
      const total = row.size
      if (total > 0 && used > 0) {
        const pct = Math.round((used / total) * 100)
        return (
          <div className="metric-cell">
            <div className="metric-cell__header">
              <span className="metric-cell__percent font-number">{pct}%</span>
              <span className="metric-cell__detail font-number">{formatSize(used)} / {formatSize(total)}</span>
            </div>
            <div className="metric-progress">
              <div className={`metric-progress__fill ${pct >= 90 ? 'metric-progress__fill--red' : pct >= 70 ? 'metric-progress__fill--yellow' : 'metric-progress__fill--green'}`} style={{ width: `${Math.min(pct, 100)}%` }} />
            </div>
          </div>
        )
      }
      return <span className="text-sm font-number">{formatSize(total)}</span>
    } },
    { key: 'model', title: '型号', width: 140, render: (row) => <span className="text-sm text-gray-600">{row.model || '-'}</span> },
    { key: 'type', title: '类型', width: 80, render: (row) => <span className="text-sm text-gray-600">{row.type || '-'}</span> },
    { key: 'filesystem', title: '文件系统', width: 100, render: (row) => <span className="text-sm text-gray-600">{row.filesystem || '-'}</span> },
    { key: 'mount_point', title: '挂载点', width: 140, render: (row) => <span className="text-sm font-number text-gray-600">{row.mount_point || '-'}</span> },
    {
      key: 'is_system',
      title: '系统盘',
      width: 80,
      render: (row) => row.is_system
        ? <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-red-100 text-red-700">系统</span>
        : <span className="text-xs text-gray-400">-</span>,
    },
    {
      key: 'partitions',
      title: '分区',
      render: (row) =>
        row.partitions && row.partitions.length > 0 ? (
          <div className="space-y-1">
            {row.partitions.map((p) => (
              <div key={p.device} className="flex items-center gap-2 text-xs">
                <span className="font-number">{p.device}</span>
                <span className="text-gray-500">{formatSize(p.size)}</span>
                {p.used != null && p.used > 0 && <span className="text-gray-400">({formatSize(p.used)} 已用)</span>}
                {p.filesystem && <span className="text-gray-500">[{p.filesystem}]</span>}
                {p.mount_point && <span className="text-gray-500">@ {p.mount_point}</span>}
                {p.is_system && <span className="text-xs font-medium px-1.5 py-0.5 rounded-full bg-red-100 text-red-700">系统</span>}
                {!p.is_system && (
                  <button
                    className="text-red-500 hover:text-red-700 ml-1"
                    onClick={(e) => {
                      e.stopPropagation()
                      handleDeletePartition(p.device)
                    }}
                  >
                    删除
                  </button>
                )}
              </div>
            ))}
          </div>
        ) : (
          <span className="text-xs text-gray-400">无分区</span>
        ),
    },
    {
      key: 'action',
      title: '操作',
      width: 120,
      render: (row: DiskInfo) => (
        <div className="flex items-center gap-3">
          {!row.is_system && (!row.partitions || row.partitions.length === 0) && (
            <button
              className="text-sm text-blue-500 hover:text-blue-700"
              onClick={() => { setTargetDevice(row.device); setCreatePartPanelOpen(true) }}
            >
              创建分区
            </button>
          )}
          {!row.is_system && row.partitions && row.partitions.length > 0 && (
            <button
              className="text-sm text-blue-500 hover:text-blue-700"
              onClick={() => { setTargetDevice(row.device); setFormatPanelOpen(true) }}
            >
              格式化
            </button>
          )}
        </div>
      ),
    },
  ]

  const poolColumns: Column<StoragePool>[] = [
    { key: 'name', title: '名称', width: 160, render: (row) => <span className="text-sm font-number">{row.name}</span> },
    { key: 'driver', title: '驱动', width: 100, render: (row) => (
      <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${
        row.driver === 'zfs' ? 'bg-blue-100 text-blue-700' :
        row.driver === 'btrfs' ? 'bg-purple-100 text-purple-700' :
        row.driver === 'lvm' ? 'bg-orange-100 text-orange-700' :
        'bg-gray-100 text-gray-600'
      }`}>
        {row.driver}
      </span>
    ) },
    { key: 'source', title: '源', width: 200, render: (row) => <span className="text-sm font-number text-gray-600">{row.source || '-'}</span> },
    {
      key: 'usage',
      title: '使用量',
      width: 160,
      render: (row) => {
        if (row.size && row.size > 0) {
          const pct = Math.round(((row.used || 0) / row.size) * 100)
          return (
            <div className="metric-cell">
              <div className="metric-cell__header">
                <span className="metric-cell__percent font-number">{pct}%</span>
                <span className="metric-cell__detail font-number">{formatSize(row.used || 0)} / {formatSize(row.size)}</span>
              </div>
              <div className="metric-progress">
                <div className={`metric-progress__fill ${pct >= 90 ? 'metric-progress__fill--red' : pct >= 70 ? 'metric-progress__fill--yellow' : 'metric-progress__fill--green'}`} style={{ width: `${Math.min(pct, 100)}%` }} />
              </div>
            </div>
          )
        }
        return <span className="text-sm text-gray-400">-</span>
      },
    },
    {
      key: 'in_use',
      title: '状态',
      width: 80,
      render: (row) => (
        <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${row.in_use ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-600'}`}>
          {row.in_use ? '使用中' : '空闲'}
        </span>
      ),
    },
    {
      key: 'quota_supported',
      title: '配额',
      width: 80,
      render: (row) => (
        row.quota_supported
          ? <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-green-100 text-green-700">支持</span>
          : <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-gray-100 text-gray-500">不支持</span>
      ),
    },
    {
      key: 'action',
      title: '操作',
      render: (row: StoragePool) => (
        <button className="text-sm text-red-500 hover:text-red-700" onClick={() => handleDeletePool(row.name)}>删除</button>
      ),
    },
  ]

  const nodeOptions = nodes.map((n) => ({ label: `${n.name} (${n.id.slice(0, 8)})`, value: n.id }))

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <HardDrive size={22} className="text-black" />
          <h1 className="text-xl font-semibold text-black">存储管理</h1>
        </div>
        {selectedNodeId && tab === 'pools' && (
          <Button icon={<Plus size={16} />} onClick={() => setCreatePoolPanelOpen(true)}>创建存储池</Button>
        )}
      </div>

      {/* 节点选择器 */}
      <div className="flex items-center gap-3">
        <label className="text-sm font-medium text-gray-700 whitespace-nowrap">选择宿主机节点</label>
        <div className="w-80">
          <Select value={selectedNodeId} options={nodeOptions} placeholder="请选择 Agent 节点" onChange={(v) => setSelectedNodeId(v as string)} />
        </div>
      </div>

      {selectedNodeId ? (
        <>
          {/* 标签页 */}
          <div className="flex border-b border-gray-200">
            <button
              className={`px-4 py-2 text-sm font-medium ${tab === 'disks' ? 'text-black border-b-2 border-black' : 'text-gray-500 hover:text-gray-700'}`}
              onClick={() => setTab('disks')}
            >
              磁盘管理
            </button>
            <button
              className={`px-4 py-2 text-sm font-medium ${tab === 'pools' ? 'text-black border-b-2 border-black' : 'text-gray-500 hover:text-gray-700'}`}
              onClick={() => setTab('pools')}
            >
              存储池管理
            </button>
          </div>

          {tab === 'disks' && (
            <DataTable columns={diskColumns} data={disks} rowKey={(r) => r.device} loading={disksLoading} emptyText="暂无磁盘信息" />
          )}

          {tab === 'pools' && (
            <DataTable columns={poolColumns} data={pools} rowKey={(r) => r.name} loading={poolsLoading} emptyText="暂无存储池" />
          )}
        </>
      ) : (
        <div className="flex flex-col items-center justify-center py-20 text-gray-400">
          <HardDrive size={48} className="mb-3 opacity-40" />
          <p className="text-sm">请先选择宿主机节点以管理存储</p>
        </div>
      )}

      <CreatePartitionPanel
        open={createPartPanelOpen}
        device={targetDevice}
        nodeId={selectedNodeId}
        onClose={() => setCreatePartPanelOpen(false)}
        onSuccess={fetchDisks}
        onTaskCreated={handleTaskCreated}
      />
      <FormatDiskPanel
        open={formatPanelOpen}
        device={targetDevice}
        nodeId={selectedNodeId}
        onClose={() => setFormatPanelOpen(false)}
        onSuccess={fetchDisks}
        onTaskCreated={handleTaskCreated}
      />
      <CreatePoolPanel
        open={createPoolPanelOpen}
        nodeId={selectedNodeId}
        disks={disks}
        onClose={() => setCreatePoolPanelOpen(false)}
        onSuccess={fetchPools}
        onTaskCreated={handleTaskCreated}
      />

      <TaskProgressModal
        taskId={activeTaskId}
        taskType={activeTaskType}
        onClose={() => { setActiveTaskId(null); fetchDisks(); fetchPools() }}
      />
    </div>
  )
}
