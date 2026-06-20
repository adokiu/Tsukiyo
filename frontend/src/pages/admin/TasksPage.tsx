import { useEffect, useState, useRef } from 'react'
import { List, Clock, CheckCircle, XCircle, AlertCircle, RefreshCw } from 'lucide-react'
import apiClient from '@/api/client'
import { DataTable, type Column } from '@/components/DataTable/DataTable'
import { Button } from '@/components/Button/Button'

interface Task {
  id: string
  type: string
  status: string
  node_id: string
  node_name: string
  instance_id?: string
  instance_name?: string
  user_id: number
  error?: string
  retry_count: number
  created_at: string
  started_at?: string
  completed_at?: string
}

interface TaskLog {
  id: string
  task_id: string
  level: string
  message: string
  created_at: string
}

const statusMap: Record<string, { label: string; color: string; icon: any }> = {
  pending: { label: '待处理', color: 'bg-gray-100 text-gray-600', icon: Clock },
  running: { label: '执行中', color: 'bg-blue-100 text-blue-600', icon: RefreshCw },
  completed: { label: '已完成', color: 'bg-green-100 text-green-600', icon: CheckCircle },
  failed: { label: '失败', color: 'bg-red-100 text-red-600', icon: XCircle },
  canceled: { label: '已取消', color: 'bg-yellow-100 text-yellow-600', icon: AlertCircle },
}

const typeMap: Record<string, string> = {
  create_instance: '创建实例',
  delete_instance: '删除实例',
  start_instance: '启动实例',
  stop_instance: '停止实例',
  restart_instance: '重启实例',
  reinstall_instance: '重装实例',
  resize_instance: '调整配置',
  create_snapshot: '创建快照',
  restore_snapshot: '恢复快照',
  delete_snapshot: '删除快照',
  download_image: '下载镜像',
  delete_image: '删除镜像',
  cancel_image_download: '取消下载',
  apply_network: '应用网络',
  apply_firewall: '应用防火墙',
  format_disk: '格式化磁盘',
  init_storage: '初始化存储',
  create_partition: '创建分区',
  delete_partition: '删除分区',
  migrate_instance: '迁移实例',
  bridge_network: 'Bridge 网络',
}

export default function TasksPage() {
  const [tasks, setTasks] = useState<Task[]>([])
  const [selectedTask, setSelectedTask] = useState<Task | null>(null)
  const [logs, setLogs] = useState<TaskLog[]>([])
  const [loading, setLoading] = useState(true)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [statusFilter, setStatusFilter] = useState('')
  const wsRef = useRef<WebSocket | null>(null)

  const fetchTasks = async () => {
    setLoading(true)
    try {
      const params: any = { page, per_page: 20 }
      if (statusFilter) params.status = statusFilter
      const res = await apiClient.get('/tasks', { params })
      setTasks(res.data.data || [])
      setTotal(res.data.total || 0)
    } finally {
      setLoading(false)
    }
  }

  const fetchLogs = async (taskId: string) => {
    try {
      const res = await apiClient.get(`/tasks/${taskId}/logs`)
      setLogs(res.data.data || [])
    } catch (err) {
      console.error('获取任务日志失败', err)
    }
  }

  const connectWebSocket = () => {
    if (wsRef.current) {
      wsRef.current.close()
    }
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${proto}//${window.location.host}/ws/images`
    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onopen = () => {
      console.log('任务状态 WebSocket 已连接')
    }
    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        if (msg.type === 'task_status') {
          setTasks((prev) =>
            prev.map((task) =>
              task.id === msg.task_id
                ? { ...task, status: msg.status, error: msg.error }
                : task
            )
          )
          if (selectedTask && selectedTask.id === msg.task_id) {
            setSelectedTask((prev) => prev ? { ...prev, status: msg.status, error: msg.error } : null)
            fetchLogs(msg.task_id)
          }
        }
      } catch {
        // ignore
      }
    }
    ws.onclose = () => {
      console.log('任务状态 WebSocket 已断开，3秒后重连')
      wsRef.current = null
      setTimeout(connectWebSocket, 3000)
    }
    ws.onerror = (err) => {
      console.error('任务状态 WebSocket 错误', err)
    }
  }

  useEffect(() => {
    fetchTasks()
    connectWebSocket()
    return () => {
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
    }
  }, [page, statusFilter])

  useEffect(() => {
    if (selectedTask) {
      fetchLogs(selectedTask.id)
    }
  }, [selectedTask])

  const columns: Column<Task>[] = [
    {
      key: 'type',
      title: '任务类型',
      render: (row: Task) => (
        <span className="text-sm">{typeMap[row.type] || row.type}</span>
      ),
    },
    {
      key: 'status',
      title: '状态',
      render: (row: Task) => {
        const status = statusMap[row.status] || statusMap.pending
        const Icon = status.icon
        return (
          <span className={`flex items-center gap-1 text-xs px-2 py-0.5 rounded ${status.color}`}>
            <Icon size={14} />
            {status.label}
          </span>
        )
      },
    },
    {
      key: 'node',
      title: '节点',
      render: (row: Task) => (
        <span className="text-sm">{row.node_name || row.node_id}</span>
      ),
    },
    {
      key: 'instance',
      title: '实例',
      render: (row: Task) => (
        <span className="text-sm">{row.instance_name || '-'}</span>
      ),
    },
    {
      key: 'created_at',
      title: '创建时间',
      render: (row: Task) => (
        <span className="text-sm text-gray-600">
          {new Date(row.created_at).toLocaleString('zh-CN')}
        </span>
      ),
    },
    {
      key: 'actions',
      title: '操作',
      width: 100,
      render: (row: Task) => (
        <Button
          size="sm"
          variant="ghost"
          onClick={() => setSelectedTask(row)}
        >
          查看日志
        </Button>
      ),
    },
  ]

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <List size={22} className="text-black" />
          <h1 className="text-xl font-semibold text-black">任务队列</h1>
        </div>
        <div className="flex items-center gap-3">
          <select
            className="text-sm border border-gray-200 rounded-md px-3 py-1.5 bg-white focus:border-black focus:outline-none"
            value={statusFilter}
            onChange={(e) => {
              setStatusFilter(e.target.value)
              setPage(1)
            }}
          >
            <option value="">全部状态</option>
            <option value="pending">待处理</option>
            <option value="running">执行中</option>
            <option value="completed">已完成</option>
            <option value="failed">失败</option>
            <option value="canceled">已取消</option>
          </select>
          <Button size="sm" variant="ghost" onClick={() => fetchTasks()}>
            <RefreshCw size={14} className="mr-1" />
            刷新
          </Button>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-2">
          <DataTable
            columns={columns}
            data={tasks}
            rowKey={(r) => r.id}
            loading={loading}
          />
          {total > 20 && (
            <div className="flex items-center justify-between mt-4 px-4">
              <span className="text-sm text-gray-600">
                共 {total} 条记录
              </span>
              <div className="flex items-center gap-2">
                <Button
                  size="sm"
                  variant="ghost"
                  disabled={page === 1}
                  onClick={() => setPage(page - 1)}
                >
                  上一页
                </Button>
                <span className="text-sm">第 {page} 页</span>
                <Button
                  size="sm"
                  variant="ghost"
                  disabled={page * 20 >= total}
                  onClick={() => setPage(page + 1)}
                >
                  下一页
                </Button>
              </div>
            </div>
          )}
        </div>

        {selectedTask && (
          <div className="bg-white border border-gray-200 rounded-lg p-4">
            <div className="flex items-center justify-between mb-4">
              <h3 className="font-semibold">任务日志</h3>
              <Button size="sm" variant="ghost" onClick={() => setSelectedTask(null)}>
                关闭
              </Button>
            </div>
            <div className="space-y-2 mb-4 text-sm">
              <div><span className="text-gray-600">任务 ID:</span> {selectedTask.id}</div>
              <div><span className="text-gray-600">类型:</span> {typeMap[selectedTask.type] || selectedTask.type}</div>
              <div><span className="text-gray-600">状态:</span> {statusMap[selectedTask.status]?.label || selectedTask.status}</div>
              {selectedTask.error && (
                <div><span className="text-gray-600">错误:</span> <span className="text-red-600">{selectedTask.error}</span></div>
              )}
            </div>
            <div className="bg-gray-900 rounded-lg p-3 h-96 overflow-y-auto">
              {logs.length === 0 ? (
                <div className="text-gray-500 text-sm">暂无日志</div>
              ) : (
                logs.map((log) => (
                  <div
                    key={log.id}
                    className={`text-xs font-mono mb-1 ${
                      log.level === 'error' ? 'text-red-400' :
                      log.level === 'warn' ? 'text-yellow-400' :
                      'text-green-400'
                    }`}
                  >
                    <span className="text-gray-500">
                      [{new Date(log.created_at).toLocaleTimeString('zh-CN')}]
                    </span>
                    <span className="ml-2">{log.message}</span>
                  </div>
                ))
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
