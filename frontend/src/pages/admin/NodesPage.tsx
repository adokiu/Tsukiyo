import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Plus, Server, Trash2, Copy, Settings } from 'lucide-react'
import apiClient from '@/api/client'
import { DataTable, type Column } from '@/components/DataTable/DataTable'
import { Button } from '@/components/Button/Button'
import { SlidePanel } from '@/components/SlidePanel/SlidePanel'
import { useToastStore } from '@/stores/toast'

interface SystemInfo {
  hostname?: string
  os?: string
  kernel?: string
  arch?: string
  uptime?: string
  process_count?: number
  cpu?: {
    model: string
    cores: number
    threads: number
    virtualization: string
    nested_kvm: boolean
  }
  memory?: {
    total: number
    used: number
  }
  disks?: Array<{
    device: string
    model: string
    size: string
    type: string
    mount_point: string
    health: string
  }>
  networks?: Array<{
    name: string
    status: string
    driver: string
    mac: string
    ipv4: string[]
    ipv6: string[]
    speed: string
  }>
  environment?: {
    systemd_version: string
    lxc_version: string
    iproute2_version: string
    conntrack_version: string
    libvirt_version: string
    qemu_version: string
    smartctl_version: string
    has_kvm: boolean
    has_ipv4_forward: boolean
    lxcfs_active: boolean
    libvirt_active: boolean
  }
}

interface Node {
  id: string
  name: string
  token: string
  hostname: string
  ip_address: string
  status: string
  is_online: boolean
  initialized: boolean
  instance_count: number
  created_at: string
  incus_socket_path: string
  metrics_interval: number
  heartbeat_interval: number
  network_interface: string
  enable_nat: boolean
  enable_firewall: boolean
  enable_security_scan: boolean
  scan_interval: number
  console_bind_addr: string
  default_storage_pool: string
  storage_pool_type: string
  storage_pool_source: string
  system_info: string | SystemInfo
}

export default function NodesPage() {
  const { t } = useTranslation()
  const toast = useToastStore()
  const [nodes, setNodes] = useState<Node[]>([])
  const [loading, setLoading] = useState(true)
  const [panelOpen, setPanelOpen] = useState(false)
  const [configOpen, setConfigOpen] = useState(false)
  const [tokenOpen, setTokenOpen] = useState(false)
  const [currentNode, setCurrentNode] = useState<Node | null>(null)
  const [nodeName, setNodeName] = useState('')
  const [newToken, setNewToken] = useState('')

  const fetchNodes = async () => {
    setLoading(true)
    try {
      const res = await apiClient.get('/nodes')
      setNodes(res.data.data || [])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { fetchNodes() }, [])

  const handleCreate = async () => {
    if (!nodeName.trim()) return
    const res = await apiClient.post('/nodes', { name: nodeName })
    const data = res.data
    toast.success(`节点创建成功: ${data.name}`)
    setNewToken(data.token)
    setTokenOpen(true)
    setNodeName('')
    setPanelOpen(false)
    fetchNodes()
  }

  const handleDelete = async (id: string) => {
    if (!confirm('确认删除该节点？')) return
    await apiClient.delete(`/nodes/${id}`)
    toast.success('节点删除成功')
    fetchNodes()
  }

  const copyToken = (token: string) => {
    navigator.clipboard.writeText(token)
    toast.success('Token 已复制到剪贴板')
  }

  const openConfig = (node: Node) => {
    setCurrentNode(node)
    setConfigOpen(true)
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
    enable_firewall: true,
    enable_security_scan: true,
    scan_interval: 300,
    default_storage_pool: 'default',
    storage_pool_type: 'dir' as 'dir' | 'zfs' | 'btrfs' | 'lvm',
    storage_pool_source: '/var/lib/incus/storage-pools/default',
  })

  useEffect(() => {
    if (currentNode) {
      setCfgForm({
        incus_socket_path: currentNode.incus_socket_path || '/var/lib/incus/unix.socket',
        metrics_interval: currentNode.metrics_interval || 1,
        heartbeat_interval: currentNode.heartbeat_interval || 1,
        enable_nat: currentNode.enable_nat !== false,
        enable_firewall: currentNode.enable_firewall !== false,
        enable_security_scan: currentNode.enable_security_scan !== false,
        scan_interval: currentNode.scan_interval || 300,
        default_storage_pool: currentNode.default_storage_pool || 'default',
        storage_pool_type: (currentNode.storage_pool_type || 'dir') as 'dir' | 'zfs' | 'btrfs' | 'lvm',
        storage_pool_source: currentNode.storage_pool_source || '/var/lib/incus/storage-pools/default',
      })
    }
  }, [currentNode])

  const handleSaveConfig = async () => {
    if (!currentNode) return
    await apiClient.put(`/nodes/${currentNode.id}/config`, cfgForm)
    toast.success('配置已保存并下发给 Agent')
    await fetchNodes()
    setConfigOpen(false)
  }

  const columns: Column<Node>[] = [
    { key: 'name', title: '名称' },
    { key: 'hostname', title: '主机名' },
    { key: 'ip_address', title: 'IP 地址' },
    {
      key: 'initialized',
      title: '初始化',
      render: (row: Node) => (
        <span className={`text-xs px-2 py-0.5 rounded-full ${row.initialized ? 'bg-green-100 text-green-700' : 'bg-yellow-100 text-yellow-700'}`}>
          {row.initialized ? '已配置' : '未配置'}
        </span>
      ),
    },
    { key: 'status', title: '状态' },
    { key: 'instance_count', title: '实例数' },
    { key: 'created_at', title: '创建时间' },
    {
      key: 'action',
      title: '操作',
      render: (row: Node) => (
        <div className="flex items-center gap-2">
          <button
            className="text-blue-600 hover:text-blue-800"
            title="复制 Token"
            onClick={() => copyToken(row.token)}
          >
            <Copy size={16} />
          </button>
          <button
            className="text-gray-600 hover:text-gray-800"
            title="初始化配置"
            onClick={() => openConfig(row)}
          >
            <Settings size={16} />
          </button>
          <button className="text-red-500 hover:text-red-700" onClick={() => handleDelete(row.id)}>
            <Trash2 size={16} />
          </button>
        </div>
      ),
    },
  ]

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Server size={22} className="text-black" />
          <h1 className="text-xl font-semibold text-black">{t('node.title')}</h1>
        </div>
        <Button icon={<Plus size={16} />} onClick={() => setPanelOpen(true)}>
          {t('node.addNode')}
        </Button>
      </div>

      <DataTable columns={columns} data={nodes} rowKey={(r) => r.id} loading={loading} />

      {/* 新建节点 */}
      <SlidePanel open={panelOpen} onClose={() => setPanelOpen(false)} title="新建节点">
        <div className="space-y-4">
          <label className="block text-sm font-medium text-black">节点名称</label>
          <input
            className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-black focus:outline-none focus:ring-2 focus:ring-black/5"
            placeholder="输入节点名称"
            value={nodeName}
            onChange={(e) => setNodeName(e.target.value)}
          />
        </div>
        <div slot="footer" className="flex justify-end gap-2 pt-4 border-t border-gray-200">
          <Button variant="ghost" onClick={() => setPanelOpen(false)}>{t('common.cancel')}</Button>
          <Button onClick={handleCreate}>{t('common.confirm')}</Button>
        </div>
      </SlidePanel>

      {/* 显示 Token */}
      <SlidePanel open={tokenOpen} onClose={() => setTokenOpen(false)} title="节点创建成功">
        <div className="space-y-4">
          <p className="text-sm text-gray-600">请复制以下 Token 配置到 Agent 的 config.yaml 文件中：</p>
          <div className="bg-gray-50 rounded-lg p-4 border border-gray-200">
            <pre className="text-sm font-mono text-gray-800 whitespace-pre-wrap break-all">
{`master: "wss://your-master-domain"
token: "${newToken}"`}
            </pre>
          </div>
          <Button onClick={() => copyToken(newToken)} icon={<Copy size={16} />}>
            复制 Token
          </Button>
        </div>
        <div slot="footer" className="flex justify-end gap-2 pt-4 border-t border-gray-200">
          <Button onClick={() => setTokenOpen(false)}>关闭</Button>
        </div>
      </SlidePanel>

      {/* 初始化配置 */}
      <SlidePanel open={configOpen} onClose={() => setConfigOpen(false)} title={`初始化配置 - ${currentNode?.name || ''}`}>
        <div className="space-y-4 max-h-[70vh] overflow-y-auto pr-2">
          {/* 宿主机探测报告 */}
          {(() => {
            const info = parseSystemInfo(currentNode)
            if (!info) return (
              <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-3 text-sm text-yellow-800">
                等待 Agent 上报宿主机信息...
              </div>
            )
            return (
              <div className="bg-gray-50 rounded-lg p-4 border border-gray-200 space-y-3">
                <h3 className="text-sm font-semibold text-black">宿主机探测报告</h3>
                <div className="grid grid-cols-2 gap-2 text-xs">
                  <div><span className="text-gray-500">主机名:</span> {info.hostname}</div>
                  <div><span className="text-gray-500">OS:</span> {info.os}</div>
                  <div><span className="text-gray-500">内核:</span> {info.kernel}</div>
                  <div><span className="text-gray-500">架构:</span> {info.arch}</div>
                  <div><span className="text-gray-500">运行时间:</span> {info.uptime}</div>
                  <div><span className="text-gray-500">进程数:</span> {info.process_count}</div>
                </div>
                {info.cpu && (
                  <div className="text-xs space-y-1">
                    <div className="font-medium text-black">CPU</div>
                    <div className="text-gray-600">{info.cpu.model} / {info.cpu.cores}核 / {info.cpu.threads}线程</div>
                    <div className="text-gray-600">虚拟化: {info.cpu.virtualization || '无'} {info.cpu.nested_kvm ? '(嵌套KVM)' : ''}</div>
                  </div>
                )}
                {info.memory && (
                  <div className="text-xs space-y-1">
                    <div className="font-medium text-black">内存</div>
                    <div className="text-gray-600">{(info.memory.total / 1024 / 1024 / 1024).toFixed(1)} GB 总计 / {(info.memory.used / 1024 / 1024 / 1024).toFixed(1)} GB 已用</div>
                  </div>
                )}
                {info.disks && info.disks.length > 0 && (
                  <div className="text-xs space-y-1">
                    <div className="font-medium text-black">磁盘 ({info.disks.length}块)</div>
                    {info.disks.map((d, i) => (
                      <div key={i} className="text-gray-600">{d.device} {d.size} {d.model} {d.health && `(${d.health})`}</div>
                    ))}
                  </div>
                )}
                {info.networks && info.networks.length > 0 && (
                  <div className="text-xs space-y-1">
                    <div className="font-medium text-black">网卡</div>
                    {info.networks.filter(n => n.name !== 'lo').map((n, i) => (
                      <div key={i} className="text-gray-600">
                        {n.name} {n.status === 'up' ? 'UP' : 'DOWN'} {n.driver} {n.mac}
                        {n.ipv4?.length > 0 && ` ${n.ipv4.join(', ')}`}
                      </div>
                    ))}
                  </div>
                )}
                {info.environment && (
                  <div className="text-xs space-y-1">
                    <div className="font-medium text-black">环境支持</div>
                    <div className="grid grid-cols-2 gap-1 text-gray-600">
                      <div>systemd: {info.environment.systemd_version || '-'}</div>
                      <div>incus: {info.environment.lxc_version || '-'}</div>
                      <div>KVM: {info.environment.has_kvm ? '可用' : '不可用'}</div>
                      <div>IPv4转发: {info.environment.has_ipv4_forward ? '开启' : '关闭'}</div>
                      <div>lxcfs: {info.environment.lxcfs_active ? '运行中' : '未运行'}</div>
                    </div>
                  </div>
                )}
              </div>
            )
          })()}

          <div>
            <label className="block text-sm font-medium text-black">Incus Socket 路径</label>
            <input
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm"
              value={cfgForm.incus_socket_path}
              onChange={(e) => setCfgForm({ ...cfgForm, incus_socket_path: e.target.value })}
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-black">监控间隔（秒）</label>
              <input
                type="number"
                className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm"
                value={cfgForm.metrics_interval}
                onChange={(e) => setCfgForm({ ...cfgForm, metrics_interval: Number(e.target.value) })}
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-black">心跳间隔（秒）</label>
              <input
                type="number"
                className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm"
                value={cfgForm.heartbeat_interval}
                onChange={(e) => setCfgForm({ ...cfgForm, heartbeat_interval: Number(e.target.value) })}
              />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={cfgForm.enable_firewall}
                onChange={(e) => setCfgForm({ ...cfgForm, enable_firewall: e.target.checked })}
              />
              启用防火墙
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={cfgForm.enable_security_scan}
                onChange={(e) => setCfgForm({ ...cfgForm, enable_security_scan: e.target.checked })}
              />
              启用安全扫描
            </label>
          </div>

          {/* 存储池配置 */}
          <div className="border-t border-gray-200 pt-4 space-y-3">
            <h4 className="text-sm font-semibold text-black">默认存储池</h4>
            <div>
              <label className="block text-sm font-medium text-black">存储池名称</label>
              <input
                className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm"
                value={cfgForm.default_storage_pool}
                onChange={(e) => setCfgForm({ ...cfgForm, default_storage_pool: e.target.value })}
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-black">存储类型</label>
              <select
                className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm"
                value={cfgForm.storage_pool_type}
                onChange={(e) => {
                  const t = e.target.value as 'dir' | 'zfs' | 'btrfs' | 'lvm'
                  const info = parseSystemInfo(currentNode)
                  const disks = info?.disks || []
                  const hasExtraDisk = disks.filter(d => d.type !== 'virtual' && d.device !== '/dev/sr0').length > 1
                  let source = cfgForm.storage_pool_source
                  if (t === 'dir') {
                    source = '/var/lib/incus/storage-pools/default'
                  } else if (!hasExtraDisk) {
                    // 没有其他盘，强制回 dir
                    toast.success('系统只有一块磁盘，已自动切换为 dir 类型')
                    setCfgForm({ ...cfgForm, storage_pool_type: 'dir', storage_pool_source: '/var/lib/incus/storage-pools/default' })
                    return
                  }
                  setCfgForm({ ...cfgForm, storage_pool_type: t, storage_pool_source: source })
                }}
              >
                <option value="dir">dir (目录)</option>
                <option value="zfs">zfs</option>
                <option value="btrfs">btrfs</option>
                <option value="lvm">lvm</option>
              </select>
            </div>
            {(() => {
              const info = parseSystemInfo(currentNode)
              const disks = info?.disks?.filter(d => d.type !== 'virtual' && d.device !== '/dev/sr0') || []
              const hasExtraDisk = disks.length > 1
              if (cfgForm.storage_pool_type === 'dir') {
                return (
                  <div>
                    <label className="block text-sm font-medium text-black">目录路径</label>
                    <input
                      className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm"
                      value={cfgForm.storage_pool_source}
                      onChange={(e) => setCfgForm({ ...cfgForm, storage_pool_source: e.target.value })}
                    />
                  </div>
                )
              }
              if (!hasExtraDisk) {
                return (
                  <div className="text-xs text-yellow-700 bg-yellow-50 border border-yellow-200 rounded p-2">
                    系统只有一块磁盘，无法使用 {cfgForm.storage_pool_type} 类型，已自动切换为 dir
                  </div>
                )
              }
              return (
                <div>
                  <label className="block text-sm font-medium text-black">选择磁盘</label>
                  <select
                    className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm"
                    value={cfgForm.storage_pool_source}
                    onChange={(e) => setCfgForm({ ...cfgForm, storage_pool_source: e.target.value })}
                  >
                    <option value="">请选择磁盘</option>
                    {disks.map((d) => (
                      <option key={d.device} value={d.device}>
                        {d.device} {d.size} {d.model}
                      </option>
                    ))}
                  </select>
                </div>
              )
            })()}
          </div>
          <div>
            <label className="block text-sm font-medium text-black">扫描间隔（秒）</label>
            <input
              type="number"
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm"
              value={cfgForm.scan_interval}
              onChange={(e) => setCfgForm({ ...cfgForm, scan_interval: Number(e.target.value) })}
            />
          </div>
        </div>
        <div slot="footer" className="flex justify-end gap-2 pt-4 border-t border-gray-200">
          <Button variant="ghost" onClick={() => setConfigOpen(false)}>{t('common.cancel')}</Button>
          <Button onClick={handleSaveConfig}>{t('common.confirm')}</Button>
        </div>
      </SlidePanel>
    </div>
  )
}
