import { useEffect, useState } from 'react'
import { Network, Plus } from 'lucide-react'
import apiClient from '@/api/client'
import { DataTable, type Column } from '@/components/DataTable/DataTable'
import { Button } from '@/components/Button/Button'
import { Select } from '@/components/Select/Select'
import { SlidePanel } from '@/components/SlidePanel/SlidePanel'
import { useToastStore } from '@/stores/toast'

// ============== 类型定义 ==============
interface Node { id: string; name: string; status: string }

interface IPProbe {
  interface: string
  address: string
  prefix_len: number
  scope: string
  gateway?: string
}

interface NodeNetwork {
  name: string
  state: string
  mac: string
  ipv4: IPProbe[]
  ipv6: IPProbe[]
}

interface Bridge {
  id: string
  node_id: string
  name: string
  bridge_name: string
  ipv4_enabled: boolean
  ipv4_cidr: string
  ipv4_gateway: string
  ipv6_enabled: boolean
  ipv6_cidr: string
  ipv6_gateway: string
  dns_servers: string[]
  nat_egress_ipv4_id?: string
  nat_egress_ipv6_id?: string
  port_range_start: number
  port_range_end: number
  status: string
  instance_count?: number
  created_at: string
}

interface EIPPool {
  id: string
  node_id: string
  ip_version: string
  cidr: string
  interface: string
  gateway: string
  prefix_len: number
  alias: string
  pool_type: string
  status: string
  created_at: string
}

interface EIPAllocation {
  id: string
  pool_id: string
  node_id: string
  cidr: string
  prefix_len: number
  ip_version: string
  usage: string
  bridge_id?: string
  instance_id?: string
  status: string
  allocated_at: string
}

// ============== Bridge 表单侧边栏（创建/编辑共用） ==============
function BridgeFormPanel({ open, mode, bridge, nodeId, existingBridges, onClose, onSuccess }: {
  open: boolean
  mode: 'create' | 'edit'
  bridge: Bridge | null
  nodeId: string
  existingBridges: Bridge[]
  onClose: () => void
  onSuccess: () => void
}) {
  const toast = useToastStore()
  const [loading, setLoading] = useState(false)
  const [eipPools, setEipPools] = useState<EIPPool[]>([])

  const [name, setName] = useState('')
  const [ipv4Enabled, setIpv4Enabled] = useState(true)
  const [ipv4Cidr, setIpv4Cidr] = useState('')
  const [gatewayV4, setGatewayV4] = useState('')
  const [ipv6Enabled, setIpv6Enabled] = useState(false)
  const [ipv6Cidr, setIpv6Cidr] = useState('')
  const [gatewayV6, setGatewayV6] = useState('')
  const [dnsServers, setDnsServers] = useState('')
  const [portRangeStart, setPortRangeStart] = useState(20000)
  const [portRangeEnd, setPortRangeEnd] = useState(65535)
  const [hasInstances, setHasInstances] = useState(false)

  // NAT 出口 EIP 选择
  const [natEgressV4PoolId, setNatEgressV4PoolId] = useState('')
  const [natEgressV6PoolId, setNatEgressV6PoolId] = useState('')
  const [natEgressV4Info, setNatEgressV4Info] = useState('')
  const [natEgressV6Info, setNatEgressV6Info] = useState('')

  useEffect(() => {
    if (!open) return
    if (mode === 'edit' && bridge) {
      setName(bridge.name)
      setIpv4Enabled(bridge.ipv4_enabled)
      setIpv4Cidr(bridge.ipv4_cidr)
      setGatewayV4(bridge.ipv4_gateway || '')
      setIpv6Enabled(bridge.ipv6_enabled)
      setIpv6Cidr(bridge.ipv6_cidr || '')
      setGatewayV6(bridge.ipv6_gateway || '')
      setDnsServers((bridge.dns_servers || []).join(', '))
      setPortRangeStart(bridge.port_range_start)
      setPortRangeEnd(bridge.port_range_end)
      setHasInstances((bridge.instance_count || 0) > 0)
      // 编辑模式下显示已绑定的 NAT 出口 EIP 信息
      setNatEgressV4PoolId('')
      setNatEgressV6PoolId('')
      setNatEgressV4Info(bridge.nat_egress_ipv4_id ? `已绑定: ${bridge.nat_egress_ipv4_id.slice(0, 8)}` : '未绑定')
      setNatEgressV6Info(bridge.nat_egress_ipv6_id ? `已绑定: ${bridge.nat_egress_ipv6_id.slice(0, 8)}` : '未绑定')
    } else {
      setName('')
      setIpv4Enabled(true)
      setIpv4Cidr('')
      setGatewayV4('')
      setIpv6Enabled(false)
      setIpv6Cidr('')
      setGatewayV6('')
      setDnsServers('')
      setPortRangeStart(20000)
      setPortRangeEnd(65535)
      setHasInstances(false)
      setNatEgressV4PoolId('')
      setNatEgressV6PoolId('')
      setNatEgressV4Info('')
      setNatEgressV6Info('')
    }
  }, [open, mode, bridge])

  // 拉取 EIP 池列表
  useEffect(() => {
    if (!open || !nodeId) {
      setEipPools([])
      return
    }
    apiClient.get(`/network/eip-pools?node_id=${nodeId}`).then((r) => {
      setEipPools(r.data.data || [])
    }).catch(() => setEipPools([]))
  }, [open, nodeId])

  // 自动生成不冲突的 IPv4 CIDR
  const autoGenerateV4Cidr = () => {
    const usedSegments = new Set<number>()
    for (const b of existingBridges) {
      if (b.ipv4_enabled && b.ipv4_cidr) {
        const parts = b.ipv4_cidr.split('/')[0].split('.')
        if (parts.length === 4 && parts[0] === '10' && parts[1] === '10') {
          usedSegments.add(parseInt(parts[2]))
        }
      }
    }
    for (let i = 1; i <= 254; i++) {
      if (!usedSegments.has(i)) {
        const cidr = `10.10.${i}.0/24`
        setIpv4Cidr(cidr)
        setGatewayV4(`10.10.${i}.1`)
        return
      }
    }
    toast.error('已无可用的 10.10.x.0/24 网段')
  }

  // 自动生成不冲突的 IPv6 CIDR
  const autoGenerateV6Cidr = () => {
    const usedSegments = new Set<number>()
    for (const b of existingBridges) {
      if (b.ipv6_enabled && b.ipv6_cidr) {
        const match = b.ipv6_cidr.match(/fd00:(\d+)::/)
        if (match) {
          usedSegments.add(parseInt(match[1]))
        }
      }
    }
    for (let i = 1; i <= 65535; i++) {
      if (!usedSegments.has(i)) {
        const cidr = `fd00:${i}::/64`
        setIpv6Cidr(cidr)
        setGatewayV6(`fd00:${i}::1`)
        return
      }
    }
    toast.error('已无可用的 fd00:x::/64 网段')
  }

  // 自动推断网关
  useEffect(() => {
    if (!ipv4Cidr) return
    const parts = ipv4Cidr.split('/')
    if (parts.length === 2) {
      const ipParts = parts[0].split('.')
      if (ipParts.length === 4) {
        ipParts[3] = '1'
        setGatewayV4(ipParts.join('.'))
      }
    }
  }, [ipv4Cidr])

  // IPv4 EIP 池选项（active 状态）
  const v4PoolOptions = eipPools
    .filter(p => p.ip_version === 'ipv4' && p.status === 'active')
    .map(p => ({ label: `${p.cidr} (${p.interface || '无网卡'}) [${p.pool_type === 'host' ? '宿主机' : 'EIP'}]`, value: p.id }))

  // IPv6 EIP 池选项
  const v6PoolOptions = eipPools
    .filter(p => p.ip_version === 'ipv6' && p.status === 'active')
    .map(p => ({ label: `${p.cidr} (${p.interface || '无网卡'}) [${p.pool_type === 'host' ? '宿主机' : 'EIP'}]`, value: p.id }))

  const handleSubmit = async () => {
    if (!nodeId || !name || !ipv4Cidr) {
      toast.error('请填写名称和 IPv4 CIDR')
      return
    }
    setLoading(true)
    try {
      const payload: Record<string, any> = {
        node_id: nodeId,
        name,
        ipv4_enabled: ipv4Enabled,
        ipv4_cidr: ipv4Cidr,
        ipv4_gateway: gatewayV4 || undefined,
        ipv6_enabled: ipv6Enabled,
        ipv6_cidr: ipv6Cidr || undefined,
        ipv6_gateway: gatewayV6 || undefined,
        dns_servers: dnsServers ? dnsServers.split(',').map(s => s.trim()).filter(Boolean) : [],
        port_range_start: portRangeStart,
        port_range_end: portRangeEnd,
      }
      // 创建模式时传递 NAT 出口 EIP 池 ID
      if (mode === 'create') {
        if (natEgressV4PoolId) payload.nat_egress_v4_pool_id = natEgressV4PoolId
        if (natEgressV6PoolId) payload.nat_egress_v6_pool_id = natEgressV6PoolId
      }
      if (mode === 'create') {
        await apiClient.post('/network/bridges', payload)
        toast.success('Bridge 创建成功')
      } else if (bridge) {
        await apiClient.put(`/network/bridges/${bridge.id}`, payload)
        toast.success('Bridge 更新成功')
      }
      onSuccess()
      onClose()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '操作失败')
    } finally {
      setLoading(false)
    }
  }

  // 解绑 NAT 出口 EIP
  const handleUnbindEgress = async (ipVersion: 'ipv4' | 'ipv6') => {
    if (!bridge) return
    try {
      await apiClient.post(`/network/bridges/${bridge.id}/unbind-egress`, { ip_version: ipVersion })
      toast.success(`${ipVersion === 'ipv4' ? 'IPv4' : 'IPv6'} NAT 出口 EIP 已解绑`)
      if (ipVersion === 'ipv4') setNatEgressV4Info('未绑定')
      else setNatEgressV6Info('未绑定')
      onSuccess()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '解绑失败')
    }
  }

  // 绑定 NAT 出口 EIP（编辑模式，从选中的 EIP 池分配）
  const handleBindEgress = async (ipVersion: 'ipv4' | 'ipv6') => {
    if (!bridge) return
    const poolId = ipVersion === 'ipv4' ? natEgressV4PoolId : natEgressV6PoolId
    if (!poolId) {
      toast.error('请先选择 EIP 资源池')
      return
    }
    try {
      await apiClient.post(`/network/bridges/${bridge.id}/bind-egress`, {
        pool_id: poolId,
        ip_version: ipVersion,
      })
      toast.success(`${ipVersion === 'ipv4' ? 'IPv4' : 'IPv6'} NAT 出口 EIP 已绑定`)
      if (ipVersion === 'ipv4') setNatEgressV4Info('已绑定')
      else setNatEgressV6Info('已绑定')
      onSuccess()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '绑定失败')
    }
  }

  return (
    <SlidePanel
      open={open}
      onClose={onClose}
      title={mode === 'create' ? '创建 Bridge 网络' : '编辑 Bridge 网络'}
      width={700}
      footer={
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>取消</Button>
          <Button loading={loading} onClick={handleSubmit}>{mode === 'create' ? '创建' : '保存'}</Button>
        </div>
      }
    >
      <div className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Bridge 名称 <span className="text-red-500">*</span></label>
          <input value={name} onChange={(e) => setName(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" placeholder="如：生产网络-01" />
        </div>

        {/* IPv4 配置 */}
        <div className="border-t border-gray-100 pt-3">
          <div className="flex items-center gap-2 mb-3">
            <input type="checkbox" id="ipv4_enabled" checked={ipv4Enabled} onChange={(e) => setIpv4Enabled(e.target.checked)} className="w-4 h-4" />
            <label htmlFor="ipv4_enabled" className="text-sm font-medium text-gray-700">启用 IPv4</label>
          </div>
          {ipv4Enabled && (
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">IPv4 CIDR <span className="text-red-500">*</span> {hasInstances && <span className="text-amber-600 text-xs">(已锁定)</span>}</label>
                <div className="flex gap-2">
                  <input value={ipv4Cidr} disabled={hasInstances} onChange={(e) => setIpv4Cidr(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm disabled:bg-gray-100" placeholder="10.10.1.0/24" />
                  {!hasInstances && (
                    <button type="button" onClick={autoGenerateV4Cidr} className="shrink-0 px-3 py-2 border border-gray-300 rounded-lg text-xs text-gray-600 hover:bg-gray-50 whitespace-nowrap">自动生成</button>
                  )}
                </div>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">IPv4 网关 {hasInstances && <span className="text-amber-600 text-xs">(已锁定)</span>}</label>
                <input value={gatewayV4} disabled={hasInstances} onChange={(e) => setGatewayV4(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm disabled:bg-gray-100" placeholder="自动推断" />
              </div>
            </div>
          )}
        </div>

        {/* IPv6 配置 */}
        <div className="border-t border-gray-100 pt-3">
          <div className="flex items-center gap-2 mb-3">
            <input type="checkbox" id="ipv6_enabled" checked={ipv6Enabled} onChange={(e) => setIpv6Enabled(e.target.checked)} className="w-4 h-4" />
            <label htmlFor="ipv6_enabled" className="text-sm font-medium text-gray-700">启用 IPv6</label>
          </div>
          {ipv6Enabled && (
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">IPv6 CIDR {hasInstances && <span className="text-amber-600 text-xs">(已锁定)</span>}</label>
                <div className="flex gap-2">
                  <input value={ipv6Cidr} disabled={hasInstances} onChange={(e) => setIpv6Cidr(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm disabled:bg-gray-100" placeholder="fd00:1::/64" />
                  {!hasInstances && (
                    <button type="button" onClick={autoGenerateV6Cidr} className="shrink-0 px-3 py-2 border border-gray-300 rounded-lg text-xs text-gray-600 hover:bg-gray-50 whitespace-nowrap">自动生成</button>
                  )}
                </div>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">IPv6 网关 {hasInstances && <span className="text-amber-600 text-xs">(已锁定)</span>}</label>
                <input value={gatewayV6} disabled={hasInstances} onChange={(e) => setGatewayV6(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm disabled:bg-gray-100" placeholder="fd00:1::1" />
              </div>
            </div>
          )}
        </div>

        {/* NAT 出口 EIP 配置 */}
        <div className="border-t border-gray-100 pt-3">
          <label className="block text-sm font-medium text-gray-700 mb-2">NAT 出口 EIP</label>
          {ipv4Enabled && (
            <div className="mb-3">
              <label className="block text-xs text-gray-500 mb-1">IPv4 NAT 出口</label>
              {mode === 'create' ? (
                <Select
                  value={natEgressV4PoolId}
                  options={v4PoolOptions}
                  placeholder="选择 IPv4 EIP 资源池（可选）"
                  onChange={(v) => setNatEgressV4PoolId(v as string)}
                />
              ) : (
                <div className="flex items-center gap-2">
                  <span className="text-sm text-gray-600 flex-1">{natEgressV4Info}</span>
                  {natEgressV4Info !== '未绑定' && (
                    <button type="button" onClick={() => handleUnbindEgress('ipv4')} className="text-xs text-red-500 hover:text-red-700">解绑</button>
                  )}
                  {natEgressV4Info === '未绑定' && v4PoolOptions.length > 0 && (
                    <>
                      <Select
                        value={natEgressV4PoolId}
                        options={v4PoolOptions}
                        placeholder="选择 EIP 池"
                        onChange={(v) => setNatEgressV4PoolId(v as string)}
                      />
                      <button type="button" onClick={() => handleBindEgress('ipv4')} className="text-xs text-blue-500 hover:text-blue-700">绑定</button>
                    </>
                  )}
                </div>
              )}
            </div>
          )}
          {ipv6Enabled && (
            <div>
              <label className="block text-xs text-gray-500 mb-1">IPv6 NAT 出口</label>
              {mode === 'create' ? (
                <Select
                  value={natEgressV6PoolId}
                  options={v6PoolOptions}
                  placeholder="选择 IPv6 EIP 资源池（可选）"
                  onChange={(v) => setNatEgressV6PoolId(v as string)}
                />
              ) : (
                <div className="flex items-center gap-2">
                  <span className="text-sm text-gray-600 flex-1">{natEgressV6Info}</span>
                  {natEgressV6Info !== '未绑定' && (
                    <button type="button" onClick={() => handleUnbindEgress('ipv6')} className="text-xs text-red-500 hover:text-red-700">解绑</button>
                  )}
                  {natEgressV6Info === '未绑定' && v6PoolOptions.length > 0 && (
                    <>
                      <Select
                        value={natEgressV6PoolId}
                        options={v6PoolOptions}
                        placeholder="选择 EIP 池"
                        onChange={(v) => setNatEgressV6PoolId(v as string)}
                      />
                      <button type="button" onClick={() => handleBindEgress('ipv6')} className="text-xs text-blue-500 hover:text-blue-700">绑定</button>
                    </>
                  )}
                </div>
              )}
            </div>
          )}
          {v4PoolOptions.length === 0 && v6PoolOptions.length === 0 && (
            <p className="text-xs text-gray-400">暂无可用 EIP 资源池，请先在 EIP 资源池标签页中创建</p>
          )}
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">DNS 服务器（逗号分隔）</label>
          <input value={dnsServers} onChange={(e) => setDnsServers(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" placeholder="8.8.8.8, 8.8.4.4" />
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
    </SlidePanel>
  )
}

// ============== EIP 池创建表单 ==============

interface EIPPoolDraftItem {
  id: string
  cidr: string
  cidrManual: boolean
  gateway: string
  gatewayManual: boolean
  interface: string
  prefix: string
  alias: string
  poolType: 'host' | 'eip'
  detecting: boolean
}

function makeDraftItem(): EIPPoolDraftItem {
  return { id: Math.random().toString(36).slice(2), cidr: '', cidrManual: false, gateway: '', gatewayManual: false, interface: '', prefix: '', alias: '', poolType: 'host', detecting: false }
}

function EIPPoolFormPanel({ open, nodeId, existingPools, onClose, onSuccess }: {
  open: boolean
  nodeId: string
  existingPools: EIPPool[]
  onClose: () => void
  onSuccess: () => void
}) {
  const toast = useToastStore()
  const [loading, setLoading] = useState(false)
  const [nodeNetworks, setNodeNetworks] = useState<NodeNetwork[]>([])
  const [ipVersion, setIpVersion] = useState<'ipv4' | 'ipv6'>('ipv4')
  const [draftItems, setDraftItems] = useState<EIPPoolDraftItem[]>([makeDraftItem()])

  useEffect(() => {
    if (!open || !nodeId) {
      setNodeNetworks([])
      return
    }
    apiClient.get(`/nodes/${nodeId}/networks`).then((r) => {
      setNodeNetworks(r.data.networks || [])
    }).catch(() => setNodeNetworks([]))
  }, [open, nodeId])

  useEffect(() => {
    if (!open) return
    setDraftItems([makeDraftItem()])
  }, [open, ipVersion])

  const updateDraftItem = (id: string, patch: Partial<EIPPoolDraftItem>) => {
    setDraftItems(items => items.map(it => it.id === id ? { ...it, ...patch } : it))
  }

  // CIDR 变化时自动检测别名
  const handleCidrChange = (item: EIPPoolDraftItem, value: string) => {
    const patch: Partial<EIPPoolDraftItem> = { cidr: value, cidrManual: true }
    // v6: 从 CIDR 中提取前缀填入前缀字段
    if (ipVersion === 'ipv6') {
      const slashIdx = value.indexOf('/')
      if (slashIdx > 0) {
        patch.prefix = value.substring(slashIdx + 1)
      }
    }
    updateDraftItem(item.id, patch)
  }

  // 网卡选择时自动填充网关和 CIDR（仅在用户未手动修改过时填充）
  const handleIfaceChange = (item: EIPPoolDraftItem, iface: string) => {
    const net = nodeNetworks.find(n => n.name === iface)
    const patch: Partial<EIPPoolDraftItem> = { interface: iface }
    if (net) {
      const ips = ipVersion === 'ipv4' ? (net.ipv4 || []) : (net.ipv6 || [])
      if (ips.length > 0) {
        const ip = ips[0]
        if (!item.cidrManual) {
          patch.cidr = ip.address + '/' + ip.prefix_len
          if (ipVersion === 'ipv6') {
            patch.prefix = String(ip.prefix_len)
          }
        }
        if (!item.gatewayManual && ip.gateway) {
          patch.gateway = ip.gateway
        }
      }
    }
    updateDraftItem(item.id, patch)
  }

  const addRow = () => {
    setDraftItems(items => [...items, makeDraftItem()])
  }

  const removeRow = (id: string) => {
    setDraftItems(items => items.filter(it => it.id !== id))
  }

  // 检查同批次内是否有重复
  const isDuplicateInBatch = (cidr: string, currentId: string): boolean => {
    return draftItems.some(it => it.id !== currentId && it.cidr === cidr)
  }

  const handleSubmit = async () => {
    const validItems = draftItems.filter(it => it.cidr.trim())
    if (validItems.length === 0) {
      toast.error('请至少添加一条 IP 记录')
      return
    }
    // 检查同批次重复
    for (const item of validItems) {
      if (isDuplicateInBatch(item.cidr, item.id)) {
        toast.error(`CIDR ${item.cidr} 在批次内重复`)
        return
      }
    }
    setLoading(true)
    try {
      for (const item of validItems) {
        const payload: any = {
          node_id: nodeId,
          ip_version: ipVersion,
          cidr: item.cidr,
          interface: item.interface || undefined,
          gateway: item.gateway || undefined,
          alias: item.alias || undefined,
          pool_type: item.poolType,
        }
        await apiClient.post('/network/eip-pools', payload)
      }
      toast.success(`成功创建 ${validItems.length} 条 EIP 记录`)
      onSuccess()
      onClose()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '操作失败')
    } finally {
      setLoading(false)
    }
  }

  // 网卡选项
  const ifaceOptions = nodeNetworks.map(n => {
    const ipList = ipVersion === 'ipv4' ? (n.ipv4 || []) : (n.ipv6 || [])
    const ipStr = ipList.map(ip => ip.address).join(', ') || '无IP'
    return { label: `${n.name} (${ipStr})`, value: n.name }
  })

  // 每行的 CIDR 选项（根据选中的网卡过滤）
  const getCidrOptions = (iface: string) => {
    const opts: { label: string; value: string }[] = []
    for (const n of nodeNetworks) {
      if (iface && n.name !== iface) continue
      const ipList = ipVersion === 'ipv4' ? (n.ipv4 || []) : (n.ipv6 || [])
      for (const ip of ipList) {
        const cidr = `${ip.address}/${ip.prefix_len}`
        opts.push({ label: cidr, value: cidr })
      }
    }
    return opts
  }

  // 每行的网关选项（根据选中的 CIDR 匹配对应 IP 的网关）
  const getGatewayOptions = (cidr: string, iface: string) => {
    const opts: { label: string; value: string }[] = []
    const seen = new Set<string>()
    const ipStr = cidr.split('/')[0]
    for (const n of nodeNetworks) {
      if (iface && n.name !== iface) continue
      const ipList = ipVersion === 'ipv4' ? (n.ipv4 || []) : (n.ipv6 || [])
      for (const ip of ipList) {
        if (ip.address === ipStr && ip.gateway && !seen.has(ip.gateway)) {
          seen.add(ip.gateway)
          opts.push({ label: ip.gateway, value: ip.gateway })
        }
      }
    }
    return opts
  }

  return (
    <SlidePanel
      open={open}
      onClose={onClose}
      title="添加 EIP"
      width={960}
      footer={
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>取消</Button>
          <Button loading={loading} onClick={handleSubmit}>保存</Button>
        </div>
      }
    >
      <div className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">IP 版本</label>
          <Select
            value={ipVersion}
            options={[
              { label: 'IPv4', value: 'ipv4' },
              { label: 'IPv6', value: 'ipv6' },
            ]}
            onChange={(v) => setIpVersion(v as 'ipv4' | 'ipv6')}
          />
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="border-b border-gray-200 bg-gray-50 text-xs text-gray-500">
              <tr>
                <th className="px-3 py-2 text-left font-medium" style={{ minWidth: 160 }}>网卡</th>
                <th className="px-3 py-2 text-left font-medium" style={{ minWidth: 200 }}>{ipVersion === 'ipv4' ? 'IPv4 / CIDR' : 'IPv6 / CIDR'}</th>
                <th className="px-3 py-2 text-left font-medium" style={{ minWidth: 140 }}>网关</th>
                <th className="px-3 py-2 text-left font-medium" style={{ minWidth: 160 }}>别名</th>
                <th className="px-3 py-2 text-left font-medium" style={{ width: 100 }}>类型</th>
                <th className="px-3 py-2 text-right font-medium" style={{ width: 50 }}>操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {draftItems.map((item) => (
                <tr key={item.id}>
                  <td className="px-3 py-2">
                    <Select
                      value={item.interface}
                      options={ifaceOptions}
                      editable
                      placeholder="选择或输入网卡"
                      onChange={(v) => handleIfaceChange(item, String(v))}
                    />
                  </td>
                  <td className="px-3 py-2">
                    <Select
                      value={item.cidr}
                      options={getCidrOptions(item.interface)}
                      editable
                      placeholder={ipVersion === 'ipv4' ? '8.8.8.8/24' : '2001:db8::/64'}
                      onChange={(v) => handleCidrChange(item, String(v))}
                    />
                  </td>
                  <td className="px-3 py-2">
                    <Select
                      value={item.gateway}
                      options={getGatewayOptions(item.cidr, item.interface)}
                      editable
                      placeholder="选择或输入网关"
                      onChange={(v) => updateDraftItem(item.id, { gateway: String(v), gatewayManual: true })}
                    />
                  </td>
                  <td className="px-3 py-2">
                    <div className="relative">
                      <input
                        value={item.alias}
                        onChange={(e) => updateDraftItem(item.id, { alias: e.target.value })}
                        className="w-full px-2 py-1.5 border border-gray-300 rounded text-sm"
                        placeholder="可选，留空自动检测"
                      />
                    </div>
                  </td>
                  <td className="px-3 py-2">
                    <Select
                      value={item.poolType}
                      options={[
                        { label: '宿主机', value: 'host' },
                        { label: '弹性', value: 'eip' },
                      ]}
                      onChange={(v) => updateDraftItem(item.id, { poolType: v as 'host' | 'eip' })}
                    />
                  </td>
                  <td className="px-3 py-2 text-right">
                    <button
                      onClick={() => removeRow(item.id)}
                      className="text-red-500 hover:text-red-700 text-xs"
                    >
                      删除
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <div className="flex items-center justify-between">
          <button
            onClick={addRow}
            className="inline-flex items-center gap-1.5 rounded-md border border-gray-300 px-3 py-1.5 text-xs text-gray-700 hover:bg-gray-50"
          >
            <Plus size={14} />
            添加{ipVersion === 'ipv4' ? ' IPv4' : ' IPv6 前缀'}
          </button>
        </div>

        <div className="text-xs text-gray-400 space-y-1">
          <p>CIDR 是宿主机网卡上的 IP 和子网掩码，如 172.18.10.10/24</p>
          <p>类型：宿主机 = 仅用于网桥 NAT 出口，不能分配给实例；弹性 = 可分配给实例或网桥</p>
          <p>别名：可填域名或 IP，留空时内网 IP 自动检测出口 IP，公网 IP 直接使用 IP 本身</p>
        </div>
      </div>
    </SlidePanel>
  )
}

// ============== 主页面 ==============
export default function NetworkPage() {
  const toast = useToastStore()
  const [tab, setTab] = useState<'bridge' | 'eip_pool' | 'eip_allocation'>('bridge')

  // 节点选择
  const [nodes, setNodes] = useState<Node[]>([])
  const [selectedNodeId, setSelectedNodeId] = useState<string>('')

  // Bridge 状态
  const [bridges, setBridges] = useState<Bridge[]>([])
  const [bridgesLoading, setBridgesLoading] = useState(false)
  const [bridgePanelOpen, setBridgePanelOpen] = useState(false)
  const [bridgeFormMode, setBridgeFormMode] = useState<'create' | 'edit'>('create')
  const [editBridge, setEditBridge] = useState<Bridge | null>(null)

  // EIP 池状态
  const [eipPools, setEipPools] = useState<EIPPool[]>([])
  const [eipPoolsLoading, setEipPoolsLoading] = useState(false)
  const [eipPoolPanelOpen, setEipPoolPanelOpen] = useState(false)

  // EIP 分配状态
  const [eipAllocations, setEipAllocations] = useState<EIPAllocation[]>([])
  const [eipAllocationsLoading, setEipAllocationsLoading] = useState(false)

  useEffect(() => {
    apiClient.get('/nodes').then((r) => {
      const list = r.data.data || []
      setNodes(list)
      if (list.length > 0 && !selectedNodeId) {
        setSelectedNodeId(list[0].id)
      }
    })
  }, [])

  const fetchBridges = () => {
    if (!selectedNodeId) return
    setBridgesLoading(true)
    apiClient.get(`/network/bridges?node_id=${selectedNodeId}`).then((res) => setBridges(res.data.data || [])).finally(() => setBridgesLoading(false))
  }

  const fetchEIPPools = () => {
    if (!selectedNodeId) return
    setEipPoolsLoading(true)
    apiClient.get(`/network/eip-pools?node_id=${selectedNodeId}`).then((res) => setEipPools(res.data.data || [])).finally(() => setEipPoolsLoading(false))
  }

  const fetchEIPAllocations = () => {
    if (!selectedNodeId) return
    setEipAllocationsLoading(true)
    apiClient.get(`/network/eip-allocations?node_id=${selectedNodeId}`).then((res) => setEipAllocations(res.data.data || [])).finally(() => setEipAllocationsLoading(false))
  }

  useEffect(() => {
    if (selectedNodeId) {
      fetchBridges()
      fetchEIPPools()
      fetchEIPAllocations()
    } else {
      setBridges([])
      setEipPools([])
      setEipAllocations([])
    }
  }, [selectedNodeId])

  const handleDeleteBridge = async (id: string) => {
    if (!confirm('确认删除该 Bridge？Bridge 下的实例必须先行迁移或删除。')) return
    try {
      await apiClient.delete(`/network/bridges/${id}`)
      toast.success('Bridge 已删除')
      fetchBridges()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '删除失败')
    }
  }

  const handleDeleteEIPPool = async (id: string) => {
    if (!confirm('确认删除该 EIP 资源池？')) return
    try {
      await apiClient.delete(`/network/eip-pools/${id}`)
      toast.success('EIP 池已删除')
      fetchEIPPools()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '删除失败')
    }
  }

  const handleReleaseEIP = async (id: string) => {
    if (!confirm('确认释放该 EIP 分配？')) return
    try {
      await apiClient.post(`/network/eip-allocations/${id}/release`)
      toast.success('EIP 已释放')
      fetchEIPAllocations()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '操作失败')
    }
  }

  const bridgeColumns: Column<Bridge>[] = [
    { key: 'name', title: '名称', width: 120, render: (row) => <span className="text-sm font-number">{row.name}</span> },
    { key: 'bridge_name', title: 'Bridge', width: 140, render: (row) => <span className="text-sm font-number">{row.bridge_name}</span> },
    { key: 'ipv4_cidr', title: 'IPv4 网段', width: 180, render: (row) => <span className="text-sm font-number">{row.ipv4_enabled ? row.ipv4_cidr : '-'}</span> },
    { key: 'ipv6_cidr', title: 'IPv6 网段', width: 200, render: (row) => <span className="text-sm font-number text-gray-600">{row.ipv6_enabled ? row.ipv6_cidr : '-'}</span> },
    { key: 'dns', title: 'DNS', width: 160, render: (row) => <span className="text-sm font-number text-gray-600">{(row.dns_servers || []).join(', ') || '-'}</span> },
    {
      key: 'port_range',
      title: '端口范围',
      width: 140,
      render: (row) => <span className="text-sm font-number text-gray-600">{row.port_range_start}-{row.port_range_end}</span>,
    },
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
      render: (row: Bridge) => (
        <div className="flex items-center gap-3">
          <button className="text-sm text-blue-500 hover:text-blue-700" onClick={() => { setEditBridge(row); setBridgeFormMode('edit'); setBridgePanelOpen(true) }}>编辑</button>
          <button className="text-sm text-red-500 hover:text-red-700" onClick={() => handleDeleteBridge(row.id)}>删除</button>
        </div>
      ),
    },
  ]

  const eipPoolColumns: Column<EIPPool>[] = [
    { key: 'cidr', title: 'CIDR', width: 200, render: (row) => <span className="text-sm font-number">{row.cidr}</span> },
    { key: 'ip_version', title: 'IP 版本', width: 80, render: (row) => <span className="text-sm">{row.ip_version}</span> },
    { key: 'interface', title: '网卡', width: 120, render: (row) => <span className="text-sm font-number text-gray-600">{row.interface || '-'}</span> },
    { key: 'gateway', title: '网关', width: 140, render: (row) => <span className="text-sm font-number text-gray-600">{row.gateway || '-'}</span> },
    { key: 'pool_type', title: '类型', width: 80, render: (row) => (
      <span className={`text-xs px-2 py-0.5 rounded-full ${row.pool_type === 'eip' ? 'bg-blue-100 text-blue-700' : 'bg-gray-100 text-gray-600'}`}>
        {row.pool_type === 'eip' ? 'EIP' : 'Host'}
      </span>
    ) },
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
      render: (row: EIPPool) => (
        <button className="text-red-500 hover:text-red-700 text-sm" onClick={() => handleDeleteEIPPool(row.id)}>删除</button>
      ),
    },
  ]

  const eipAllocationColumns: Column<EIPAllocation>[] = [
    { key: 'cidr', title: 'IP/CIDR', width: 200, render: (row) => <span className="text-sm font-number">{row.cidr}</span> },
    { key: 'ip_version', title: '版本', width: 60, render: (row) => <span className="text-sm">{row.ip_version}</span> },
    { key: 'usage', title: '用途', width: 140, render: (row) => (
      <span className={`text-xs px-2 py-0.5 rounded-full ${row.usage === 'bridge_nat_egress' ? 'bg-blue-100 text-blue-700' : 'bg-purple-100 text-purple-700'}`}>
        {row.usage === 'bridge_nat_egress' ? 'Bridge NAT 出口' : '实例 EIP'}
      </span>
    ) },
    { key: 'bridge_id', title: '关联 Bridge', width: 120, render: (row) => <span className="text-sm font-number text-gray-500">{row.bridge_id?.slice(0, 8) || '-'}</span> },
    { key: 'instance_id', title: '关联实例', width: 120, render: (row) => <span className="text-sm font-number text-gray-500">{row.instance_id?.slice(0, 8) || '-'}</span> },
    {
      key: 'status',
      title: '状态',
      render: (row) => (
        <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${row.status === 'assigned' ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-600'}`}>
          {row.status === 'assigned' ? '已分配' : '已释放'}
        </span>
      ),
    },
    {
      key: 'action',
      title: '操作',
      render: (row: EIPAllocation) => (
        row.status === 'assigned' ? (
          <button className="text-red-500 hover:text-red-700 text-sm" onClick={() => handleReleaseEIP(row.id)}>释放</button>
        ) : <span className="text-gray-400 text-sm">-</span>
      ),
    },
  ]

  const nodeOptions = nodes.map((n) => ({ label: `${n.name} (${n.id.slice(0, 8)})`, value: n.id }))

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Network size={22} className="text-black" />
          <h1 className="text-xl font-semibold text-black">网络管理</h1>
        </div>
        {selectedNodeId && tab === 'bridge' && (
          <Button icon={<Plus size={16} />} onClick={() => { setEditBridge(null); setBridgeFormMode('create'); setBridgePanelOpen(true) }}>创建 Bridge</Button>
        )}
        {selectedNodeId && tab === 'eip_pool' && (
          <Button icon={<Plus size={16} />} onClick={() => setEipPoolPanelOpen(true)}>创建 EIP 池</Button>
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
              className={`px-4 py-2 text-sm font-medium ${tab === 'bridge' ? 'text-black border-b-2 border-black' : 'text-gray-500 hover:text-gray-700'}`}
              onClick={() => setTab('bridge')}
            >
              Bridge 网络
            </button>
            <button
              className={`px-4 py-2 text-sm font-medium ${tab === 'eip_pool' ? 'text-black border-b-2 border-black' : 'text-gray-500 hover:text-gray-700'}`}
              onClick={() => setTab('eip_pool')}
            >
              EIP 资源池
            </button>
            <button
              className={`px-4 py-2 text-sm font-medium ${tab === 'eip_allocation' ? 'text-black border-b-2 border-black' : 'text-gray-500 hover:text-gray-700'}`}
              onClick={() => setTab('eip_allocation')}
            >
              EIP 分配
            </button>
          </div>

          {tab === 'bridge' && (
            <DataTable columns={bridgeColumns} data={bridges} rowKey={(r) => r.id} loading={bridgesLoading} emptyText="暂无 Bridge 网络" />
          )}

          {tab === 'eip_pool' && (
            <DataTable columns={eipPoolColumns} data={eipPools} rowKey={(r) => r.id} loading={eipPoolsLoading} emptyText="暂无 EIP 资源池" />
          )}

          {tab === 'eip_allocation' && (
            <DataTable columns={eipAllocationColumns} data={eipAllocations} rowKey={(r) => r.id} loading={eipAllocationsLoading} emptyText="暂无 EIP 分配记录" />
          )}
        </>
      ) : (
        <div className="flex flex-col items-center justify-center py-20 text-gray-400">
          <Network size={48} className="mb-3 opacity-40" />
          <p className="text-sm">请先选择宿主机节点以管理网络</p>
        </div>
      )}

      <BridgeFormPanel open={bridgePanelOpen} mode={bridgeFormMode} bridge={editBridge} nodeId={selectedNodeId} existingBridges={bridges} onClose={() => setBridgePanelOpen(false)} onSuccess={fetchBridges} />
      <EIPPoolFormPanel open={eipPoolPanelOpen} nodeId={selectedNodeId} existingPools={eipPools} onClose={() => setEipPoolPanelOpen(false)} onSuccess={fetchEIPPools} />
    </div>
  )
}
