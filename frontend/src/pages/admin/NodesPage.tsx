import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Plus, Copy, Check, Cpu, MemoryStick, HardDrive, Activity, CheckCircle2, XCircle } from 'lucide-react'
import apiClient from '@/api/client'
import { DataTable, type Column } from '@/components/DataTable/DataTable'
import { Button } from '@/components/Button/Button'
import { SlidePanel } from '@/components/SlidePanel/SlidePanel'
import { Select } from '@/components/Select/Select'
import { Modal } from '@/components/Modal/Modal'
import { SearchInput } from '@/components/SearchInput/SearchInput'
import { FilterBar, type FilterField } from '@/components/FilterBar/FilterBar'
import { PageLayout } from '@/components/PageLayout/PageLayout'
import { useListQuery } from '@/hooks/useListQuery'
import { useToastStore } from '@/stores/toast'
import { getOSImage } from '@/utils/osImageHelper'
import '@/components/PageTransition/PageTransition.css'

interface IPProbe {
  interface: string
  address: string
  prefix_len: number
  scope: string
  gateway?: string
}

interface IPv4Prefix {
  interface: string
  address: string
  prefix: string
  prefix_len: number
  subnet_mask: string
  gateway: string
  source: string
}

interface IPv6Prefix {
  address: string
  prefix_len: number
  interface: string
  gateway: string
}

interface GatewayProbe {
  family: string
  interface: string
  gateway: string
}

interface GPUInfo {
  name: string
  vendor: string
  driver: string
  type: string
}

interface MemModule {
  locator: string
  size: string
  type: string
  speed: string
  manufacturer: string
  part_number: string
  serial_number: string
}

interface DiskSMART {
  available: boolean
  life_used_percent?: number
  power_on_hours?: number
  power_cycle_count?: number
  read_data_bytes?: number
  written_data_bytes?: number
  read_commands?: number
  write_commands?: number
  wear_leveling_count?: string
  erase_count?: string
  media_errors?: number
}

interface EnvCheck {
  key: string
  label: string
  ok: boolean
  required: boolean
  detail: string
}

interface SystemInfo {
  generated_at?: string
  hostname?: string
  os?: string
  kernel?: string
  cpu?: {
    model: string
    cores: number
    threads: number
    architecture: string
    flags: string[]
    has_integrated_gpu: boolean
    virtualization: boolean
    virtualization_key: string
  }
  memory?: {
    total_mb: number
    used_mb: number
    free_mb: number
  }
  mem_modules?: MemModule[]
  disks?: Array<{
    name: string
    path: string
    model: string
    serial: string
    size_bytes: number
    type: string
    virtual: boolean
    rotational: boolean
    mountpoints: string[]
    health: string
    health_detail: string
    smart: DiskSMART
  }>
  network_interfaces?: Array<{
    name: string
    mac: string
    state: string
    speed_mbps: number
    driver: string
    model: string
    ipv4: IPProbe[]
    ipv6: IPProbe[]
  }>
  public_ipv4?: string[]
  ipv4_addresses?: IPProbe[]
  ipv4_prefixes?: IPv4Prefix[]
  ipv6_addresses?: IPProbe[]
  ipv6_prefixes?: IPv6Prefix[]
  gateways?: GatewayProbe[]
  gpus?: GPUInfo[]
  runtime?: {
    lxc_available: boolean
    kvm_available: boolean
    dev_kvm: boolean
    nested_virtualization: boolean
    nested_detail: string
  }
  system?: {
    uptime_seconds: number
    uptime_text: string
    process_count: number
  }
  environment?: EnvCheck[]
}

interface Node {
  id: string
  name: string
  token: string
  hostname: string
  ip_address: string
  ipv6_address: string
  country_code: string
  status: string
  is_online: boolean
  instance_count: number
  running_count: number
  created_at: string
  incus_socket_path: string
  metrics_interval: number
  heartbeat_interval: number
  network_interface: string
  enable_nat: boolean
  console_bind_addr: string
  agent_url: string
  image_remote_url: string
  system_info: string | SystemInfo
  total_cpu: number
  total_memory: number
  total_disk: number
  used_cpu: number
  used_memory: number
  used_disk: number
  net_in: number
  net_out: number
  uptime: number
  last_heartbeat: string | null
}

export default function NodesPage() {
  const { t } = useTranslation()
  const toast = useToastStore()
  const { data: nodes, total, loading, page, perPage, search, filters, setPage, setPerPage, setSearch, setFilter, refresh } = useListQuery<Node>('/nodes', {}, {
    wsUrl: '/ws/nodes',
    wsType: 'node_heartbeat',
    wsUpdate: (prev: any[], msg: any) => {
      if (!msg.node_id || !msg.payload) return prev
      const p = msg.payload
      return prev.map((n: any) =>
        n.id === msg.node_id
          ? {
              ...n,
              status: p.status ?? n.status,
              is_online: p.is_online ?? n.is_online,
              used_cpu: p.used_cpu ?? n.used_cpu,
              used_memory: p.used_memory ?? n.used_memory,
              total_memory: p.mem_total ?? n.total_memory,
              used_disk: p.used_disk ?? n.used_disk,
              total_disk: p.disk_total ?? n.total_disk,
              net_in: p.net_in ?? n.net_in,
              net_out: p.net_out ?? n.net_out,
              uptime: p.uptime ?? n.uptime,
              instance_count: p.instance_count ?? n.instance_count,
              running_count: p.running_count ?? n.running_count,
              last_heartbeat: p.last_heartbeat ?? n.last_heartbeat,
            }
          : n
      )
    },
    wsRefreshTypes: ['data_refresh'],
  })
  const [panelOpen, setPanelOpen] = useState(false)
  const [configOpen, setConfigOpen] = useState(false)
  const [detailOpen, setDetailOpen] = useState(false)
  const [tokenOpen, setTokenOpen] = useState(false)
  const [currentNode, setCurrentNode] = useState<Node | null>(null)
  const [nodeName, setNodeName] = useState('')
  const [newToken, setNewToken] = useState('')

  // 确认弹窗状态
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [confirmTitle, setConfirmTitle] = useState('')
  const [confirmMessage, setConfirmMessage] = useState('')
  const [confirmAction, setConfirmAction] = useState<() => void>(() => {})
  const [confirmRequireInput, setConfirmRequireInput] = useState(false)
  const [confirmRequireLabel, setConfirmRequireLabel] = useState('')
  const [confirmRequireValue, setConfirmRequireValue] = useState('')

  const handleCreate = async () => {
    if (!nodeName.trim()) return
    const res = await apiClient.post('/nodes', { name: nodeName })
    const data = res.data
    toast.success(`节点创建成功: ${data.name}`)
    setNewToken(data.token)
    setTokenOpen(true)
    setNodeName('')
    setPanelOpen(false)
    refresh()
  }

  const handleDelete = async (id: string, name: string) => {
    setConfirmTitle('删除节点')
    setConfirmMessage(`确认删除节点「${name}」？`)
    setConfirmRequireInput(true)
    setConfirmRequireLabel('请输入节点名称以确认')
    setConfirmRequireValue(name)
    setConfirmAction(() => async () => {
      await apiClient.delete(`/nodes/${id}`)
      toast.success('节点删除成功')
      refresh()
    })
    setConfirmOpen(true)
  }

  const copyToken = (token: string) => {
    navigator.clipboard.writeText(token)
    toast.success('Token 已复制到剪贴板')
  }

  const parseSystemInfo = (node: Node | null): SystemInfo | null => {
    if (!node || !node.system_info) return null
    if (typeof node.system_info === 'string') {
      try {
        return JSON.parse(node.system_info) as SystemInfo
      } catch {
        return null
      }
    }
    return node.system_info as SystemInfo
  }

  const [cfgForm, setCfgForm] = useState({
    incus_socket_path: '/var/lib/incus/unix.socket',
    metrics_interval: 1,
    heartbeat_interval: 1,
    enable_nat: true,
    agent_url: '',
    image_remote_url: '',
  })

  useEffect(() => {
    if (currentNode) {
      setCfgForm({
        incus_socket_path: currentNode.incus_socket_path || '/var/lib/incus/unix.socket',
        metrics_interval: currentNode.metrics_interval || 1,
        heartbeat_interval: currentNode.heartbeat_interval || 1,
        enable_nat: currentNode.enable_nat !== false,
        agent_url: currentNode.agent_url || '',
        image_remote_url: currentNode.image_remote_url || '',
      })
    }
  }, [currentNode])

  const handleSaveConfig = async () => {
    if (!currentNode) return
    await apiClient.put(`/nodes/${currentNode.id}/config`, cfgForm)
    toast.success('配置已保存并下发给 Agent')
    await refresh()
    setConfigOpen(false)
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
    return `${val.toFixed(idx === 0 ? 0 : 1)} ${units[idx]}`
  }

  const formatUptime = (seconds: number): string => {
    if (!seconds || seconds <= 0) return '-'
    const d = Math.floor(seconds / 86400)
    const h = Math.floor((seconds % 86400) / 3600)
    const m = Math.floor((seconds % 3600) / 60)
    if (d > 0) return `${d}天${h}小时`
    if (h > 0) return `${h}小时${m}分钟`
    return `${m}分钟`
  }

  const getOSName = (node: Node): string => {
    const info = parseSystemInfo(node)
    return info?.os || ''
  }

  const getArch = (node: Node): string => {
    const info = parseSystemInfo(node)
    return info?.cpu?.architecture || '-'
  }

  const getFlagUrl = (code: string): string => {
    if (!code) return ''
    return `https://flagcdn.com/${code.toLowerCase()}.svg`
  }

  const getIPv6 = (node: Node): string => {
    return node.ipv6_address || ''
  }

  const [copiedId, setCopiedId] = useState<string | null>(null)
  const copyIP = (id: string, ip: string) => {
    navigator.clipboard.writeText(ip)
    setCopiedId(id)
    setTimeout(() => setCopiedId(null), 1500)
  }

  const getProgressColor = (percent: number): string => {
    if (percent >= 90) return 'metric-progress__fill--red'
    if (percent >= 70) return 'metric-progress__fill--yellow'
    return 'metric-progress__fill--green'
  }

  const columns: Column<Node>[] = [
    {
      key: 'id',
      title: 'ID',
      width: 100,
      render: (row: Node) => (
        <span className="text-sm font-number text-tertiary">{row.id.slice(0, 8)}</span>
      ),
    },
    {
      key: 'name',
      title: t('node.name'),
      width: 160,
      render: (row: Node) => (
        <button
          className="data-table-link-btn font-medium"
          onClick={() => { setCurrentNode(row); setDetailOpen(true) }}
        >
          {row.name}
        </button>
      ),
    },
    {
      key: 'arch',
      title: t('node.arch'),
      width: 80,
      render: (row: Node) => getArch(row),
    },
    {
      key: 'ip_address',
      title: t('node.ipAddress'),
      width: 220,
      render: (row: Node) => {
        const ipv4 = row.ip_address || ''
        const ipv6 = getIPv6(row)
        if (!ipv4 && !ipv6) return '-'
        const flagUrl = getFlagUrl(row.country_code)
        return (
          <div className="ip-cell">
            {ipv4 && (
              <div className="ip-cell__line">
                {flagUrl && <img src={flagUrl} alt={row.country_code} className="ip-cell__flag" />}
                <span>{ipv4}</span>
                <button
                  className="ip-cell__copy"
                  onClick={() => copyIP(row.id + '_v4', ipv4)}
                  title="复制"
                >
                  {copiedId === row.id + '_v4' ? <Check size={12} /> : <Copy size={12} />}
                </button>
              </div>
            )}
            {ipv6 && (
              <div className="ip-cell__line">
                <div className="ip-cell__v6">
                  <span className="ip-cell__v6-text">{ipv6}</span>
                </div>
                <button
                  className="ip-cell__copy"
                  onClick={() => copyIP(row.id + '_v6', ipv6)}
                  title="复制"
                >
                  {copiedId === row.id + '_v6' ? <Check size={12} /> : <Copy size={12} />}
                </button>
              </div>
            )}
          </div>
        )
      },
    },
    {
      key: 'status',
      title: t('node.status'),
      width: 80,
      render: (row: Node) => (
        <span className={`data-table-tag ${row.is_online ? 'data-table-tag--online' : 'data-table-tag--offline'}`}>
          {row.is_online ? t('node.statusOnline') : t('node.statusOffline')}
        </span>
      ),
    },
    { key: 'instance_count', title: t('node.instances'), width: 70, render: (row: Node) => <span className="font-number">{String(row.instance_count ?? 0)}</span> },
    {
      key: 'os',
      title: t('node.os'),
      width: 180,
      render: (row: Node) => {
        const osName = getOSName(row)
        if (!osName) return '-'
        const icon = getOSImage(osName)
        return (
          <div className="os-cell">
            <img src={icon} alt={osName} className="os-cell__icon" />
            <span className="os-cell__name">{osName}</span>
          </div>
        )
      },
    },
    {
      key: 'cpu',
      title: t('node.cpuUsage'),
      width: 160,
      render: (row: Node) => {
        const used = row.used_cpu ?? 0
        const total = row.total_cpu ?? 0
        return (
          <div className="metric-cell">
            <div className="metric-cell__header">
              <span className="metric-cell__percent font-number">{used.toFixed(1)}%</span>
              <span className="metric-cell__detail font-number">/ {total}核</span>
            </div>
            <div className="metric-progress">
              <div className={`metric-progress__fill ${getProgressColor(used)}`} style={{ width: `${Math.min(used, 100)}%` }} />
            </div>
          </div>
        )
      },
    },
    {
      key: 'memory',
      title: t('node.memoryUsage'),
      width: 160,
      render: (row: Node) => {
        const used = row.used_memory ?? 0
        const total = row.total_memory ?? 0
        if (!total) return '-'
        const percent = (used / total) * 100
        return (
          <div className="metric-cell">
            <div className="metric-cell__header">
              <span className="metric-cell__percent font-number">{percent.toFixed(1)}%</span>
              <span className="metric-cell__detail font-number">{formatBytes(used * 1024 * 1024)} / {formatBytes(total * 1024 * 1024)}</span>
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
      title: t('node.diskUsage'),
      width: 160,
      render: (row: Node) => {
        const used = row.used_disk ?? 0
        const total = row.total_disk ?? 0
        if (!total) return '-'
        const percent = (used / total) * 100
        return (
          <div className="metric-cell">
            <div className="metric-cell__header">
              <span className="metric-cell__percent font-number">{percent.toFixed(1)}%</span>
              <span className="metric-cell__detail font-number">{used}GB / {total}GB</span>
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
      title: t('node.networkIO'),
      width: 120,
      render: (row: Node) => {
        const rx = row.net_in ?? 0
        const tx = row.net_out ?? 0
        return (
          <div className="net-cell font-number">
            <span className="net-cell__up">↑{formatNetSpeed(tx)}</span>
            <span className="net-cell__down">↓{formatNetSpeed(rx)}</span>
          </div>
        )
      },
    },
    {
      key: 'uptime',
      title: t('node.uptime'),
      width: 100,
      render: (row: Node) => <span className="font-number">{formatUptime(row.uptime ?? 0)}</span>,
    },
    {
      key: 'action',
      title: t('node.action'),
      width: 200,
      render: (row: Node) => (
        <div className="flex items-center gap-3">
          <button
            className="data-table-link-btn"
            onClick={() => { setCurrentNode(row); setDetailOpen(true) }}
          >
            {t('node.viewDetails')}
          </button>
          <button
            className="data-table-link-btn"
            onClick={() => { setCurrentNode(row); setConfigOpen(true) }}
          >
            {t('node.config')}
          </button>
          <button
            className="data-table-link-btn"
            style={{ color: '#dc2626' }}
            onClick={() => handleDelete(row.id, row.name)}
          >
            {t('node.delete')}
          </button>
        </div>
      ),
    },
  ]

  return (
    <PageLayout
      leftSlot={
        <>
          <SearchInput value={search} placeholder="搜索节点名称、主机名、IP" onChange={setSearch} />
          <FilterBar
            fields={[
              { key: 'status', label: '状态', options: [
                { label: '在线', value: 'online' },
                { label: '离线', value: 'offline' },
              ] },
            ] as FilterField[]}
            values={filters}
            onChange={setFilter}
          />
        </>
      }
      rightSlot={
        <Button icon={<Plus size={16} />} onClick={() => setPanelOpen(true)}>
          {t('node.addNode')}
        </Button>
      }
    >
      <DataTable
        columns={columns}
        data={nodes}
        rowKey={(r) => r.id}
        loading={loading}
        pagination={{ page, size: perPage, total }}
        onPageChange={setPage}
        onSizeChange={setPerPage}
      />

      {/* 新建节点 */}
      <SlidePanel open={panelOpen} onClose={() => setPanelOpen(false)} title="新建节点"
        footer={
          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setPanelOpen(false)}>{t('common.cancel')}</Button>
            <Button onClick={handleCreate}>{t('common.confirm')}</Button>
          </div>
        }
      >
        <div className="space-y-4">
          <label className="block text-sm font-medium text-primary">节点名称</label>
          <input
            className="w-full rounded-lg border border-surface px-3 py-2 text-sm focus:border-surface-strong focus:outline-none focus:ring-2 focus:ring-black/5"
            placeholder="输入节点名称"
            value={nodeName}
            onChange={(e) => setNodeName(e.target.value)}
          />
        </div>
      </SlidePanel>

      {/* 显示 Token */}
      <SlidePanel open={tokenOpen} onClose={() => setTokenOpen(false)} title="节点创建成功"
        footer={
          <div className="flex justify-end gap-2">
            <Button onClick={() => setTokenOpen(false)}>关闭</Button>
          </div>
        }
      >
        <div className="space-y-4">
          <p className="text-sm text-tertiary">请复制以下 Token 配置到 Agent 的 config.yaml 文件中：</p>
          <div className="bg-surface-secondary rounded-lg p-4 border border-surface">
            <pre className="text-sm font-mono text-secondary whitespace-pre-wrap break-all">
{`master: "wss://your-master-domain"
token: "${newToken}"`}
            </pre>
          </div>
          <Button onClick={() => copyToken(newToken)} icon={<Copy size={16} />}>
            复制 Token
          </Button>
        </div>
      </SlidePanel>

      {/* 宿主机详情 */}
      <SlidePanel open={detailOpen} onClose={() => setDetailOpen(false)} title={`宿主机详情 - ${currentNode?.name || ''}`} width={960}>
        <div className="space-y-5 pr-2">
          {(() => {
            const info = parseSystemInfo(currentNode)
            if (!info) return (
              <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-3 text-sm text-yellow-800">
                等待 Agent 上报宿主机信息...
              </div>
            )

            const formatMB = (v: number) => v >= 1024 ? `${(v / 1024).toFixed(1)} GB` : `${v} MB`
            const formatBytes = (v: number) => {
              if (!v) return '-'
              const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
              let next = v, idx = 0
              while (next >= 1024 && idx < units.length - 1) { next /= 1024; idx++ }
              return `${next.toFixed(idx === 0 ? 0 : 1)} ${units[idx]}`
            }
            const diskTypeLabel = (d: any) => d.virtual || d.type === 'Virtual' ? '虚拟磁盘' : (d.type || (d.rotational ? 'HDD' : 'SSD'))
            const diskHealthLabel = (h: string) => {
              switch (h) {
                case 'ok': return '健康'
                case 'failed': return '异常'
                case 'virtual': return '虚拟磁盘'
                default: return '未知'
              }
            }
            const gpuTypeLabel = (t: string) => t === 'integrated' ? '核显' : t === 'discrete' ? '独显' : t || '-'

            return (
              <>
                {/* 概览卡片 */}
                <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
                  <div className="rounded-lg border border-surface bg-surface px-3 py-3">
                    <div className="mb-2 flex items-center gap-2 text-xs font-medium text-tertiary"><Cpu className="h-4 w-4" />CPU</div>
                    <div className="line-clamp-2 break-words text-sm font-semibold text-primary" title={info.cpu?.model || 'Unknown'}>{info.cpu?.model || 'Unknown'}</div>
                    <div className="mt-1 truncate text-xs text-tertiary">{info.cpu ? `${info.cpu.cores} 核 / ${info.cpu.threads} 线程` : '-'}</div>
                  </div>
                  <div className="rounded-lg border border-surface bg-surface px-3 py-3">
                    <div className="mb-2 flex items-center gap-2 text-xs font-medium text-tertiary"><MemoryStick className="h-4 w-4" />RAM</div>
                    <div className="font-number text-sm font-semibold text-primary">{info.memory ? formatMB(info.memory.total_mb) : '-'}</div>
                    <div className="font-number mt-1 truncate text-xs text-tertiary">{info.memory ? `${formatMB(info.memory.used_mb)} 已用` : '-'}</div>
                  </div>
                  <div className="rounded-lg border border-surface bg-surface px-3 py-3">
                    <div className="mb-2 flex items-center gap-2 text-xs font-medium text-tertiary"><HardDrive className="h-4 w-4" />DISK</div>
                    <div className="font-number text-sm font-semibold text-primary">{info.disks?.length || 0} 块硬盘</div>
                    <div className="mt-1 truncate text-xs text-tertiary">{info.disks?.map(d => diskTypeLabel(d)).filter(Boolean).join(' / ') || 'Unknown'}</div>
                  </div>
                  <div className="rounded-lg border border-surface bg-surface px-3 py-3">
                    <div className="mb-2 flex items-center gap-2 text-xs font-medium text-tertiary"><Activity className="h-4 w-4" />运行状态</div>
                    <div className="text-sm font-semibold text-primary">{info.system?.uptime_text || '-'}</div>
                    <div className="font-number mt-1 truncate text-xs text-tertiary">{info.system ? `${info.system.process_count} 个进程` : '-'}</div>
                  </div>
                </div>

                {/* 系统概览 */}
                <section>
                  <h2 className="mb-2 text-sm font-semibold text-primary">系统概览</h2>
                  <div className="overflow-hidden rounded-lg border border-surface bg-surface">
                    {[
                      ['主机名', info.hostname],
                      ['操作系统', info.os],
                      ['内核', info.kernel],
                      ['生成时间', info.generated_at],
                      ['CPU 架构', info.cpu?.architecture],
                      ['CPU 虚拟化指令', info.cpu?.virtualization ? `支持 (${info.cpu.virtualization_key})` : '未检测到'],
                      ['CPU 核显', info.cpu?.has_integrated_gpu ? '检测到' : '未检测到'],
                      ['显卡', info.gpus?.length ? `${info.gpus.length} 个` : '未检测到'],
                      ['运行能力-容器', info.runtime ? (info.runtime.lxc_available ? '可创建容器' : '不可创建容器') : '-'],
                      ['运行能力-虚拟机', info.runtime ? (info.runtime.kvm_available ? '可创建虚拟机' : '不可创建虚拟机') : '-'],
                      ['KVM 嵌套虚拟化', info.runtime ? `${info.runtime.nested_virtualization ? '支持' : '未检测到'} (${info.runtime.nested_detail || '-'})` : '-'],
                    ].map(([label, value]) => (
                      <div key={label} className="grid gap-2 border-b border-surface-light px-3 py-2 text-xs last:border-b-0 md:grid-cols-[160px_1fr]">
                        <div className="font-medium text-tertiary">{label}</div>
                        <div className="font-number whitespace-pre-wrap break-words text-secondary">{value || '-'}</div>
                      </div>
                    ))}
                  </div>
                </section>

                {/* 公网与路由 */}
                <section>
                  <h2 className="mb-2 text-sm font-semibold text-primary">公网与路由</h2>
                  <div className="overflow-hidden rounded-lg border border-surface bg-surface">
                    {[
                      ['公网 IPv4', info.public_ipv4?.length ? info.public_ipv4.join('\n') : '未检测到'],
                      ['IPv4 地址', info.ipv4_addresses?.length ? info.ipv4_addresses.map(ip => `${ip.address}/${ip.prefix_len} (${ip.interface})`).join('\n') : '未检测到'],
                      ['IPv4 段', info.ipv4_prefixes?.length ? info.ipv4_prefixes.map(p => `${p.prefix} mask ${p.subnet_mask} via ${p.gateway || '-'} dev ${p.interface} [${p.source}]`).join('\n') : '未检测到'],
                      ['IPv6 地址', info.ipv6_addresses?.length ? info.ipv6_addresses.map(ip => `${ip.address}/${ip.prefix_len} (${ip.interface})`).join('\n') : '未检测到'],
                      ['IPv6 段', info.ipv6_prefixes?.length ? info.ipv6_prefixes.map(p => `${p.address}/${p.prefix_len} via ${p.gateway || '-'} dev ${p.interface}`).join('\n') : '未检测到'],
                      ['网关', info.gateways?.length ? info.gateways.map(g => `${g.family}: ${g.gateway || '-'} dev ${g.interface || '-'}`).join('\n') : '未检测到'],
                    ].map(([label, value]) => (
                      <div key={label} className="grid gap-2 border-b border-surface-light px-3 py-2 text-xs last:border-b-0 md:grid-cols-[160px_1fr]">
                        <div className="font-medium text-tertiary">{label}</div>
                        <div className="font-number whitespace-pre-wrap break-words text-secondary">{value || '-'}</div>
                      </div>
                    ))}
                  </div>
                </section>

                {/* 内存条 */}
                {(info.mem_modules?.length ?? 0) > 0 && (
                  <section>
                    <h2 className="mb-2 text-sm font-semibold text-primary">内存条</h2>
                    <div className="overflow-x-auto rounded-lg border border-surface bg-surface">
                      <table className="w-full text-xs">
                        <thead>
                          <tr className="border-b border-surface-light bg-surface-secondary text-left text-tertiary">
                            <th className="px-3 py-2 font-medium">插槽</th>
                            <th className="px-3 py-2 font-medium">容量</th>
                            <th className="px-3 py-2 font-medium">类型</th>
                            <th className="px-3 py-2 font-medium">频率</th>
                            <th className="px-3 py-2 font-medium">厂商</th>
                            <th className="px-3 py-2 font-medium">型号/序列号</th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-surface-light">
                          {info.mem_modules!.map((m, i) => (
                            <tr key={i} className="align-top">
                              <td className="px-3 py-2 text-secondary">{m.locator || '-'}</td>
                              <td className="px-3 py-2 text-secondary">{m.size || '-'}</td>
                              <td className="px-3 py-2 text-secondary">{m.type || '-'}</td>
                              <td className="px-3 py-2 text-secondary">{m.speed || '-'}</td>
                              <td className="px-3 py-2 text-secondary">{m.manufacturer || '-'}</td>
                              <td className="px-3 py-2 text-secondary">{[m.part_number, m.serial_number].filter(Boolean).join(' / ') || '-'}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </section>
                )}

                {/* 硬盘与健康 */}
                {(info.disks?.length ?? 0) > 0 && (
                  <section>
                    <h2 className="mb-2 text-sm font-semibold text-primary">硬盘与健康</h2>
                    <div className="overflow-x-auto rounded-lg border border-surface bg-surface">
                      <table className="w-full text-xs">
                        <thead>
                          <tr className="border-b border-surface-light bg-surface-secondary text-left text-tertiary">
                            <th className="px-3 py-2 font-medium">设备</th>
                            <th className="px-3 py-2 font-medium">型号</th>
                            <th className="px-3 py-2 font-medium">容量</th>
                            <th className="px-3 py-2 font-medium">类型</th>
                            <th className="px-3 py-2 font-medium">挂载点</th>
                            <th className="px-3 py-2 font-medium">健康</th>
                            <th className="px-3 py-2 font-medium">寿命</th>
                            <th className="px-3 py-2 font-medium">通电</th>
                            <th className="px-3 py-2 font-medium">读取</th>
                            <th className="px-3 py-2 font-medium">写入</th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-surface-light">
                          {info.disks!.map((d, i) => (
                            <tr key={i} className="align-top">
                              <td className="max-w-[180px] whitespace-pre-wrap break-words px-3 py-2 text-secondary">{d.path || d.name}{d.serial ? `\n${d.serial}` : ''}</td>
                              <td className="px-3 py-2 text-secondary">{d.model || '-'}</td>
                              <td className="font-number px-3 py-2 text-secondary">{formatBytes(d.size_bytes)}</td>
                              <td className="px-3 py-2 text-secondary">{diskTypeLabel(d)}</td>
                              <td className="max-w-[120px] whitespace-pre-wrap break-words px-3 py-2 text-secondary">{d.mountpoints?.length ? d.mountpoints.join('\n') : '-'}</td>
                              <td className="px-3 py-2 text-secondary">{diskHealthLabel(d.health)}{d.health_detail ? `\n${d.health_detail}` : ''}</td>
                              <td className="font-number px-3 py-2 text-secondary">{d.virtual ? '不支持' : (d.smart.life_used_percent !== undefined ? `${d.smart.life_used_percent}% 已用\n${Math.max(0, 100 - d.smart.life_used_percent)}% 剩余` : '-')}</td>
                              <td className="font-number px-3 py-2 text-secondary">{d.virtual ? '不支持' : (d.smart.power_on_hours ? `${d.smart.power_on_hours} 小时` : '-')}</td>
                              <td className="font-number px-3 py-2 text-secondary">{d.virtual ? '不支持' : formatBytes(d.smart.read_data_bytes || 0)}</td>
                              <td className="font-number px-3 py-2 text-secondary">{d.virtual ? '不支持' : formatBytes(d.smart.written_data_bytes || 0)}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </section>
                )}

                {/* 网卡 */}
                {(info.network_interfaces?.length ?? 0) > 0 && (
                  <section>
                    <h2 className="mb-2 text-sm font-semibold text-primary">网卡</h2>
                    <div className="overflow-x-auto rounded-lg border border-surface bg-surface">
                      <table className="w-full text-xs">
                        <thead>
                          <tr className="border-b border-surface-light bg-surface-secondary text-left text-tertiary">
                            <th className="px-3 py-2 font-medium">网卡</th>
                            <th className="px-3 py-2 font-medium">状态</th>
                            <th className="px-3 py-2 font-medium">驱动/速率</th>
                            <th className="px-3 py-2 font-medium">MAC</th>
                            <th className="px-3 py-2 font-medium">IPv4</th>
                            <th className="px-3 py-2 font-medium">IPv6</th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-surface-light">
                          {info.network_interfaces!.map((n, i) => (
                            <tr key={i} className="align-top">
                              <td className="max-w-[180px] whitespace-pre-wrap break-words px-3 py-2 text-secondary">{n.name}{n.model ? `\n${n.model}` : ''}</td>
                              <td className="px-3 py-2 text-secondary">{n.state || '-'}</td>
                              <td className="font-number px-3 py-2 text-secondary">{n.driver || '-'}{n.speed_mbps > 0 ? `\n${n.speed_mbps} Mbps` : ''}</td>
                              <td className="font-number px-3 py-2 text-secondary">{n.mac || '-'}</td>
                              <td className="font-number max-w-[200px] whitespace-pre-wrap break-words px-3 py-2 text-secondary">{n.ipv4?.length ? n.ipv4.map(ip => `${ip.address}/${ip.prefix_len}`).join('\n') : '-'}</td>
                              <td className="font-number max-w-[200px] whitespace-pre-wrap break-words px-3 py-2 text-secondary">{n.ipv6?.length ? n.ipv6.map(ip => `${ip.address}/${ip.prefix_len} ${ip.scope}`).join('\n') : '-'}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </section>
                )}

                {/* 显卡 */}
                {(info.gpus?.length ?? 0) > 0 && (
                  <section>
                    <h2 className="mb-2 text-sm font-semibold text-primary">显卡</h2>
                    <div className="overflow-x-auto rounded-lg border border-surface bg-surface">
                      <table className="w-full text-xs">
                        <thead>
                          <tr className="border-b border-surface-light bg-surface-secondary text-left text-tertiary">
                            <th className="px-3 py-2 font-medium">名称</th>
                            <th className="px-3 py-2 font-medium">厂商</th>
                            <th className="px-3 py-2 font-medium">类型</th>
                            <th className="px-3 py-2 font-medium">驱动</th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-surface-light">
                          {info.gpus!.map((g, i) => (
                            <tr key={i} className="align-top">
                              <td className="px-3 py-2 text-secondary">{g.name}</td>
                              <td className="px-3 py-2 text-secondary">{g.vendor || '-'}</td>
                              <td className="px-3 py-2 text-secondary">{gpuTypeLabel(g.type)}</td>
                              <td className="px-3 py-2 text-secondary">{g.driver || '-'}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </section>
                )}

                {/* 环境支持 */}
                {(info.environment?.length ?? 0) > 0 && (
                  <section>
                    <h2 className="mb-2 text-sm font-semibold text-primary">环境支持</h2>
                    <div className="grid gap-2 md:grid-cols-2">
                      {info.environment!.map((item) => (
                        <div key={item.key} className="flex items-start gap-2 rounded-lg border border-surface bg-surface px-3 py-2">
                          {item.ok ? <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-green-600" /> : <XCircle className={`mt-0.5 h-4 w-4 shrink-0 ${item.required ? 'text-red-600' : 'text-amber-600'}`} />}
                          <div className="min-w-0">
                            <div className="flex flex-wrap items-center gap-2 text-xs font-medium text-secondary">
                              <span>{item.label}</span>
                              <span className={`rounded px-1.5 py-0.5 text-[10px] ${item.required ? 'bg-surface-secondary text-tertiary' : 'bg-blue-50 text-blue-700'}`}>
                                {item.required ? '必要' : '可选'}
                              </span>
                            </div>
                            <div className="font-number mt-1 break-all text-[11px] text-tertiary">{item.detail || '-'}</div>
                          </div>
                        </div>
                      ))}
                    </div>
                  </section>
                )}
              </>
            )
          })()}
        </div>
      </SlidePanel>

      {/* 宿主机配置 */}
      <SlidePanel open={configOpen} onClose={() => setConfigOpen(false)} title={`宿主机配置 - ${currentNode?.name || ''}`}
        footer={
          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setConfigOpen(false)}>{t('common.cancel')}</Button>
            <Button onClick={handleSaveConfig}>{t('common.confirm')}</Button>
          </div>
        }
      >
        <div className="space-y-4 pr-2">
          <div>
            <label className="block text-sm font-medium text-primary">Incus Socket 路径</label>
            <input
              className="w-full rounded-lg border border-surface px-3 py-2 text-sm"
              value={cfgForm.incus_socket_path}
              onChange={(e) => setCfgForm({ ...cfgForm, incus_socket_path: e.target.value })}
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-primary">监控间隔（秒）</label>
              <input
                type="number"
                className="w-full rounded-lg border border-surface px-3 py-2 text-sm"
                value={cfgForm.metrics_interval}
                onChange={(e) => setCfgForm({ ...cfgForm, metrics_interval: Number(e.target.value) })}
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-primary">心跳间隔（秒）</label>
              <input
                type="number"
                className="w-full rounded-lg border border-surface px-3 py-2 text-sm"
                value={cfgForm.heartbeat_interval}
                onChange={(e) => setCfgForm({ ...cfgForm, heartbeat_interval: Number(e.target.value) })}
              />
            </div>
          </div>
          <div>
            <label className="block text-sm font-medium text-primary">Agent URL（外部可访问地址）</label>
            <input
              type="text"
              placeholder="https://us-lax.testnode.com"
              className="w-full rounded-lg border border-surface px-3 py-2 text-sm"
              value={cfgForm.agent_url}
              onChange={(e) => setCfgForm({ ...cfgForm, agent_url: e.target.value })}
            />
            <p className="mt-1 text-xs text-tertiary">用于前端直连宿主机 VNC / WebSSH 等服务，需包含协议（http/https）</p>
          </div>
          <div>
            <label className="block text-sm font-medium text-primary">镜像源地址</label>
            <Select
              value={cfgForm.image_remote_url}
              editable
              placeholder="留空使用站点默认配置"
              options={[
                { label: 'Spiritlhl 镜像源(默认)', value: 'spiritlhl:' },
                { label: '官方镜像源', value: 'images:' },
                { label: '清华 TUNA 镜像源', value: 'https://mirrors.tuna.tsinghua.edu.cn/lxc-images' },
                { label: '中科院 ISCAS 镜像源', value: 'https://mirror.iscas.ac.cn/lxc-images' },
                { label: 'CERNET 教育网镜像源', value: 'https://mirrors.cernet.edu.cn/lxc-images' },
                { label: '南阳理工镜像源', value: 'https://mirror.nyist.edu.cn/lxc-images' },
              ]}
              onChange={(v) => setCfgForm({ ...cfgForm, image_remote_url: String(v) })}
            />
            <p className="mt-1 text-xs text-tertiary">留空则使用站点默认配置，可从预设选择或自行输入镜像站 URL</p>
          </div>
        </div>
      </SlidePanel>

      <Modal
        open={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        title={confirmTitle}
        confirmMode
        confirmText="确认"
        confirmVariant="danger"
        requireInput={confirmRequireInput}
        requireInputLabel={confirmRequireLabel}
        requireInputValue={confirmRequireValue}
        onConfirm={() => { setConfirmOpen(false); confirmAction() }}
        width={440}
      >
        {confirmMessage}
      </Modal>
    </PageLayout>
  )
}
