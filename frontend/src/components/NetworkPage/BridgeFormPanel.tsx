import { useEffect, useState } from 'react'
import apiClient from '@/api/client'
import { Button } from '@/components/Button/Button'
import { Select } from '@/components/Select/Select'
import { SlidePanel } from '@/components/SlidePanel/SlidePanel'
import { useToastStore } from '@/stores/toast'
import type { Bridge, EIPPool } from './types'

interface BridgeFormPanelProps {
  open: boolean
  mode: 'create' | 'edit'
  bridge: Bridge | null
  nodeId: string
  existingBridges: Bridge[]
  onClose: () => void
  onSuccess: () => void
}

export function BridgeFormPanel({ open, mode, bridge, nodeId, existingBridges, onClose, onSuccess }: BridgeFormPanelProps) {
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

  useEffect(() => {
    if (!open || !nodeId) {
      setEipPools([])
      return
    }
    apiClient.get(`/network/eip-pools?node_id=${nodeId}`).then((r) => {
      setEipPools(r.data.data || [])
    }).catch(() => setEipPools([]))
  }, [open, nodeId])

  useEffect(() => {
    if (!natEgressV4PoolId) { setNatEgressV4AddrList([]); return }
    apiClient.get('/network/eip-available-list', { params: { pool_id: natEgressV4PoolId, prefix_len: 32, max_count: 10 } }).then((r) => {
      setNatEgressV4AddrList(r.data.addresses || [])
    }).catch(() => setNatEgressV4AddrList([]))
  }, [natEgressV4PoolId])

  useEffect(() => {
    if (!ipv6EipPoolId || !ipv6Enabled) { setIpv6AvailableAddrs([]); return }
    apiClient.get('/network/eip-available-list', { params: { pool_id: ipv6EipPoolId, prefix_len: ipv6PrefixLen, max_count: 10 } }).then((r) => {
      setIpv6AvailableAddrs(r.data.addresses || [])
    }).catch(() => setIpv6AvailableAddrs([]))
  }, [ipv6EipPoolId, ipv6PrefixLen, ipv6Enabled])

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

  const v4PoolOptions = eipPools
    .filter(p => p.ip_version === 'ipv4' && p.status === 'active')
    .map(p => ({ label: `${p.cidr} (${p.interface || '无网卡'}) [${p.pool_type === 'host' ? '宿主机' : 'EIP'}]`, value: p.id }))

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
