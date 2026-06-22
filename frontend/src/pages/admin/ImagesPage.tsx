import { useEffect, useState, useRef, useMemo, Fragment, useCallback } from 'react'
import { Download, Square, CheckCircle, AlertCircle, Trash2, RefreshCw, Monitor, Box, Server, X } from 'lucide-react'
import apiClient from '@/api/client'
import { Button } from '@/components/Button/Button'
import { Select } from '@/components/Select/Select'
import { SlidePanel } from '@/components/SlidePanel/SlidePanel'
import { Modal } from '@/components/Modal/Modal'
import { useToastStore } from '@/stores/toast'
import { PageLayout } from '@/components/PageLayout/PageLayout'
import { getOSImage } from '@/utils/osImageHelper'
import '@/components/PageTransition/PageTransition.css'
import '@/components/DataTable/DataTable.css'

interface Node {
  id: string
  name: string
  hostname: string
  status: string
  is_online: boolean
}

// 远程镜像（下载tab）
interface RemoteImage {
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

// 已安装镜像（agent上报）
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

// 分类
interface Category {
  id: string
  name: string
  image_type: string
  sort_order: number
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

function sourceLabel(source: string): string {
  switch (source) {
    case 'spiritlhl': return 'Spiritlhl'
    case 'images': return '官方'
    case 'manual': return '手动导入'
    default: return source || '未知'
  }
}

function typeLabel(type: string): string {
  switch (type) {
    case 'container': return '容器'
    case 'virtual-machine': return '虚拟机'
    default: return type
  }
}

export default function ImagesPage() {
  const toast = useToastStore()
  const [activeTab, setActiveTab] = useState<'installed' | 'remote'>('installed')
  const [nodes, setNodes] = useState<Node[]>([])
  const [selectedNode, setSelectedNode] = useState('')
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const wsRef = useRef<WebSocket | null>(null)
  const wsManualClose = useRef(false)

  // 远程镜像tab状态
  const [remoteImages, setRemoteImages] = useState<RemoteImage[]>([])
  const [imageSource, setImageSource] = useState('')

  // 已安装镜像tab状态
  const [installedImages, setInstalledImages] = useState<InstalledImage[]>([])
  const [categories, setCategories] = useState<Category[]>([])
  const [installedFilterType, setInstalledFilterType] = useState('')
  const [editingImage, setEditingImage] = useState<InstalledImage | null>(null)
  const [editDisplayName, setEditDisplayName] = useState('')
  const [editCategoryName, setEditCategoryName] = useState('')
  const [editInstallSSH, setEditInstallSSH] = useState(false)
  const [editPanelOpen, setEditPanelOpen] = useState(false)
  const [categoryInput, setCategoryInput] = useState('')

  // 删除确认
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [confirmImage, setConfirmImage] = useState<RemoteImage | null>(null)

  // 局部更新单个远程镜像
  const patchRemoteImage = useCallback((imageKey: string, patch: Partial<RemoteImage>) => {
    setRemoteImages((prev) =>
      prev.map((img) => (img.id === imageKey ? { ...img, ...patch } : img))
    )
  }, [])

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

  const fetchRemoteImages = async (nodeId?: string, silent = false) => {
    if (!silent) setLoading(true)
    try {
      const id = nodeId ?? selectedNode
      if (!id) { setRemoteImages([]); return }
      const res = await apiClient.get('/images', { params: { node_id: id } })
      setRemoteImages(res.data.data || [])
    } finally {
      if (!silent) setLoading(false)
    }
  }

  const fetchInstalledImages = async (nodeId?: string, silent = false) => {
    if (!silent) setLoading(true)
    try {
      const id = nodeId ?? selectedNode
      if (!id) { setInstalledImages([]); return }
      const params: Record<string, string> = { node_id: id }
      if (installedFilterType) params.type = installedFilterType
      const res = await apiClient.get('/images/installed', { params })
      setInstalledImages(res.data.data || [])
    } catch {
      setInstalledImages([])
    } finally {
      if (!silent) setLoading(false)
    }
  }

  const fetchCategories = async (nodeId?: string) => {
    const id = nodeId ?? selectedNode
    if (!id) { setCategories([]); return }
    try {
      const params: Record<string, string> = { node_id: id }
      if (installedFilterType) params.type = installedFilterType
      const res = await apiClient.get('/images/categories', { params })
      setCategories(res.data.data || [])
    } catch {
      setCategories([])
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
      if (selectedNode) await fetchRemoteImages(selectedNode, true)
    } catch (err: any) {
      toast.error(err.response?.data?.error || '切换失败')
    } finally {
      setRefreshing(false)
    }
  }

  const handleRefresh = async () => {
    setRefreshing(true)
    try {
      if (activeTab === 'installed') {
        if (selectedNode) {
          await apiClient.post('/images/sync', null, { params: { node_id: selectedNode } })
          toast.success('同步任务已触发')
          setTimeout(() => fetchInstalledImages(selectedNode, true), 2000)
        }
      } else {
        await apiClient.post('/images/refresh', null, { params: selectedNode ? { node_id: selectedNode } : {} })
        toast.success('镜像缓存已刷新')
        if (selectedNode) await fetchRemoteImages(selectedNode, true)
      }
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
          if (p.stage === 'sync') {
            // 镜像列表同步完成，刷新已安装镜像
            if (selectedNode) fetchInstalledImages(selectedNode, true)
          } else {
            patchRemoteImage(p.image_id, {
              stage: p.stage,
              progress: p.progress,
              speed_bps: p.speed_bps,
              download_error: p.error,
            })
          }
        }
      } catch { /* ignore */ }
    }
    ws.onclose = () => {
      wsRef.current = null
      if (!wsManualClose.current) {
        setTimeout(connectWebSocket, 3000)
      }
    }
    ws.onerror = () => {
      ws.close()
    }
  }

  useEffect(() => {
    const init = async () => {
      await fetchImageSource()
      await fetchNodes()
    }
    init()
    connectWebSocket()
    return () => { wsManualClose.current = true; wsRef.current?.close(); wsRef.current = null }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (selectedNode) {
      if (activeTab === 'installed') {
        fetchInstalledImages(selectedNode)
        fetchCategories(selectedNode)
      } else {
        fetchRemoteImages(selectedNode)
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedNode])

  useEffect(() => {
    if (selectedNode && activeTab === 'installed') {
      fetchInstalledImages(selectedNode, true)
      fetchCategories(selectedNode)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [installedFilterType])

  const handleDownload = async (image: RemoteImage) => {
    const nodeId = selectedNode
    if (!nodeId) { toast.error('请先选择节点'); return }
    patchRemoteImage(image.id, { stage: 'downloading', progress: 0 })
    try {
      await apiClient.post('/images/download', {
        node_id: nodeId,
        image_key: image.id,
      })
      toast.success('下载任务已下发')
    } catch (err: any) {
      patchRemoteImage(image.id, { stage: undefined, progress: undefined })
      toast.error(err.response?.data?.error || '下发失败')
    }
  }

  const handleCancel = async (image: RemoteImage) => {
    const nodeId = selectedNode
    if (!nodeId) return
    try {
      await apiClient.post('/images/cancel', { node_id: nodeId, image_key: image.id })
      toast.success('取消任务已下发')
    } catch (err: any) {
      toast.error(err.response?.data?.error || '取消失败')
    }
  }

  const handleDeleteRemote = async (image: RemoteImage) => {
    setConfirmImage(image)
    setConfirmOpen(true)
  }

  const doDelete = async () => {
    const image = confirmImage
    const nodeId = selectedNode
    if (!image || !nodeId) return
    try {
      await apiClient.delete('/images', { data: { node_id: nodeId, image_key: image.id } })
      toast.success('删除任务已下发')
      fetchRemoteImages(nodeId)
      setTimeout(() => fetchInstalledImages(nodeId, true), 2000)
    } catch (err: any) {
      toast.error(err.response?.data?.error || '删除失败')
    }
  }

  // 已安装镜像编辑
  const handleEditImage = (img: InstalledImage) => {
    setEditingImage(img)
    setEditDisplayName(img.display_name)
    setEditCategoryName(img.category_name || '')
    setEditInstallSSH(img.install_ssh)
    setCategoryInput('')
    setEditPanelOpen(true)
  }

  const doEditImage = async () => {
    if (!editingImage || !selectedNode) return
    try {
      await apiClient.put('/images/alias', {
        node_id: selectedNode,
        fingerprint: editingImage.fingerprint,
        image_type: editingImage.type,
        category_name: editCategoryName || '',
        display_name: editDisplayName,
        install_ssh: editInstallSSH,
      })
      toast.success('镜像已更新')
      setEditPanelOpen(false)
      fetchInstalledImages(selectedNode, true)
      fetchCategories(selectedNode)
    } catch (err: any) {
      toast.error(err.response?.data?.error || '更新失败')
    }
  }

  // 远程镜像 - 按发行版分组
  const groupedRemoteImages = useMemo(() => {
    const groups: Record<string, RemoteImage[]> = {}
    for (const img of remoteImages) {
      const key = img.distro || 'unknown'
      if (!groups[key]) groups[key] = []
      groups[key].push(img)
    }
    return Object.entries(groups).sort((a, b) => a[0].localeCompare(b[0]))
  }, [remoteImages])

  const groupedByReleaseArch = (distroImages: RemoteImage[]) => {
    const map: Record<string, { release: string; arch: string; vm?: RemoteImage; container?: RemoteImage }> = {}
    for (const img of distroImages) {
      const key = `${img.release}|${img.arch}`
      if (!map[key]) map[key] = { release: img.release, arch: img.arch }
      if (img.type === 'virtual-machine') map[key].vm = img
      else if (img.type === 'container') map[key].container = img
    }
    return Object.values(map).sort((a, b) => a.release.localeCompare(b.release) || a.arch.localeCompare(b.arch))
  }

  // 已安装镜像 - 按分类分组
  const groupedInstalledImages = useMemo(() => {
    const groups: Record<string, InstalledImage[]> = {}
    for (const img of installedImages) {
      const key = img.category_name || '__uncategorized__'
      if (!groups[key]) groups[key] = []
      groups[key].push(img)
    }
    return Object.entries(groups).sort((a, b) => {
      if (a[0] === '__uncategorized__') return 1
      if (b[0] === '__uncategorized__') return -1
      return a[0].localeCompare(b[0])
    })
  }, [installedImages])

  const renderDownloadCell = (img: RemoteImage | undefined, typeLabel: string) => {
    if (!img) return <span className="text-xs text-muted">-</span>
    if (!selectedNode) return <span className="text-xs text-muted">{typeLabel}</span>

    if (img.stage === 'downloading') {
      return (
        <div className="flex items-center gap-1.5">
          <span className="text-xs text-tertiary font-number">{img.progress || 0}%</span>
          <button onClick={() => handleCancel(img)} className="text-muted hover:text-red-500" title="取消">
            <Square size={11} />
          </button>
        </div>
      )
    }
    if (img.stage === 'done') {
      return (
        <div className="flex items-center gap-1.5">
          <span className="inline-flex items-center gap-1 text-xs text-green-600"><CheckCircle size={12} />已下载</span>
          <button onClick={() => handleDeleteRemote(img)} className="text-muted hover:text-red-500" title="删除">
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
      <button onClick={() => handleDownload(img)} className="inline-flex items-center gap-1 text-xs text-tertiary hover:text-primary transition-colors">
        <Download size={12} />下载
      </button>
    )
  }

  const renderProgressBar = (img: RemoteImage | undefined) => {
    if (!img || img.stage !== 'downloading') return null
    const totalBytes = img.total_bytes || 0
    const hasReal = totalBytes > 0
    const downloadedBytes = hasReal ? Math.floor((img.progress || 0) * totalBytes / 100) : 0
    return (
      <td colSpan={6} className="px-4 py-1.5 bg-surface-secondary/50">
        <div className="flex items-center gap-3">
          {hasReal ? (
            <div className="flex-1 bg-surface-secondary rounded-full h-1.5">
              <div className="bg-primary h-1.5 rounded-full transition-all" style={{ width: `${img.progress || 0}%` }} />
            </div>
          ) : (
            <div className="flex-1 bg-surface-secondary rounded-full h-1.5 animate-pulse" />
          )}
          <span className="text-xs text-tertiary whitespace-nowrap font-number">
            {hasReal ? `${formatBytes(downloadedBytes)}/${formatBytes(totalBytes)}` : '下载中...'}
          </span>
          {img.speed_bps ? <span className="text-xs text-muted whitespace-nowrap font-number">{formatSpeed(img.speed_bps)}</span> : null}
        </div>
      </td>
    )
  }

  const nodeOptions = nodes.map((n) => ({ label: `${n.name} (${n.hostname}) ${n.is_online ? '在线' : '离线'}`, value: n.id }))

  // 渲染已安装镜像tab
  const renderInstalledTab = () => {
    if (loading) {
      return <div className="flex items-center justify-center py-20 text-muted"><RefreshCw size={20} className="animate-spin mr-2" />加载中...</div>
    }
    if (installedImages.length === 0) {
      return <div className="flex items-center justify-center py-20 text-muted text-sm">暂无已安装镜像，请先在"远程下载"Tab中下载镜像</div>
    }
    return (
      <div className="space-y-4">
        {/* 按分类分组的镜像列表 */}
        {groupedInstalledImages.map(([catName, catImages]) => {
          const displayName = catName === '__uncategorized__' ? '未分类' : catName
          return (
            <div key={catName} className="rounded-lg border border-surface bg-surface overflow-hidden">
              <div className="flex items-center gap-2 px-4 py-3 border-b border-surface-light bg-surface-secondary/80">
                <h3 className="text-sm font-semibold text-primary">{displayName}</h3>
                <span className="text-xs text-muted">{catImages.length} 个镜像</span>
              </div>
              <table className="data-table">
                <thead>
                  <tr>
                    <th>别名</th>
                    <th>显示名</th>
                    <th style={{ width: 80 }}>类型</th>
                    <th style={{ width: 80 }}>架构</th>
                    <th style={{ width: 80 }}>来源</th>
                    <th style={{ width: 96 }}>大小</th>
                    <th style={{ width: 80 }}>SSH</th>
                    <th style={{ width: 64 }}>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {catImages.map((img) => (
                    <tr key={img.id}>
                      <td className="text-secondary">{img.alias}</td>
                      <td className="text-secondary">{img.display_name || '-'}</td>
                      <td><span className="data-table-tag">{typeLabel(img.type)}</span></td>
                      <td className="text-tertiary">{formatArch(img.architecture)}</td>
                      <td className="text-tertiary">{sourceLabel(img.image_source)}</td>
                      <td className="text-tertiary font-number">{img.size ? formatBytes(img.size) : '-'}</td>
                      <td>
                        {img.install_ssh ? <span className="text-success">是</span> : <span className="text-muted">否</span>}
                      </td>
                      <td>
                        <button onClick={() => handleEditImage(img)} className="data-table-link-btn">
                          编辑
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )
        })}
      </div>
    )
  }

  // 渲染远程下载tab
  const renderRemoteTab = () => {
    if (loading) {
      return <div className="flex items-center justify-center py-20 text-muted"><RefreshCw size={20} className="animate-spin mr-2" />加载中...</div>
    }
    if (groupedRemoteImages.length === 0) {
      return <div className="flex items-center justify-center py-20 text-muted text-sm">暂无镜像数据</div>
    }
    return (
      <div className="space-y-4">
        {groupedRemoteImages.map(([distro, distroImages]) => {
          const info = getDistroInfo(distro)
          const rows = groupedByReleaseArch(distroImages)
          return (
            <div key={distro} className="rounded-lg border border-surface bg-surface overflow-hidden">
              <div className="flex items-center gap-3 px-4 py-3 border-b border-surface-light bg-surface-secondary/80">
                <img src={getOSImage(info.name)} alt={info.name} className="w-7 h-7 rounded object-contain flex-shrink-0" />
                <div className="min-w-0">
                  <div className="flex items-baseline gap-2">
                    <h2 className="text-sm font-semibold text-primary">{info.name}</h2>
                    <span className="text-xs text-muted">{distroImages.length} 个镜像</span>
                  </div>
                  {info.desc && <p className="text-xs text-tertiary leading-relaxed mt-0.5 line-clamp-2">{info.desc}</p>}
                </div>
              </div>
              <table className="data-table">
                <thead>
                  <tr>
                    <th>版本</th>
                    <th style={{ width: 80 }}>架构</th>
                    <th style={{ width: 96 }}>
                      <span className="inline-flex items-center gap-1"><Monitor size={12} />VM大小</span>
                    </th>
                    <th style={{ width: 144 }}>VM下载</th>
                    <th style={{ width: 96 }}>
                      <span className="inline-flex items-center gap-1"><Box size={12} />容器大小</span>
                    </th>
                    <th style={{ width: 144 }}>容器下载</th>
                  </tr>
                </thead>
                <tbody>
                  {rows.map((row) => {
                    const vmDownloading = row.vm?.stage === 'downloading'
                    const containerDownloading = row.container?.stage === 'downloading'
                    return (
                      <Fragment key={`${row.release}|${row.arch}`}>
                        <tr>
                          <td className="font-medium text-secondary">{info.name} {row.release}</td>
                          <td>
                            <span className="data-table-tag">{formatArch(row.arch)}</span>
                          </td>
                          <td className="text-tertiary font-number">
                            {row.vm?.total_bytes ? formatBytes(row.vm.total_bytes) : '-'}
                          </td>
                          <td>{renderDownloadCell(row.vm, '虚拟机')}</td>
                          <td className="text-tertiary font-number">
                            {row.container?.total_bytes ? formatBytes(row.container.total_bytes) : '-'}
                          </td>
                          <td>{renderDownloadCell(row.container, '容器')}</td>
                        </tr>
                        {vmDownloading && (
                          <tr className="border-b border-surface-light">
                            {renderProgressBar(row.vm)}
                          </tr>
                        )}
                        {containerDownloading && (
                          <tr className="border-b border-surface-light">
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
    )
  }

  return (
    <PageLayout
      leftSlot={
        <>
          {activeTab === 'remote' && (
            <Select
              value={imageSource}
              editable
              placeholder="镜像源"
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
          )}
          {activeTab === 'installed' && (
            <Select
              value={installedFilterType}
              options={[{ label: '全部类型', value: '' }, { label: '容器', value: 'container' }, { label: '虚拟机', value: 'virtual-machine' }]}
              placeholder="类型筛选"
              onChange={(v) => setInstalledFilterType(String(v))}
            />
          )}
          <Select
            value={selectedNode}
            options={nodeOptions}
            placeholder="选择节点"
            emptyText="无可用节点"
            onChange={(v) => setSelectedNode(String(v))}
          />
        </>
      }
      rightSlot={
        <Button onClick={handleRefresh} disabled={refreshing} icon={<RefreshCw size={16} className={refreshing ? 'animate-spin' : ''} />}>
          {refreshing ? '刷新中' : '刷新'}
        </Button>
      }
    >
      <div className="page-transition__content" style={{ flex: 1, overflow: 'auto' }}>
        {/* Tab 切换 */}
        <div className="flex items-center gap-1 mb-4 border-b border-surface-light">
          <button
            onClick={() => { setActiveTab('installed'); if (selectedNode) { fetchInstalledImages(selectedNode); fetchCategories(selectedNode) } }}
            className={`px-4 py-2 text-sm font-medium transition-colors border-b-2 ${activeTab === 'installed' ? 'border-primary text-primary' : 'border-transparent text-muted hover:text-secondary'}`}
          >
            <span className="inline-flex items-center gap-1.5"><Server size={14} />已安装镜像</span>
          </button>
          <button
            onClick={() => { setActiveTab('remote'); if (selectedNode) fetchRemoteImages(selectedNode) }}
            className={`px-4 py-2 text-sm font-medium transition-colors border-b-2 ${activeTab === 'remote' ? 'border-primary text-primary' : 'border-transparent text-muted hover:text-secondary'}`}
          >
            <span className="inline-flex items-center gap-1.5"><Download size={14} />远程下载</span>
          </button>
        </div>

        {activeTab === 'installed' ? renderInstalledTab() : renderRemoteTab()}
      </div>

      {/* 删除远程镜像确认 */}
      <Modal
        open={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        title="删除镜像"
        confirmMode
        confirmText="确认删除"
        confirmVariant="danger"
        onConfirm={() => { setConfirmOpen(false); doDelete() }}
        width={440}
      >
        确认删除镜像 {confirmImage?.alias} ({confirmImage?.type === 'container' ? '容器' : '虚拟机'}, {confirmImage ? formatArch(confirmImage.arch) : ''})？
      </Modal>

      {/* 编辑镜像 - SlidePanel */}
      <SlidePanel
        open={editPanelOpen}
        onClose={() => setEditPanelOpen(false)}
        title="编辑镜像"
        width={480}
        footer={
          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setEditPanelOpen(false)}>取消</Button>
            <Button onClick={doEditImage}>保存</Button>
          </div>
        }
      >
        <div className="space-y-5">
          <div>
            <label className="block text-xs text-tertiary mb-1.5">镜像别名</label>
            <span className="text-sm text-secondary">{editingImage?.alias}</span>
          </div>
          <div>
            <label className="block text-xs text-tertiary mb-1.5">显示名称</label>
            <input
              type="text"
              value={editDisplayName}
              onChange={(e) => setEditDisplayName(e.target.value)}
              placeholder="自定义显示名"
              className="w-full px-3 py-2 text-sm border border-gray-200 rounded-lg outline-none focus:border-primary transition-colors"
              style={{ fontFamily: 'inherit' }}
            />
          </div>
          <div>
            <label className="block text-xs text-tertiary mb-1.5">分类</label>
            <div className="flex flex-wrap items-center gap-1.5 px-2 py-2 border border-gray-200 rounded-lg min-h-[38px]" style={{ fontFamily: 'inherit' }}>
              {editCategoryName && (
                <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bg-gray-100 text-xs text-secondary">
                  {editCategoryName}
                  <button
                    onClick={() => setEditCategoryName('')}
                    className="text-muted hover:text-red-500"
                  >
                    <X size={12} />
                  </button>
                </span>
              )}
              <input
                type="text"
                value={categoryInput}
                onChange={(e) => setCategoryInput(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault()
                    const val = categoryInput.trim()
                    if (val) {
                      setEditCategoryName(val)
                      setCategoryInput('')
                    }
                  }
                  if (e.key === 'Backspace' && !categoryInput && editCategoryName) {
                    setEditCategoryName('')
                  }
                }}
                placeholder={editCategoryName ? '' : '输入分类名称后回车'}
                className="flex-1 min-w-[120px] text-sm outline-none bg-transparent"
                style={{ fontFamily: 'inherit' }}
              />
            </div>
            {categories.length > 0 && (
              <div className="flex flex-wrap gap-1 mt-2">
                {categories.filter(c => c.image_type === editingImage?.type).map((cat) => (
                  <button
                    key={cat.id}
                    onClick={() => setEditCategoryName(cat.name)}
                    className="px-2 py-0.5 rounded-md bg-gray-50 text-xs text-muted hover:bg-gray-100 hover:text-secondary transition-colors"
                  >
                    {cat.name}
                  </button>
                ))}
              </div>
            )}
          </div>
          <div>
            <label className="flex items-center gap-2 text-sm text-secondary cursor-pointer">
              <input
                type="checkbox"
                checked={editInstallSSH}
                onChange={(e) => setEditInstallSSH(e.target.checked)}
                className="rounded"
              />
              创建实例时默认安装 SSH
            </label>
          </div>
        </div>
      </SlidePanel>
    </PageLayout>
  )
}
