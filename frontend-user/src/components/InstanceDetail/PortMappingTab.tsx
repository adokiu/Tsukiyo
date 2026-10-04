import { useState } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import { PortMapping } from '@/api/instances'

interface PortMappingTabProps {
  portMappings: PortMapping[]
  portMappingLimit?: number
  onAdd: (containerPort: number, hostPort: number | null, protocol: string, description: string) => void
  onDelete: (id: string) => void
}

export function PortMappingTab({ portMappings, portMappingLimit, onAdd, onDelete }: PortMappingTabProps) {
  const [containerPort, setContainerPort] = useState('')
  const [hostPort, setHostPort] = useState('')
  const [protocol, setProtocol] = useState('tcp')
  const [description, setDescription] = useState('')

  const handleAdd = () => {
    const cp = parseInt(containerPort)
    if (!cp || cp < 1 || cp > 65535) return
    const hp = hostPort ? parseInt(hostPort) : null
    if (hp && (hp < 1 || hp > 65535)) return
    onAdd(cp, hp, protocol, description)
    setContainerPort('')
    setHostPort('')
    setDescription('')
  }

  return (
    <div className="space-y-4">
      <div className="card p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-sm font-semibold">端口映射</h3>
          {portMappingLimit !== undefined && (
            <span className="text-xs text-muted-foreground">上限: {portMappingLimit} 个</span>
          )}
        </div>

        <div className="flex gap-2 mb-4">
          <input
            type="number"
            value={containerPort}
            onChange={(e) => setContainerPort(e.target.value)}
            placeholder="容器端口"
            className="w-32 px-3 py-2 border border-gray-200 rounded-lg text-sm"
          />
          <input
            type="number"
            value={hostPort}
            onChange={(e) => setHostPort(e.target.value)}
            placeholder="宿主端口（留空随机）"
            className="w-40 px-3 py-2 border border-gray-200 rounded-lg text-sm"
          />
          <select
            value={protocol}
            onChange={(e) => setProtocol(e.target.value)}
            className="px-3 py-2 border border-gray-200 rounded-lg text-sm"
          >
            <option value="tcp">TCP</option>
            <option value="udp">UDP</option>
          </select>
          <input
            type="text"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="描述（可选）"
            className="flex-1 px-3 py-2 border border-gray-200 rounded-lg text-sm"
          />
          <button
            onClick={handleAdd}
            disabled={!containerPort}
            className="btn btn--primary btn--md disabled:opacity-50"
          >
            <Plus size={16} />
            <span>添加</span>
          </button>
        </div>

        {portMappings.length === 0 ? (
          <div className="text-center text-muted-foreground py-8 text-sm">暂无端口映射</div>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-100">
                <th className="text-left py-2 text-muted-foreground font-medium">容器端口</th>
                <th className="text-left py-2 text-muted-foreground font-medium">宿主端口</th>
                <th className="text-left py-2 text-muted-foreground font-medium">协议</th>
                <th className="text-left py-2 text-muted-foreground font-medium">描述</th>
                <th className="text-right py-2 text-muted-foreground font-medium">操作</th>
              </tr>
            </thead>
            <tbody>
              {portMappings.map(pm => (
                <tr key={pm.id} className="border-b border-gray-50">
                  <td className="py-2 font-number">{pm.container_port}</td>
                  <td className="py-2 font-number">{pm.host_port}</td>
                  <td className="py-2">{pm.protocol.toUpperCase()}</td>
                  <td className="py-2 text-muted-foreground">{pm.description || '-'}</td>
                  <td className="py-2 text-right">
                    <button
                      onClick={() => onDelete(pm.id)}
                      className="text-red-500 hover:text-red-700"
                    >
                      <Trash2 size={14} />
                    </button>
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
