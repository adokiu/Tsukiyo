import { useEffect, useState } from 'react'
import { Plus } from 'lucide-react'
import apiClient from '@/api/client'
import { DataTable, type Column } from '@/components/DataTable/DataTable'
import { Button } from '@/components/Button/Button'
import { Select } from '@/components/Select/Select'
import { SlidePanel } from '@/components/SlidePanel/SlidePanel'
import { Modal } from '@/components/Modal/Modal'
import { Tooltip } from '@/components/Tooltip/Tooltip'
import { useToastStore } from '@/stores/toast'
import { PageLayout, type PageTab } from '@/components/PageLayout/PageLayout'
import '@/components/PageTransition/PageTransition.css'

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
  ipv6_eip_pool_id?: string
  nat_egress_ipv4_addr?: string
  port_range_start: number
  port_range_end: number
  port_used: number
  port_total: number
  status: string
  instance_count: number
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
  netmask_prefix: number
  alias: string
  pool_type: string
  status: string
  used_count: number
  total_ips: number
  created_at: string
}

interface EIPAllocation {
  id: string
  pool_id: string
  node_id: string
  cidr: string
  prefix_len: number
  ip_version: string
  alias: string
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
  const [ipv6EipPoolId, setIpv6EipPoolId] = useState('')
  const [ipv6PrefixLen, setIpv6PrefixLen] = useState(64)
  const [ipv6SpecificIP, setIpv6SpecificIP] = useState('')
  const [ipv6AvailableAddrs, setIpv6AvailableAddrs] = useState<string[]>([])
  const [dnsServers, setDnsServers] = useState('')
  const [portRangeStart, setPortRangeStart] = useState(20000)
  const [portRangeEnd, setPortRangeEnd] = useState(65535)
  const [hasInstances, setHasInstances] = useState(false)

  // NAT 出口 EIP 选择
  const [natEgressV4PoolId, setNatEgressV4PoolId] = useState('')
  const [natEgressV4Info, setNatEgressV4Info] = useState('')
  const [natEgressV4AddrList, setNatEgressV4AddrList] = useState<string[]>([])
  const [natEgressV4SelectedIP, setNatEgressV4SelectedIP] = useState('')

  useEffect(() => {
    if (!open) return
    if (mode === 'edit' && bridge) {
      setName(bridge.name)
      setIpv4Enabled(bridge.ipv4_enabled)
      setIpv4Cidr(bridge.ipv4_cidr)
      setGatewayV4(bridge.ipv4_gateway || '')
      setIpv6Enabled(bridge.ipv6_enabled)
      setIpv6EipPoolId(bridge.ipv6_eip_pool_id || '')
      setIpv6PrefixLen(64)
      setIpv6SpecificIP('')
      setDnsServers((bridge.dns_servers || []).join(', '))
      setPortRangeStart(bridge.port_range_start)
      setPortRangeEnd(bridge.port_range_end)
      setHasInstances((bridge.instance_count || 0) > 0)
      // 编辑模式下显示已绑定的 NAT 出口 EIP 信息
      setNatEgressV4PoolId('')
      setNatEgressV4Info(bridge.nat_egress_ipv4_id ? `已绑定: ${bridge.nat_egress_ipv4_id.slice(0, 8)}` : '未绑定')
    } else {
      setName('')
      setIpv4Enabled(true)
      setIpv4Cidr('')
      setGatewayV4('')
      setIpv6Enabled(false)
      setIpv6EipPoolId('')
      setIpv6PrefixLen(64)
      setIpv6SpecificIP('')
      setDnsServers('')
      setPortRangeStart(20000)
      setPortRangeEnd(65535)
      setHasInstances(false)
      setNatEgressV4PoolId('')
      setNatEgressV4Info('')
      setNatEgressV4AddrList([])
      setNatEgressV4SelectedIP('')
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

  // NAT 出口 IPv4 池变化时查询可用地址列表（固定 /32）
  useEffect(() => {
    if (!natEgressV4PoolId) { setNatEgressV4AddrList([]); return }
    apiClient.get('/network/eip-available-list', { params: { pool_id: natEgressV4PoolId, prefix_len: 32, max_count: 10 } }).then((r) => {
      setNatEgressV4AddrList(r.data.addresses || [])
    }).catch(() => setNatEgressV4AddrList([]))
  }, [natEgressV4PoolId])

  // IPv6 EIP 池变化时查询可用子段列表
  useEffect(() => {
    if (!ipv6EipPoolId || !ipv6Enabled) { setIpv6AvailableAddrs([]); return }
    apiClient.get('/network/eip-available-list', { params: { pool_id: ipv6EipPoolId, prefix_len: ipv6PrefixLen, max_count: 10 } }).then((r) => {
      setIpv6AvailableAddrs(r.data.addresses || [])
    }).catch(() => setIpv6AvailableAddrs([]))
  }, [ipv6EipPoolId, ipv6PrefixLen, ipv6Enabled])

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
        ipv6_eip_pool_id: ipv6EipPoolId || undefined,
        ipv6_prefix_len: ipv6PrefixLen,
        ipv6_specific_ip: ipv6SpecificIP || undefined,
        dns_servers: dnsServers ? dnsServers.split(',').map(s => s.trim()).filter(Boolean) : [],
        port_range_start: portRangeStart,
        port_range_end: portRangeEnd,
      }
      // 创建模式时传递 NAT 出口 EIP 池 ID 和指定 IP
      if (mode === 'create') {
        if (natEgressV4PoolId) {
          payload.nat_egress_v4_pool_id = natEgressV4PoolId
          if (natEgressV4SelectedIP) payload.nat_egress_v4_specific_ip = natEgressV4SelectedIP
        }
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
  const handleUnbindEgress = async (ipVersion: 'ipv4') => {
    if (!bridge) return
    try {
      await apiClient.post(`/network/bridges/${bridge.id}/unbind-egress`, { ip_version: ipVersion })
      toast.success(`IPv4 NAT 出口 EIP 已解绑`)
      setNatEgressV4Info('未绑定')
      onSuccess()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '解绑失败')
    }
  }

  // 绑定 NAT 出口 EIP（编辑模式，从选中的 EIP 池分配）
  const handleBindEgress = async (ipVersion: 'ipv4') => {
    if (!bridge) return
    const poolId = natEgressV4PoolId
    if (!poolId) {
      toast.error('请先选择 EIP 资源池')
      return
    }
    const specificIP = natEgressV4SelectedIP
    try {
      await apiClient.post(`/network/bridges/${bridge.id}/bind-egress`, {
        pool_id: poolId,
        ip_version: ipVersion,
        specific_ip: specificIP || undefined,
      })
      toast.success(`IPv4 NAT 出口 EIP 已绑定`)
      setNatEgressV4Info('已绑定')
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
          <label className="block text-sm font-medium text-secondary mb-1">Bridge 名称 <span className="text-red-500">*</span></label>
          <input value={name} onChange={(e) => setName(e.target.value)} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" placeholder="如：生产网络-01" />
        </div>

        {/* IPv4 配置 */}
        <div className="border-t border-surface-light pt-3">
          <div className="flex items-center gap-2 mb-3">
            <input type="checkbox" id="ipv4_enabled" checked={ipv4Enabled} onChange={(e) => setIpv4Enabled(e.target.checked)} className="w-4 h-4" />
            <label htmlFor="ipv4_enabled" className="text-sm font-medium text-secondary">启用 IPv4</label>
          </div>
          {ipv4Enabled && (
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-secondary mb-1">IPv4 CIDR <span className="text-red-500">*</span> {hasInstances && <span className="text-amber-600 text-xs">(已锁定)</span>}</label>
                <div className="flex gap-2">
                  <input value={ipv4Cidr} disabled={hasInstances} onChange={(e) => setIpv4Cidr(e.target.value)} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm disabled:bg-surface-secondary" placeholder="10.10.1.0/24" />
                  {!hasInstances && (
                    <button type="button" onClick={autoGenerateV4Cidr} className="shrink-0 px-3 py-2 border border-surface-strong rounded-lg text-xs text-tertiary hover:bg-surface-secondary whitespace-nowrap">自动生成</button>
                  )}
                </div>
              </div>
              <div>
                <label className="block text-sm font-medium text-secondary mb-1">IPv4 网关 {hasInstances && <span className="text-amber-600 text-xs">(已锁定)</span>}</label>
                <input value={gatewayV4} disabled={hasInstances} onChange={(e) => setGatewayV4(e.target.value)} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm disabled:bg-surface-secondary" placeholder="自动推断" />
              </div>
            </div>
          )}
        </div>

        {/* IPv6 配置 */}
        <div className="border-t border-surface-light pt-3">
          <div className="flex items-center gap-2 mb-3">
            <input type="checkbox" id="ipv6_enabled" checked={ipv6Enabled} onChange={(e) => setIpv6Enabled(e.target.checked)} className="w-4 h-4" />
            <label htmlFor="ipv6_enabled" className="text-sm font-medium text-secondary">启用 IPv6</label>
          </div>
          {ipv6Enabled && (
            <div className="space-y-3">
              <div>
                <label className="block text-sm font-medium text-secondary mb-1">IPv6 EIP 资源池 <span className="text-red-500">*</span></label>
                <Select
                  value={ipv6EipPoolId}
                  options={v6PoolOptions}
                  placeholder="选择 IPv6 EIP 资源池"
                  onChange={(v) => { setIpv6EipPoolId(v as string); setIpv6SpecificIP('') }}
                />
                {v6PoolOptions.length === 0 && (
                  <p className="text-xs text-muted mt-1">暂无 IPv6 EIP 资源池，请先创建</p>
                )}
              </div>
              <div>
                <label className="block text-sm font-medium text-secondary mb-1">IPv6 前缀长度</label>
                <input
                  type="number"
                  value={ipv6PrefixLen}
                  onChange={(e) => setIpv6PrefixLen(Number(e.target.value))}
                  className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm"
                  placeholder="如 64"
                  min={48}
                  max={128}
                />
                <p className="text-xs text-muted mt-1">从 EIP 池中切出此长度的子段作为 bridge IPv6 CIDR</p>
              </div>
              {ipv6EipPoolId && ipv6AvailableAddrs.length > 0 && (
                <div>
                  <label className="block text-sm font-medium text-secondary mb-1">指定子段（留空自动分配）</label>
                  <Select
                    value={ipv6SpecificIP}
                    options={ipv6AvailableAddrs.map(addr => ({ label: addr, value: addr }))}
                    placeholder="选择可用子段（留空自动分配）"
                    onChange={(v) => setIpv6SpecificIP(String(v))}
                  />
                </div>
              )}
            </div>
          )}
        </div>

        {/* NAT 出口 EIP 配置 */}
        <div className="border-t border-surface-light pt-3">
          <label className="block text-sm font-medium text-secondary mb-2">IPv4 NAT 出口 EIP</label>
          {ipv4Enabled && (
            <div className="mb-3 space-y-2">
              <label className="block text-xs text-tertiary">IPv4 NAT 出口（/32）</label>
              {mode === 'create' ? (
                <>
                  <Select
                    value={natEgressV4PoolId}
                    options={v4PoolOptions}
                    placeholder="选择 IPv4 EIP 资源池（可选）"
                    onChange={(v) => { setNatEgressV4PoolId(v as string); setNatEgressV4SelectedIP('') }}
                  />
                  {natEgressV4PoolId && natEgressV4AddrList.length > 0 && (
                    <Select
                      value={natEgressV4SelectedIP}
                      options={natEgressV4AddrList.map(addr => ({ label: addr, value: addr.split('/')[0] }))}
                      placeholder="选择可用 IP（留空自动分配）"
                      onChange={(v) => setNatEgressV4SelectedIP(String(v))}
                    />
                  )}
                  {natEgressV4PoolId && natEgressV4AddrList.length === 0 && (
                    <p className="text-xs text-muted">该池无可用 IP</p>
                  )}
                </>
              ) : (
                <div className="space-y-2">
                  <div className="flex items-center gap-2">
                    <span className="text-sm text-tertiary flex-1">{natEgressV4Info}</span>
                    {natEgressV4Info !== '未绑定' && (
                      <button type="button" onClick={() => handleUnbindEgress('ipv4')} className="text-xs text-red-500 hover:text-red-700">解绑</button>
                    )}
                  </div>
                  {natEgressV4Info === '未绑定' && v4PoolOptions.length > 0 && (
                    <>
                      <Select
                        value={natEgressV4PoolId}
                        options={v4PoolOptions}
                        placeholder="选择 EIP 池"
                        onChange={(v) => { setNatEgressV4PoolId(v as string); setNatEgressV4SelectedIP('') }}
                      />
                      {natEgressV4AddrList.length > 0 && (
                        <Select
                          value={natEgressV4SelectedIP}
                          options={natEgressV4AddrList.map(addr => ({ label: addr, value: addr.split('/')[0] }))}
                          placeholder="选择可用 IP（留空自动分配）"
                          onChange={(v) => setNatEgressV4SelectedIP(String(v))}
                        />
                      )}
                      <button type="button" onClick={() => handleBindEgress('ipv4')} className="text-xs text-blue-500 hover:text-blue-700">绑定</button>
                    </>
                  )}
                </div>
              )}
            </div>
          )}
          {v4PoolOptions.length === 0 && (
            <p className="text-xs text-muted">暂无可用 IPv4 EIP 资源池，请先在 EIP 资源池标签页中创建</p>
          )}
        </div>

        <div>
          <label className="block text-sm font-medium text-secondary mb-1">DNS 服务器（逗号分隔）</label>
          <input value={dnsServers} onChange={(e) => setDnsServers(e.target.value)} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" placeholder="8.8.8.8, 8.8.4.4" />
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-secondary mb-1">端口映射起始</label>
            <input type="number" value={portRangeStart} onChange={(e) => setPortRangeStart(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" />
          </div>
          <div>
            <label className="block text-sm font-medium text-secondary mb-1">端口映射结束</label>
            <input type="number" value={portRangeEnd} onChange={(e) => setPortRangeEnd(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" />
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
  hostAddr: string
  netmask: string
  alias: string
  poolType: 'host' | 'eip'
  detecting: boolean
}

function makeDraftItem(): EIPPoolDraftItem {
  return { id: Math.random().toString(36).slice(2), cidr: '', cidrManual: false, gateway: '', gatewayManual: false, interface: '', prefix: '', hostAddr: '', netmask: '', alias: '', poolType: 'eip', detecting: false }
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

  // IPv4: CIDR 变化时提取掩码
  const handleCidrChange = (item: EIPPoolDraftItem, value: string) => {
    const patch: Partial<EIPPoolDraftItem> = { cidr: value, cidrManual: true }
    const slashIdx = value.indexOf('/')
    if (slashIdx > 0) {
      patch.netmask = value.substring(slashIdx + 1)
    }
    updateDraftItem(item.id, patch)
  }

  // IPv6: 前缀变化
  const handlePrefixChange = (item: EIPPoolDraftItem, value: string) => {
    updateDraftItem(item.id, { prefix: value })
  }

  // IPv6: 宿主地址变化
  const handleHostAddrChange = (item: EIPPoolDraftItem, value: string) => {
    updateDraftItem(item.id, { hostAddr: value })
  }

  // 网卡选择
  const handleIfaceChange = (item: EIPPoolDraftItem, iface: string) => {
    updateDraftItem(item.id, { interface: iface })
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
    let validItems: EIPPoolDraftItem[] = []
    if (ipVersion === 'ipv4') {
      validItems = draftItems.filter(it => it.cidr.trim())
    } else {
      // IPv6: 合并 hostAddr + prefix 为 cidr
      validItems = draftItems.filter(it => it.hostAddr.trim() && it.prefix.trim()).map(it => ({
        ...it,
        cidr: `${it.hostAddr}/${it.prefix}`,
      }))
    }
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
        if (ipVersion === 'ipv4' && item.netmask) {
          payload.netmask_prefix = parseInt(item.netmask) || 0
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

  // 每行的 CIDR 选项（IPv4 根据选中的网卡过滤）
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

  // IPv6 宿主地址选项
  const getHostAddrOptions = (iface: string) => {
    const opts: { label: string; value: string }[] = []
    for (const n of nodeNetworks) {
      if (iface && n.name !== iface) continue
      const ipList = (n.ipv6 || [])
      for (const ip of ipList) {
        opts.push({ label: ip.address, value: ip.address })
      }
    }
    return opts
  }

  // 每行的网关选项
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
      width={ipVersion === 'ipv4' ? 1060 : 960}
      footer={
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>取消</Button>
          <Button loading={loading} onClick={handleSubmit}>保存</Button>
        </div>
      }
    >
      <div className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-secondary mb-1">IP 版本</label>
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
            <thead className="border-b border-surface bg-surface-secondary text-xs text-tertiary">
              {ipVersion === 'ipv4' ? (
                <tr>
                  <th className="px-3 py-2 text-left font-medium" style={{ minWidth: 150 }}>网卡</th>
                  <th className="px-3 py-2 text-left font-medium" style={{ minWidth: 180 }}>IPv4 / CIDR</th>
                  <th className="px-3 py-2 text-left font-medium" style={{ minWidth: 100 }}>掩码</th>
                  <th className="px-3 py-2 text-left font-medium" style={{ minWidth: 140 }}>网关</th>
                  <th className="px-3 py-2 text-left font-medium" style={{ minWidth: 140 }}>别名</th>
                  <th className="px-3 py-2 text-left font-medium" style={{ width: 100 }}>类型</th>
                  <th className="px-3 py-2 text-right font-medium" style={{ width: 50 }}>操作</th>
                </tr>
              ) : (
                <tr>
                  <th className="px-3 py-2 text-left font-medium" style={{ minWidth: 120 }}>前缀</th>
                  <th className="px-3 py-2 text-left font-medium" style={{ minWidth: 180 }}>宿主地址</th>
                  <th className="px-3 py-2 text-left font-medium" style={{ minWidth: 150 }}>网卡</th>
                  <th className="px-3 py-2 text-left font-medium" style={{ minWidth: 140 }}>网关</th>
                  <th className="px-3 py-2 text-left font-medium" style={{ minWidth: 100 }}>类型</th>
                  <th className="px-3 py-2 text-right font-medium" style={{ width: 50 }}>操作</th>
                </tr>
              )}
            </thead>
            <tbody className="divide-y divide-surface-light">
              {draftItems.map((item) => (
                ipVersion === 'ipv4' ? (
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
                        placeholder="8.8.8.8/24"
                        onChange={(v) => handleCidrChange(item, String(v))}
                      />
                    </td>
                    <td className="px-3 py-2">
                      <input
                        value={item.netmask}
                        onChange={(e) => updateDraftItem(item.id, { netmask: e.target.value })}
                        className="w-full px-2 py-1.5 border border-surface-strong rounded text-sm"
                        placeholder="如 24"
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
                      <input
                        value={item.alias}
                        onChange={(e) => updateDraftItem(item.id, { alias: e.target.value })}
                        className="w-full px-2 py-1.5 border border-surface-strong rounded text-sm"
                        placeholder="可选"
                      />
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
                      <button onClick={() => removeRow(item.id)} className="text-red-500 hover:text-red-700 text-xs">删除</button>
                    </td>
                  </tr>
                ) : (
                  <tr key={item.id}>
                    <td className="px-3 py-2">
                      <input
                        value={item.prefix}
                        onChange={(e) => handlePrefixChange(item, e.target.value)}
                        className="w-full px-2 py-1.5 border border-surface-strong rounded text-sm"
                        placeholder="如 64"
                      />
                    </td>
                    <td className="px-3 py-2">
                      <Select
                        value={item.hostAddr}
                        options={getHostAddrOptions(item.interface)}
                        editable
                        placeholder="如 240e:525::"
                        onChange={(v) => handleHostAddrChange(item, String(v))}
                      />
                    </td>
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
                        value={item.gateway}
                        options={getGatewayOptions(item.hostAddr, item.interface)}
                        editable
                        placeholder="选择或输入网关"
                        onChange={(v) => updateDraftItem(item.id, { gateway: String(v), gatewayManual: true })}
                      />
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
                      <button onClick={() => removeRow(item.id)} className="text-red-500 hover:text-red-700 text-xs">删除</button>
                    </td>
                  </tr>
                )
              ))}
            </tbody>
          </table>
        </div>

        <div className="flex items-center justify-between">
          <button
            onClick={addRow}
            className="inline-flex items-center gap-1.5 rounded-md border border-surface-strong px-3 py-1.5 text-xs text-secondary hover:bg-surface-secondary"
          >
            <Plus size={14} />
            添加{ipVersion === 'ipv4' ? ' IPv4' : ' IPv6 前缀'}
          </button>
        </div>

        <div className="text-xs text-muted space-y-1">
          {ipVersion === 'ipv4' ? (
            <>
              <p>IPv4 / CIDR：该 IP 池包含的 IP 范围，如 8.8.8.8/24 表示 256 个 IP</p>
              <p>掩码：宿主机网卡上该 IP 的子网掩码前缀长度，如 24（即 255.255.255.0）</p>
            </>
          ) : (
            <>
              <p>前缀：IPv6 前缀长度，如 64</p>
              <p>宿主地址：IPv6 前缀地址，如 240e:525::，与前缀组合为 240e:525::/64</p>
            </>
          )}
          <p>类型：宿主机 = 仅用于网桥 NAT 出口，不能分配给实例；弹性 = 可分配给实例或网桥</p>
          <p>别名：DMZ 场景下对应的公网 IP 段（CIDR），前缀长度必须与池 CIDR 相同，如池 172.19.10.10/30 则别名填 125.25.1.10/30</p>
        </div>
      </div>
    </SlidePanel>
  )
}

// ============== EIP 池编辑面板 ==============
function EIPPoolEditPanel({ open, pool, onClose, onSuccess }: {
  open: boolean
  pool: EIPPool | null
  onClose: () => void
  onSuccess: () => void
}) {
  const toast = useToastStore()
  const [loading, setLoading] = useState(false)
  const [interfaceName, setInterfaceName] = useState('')
  const [gateway, setGateway] = useState('')
  const [alias, setAlias] = useState('')
  const [netmaskPrefix, setNetmaskPrefix] = useState(0)
  const [poolType, setPoolType] = useState('eip')
  const [status, setStatus] = useState('active')

  useEffect(() => {
    if (pool) {
      setInterfaceName(pool.interface || '')
      setGateway(pool.gateway || '')
      setAlias(pool.alias || '')
      setNetmaskPrefix(pool.netmask_prefix || 0)
      setPoolType(pool.pool_type || 'eip')
      setStatus(pool.status || 'active')
    }
  }, [pool])

  const handleSubmit = async () => {
    if (!pool) return
    setLoading(true)
    try {
      await apiClient.put(`/network/eip-pools/${pool.id}`, {
        interface: interfaceName,
        gateway,
        alias,
        netmask_prefix: netmaskPrefix,
        pool_type: poolType,
        status,
      })
      toast.success('EIP 池已更新')
      onSuccess()
      onClose()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '操作失败')
    } finally {
      setLoading(false)
    }
  }

  if (!pool) return null

  return (
    <SlidePanel
      open={open}
      onClose={onClose}
      title={`编辑 EIP 池 - ${pool.cidr}`}
      width={500}
      footer={
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>取消</Button>
          <Button loading={loading} onClick={handleSubmit}>保存</Button>
        </div>
      }
    >
      <div className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-secondary mb-1">CIDR</label>
          <input value={pool.cidr} disabled className="w-full px-3 py-2 border border-surface-strong rounded text-sm bg-surface-secondary text-tertiary" />
        </div>
        <div>
          <label className="block text-sm font-medium text-secondary mb-1">IP 版本</label>
          <input value={pool.ip_version} disabled className="w-full px-3 py-2 border border-surface-strong rounded text-sm bg-surface-secondary text-tertiary" />
        </div>
        <div>
          <label className="block text-sm font-medium text-secondary mb-1">网卡</label>
          <input value={interfaceName} onChange={(e) => setInterfaceName(e.target.value)} className="w-full px-3 py-2 border border-surface-strong rounded text-sm" placeholder="如 eth0" />
        </div>
        {pool.ip_version === 'ipv4' && (
          <div>
            <label className="block text-sm font-medium text-secondary mb-1">掩码前缀</label>
            <input type="number" value={netmaskPrefix} onChange={(e) => setNetmaskPrefix(parseInt(e.target.value) || 0)} className="w-full px-3 py-2 border border-surface-strong rounded text-sm" placeholder="如 24" />
          </div>
        )}
        <div>
          <label className="block text-sm font-medium text-secondary mb-1">网关</label>
          <input value={gateway} onChange={(e) => setGateway(e.target.value)} className="w-full px-3 py-2 border border-surface-strong rounded text-sm" placeholder="如 172.18.10.254" />
        </div>
        <div>
          <label className="block text-sm font-medium text-secondary mb-1">别名 (公网 CIDR)</label>
          <input value={alias} onChange={(e) => setAlias(e.target.value)} className="w-full px-3 py-2 border border-surface-strong rounded text-sm" placeholder="如 125.25.1.10/30，IP 数量需与池一致" />
          <p className="text-xs text-muted mt-1">DMZ 场景下对应的公网 IP 段，前缀长度必须与池 CIDR 相同</p>
        </div>
        <div>
          <label className="block text-sm font-medium text-secondary mb-1">类型</label>
          <Select
            value={poolType}
            options={[
              { label: '宿主机', value: 'host' },
              { label: '弹性', value: 'eip' },
            ]}
            onChange={(v) => setPoolType(v as string)}
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-secondary mb-1">状态</label>
          <Select
            value={status}
            options={[
              { label: '启用', value: 'active' },
              { label: '禁用', value: 'disabled' },
            ]}
            onChange={(v) => setStatus(v as string)}
          />
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
  const [editEIPPool, setEditEIPPool] = useState<EIPPool | null>(null)
  const [eipPoolEditOpen, setEipPoolEditOpen] = useState(false)

  // EIP 分配状态
  const [eipAllocations, setEipAllocations] = useState<EIPAllocation[]>([])
  const [eipAllocationsLoading, setEipAllocationsLoading] = useState(false)

  // 确认弹窗状态
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [confirmTitle, setConfirmTitle] = useState('')
  const [confirmMessage, setConfirmMessage] = useState('')
  const [confirmAction, setConfirmAction] = useState<() => void>(() => {})
  const [confirmRequireInput, setConfirmRequireInput] = useState(false)
  const [confirmRequireLabel, setConfirmRequireLabel] = useState('')
  const [confirmRequireValue, setConfirmRequireValue] = useState('')

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

  const handleDeleteBridge = async (id: string, name: string) => {
    setConfirmTitle('删除 Bridge')
    setConfirmMessage(`确认删除 Bridge「${name}」？Bridge 下的实例必须先行迁移或删除。`)
    setConfirmRequireInput(true)
    setConfirmRequireLabel('请输入 Bridge 名称以确认删除')
    setConfirmRequireValue(name)
    setConfirmAction(() => async () => {
      try {
        await apiClient.delete(`/network/bridges/${id}`)
        toast.success('Bridge 已删除')
        fetchBridges()
      } catch (err: any) {
        toast.error(err.response?.data?.error || '删除失败')
      }
    })
    setConfirmOpen(true)
  }

  const handleDeleteEIPPool = async (id: string, cidr: string) => {
    setConfirmTitle('删除 EIP 资源池')
    setConfirmMessage(`确认删除 EIP 资源池「${cidr}」？`)
    setConfirmRequireInput(true)
    setConfirmRequireLabel('请输入 CIDR 以确认删除')
    setConfirmRequireValue(cidr)
    setConfirmAction(() => async () => {
      try {
        await apiClient.delete(`/network/eip-pools/${id}`)
        toast.success('EIP 池已删除')
        fetchEIPPools()
      } catch (err: any) {
        toast.error(err.response?.data?.error || '删除失败')
      }
    })
    setConfirmOpen(true)
  }

  const handleReleaseEIP = async (id: string, cidr: string) => {
    setConfirmTitle('释放 EIP 分配')
    setConfirmMessage(`确认释放 EIP「${cidr}」？`)
    setConfirmRequireInput(false)
    setConfirmAction(() => async () => {
      try {
        await apiClient.post(`/network/eip-allocations/${id}/release`)
        toast.success('EIP 已释放')
        fetchEIPAllocations()
      } catch (err: any) {
        toast.error(err.response?.data?.error || '操作失败')
      }
    })
    setConfirmOpen(true)
  }

  const bridgeColumns: Column<Bridge>[] = [
    { key: 'id', title: 'ID', width: 100, render: (row) => <span className="text-sm font-number text-tertiary">{row.id.slice(0, 8)}</span> },
    { key: 'name', title: '名称', width: 120, render: (row) => <span className="text-sm font-number">{row.name}</span> },
    { key: 'bridge_name', title: 'Bridge', width: 140, render: (row) => <span className="text-sm font-number">{row.bridge_name}</span> },
    { key: 'ipv4_cidr', title: 'IPv4 网段', width: 180, render: (row) => <span className="text-sm font-number">{row.ipv4_enabled ? row.ipv4_cidr : '-'}</span> },
    { key: 'ipv6_cidr', title: 'IPv6 网段', width: 200, render: (row) => <span className="text-sm font-number text-tertiary">{row.ipv6_enabled ? row.ipv6_cidr : '-'}</span> },
    {
      key: 'nat_egress_v4',
      title: 'NAT 出口 v4',
      width: 140,
      render: (row) => <span className="text-sm font-number text-tertiary">{row.nat_egress_ipv4_addr || '-'}</span>,
    },
    {
      key: 'dns',
      title: 'DNS',
      width: 160,
      render: (row) => {
        const dns = (row.dns_servers || []).join(', ') || '-'
        return (
          <Tooltip content={dns}>
            <span className="text-sm font-number text-tertiary">{dns}</span>
          </Tooltip>
        )
      },
    },
    {
      key: 'port_usage',
      title: '端口使用',
      width: 120,
      render: (row) => <span className="text-sm font-number text-tertiary">{row.port_used} / {row.port_total}</span>,
    },
    {
      key: 'instance_count',
      title: '实例数',
      width: 80,
      render: (row) => <span className="text-sm font-number text-tertiary">{row.instance_count}</span>,
    },
    {
      key: 'status',
      title: '状态',
      width: 80,
      render: (row) => (
        <span className={`data-table-tag ${row.status === 'active' ? 'data-table-tag--online' : ''}`}>
          {row.status}
        </span>
      ),
    },
    {
      key: 'action',
      title: '操作',
      width: 120,
      render: (row: Bridge) => (
        <div className="flex items-center gap-3">
          <button className="data-table-link-btn" onClick={() => { setEditBridge(row); setBridgeFormMode('edit'); setBridgePanelOpen(true) }}>编辑</button>
          <button className="data-table-link-btn" style={{ color: '#dc2626' }} onClick={() => handleDeleteBridge(row.id, row.name)}>删除</button>
        </div>
      ),
    },
  ]

  const eipPoolColumns: Column<EIPPool>[] = [
    { key: 'id', title: 'ID', width: 100, render: (row) => <span className="text-sm font-number text-tertiary">{row.id.slice(0, 8)}</span> },
    { key: 'cidr', title: 'CIDR', width: 200, render: (row) => <span className="text-sm font-number">{row.cidr}</span> },
    { key: 'ip_version', title: 'IP 版本', width: 80, render: (row) => <span className="text-sm">{row.ip_version}</span> },
    { key: 'interface', title: '网卡', width: 120, render: (row) => <span className="text-sm font-number text-tertiary">{row.interface || '-'}</span> },
    { key: 'netmask_prefix', title: '掩码', width: 80, render: (row) => <span className="text-sm font-number text-tertiary">{row.ip_version === 'ipv4' && row.netmask_prefix ? '/' + row.netmask_prefix : '-'}</span> },
    { key: 'gateway', title: '网关', width: 140, render: (row) => <span className="text-sm font-number text-tertiary">{row.gateway || '-'}</span> },
    { key: 'alias', title: '别名', width: 140, render: (row) => <span className="text-sm font-number text-tertiary">{row.alias || '-'}</span> },
    { key: 'usage', title: '使用量', width: 140, render: (row) => {
      if (row.ip_version === 'ipv6') {
        const pct = row.total_ips > 0 ? (row.used_count / row.total_ips * 100) : 0
        return <span className="text-sm font-number text-tertiary">{pct.toFixed(2)}% ({row.used_count})</span>
      }
      return <span className="text-sm font-number text-tertiary">{row.used_count} / {row.total_ips}</span>
    } },
    { key: 'pool_type', title: '类型', width: 80, render: (row) => (
      <span className={`data-table-tag ${row.pool_type === 'eip' ? 'data-table-tag--online' : ''}`}>
        {row.pool_type === 'eip' ? 'EIP' : 'Host'}
      </span>
    ) },
    {
      key: 'status',
      title: '状态',
      width: 80,
      render: (row) => (
        <span className={`data-table-tag ${row.status === 'active' ? 'data-table-tag--online' : ''}`}>
          {row.status}
        </span>
      ),
    },
    {
      key: 'action',
      title: '操作',
      width: 120,
      render: (row: EIPPool) => (
        <div className="flex gap-2">
          <button className="data-table-link-btn" onClick={() => { setEditEIPPool(row); setEipPoolEditOpen(true) }}>编辑</button>
          <button className="data-table-link-btn" style={{ color: '#dc2626' }} onClick={() => handleDeleteEIPPool(row.id, row.cidr)}>删除</button>
        </div>
      ),
    },
  ]

  const eipAllocationColumns: Column<EIPAllocation>[] = [
    { key: 'id', title: 'ID', width: 100, render: (row) => <span className="text-sm font-number text-tertiary">{row.id.slice(0, 8)}</span> },
    { key: 'cidr', title: 'IP/CIDR', width: 200, render: (row) => <span className="text-sm font-number">{row.cidr}</span> },
    { key: 'alias', title: '别名(公网)', width: 200, render: (row) => <span className="text-sm font-number text-tertiary">{row.alias || '-'}</span> },
    { key: 'ip_version', title: '版本', width: 60, render: (row) => <span className="text-sm">{row.ip_version}</span> },
    { key: 'usage', title: '用途', width: 140, render: (row) => (
      <span className={`data-table-tag ${row.usage === 'bridge_nat_egress' ? 'data-table-tag--online' : ''}`}>
        {row.usage === 'bridge_nat_egress' ? 'Bridge NAT 出口' : '实例 EIP'}
      </span>
    ) },
    { key: 'bridge_id', title: '关联 Bridge', width: 120, render: (row) => <span className="text-sm font-number text-tertiary">{row.bridge_id?.slice(0, 8) || '-'}</span> },
    { key: 'instance_id', title: '关联实例', width: 120, render: (row) => <span className="text-sm font-number text-tertiary">{row.instance_id?.slice(0, 8) || '-'}</span> },
    {
      key: 'status',
      title: '状态',
      width: 80,
      render: (row) => (
        <span className={`data-table-tag ${row.status === 'assigned' ? 'data-table-tag--online' : ''}`}>
          {row.status === 'assigned' ? '已分配' : '已释放'}
        </span>
      ),
    },
    {
      key: 'action',
      title: '操作',
      width: 80,
      render: (row: EIPAllocation) => (
        row.status === 'assigned' ? (
          <button className="data-table-link-btn" style={{ color: '#dc2626' }} onClick={() => handleReleaseEIP(row.id, row.cidr)}>释放</button>
        ) : <span className="text-muted text-sm">-</span>
      ),
    },
  ]

  const nodeOptions = nodes.map((n) => ({ label: `${n.name} (${n.id.slice(0, 8)})`, value: n.id }))

  const tabs: PageTab[] = [
    { key: 'bridge', label: 'Bridge 网络' },
    { key: 'eip_pool', label: 'EIP 资源池' },
    { key: 'eip_allocation', label: 'EIP 分配' },
  ]

  const rightSlot = (
    <>
      {selectedNodeId && tab === 'bridge' && (
        <Button icon={<Plus size={16} />} onClick={() => { setEditBridge(null); setBridgeFormMode('create'); setBridgePanelOpen(true) }}>创建 Bridge</Button>
      )}
      {selectedNodeId && tab === 'eip_pool' && (
        <Button icon={<Plus size={16} />} onClick={() => setEipPoolPanelOpen(true)}>创建 EIP 池</Button>
      )}
    </>
  )

  return (
    <PageLayout
      tabs={tabs}
      activeTab={tab}
      onTabChange={(key) => setTab(key as 'bridge' | 'eip_pool' | 'eip_allocation')}
      leftSlot={
        <Select value={selectedNodeId} options={nodeOptions} placeholder="选择节点" emptyText="无可用节点" onChange={(v) => setSelectedNodeId(v as string)} />
      }
      rightSlot={rightSlot}
    >
      {selectedNodeId ? (
        <>
          {tab === 'bridge' && (
            <div key="bridge" className="page-transition__content">
              <DataTable columns={bridgeColumns} data={bridges} rowKey={(r) => r.id} loading={bridgesLoading} emptyText="暂无 Bridge 网络" />
            </div>
          )}

          {tab === 'eip_pool' && (
            <div key="eip_pool" className="page-transition__content">
              <DataTable columns={eipPoolColumns} data={eipPools} rowKey={(r) => r.id} loading={eipPoolsLoading} emptyText="暂无 EIP 资源池" />
            </div>
          )}

          {tab === 'eip_allocation' && (
            <div key="eip_allocation" className="page-transition__content">
              <DataTable columns={eipAllocationColumns} data={eipAllocations} rowKey={(r) => r.id} loading={eipAllocationsLoading} emptyText="暂无 EIP 分配记录" />
            </div>
          )}
        </>
      ) : (
        <div className="flex flex-col items-center justify-center py-20 text-muted">
          <p className="text-sm">请先选择宿主机节点以管理网络</p>
        </div>
      )}

      <BridgeFormPanel open={bridgePanelOpen} mode={bridgeFormMode} bridge={editBridge} nodeId={selectedNodeId} existingBridges={bridges} onClose={() => setBridgePanelOpen(false)} onSuccess={fetchBridges} />
      <EIPPoolFormPanel open={eipPoolPanelOpen} nodeId={selectedNodeId} existingPools={eipPools} onClose={() => setEipPoolPanelOpen(false)} onSuccess={fetchEIPPools} />
      <EIPPoolEditPanel open={eipPoolEditOpen} pool={editEIPPool} onClose={() => setEipPoolEditOpen(false)} onSuccess={fetchEIPPools} />

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
