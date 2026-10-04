import { HardDrive } from 'lucide-react'
import { DataDisk, InstanceMetricsData } from '@/api/instances'
import { formatBytes } from '@/utils/format'

interface DiskTabProps {
  diskMB: number
  storagePool: string
  dataDisks: DataDisk[]
  metrics: InstanceMetricsData | null
}

export function DiskTab({ diskMB, storagePool, dataDisks, metrics }: DiskTabProps) {
  const diskTotalBytes = metrics?.disk_total || (diskMB * 1024 * 1024)
  const diskPercent = diskTotalBytes ? ((metrics?.disk_used || 0) / diskTotalBytes) * 100 : 0

  return (
    <div className="space-y-4">
      <div className="card p-5">
        <h3 className="text-sm font-semibold flex items-center gap-2 mb-4">
          <HardDrive size={16} /> 磁盘管理
        </h3>

        <div className="space-y-3">
          <div className="flex items-center justify-between p-3 bg-gray-50 rounded-lg">
            <div>
              <div className="text-sm font-medium">系统盘</div>
              <div className="text-xs text-muted-foreground">存储池: {storagePool || 'default'}</div>
            </div>
            <div className="text-right">
              <div className="text-sm font-number">{(diskMB / 1024).toFixed(0)} GB</div>
              {metrics?.disk_used !== undefined && (
                <div className="text-xs text-muted-foreground">{formatBytes(metrics.disk_used)} / {formatBytes(diskTotalBytes)} ({diskPercent.toFixed(1)}%)</div>
              )}
            </div>
          </div>

          {dataDisks.length > 0 && (
            <div className="pt-3 border-t border-gray-100">
              <div className="text-sm font-medium mb-3">数据盘 ({dataDisks.length})</div>
              {dataDisks.map(disk => (
                <div key={disk.id} className="flex items-center justify-between p-3 bg-gray-50 rounded-lg mb-2">
                  <div>
                    <div className="text-sm font-medium">{disk.name}</div>
                    <div className="text-xs text-muted-foreground">
                      挂载: {disk.mount_point || '-'} · 存储池: {disk.storage_pool || '-'}
                    </div>
                  </div>
                  <div className="text-right">
                    <div className="text-sm font-number">{(disk.size_mb / 1024).toFixed(0)} GB</div>
                    <div className="text-xs text-muted-foreground">{disk.status || '-'}</div>
                  </div>
                </div>
              ))}
            </div>
          )}

          {dataDisks.length === 0 && (
            <div className="text-center text-muted-foreground py-4 text-sm">暂无数据盘</div>
          )}
        </div>
      </div>
    </div>
  )
}
