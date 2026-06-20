import { useEffect, useState, useRef, useCallback } from 'react'
import { X, CheckCircle, XCircle, Loader2 } from 'lucide-react'
import apiClient from '@/api/client'

interface TaskLogEntry {
  level: string
  message: string
  timestamp: number
}

interface TaskProgressModalProps {
  taskId: string | null
  taskType: string
  onClose: () => void
}

const typeLabels: Record<string, string> = {
  create_partition: '创建分区',
  delete_partition: '删除分区',
  format_disk: '格式化磁盘',
  init_storage: '初始化存储池',
  delete_storage: '删除存储池',
}

export function TaskProgressModal({ taskId, taskType, onClose }: TaskProgressModalProps) {
  const [logs, setLogs] = useState<TaskLogEntry[]>([])
  const [status, setStatus] = useState<string>('running')
  const [error, setError] = useState<string>('')
  const logEndRef = useRef<HTMLDivElement>(null)
  const wsRef = useRef<WebSocket | null>(null)

  const fetchExistingLogs = useCallback(async () => {
    if (!taskId) return
    try {
      const res = await apiClient.get(`/tasks/${taskId}/logs`)
      const existing: TaskLogEntry[] = (res.data.data || []).map((l: any) => ({
        level: l.level,
        message: l.message,
        timestamp: new Date(l.created_at).getTime() / 1000,
      }))
      setLogs(existing)
    } catch {
      // ignore
    }
  }, [taskId])

  const fetchTaskStatus = useCallback(async () => {
    if (!taskId) return
    try {
      const res = await apiClient.get(`/tasks/${taskId}`)
      const task = res.data
      if (task && task.status) {
        setStatus(task.status)
        if (task.error) setError(task.error)
      }
    } catch {
      // ignore
    }
  }, [taskId])

  useEffect(() => {
    if (!taskId) return

    setLogs([])
    setStatus('running')
    setError('')

    // 先通过 REST API 获取当前状态和已有日志（避免错过 WS 连接前的消息）
    fetchTaskStatus()
    fetchExistingLogs()

    // 连接 WebSocket
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${proto}//${window.location.host}/ws/tasks`
    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        if (msg.type === 'task_log' && msg.task_id === taskId) {
          setLogs((prev) => [...prev, {
            level: msg.level,
            message: msg.message,
            timestamp: msg.timestamp,
          }])
        } else if (msg.type === 'task_status' && msg.task_id === taskId) {
          setStatus(msg.status)
          if (msg.error) setError(msg.error)
        }
      } catch {
        // ignore
      }
    }

    // 如果任务还在运行，定时轮询状态作为兜底
    const pollInterval = setInterval(async () => {
      const res = await apiClient.get(`/tasks/${taskId}`).catch(() => null)
      if (res?.data?.status) {
        setStatus(res.data.status)
        if (res.data.error) setError(res.data.error)
        if (res.data.status === 'completed' || res.data.status === 'failed') {
          clearInterval(pollInterval)
        }
      }
    }, 3000)

    return () => {
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
      clearInterval(pollInterval)
    }
  }, [taskId, fetchExistingLogs, fetchTaskStatus])

  useEffect(() => {
    logEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [logs])

  if (!taskId) return null

  const isSuccess = status === 'completed'
  const isFailed = status === 'failed'
  const isRunning = status === 'running' || status === 'pending'

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="bg-white rounded-lg shadow-xl w-[600px] max-h-[80vh] flex flex-col">
        {/* 头部 */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-gray-200">
          <div className="flex items-center gap-2">
            {isRunning && <Loader2 size={18} className="text-blue-500 animate-spin" />}
            {isSuccess && <CheckCircle size={18} className="text-green-500" />}
            {isFailed && <XCircle size={18} className="text-red-500" />}
            <h3 className="text-sm font-semibold text-black">
              {typeLabels[taskType] || taskType} - 任务进度
            </h3>
          </div>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600"
          >
            <X size={18} />
          </button>
        </div>

        {/* 状态信息 */}
        <div className="px-5 py-3 border-b border-gray-100">
          <div className="flex items-center gap-3 text-xs">
            <span className="text-gray-500">任务 ID:</span>
            <span className="font-mono text-gray-700">{taskId.substring(0, 8)}</span>
            <span className="text-gray-300">|</span>
            <span className="text-gray-500">状态:</span>
            {isRunning && <span className="text-blue-600 font-medium">执行中</span>}
            {isSuccess && <span className="text-green-600 font-medium">已完成</span>}
            {isFailed && <span className="text-red-600 font-medium">失败</span>}
          </div>
          {error && (
            <div className="mt-2 text-xs text-red-600 bg-red-50 rounded px-2 py-1">{error}</div>
          )}
        </div>

        {/* 日志区域 */}
        <div className="flex-1 overflow-y-auto bg-gray-900 px-4 py-3" style={{ maxHeight: '400px' }}>
          {logs.length === 0 ? (
            <div className="text-gray-500 text-xs">等待日志输出...</div>
          ) : (
            logs.map((log, i) => (
              <div
                key={i}
                className={`text-xs font-mono mb-1 ${
                  log.level === 'error' ? 'text-red-400' :
                  log.level === 'warn' ? 'text-yellow-400' :
                  'text-green-400'
                }`}
              >
                <span className="text-gray-500">
                  [{new Date(log.timestamp * 1000).toLocaleTimeString('zh-CN')}]
                </span>
                <span className="ml-2">{log.message}</span>
              </div>
            ))
          )}
          <div ref={logEndRef} />
        </div>

        {/* 底部 */}
        <div className="flex justify-end px-5 py-3 border-t border-gray-200">
          <button
            onClick={onClose}
            className="text-sm px-4 py-1.5 border border-gray-300 rounded-lg text-gray-700 hover:bg-gray-50"
          >
            {isRunning ? '关闭（任务继续执行）' : '关闭'}
          </button>
        </div>
      </div>
    </div>
  )
}
