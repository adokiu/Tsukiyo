import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import apiClient from '@/api/client'
import { Button } from '@/components/Button/Button'
import { Select } from '@/components/Select/Select'
import { SlidePanel } from '@/components/SlidePanel/SlidePanel'
import { useToastStore } from '@/stores/toast'
import type { Instance } from '@/pages/admin/InstancesPage'

interface EditInstancePanelProps {
  open: boolean
  instance: Instance | null
  onClose: () => void
  onSuccess: () => void
  onInstanceUpdate: (inst: Instance) => void
}

export function EditInstancePanel({ open, instance, onClose, onSuccess, onInstanceUpdate }: EditInstancePanelProps) {
  const { t } = useTranslation()
  const toast = useToastStore()
  const [saving, setSaving] = useState(false)
  const [vcpu, setVcpu] = useState(0)
  const [memoryMb, setMemoryMb] = useState(0)
  const [swapMb, setSwapMb] = useState(0)
  const [expiresAt, setExpiresAt] = useState('')
  const [networkDown, setNetworkDown] = useState(0)
  const [networkUp, setNetworkUp] = useState(0)
  const [monthlyTraffic, setMonthlyTraffic] = useState(0)
  const [overLimitAction, setOverLimitAction] = useState('shutdown')
  const [throttleMbps, setThrottleMbps] = useState(1)
  const [portMappingLimit, setPortMappingLimit] = useState(2)
  const [snapshotLimit, setSnapshotLimit] = useState(3)
  const [ioReadIops, setIoReadIops] = useState(0)
  const [ioWriteIops, setIoWriteIops] = useState(0)
  const [statusOverride, setStatusOverride] = useState('')
  const [eipPools, setEipPools] = useState<any[]>([])
  const [eipv4PoolId, setEipv4PoolId] = useState('')
  const [eipv4AddrList, setEipv4AddrList] = useState<string[]>([])
  const [eipv4SelectedIP, setEipv4SelectedIP] = useState('')
  const [eipv6AddrList, setEipv6AddrList] = useState<string[]>([])
  const [eipv6SelectedIP, setEipv6SelectedIP] = useState('')
  const [eipv6PrefixLen, setEipv6PrefixLen] = useState(128)
  const [newDiskName, setNewDiskName] = useState('')
  const [newDiskSize, setNewDiskSize] = useState(10)
  const [newDiskPool, setNewDiskPool] = useState('default')
  const [newDiskMount, setNewDiskMount] = useState('')
  const [resizeDiskId, setResizeDiskId] = useState('')
  const [resizeDiskSize, setResizeDiskSize] = useState(0)
  const [sysDiskResize, setSysDiskResize] = useState(0)
  const [sysDiskUnit, setSysDiskUnit] = useState<'MB' | 'GB' | 'TB'>('GB')
  const [diskLoading, setDiskLoading] = useState(false)
  const [eipLoading, setEipLoading] = useState(false)

  useEffect(() => {
    if (instance) {
      setVcpu(instance.vcpu)
      setMemoryMb(instance.memory_mb)
      setSwapMb(instance.swap_mb || 0)
      setExpiresAt(instance.expires_at ? instance.expires_at.slice(0, 16) : '')
      setNetworkDown(instance.network_down || 0)
      setNetworkUp(instance.network_up || 0)
      setMonthlyTraffic(instance.monthly_traffic || 0)
      setOverLimitAction(instance.over_limit_action || 'shutdown')
      setThrottleMbps(instance.throttle_mbps || 1)
      setPortMappingLimit(instance.port_mapping_limit || 2)
      setSnapshotLimit(instance.snapshot_limit || 3)
      setIoReadIops(instance.io_read_iops || 0)
      setIoWriteIops(instance.io_write_iops || 0)
      setStatusOverride(instance.status)
      setSysDiskResize(Math.round(instance.disk_mb / 1024))
      setSysDiskUnit('GB')
      if (instance.node_id) {
        apiClient.get('/network/eip-pools', { params: { node_id: instance.node_id } }).then(res => {
          setEipPools((res.data.data || []).filter((p: any) => p.status === 'active' && p.pool_type === 'eip'))
        }).catch(() => setEipPools([]))
      }
      if (instance.bridge_id) {
        apiClient.get('/network/bridge-ipv6-available', { params: { bridge_id: instance.bridge_id, prefix_len: eipv6PrefixLen, max_count: 10 } }).then(res => {
          setEipv6AddrList(res.data.addresses || [])
        }).catch(() => setEipv6AddrList([]))
      }
    }
  }, [instance])

  useEffect(() => {
    if (!eipv4PoolId) { setEipv4AddrList([]); return }
    apiClient.get('/network/eip-available-list', { params: { pool_id: eipv4PoolId, prefix_len: 32, max_count: 10 } }).then(res => {
      setEipv4AddrList(res.data.addresses || [])
    }).catch(() => setEipv4AddrList([]))
  }, [eipv4PoolId])

  useEffect(() => {
    if (!instance?.bridge_id) return
    apiClient.get('/network/bridge-ipv6-available', { params: { bridge_id: instance.bridge_id, prefix_len: eipv6PrefixLen, max_count: 10 } }).then(res => {
      setEipv6AddrList(res.data.addresses || [])
    }).catch(() => setEipv6AddrList([]))
  }, [eipv6PrefixLen, instance?.bridge_id])

  const handleSave = async () => {
    if (!instance) return
    setSaving(true)
    try {
      const payload: Record<string, any> = {
        vcpu,
        memory_mb: memoryMb,
        swap_mb: swapMb,
        network_down_mbps: networkDown,
        network_up_mbps: networkUp,
        monthly_traffic_gb: monthlyTraffic,
        over_limit_action: overLimitAction,
        throttle_mbps: throttleMbps,
        port_mapping_limit: portMappingLimit,
        snapshot_limit: snapshotLimit,
        io_read_iops: ioReadIops,
        io_write_iops: ioWriteIops,
      }
      if (expiresAt) {
        payload.expires_at = expiresAt
      }
      await apiClient.put(`/instances/${instance.id}`, payload)
      if (statusOverride !== instance.status) {
        await apiClient.post(`/instances/${instance.id}/status`, { status: statusOverride })
      }
      toast.success('实例配置已更新')
      onSuccess()
      onClose()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '更新失败')
    } finally {
      setSaving(false)
    }
  }

  const reloadInstance = async () => {
    const res = await apiClient.get(`/instances/${instance!.id}`)
    onInstanceUpdate(res.data as Instance)
  }

  const handleBindEIPv4 = async () => {
    if (!instance || !eipv4PoolId) return
    setEipLoading(true)
    try {
      const specificIP = eipv4SelectedIP || undefined
      const allocRes = await apiClient.post('/network/eip-allocations/allocate', {
        node_id: instance.node_id,
        ip_version: 'ipv4',
        prefix_len: 32,
        specific_ip: specificIP,
      })
      const allocId = allocRes.data.id
      await apiClient.post(`/network/eip-allocations/${allocId}/assign`, { instance_id: instance.id })
      toast.success('IPv4 EIP 绑定成功')
      setEipv4SelectedIP('')
      onSuccess()
      await reloadInstance()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '绑定失败')
    } finally {
      setEipLoading(false)
    }
  }

  const handleBindEIPv6 = async () => {
    if (!instance || !eipv6SelectedIP) return
    setEipLoading(true)
    try {
      const allocRes = await apiClient.post('/network/eip-allocations/allocate', {
        node_id: instance.node_id,
        ip_version: 'ipv6',
        prefix_len: eipv6PrefixLen,
        specific_ip: eipv6SelectedIP,
        bridge_id: instance.bridge_id,
      })
      const allocId = allocRes.data.id
      await apiClient.post(`/network/eip-allocations/${allocId}/assign`, { instance_id: instance.id })
      toast.success('IPv6 EIP 绑定成功')
      setEipv6SelectedIP('')
      onSuccess()
      await reloadInstance()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '绑定失败')
    } finally {
      setEipLoading(false)
    }
  }

  const handleUnbindEIP = async (allocId: string) => {
    if (!instance) return
    setEipLoading(true)
    try {
      await apiClient.post(`/network/eip-allocations/${allocId}/release`)
      toast.success('EIP 解绑成功')
      onSuccess()
      await reloadInstance()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '解绑失败')
    } finally {
      setEipLoading(false)
    }
  }

  const handleResizeSysDisk = async () => {
    const targetMb = sysDiskUnit === 'MB' ? sysDiskResize : sysDiskUnit === 'GB' ? sysDiskResize * 1024 : sysDiskResize * 1024 * 1024
    if (!instance || targetMb <= instance.disk_mb) return
    setDiskLoading(true)
    try {
      await apiClient.put(`/instances/${instance.id}`, { disk_mb: targetMb })
      toast.success('系统盘扩容任务已创建')
      onSuccess()
      await reloadInstance()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '扩容失败')
    } finally {
      setDiskLoading(false)
    }
  }

  const handleAddDisk = async () => {
    if (!instance || !newDiskName.trim()) return
    setDiskLoading(true)
    try {
      await apiClient.post(`/instances/${instance.id}/disks`, {
        name: newDiskName.trim(),
        size_mb: newDiskSize * 1024,
        storage_pool: newDiskPool || undefined,
        mount_point: newDiskMount || undefined,
      })
      toast.success('添加数据盘任务已创建')
      setNewDiskName('')
      setNewDiskSize(10)
      setNewDiskMount('')
      onSuccess()
      const res = await apiClient.get(`/instances/${instance.id}`)
      onInstanceUpdate(res.data as Instance)
    } catch (err: any) {
      toast.error(err.response?.data?.error || '添加失败')
    } finally {
      setDiskLoading(false)
    }
  }

  const handleDeleteDisk = async (diskId: string) => {
    if (!instance) return
    if (!confirm('确认删除该数据盘？')) return
    setDiskLoading(true)
    try {
      await apiClient.delete(`/instances/${instance.id}/disks/${diskId}`)
      toast.success('删除数据盘任务已创建')
      onSuccess()
      const res = await apiClient.get(`/instances/${instance.id}`)
      onInstanceUpdate(res.data as Instance)
    } catch (err: any) {
      toast.error(err.response?.data?.error || '删除失败')
    } finally {
      setDiskLoading(false)
    }
  }

  const handleResizeDisk = async () => {
    if (!instance || !resizeDiskId || resizeDiskSize <= 0) return
    setDiskLoading(true)
    try {
      await apiClient.put(`/instances/${instance.id}/disks/${resizeDiskId}`, { size_mb: resizeDiskSize * 1024 })
      toast.success('扩容任务已创建')
      setResizeDiskId('')
      setResizeDiskSize(0)
      onSuccess()
      const res = await apiClient.get(`/instances/${instance.id}`)
      onInstanceUpdate(res.data as Instance)
    } catch (err: any) {
      toast.error(err.response?.data?.error || '扩容失败')
    } finally {
      setDiskLoading(false)
    }
  }

  if (!instance) return null

  return (
    <SlidePanel
      open={open}
      onClose={onClose}
      title={`编辑实例 - ${instance.name}`}
      width={520}
      footer={
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>取消</Button>
          <Button onClick={handleSave} loading={saving}>保存</Button>
        </div>
      }
    >
      <div className="space-y-5">
        {/* 基本信息 */}
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">基本信息</h4>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">CPU (核)</label>
              <input type="number" min={0.1} step={0.1} value={vcpu} onChange={(e) => setVcpu(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" />
            </div>
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">内存 (MB)</label>
              <input type="number" min={64} value={memoryMb} onChange={(e) => setMemoryMb(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" />
            </div>
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">Swap (MB)</label>
              <input type="number" min={0} step={128} value={swapMb} onChange={(e) => setSwapMb(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" placeholder="0 = 不限" />
            </div>
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">到期时间</label>
              <input type="datetime-local" value={expiresAt} onChange={(e) => setExpiresAt(e.target.value)} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" />
            </div>
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">状态</label>
              <Select
                value={statusOverride}
                onChange={(v) => setStatusOverride(String(v))}
                options={[
                  { label: t('common.running'), value: 'running' },
                  { label: t('common.stopped'), value: 'stopped' },
                  { label: t('common.error'), value: 'error' },
                  { label: t('common.expired'), value: 'expired' },
                  { label: t('common.banned'), value: 'banned' },
                  { label: t('common.nodeOffline'), value: 'offline' },
                  { label: t('common.missing'), value: 'missing' },
                ]}
              />
            </div>
          </div>
        </div>

        {/* 网络限速 */}
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">网络限速</h4>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">下行限速 (Mbps)</label>
              <input type="number" min={0} value={networkDown} onChange={(e) => setNetworkDown(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" placeholder="0 = 不限" />
            </div>
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">上行限速 (Mbps)</label>
              <input type="number" min={0} value={networkUp} onChange={(e) => setNetworkUp(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" placeholder="0 = 不限" />
            </div>
          </div>
        </div>

        {/* IO 限制 */}
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">IO 限制</h4>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">读 IOPS</label>
              <input type="number" min={0} value={ioReadIops} onChange={(e) => setIoReadIops(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" placeholder="0 = 不限" />
            </div>
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">写 IOPS</label>
              <input type="number" min={0} value={ioWriteIops} onChange={(e) => setIoWriteIops(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" placeholder="0 = 不限" />
            </div>
          </div>
        </div>

        {/* 流量配额 */}
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">流量配额</h4>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">月度流量 (GB)</label>
              <input type="number" min={0} value={monthlyTraffic} onChange={(e) => setMonthlyTraffic(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" placeholder="0 = 不限" />
            </div>
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">超限策略</label>
              <Select
                value={overLimitAction}
                options={[{ label: '直接关机', value: 'shutdown' }, { label: '限速', value: 'throttle' }]}
                onChange={(v) => setOverLimitAction(v as string)}
              />
            </div>
            {overLimitAction === 'throttle' && (
              <div>
                <label className="block text-sm font-medium text-secondary mb-1">限速值 (Mbps)</label>
                <input type="number" min={1} value={throttleMbps} onChange={(e) => setThrottleMbps(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" />
              </div>
            )}
          </div>
        </div>

        {/* 限额配置 */}
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">限额配置</h4>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">端口映射限额</label>
              <input type="number" min={0} value={portMappingLimit} onChange={(e) => setPortMappingLimit(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" />
            </div>
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">快照限额</label>
              <input type="number" min={0} value={snapshotLimit} onChange={(e) => setSnapshotLimit(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" />
            </div>
          </div>
        </div>

        {/* 网卡与 EIP */}
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">网卡与 EIP</h4>
          <div className="space-y-3">
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-secondary mb-1">内网 IPv4</label>
                <div className="px-3 py-2 border border-surface rounded-lg text-sm text-tertiary bg-surface-secondary font-number">{instance.internal_ipv4 || '-'}</div>
              </div>
              <div>
                <label className="block text-sm font-medium text-secondary mb-1">内网 IPv6</label>
                <div className="px-3 py-2 border border-surface rounded-lg text-sm text-tertiary bg-surface-secondary font-number">{instance.internal_ipv6 || '-'}</div>
              </div>
              <div>
                <label className="block text-sm font-medium text-secondary mb-1">IPv4 模式</label>
                <div className="px-3 py-2 border border-surface rounded-lg text-sm text-tertiary bg-surface-secondary">{instance.ipv4_mode || 'nat'}</div>
              </div>
              <div>
                <label className="block text-sm font-medium text-secondary mb-1">IPv6 模式</label>
                <div className="px-3 py-2 border border-surface rounded-lg text-sm text-tertiary bg-surface-secondary">{instance.ipv6_mode || 'none'}</div>
              </div>
              <div>
                <label className="block text-sm font-medium text-secondary mb-1">网桥</label>
                <div className="px-3 py-2 border border-surface rounded-lg text-sm text-tertiary bg-surface-secondary">{instance.bridge_name || '-'}</div>
              </div>
              <div>
                <label className="block text-sm font-medium text-secondary mb-1">SSH 端口</label>
                <div className="px-3 py-2 border border-surface rounded-lg text-sm text-tertiary bg-surface-secondary font-number">{instance.ssh_port || '-'}</div>
              </div>
            </div>

            {/* EIP 管理区 */}
            <div className="border-t pt-3">
              <div className="flex items-center justify-between mb-2">
                <span className="text-sm font-medium text-secondary">EIP 绑定</span>
              </div>
              {/* 已绑定的 EIP */}
              <div className="space-y-2 mb-3">
                {instance.ipv4_eip && (
                  <div key="v4" className="flex items-center justify-between px-3 py-2 bg-surface-secondary rounded-lg">
                    <div className="flex items-center gap-2">
                      <span className="text-xs px-1.5 py-0.5 rounded bg-blue-100 text-blue-700">IPv4</span>
                      <span className="font-number text-sm">{instance.ipv4_eip}</span>
                    </div>
                    <button className="data-table-link-btn" onClick={() => instance.ipv4_eip_allocation_id && handleUnbindEIP(instance.ipv4_eip_allocation_id)} disabled={eipLoading} style={{ color: 'var(--color-red-500)' }}>解绑</button>
                  </div>
                )}
                {instance.ipv6_eip && (
                  <div key="v6" className="flex items-center justify-between px-3 py-2 bg-surface-secondary rounded-lg">
                    <div className="flex items-center gap-2">
                      <span className="text-xs px-1.5 py-0.5 rounded bg-blue-100 text-blue-700">IPv6</span>
                      <span className="font-number text-sm">{instance.ipv6_eip}</span>
                    </div>
                    <button className="data-table-link-btn" onClick={() => instance.ipv6_eip_allocation_id && handleUnbindEIP(instance.ipv6_eip_allocation_id)} disabled={eipLoading} style={{ color: 'var(--color-red-500)' }}>解绑</button>
                  </div>
                )}
                {!instance.ipv4_eip && !instance.ipv6_eip && (
                  <div className="text-sm text-muted px-3 py-2">暂无绑定的 EIP</div>
                )}
              </div>

              {/* 绑定 IPv4 EIP */}
              {!instance.ipv4_eip && (
                <div className="space-y-2 mb-3 p-3 border border-surface-light rounded-lg">
                  <div className="text-xs font-medium text-tertiary">绑定 IPv4 EIP</div>
                  <Select
                    value={eipv4PoolId}
                    placeholder="选择 IPv4 EIP 池"
                    options={eipPools.filter(p => p.ip_version === 'ipv4').map(p => ({ label: `${p.cidr} (${p.interface || '无网卡'})`, value: p.id }))}
                    onChange={(v) => { setEipv4PoolId(String(v)); setEipv4SelectedIP('') }}
                  />
                  {eipv4PoolId && eipv4AddrList.length > 0 && (
                    <div>
                      <label className="block text-xs text-tertiary mb-1">选择地址（留空自动分配）</label>
                      <Select
                        value={eipv4SelectedIP}
                        placeholder="自动分配"
                        options={[{ label: '自动分配', value: '' }, ...eipv4AddrList.map(addr => ({ label: addr.split('/')[0], value: addr }))]}
                        onChange={(v) => setEipv4SelectedIP(v as string)}
                      />
                    </div>
                  )}
                  <button className="data-table-link-btn" onClick={handleBindEIPv4} disabled={eipLoading || !eipv4PoolId}>绑定 IPv4 EIP</button>
                </div>
              )}

              {/* 绑定 IPv6 EIP */}
              {!instance.ipv6_eip && instance.bridge_id && (
                <div className="space-y-2 p-3 border border-surface-light rounded-lg">
                  <div className="text-xs font-medium text-tertiary">绑定 IPv6 EIP</div>
                  <div>
                    <label className="block text-xs text-tertiary mb-1">前缀长度</label>
                    <input type="number" min={64} max={128} value={eipv6PrefixLen} onChange={(e) => setEipv6PrefixLen(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" />
                  </div>
                  {eipv6AddrList.length > 0 && (
                    <div>
                      <label className="block text-xs text-tertiary mb-1">选择子段</label>
                      <Select
                        value={eipv6SelectedIP}
                        placeholder="选择 IPv6 子段"
                        options={eipv6AddrList.map(addr => ({ label: addr, value: addr }))}
                        onChange={(v) => setEipv6SelectedIP(v as string)}
                      />
                    </div>
                  )}
                  {eipv6AddrList.length === 0 && (
                    <div className="text-xs text-muted">无可用 IPv6 子段</div>
                  )}
                  <button className="data-table-link-btn" onClick={handleBindEIPv6} disabled={eipLoading || !eipv6SelectedIP}>绑定 IPv6 EIP</button>
                </div>
              )}
            </div>
          </div>
        </div>

        {/* 磁盘管理 */}
        <div>
          <h4 className="text-xs font-semibold text-tertiary uppercase mb-3">磁盘管理</h4>
          <div className="space-y-3">
            {/* 系统盘（可扩容） */}
            <div className="flex items-center justify-between px-3 py-2 bg-surface-secondary rounded-lg">
              <div className="flex items-center gap-2">
                <span className="text-xs px-1.5 py-0.5 rounded bg-surface-secondary">系统盘</span>
                <span className="font-number text-sm">{(instance.disk_mb / 1024).toFixed(0)} GB</span>
              </div>
              <div className="flex items-center gap-2">
                <input type="number" min={1} value={sysDiskResize} onChange={(e) => setSysDiskResize(Number(e.target.value))} className="w-20 px-2 py-1 border border-surface-strong rounded text-sm font-number" />
                <select value={sysDiskUnit} onChange={(e) => setSysDiskUnit(e.target.value as 'MB' | 'GB' | 'TB')} className="px-1 py-1 border border-surface-strong rounded text-xs bg-surface">
                  <option value="MB">MB</option>
                  <option value="GB">GB</option>
                  <option value="TB">TB</option>
                </select>
                <button className="data-table-link-btn" onClick={handleResizeSysDisk} disabled={diskLoading || (sysDiskUnit === 'MB' ? sysDiskResize : sysDiskUnit === 'GB' ? sysDiskResize * 1024 : sysDiskResize * 1024 * 1024) <= instance.disk_mb}>扩容</button>
              </div>
            </div>
            {/* 数据盘列表 */}
            {instance.data_disks && instance.data_disks.length > 0 && (
              <div className="space-y-2">
                {instance.data_disks.map(disk => (
                  <div key={disk.id} className="flex items-center justify-between px-3 py-2 border border-surface-light rounded-lg">
                    <div className="flex items-center gap-2">
                      <span className="text-xs px-1.5 py-0.5 rounded bg-surface-secondary">数据盘</span>
                      <span className="text-sm">{disk.name}</span>
                      <span className="font-number text-sm text-tertiary">{(disk.size_mb / 1024).toFixed(0)} GB</span>
                      {disk.mount_point && <span className="text-xs text-muted">{disk.mount_point}</span>}
                    </div>
                    <div className="flex items-center gap-2">
                      <button className="data-table-link-btn" onClick={() => { setResizeDiskId(disk.id); setResizeDiskSize(Math.round(disk.size_mb / 1024)) }}>扩容</button>
                      <button className="data-table-link-btn" onClick={() => handleDeleteDisk(disk.id)} disabled={diskLoading} style={{ color: 'var(--color-red-500)' }}>删除</button>
                    </div>
                  </div>
                ))}
              </div>
            )}
            {/* 扩容输入区 */}
            {resizeDiskId && (
              <div className="flex items-center gap-2 px-3 py-2 border border-blue-200 rounded-lg bg-blue-50">
                <span className="text-sm text-tertiary">扩容至</span>
                <input type="number" min={1} value={resizeDiskSize} onChange={(e) => setResizeDiskSize(Number(e.target.value))} className="w-24 px-2 py-1 border border-surface-strong rounded text-sm font-number" />
                <span className="text-sm text-tertiary">GB</span>
                <button className="data-table-link-btn" onClick={handleResizeDisk} disabled={diskLoading}>确认</button>
                <button className="data-table-link-btn" onClick={() => { setResizeDiskId(''); setResizeDiskSize(0) }}>取消</button>
              </div>
            )}
            {/* 添加数据盘 */}
            <div className="border-t pt-3 space-y-2">
              <div className="text-xs text-tertiary mb-1">添加数据盘</div>
              <div className="grid grid-cols-2 gap-3">
                <input type="text" value={newDiskName} onChange={(e) => setNewDiskName(e.target.value)} className="px-3 py-2 border border-surface-strong rounded-lg text-sm" placeholder="磁盘名称" />
                <input type="number" min={1} value={newDiskSize} onChange={(e) => setNewDiskSize(Number(e.target.value))} className="px-3 py-2 border border-surface-strong rounded-lg text-sm font-number" placeholder="大小 GB" />
                <input type="text" value={newDiskPool} onChange={(e) => setNewDiskPool(e.target.value)} className="px-3 py-2 border border-surface-strong rounded-lg text-sm" placeholder="存储池 (默认 default)" />
                <input type="text" value={newDiskMount} onChange={(e) => setNewDiskMount(e.target.value)} className="px-3 py-2 border border-surface-strong rounded-lg text-sm" placeholder="挂载点 (可选)" />
              </div>
              <button className="data-table-link-btn" onClick={handleAddDisk} disabled={diskLoading || !newDiskName.trim()}>+ 添加数据盘</button>
            </div>
          </div>
        </div>
      </div>
    </SlidePanel>
  )
}
