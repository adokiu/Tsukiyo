import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { Plus, Boxes, Trash2, Play, Square, RotateCw, Terminal } from 'lucide-react'
import apiClient from '@/api/client'
import { DataTable, type Column } from '@/components/DataTable/DataTable'
import { Button } from '@/components/Button/Button'
import { useToastStore } from '@/stores/toast'
import CreateInstanceModal from './CreateInstanceModal'

interface Instance {
  id: string
  name: string
  type: string
  status: string
  node_id: string
  user_id: number
  incus_name: string
  vcpu: number
  memory_mb: number
  disk_gb: number
  internal_ipv4?: string
  ipv4_address?: string
  ssh_port?: number
  bridge_id?: string
  created_at: string
}

interface Props {
  instanceType?: 'vm' | 'container'
}

export default function InstancesPage({ instanceType }: Props) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const toast = useToastStore()
  const [instances, setInstances] = useState<Instance[]>([])
  const [loading, setLoading] = useState(true)
  const [modalOpen, setModalOpen] = useState(false)

  const fetchInstances = () => {
    setLoading(true)
    apiClient.get('/instances').then((res) => {
      let list: Instance[] = res.data.data || []
      if (instanceType === 'vm') {
        list = list.filter((i) => i.type === 'vm' || i.type === 'virtual-machine')
      } else if (instanceType === 'container') {
        list = list.filter((i) => i.type === 'container' || i.type === 'lxc')
      }
      setInstances(list)
    }).finally(() => setLoading(false))
  }

  useEffect(() => { fetchInstances() }, [instanceType])

  const pageTitle = instanceType === 'vm'
    ? t('nav.virtualMachines')
    : instanceType === 'container'
      ? t('nav.containers')
      : t('instance.title')

  const handleAction = async (id: string, action: string) => {
    try {
      if (action === 'console') {
        const res = await apiClient.get(`/instances/${id}/console`)
        if (res.data.token) {
          window.open(`/console?token=${res.data.token}`, '_blank')
        }
      } else {
        await apiClient.post(`/instances/${id}/${action}`)
        toast.success(`操作 ${action} 已下发`)
        fetchInstances()
      }
    } catch (err: any) {
      toast.error(err.response?.data?.error || '操作失败')
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('确认删除该实例？')) return
    try {
      await apiClient.delete(`/instances/${id}`)
      toast.success('删除任务已下发')
      fetchInstances()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '删除任务下发失败')
    }
  }

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'running': return 'text-green-600'
      case 'stopped': return 'text-gray-500'
      case 'creating': return 'text-amber-600'
      case 'error': return 'text-red-600'
      default: return 'text-gray-500'
    }
  }

  const columns: Column<Instance>[] = [
    { key: 'name', title: '名称', render: (row) => (
      <button className="text-sm font-medium text-blue-600 hover:text-blue-800 hover:underline" onClick={() => navigate(`/admin/instanceManagement/instances/${row.id}`)}>
        {row.name}
      </button>
    )},
    { key: 'type', title: '类型', render: (row) => <span className="text-xs font-mono">{row.type}</span> },
    { key: 'status', title: '状态', render: (row) => <span className={`text-xs font-medium ${getStatusColor(row.status)}`}>{row.status}</span> },
    { key: 'node_id', title: '节点', render: (row) => <span className="text-xs font-mono text-gray-500">{row.node_id.slice(0, 8)}</span> },
    { key: 'user_id', title: '用户' },
    { key: 'vcpu', title: 'CPU' },
    { key: 'memory_mb', title: '内存 (MB)' },
    { key: 'disk_gb', title: '磁盘 (GB)' },
    {
      key: 'network',
      title: '网络',
      render: (row) => (
        <div className="text-xs text-gray-600 space-y-0.5">
          {row.internal_ipv4 ? <div className="font-mono text-gray-700">内网: {row.internal_ipv4}</div> : null}
          {row.ipv4_address ? <div className="font-mono text-blue-600">公网: {row.ipv4_address}</div> : null}
          {row.bridge_id ? <div className="text-gray-400">Bridge:{row.bridge_id.slice(0, 8)}</div> : null}
          {row.ssh_port ? <div className="text-gray-400">SSH:{row.ssh_port}</div> : null}
        </div>
      ),
    },
    { key: 'created_at', title: '创建时间', render: (row) => <span className="text-xs text-gray-500">{row.created_at}</span> },
    {
      key: 'action',
      title: '操作',
      render: (row: Instance) => (
        <div className="flex items-center gap-1">
          {row.status === 'stopped' && (
            <button className="p-1 text-green-600 hover:text-green-800" onClick={() => handleAction(row.id, 'start')} title="启动">
              <Play size={14} />
            </button>
          )}
          {row.status === 'running' && (
            <button className="p-1 text-amber-600 hover:text-amber-800" onClick={() => handleAction(row.id, 'stop')} title="停止">
              <Square size={14} />
            </button>
          )}
          <button className="p-1 text-blue-600 hover:text-blue-800" onClick={() => handleAction(row.id, 'restart')} title="重启">
            <RotateCw size={14} />
          </button>
          <button className="p-1 text-gray-600 hover:text-gray-800" onClick={() => handleAction(row.id, 'console')} title="控制台">
            <Terminal size={14} />
          </button>
          <button className="p-1 text-red-500 hover:text-red-700" onClick={() => handleDelete(row.id)} title="删除">
            <Trash2 size={14} />
          </button>
        </div>
      ),
    },
  ]

  return (
    <div className="page-container">
      <div className="page-header">
        <h1 className="page-title flex items-center gap-2">
          <Boxes size={20} />
          {pageTitle}
        </h1>
        <Button icon={<Plus size={16} />} onClick={() => setModalOpen(true)}>
          {t('instance.createInstance')}
        </Button>
      </div>

      <div className="page-card p-4">
        <DataTable columns={columns} data={instances} rowKey={(r) => r.id} loading={loading} />
      </div>
      <CreateInstanceModal open={modalOpen} onClose={() => setModalOpen(false)} onSuccess={fetchInstances} />
    </div>
  )
}
