import { useEffect, useState } from 'react'
import { Shield, CheckCircle, XCircle, RefreshCw } from 'lucide-react'
import apiClient from '@/api/client'
import { DataTable, type Column } from '@/components/DataTable/DataTable'
import { Button } from '@/components/Button/Button'
import { useToastStore } from '@/stores/toast'

interface SecurityAlert {
  id: string
  node_id: string
  instance_id?: string
  alert_type: string
  severity: string
  status: string
  source_ip?: string
  dest_port?: number
  protocol?: string
  details: string
  auto_action?: string
  detected_at: string
}

interface Summary {
  open_alerts: number
  critical_alerts: number
  warning_alerts: number
  last_24h: number
  by_type: Record<string, number>
}

export default function SecurityPage() {
  const toast = useToastStore()
  const [alerts, setAlerts] = useState<SecurityAlert[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [summary, setSummary] = useState<Summary | null>(null)
  const [statusFilter, setStatusFilter] = useState('open')

  const fetchAlerts = async () => {
    setLoading(true)
    try {
      const params: Record<string, string> = { limit: '100' }
      if (statusFilter) params.status = statusFilter
      const res = await apiClient.get('/security/alerts', { params })
      setAlerts(res.data.data || [])
      setTotal(res.data.total || 0)
    } finally {
      setLoading(false)
    }
  }

  const fetchSummary = async () => {
    try {
      const res = await apiClient.get('/security/summary')
      setSummary(res.data)
    } catch { /* ignore */ }
  }

  useEffect(() => { fetchAlerts(); fetchSummary() }, [statusFilter])

  const handleResolve = async (id: string) => {
    try {
      await apiClient.post(`/security/alerts/${id}/resolve`)
      toast.success('告警已标记为已解决')
      fetchAlerts()
      fetchSummary()
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '操作失败')
    }
  }

  const handleIgnore = async (id: string) => {
    try {
      await apiClient.post(`/security/alerts/${id}/ignore`)
      toast.success('告警已忽略')
      fetchAlerts()
      fetchSummary()
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '操作失败')
    }
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
    smtp_abuse: 'SMTP 滥用',
    mining: '挖矿检测',
    packet_flood: '发包攻击',
  }

  const columns: Column<SecurityAlert>[] = [
    {
      key: 'severity',
      title: '级别',
      render: (row) => (
        <span className={`text-xs px-2 py-0.5 rounded-full font-medium ${
          row.severity === 'critical' ? 'bg-red-100 text-red-700' :
          row.severity === 'warning' ? 'bg-yellow-100 text-yellow-700' :
          'bg-blue-100 text-blue-700'
        }`}>
          {severityMap[row.severity] || row.severity}
        </span>
      ),
    },
    {
      key: 'alert_type',
      title: '类型',
      render: (row) => <span>{typeMap[row.alert_type] || row.alert_type}</span>,
    },
    {
      key: 'details',
      title: '详情',
      render: (row) => <span className="text-xs max-w-[300px] truncate block">{row.details}</span>,
    },
    {
      key: 'instance_id',
      title: '实例',
      render: (row) => <span className="text-xs text-gray-500 font-mono">{row.instance_id || '-'}</span>,
    },
    {
      key: 'node_id',
      title: '节点',
      render: (row) => <span className="text-xs text-gray-500 font-mono">{row.node_id?.slice(0, 8) || '-'}</span>,
    },
    {
      key: 'source_ip',
      title: '来源 IP',
      render: (row) => <span className="text-xs font-mono">{row.source_ip || '-'}</span>,
    },
    {
      key: 'status',
      title: '状态',
      render: (row) => (
        <span className={`text-xs px-2 py-0.5 rounded-full ${
          row.status === 'open' ? 'bg-red-100 text-red-700' :
          row.status === 'resolved' ? 'bg-green-100 text-green-700' :
          'bg-gray-100 text-gray-600'
        }`}>
          {row.status === 'open' ? '待处理' : row.status === 'resolved' ? '已解决' : '已忽略'}
        </span>
      ),
    },
    {
      key: 'detected_at',
      title: '检测时间',
      render: (row) => <span className="text-xs text-gray-500">{new Date(row.detected_at).toLocaleString()}</span>,
    },
    {
      key: 'action',
      title: '操作',
      render: (row) => row.status === 'open' ? (
        <div className="flex items-center gap-2">
          <button
            className="flex items-center gap-1 text-xs text-green-600 hover:text-green-800"
            onClick={() => handleResolve(row.id)}
            title="标记已解决"
          >
            <CheckCircle size={14} />
            解决
          </button>
          <button
            className="flex items-center gap-1 text-xs text-gray-500 hover:text-gray-700"
            onClick={() => handleIgnore(row.id)}
            title="忽略"
          >
            <XCircle size={14} />
            忽略
          </button>
        </div>
      ) : <span className="text-xs text-gray-400">-</span>,
    },
  ]

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Shield size={22} className="text-black" />
          <h1 className="text-xl font-semibold text-black">安全管理</h1>
        </div>
        <Button size="sm" onClick={() => { fetchAlerts(); fetchSummary() }}>
          <RefreshCw size={14} className="mr-1" />
          刷新
        </Button>
      </div>

      {summary && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <StatCard label="待处理告警" value={summary.open_alerts} color="text-red-600" />
          <StatCard label="严重告警" value={summary.critical_alerts} color="text-red-700" />
          <StatCard label="警告" value={summary.warning_alerts} color="text-yellow-600" />
          <StatCard label="24h 新增" value={summary.last_24h} color="text-blue-600" />
        </div>
      )}

      <div className="flex gap-2">
        {(['open', 'resolved', 'ignored', ''] as const).map((s) => (
          <button
            key={s || 'all'}
            className={`text-xs px-3 py-1.5 rounded-full border transition-colors ${
              statusFilter === s ? 'bg-black text-white border-black' : 'bg-white text-gray-600 border-gray-300 hover:border-gray-400'
            }`}
            onClick={() => setStatusFilter(s)}
          >
            {s === 'open' ? '待处理' : s === 'resolved' ? '已解决' : s === 'ignored' ? '已忽略' : '全部'}
          </button>
        ))}
        <span className="text-xs text-gray-400 self-center ml-2">共 {total} 条</span>
      </div>

      <DataTable columns={columns} data={alerts} rowKey={(r) => r.id} loading={loading} />
    </div>
  )
}

function StatCard({ label, value, color }: { label: string; value: number; color: string }) {
  return (
    <div className="rounded-lg border p-4">
      <p className="text-xs text-gray-500">{label}</p>
      <p className={`text-2xl font-bold ${color}`}>{value}</p>
    </div>
  )
}
