import { Plus, RotateCcw, Trash2 } from 'lucide-react'
import { Snapshot } from '@/api/instances'

interface SnapshotTabProps {
  snapshotLimit: number
  snapshots: Snapshot[]
  onCreate: () => void
  onRestore: (name: string) => void
  onDelete: (id: string) => void
}

export function SnapshotTab({ snapshotLimit, snapshots, onCreate, onRestore, onDelete }: SnapshotTabProps) {
  return (
    <div className="space-y-4">
      <div className="card p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-sm font-semibold">快照管理</h3>
          <div className="flex items-center gap-3">
            <span className="text-xs text-muted-foreground">{snapshots.length} / {snapshotLimit}</span>
            <button
              onClick={onCreate}
              disabled={snapshots.length >= snapshotLimit}
              className="btn btn--primary btn--sm disabled:opacity-50"
            >
              <Plus size={14} />
              <span>创建快照</span>
            </button>
          </div>
        </div>

        {snapshots.length === 0 ? (
          <div className="text-center text-muted-foreground py-8 text-sm">暂无快照</div>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-100">
                <th className="text-left py-2 text-muted-foreground font-medium">名称</th>
                <th className="text-left py-2 text-muted-foreground font-medium">创建时间</th>
                <th className="text-left py-2 text-muted-foreground font-medium">大小</th>
                <th className="text-right py-2 text-muted-foreground font-medium">操作</th>
              </tr>
            </thead>
            <tbody>
              {snapshots.map(snap => (
                <tr key={snap.id} className="border-b border-gray-50">
                  <td className="py-2 font-medium">{snap.name}</td>
                  <td className="py-2 text-muted-foreground">{new Date(snap.created_at).toLocaleString('zh-CN')}</td>
                  <td className="py-2 font-number">{snap.size ? `${(snap.size / 1024 / 1024).toFixed(1)} MB` : '-'}</td>
                  <td className="py-2 text-right">
                    <div className="flex justify-end gap-2">
                      <button
                        onClick={() => onRestore(snap.name)}
                        className="text-blue-600 hover:text-blue-800"
                        title="恢复"
                      >
                        <RotateCcw size={14} />
                      </button>
                      <button
                        onClick={() => onDelete(snap.id)}
                        className="text-red-500 hover:text-red-700"
                        title="删除"
                      >
                        <Trash2 size={14} />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}
