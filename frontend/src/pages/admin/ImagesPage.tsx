import { useEffect, useState, useRef, useMemo, Fragment } from 'react'
import { HardDrive, Download, Square, CheckCircle, AlertCircle, Server, Trash2, RefreshCw, Monitor, Box } from 'lucide-react'
import apiClient from '@/api/client'
import { Button } from '@/components/Button/Button'
import { Select } from '@/components/Select/Select'
import { useToastStore } from '@/stores/toast'
import { getOSImage } from '@/utils/osImageHelper'

interface Node {
  id: string
  name: string
  hostname: string
  status: string
  is_online: boolean
}

interface Image {
  id: string
  alias: string
  name: string
  type: string
  distro: string
  release: string
  arch: string
  stage?: string
  progress?: number
  downloaded_bytes?: number
  total_bytes?: number
  speed_bps?: number
  download_error?: string
}

const DISTRO_INFO: Record<string, { name: string; desc: string }> = {
  debian: { name: 'Debian', desc: 'Debian 是完全由自由软件组成的类UNIX操作系统，其包含的多数软件使用GNU通用公共许可协议授权，并由Debian计划的参与者组成团队对其进行打包、开发与维护。' },
  ubuntu: { name: 'Ubuntu', desc: 'Ubuntu 是以桌面应用为主的Linux发行版，基于Debian，由Canonical公司发布，目标是为一般用户提供一个主要由自由软件组成的、同时稳定又易于使用的操作系统。' },
  alpine: { name: 'Alpine', desc: 'Alpine Linux 是一个面向安全应用的轻量级Linux发行版，采用musl libc和BusyBox，体积小巧，适合容器和嵌入式系统。' },
  centos: { name: 'CentOS', desc: 'CentOS 是基于Red Hat Enterprise Linux源代码编译而成的社区企业操作系统，提供稳定、可预测、可管理且可复现的Linux平台。' },
  rocky: { name: 'Rocky Linux', desc: 'Rocky Linux 是一个社区企业操作系统，设计为与Red Hat Enterprise Linux 100%兼容，是CentOS的替代方案。' },
  fedora: { name: 'Fedora', desc: 'Fedora 是一个由社区开发的、基于Linux的操作系统，以快速创新和前沿技术著称，是Red Hat Enterprise Linux的上游。' },
  opensuse: { name: 'openSUSE', desc: 'openSUSE 是一个基于Linux的操作系统，以稳定性和易用性著称，提供Tumbleweed滚动发布和Leap定期发布两个版本。' },
  arch: { name: 'Arch Linux', desc: 'Arch Linux 是一个独立开发的、面向高级用户的Linux发行版，采用滚动发布模式，遵循KISS原则。' },
  oracle: { name: 'Oracle Linux', desc: 'Oracle Linux 是由Oracle公司提供支持的企业级Linux发行版，基于Red Hat Enterprise Linux源代码构建。' },
  kali: { name: 'Kali Linux', desc: 'Kali Linux 是一个基于Debian的Linux发行版，专为数字取证和渗透测试设计，预装了大量安全测试工具。' },
  void: { name: 'Void Linux', desc: 'Void Linux 是一个独立的Linux发行版，使用xbps包管理器和runit init系统，以轻量和快速著称。' },
  openwrt: { name: 'OpenWrt', desc: 'OpenWrt 是一个基于Linux的开源项目，主要用于嵌入式设备特别是无线路由器，提供完整的可写文件系统和包管理。' },
  nixos: { name: 'NixOS', desc: 'NixOS 是一个基于Nix包管理器的Linux发行版，支持声明式配置和原子升级及回滚。' },
  gentoo: { name: 'Gentoo', desc: 'Gentoo 是一个基于源代码编译的Linux发行版，使用Portage包管理系统，以高度可定制性和性能优化著称。' },
  mageia: { name: 'Mageia', desc: 'Mageia 是一个由社区驱动的Linux发行版，源自Mandriva Linux，致力于提供稳定且易用的桌面和服务器操作系统。' },
  openeuler: { name: 'openEuler', desc: 'openEuler 是由开放原子开源基金会孵化及运营的开源Linux发行版，面向服务器、云计算和边缘计算场景。' },
}

function getDistroInfo(distro: string): { name: string; desc: string } {
  const key = distro.toLowerCase()
  return DISTRO_INFO[key] || { name: distro, desc: '' }
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

export default function ImagesPage() {
  const toast = useToastStore()
  const [images, setImages] = useState<Image[]>([])
  const [nodes, setNodes] = useState<Node[]>([])
  const [selectedNode, setSelectedNode] = useState('')
  const [loading, setLoading] = useState(true)
  const [imageSource, setImageSource] = useState('')
  const [refreshing, setRefreshing] = useState(false)
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
        const online = list.find((n: Node) => n.is_online)
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

  const fetchImages = async (nodeId?: string, silent = false) => {
    if (!silent) setLoading(true)
    try {
      const id = nodeId ?? selectedNode
      if (!id) { setImages([]); return }
      const res = await apiClient.get('/images', { params: { node_id: id } })
      setImages(res.data.data || [])
    } finally {
      if (!silent) setLoading(false)
    }
  }

  const fetchImageSource = async () => {
    try {
      const res = await apiClient.get('/images/source')
      setImageSource(res.data.source || 'spiritlhl:')
    } catch { /* ignore */ }
  }

  const handleSourceChange = async (value: string) => {
    setImageSource(String(value))
    setRefreshing(true)
    try {
      await apiClient.put('/images/source', { source: String(value) })
      toast.success('镜像源已切换，缓存已刷新')
      if (selectedNode) await fetchImages(selectedNode, true)
    } catch (err: any) {
      toast.error(err.response?.data?.error || '切换失败')
    } finally {
      setRefreshing(false)
    }
  }

  const handleRefresh = async () => {
    setRefreshing(true)
    try {
      await apiClient.post('/images/refresh', null, { params: selectedNode ? { node_id: selectedNode } : {} })
      toast.success('镜像缓存已刷新')
      if (selectedNode) await fetchImages(selectedNode, true)
    } catch (err: any) {
      toast.error(err.response?.data?.error || '刷新失败')
    } finally {
      setRefreshing(false)
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
      await fetchImageSource()
      await fetchNodes()
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
    if (!confirm(`确认删除镜像 ${image.alias} (${image.type === 'container' ? '容器' : '虚拟机'}, ${formatArch(image.arch)})？`)) return
    try {
      await apiClient.delete('/images', { data: { node_id: nodeId, image_key: image.id } })
      toast.success('删除任务已下发')
      fetchImages(nodeId)
    } catch (err: any) {
      toast.error(err.response?.data?.error || '删除失败')
    }
  }

  // 按发行版分组
  const groupedImages = useMemo(() => {
    const groups: Record<string, Image[]> = {}
    for (const img of images) {
      const key = img.distro || 'unknown'
      if (!groups[key]) groups[key] = []
      groups[key].push(img)
    }
    return Object.entries(groups).sort((a, b) => a[0].localeCompare(b[0]))
  }, [images])

  // 按 release+arch 分组，同一行显示 VM 和容器
  const groupedByReleaseArch = (distroImages: Image[]) => {
    const map: Record<string, { release: string; arch: string; vm?: Image; container?: Image }> = {}
    for (const img of distroImages) {
      const key = `${img.release}|${img.arch}`
      if (!map[key]) map[key] = { release: img.release, arch: img.arch }
      if (img.type === 'virtual-machine') map[key].vm = img
      else if (img.type === 'container') map[key].container = img
    }
    return Object.values(map).sort((a, b) => a.release.localeCompare(b.release) || a.arch.localeCompare(b.arch))
  }

  const renderDownloadCell = (img: Image | undefined, typeLabel: string) => {
    if (!img) return <span className="text-xs text-gray-300">-</span>
    if (!selectedNode) return <span className="text-xs text-gray-300">{typeLabel}</span>

    if (img.stage === 'downloading') {
      return (
        <div className="flex items-center gap-1.5">
          <span className="text-xs text-gray-600 font-number">{img.progress || 0}%</span>
          <button onClick={() => handleCancel(img)} className="text-gray-400 hover:text-red-500" title="取消">
            <Square size={11} />
          </button>
        </div>
      )
    }
    if (img.stage === 'done') {
      return (
        <div className="flex items-center gap-1.5">
          <span className="inline-flex items-center gap-1 text-xs text-green-600"><CheckCircle size={12} />已下载</span>
          <button onClick={() => handleDelete(img)} className="text-gray-400 hover:text-red-500" title="删除">
            <Trash2 size={11} />
          </button>
        </div>
      )
    }
    if (img.stage === 'error') {
      return (
        <span className="inline-flex items-center gap-1 text-xs text-red-600" title={img.download_error || ''}>
          <AlertCircle size={12} />失败
        </span>
      )
    }
    return (
      <button onClick={() => handleDownload(img)} className="inline-flex items-center gap-1 text-xs text-gray-500 hover:text-black transition-colors">
        <Download size={12} />下载
      </button>
    )
  }

  const renderProgressBar = (img: Image | undefined) => {
    if (!img || img.stage !== 'downloading') return null
    const totalBytes = img.total_bytes || 0
    const hasReal = totalBytes > 0
    const downloadedBytes = hasReal ? Math.floor((img.progress || 0) * totalBytes / 100) : 0
    return (
      <td colSpan={6} className="px-4 py-1.5 bg-gray-50/50">
        <div className="flex items-center gap-3">
          {hasReal ? (
            <div className="flex-1 bg-gray-200 rounded-full h-1.5">
              <div className="bg-black h-1.5 rounded-full transition-all" style={{ width: `${img.progress || 0}%` }} />
            </div>
          ) : (
            <div className="flex-1 bg-gray-200 rounded-full h-1.5 animate-pulse" />
          )}
          <span className="text-xs text-gray-500 whitespace-nowrap font-number">
            {hasReal ? `${formatBytes(downloadedBytes)}/${formatBytes(totalBytes)}` : '下载中...'}
          </span>
          {img.speed_bps ? <span className="text-xs text-gray-400 whitespace-nowrap font-number">{formatSpeed(img.speed_bps)}</span> : null}
        </div>
      </td>
    )
  }

  return (
    <div className="p-6 space-y-6">
      {/* 标题 */}
      <div className="flex items-center gap-3">
        <HardDrive size={22} className="text-black" />
        <h1 className="text-xl font-semibold text-black">实例模板管理</h1>
      </div>

      {/* 工具栏 */}
      <div className="rounded-lg border border-gray-200 bg-white px-4 py-3 overflow-visible">
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2 flex-shrink-0">
            <label className="text-xs font-medium text-gray-500 whitespace-nowrap">镜像源</label>
            <div className="w-52 flex-shrink-0">
              <Select
                value={imageSource}
                editable
                placeholder="选择镜像源"
                options={[
                  { label: 'Spiritlhl 镜像源(默认)', value: 'spiritlhl:' },
                  { label: '官方镜像源', value: 'images:' },
                  { label: '清华 TUNA 镜像源', value: 'https://mirrors.tuna.tsinghua.edu.cn/lxc-images' },
                  { label: '中科院 ISCAS 镜像源', value: 'https://mirror.iscas.ac.cn/lxc-images' },
                  { label: 'CERNET 教育网镜像源', value: 'https://mirrors.cernet.edu.cn/lxc-images' },
                  { label: '南阳理工镜像源', value: 'https://mirror.nyist.edu.cn/lxc-images' },
                ]}
                onChange={(v) => handleSourceChange(String(v))}
              />
            </div>
          </div>

          <div className="h-5 w-px bg-gray-200 flex-shrink-0" />

          <div className="flex items-center gap-2 flex-shrink-0">
            <Server size={14} className="text-gray-400" />
            <label className="text-xs font-medium text-gray-500 whitespace-nowrap">节点</label>
            <select
              className="text-sm border border-gray-200 rounded-md px-2.5 py-1.5 bg-white focus:border-black focus:outline-none"
              value={selectedNode}
              onChange={(e) => setSelectedNode(e.target.value)}
            >
              <option value="">-- 选择节点 --</option>
              {nodes.map((n) => (
                <option key={n.id} value={n.id}>
                  {n.name} ({n.hostname}) {n.is_online ? '在线' : '离线'}
                </option>
              ))}
            </select>
          </div>

          <div className="ml-auto flex-shrink-0">
            <Button size="sm" variant="secondary" onClick={handleRefresh} disabled={refreshing} icon={<RefreshCw size={14} className={refreshing ? 'animate-spin' : ''} />}>
              {refreshing ? '刷新中' : '刷新'}
            </Button>
          </div>
        </div>
      </div>

      {/* 镜像列表 - 按发行版分组的表格 */}
      {loading ? (
        <div className="flex items-center justify-center py-20 text-gray-400">
          <RefreshCw size={20} className="animate-spin mr-2" />加载中...
        </div>
      ) : groupedImages.length === 0 ? (
        <div className="flex items-center justify-center py-20 text-gray-400 text-sm">暂无镜像数据</div>
      ) : (
        <div className="space-y-4">
          {groupedImages.map(([distro, distroImages]) => {
            const info = getDistroInfo(distro)
            const rows = groupedByReleaseArch(distroImages)

            return (
              <div key={distro} className="rounded-lg border border-gray-200 bg-white overflow-hidden">
                {/* 表头：发行版信息 */}
                <div className="flex items-center gap-3 px-4 py-3 border-b border-gray-100 bg-gray-50/80">
                  <img src={getOSImage(info.name)} alt={info.name} className="w-7 h-7 rounded object-contain flex-shrink-0" />
                  <div className="min-w-0">
                    <div className="flex items-baseline gap-2">
                      <h2 className="text-sm font-semibold text-gray-900">{info.name}</h2>
                      <span className="text-xs text-gray-400">{distroImages.length} 个镜像</span>
                    </div>
                    {info.desc && <p className="text-xs text-gray-500 leading-relaxed mt-0.5 line-clamp-2">{info.desc}</p>}
                  </div>
                </div>
                {/* 表格 */}
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-gray-100 text-left text-xs text-gray-500">
                      <th className="px-4 py-2 font-medium">版本</th>
                      <th className="px-4 py-2 font-medium w-20">架构</th>
                      <th className="px-4 py-2 font-medium w-24">
                        <span className="inline-flex items-center gap-1"><Monitor size={12} />VM大小</span>
                      </th>
                      <th className="px-4 py-2 font-medium w-36">VM下载</th>
                      <th className="px-4 py-2 font-medium w-24">
                        <span className="inline-flex items-center gap-1"><Box size={12} />容器大小</span>
                      </th>
                      <th className="px-4 py-2 font-medium w-36">容器下载</th>
                    </tr>
                  </thead>
                  <tbody>
                    {rows.map((row) => {
                      const vmDownloading = row.vm?.stage === 'downloading'
                      const containerDownloading = row.container?.stage === 'downloading'
                      return (
                        <Fragment key={`${row.release}|${row.arch}`}>
                          <tr className="border-b border-gray-50 last:border-0 hover:bg-gray-50/50">
                            <td className="px-4 py-2 font-medium text-gray-800">{info.name} {row.release}</td>
                            <td className="px-4 py-2">
                              <span className="text-xs px-1.5 py-0.5 rounded bg-gray-100 text-gray-600">{formatArch(row.arch)}</span>
                            </td>
                            <td className="px-4 py-2 text-xs text-gray-500 font-number">
                              {row.vm?.total_bytes ? formatBytes(row.vm.total_bytes) : '-'}
                            </td>
                            <td className="px-4 py-2">{renderDownloadCell(row.vm, '虚拟机')}</td>
                            <td className="px-4 py-2 text-xs text-gray-500 font-number">
                              {row.container?.total_bytes ? formatBytes(row.container.total_bytes) : '-'}
                            </td>
                            <td className="px-4 py-2">{renderDownloadCell(row.container, '容器')}</td>
                          </tr>
                          {vmDownloading && (
                            <tr className="border-b border-gray-50">
                              {renderProgressBar(row.vm)}
                            </tr>
                          )}
                          {containerDownloading && (
                            <tr className="border-b border-gray-50">
                              {renderProgressBar(row.container)}
                            </tr>
                          )}
                        </Fragment>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
