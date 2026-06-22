import { useEffect, useState } from 'react'
import { RefreshCw } from 'lucide-react'
import apiClient from '@/api/client'
import { DataTable, type Column } from '@/components/DataTable/DataTable'
import { Button } from '@/components/Button/Button'
import { SlidePanel } from '@/components/SlidePanel/SlidePanel'
import { useToastStore } from '@/stores/toast'
import { PageLayout } from '@/components/PageLayout/PageLayout'
import { SearchInput } from '@/components/SearchInput/SearchInput'
import { FilterBar, type FilterField } from '@/components/FilterBar/FilterBar'
import { useListQuery } from '@/hooks/useListQuery'
import '@/components/PageTransition/PageTransition.css'

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

const statusMap: Record<string, { label: string; tag: string }> = {
  pending: { label: '待处理', tag: 'data-table-tag--offline' },
  running: { label: '执行中', tag: 'data-table-tag--online' },
  completed: { label: '已完成', tag: 'data-table-tag--completed' },
  failed: { label: '失败', tag: 'data-table-tag--failed' },
  canceled: { label: '已取消', tag: 'data-table-tag--offline' },
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
  const toast = useToastStore()
  const { data: tasks, total, loading, page, perPage, search, filters, setPage, setPerPage, setSearch, setFilter, refresh } = useListQuery<Task>('/tasks', {}, {
    wsUrl: '/ws/tasks',
    wsType: 'task_status',
    wsUpdate: (prev: any[], msg: any) => {
      return prev.map((task: any) =>
        task.id === msg.task_id
          ? { ...task, status: msg.status, error: msg.error }
          : task
      )
    },
    wsRefreshTypes: ['data_refresh'],
  })
  const [selectedTask, setSelectedTask] = useState<Task | null>(null)
  const [logs, setLogs] = useState<TaskLog[]>([])
  const [logPanelOpen, setLogPanelOpen] = useState(false)

  const fetchLogs = async (taskId: string) => {
    try {
      const res = await apiClient.get(`/tasks/${taskId}/logs`)
      setLogs(res.data.data || [])
    } catch {
      toast.error('获取任务日志失败')
    }
  }

  useEffect(() => {
    if (selectedTask) {
      fetchLogs(selectedTask.id)
    }
  }, [selectedTask?.id])

  const columns: Column<Task>[] = [
    {
      key: 'id',
      title: 'ID',
      width: 100,
      render: (row: Task) => (
        <span className="text-sm font-number text-tertiary">{row.id.slice(0, 8)}</span>
      ),
    },
    {
      key: 'type',
      title: '任务类型',
      width: 140,
      render: (row: Task) => (
        <span className="text-sm">{typeMap[row.type] || row.type}</span>
      ),
    },
    {
      key: 'status',
      title: '状态',
      width: 100,
      render: (row: Task) => {
        const status = statusMap[row.status] || statusMap.pending
        return (
          <span className={`data-table-tag ${status.tag}`}>
            {status.label}
          </span>
        )
      },
    },
    {
      key: 'node',
      title: '节点',
      width: 120,
      render: (row: Task) => (
        <span className="text-sm">{row.node_name || row.node_id}</span>
      ),
    },
    {
      key: 'instance',
      title: '实例',
      width: 140,
      render: (row: Task) => (
        <span className="text-sm">{row.instance_name || '-'}</span>
      ),
    },
    {
      key: 'retry_count',
      title: '重试',
      width: 60,
      render: (row: Task) => (
        <span className="font-number text-sm text-tertiary">{row.retry_count || 0}</span>
      ),
    },
    {
      key: 'created_at',
      title: '创建时间',
      width: 180,
      render: (row: Task) => (
        <span className="text-sm text-tertiary font-number">
          {new Date(row.created_at).toLocaleString('zh-CN')}
        </span>
      ),
    },
    {
      key: 'actions',
      title: '操作',
      width: 100,
      render: (row: Task) => (
        <button
          className="data-table-link-btn"
          onClick={() => { setSelectedTask(row); setLogPanelOpen(true) }}
        >
          查看日志
        </button>
      ),
    },
  ]

  const statusOptions = [
    { label: '待处理', value: 'pending' },
    { label: '执行中', value: 'running' },
    { label: '已完成', value: 'completed' },
    { label: '失败', value: 'failed' },
    { label: '已取消', value: 'canceled' },
  ]

  return (
    <PageLayout
      leftSlot={
        <>
          <SearchInput value={search} placeholder="搜索任务类型、节点、实例" onChange={setSearch} />
          <FilterBar
            fields={[
              { key: 'status', label: '状态', options: statusOptions },
            ] as FilterField[]}
            values={filters}
            onChange={setFilter}
          />
        </>
      }
      rightSlot={
        <Button variant="ghost" onClick={() => refresh()} icon={<RefreshCw size={16} />}>
          刷新
        </Button>
      }
    >
      <div className="page-transition__content" style={{ flex: 1, overflow: 'auto' }}>
      <DataTable
        columns={columns}
        data={tasks}
        rowKey={(r) => r.id}
        loading={loading}
        emptyText="暂无任务"
        pagination={{ page, size: perPage, total }}
        onPageChange={setPage}
        onSizeChange={setPerPage}
      />

      </div>

      {/* 任务日志侧边栏 */}
      <SlidePanel
        open={logPanelOpen}
        onClose={() => setLogPanelOpen(false)}
        title={`任务日志 - ${selectedTask ? (typeMap[selectedTask.type] || selectedTask.type) : ''}`}
        width={720}
      >
        {selectedTask && (
          <div className="space-y-4">
            {/* 任务详情 */}
            <div className="overflow-hidden rounded-lg border border-surface bg-surface">
              {[
                ['任务 ID', selectedTask.id],
                ['类型', typeMap[selectedTask.type] || selectedTask.type],
                ['状态', statusMap[selectedTask.status]?.label || selectedTask.status],
                ['节点', selectedTask.node_name || selectedTask.node_id],
                ['实例', selectedTask.instance_name || '-'],
                ['重试次数', String(selectedTask.retry_count || 0)],
                ['创建时间', new Date(selectedTask.created_at).toLocaleString('zh-CN')],
                ['开始时间', selectedTask.started_at ? new Date(selectedTask.started_at).toLocaleString('zh-CN') : '-'],
                ['完成时间', selectedTask.completed_at ? new Date(selectedTask.completed_at).toLocaleString('zh-CN') : '-'],
              ].map(([label, value]) => (
                <div key={label} className="grid gap-2 border-b border-surface-light px-3 py-2 text-xs last:border-b-0 md:grid-cols-[120px_1fr]">
                  <div className="font-medium text-tertiary">{label}</div>
                  <div className="font-number whitespace-pre-wrap break-words text-secondary">{value}</div>
                </div>
              ))}
            </div>

            {/* 错误信息 */}
            {selectedTask.error && (
              <div className="rounded-lg border border-red-200 bg-red-50 p-3">
                <div className="text-xs font-medium text-red-800 mb-1">错误信息</div>
                <div className="text-xs text-red-600 whitespace-pre-wrap break-words">{selectedTask.error}</div>
              </div>
            )}

            {/* 日志输出 */}
            <div>
              <h3 className="mb-2 text-sm font-semibold text-primary">日志输出</h3>
              <div className="bg-gray-900 rounded-lg p-3 h-96 overflow-y-auto">
                {logs.length === 0 ? (
                  <div className="text-tertiary text-sm">暂无日志</div>
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
                      <span className="text-tertiary">
                        [{new Date(log.created_at).toLocaleTimeString('zh-CN')}]
                      </span>
                      <span className="ml-2">{log.message}</span>
                    </div>
                  ))
                )}
              </div>
            </div>
          </div>
        )}
      </SlidePanel>
    </PageLayout>
  )
}
