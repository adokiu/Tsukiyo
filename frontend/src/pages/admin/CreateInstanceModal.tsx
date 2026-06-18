import { useEffect, useState } from 'react'
import { X, Plus, Trash2 } from 'lucide-react'
import apiClient from '@/api/client'
import { useToastStore } from '@/stores/toast'

interface Node { id: string; name: string; status: string }
interface StoragePool { name: string; driver: string; size: number; used: number }
interface ImageTemplate { id: string; name: string; type: string; distro: string; release: string; arch: string; enabled: boolean; downloaded: boolean }
interface User { id: number; username: string }
interface VPC { id: string; name: string; node_id: string; ipv4_cidr: string; default_gateway_v4: string; bridge_name: string }

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
  const [templates, setTemplates] = useState<ImageTemplate[]>([])
  const [storages, setStorages] = useState<StoragePool[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [vpcs, setVpcs] = useState<VPC[]>([])

  const [name, setName] = useState('')
  const [type, setType] = useState<'container' | 'vm'>('container')
  const [nodeId, setNodeId] = useState('')
  const [templateId, setTemplateId] = useState('')
  const [assignToUserId, setAssignToUserId] = useState<number | ''>('')
  const [vpcId, setVpcId] = useState('')

  const [vcpu, setVcpu] = useState(1)
  const [memoryMb, setMemoryMb] = useState(512)
  const [diskGb, setDiskGb] = useState(5)
  const [storagePool, setStoragePool] = useState('')

  const [assignNat, setAssignNat] = useState(true)
  const [portMappingCount, setPortMappingCount] = useState(2)
  const [assignIpv4, setAssignIpv4] = useState(false)
  const [ipv4Count, setIpv4Count] = useState(0)
  const [assignIpv6, setAssignIpv6] = useState(false)
  const [ipv6Count, setIpv6Count] = useState(0)

  const [loginMethod, setLoginMethod] = useState<'auto' | 'password' | 'sshkey'>('auto')
  const [sshPassword, setSshPassword] = useState('')
  const [sshPublicKey, setSshPublicKey] = useState('')

  const [networkDown, setNetworkDown] = useState(0)
  const [networkUp, setNetworkUp] = useState(0)
  const [ioRead, setIoRead] = useState(0)
  const [ioWrite, setIoWrite] = useState(0)
  const [monthlyTraffic, setMonthlyTraffic] = useState(0)
  const [trafficMode, setTrafficMode] = useState<'total' | 'in' | 'out' | 'in_out'>('total')
  const [snapshotLimit, setSnapshotLimit] = useState(3)

  const [dataDisks, setDataDisks] = useState<{name: string; size_gb: number; storage_pool: string; mount_point: string}[]>([])

  useEffect(() => {
    if (!open) return
    apiClient.get('/nodes').then((r) => setNodes(r.data.data || []))
    apiClient.get('/users').then((r) => setUsers(r.data.data || []))
    setStep(1)
    resetForm()
  }, [open])

  useEffect(() => {
    if (!open || !nodeId) {
      setTemplates([])
      return
    }
    apiClient.get('/images', { params: { node_id: nodeId } }).then((r) => {
      setTemplates(r.data.data || [])
    }).catch(() => setTemplates([]))
  }, [open, nodeId])

  useEffect(() => {
    if (!nodeId) { setStorages([]); setVpcs([]); setVpcId(''); return }
    apiClient.get(`/nodes/${nodeId}/storages`).then((r) => {
      const list = r.data.data || []
      setStorages(list)
      if (list.length > 0) setStoragePool(list[0].name)
    }).catch(() => setStorages([]))
    // 加载节点VPC列表
    apiClient.get('/network/vpcs', { params: { node_id: nodeId } }).then((r) => {
      const list = r.data.data || []
      setVpcs(list)
      if (list.length > 0) setVpcId(list[0].id)
    }).catch(() => { setVpcs([]); setVpcId('') })
  }, [nodeId])

  const resetForm = () => {
    setName('')
    setType('container')
    setNodeId('')
    setTemplateId('')
    setAssignToUserId('')
    setVpcId('')
    setVcpu(1)
    setMemoryMb(512)
    setDiskGb(5)
    setStoragePool('')
    setAssignNat(true)
    setPortMappingCount(2)
    setAssignIpv4(false)
    setIpv4Count(0)
    setAssignIpv6(false)
    setIpv6Count(0)
    setLoginMethod('auto')
    setSshPassword('')
    setSshPublicKey('')
    setNetworkDown(0)
    setNetworkUp(0)
    setIoRead(0)
    setIoWrite(0)
    setMonthlyTraffic(0)
    setTrafficMode('total')
    setSnapshotLimit(3)
    setDataDisks([])
  }

  const filteredTemplates = templates.filter((t) => {
    if (t.type !== type) return false
    if (!t.enabled) return false
    if (!t.downloaded) return false
    return true
  })

  // waitForImageDownloadViaWS 通过 WebSocket 实时等待镜像下载完成
  const waitForImageDownloadViaWS = (imageId: string, _nodeId: string, maxWaitMs: number = 600000): Promise<boolean> => {
    return new Promise((resolve) => {
      const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const wsUrl = `${proto}//${window.location.host}/ws/images`
      const ws = new WebSocket(wsUrl)
      const timer = setTimeout(() => {
        ws.close()
        toast.error('镜像下载等待超时')
        resolve(false)
      }, maxWaitMs)

      ws.onopen = () => {
        console.log('创建实例: 镜像进度 WebSocket 已连接')
      }
      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data)
          if (msg.type === 'image_progress' && msg.payload.image_id === imageId) {
            const p = msg.payload
            if (p.stage === 'done') {
              clearTimeout(timer)
              ws.close()
              resolve(true)
            } else if (p.stage === 'error') {
              clearTimeout(timer)
              ws.close()
              toast.error(`镜像下载失败: ${p.error || '未知错误'}`)
              resolve(false)
            }
          }
        } catch {
          // ignore
        }
      }
      ws.onerror = () => {
        clearTimeout(timer)
        ws.close()
        toast.error('镜像下载 WebSocket 连接失败')
        resolve(false)
      }
      ws.onclose = () => {
        clearTimeout(timer)
      }
    })
  }

  const handleSubmit = async () => {
    if (!name || !nodeId || !templateId || assignToUserId === '') {
      toast.error('请填写所有必填字段')
      return
    }
    setLoading(true)
    try {
      // 检查模板是否已下载，未下载则自动触发并等待
      const selectedTemplate = templates.find((t) => t.id === templateId)
      if (selectedTemplate && nodeId && !selectedTemplate.downloaded) {
        toast.success('模板未下载，开始自动下载...')
        await apiClient.post(`/images/${templateId}/download`, { node_id: nodeId })
        const ok = await waitForImageDownloadViaWS(templateId, nodeId)
        if (!ok) {
          setLoading(false)
          return
        }
      }

      const payload: Record<string, unknown> = {
        name,
        type,
        template_id: templateId,
        node_id: nodeId,
        assign_to_user_id: assignToUserId,
        vpc_id: vpcId || undefined,
        login_method: loginMethod,
        vcpu,
        memory_mb: memoryMb,
        disk_gb: diskGb,
        storage_pool: storagePool || undefined,
        assign_nat: assignNat,
        port_mapping_count: assignNat ? portMappingCount : 0,
        assign_ipv4: assignIpv4,
        ipv4_count: assignIpv4 ? ipv4Count : 0,
        assign_ipv6: assignIpv6,
        ipv6_count: assignIpv6 ? ipv6Count : 0,
        ssh_password: loginMethod === 'password' ? sshPassword : undefined,
        ssh_public_key: loginMethod === 'sshkey' ? sshPublicKey : undefined,
        network_down_mbps: networkDown || undefined,
        network_up_mbps: networkUp || undefined,
        io_read_mbps: ioRead || undefined,
        io_write_mbps: ioWrite || undefined,
        monthly_traffic_gb: monthlyTraffic || undefined,
        traffic_mode: trafficMode,
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

  const addDataDisk = () => setDataDisks([...dataDisks, { name: `disk${dataDisks.length + 1}`, size_gb: 10, storage_pool: storagePool || '', mount_point: `/mnt/disk${dataDisks.length + 1}` }])
  const updateDataDisk = (idx: number, field: 'name' | 'size_gb' | 'storage_pool' | 'mount_point', val: string | number) => {
    const next = [...dataDisks]
    next[idx] = { ...next[idx], [field]: val }
    setDataDisks(next)
  }
  const removeDataDisk = (idx: number) => setDataDisks(dataDisks.filter((_, i) => i !== idx))

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-white rounded-xl shadow-xl w-[720px] max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200">
          <h2 className="text-lg font-semibold text-gray-900">创建实例</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600">
            <X size={18} />
          </button>
        </div>

        <div className="px-6 py-4 space-y-4">
          {/* Step 1: 基本信息 */}
          {step === 1 && (
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">名称 <span className="text-red-500">*</span></label>
                  <input value={name} onChange={(e) => setName(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-1 focus:ring-black" placeholder="实例名称" />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">类型</label>
                  <select value={type} onChange={(e) => { setType(e.target.value as 'container' | 'vm'); setTemplateId('') }} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm">
                    <option value="container">容器</option>
                    <option value="vm">虚拟机</option>
                  </select>
                </div>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">节点 <span className="text-red-500">*</span></label>
                  <select value={nodeId} onChange={(e) => setNodeId(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm">
                    <option value="">选择节点</option>
                    {nodes.map((n) => (
                      <option key={n.id} value={n.id}>{n.name} ({n.status})</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">分配给用户 <span className="text-red-500">*</span></label>
                  <select value={assignToUserId} onChange={(e) => setAssignToUserId(Number(e.target.value))} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm">
                    <option value="">选择用户</option>
                    {users.map((u) => (
                      <option key={u.id} value={u.id}>{u.username}</option>
                    ))}
                  </select>
                </div>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">系统模板 <span className="text-red-500">*</span></label>
                <select value={templateId} onChange={(e) => setTemplateId(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm">
                  <option value="">选择模板</option>
                  {filteredTemplates.map((t) => (
                    <option key={t.id} value={t.id}>
                      {t.name} ({t.distro} {t.release} / {t.arch})
                    </option>
                  ))}
                </select>
              </div>
              <div className="grid grid-cols-3 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">CPU</label>
                  <input type="number" min={0.1} step={0.1} value={vcpu} onChange={(e) => setVcpu(Number(e.target.value))} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">内存 (MB)</label>
                  <input type="number" min={64} step={64} value={memoryMb} onChange={(e) => setMemoryMb(Number(e.target.value))} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">磁盘 (GB)</label>
                  <input type="number" min={1} value={diskGb} onChange={(e) => setDiskGb(Number(e.target.value))} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" />
                </div>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">存储池</label>
                <select value={storagePool} onChange={(e) => setStoragePool(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm">
                  {storages.length === 0 && <option value="">默认: default</option>}
                  {storages.map((s) => (
                    <option key={s.name} value={s.name}>{s.name} ({s.driver})</option>
                  ))}
                </select>
              </div>
            </div>
          )}

          {/* Step 2: 网络和SSH */}
          {step === 2 && (
            <div className="space-y-4">
              {/* VPC 选择 */}
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">所属 VPC 网络</label>
                <select value={vpcId} onChange={(e) => setVpcId(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm">
                  {vpcs.length === 0 && <option value="">该节点暂无 VPC，请先创建</option>}
                  {vpcs.map((v) => (
                    <option key={v.id} value={v.id}>{v.name} ({v.ipv4_cidr})</option>
                  ))}
                </select>
                {vpcId && (
                  <p className="text-xs text-gray-500 mt-1">
                    网关: {vpcs.find((v) => v.id === vpcId)?.default_gateway_v4} | Bridge: {vpcs.find((v) => v.id === vpcId)?.bridge_name}
                  </p>
                )}
              </div>

              <div className="flex items-center gap-2">
                <input type="checkbox" id="nat" checked={assignNat} onChange={(e) => setAssignNat(e.target.checked)} className="w-4 h-4" />
                <label htmlFor="nat" className="text-sm text-gray-700">自动分配 NAT 端口映射</label>
              </div>
              {assignNat && (
                <div className="pl-6">
                  <label className="block text-sm font-medium text-gray-700 mb-1">端口映射配额（含 SSH）</label>
                  <input type="number" min={1} max={64} value={portMappingCount} onChange={(e) => setPortMappingCount(Math.max(1, Math.min(64, Number(e.target.value))))} className="w-32 px-3 py-2 border border-gray-300 rounded-lg text-sm" />
                  <p className="text-xs text-gray-400 mt-1">自动分配 SSH (22)，其余由系统分配</p>
                </div>
              )}

              <div className="border-t border-gray-100 pt-4 grid grid-cols-2 gap-4">
                <div>
                  <div className="flex items-center gap-2 mb-2">
                    <input type="checkbox" id="ipv4" checked={assignIpv4} onChange={(e) => setAssignIpv4(e.target.checked)} className="w-4 h-4" />
                    <label htmlFor="ipv4" className="text-sm font-medium text-gray-700">分配公网 IPv4</label>
                  </div>
                  {assignIpv4 && (
                    <input type="number" min={1} max={64} value={ipv4Count} onChange={(e) => setIpv4Count(Math.max(1, Math.min(64, Number(e.target.value))))} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" placeholder="数量" />
                  )}
                </div>
                <div>
                  <div className="flex items-center gap-2 mb-2">
                    <input type="checkbox" id="ipv6" checked={assignIpv6} onChange={(e) => setAssignIpv6(e.target.checked)} className="w-4 h-4" />
                    <label htmlFor="ipv6" className="text-sm font-medium text-gray-700">分配 IPv6 前缀</label>
                  </div>
                  {assignIpv6 && (
                    <input type="number" min={1} max={64} value={ipv6Count} onChange={(e) => setIpv6Count(Math.max(1, Math.min(64, Number(e.target.value))))} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" placeholder="数量" />
                  )}
                </div>
              </div>

              <div className="border-t border-gray-100 pt-4">
                <label className="block text-sm font-medium text-gray-700 mb-2">SSH 登录方式</label>
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
                  <input type="text" value={sshPassword} onChange={(e) => setSshPassword(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" placeholder="输入 root 密码" />
                )}
                {loginMethod === 'sshkey' && (
                  <textarea value={sshPublicKey} onChange={(e) => setSshPublicKey(e.target.value)} rows={3} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" placeholder="粘贴 SSH 公钥 (ssh-rsa / ssh-ed25519 ...)" />
                )}
              </div>
            </div>
          )}

          {/* Step 3: 资源限制 */}
          {step === 3 && (
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">出站带宽限制 (Mbps)</label>
                  <input type="number" min={0} value={networkDown} onChange={(e) => setNetworkDown(Number(e.target.value))} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" placeholder="0 = 不限" />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">入站带宽限制 (Mbps)</label>
                  <input type="number" min={0} value={networkUp} onChange={(e) => setNetworkUp(Number(e.target.value))} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" placeholder="0 = 不限" />
                </div>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">磁盘读取限制 (MB/s)</label>
                  <input type="number" min={0} value={ioRead} onChange={(e) => setIoRead(Number(e.target.value))} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" placeholder="0 = 不限" />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">磁盘写入限制 (MB/s)</label>
                  <input type="number" min={0} value={ioWrite} onChange={(e) => setIoWrite(Number(e.target.value))} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" placeholder="0 = 不限" />
                </div>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">月度流量限制 (GB)</label>
                  <input type="number" min={0} value={monthlyTraffic} onChange={(e) => setMonthlyTraffic(Number(e.target.value))} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" placeholder="0 = 不限" />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">流量计算方式</label>
                  <select value={trafficMode} onChange={(e) => setTrafficMode(e.target.value as any)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm">
                    <option value="total">出站+入站合计</option>
                    <option value="in">仅入站</option>
                    <option value="out">仅出站</option>
                    <option value="in_out">出入取较大值</option>
                  </select>
                </div>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">快照数量限制</label>
                <input type="number" min={0} value={snapshotLimit} onChange={(e) => setSnapshotLimit(Number(e.target.value))} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" />
              </div>

              <div className="border-t border-gray-100 pt-4">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm font-medium text-gray-700">数据盘</span>
                  <button onClick={addDataDisk} className="flex items-center gap-1 text-xs text-black hover:text-gray-700">
                    <Plus size={12} /> 添加数据盘
                  </button>
                </div>
                {dataDisks.map((disk, idx) => (
                  <div key={idx} className="grid grid-cols-4 gap-2 mb-2 items-center">
                    <input value={disk.name} onChange={(e) => updateDataDisk(idx, 'name', e.target.value)} className="px-2 py-1 border border-gray-300 rounded text-sm" placeholder="名称" />
                    <input type="number" min={1} value={disk.size_gb} onChange={(e) => updateDataDisk(idx, 'size_gb', Number(e.target.value))} className="px-2 py-1 border border-gray-300 rounded text-sm" placeholder="GB" />
                    <input value={disk.mount_point} onChange={(e) => updateDataDisk(idx, 'mount_point', e.target.value)} className="px-2 py-1 border border-gray-300 rounded text-sm" placeholder="挂载点" />
                    <button onClick={() => removeDataDisk(idx)} className="text-red-500 hover:text-red-700 justify-self-start">
                      <Trash2 size={14} />
                    </button>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        <div className="flex items-center justify-between px-6 py-4 border-t border-gray-200 bg-gray-50 rounded-b-xl">
          <div className="flex gap-2">
            {[1, 2, 3].map((s) => (
              <div key={s} className={`w-2 h-2 rounded-full ${s === step ? 'bg-black' : 'bg-gray-300'}`} />
            ))}
          </div>
          <div className="flex gap-2">
            {step > 1 && (
              <button onClick={() => setStep(step - 1)} className="px-4 py-2 text-sm text-gray-600 hover:text-gray-900">
                上一步
              </button>
            )}
            {step < 3 ? (
              <button onClick={() => setStep(step + 1)} className="px-4 py-2 bg-black text-white rounded-lg text-sm hover:bg-gray-800">
                下一步
              </button>
            ) : (
              <button onClick={handleSubmit} disabled={loading} className="px-4 py-2 bg-black text-white rounded-lg text-sm hover:bg-gray-800 disabled:opacity-50">
                {loading ? '创建中...' : '创建实例'}
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
