import { useEffect, useState } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import apiClient from '@/api/client'
import { useToastStore } from '@/stores/toast'
import { SlidePanel } from '@/components/SlidePanel/SlidePanel'
import { Button } from '@/components/Button/Button'
import { Select } from '@/components/Select/Select'

interface Node { id: string; name: string; status: string }
interface StoragePool { name: string; driver: string; size: number; used: number }
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
interface Category { id: string; name: string; image_type: string; sort_order: number }
interface User { id: number; username: string }
interface Bridge { id: string; name: string; node_id: string; ipv4_cidr: string; ipv4_gateway: string; bridge_name: string }
interface EIPPoolInfo { id: string; cidr: string; ip_version: string; interface: string; pool_type: string; status: string }

interface Props {
  open: boolean
  onClose: () => void
  onSuccess: () => void
}

export default function CreateInstanceModal({ open, onClose, onSuccess }: Props) {
  const toast = useToastStore()
  const [step, setStep] = useState(1)
  const [loading, setLoading] = useState(false)

  const [nodes, setNodes] = useState<Node[]>([])
  const [installedImages, setInstalledImages] = useState<InstalledImage[]>([])
  const [categories, setCategories] = useState<Category[]>([])
  const [selectedCategory, setSelectedCategory] = useState('')
  const [storages, setStorages] = useState<StoragePool[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [bridges, setBridges] = useState<Bridge[]>([])

  const [name, setName] = useState('')
  const [type, setType] = useState<'container' | 'vm'>('container')
  const [nodeId, setNodeId] = useState('')
  const [templateId, setTemplateId] = useState('')
  const [assignToUserId, setAssignToUserId] = useState<number | ''>('')
  const [bridgeId, setBridgeId] = useState('')

  const [vcpu, setVcpu] = useState(1)
  const [memoryMb, setMemoryMb] = useState(512)
  const [swapMb, setSwapMb] = useState(0)
  const [diskMb, setDiskMb] = useState(5120)
  const [diskUnit, setDiskUnit] = useState<'MB' | 'GB' | 'TB'>('GB')
  const [storagePool, setStoragePool] = useState('')

  const [assignNat, setAssignNat] = useState(true)
  const [portMappingCount, setPortMappingCount] = useState(2)
  const [assignEipv4, setAssignEipv4] = useState(false)
  const [assignEipv6, setAssignEipv6] = useState(false)
  const [eipv4Count, setEipv4Count] = useState(1)
  const [eipv6Count, setEipv6Count] = useState(1)
  const [eipv6PrefixLen, setEipv6PrefixLen] = useState(128)
  const [eipv4Mode, setEipv4Mode] = useState<'auto' | 'manual'>('auto')
  const [eipv6Mode, setEipv6Mode] = useState<'auto' | 'manual'>('auto')
  const [eipv4SpecificIP, setEipv4SpecificIP] = useState('')
  const [eipv6SpecificIP, setEipv6SpecificIP] = useState('')
  const [eipv4Available, setEipv4Available] = useState<number | null>(null)
  const [eipv6Available, setEipv6Available] = useState<number | null>(null)
  const [eipPools, setEipPools] = useState<EIPPoolInfo[]>([])
  const [eipv4PoolId, setEipv4PoolId] = useState('')
  const [eipv4AddrList, setEipv4AddrList] = useState<string[]>([])
  const [eipv6AddrList, setEipv6AddrList] = useState<string[]>([])

  const [loginMethod, setLoginMethod] = useState<'auto' | 'password' | 'sshkey'>('auto')
  const [sshPassword, setSshPassword] = useState('')
  const [sshPublicKey, setSshPublicKey] = useState('')

  const [networkDown, setNetworkDown] = useState(0)
  const [networkUp, setNetworkUp] = useState(0)
  const [ioRead, setIoRead] = useState(0)
  const [ioWrite, setIoWrite] = useState(0)
  const [monthlyTraffic, setMonthlyTraffic] = useState(0)
  const [trafficMode, setTrafficMode] = useState<'total' | 'in' | 'out' | 'in_out'>('total')
  const [overLimitAction, setOverLimitAction] = useState<'shutdown' | 'throttle'>('shutdown')
  const [throttleMbps, setThrottleMbps] = useState(1)
  const [snapshotLimit, setSnapshotLimit] = useState(3)

  const [dataDisks, setDataDisks] = useState<{name: string; size_mb: number; storage_pool: string; mount_point: string}[]>([])

  useEffect(() => {
    if (!open) return
    apiClient.get('/nodes').then((r) => setNodes(r.data.data || []))
    apiClient.get('/users').then((r) => setUsers(r.data.data || []))
    setStep(1)
    resetForm()
  }, [open])

  useEffect(() => {
    if (!open || !nodeId) {
      setInstalledImages([])
      setCategories([])
      return
    }
    const imageType = type === 'vm' ? 'virtual-machine' : 'container'
    // 获取已安装镜像
    apiClient.get('/images/installed', { params: { node_id: nodeId, type: imageType } }).then((r) => {
      setInstalledImages(r.data.data || [])
    }).catch(() => setInstalledImages([]))
    // 获取分类
    apiClient.get('/images/categories', { params: { node_id: nodeId, type: imageType } }).then((r) => {
      setCategories(r.data.data || [])
    }).catch(() => setCategories([]))
  }, [open, nodeId, type])

  useEffect(() => {
    if (!nodeId) { setStorages([]); setBridges([]); setBridgeId(''); return }
    apiClient.get(`/nodes/${nodeId}/storages`).then((r) => {
      const list = r.data.data || []
      setStorages(list)
      if (list.length > 0) setStoragePool(list[0].name)
    }).catch(() => setStorages([]))
    // 加载节点 Bridge 列表
    apiClient.get('/network/bridges', { params: { node_id: nodeId } }).then((r) => {
      const list = r.data.data || []
      setBridges(list)
      if (list.length > 0) setBridgeId(list[0].id)
    }).catch(() => { setBridges([]); setBridgeId('') })
    // 查询可用 EIP 数量
    apiClient.get('/network/eip-available', { params: { node_id: nodeId, ip_version: 'ipv4' } }).then((r) => {
      setEipv4Available(r.data.available ?? 0)
    }).catch(() => setEipv4Available(0))
    // 加载 EIP 池列表
    apiClient.get('/network/eip-pools', { params: { node_id: nodeId } }).then((r) => {
      setEipPools((r.data.data || []).filter((p: EIPPoolInfo) => p.status === 'active' && p.pool_type === 'eip'))
    }).catch(() => setEipPools([]))
  }, [nodeId])

  // IPv6 前缀长度或 bridge 变化时重新查询可用子段
  useEffect(() => {
    if (!bridgeId || !assignEipv6) { setEipv6AddrList([]); setEipv6Available(null); return }
    apiClient.get('/network/bridge-ipv6-available', { params: { bridge_id: bridgeId, prefix_len: eipv6PrefixLen, max_count: 100 } }).then((r) => {
      const list = r.data.addresses || []
      setEipv6AddrList(list)
      setEipv6Available(list.length)
    }).catch(() => { setEipv6AddrList([]); setEipv6Available(0) })
  }, [bridgeId, eipv6PrefixLen, assignEipv6])

  // IPv4 池+前缀变化时查询可用地址列表
  useEffect(() => {
    if (!eipv4PoolId) { setEipv4AddrList([]); return }
    apiClient.get('/network/eip-available-list', { params: { pool_id: eipv4PoolId, prefix_len: 32, max_count: 10 } }).then((r) => {
      setEipv4AddrList(r.data.addresses || [])
    }).catch(() => setEipv4AddrList([]))
  }, [eipv4PoolId])

  // IPv6 池+前缀变化时查询可用地址列表（已移除，改为从 bridge 查询）

  const resetForm = () => {
    setName('')
    setType('container')
    setNodeId('')
    setTemplateId('')
    setSelectedCategory('')
    setAssignToUserId('')
    setBridgeId('')
    setVcpu(1)
    setMemoryMb(512)
    setSwapMb(0)
    setDiskMb(5120)
    setDiskUnit('GB')
    setStoragePool('')
    setAssignNat(true)
    setPortMappingCount(2)
    setAssignEipv4(false)
    setAssignEipv6(false)
    setEipv4Count(1)
    setEipv6Count(1)
    setEipv6PrefixLen(128)
    setEipv4Mode('auto')
    setEipv6Mode('auto')
    setEipv4SpecificIP('')
    setEipv6SpecificIP('')
    setEipv4Available(null)
    setEipv6Available(null)
    setEipPools([])
    setEipv4PoolId('')
    setEipv4AddrList([])
    setEipv6AddrList([])
    setLoginMethod('auto')
    setSshPassword('')
    setSshPublicKey('')
    setNetworkDown(0)
    setNetworkUp(0)
    setIoRead(0)
    setIoWrite(0)
    setMonthlyTraffic(0)
    setTrafficMode('total')
    setOverLimitAction('shutdown')
    setThrottleMbps(1)
    setSnapshotLimit(3)
    setDataDisks([])
  }

  // 按分类分组，两级选择
  const categoryOptions = [
    { label: '全部分类', value: '' },
    ...categories.map((c) => ({ label: c.name, value: c.id }))
  ]
  const filteredImages = selectedCategory
    ? installedImages.filter((img) => img.category_id === selectedCategory)
    : installedImages

  const handleSubmit = async () => {
    if (!name || !nodeId || !templateId || assignToUserId === '') {
      toast.error('请填写所有必填字段')
      return
    }
    setLoading(true)
    try {
      // 检查模板是否已下载，未下载则自动触发并等待
      // 后端已过滤仅已下载镜像，无需检查下载状态
      const selectedTemplate = installedImages.find((t) => t.id === templateId)

      const payload: Record<string, unknown> = {
        name,
        type,
        template_id: selectedTemplate?.alias || templateId,
        image_key: templateId,
        node_id: nodeId,
        assign_to_user_id: assignToUserId,
        bridge_id: bridgeId || undefined,
        login_method: loginMethod,
        vcpu,
        memory_mb: memoryMb,
        swap_mb: swapMb || undefined,
        disk_mb: diskUnit === 'MB' ? diskMb : diskUnit === 'GB' ? diskMb * 1024 : diskMb * 1024 * 1024,
        storage_pool: storagePool || undefined,
        assign_eip_ipv4: assignEipv4,
        assign_eip_ipv6: assignEipv6,
        eip_ipv4_count: assignEipv4 ? eipv4Count : undefined,
        eip_ipv6_count: assignEipv6 ? eipv6Count : undefined,
        eip_ipv6_prefix_len: assignEipv6 ? eipv6PrefixLen : undefined,
        eip_ipv4_specific_ip: assignEipv4 && eipv4Mode === 'manual' ? eipv4SpecificIP : undefined,
        eip_ipv6_specific_ip: assignEipv6 && eipv6Mode === 'manual' ? eipv6SpecificIP : undefined,
        eip_ipv4_pool_id: assignEipv4 ? eipv4PoolId || undefined : undefined,
        port_mapping_count: assignNat ? portMappingCount : 0,
        ssh_password: loginMethod === 'password' ? sshPassword : undefined,
        ssh_public_key: loginMethod === 'sshkey' ? sshPublicKey : undefined,
        network_down_mbps: networkDown || undefined,
        network_up_mbps: networkUp || undefined,
        io_read_iops: ioRead || undefined,
        io_write_iops: ioWrite || undefined,
        monthly_traffic_gb: monthlyTraffic || undefined,
        traffic_mode: trafficMode,
        over_limit_action: overLimitAction,
        throttle_mbps: overLimitAction === 'throttle' ? (throttleMbps || 1) : undefined,
        snapshot_limit: snapshotLimit || undefined,
        data_disks: dataDisks.length > 0 ? dataDisks : undefined,
      }
      await apiClient.post('/instances', payload)
      toast.success('创建实例任务已下发')
      onSuccess()
      onClose()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '创建失败')
    } finally {
      setLoading(false)
    }
  }

  const addDataDisk = () => setDataDisks([...dataDisks, { name: `disk${dataDisks.length + 1}`, size_mb: 10240, storage_pool: storagePool || '', mount_point: `/mnt/disk${dataDisks.length + 1}` }])
  const updateDataDisk = (idx: number, field: 'name' | 'size_mb' | 'storage_pool' | 'mount_point', val: string | number) => {
    const next = [...dataDisks]
    next[idx] = { ...next[idx], [field]: val }
    setDataDisks(next)
  }
  const removeDataDisk = (idx: number) => setDataDisks(dataDisks.filter((_, i) => i !== idx))

  if (!open) return null

  return (
    <SlidePanel
      open={open}
      onClose={onClose}
      title="创建实例"
      width={560}
      footer={
        <div className="flex items-center justify-between w-full">
          <div className="flex gap-2">
            {[1, 2, 3].map((s) => (
              <div key={s} className={`w-2 h-2 rounded-full ${s === step ? 'bg-primary' : 'bg-surface-strong'}`} />
            ))}
          </div>
          <div className="flex gap-2">
            {step > 1 && (
              <Button variant="ghost" onClick={() => setStep(step - 1)}>上一步</Button>
            )}
            {step < 3 ? (
              <Button onClick={() => setStep(step + 1)}>下一步</Button>
            ) : (
              <Button onClick={handleSubmit} loading={loading} disabled={loading}>
                {loading ? '创建中...' : '创建实例'}
              </Button>
            )}
          </div>
        </div>
      }
    >
      {/* Step 1: 基本信息 */}
      {step === 1 && (
        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">名称 <span className="text-red-500">*</span></label>
              <input value={name} onChange={(e) => setName(e.target.value)} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm focus:outline-none focus:ring-1 focus:ring-black" placeholder="实例名称" />
            </div>
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">类型</label>
              <Select
                value={type}
                options={[{ label: '容器', value: 'container' }, { label: '虚拟机', value: 'vm' }]}
                onChange={(v) => { setType(v as 'container' | 'vm'); setTemplateId('') }}
              />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">节点 <span className="text-red-500">*</span></label>
              <Select
                value={nodeId}
                placeholder="选择节点"
                options={nodes.map((n) => ({ label: `${n.name} (${n.status})`, value: n.id }))}
                onChange={(v) => setNodeId(String(v))}
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">分配给用户 <span className="text-red-500">*</span></label>
              <Select
                value={assignToUserId}
                placeholder="选择用户"
                options={users.map((u) => ({ label: u.username, value: u.id }))}
                onChange={(v) => setAssignToUserId(Number(v))}
              />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">分类 <span className="text-red-500">*</span></label>
              <Select
                value={selectedCategory}
                placeholder="选择分类"
                options={categoryOptions}
                onChange={(v) => { setSelectedCategory(String(v)); setTemplateId('') }}
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">镜像版本 <span className="text-red-500">*</span></label>
              <Select
                value={templateId}
                placeholder="选择镜像"
                options={filteredImages.map((img) => ({ label: `${img.display_name || img.alias} (${img.architecture})`, value: img.id }))}
                onChange={(v) => setTemplateId(String(v))}
              />
            </div>
          </div>
          <div className="grid grid-cols-3 gap-4">
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">CPU</label>
              <input type="number" min={0.1} step={0.1} value={vcpu} onChange={(e) => setVcpu(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" />
            </div>
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">内存 (MB)</label>
              <input type="number" min={64} step={64} value={memoryMb} onChange={(e) => setMemoryMb(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" />
            </div>
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">Swap (MB)</label>
              <input type="number" min={0} step={128} value={swapMb} onChange={(e) => setSwapMb(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" placeholder="0 = 不限" />
            </div>
          </div>
          <div>
            <label className="block text-sm font-medium text-secondary mb-1">磁盘</label>
            <div className="flex gap-2">
              <input type="number" min={1} value={diskMb} onChange={(e) => setDiskMb(Number(e.target.value))} className="flex-1 px-3 py-2 border border-surface-strong rounded-lg text-sm" />
              <select value={diskUnit} onChange={(e) => setDiskUnit(e.target.value as 'MB' | 'GB' | 'TB')} className="px-3 py-2 border border-surface-strong rounded-lg text-sm bg-surface">
                <option value="MB">MB</option>
                <option value="GB">GB</option>
                <option value="TB">TB</option>
              </select>
            </div>
          </div>
          <div>
            <label className="block text-sm font-medium text-secondary mb-1">存储池</label>
            <Select
              value={storagePool}
              placeholder="默认: default"
              options={storages.map((s) => ({ label: `${s.name} (${s.driver})`, value: s.name }))}
              onChange={(v) => setStoragePool(String(v))}
            />
          </div>
        </div>
      )}

      {/* Step 2: 网络和SSH */}
      {step === 2 && (
        <div className="space-y-4">
          {/* Bridge 选择 */}
          <div>
            <label className="block text-sm font-medium text-secondary mb-1">所属 Bridge 网络</label>
            <Select
              value={bridgeId}
              placeholder={bridges.length === 0 ? '该节点暂无 Bridge，请先创建' : '选择 Bridge'}
              options={bridges.map((b) => ({ label: `${b.name} (${b.ipv4_cidr})`, value: b.id }))}
              onChange={(v) => setBridgeId(String(v))}
            />
            {bridgeId && (
              <p className="text-xs text-tertiary mt-1">
                网关: {bridges.find((b) => b.id === bridgeId)?.ipv4_gateway} | Bridge: {bridges.find((b) => b.id === bridgeId)?.bridge_name}
              </p>
            )}
          </div>

          <div className="flex items-center gap-2">
            <input type="checkbox" id="nat" checked={assignNat} onChange={(e) => setAssignNat(e.target.checked)} className="w-4 h-4" />
            <label htmlFor="nat" className="text-sm text-secondary">自动分配端口映射</label>
          </div>
          {assignNat && (
            <div className="pl-6">
              <label className="block text-sm font-medium text-secondary mb-1">端口映射配额（含 SSH）</label>
              <input type="number" min={1} max={64} value={portMappingCount} onChange={(e) => setPortMappingCount(Math.max(1, Math.min(64, Number(e.target.value))))} className="w-32 px-3 py-2 border border-surface-strong rounded-lg text-sm" />
              <p className="text-xs text-muted mt-1">自动分配 SSH (22)，其余由系统分配</p>
            </div>
          )}

          <div className="border-t border-surface-light pt-4 space-y-4">
            {/* EIP IPv4 */}
            <div className="border border-surface rounded-lg p-3">
              <div className="flex items-center gap-2 mb-2">
                <input type="checkbox" id="eipv4" checked={assignEipv4} onChange={(e) => setAssignEipv4(e.target.checked)} className="w-4 h-4" />
                <label htmlFor="eipv4" className="text-sm font-medium text-secondary">公网 IPv4</label>
                {eipv4Available !== null && (
                  <span className={`text-xs ${eipv4Available > 0 ? 'text-green-600' : 'text-red-500'}`}>
                    可用 {eipv4Available} 个
                  </span>
                )}
              </div>
              {assignEipv4 && (
                <div className="pl-6 space-y-2">
                  <div>
                    <label className="block text-xs text-tertiary mb-1">EIP 池</label>
                    <Select
                      value={eipv4PoolId}
                      placeholder="选择 IPv4 EIP 池"
                      options={eipPools.filter(p => p.ip_version === 'ipv4').map(p => ({ label: `${p.cidr} (${p.interface || '无网卡'})`, value: p.id }))}
                      onChange={(v) => { setEipv4PoolId(String(v)); setEipv4SpecificIP('') }}
                    />
                  </div>
                  <div className="flex gap-4">
                    <label className="flex items-center gap-1.5 text-sm">
                      <input type="radio" name="eipv4mode" checked={eipv4Mode === 'auto'} onChange={() => setEipv4Mode('auto')} />
                      自动分配
                    </label>
                    <label className="flex items-center gap-1.5 text-sm">
                      <input type="radio" name="eipv4mode" checked={eipv4Mode === 'manual'} onChange={() => setEipv4Mode('manual')} disabled={!eipv4PoolId} />
                      手动指定
                    </label>
                  </div>
                  {eipv4Mode === 'auto' && (
                    <div>
                      <label className="block text-xs text-tertiary mb-1">数量</label>
                      <input type="number" min={1} max={10} value={eipv4Count} onChange={(e) => setEipv4Count(Math.max(1, Math.min(10, Number(e.target.value))))} className="w-32 px-3 py-2 border border-surface-strong rounded-lg text-sm" />
                    </div>
                  )}
                  {eipv4Mode === 'manual' && eipv4PoolId && (
                    <div>
                      <label className="block text-xs text-tertiary mb-1">选择地址（/32）</label>
                      {eipv4AddrList.length > 0 ? (
                        <Select
                          value={eipv4SpecificIP}
                          options={eipv4AddrList.map(addr => ({ label: addr, value: addr.split('/')[0] }))}
                          placeholder="选择可用 IP"
                          onChange={(v) => setEipv4SpecificIP(String(v))}
                        />
                      ) : (
                        <p className="text-xs text-muted">该池无可用 IP</p>
                      )}
                    </div>
                  )}
                </div>
              )}
            </div>

            {/* EIP IPv6 */}
            <div className="border border-surface rounded-lg p-3">
              <div className="flex items-center gap-2 mb-2">
                <input type="checkbox" id="eipv6" checked={assignEipv6} onChange={(e) => setAssignEipv6(e.target.checked)} className="w-4 h-4" />
                <label htmlFor="eipv6" className="text-sm font-medium text-secondary">公网 IPv6</label>
                {eipv6Available !== null && (
                  <span className={`text-xs ${eipv6Available > 0 ? 'text-green-600' : 'text-red-500'}`}>
                    可用 {eipv6Available} 个
                  </span>
                )}
              </div>
              {assignEipv6 && (
                <div className="pl-6 space-y-2">
                  <p className="text-xs text-muted">IPv6 地址从所选 Bridge 的 IPv6 CIDR 中分配</p>
                  <div>
                    <label className="block text-xs text-tertiary mb-1">前缀长度</label>
                    <input type="number" min={64} max={128} value={eipv6PrefixLen} onChange={(e) => setEipv6PrefixLen(Math.max(64, Math.min(128, Number(e.target.value))))} className="w-32 px-3 py-2 border border-surface-strong rounded-lg text-sm" />
                  </div>
                  <div className="flex gap-4">
                    <label className="flex items-center gap-1.5 text-sm">
                      <input type="radio" name="eipv6mode" checked={eipv6Mode === 'auto'} onChange={() => setEipv6Mode('auto')} />
                      自动分配
                    </label>
                    <label className="flex items-center gap-1.5 text-sm">
                      <input type="radio" name="eipv6mode" checked={eipv6Mode === 'manual'} onChange={() => setEipv6Mode('manual')} disabled={!bridgeId} />
                      手动指定
                    </label>
                  </div>
                  {eipv6Mode === 'auto' && (
                    <div>
                      <label className="block text-xs text-tertiary mb-1">数量</label>
                      <input type="number" min={1} max={10} value={eipv6Count} onChange={(e) => setEipv6Count(Math.max(1, Math.min(10, Number(e.target.value))))} className="w-32 px-3 py-2 border border-surface-strong rounded-lg text-sm" />
                    </div>
                  )}
                  {eipv6Mode === 'manual' && bridgeId && (
                    <div>
                      <label className="block text-xs text-tertiary mb-1">选择地址（/{eipv6PrefixLen}）</label>
                      {eipv6AddrList.length > 0 ? (
                        <Select
                          value={eipv6SpecificIP}
                          options={eipv6AddrList.map(addr => ({ label: addr, value: addr.split('/')[0] }))}
                          placeholder="选择可用地址"
                          onChange={(v) => setEipv6SpecificIP(String(v))}
                        />
                      ) : (
                        <p className="text-xs text-muted">该 Bridge 无可用 IPv6 地址</p>
                      )}
                    </div>
                  )}
                </div>
              )}
            </div>
          </div>

          <div className="border-t border-surface-light pt-4">
            <label className="block text-sm font-medium text-secondary mb-2">SSH 登录方式</label>
            <div className="flex gap-4 mb-3">
              <label className="flex items-center gap-1.5 text-sm">
                <input type="radio" name="login" checked={loginMethod === 'auto'} onChange={() => setLoginMethod('auto')} />
                自动密码
              </label>
              <label className="flex items-center gap-1.5 text-sm">
                <input type="radio" name="login" checked={loginMethod === 'password'} onChange={() => setLoginMethod('password')} />
                自定义密码
              </label>
              <label className="flex items-center gap-1.5 text-sm">
                <input type="radio" name="login" checked={loginMethod === 'sshkey'} onChange={() => setLoginMethod('sshkey')} />
                SSH 公钥
              </label>
            </div>
            {loginMethod === 'password' && (
              <input type="text" value={sshPassword} onChange={(e) => setSshPassword(e.target.value)} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" placeholder="输入 root 密码" />
            )}
            {loginMethod === 'sshkey' && (
              <textarea value={sshPublicKey} onChange={(e) => setSshPublicKey(e.target.value)} rows={3} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" placeholder="粘贴 SSH 公钥 (ssh-rsa / ssh-ed25519 ...)" />
            )}
          </div>
        </div>
      )}

      {/* Step 3: 资源限制 */}
      {step === 3 && (
        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">出站带宽限制 (Mbps)</label>
              <input type="number" min={0} value={networkDown} onChange={(e) => setNetworkDown(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" placeholder="0 = 不限" />
            </div>
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">入站带宽限制 (Mbps)</label>
              <input type="number" min={0} value={networkUp} onChange={(e) => setNetworkUp(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" placeholder="0 = 不限" />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">磁盘读取限制 (IOPS)</label>
              <input type="number" min={0} value={ioRead} onChange={(e) => setIoRead(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" placeholder="0 = 不限" />
            </div>
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">磁盘写入限制 (IOPS)</label>
              <input type="number" min={0} value={ioWrite} onChange={(e) => setIoWrite(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" placeholder="0 = 不限" />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">月度流量限制 (GB)</label>
              <input type="number" min={0} value={monthlyTraffic} onChange={(e) => setMonthlyTraffic(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" placeholder="0 = 不限" />
            </div>
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">流量计算方式</label>
              <Select
                value={trafficMode}
                options={[{ label: '出站+入站合计', value: 'total' }, { label: '仅入站', value: 'in' }, { label: '仅出站', value: 'out' }, { label: '出入取较大值', value: 'in_out' }]}
                onChange={(v) => setTrafficMode(v as 'total' | 'in' | 'out' | 'in_out')}
              />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-secondary mb-1">流量超限策略</label>
              <Select
                value={overLimitAction}
                options={[{ label: '直接关机', value: 'shutdown' }, { label: '限速', value: 'throttle' }]}
                onChange={(v) => setOverLimitAction(v as 'shutdown' | 'throttle')}
              />
            </div>
            {overLimitAction === 'throttle' && (
              <div>
                <label className="block text-sm font-medium text-secondary mb-1">限速值 (Mbps)</label>
                <input type="number" min={1} value={throttleMbps} onChange={(e) => setThrottleMbps(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" placeholder="上下行均限速到此值" />
              </div>
            )}
          </div>
          <div>
            <label className="block text-sm font-medium text-secondary mb-1">快照数量限制</label>
            <input type="number" min={0} value={snapshotLimit} onChange={(e) => setSnapshotLimit(Number(e.target.value))} className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm" />
          </div>

          <div className="border-t border-surface-light pt-4">
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm font-medium text-secondary">数据盘</span>
              <button onClick={addDataDisk} className="flex items-center gap-1 text-xs text-primary hover:text-secondary">
                <Plus size={12} /> 添加数据盘
              </button>
            </div>
            {dataDisks.map((disk, idx) => (
              <div key={idx} className="grid grid-cols-4 gap-2 mb-2 items-center">
                <input value={disk.name} onChange={(e) => updateDataDisk(idx, 'name', e.target.value)} className="px-2 py-1 border border-surface-strong rounded text-sm" placeholder="名称" />
                <input type="number" min={1} value={disk.size_mb} onChange={(e) => updateDataDisk(idx, 'size_mb', Number(e.target.value))} className="px-2 py-1 border border-surface-strong rounded text-sm" placeholder="MB" />
                <input value={disk.mount_point} onChange={(e) => updateDataDisk(idx, 'mount_point', e.target.value)} className="px-2 py-1 border border-surface-strong rounded text-sm" placeholder="挂载点" />
                <button onClick={() => removeDataDisk(idx)} className="text-red-500 hover:text-red-700 justify-self-start">
                  <Trash2 size={14} />
                </button>
              </div>
            ))}
          </div>
        </div>
      )}
    </SlidePanel>
  )
}
