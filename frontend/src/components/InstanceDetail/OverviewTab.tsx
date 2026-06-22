import {
  Cpu, MemoryStick, HardDrive, Gauge, Server, Network,
  Eye, EyeOff, Copy, Lock
} from 'lucide-react'
import { formatBytes } from '@/utils/format'

interface InstanceData {
  id: string
  name: string
  incus_name: string
  type: string
  status: string
  vcpu: number
  memory_mb: number
  swap_mb: number
  disk_mb: number
  storage_pool: string
  node_name?: string
  node_id: string
  owner_name?: string
  template_id: string
  io_read_iops: number
  io_write_iops: number
  created_at: string
  expires_at: string
  traffic_mode: string
  monthly_traffic: number
  over_limit_action: string
  throttle_mbps: number
  is_over_limit: boolean
  ssh_password: string
  internal_ipv4?: string
  ipv4_eip?: string
  ipv4_eip_alias?: string
  ipv6_eip?: string
  ipv6_eip_alias?: string
  bridge_id?: string
  bridge_name?: string
  bridge_iface?: string
  bridge_cidr?: string
  bridge_gateway?: string
  traffic_used_gb?: number
}

interface MetricsData {
  cpu_usage?: number
  memory_usage?: number
  memory_total?: number
  disk_used?: number
  disk_total?: number
  traffic_used_gb?: number
  monthly_traffic?: number
}

interface OverviewTabProps {
  instance: InstanceData
  metrics: MetricsData | null
  cpuPercent: number
  memPercent: number
  diskTotalBytes: number
  diskPercent: number
  showPassword: boolean
  setShowPassword: (v: boolean) => void
  copyToClipboard: (text: string) => void
  onResetPassword: () => void
  isBanned: boolean
  isExpired: boolean
  isBusy: boolean
}

function stripCIDR(ip: string): string {
  if (!ip) return ''
  const idx = ip.indexOf('/')
  if (idx === -1) return ip
  const prefix = ip.substring(idx + 1)
  if (prefix === '32' || prefix === '128') return ip.substring(0, idx)
  return ip
}

function formatImageName(templateId: string): string {
  if (!templateId) return '-'
  const parts = templateId.split('/')
  if (parts.length >= 2) {
    const distro = parts[0].charAt(0).toUpperCase() + parts[0].slice(1)
    const version = parts[1]
    return `${distro} ${version}`
  }
  return templateId
}

function formatTrafficMode(mode: string): string {
  switch (mode) {
    case 'total': return '总计（入站+出站）'
    case 'inbound': return '入站'
    case 'outbound': return '出站'
    case 'max': return '最大值（入站/出站取大）'
    default: return mode || '-'
  }
}

export function OverviewTab({
  instance, metrics, cpuPercent, memPercent, diskTotalBytes, diskPercent,
  showPassword, setShowPassword, copyToClipboard, onResetPassword,
  isBanned, isExpired, isBusy,
}: OverviewTabProps) {
  const sshAddress = stripCIDR(instance.ipv4_eip || instance.internal_ipv4 || '')
  const sshPort = (instance as any).ssh_port || 22

  return (
    <div className="space-y-4">
      {/* 资源配置卡片 */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div className="bg-surface rounded-xl border border-surface p-4">
          <div className="flex items-center gap-2 text-tertiary mb-2">
            <Cpu size={16} />
            <span className="text-sm">CPU</span>
          </div>
          <div className="text-2xl font-semibold">{instance.vcpu} <span className="text-sm font-normal text-tertiary">核</span></div>
          {metrics?.cpu_usage !== undefined && (
            <div className="mt-2">
              <div className="text-xs text-tertiary mb-1">使用率: {cpuPercent.toFixed(1)}%</div>
              <div className="h-1.5 bg-surface-secondary rounded-full overflow-hidden">
                <div className="h-full bg-gradient-to-r from-teal-500 to-orange-500 rounded-full" style={{ width: `${Math.min(cpuPercent, 100)}%` }} />
              </div>
            </div>
          )}
        </div>
        <div className="bg-surface rounded-xl border border-surface p-4">
          <div className="flex items-center gap-2 text-tertiary mb-2">
            <MemoryStick size={16} />
            <span className="text-sm">内存</span>
          </div>
          <div className="text-2xl font-semibold">{instance.memory_mb} <span className="text-sm font-normal text-tertiary">MB</span></div>
          {instance.swap_mb > 0 && (
            <div className="text-xs text-tertiary mt-1">Swap: {instance.swap_mb} MB</div>
          )}
          {metrics?.memory_usage !== undefined && metrics?.memory_total !== undefined && (
            <div className="mt-2">
              <div className="text-xs text-tertiary mb-1">已用: {formatBytes(metrics.memory_usage)} / {formatBytes(metrics.memory_total)}</div>
              <div className="h-1.5 bg-surface-secondary rounded-full overflow-hidden">
                <div className="h-full bg-gradient-to-r from-teal-500 to-orange-500 rounded-full" style={{ width: `${Math.min(memPercent, 100)}%` }} />
              </div>
            </div>
          )}
        </div>
        <div className="bg-surface rounded-xl border border-surface p-4">
          <div className="flex items-center gap-2 text-tertiary mb-2">
            <HardDrive size={16} />
            <span className="text-sm">系统盘</span>
          </div>
          <div className="text-2xl font-semibold">{(instance.disk_mb / 1024).toFixed(0)} <span className="text-sm font-normal text-tertiary">GB</span></div>
          {metrics?.disk_used !== undefined && (
            <div className="mt-2">
              <div className="text-xs text-tertiary mb-1">
                已用: {formatBytes(metrics.disk_used)} / {diskTotalBytes ? formatBytes(diskTotalBytes) : `${(instance.disk_mb / 1024).toFixed(0)} GB`}
              </div>
              <div className="h-1.5 bg-surface-secondary rounded-full overflow-hidden">
                <div className="h-full bg-gradient-to-r from-teal-500 to-orange-500 rounded-full" style={{ width: `${Math.min(diskPercent, 100)}%` }} />
              </div>
            </div>
          )}
        </div>
        <div className="bg-surface rounded-xl border border-surface p-4">
          <div className="flex items-center gap-2 text-tertiary mb-2">
            <Gauge size={16} />
            <span className="text-sm">流量</span>
          </div>
          {(() => {
            const usedGB = metrics?.traffic_used_gb ?? instance.traffic_used_gb ?? 0
            const totalGB = metrics?.monthly_traffic ?? instance.monthly_traffic ?? 0
            const percent = totalGB > 0 ? (usedGB / totalGB * 100) : 0
            return (
              <>
                <div className="text-2xl font-semibold">
                  {usedGB.toFixed(2)} <span className="text-sm font-normal text-tertiary">/ {totalGB} GB</span>
                </div>
                <div className="mt-2">
                  <div className="text-xs text-tertiary mb-1">已用 {percent.toFixed(1)}%</div>
                  <div className="h-1.5 bg-surface-secondary rounded-full overflow-hidden">
                    <div className="h-full bg-gradient-to-r from-teal-500 to-orange-500 rounded-full" style={{ width: `${Math.min(percent, 100)}%` }} />
                  </div>
                </div>
              </>
            )
          })()}
        </div>
      </div>

      <div className="grid grid-cols-2 gap-6">
        {/* 实例信息 */}
        <div className="bg-surface rounded-xl border border-surface p-5 space-y-4">
          <h3 className="text-sm font-semibold text-primary flex items-center gap-2">
            <Server size={16} /> 实例信息
          </h3>
          <div className="grid grid-cols-2 gap-y-3 text-sm">
            <div className="text-tertiary">宿主机</div>
            <div>{instance.node_name || instance.node_id.slice(0, 8)}</div>
            <div className="text-tertiary">所有者</div>
            <div>{instance.owner_name || '-'}</div>
            <div className="text-tertiary">系统镜像</div>
            <div>{formatImageName(instance.template_id)}</div>
            <div className="text-tertiary">存储池</div>
            <div>{instance.storage_pool || 'default'}</div>
            <div className="text-tertiary">磁盘IO限制</div>
            <div>读 {instance.io_read_iops || 0} IOPS / 写 {instance.io_write_iops || 0} IOPS</div>
            <div className="text-tertiary">创建时间</div>
            <div>{instance.created_at ? new Date(instance.created_at).toLocaleString() : '-'}</div>
            <div className="text-tertiary">到期时间</div>
            <div>{instance.expires_at ? new Date(instance.expires_at).toLocaleString() : '-'}</div>
            <div className="text-tertiary">流量模式</div>
            <div>{formatTrafficMode(instance.traffic_mode)}</div>
            <div className="text-tertiary">月流量</div>
            <div>{instance.monthly_traffic || 0} GB</div>
            <div className="text-tertiary">超限策略</div>
            <div>{instance.over_limit_action === 'throttle' ? `限速 ${instance.throttle_mbps || 1} Mbps` : '直接关机'}</div>
            <div className="text-tertiary">流量状态</div>
            <div>{instance.is_over_limit ? <span className="text-red-500">已超限</span> : <span className="text-green-500">正常</span>}</div>
          </div>
        </div>

        {/* 连接信息 */}
        <div className="bg-surface rounded-xl border border-surface p-5 space-y-4">
          <h3 className="text-sm font-semibold text-primary flex items-center gap-2">
            <Network size={16} /> 连接信息
          </h3>

          {/* SSH 地址 */}
          <div>
            <div className="flex items-center justify-between mb-1">
              <span className="text-sm text-tertiary">SSH 连接地址</span>
              <button onClick={() => copyToClipboard(`${sshAddress}:${sshPort}`)} className="text-muted hover:text-tertiary">
                <Copy size={14} />
              </button>
            </div>
            <div className="font-mono text-sm bg-surface-secondary px-3 py-2 rounded-lg">{sshAddress ? `${sshAddress}:${sshPort}` : '-'}</div>
          </div>

          {/* 用户名 */}
          <div>
            <div className="flex items-center justify-between mb-1">
              <span className="text-sm text-tertiary">用户名</span>
              <button onClick={() => copyToClipboard('root')} className="text-muted hover:text-tertiary">
                <Copy size={14} />
              </button>
            </div>
            <div className="font-mono text-sm bg-surface-secondary px-3 py-2 rounded-lg">root</div>
          </div>

          {/* 密码 */}
          <div>
            <div className="flex items-center justify-between mb-1">
              <span className="text-sm text-tertiary">SSH 密码</span>
              <div className="flex items-center gap-2">
                <button onClick={() => setShowPassword(!showPassword)} className="text-muted hover:text-tertiary">
                  {showPassword ? <EyeOff size={14} /> : <Eye size={14} />}
                </button>
                {instance.ssh_password && (
                  <button onClick={() => copyToClipboard(instance.ssh_password!)} className="text-muted hover:text-tertiary">
                    <Copy size={14} />
                  </button>
                )}
                <button
                  onClick={onResetPassword}
                  disabled={isBanned || isExpired || isBusy}
                  className="text-xs text-blue-600 hover:text-blue-800 disabled:text-muted"
                >
                  <Lock size={12} className="inline mr-1" />重置
                </button>
              </div>
            </div>
            <div className="font-mono text-sm bg-surface-secondary px-3 py-2 rounded-lg">
              {showPassword ? (instance.ssh_password || '-') : '••••••••'}
            </div>
          </div>

          {/* 网络信息 */}
          <div className="pt-3 border-t border-surface-light">
            <div className="grid grid-cols-2 gap-y-2 text-sm">
              <div className="text-tertiary">内网 IPv4</div>
              <div className="font-mono">{stripCIDR(instance.internal_ipv4 || '') || '-'}</div>
              <div className="text-tertiary">公网 IPv4 (EIP)</div>
              <div className="font-mono">{stripCIDR(instance.ipv4_eip || '') || '-'}{instance.ipv4_eip_alias ? ` (${instance.ipv4_eip_alias})` : ''}</div>
              <div className="text-tertiary">公网 IPv6 (EIP)</div>
              <div className="font-mono">{stripCIDR(instance.ipv6_eip || '') || '-'}{instance.ipv6_eip_alias ? ` (${instance.ipv6_eip_alias})` : ''}</div>
            </div>
          </div>

          {instance.bridge_id && (
            <div className="pt-3 border-t border-surface-light">
              <div className="text-sm font-medium text-primary mb-2">Bridge 网络</div>
              <div className="grid grid-cols-2 gap-y-2 text-sm">
                <div className="text-tertiary">名称</div>
                <div>{instance.bridge_name || '-'}</div>
                <div className="text-tertiary">接口</div>
                <div className="font-mono">{instance.bridge_iface || '-'}</div>
                <div className="text-tertiary">CIDR</div>
                <div className="font-mono">{instance.bridge_cidr || '-'}</div>
                <div className="text-tertiary">网关</div>
                <div className="font-mono">{instance.bridge_gateway || '-'}</div>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
