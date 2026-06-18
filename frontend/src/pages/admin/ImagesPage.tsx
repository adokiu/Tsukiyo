import { useEffect, useState, useRef } from 'react'
import { HardDrive, Download, Square, CheckCircle, AlertCircle, Server } from 'lucide-react'
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
  id: string
  name: string
  type: string
  distro: string
  release: string
  arch: string
  enabled: boolean
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

export default function ImagesPage() {
  const toast = useToastStore()
  const [images, setImages] = useState<Image[]>([])
  const [nodes, setNodes] = useState<Node[]>([])
  const [selectedNode, setSelectedNode] = useState('')
  const [loading, setLoading] = useState(true)
  const wsRef = useRef<WebSocket | null>(null)

  // 局部更新单个镜像字段，不触发整页刷新
  const patchImage = (imageId: string, patch: Partial<Image>) => {
    setImages((prev) =>
      prev.map((img) => (img.id === imageId ? { ...img, ...patch } : img))
    )
  }

  // 获取节点列表
  const fetchNodes = async () => {
    try {
      const res = await apiClient.get('/nodes')
      const list = res.data.data || []
      setNodes(list)
      // 默认选择第一个在线且已初始化的节点
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

  // 获取镜像列表（按节点）
  const fetchImages = async (nodeId?: string) => {
    setLoading(true)
    try {
      const id = nodeId ?? selectedNode
      const params = id ? { params: { node_id: id } } : undefined
      const res = await apiClient.get('/images', params)
      const list = res.data.data || []
      setImages(list)
    } finally {
      setLoading(false)
    }
  }

  // WebSocket 连接：接收实时镜像进度推送
  const connectWebSocket = () => {
    if (wsRef.current) {
      wsRef.current.close()
    }
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${proto}//${window.location.host}/ws/images`
    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onopen = () => {
      console.log('镜像进度 WebSocket 已连接')
    }
    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        if (msg.type === 'image_progress') {
          const p = msg.payload
          // 只更新对应镜像，不触发整页刷新
          patchImage(p.image_id, {
            stage: p.stage,
            progress: p.progress,
            downloaded_bytes: p.downloaded_bytes,
            total_bytes: p.total_bytes,
            speed_bps: p.speed_bps,
            download_error: p.error,
          })
        }
      } catch {
        // ignore
      }
    }
    ws.onclose = () => {
      console.log('镜像进度 WebSocket 已断开，3秒后重连')
      wsRef.current = null
      setTimeout(connectWebSocket, 3000)
    }
    ws.onerror = (err) => {
      console.error('镜像进度 WebSocket 错误', err)
    }
  }

  useEffect(() => {
    const init = async () => {
      const nodeId = await fetchNodes()
      await fetchImages(nodeId)
    }
    init()
    connectWebSocket()
    return () => {
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (selectedNode) {
      fetchImages(selectedNode)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedNode])

  const handleToggle = async (id: string) => {
    await apiClient.post(`/images/${id}/toggle`)
    toast.success('状态已更新')
    fetchImages()
  }

  const handleDownload = async (image: Image) => {
    const nodeId = selectedNode
    if (!nodeId) {
      toast.error('请先选择节点')
      return
    }
    // 立即本地显示下载中状态，不等待后端响应
    patchImage(image.id, { stage: 'downloading', progress: 0 })
    try {
      await apiClient.post(`/images/${image.id}/download`, { node_id: nodeId })
      toast.success('下载任务已下发')
      fetchImages(nodeId)
    } catch (err: any) {
      // 失败后恢复状态并提示
      patchImage(image.id, { stage: undefined, progress: undefined })
      toast.error(err.response?.data?.error || '下发失败')
    }
  }

  const handleCancel = async (image: Image) => {
    const nodeId = selectedNode
    if (!nodeId) return
    try {
      await apiClient.post(`/images/${image.id}/cancel`, { node_id: nodeId })
      toast.success('取消任务已下发')
    } catch (err: any) {
      toast.error(err.response?.data?.error || '取消失败')
    }
  }

  const columns: Column<Image>[] = [
    { key: 'name', title: '镜像名称' },
    {
      key: 'type',
      title: '类型',
      render: (row: Image) => (
        <span className="text-xs px-2 py-0.5 rounded bg-gray-100 text-gray-600">
          {row.type === 'container' ? '容器' : '虚拟机'}
        </span>
      ),
    },
    { key: 'distro', title: '发行版' },
    { key: 'release', title: '版本' },
    { key: 'arch', title: '架构' },
    {
      key: 'download_status',
      title: '节点状态',
      width: 280,
      render: (row: Image) => {
        if (!selectedNode) return <span className="text-gray-400 text-sm">选择节点查看</span>
        if (row.stage === 'downloading') {
          const totalBytes = row.total_bytes || 0
          const hasRealProgress = totalBytes > 0
          return (
            <div className="w-full space-y-1">
              <div className="flex justify-between text-xs text-gray-600">
                <span>{hasRealProgress ? `下载中 ${row.progress || 0}%` : '下载中...'}</span>
                {row.speed_bps ? <span>{formatSpeed(row.speed_bps)}</span> : null}
              </div>
              {hasRealProgress ? (
                <div className="w-full bg-gray-200 rounded-full h-1.5">
                  <div
                    className="bg-black h-1.5 rounded-full transition-all"
                    style={{ width: `${row.progress || 0}%` }}
                  />
                </div>
              ) : (
                <div className="w-full bg-gray-200 rounded-full h-1.5 animate-pulse" />
              )}
              {hasRealProgress ? (
                <div className="text-xs text-gray-500">
                  {formatBytes(row.downloaded_bytes || 0)} / {formatBytes(totalBytes)}
                </div>
              ) : null}
            </div>
          )
        }
        if (row.stage === 'done') {
          return (
            <span className="flex items-center gap-1 text-sm text-green-600">
              <CheckCircle size={14} />
              已下载
            </span>
          )
        }
        if (row.stage === 'error') {
          const errorDetail = row.download_error || '未知错误'
          return (
            <div className="flex flex-col gap-0.5">
              <span className="flex items-center gap-1 text-sm text-red-600">
                <AlertCircle size={14} />
                下载失败
              </span>
              <span className="text-xs text-red-500 max-w-[200px] truncate" title={errorDetail}>
                {errorDetail}
              </span>
            </div>
          )
        }
        return (
          <span className="text-sm text-gray-400">
            未下载
          </span>
        )
      },
    },
    {
      key: 'actions',
      title: '操作',
      width: 120,
      render: (row: Image) => {
        if (!selectedNode) return null
        const isDownloading = row.stage === 'downloading'
        if (isDownloading) {
          return (
            <Button size="sm" variant="ghost" onClick={() => handleCancel(row)}>
              <Square size={14} className="mr-1" />
              取消
            </Button>
          )
        }
        if (row.stage === 'done') {
          return (
            <span className="text-xs text-gray-400">已完成</span>
          )
        }
        return (
          <Button size="sm" variant="ghost" onClick={() => handleDownload(row)}>
            <Download size={14} className="mr-1" />
            下载
          </Button>
        )
      },
    },
    {
      key: 'enabled',
      title: '启用',
      width: 80,
      render: (row: Image) => (
        <button
          className={`text-xs px-2 py-1 rounded ${row.enabled ? 'bg-black text-white' : 'bg-gray-100 text-gray-500'}`}
          onClick={() => handleToggle(row.id)}
        >
          {row.enabled ? '启用' : '禁用'}
        </button>
      ),
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
        {!selectedNode && (
          <span className="text-xs text-gray-400">选择节点后可查看各节点已下载的镜像并执行下载</span>
        )}
      </div>

      <DataTable columns={columns} data={images} rowKey={(r) => r.id} loading={loading} />
    </div>
  )
}
