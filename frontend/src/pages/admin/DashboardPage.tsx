import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Server, Boxes, Monitor, Bell, Loader2 } from 'lucide-react'
import apiClient from '@/api/client'

interface DashboardData {
  total_users: number
  total_nodes: number
  online_nodes: number
  total_instances: number
  running_instances: number
  recent_tasks: { id: string; type: string; status: string; error: string }[]
  node_resources: { id: string; name: string; status: string; cpu_percent: number; mem_percent: number; disk_percent: number; instance_count: number }[]
}

export default function DashboardPage() {
  const { t } = useTranslation()
  const [data, setData] = useState<DashboardData | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    apiClient.get('/dashboard').then((res) => setData(res.data)).finally(() => setLoading(false))
  }, [])

  const stats = [
    { label: t('dashboard.nodeCount'), value: data?.total_nodes ?? 0, icon: Server },
    { label: t('dashboard.instanceCount'), value: data?.total_instances ?? 0, icon: Boxes },
    { label: t('dashboard.runningInstances'), value: data?.running_instances ?? 0, icon: Monitor },
    { label: t('dashboard.alertsCount'), value: 0, icon: Bell },
  ]

  return (
    <div className="p-8">
      <h1 className="text-2xl font-display font-semibold tracking-tight mb-8">{t('dashboard.title')}</h1>

      {loading && (
        <div className="flex items-center gap-2 text-muted-foreground mb-8">
          <Loader2 size={18} className="animate-spin" />
          <span className="text-sm">加载中...</span>
        </div>
      )}

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-5 mb-8">
        {stats.map((s) => (
          <div key={s.label} className="glass-card p-6">
            <div className="flex items-center justify-between mb-4">
              <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-gray-100 dark:bg-gray-700">
                <s.icon className="text-black dark:text-white" size={20} />
              </div>
            </div>
            <p className="text-3xl font-display font-semibold tracking-tight">{s.value}</p>
            <p className="text-sm text-muted-foreground mt-1">{s.label}</p>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
        <div className="lg:col-span-2 glass-card p-6">
          <h3 className="text-base font-semibold mb-4">节点资源</h3>
          {data && data.node_resources.length > 0 ? (
            <div className="space-y-3">
              {data.node_resources.map((node) => (
                <div key={node.id} className="flex items-center justify-between text-sm">
                  <span className="font-medium">{node.name}</span>
                  <div className="flex gap-4 text-muted-foreground">
                    <span>CPU {node.cpu_percent.toFixed(1)}%</span>
                    <span>内存 {node.mem_percent.toFixed(1)}%</span>
                    <span>磁盘 {node.disk_percent.toFixed(1)}%</span>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="h-64 flex items-center justify-center text-muted-foreground text-sm">{t('common.noData')}</div>
          )}
        </div>
        <div className="glass-card p-6">
          <h3 className="text-base font-semibold mb-4">最近任务</h3>
          {data && data.recent_tasks.length > 0 ? (
            <div className="space-y-3">
              {data.recent_tasks.map((task) => (
                <div key={task.id} className="text-sm">
                  <div className="flex items-center justify-between">
                    <span className="font-medium">{task.type}</span>
                    <span className="text-xs text-muted-foreground">{task.status}</span>
                  </div>
                  {task.error && <p className="text-xs text-red-500 mt-1">{task.error}</p>}
                </div>
              ))}
            </div>
          ) : (
            <div className="h-64 flex items-center justify-center text-muted-foreground text-sm">{t('common.noData')}</div>
          )}
        </div>
      </div>
    </div>
  )
}
