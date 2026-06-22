import { useEffect, useState } from 'react'
import { Plus } from 'lucide-react'
import apiClient from '@/api/client'
import { DataTable, type Column } from '@/components/DataTable/DataTable'
import { Button } from '@/components/Button/Button'
import { Select } from '@/components/Select/Select'
import { Modal } from '@/components/Modal/Modal'
import { Tooltip } from '@/components/Tooltip/Tooltip'
import { useToastStore } from '@/stores/toast'
import { PageLayout, type PageTab } from '@/components/PageLayout/PageLayout'
import { BridgeFormPanel } from '@/components/NetworkPage/BridgeFormPanel'
import { EIPPoolFormPanel } from '@/components/NetworkPage/EIPPoolFormPanel'
import { EIPPoolEditPanel } from '@/components/NetworkPage/EIPPoolEditPanel'
import type { Node, Bridge, EIPPool, EIPAllocation } from '@/components/NetworkPage/types'
import '@/components/PageTransition/PageTransition.css'

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
