import { useEffect, useState } from 'react'
import { ClipboardList } from 'lucide-react'
import apiClient from '@/api/client'
import { DataTable, type Column } from '@/components/DataTable/DataTable'

interface Log {
  id: string
  username: string
  action: string
  target: string
  ip_address: string
  success: boolean
  created_at: string
}

export default function AuditLogsPage() {
  const [logs, setLogs] = useState<Log[]>([])
  const [loading, setLoading] = useState(true)

  const fetchLogs = () => {
    setLoading(true)
    apiClient.get('/audit-logs').then((res) => setLogs(res.data.data || [])).finally(() => setLoading(false))
  }

  useEffect(() => { fetchLogs() }, [])

  const columns: Column<Log>[] = [
    { key: 'username', title: '用户' },
    { key: 'action', title: '操作' },
    { key: 'target', title: '对象' },
    { key: 'ip_address', title: 'IP 地址' },
    {
      key: 'success',
      title: '结果',
      render: (row: Log) => (
        <span className={`text-xs px-2 py-1 rounded ${row.success ? 'bg-black text-white' : 'bg-red-100 text-red-600'}`}>
          {row.success ? '成功' : '失败'}
        </span>
      ),
    },
    { key: 'created_at', title: '时间' },
  ]

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center gap-3">
        <ClipboardList size={22} className="text-black" />
        <h1 className="text-xl font-semibold text-black">操作日志</h1>
      </div>
      <DataTable columns={columns} data={logs} rowKey={(r) => r.id} loading={loading} />
    </div>
  )
}
