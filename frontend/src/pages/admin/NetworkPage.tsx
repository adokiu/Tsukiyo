import { useEffect, useState } from 'react'
import { Network, Plus, Trash2, Pencil } from 'lucide-react'
import apiClient from '@/api/client'
import { DataTable, type Column } from '@/components/DataTable/DataTable'
import { Button } from '@/components/Button/Button'
import { Modal } from '@/components/Modal/Modal'
import { Select } from '@/components/Select/Select'
import { SlidePanel } from '@/components/SlidePanel/SlidePanel'
import { useToastStore } from '@/stores/toast'

// ============== 类型定义 ==============
interface Node { id: string; name: string; status: string }

interface NodeNetwork {
  name: string
  status: string
  ipv4: string[]
  ipv6: string[]
  mac: string
}

interface VPC {
  id: string
  node_id: string
  name: string
  ipv4_cidr: string
  ipv6_ula_cidr?: string
  ipv6_gua_cidr?: string
  default_gateway_v4: string
  default_gateway_v6?: string
  egress_v4_primary?: string
  egress_v4_extra?: string[]
  port_range_start: number
  port_range_end: number
  parent_iface?: string
  status: string
  bridge_name: string
  snat_enabled: boolean
  ipv4_filter: boolean
  mac_filter: boolean
  instance_count?: number
  created_at: string
}

interface Pool {
  id: string
  node_id: string
  address: string
  gateway: string
  prefix_len: number
  status: string
}

// ============== VPC 弹窗组件 ==============
function CreateVPCModal({ open, onClose, onSuccess }: { open: boolean; onClose: () => void; onSuccess: () => void }) {
  const toast = useToastStore()
  const [nodes, setNodes] = useState<Node[]>([])
  const [loading, setLoading] = useState(false)

  const [nodeId, setNodeId] = useState<string | number>('')
  const [name, setName] = useState('')
  const [ipv4Cidr, setIpv4Cidr] = useState('')
  const [gatewayV4, setGatewayV4] = useState('')
  const [ipv6Ula, setIpv6Ula] = useState('')
  const [ipv6Gua, setIpv6Gua] = useState('')
  const [egressV4, setEgressV4] = useState('')
  const [egressV4Extra, setEgressV4Extra] = useState('')
  const [parentIface, setParentIface] = useState('')
  const [portRangeStart, setPortRangeStart] = useState(10000)
  const [portRangeEnd, setPortRangeEnd] = useState(65535)
  const [nodeNetworks, setNodeNetworks] = useState<NodeNetwork[]>([])

  useEffect(() => {
    if (!open) return
    apiClient.get('/nodes').then((r) => setNodes(r.data.data || []))
    setNodeId('')
    setName('')
    setIpv4Cidr('')
    setGatewayV4('')
    setIpv6Ula('')
    setIpv6Gua('')
    setEgressV4('')
    setEgressV4Extra('')
    setParentIface('')
    setPortRangeStart(10000)
    setPortRangeEnd(65535)
    setNodeNetworks([])
  }, [open])

  // 选择节点后拉取网卡列表
  useEffect(() => {
    if (!nodeId) {
      setNodeNetworks([])
      return
    }
    apiClient.get(`/nodes/${nodeId}/networks`).then((r) => {
      setNodeNetworks(r.data.networks || [])
    }).catch(() => setNodeNetworks([]))
  }, [nodeId])

  // 选择父网卡后自动推断出口 IP（取该网卡第一个 IPv4）
  useEffect(() => {
    if (!parentIface) return
    const net = nodeNetworks.find((n) => n.name === parentIface)
    if (net && net.ipv4.length > 0) {
      setEgressV4(net.ipv4[0])
    }
  }, [parentIface, nodeNetworks])

  // 自动推断网关
  useEffect(() => {
    if (!ipv4Cidr) return
    // 简单推断：取 CIDR 的 .1
    const parts = ipv4Cidr.split('/')
    if (parts.length === 2) {
      const ipParts = parts[0].split('.')
      if (ipParts.length === 4) {
        ipParts[3] = '1'
        setGatewayV4(ipParts.join('.'))
      }
    }
  }, [ipv4Cidr])

  const handleSubmit = async () => {
    if (!nodeId || !name || !ipv4Cidr) {
      toast.error('请填写节点、名称和 IPv4 CIDR')
      return
    }
    setLoading(true)
    try {
      const extra = egressV4Extra.split(/[,\s]+/).map((s) => s.trim()).filter(Boolean)
      await apiClient.post('/network/vpcs', {
        node_id: nodeId,
        name,
        ipv4_cidr: ipv4Cidr,
        default_gateway_v4: gatewayV4 || undefined,
        ipv6_ula_cidr: ipv6Ula || undefined,
        ipv6_gua_cidr: ipv6Gua || undefined,
        egress_v4_primary: egressV4 || undefined,
        egress_v4_extra: extra.length > 0 ? extra : undefined,
        parent_iface: parentIface || undefined,
        port_range_start: portRangeStart,
        port_range_end: portRangeEnd,
      })
      toast.success('VPC 创建任务已下发')
      onSuccess()
      onClose()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '创建失败')
    } finally {
      setLoading(false)
    }
  }

  const nodeOptions = nodes.map((n) => ({ label: `${n.name} (${n.id.slice(0, 8)})`, value: n.id }))

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="创建 VPC 网络"
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
          <label className="block text-sm font-medium text-gray-700 mb-1">所属节点 <span className="text-red-500">*</span></label>
          <Select value={nodeId} options={nodeOptions} placeholder="选择 Agent 节点" onChange={(v) => setNodeId(v)} />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">VPC 名称 <span className="text-red-500">*</span></label>
          <input value={name} onChange={(e) => setName(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" placeholder="如：生产网络-01" />
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">IPv4 CIDR <span className="text-red-500">*</span></label>
            <input value={ipv4Cidr} onChange={(e) => setIpv4Cidr(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" placeholder="10.10.1.0/24" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">IPv4 网关</label>
            <input value={gatewayV4} onChange={(e) => setGatewayV4(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" placeholder="自动推断" />
          </div>
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">IPv6 ULA CIDR</label>
            <input value={ipv6Ula} onChange={(e) => setIpv6Ula(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" placeholder="fd00:1::/64" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">IPv6 GUA CIDR</label>
            <input value={ipv6Gua} onChange={(e) => setIpv6Gua(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" placeholder="2001:db8:1::/64" />
          </div>
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">出口 IPv4 (SNAT)</label>
            <Select
              value={egressV4}
              options={nodeNetworks.flatMap((n) => (n.ipv4 || []).map((ip) => ({ label: `${n.name}: ${ip}`, value: ip })))}
              placeholder="选择出口 IP"
              disabled={!nodeId}
              onChange={(v) => setEgressV4(v as string)}
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">父网卡</label>
            <Select
              value={parentIface}
              options={nodeNetworks.map((n) => ({ label: `${n.name} (${(n.ipv4 || []).join(', ') || '无IP'})`, value: n.name }))}
              placeholder="选择父网卡"
              disabled={!nodeId}
              onChange={(v) => setParentIface(v as string)}
            />
          </div>
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">额外出口 IPv4 地址池（逗号分隔）</label>
          <input value={egressV4Extra} onChange={(e) => setEgressV4Extra(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" placeholder="203.0.113.2, 203.0.113.3" />
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">端口映射起始</label>
            <input type="number" value={portRangeStart} onChange={(e) => setPortRangeStart(Number(e.target.value))} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">端口映射结束</label>
            <input type="number" value={portRangeEnd} onChange={(e) => setPortRangeEnd(Number(e.target.value))} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" />
          </div>
        </div>
      </div>
    </Modal>
  )
}

// ============== 编辑 VPC 弹窗 ==============
function EditVPCModal({ vpc, open, onClose, onSuccess }: { vpc: VPC | null; open: boolean; onClose: () => void; onSuccess: () => void }) {
  const toast = useToastStore()
  const [loading, setLoading] = useState(false)
  const [nodeNetworks, setNodeNetworks] = useState<NodeNetwork[]>([])

  const [name, setName] = useState('')
  const [ipv4Cidr, setIpv4Cidr] = useState('')
  const [gatewayV4, setGatewayV4] = useState('')
  const [ipv6Ula, setIpv6Ula] = useState('')
  const [ipv6Gua, setIpv6Gua] = useState('')
  const [egressV4, setEgressV4] = useState('')
  const [egressV4Extra, setEgressV4Extra] = useState('')
  const [parentIface, setParentIface] = useState('')
  const [portRangeStart, setPortRangeStart] = useState(10000)
  const [portRangeEnd, setPortRangeEnd] = useState(65535)
  const [snatEnabled, setSnatEnabled] = useState(true)
  const [hasInstances, setHasInstances] = useState(false)

  useEffect(() => {
    if (!open || !vpc) return
    setName(vpc.name)
    setIpv4Cidr(vpc.ipv4_cidr)
    setGatewayV4(vpc.default_gateway_v4 || '')
    setIpv6Ula(vpc.ipv6_ula_cidr || '')
    setIpv6Gua(vpc.ipv6_gua_cidr || '')
    setEgressV4(vpc.egress_v4_primary || '')
    setEgressV4Extra((vpc.egress_v4_extra || []).join(', '))
    setParentIface(vpc.parent_iface || '')
    setPortRangeStart(vpc.port_range_start)
    setPortRangeEnd(vpc.port_range_end)
    setSnatEnabled(vpc.snat_enabled)
    setHasInstances((vpc.instance_count || 0) > 0)

    // 拉取节点网卡列表
    apiClient.get(`/nodes/${vpc.node_id}/networks`).then((r) => {
      setNodeNetworks(r.data.networks || [])
    }).catch(() => setNodeNetworks([]))
  }, [open, vpc])

  const handleSubmit = async () => {
    if (!vpc) return
    setLoading(true)
    try {
      const extra = egressV4Extra.split(/[,\s]+/).map((s) => s.trim()).filter(Boolean)
      await apiClient.put(`/network/vpcs/${vpc.id}`, {
        name: name || undefined,
        ipv4_cidr: ipv4Cidr || undefined,
        default_gateway_v4: gatewayV4 || undefined,
        ipv6_ula_cidr: ipv6Ula || undefined,
        ipv6_gua_cidr: ipv6Gua || undefined,
        egress_v4_primary: egressV4 || undefined,
        egress_v4_extra: extra.length > 0 ? extra : undefined,
        parent_iface: parentIface || undefined,
        port_range_start: portRangeStart,
        port_range_end: portRangeEnd,
        snat_enabled: snatEnabled,
      })
      toast.success('VPC 更新任务已下发')
      onSuccess()
      onClose()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '更新失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="编辑 VPC 网络"
      width={560}
      footer={
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>取消</Button>
          <Button loading={loading} onClick={handleSubmit}>保存</Button>
        </div>
      }
    >
      <div className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">VPC 名称</label>
          <input value={name} onChange={(e) => setName(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" />
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">IPv4 CIDR {hasInstances && <span className="text-amber-600 text-xs">(已锁定)</span>}</label>
            <input value={ipv4Cidr} disabled={hasInstances} onChange={(e) => setIpv4Cidr(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm disabled:bg-gray-100" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">IPv4 网关 {hasInstances && <span className="text-amber-600 text-xs">(已锁定)</span>}</label>
            <input value={gatewayV4} disabled={hasInstances} onChange={(e) => setGatewayV4(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm disabled:bg-gray-100" />
          </div>
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">IPv6 ULA CIDR {hasInstances && <span className="text-amber-600 text-xs">(已锁定)</span>}</label>
            <input value={ipv6Ula} disabled={hasInstances} onChange={(e) => setIpv6Ula(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm disabled:bg-gray-100" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">IPv6 GUA CIDR {hasInstances && <span className="text-amber-600 text-xs">(已锁定)</span>}</label>
            <input value={ipv6Gua} disabled={hasInstances} onChange={(e) => setIpv6Gua(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm disabled:bg-gray-100" />
          </div>
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">出口 IPv4 (SNAT)</label>
            <Select
              value={egressV4}
              options={((): { label: string; value: string }[] => {
                const list = nodeNetworks.flatMap((n) => (n.ipv4 || []).map((ip) => ({ label: `${n.name}: ${ip}`, value: ip })))
                if (egressV4 && !list.find((o) => o.value === egressV4)) {
                  list.unshift({ label: egressV4, value: egressV4 })
                }
                return list
              })()}
              placeholder="选择出口 IP"
              onChange={(v) => setEgressV4(v as string)}
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">父网卡</label>
            <Select
              value={parentIface}
              options={((): { label: string; value: string }[] => {
                const list = nodeNetworks.map((n) => ({ label: `${n.name} (${(n.ipv4 || []).join(', ') || '无IP'})`, value: n.name }))
                if (parentIface && !list.find((o) => o.value === parentIface)) {
                  list.unshift({ label: parentIface, value: parentIface })
                }
                return list
              })()}
              placeholder="选择父网卡"
              onChange={(v) => setParentIface(v as string)}
            />
          </div>
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">额外出口 IPv4 地址池（逗号分隔）</label>
          <input value={egressV4Extra} onChange={(e) => setEgressV4Extra(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" placeholder="203.0.113.2, 203.0.113.3" />
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">端口映射起始</label>
            <input type="number" value={portRangeStart} onChange={(e) => setPortRangeStart(Number(e.target.value))} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">端口映射结束</label>
            <input type="number" value={portRangeEnd} onChange={(e) => setPortRangeEnd(Number(e.target.value))} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" />
          </div>
        </div>
        <div className="flex items-center gap-2">
          <input type="checkbox" checked={snatEnabled} onChange={(e) => setSnatEnabled(e.target.checked)} className="rounded" />
          <label className="text-sm text-gray-700">启用 SNAT</label>
        </div>
      </div>
    </Modal>
  )
}

// ============== 主页面 ==============
export default function NetworkPage() {
  const toast = useToastStore()
  const [tab, setTab] = useState<'vpc' | 'pool'>('vpc')

  // VPC 状态
  const [vpcs, setVpcs] = useState<VPC[]>([])
  const [vpcsLoading, setVpcsLoading] = useState(true)
  const [vpcModalOpen, setVpcModalOpen] = useState(false)
  const [editVpc, setEditVpc] = useState<VPC | null>(null)
  const [editModalOpen, setEditModalOpen] = useState(false)

  // IP 池状态
  const [pools, setPools] = useState<Pool[]>([])
  const [poolsLoading, setPoolsLoading] = useState(true)

  const fetchVPCs = () => {
    setVpcsLoading(true)
    apiClient.get('/network/vpcs').then((res) => setVpcs(res.data.data || [])).finally(() => setVpcsLoading(false))
  }

  const fetchPools = () => {
    setPoolsLoading(true)
    apiClient.get('/network/pools').then((res) => setPools(res.data.data || [])).finally(() => setPoolsLoading(false))
  }

  useEffect(() => {
    fetchVPCs()
    fetchPools()
  }, [])

  const handleDeleteVPC = async (id: string) => {
    if (!confirm('确认删除该 VPC？VPC 下的实例必须先行迁移或删除。')) return
    try {
      await apiClient.delete(`/network/vpcs/${id}`)
      toast.success('删除任务已下发')
      fetchVPCs()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '删除失败')
    }
  }

  const handleDeletePool = async (id: string) => {
    if (!confirm('确认删除该 IP 池？')) return
    await apiClient.delete(`/network/pools/${id}`)
    toast.success('删除成功')
    fetchPools()
  }

  const vpcColumns: Column<VPC>[] = [
    { key: 'name', title: 'VPC 名称' },
    { key: 'node_id', title: '节点', render: (row) => <span className="text-xs font-mono text-gray-500">{row.node_id.slice(0, 8)}</span> },
    { key: 'bridge_name', title: 'Bridge', render: (row) => <span className="text-xs font-mono">{row.bridge_name}</span> },
    { key: 'ipv4_cidr', title: 'IPv4 CIDR' },
    { key: 'default_gateway_v4', title: '网关' },
    { key: 'egress_v4_primary', title: '出口 IP', render: (row) => <span className="text-xs text-gray-600">{row.egress_v4_primary || '-'}</span> },
    {
      key: 'status',
      title: '状态',
      render: (row) => (
        <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${row.status === 'active' ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-600'}`}>
          {row.status}
        </span>
      ),
    },
    {
      key: 'action',
      title: '操作',
      render: (row: VPC) => (
        <div className="flex items-center gap-2">
          <button className="text-blue-500 hover:text-blue-700 text-sm" onClick={() => { setEditVpc(row); setEditModalOpen(true) }} title="编辑">
            <Pencil size={14} />
          </button>
          <button className="text-red-500 hover:text-red-700 text-sm" onClick={() => handleDeleteVPC(row.id)} title="删除">
            <Trash2 size={14} />
          </button>
        </div>
      ),
    },
  ]

  const poolColumns: Column<Pool>[] = [
    { key: 'node_id', title: '节点', render: (row) => <span className="text-xs font-mono text-gray-500">{row.node_id.slice(0, 8)}</span> },
    { key: 'address', title: '地址' },
    { key: 'gateway', title: '网关' },
    { key: 'prefix_len', title: '前缀长度' },
    {
      key: 'status',
      title: '状态',
      render: (row) => (
        <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${row.status === 'free' ? 'bg-green-100 text-green-700' : 'bg-blue-100 text-blue-700'}`}>
          {row.status}
        </span>
      ),
    },
    {
      key: 'action',
      title: '操作',
      render: (row: Pool) => (
        <button className="text-red-500 hover:text-red-700 text-sm" onClick={() => handleDeletePool(row.id)}>删除</button>
      ),
    },
  ]

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Network size={22} className="text-black" />
          <h1 className="text-xl font-semibold text-black">网络管理</h1>
        </div>
        {tab === 'vpc' && (
          <Button icon={<Plus size={16} />} onClick={() => setVpcModalOpen(true)}>创建 VPC</Button>
        )}
      </div>

      {/* 标签页 */}
      <div className="flex border-b border-gray-200">
        <button
          className={`px-4 py-2 text-sm font-medium ${tab === 'vpc' ? 'text-black border-b-2 border-black' : 'text-gray-500 hover:text-gray-700'}`}
          onClick={() => setTab('vpc')}
        >
          VPC 网络
        </button>
        <button
          className={`px-4 py-2 text-sm font-medium ${tab === 'pool' ? 'text-black border-b-2 border-black' : 'text-gray-500 hover:text-gray-700'}`}
          onClick={() => setTab('pool')}
        >
          IP 地址池
        </button>
      </div>

      {tab === 'vpc' && (
        <DataTable columns={vpcColumns} data={vpcs} rowKey={(r) => r.id} loading={vpcsLoading} emptyText="暂无 VPC 网络" />
      )}

      {tab === 'pool' && (
        <DataTable columns={poolColumns} data={pools} rowKey={(r) => r.id} loading={poolsLoading} emptyText="暂无 IP 地址池" />
      )}

      <CreateVPCModal open={vpcModalOpen} onClose={() => setVpcModalOpen(false)} onSuccess={fetchVPCs} />
      <EditVPCModal vpc={editVpc} open={editModalOpen} onClose={() => setEditModalOpen(false)} onSuccess={fetchVPCs} />
    </div>
  )
}
