import { useEffect, useState } from 'react'
import { Plus } from 'lucide-react'
import apiClient from '@/api/client'
import { Button } from '@/components/Button/Button'
import { Select } from '@/components/Select/Select'
import { SlidePanel } from '@/components/SlidePanel/SlidePanel'
import { useToastStore } from '@/stores/toast'
import type { EIPPool, EIPPoolDraftItem, NodeNetwork } from './types'
import { makeDraftItem } from './types'

interface EIPPoolFormPanelProps {
  open: boolean
  nodeId: string
  existingPools: EIPPool[]
  onClose: () => void
  onSuccess: () => void
}

export function EIPPoolFormPanel({ open, nodeId, existingPools: _existingPools, onClose, onSuccess }: EIPPoolFormPanelProps) {
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

  const handleCidrChange = (item: EIPPoolDraftItem, value: string) => {
    const patch: Partial<EIPPoolDraftItem> = { cidr: value, cidrManual: true }
    const slashIdx = value.indexOf('/')
    if (slashIdx > 0) {
      patch.netmask = value.substring(slashIdx + 1)
    }
    updateDraftItem(item.id, patch)
  }

  const handlePrefixChange = (item: EIPPoolDraftItem, value: string) => {
    updateDraftItem(item.id, { prefix: value })
  }

  const handleHostAddrChange = (item: EIPPoolDraftItem, value: string) => {
    updateDraftItem(item.id, { hostAddr: value })
  }

  const handleIfaceChange = (item: EIPPoolDraftItem, iface: string) => {
    updateDraftItem(item.id, { interface: iface })
  }

  const addRow = () => {
    setDraftItems(items => [...items, makeDraftItem()])
  }

  const removeRow = (id: string) => {
    setDraftItems(items => items.filter(it => it.id !== id))
  }

  const isDuplicateInBatch = (cidr: string, currentId: string): boolean => {
    return draftItems.some(it => it.id !== currentId && it.cidr === cidr)
  }

  const handleSubmit = async () => {
    let validItems: EIPPoolDraftItem[] = []
    if (ipVersion === 'ipv4') {
      validItems = draftItems.filter(it => it.cidr.trim())
    } else {
      validItems = draftItems.filter(it => it.hostAddr.trim() && it.prefix.trim()).map(it => ({
        ...it,
        cidr: `${it.hostAddr}/${it.prefix}`,
      }))
    }
    if (validItems.length === 0) {
      toast.error('请至少添加一条 IP 记录')
      return
    }
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

  const ifaceOptions = nodeNetworks.map(n => {
    const ipList = ipVersion === 'ipv4' ? (n.ipv4 || []) : (n.ipv6 || [])
    const ipStr = ipList.map(ip => ip.address).join(', ') || '无IP'
    return { label: `${n.name} (${ipStr})`, value: n.name }
  })

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
