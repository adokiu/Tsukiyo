import { useState } from 'react'
import { ChevronRight, Trash2, Globe } from 'lucide-react'

export interface PortMappingItem {
  id: string
  host_port: number
  container_port: number
  protocol: string
  description?: string
}

interface PortMappingTabProps {
  portMappings: PortMappingItem[]
  portMappingLimit?: number
  onAdd: (containerPort: number, hostPort: number | null, protocol: string, description: string) => void
  onDelete: (pmID: string) => void
}

export function PortMappingTab({ portMappings, portMappingLimit, onAdd, onDelete }: PortMappingTabProps) {
  const [addingPM, setAddingPM] = useState(false)

  return (
    <div className="space-y-4">
      <div className="bg-surface rounded-xl border border-surface overflow-hidden">
        <div className="px-4 py-3 bg-surface-secondary border-b border-surface flex items-center justify-between">
          <h4 className="text-sm font-semibold text-primary">端口映射列表</h4>
          <span className="text-xs text-tertiary">
            已用 <span className="font-medium text-primary">{portMappings.length}</span>
            {portMappingLimit ? <> / <span className="font-medium text-primary">{portMappingLimit}</span></> : ''} 个
          </span>
        </div>
        {portMappings.length > 0 ? (
          <div className="divide-y divide-surface-light">
            {portMappings.map((pm) => (
              <div key={pm.id} className="flex items-center gap-3 px-4 py-2.5 bg-surface-hover transition-colors">
                <span className="px-1.5 py-0.5 bg-blue-50 text-blue-600 text-[11px] font-semibold rounded uppercase flex-shrink-0">{pm.protocol}</span>
                <div className="flex items-center gap-1.5 font-mono text-sm">
                  <span className="text-primary">{pm.host_port}</span>
                  <ChevronRight size={12} className="text-muted" />
                  <span className="text-primary">{pm.container_port}</span>
                </div>
                {pm.description && (
                  <span className="text-xs text-tertiary bg-surface-secondary px-1.5 py-0.5 rounded truncate max-w-[120px]" title={pm.description}>{pm.description}</span>
                )}
                <div className="flex-1" />
                <button
                  onClick={() => onDelete(pm.id)}
                  className="p-1 text-muted hover:text-red-500 hover:bg-red-50 rounded-md transition-colors"
                >
                  <Trash2 size={14} />
                </button>
              </div>
            ))}
          </div>
        ) : (
          <div className="flex flex-col items-center py-8 text-muted">
            <Globe size={28} className="mb-2 opacity-30" />
            <span className="text-sm">暂无端口映射</span>
          </div>
        )}
      </div>

      <div className="bg-surface rounded-xl border border-surface p-4 space-y-3">
        <div className="flex items-center justify-between">
          <h5 className="text-sm font-semibold text-primary">添加映射</h5>
          <button
            onClick={() => setAddingPM(!addingPM)}
            className="text-xs text-blue-600 hover:text-blue-800"
          >
            {addingPM ? '取消' : '添加'}
          </button>
        </div>
        {addingPM && (
          <PortMappingFormInline onSubmit={onAdd} onCancel={() => setAddingPM(false)} />
        )}
      </div>
    </div>
  )
}

function PortMappingFormInline({ onSubmit, onCancel }: { onSubmit: (cp: number, hp: number | null, proto: string, desc: string) => void; onCancel: () => void }) {
  const [containerPort, setContainerPort] = useState('')
  const [hostPort, setHostPort] = useState('')
  const [protocol, setProtocol] = useState('tcp')
  const [description, setDescription] = useState('')

  return (
    <div className="space-y-2">
      <div className="grid grid-cols-4 gap-2">
        <div>
          <label className="block text-xs text-tertiary mb-1">内部端口</label>
          <input type="number" value={containerPort} onChange={(e) => setContainerPort(e.target.value)} className="w-full px-2 py-1.5 border border-surface-strong rounded text-sm" placeholder="80" />
        </div>
        <div>
          <label className="block text-xs text-tertiary mb-1">外部端口</label>
          <input type="number" value={hostPort} onChange={(e) => setHostPort(e.target.value)} className="w-full px-2 py-1.5 border border-surface-strong rounded text-sm" placeholder="留空自动分配" />
        </div>
        <div>
          <label className="block text-xs text-tertiary mb-1">协议</label>
          <select value={protocol} onChange={(e) => setProtocol(e.target.value)} className="w-full px-2 py-1.5 border border-surface-strong rounded text-sm">
            <option value="tcp">TCP</option>
            <option value="udp">UDP</option>
            <option value="both">TCP/UDP</option>
          </select>
        </div>
        <div>
          <label className="block text-xs text-tertiary mb-1">备注</label>
          <input value={description} onChange={(e) => setDescription(e.target.value)} className="w-full px-2 py-1.5 border border-surface-strong rounded text-sm" placeholder="HTTP" />
        </div>
      </div>
      <div className="flex justify-end gap-2">
        <button className="text-xs text-tertiary hover:text-secondary" onClick={onCancel}>取消</button>
        <button className="text-xs bg-blue-600 text-white px-3 py-1.5 rounded hover:bg-blue-700" onClick={() => {
          const cp = parseInt(containerPort)
          if (!cp || cp < 1 || cp > 65535) return
          const hp = hostPort ? parseInt(hostPort) : null
          if (hp && (hp < 1 || hp > 65535)) return
          onSubmit(cp, hp, protocol, description)
        }}>确认</button>
      </div>
    </div>
  )
}
