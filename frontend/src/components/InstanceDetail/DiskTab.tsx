import { HardDrive, Plus } from 'lucide-react'
import { Button } from '@/components/Button/Button'
import apiClient from '@/api/client'
import { formatBytes } from '@/utils/format'

export interface DataDiskItem {
  id: string
  name: string
  size_mb: number
  storage_pool: string
  mount_point: string
  status?: string
  updated_at?: string
}

interface DiskMetrics {
  disk_used?: number
  disk_total?: number
}

interface DiskTabProps {
  instanceId: string
  diskMB: number
  storagePool: string
  dataDisks: DataDiskItem[]
  metrics: DiskMetrics | null
  onRefresh: () => void
  toast: { success: (msg: string) => void; error: (msg: string) => void }
}

export function DiskTab({ instanceId, diskMB, storagePool, dataDisks, metrics, onRefresh, toast }: DiskTabProps) {
  return (
    <div className="space-y-4">
      {/* 系统盘 */}
      <div className="bg-surface rounded-xl border border-surface p-5">
        <h4 className="text-sm font-semibold text-primary mb-3 flex items-center gap-2">
          <HardDrive size={16} /> 系统盘
        </h4>
        <div className="flex items-center justify-between py-2 px-3 bg-surface-secondary rounded-lg">
          <div className="flex items-center gap-3">
            <HardDrive size={14} className="text-muted" />
            <span className="text-sm">系统盘</span>
            <span className="text-xs text-muted">{(diskMB / 1024).toFixed(0)} GB</span>
            <span className="text-xs text-muted">存储池: {storagePool || 'default'}</span>
          </div>
          {metrics?.disk_used !== undefined && metrics?.disk_total !== undefined && (
            <span className="text-xs text-tertiary">{formatBytes(metrics.disk_used)} / {formatBytes(metrics.disk_total)}</span>
          )}
        </div>
      </div>

      {/* 数据盘 */}
      <div className="bg-surface rounded-xl border border-surface p-5">
        <div className="flex items-center justify-between mb-4">
          <h4 className="text-sm font-semibold text-primary flex items-center gap-2">
            <HardDrive size={16} /> 数据盘
          </h4>
          <Button
            icon={<Plus size={14} />}
            size="sm"
            onClick={() => {
              const name = prompt('请输入数据盘名称：')
              if (!name) return
              const sizeStr = prompt('请输入数据盘大小（GB）：')
              if (!sizeStr) return
              const size = parseInt(sizeStr)
              if (!size || size < 1) {
                toast.error('请输入有效大小')
                return
              }
              apiClient.post(`/instances/${instanceId}/disks`, { name, size_mb: size * 1024 })
                .then(() => { toast.success('创建任务已下发'); onRefresh() })
                .catch((err: any) => toast.error(err.response?.data?.error || '创建失败'))
            }}
          >
            添加数据盘
          </Button>
        </div>
        {dataDisks.length > 0 ? (
          <div className="space-y-2">
            {dataDisks.map((disk) => (
              <div key={disk.id} className="flex items-center justify-between text-sm py-2 px-3 bg-surface-secondary rounded-lg">
                <div className="flex items-center gap-3">
                  <HardDrive size={14} className="text-muted" />
                  <span>{disk.name}</span>
                  <span className="text-xs text-muted">{(disk.size_mb / 1024).toFixed(0)} GB</span>
                  <span className="text-xs text-muted">{disk.mount_point || '未挂载'}</span>
                  {disk.status && disk.status !== 'attached' && (
                    <span className="text-xs text-amber-600">{disk.status}</span>
                  )}
                </div>
                <div className="flex items-center gap-2">
                  <button className="text-blue-500 hover:text-blue-700 text-xs" onClick={() => {
                    const newSize = prompt(`扩容数据盘 ${disk.name}，当前 ${(disk.size_mb / 1024).toFixed(0)}GB，请输入新大小（GB）：`)
                    if (!newSize) return
                    const size = parseInt(newSize)
                    if (!size || size * 1024 <= disk.size_mb) {
                      toast.error('新大小必须大于当前大小')
                      return
                    }
                    apiClient.put(`/instances/${instanceId}/disks/${disk.id}`, { size_mb: size * 1024 })
                      .then(() => { toast.success('扩容任务已下发'); onRefresh() })
                      .catch((err: any) => toast.error(err.response?.data?.error || '扩容失败'))
                  }}>扩容</button>
                  <button className="text-red-500 hover:text-red-700 text-xs" onClick={() => {
                    if (!confirm(`确认删除数据盘 ${disk.name}？数据将丢失。`)) return
                    apiClient.delete(`/instances/${instanceId}/disks/${disk.id}`)
                      .then(() => { toast.success('删除任务已下发'); onRefresh() })
                      .catch((err: any) => toast.error(err.response?.data?.error || '删除失败'))
                  }}>删除</button>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="text-sm text-muted text-center py-4">暂无数据盘</div>
        )}
      </div>
    </div>
  )
}
