import { useEffect, useState, useRef, useMemo } from 'react'
import { HardDrive, Download, Square, CheckCircle, AlertCircle, Server, Trash2 } from 'lucide-react'
import apiClient from '@/api/client'
import { DataTable, type Column } from '@/components/DataTable/DataTable'
import { Button } from '@/components/Button/Button'
import { useToastStore } from '@/stores/toast'

interface Node {
  id: string
  name: string
  hostname: string
  status: string
  is_online: boolean
  initialized: boolean
}

interface Image {
  id: string        // image_key: alias|type|arch
  alias: string     // debian/forky/cloud
  name: string
  type: string      // container / virtual-machine
  distro: string
  release: string
  arch: string      // x86_64 / aarch64
  stage?: string
  progress?: number
  downloaded_bytes?: number
  total_bytes?: number
  speed_bps?: number
  download_error?: string
}

function formatSpeed(bps: number): string {
  if (bps >= 1024 * 1024) return `${(bps / 1024 / 1024).toFixed(1)} MB/s`
  if (bps >= 1024) return `${(bps / 1024).toFixed(1)} KB/s`
  return `${bps} B/s`
}

function formatBytes(bytes: number): string {
  if (bytes >= 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024 / 1024).toFixed(1)} GB`
  if (bytes >= 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${bytes} B`
}

function formatArch(arch: string): string {
  switch (arch) {
    case 'x86_64': return 'amd64'
    case 'aarch64': return 'arm64'
    default: return arch
  }
}

function formatType(type: string): string {
  return type === 'container' ? '容器' : '虚拟机'
}

export default function ImagesPage() {
  const toast = useToastStore()
  const [images, setImages] = useState<Image[]>([])
  const [nodes, setNodes] = useState<Node[]>([])
  const [selectedNode, setSelectedNode] = useState('')
  const [loading, setLoading] = useState(true)
  const [filterType, setFilterType] = useState<string>('all')
  const [filterArch, setFilterArch] = useState<string>('all')
  const [filterDistro, setFilterDistro] = useState<string>('all')
  const wsRef = useRef<WebSocket | null>(null)

  // 局部更新单个镜像，按 image_key (id) 匹配
  const patchImage = (imageKey: string, patch: Partial<Image>) => {
    setImages((prev) =>
      prev.map((img) => (img.id === imageKey ? { ...img, ...patch } : img))
    )
  }

  const fetchNodes = async () => {
    try {
      const res = await apiClient.get('/nodes')
      const list = res.data.data || []
      setNodes(list)
      const current = selectedNode
      if (!current) {
        const online = list.find((n: Node) => n.is_online && n.initialized)
        if (online) {
          setSelectedNode(online.id)
          return online.id
        }
      }
      return current || ''
    } catch {
      return ''
    }
  }

  const fetchImages = async (nodeId?: string, fType?: string, fArch?: string, fDistro?: string) => {
    setLoading(true)
    try {
      const id = nodeId ?? selectedNode
      if (!id) { setImages([]); return }
      const p: Record<string, string> = { node_id: id }
      const t = fType ?? filterType
      const a = fArch ?? filterArch
      const d = fDistro ?? filterDistro
      if (t && t !== 'all') p.type = t
      if (a && a !== 'all') p.arch = a
      if (d && d !== 'all') p.distro = d
      const res = await apiClient.get('/images', { params: p })
      setImages(res.data.data || [])
    } finally {
      setLoading(false)
    }
  }

  const connectWebSocket = () => {
    if (wsRef.current) wsRef.current.close()
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const ws = new WebSocket(`${proto}//${window.location.host}/ws/images`)
    wsRef.current = ws

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        if (msg.type === 'image_progress') {
          const p = msg.payload
          patchImage(p.image_id, {
            stage: p.stage,
            progress: p.progress,
            downloaded_bytes: p.downloaded_bytes,
            total_bytes: p.total_bytes,
            speed_bps: p.speed_bps,
            download_error: p.error,
          })
        }
      } catch { /* ignore */ }
    }
    ws.onclose = () => {
      wsRef.current = null
      setTimeout(connectWebSocket, 3000)
    }
  }

  useEffect(() => {
    const init = async () => {
      const nodeId = await fetchNodes()
      await fetchImages(nodeId)
    }
    init()
    connectWebSocket()
    return () => { wsRef.current?.close(); wsRef.current = null }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (selectedNode) fetchImages(selectedNode)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedNode])

  useEffect(() => {
    if (selectedNode) fetchImages(selectedNode, filterType, filterArch, filterDistro)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filterType, filterArch, filterDistro])

  // 从已加载数据中提取可选发行版/架构（用于下拉框选项）
  const distros = useMemo(() => [...new Set(images.map(i => i.distro))].filter(Boolean).sort(), [images])
  const archs = useMemo(() => [...new Set(images.map(i => i.arch))].filter(Boolean).sort(), [images])

  const handleDownload = async (image: Image) => {
    const nodeId = selectedNode
    if (!nodeId) { toast.error('请先选择节点'); return }
    patchImage(image.id, { stage: 'downloading', progress: 0 })
    try {
      await apiClient.post('/images/download', {
        node_id: nodeId,
        image_key: image.id,
      })
      toast.success('下载任务已下发')
    } catch (err: any) {
      patchImage(image.id, { stage: undefined, progress: undefined })
      toast.error(err.response?.data?.error || '下发失败')
    }
  }

  const handleCancel = async (image: Image) => {
    const nodeId = selectedNode
    if (!nodeId) return
    try {
      await apiClient.post('/images/cancel', { node_id: nodeId, image_key: image.id })
      toast.success('取消任务已下发')
    } catch (err: any) {
      toast.error(err.response?.data?.error || '取消失败')
    }
  }

  const handleDelete = async (image: Image) => {
    const nodeId = selectedNode
    if (!nodeId) { toast.error('请先选择节点'); return }
    if (!confirm(`确认删除镜像 ${image.alias} (${formatType(image.type)}, ${formatArch(image.arch)})？`)) return
    try {
      await apiClient.delete('/images', { data: { node_id: nodeId, image_key: image.id } })
      toast.success('删除任务已下发')
      fetchImages(nodeId)
    } catch (err: any) {
      toast.error(err.response?.data?.error || '删除失败')
    }
  }

  const columns: Column<Image>[] = [
    {
      key: 'alias',
      title: '镜像别名',
      render: (row: Image) => (
        <div>
          <div className="font-medium text-gray-900 text-sm">{row.alias}</div>
          <div className="text-xs text-gray-400 truncate max-w-[300px]">{row.name}</div>
        </div>
      ),
    },
    {
      key: 'type',
      title: '类型',
      width: 80,
      render: (row: Image) => (
        <span className={`text-xs px-2 py-0.5 rounded ${row.type === 'container' ? 'bg-blue-50 text-blue-600' : 'bg-purple-50 text-purple-600'}`}>
          {formatType(row.type)}
        </span>
      ),
    },
    { key: 'distro', title: '发行版', width: 100 },
    { key: 'release', title: '版本', width: 100 },
    {
      key: 'arch',
      title: '架构',
      width: 80,
      render: (row: Image) => (
        <span className="text-xs px-2 py-0.5 rounded bg-gray-100 text-gray-600">
          {formatArch(row.arch)}
        </span>
      ),
    },
    {
      key: 'download_status',
      title: '节点状态',
      width: 280,
      render: (row: Image) => {
        if (!selectedNode) return <span className="text-gray-400 text-sm">选择节点查看</span>
        if (row.stage === 'downloading') {
          const totalBytes = row.total_bytes || 0
          const hasReal = totalBytes > 0
          return (
            <div className="w-full space-y-1">
              <div className="flex justify-between text-xs text-gray-600">
                <span>{hasReal ? `下载中 ${row.progress || 0}%` : '下载中...'}</span>
                {row.speed_bps ? <span>{formatSpeed(row.speed_bps)}</span> : null}
              </div>
              {hasReal ? (
                <div className="w-full bg-gray-200 rounded-full h-1.5">
                  <div className="bg-black h-1.5 rounded-full transition-all" style={{ width: `${row.progress || 0}%` }} />
                </div>
              ) : (
                <div className="w-full bg-gray-200 rounded-full h-1.5 animate-pulse" />
              )}
              {hasReal ? <div className="text-xs text-gray-500">{formatBytes(row.downloaded_bytes || 0)} / {formatBytes(totalBytes)}</div> : null}
            </div>
          )
        }
        if (row.stage === 'done') {
          return <span className="flex items-center gap-1 text-sm text-green-600"><CheckCircle size={14} />已下载</span>
        }
        if (row.stage === 'error') {
          const err = row.download_error || '未知错误'
          return (
            <div className="flex flex-col gap-0.5">
              <span className="flex items-center gap-1 text-sm text-red-600"><AlertCircle size={14} />下载失败</span>
              <span className="text-xs text-red-500 max-w-[200px] truncate" title={err}>{err}</span>
            </div>
          )
        }
        return <span className="text-sm text-gray-400">未下载</span>
      },
    },
    {
      key: 'actions',
      title: '操作',
      width: 120,
      render: (row: Image) => {
        if (!selectedNode) return null
        if (row.stage === 'downloading') {
          return <Button size="sm" variant="ghost" onClick={() => handleCancel(row)}><Square size={14} className="mr-1" />取消</Button>
        }
        if (row.stage === 'done') {
          return <Button size="sm" variant="ghost" onClick={() => handleDelete(row)}><Trash2 size={14} className="mr-1" />删除</Button>
        }
        return <Button size="sm" variant="ghost" onClick={() => handleDownload(row)}><Download size={14} className="mr-1" />下载</Button>
      },
    },
  ]

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <HardDrive size={22} className="text-black" />
          <h1 className="text-xl font-semibold text-black">镜像管理</h1>
        </div>
      </div>

      {/* 节点选择器 */}
      <div className="flex items-center gap-3 bg-gray-50 rounded-lg p-3">
        <Server size={18} className="text-gray-500" />
        <label className="text-sm text-gray-600">选择节点：</label>
        <select
          className="text-sm border border-gray-200 rounded-md px-2 py-1.5 bg-white focus:border-black focus:outline-none"
          value={selectedNode}
          onChange={(e) => setSelectedNode(e.target.value)}
        >
          <option value="">-- 请选择节点 --</option>
          {nodes.map((n) => (
            <option key={n.id} value={n.id}>
              {n.name} ({n.hostname}) {n.is_online ? '在线' : '离线'} {n.initialized ? '' : '[未配置]'}
            </option>
          ))}
        </select>
        {!selectedNode && <span className="text-xs text-gray-400">选择节点后可查看各节点已下载的镜像并执行下载</span>}
      </div>

      {/* 筛选栏 */}
      <div className="flex items-center gap-4 text-sm">
        <div className="flex items-center gap-2">
          <label className="text-gray-500">类型:</label>
          <select className="border border-gray-200 rounded px-2 py-1 text-sm" value={filterType} onChange={e => setFilterType(e.target.value)}>
            <option value="all">全部</option>
            <option value="container">容器</option>
            <option value="virtual-machine">虚拟机</option>
          </select>
        </div>
        <div className="flex items-center gap-2">
          <label className="text-gray-500">架构:</label>
          <select className="border border-gray-200 rounded px-2 py-1 text-sm" value={filterArch} onChange={e => setFilterArch(e.target.value)}>
            <option value="all">全部</option>
            {archs.map(a => <option key={a} value={a}>{formatArch(a)}</option>)}
          </select>
        </div>
        <div className="flex items-center gap-2">
          <label className="text-gray-500">发行版:</label>
          <select className="border border-gray-200 rounded px-2 py-1 text-sm" value={filterDistro} onChange={e => setFilterDistro(e.target.value)}>
            <option value="all">全部</option>
            {distros.map(d => <option key={d} value={d}>{d}</option>)}
          </select>
        </div>
        <span className="text-gray-400 text-xs ml-2">共 {images.length} 个镜像</span>
      </div>

      <DataTable columns={columns} data={images} rowKey={(r) => r.id} loading={loading} />
    </div>
  )
}
