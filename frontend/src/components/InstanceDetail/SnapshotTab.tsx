import { Calendar, Plus } from 'lucide-react'
import { Button } from '@/components/Button/Button'

export interface SnapshotItem {
  id: string
  name: string
  created_at: string
  size?: number
}

interface SnapshotTabProps {
  snapshotLimit: number
  snapshots: SnapshotItem[]
  onCreate: () => void
  onRestore: (name: string) => void
  onDelete: (id: string) => void
}

export function SnapshotTab({ snapshotLimit, snapshots, onCreate, onRestore, onDelete }: SnapshotTabProps) {
  return (
    <div className="bg-surface rounded-xl border border-surface p-5">
      <div className="flex items-center justify-between mb-4">
        <h4 className="text-sm font-semibold text-primary flex items-center gap-2">
          <Calendar size={16} /> 快照管理
          <span className="text-xs text-muted font-normal">
            (上限 {snapshotLimit} 个)
          </span>
        </h4>
        <Button icon={<Plus size={14} />} onClick={onCreate} size="sm">创建快照</Button>
      </div>
      {snapshots.length > 0 ? (
        <div className="space-y-2">
          {snapshots.map((s) => (
            <div key={s.id} className="flex items-center justify-between text-sm py-2 px-3 bg-surface-secondary rounded-lg">
              <div className="flex items-center gap-3">
                <Calendar size={14} className="text-muted" />
                <span>{s.name}</span>
                <span className="text-xs text-muted">{new Date(s.created_at).toLocaleString()}</span>
              </div>
              <div className="flex items-center gap-2">
                <button
                  className="text-blue-500 hover:text-blue-700 text-xs"
                  onClick={() => onRestore(s.name)}
                >
                  恢复
                </button>
                <button
                  className="text-red-500 hover:text-red-700 text-xs"
                  onClick={() => onDelete(s.id)}
                >
                  删除
                </button>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="text-sm text-muted text-center py-4">暂无快照</div>
      )}
    </div>
  )
}
