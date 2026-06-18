import { useEffect, useState } from 'react'
import { Shield, Ban } from 'lucide-react'
import apiClient from '@/api/client'
import { DataTable, type Column } from '@/components/DataTable/DataTable'
import { useToastStore } from '@/stores/toast'

interface Alert {
  id: string
  type: string
  severity: string
  instance_id?: string
  node_id?: string
  description: string
  timestamp: string
  resolved: boolean
}

export default function SecurityPage() {
  const toast = useToastStore()
  const [alerts, setAlerts] = useState<Alert[]>([])
  const [loading, setLoading] = useState(true)

  const fetchAlerts = () => {
    setLoading(true)
    apiClient.get('/security/alerts').then((res) => setAlerts(res.data.data || [])).finally(() => setLoading(false))
  }

  useEffect(() => { fetchAlerts() }, [])

  const handleBlock = (alert: Alert) => {
    toast.info(`封锁实例 ${alert.instance_id || '未知'} 功能待后端实现`)
  }

  const severityMap: Record<string, string> = {
    critical: '严重',
    warning: '警告',
    info: '信息',
  }

  const typeMap: Record<string, string> = {
    abnormal_traffic: '异常流量',
    brute_force: '暴力破解',
    port_scan: '端口扫描',
    smtp_abuse: 'SMTP滥用',
    mining: '挖矿检测',
  }

  const columns: Column<Alert>[] = [
    {
      key: 'severity',
      title: '级别',
      render: (row: Alert) => (
        <span className={`text-xs px-2 py-1 rounded font-medium ${
          row.severity === 'critical' ? 'bg-red-100 text-red-700' :
          row.severity === 'warning' ? 'bg-orange-100 text-orange-700' :
          'bg-gray-100 text-gray-700'
        }`}>
          {severityMap[row.severity] || row.severity}
        </span>
      ),
    },
    {
      key: 'type',
      title: '类型',
      render: (row: Alert) => <span>{typeMap[row.type] || row.type}</span>,
    },
    { key: 'description', title: '告警内容' },
    {
      key: 'instance_id',
      title: '实例',
      render: (row: Alert) => <span className="text-xs text-gray-500">{row.instance_id || '-'}</span>,
    },
    {
      key: 'node_id',
      title: '节点',
      render: (row: Alert) => <span className="text-xs text-gray-500">{row.node_id || '-'}</span>,
    },
    {
      key: 'resolved',
      title: '状态',
      render: (row: Alert) => (
        <span className={`text-xs px-2 py-1 rounded ${row.resolved ? 'bg-gray-100 text-gray-500' : 'bg-black text-white'}`}>
          {row.resolved ? '已处理' : '未处理'}
        </span>
      ),
    },
    {
      key: 'timestamp',
      title: '时间',
      render: (row: Alert) => <span className="text-xs text-gray-500">{new Date(row.timestamp).toLocaleString()}</span>,
    },
    {
      key: 'action',
      title: '操作',
      render: (row: Alert) => (
        <button
          className="flex items-center gap-1 text-xs text-red-500 hover:text-red-700"
          onClick={() => handleBlock(row)}
        >
          <Ban size={14} />
          封锁
        </button>
      ),
    },
  ]

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center gap-3">
        <Shield size={22} className="text-black" />
        <h1 className="text-xl font-semibold text-black">安全管理</h1>
      </div>
      <DataTable columns={columns} data={alerts} rowKey={(r) => r.id} loading={loading} />
    </div>
  )
}
