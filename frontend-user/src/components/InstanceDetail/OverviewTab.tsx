import {
  Cpu, MemoryStick, HardDrive, Gauge, Server, Network,
  Eye, EyeOff, Copy, Lock
} from 'lucide-react'
import { formatBytes, stripCIDR, formatImageName, formatTrafficMode } from '@/utils/format'
import { InstanceDetail, InstanceMetricsData } from '@/api/instances'

interface OverviewTabProps {
  instance: InstanceDetail
  metrics: InstanceMetricsData | null
  cpuPercent: number
  memPercent: number
  diskTotalBytes: number
  diskPercent: number
  showPassword: boolean
  setShowPassword: (v: boolean) => void
  copyToClipboard: (text: string) => void
  onResetPassword: () => void
  isBusy: boolean
}

export function OverviewTab({
  instance, metrics, cpuPercent, memPercent, diskTotalBytes, diskPercent,
  showPassword, setShowPassword, copyToClipboard, onResetPassword, isBusy,
}: OverviewTabProps) {
  const sshAddress = stripCIDR(instance.ipv4_eip || instance.internal_ipv4 || '')
  const sshPort = instance.ssh_port || 22

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div className="card p-4">
          <div className="flex items-center gap-2 text-muted-foreground mb-2">
            <Cpu size={16} />
            <span className="text-sm">CPU</span>
          </div>
          <div className="text-2xl font-semibold font-number">{instance.vcpu} <span className="text-sm font-normal text-muted-foreground">核</span></div>
          {metrics?.cpu_usage !== undefined && (
            <div className="mt-2">
              <div className="text-xs text-muted-foreground mb-1">使用率: {cpuPercent.toFixed(1)}%</div>
              <div className="h-1.5 bg-gray-100 rounded-full overflow-hidden">
                <div className="h-full bg-gradient-to-r from-teal-500 to-orange-500 rounded-full" style={{ width: `${Math.min(cpuPercent, 100)}%` }} />
              </div>
            </div>
          )}
        </div>
        <div className="card p-4">
          <div className="flex items-center gap-2 text-muted-foreground mb-2">
            <MemoryStick size={16} />
            <span className="text-sm">内存</span>
          </div>
          <div className="text-2xl font-semibold font-number">{instance.memory_mb} <span className="text-sm font-normal text-muted-foreground">MB</span></div>
          {instance.swap_mb > 0 && (
            <div className="text-xs text-muted-foreground mt-1">Swap: {instance.swap_mb} MB</div>
          )}
          {metrics?.memory_usage !== undefined && metrics?.memory_total !== undefined && (
            <div className="mt-2">
              <div className="text-xs text-muted-foreground mb-1">已用: {formatBytes(metrics.memory_usage)} / {formatBytes(metrics.memory_total)}</div>
              <div className="h-1.5 bg-gray-100 rounded-full overflow-hidden">
                <div className="h-full bg-gradient-to-r from-teal-500 to-orange-500 rounded-full" style={{ width: `${Math.min(memPercent, 100)}%` }} />
              </div>
            </div>
          )}
        </div>
        <div className="card p-4">
          <div className="flex items-center gap-2 text-muted-foreground mb-2">
            <HardDrive size={16} />
            <span className="text-sm">系统盘</span>
          </div>
          <div className="text-2xl font-semibold font-number">{(instance.disk_mb / 1024).toFixed(0)} <span className="text-sm font-normal text-muted-foreground">GB</span></div>
          {metrics?.disk_used !== undefined && (
            <div className="mt-2">
              <div className="text-xs text-muted-foreground mb-1">
                已用: {formatBytes(metrics.disk_used)} / {diskTotalBytes ? formatBytes(diskTotalBytes) : `${(instance.disk_mb / 1024).toFixed(0)} GB`}
              </div>
              <div className="h-1.5 bg-gray-100 rounded-full overflow-hidden">
                <div className="h-full bg-gradient-to-r from-teal-500 to-orange-500 rounded-full" style={{ width: `${Math.min(diskPercent, 100)}%` }} />
              </div>
            </div>
          )}
        </div>
        <div className="card p-4">
          <div className="flex items-center gap-2 text-muted-foreground mb-2">
            <Gauge size={16} />
            <span className="text-sm">流量</span>
          </div>
          {(() => {
            const usedGB = metrics?.traffic_used_gb ?? instance.traffic_used_gb ?? 0
            const totalGB = metrics?.monthly_traffic ?? instance.monthly_traffic ?? 0
            const percent = totalGB > 0 ? (usedGB / totalGB * 100) : 0
            return (
              <>
                <div className="text-2xl font-semibold font-number">
                  {usedGB.toFixed(2)} <span className="text-sm font-normal text-muted-foreground">/ {totalGB} GB</span>
                </div>
                <div className="mt-2">
                  <div className="text-xs text-muted-foreground mb-1">已用 {percent.toFixed(1)}%</div>
                  <div className="h-1.5 bg-gray-100 rounded-full overflow-hidden">
                    <div className="h-full bg-gradient-to-r from-teal-500 to-orange-500 rounded-full" style={{ width: `${Math.min(percent, 100)}%` }} />
                  </div>
                </div>
              </>
            )
          })()}
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div className="card p-5 space-y-4">
          <h3 className="text-sm font-semibold flex items-center gap-2">
            <Server size={16} /> 实例信息
          </h3>
          <div className="grid grid-cols-2 gap-y-3 text-sm">
            <div className="text-muted-foreground">宿主机</div>
            <div>{instance.node_name || instance.node_id.slice(0, 8)}</div>
            <div className="text-muted-foreground">系统镜像</div>
            <div>{formatImageName(instance.template_id)}</div>
            <div className="text-muted-foreground">存储池</div>
            <div>{instance.storage_pool || 'default'}</div>
            <div className="text-muted-foreground">磁盘IO限制</div>
            <div>读 {instance.io_read_iops || 0} IOPS / 写 {instance.io_write_iops || 0} IOPS</div>
            <div className="text-muted-foreground">创建时间</div>
            <div>{instance.created_at ? new Date(instance.created_at).toLocaleString() : '-'}</div>
            <div className="text-muted-foreground">到期时间</div>
            <div>{instance.expires_at ? new Date(instance.expires_at).toLocaleString() : '长期有效'}</div>
            <div className="text-muted-foreground">流量模式</div>
            <div>{formatTrafficMode(instance.traffic_mode)}</div>
            <div className="text-muted-foreground">月流量</div>
            <div>{instance.monthly_traffic || 0} GB</div>
            <div className="text-muted-foreground">超限策略</div>
            <div>{instance.over_limit_action === 'throttle' ? `限速 ${instance.throttle_mbps || 1} Mbps` : '直接关机'}</div>
            <div className="text-muted-foreground">流量状态</div>
            <div>{instance.is_over_limit ? <span className="text-red-500">已超限</span> : <span className="text-green-500">正常</span>}</div>
          </div>
        </div>

        <div className="card p-5 space-y-4">
          <h3 className="text-sm font-semibold flex items-center gap-2">
            <Network size={16} /> 连接信息
          </h3>

          <div>
            <div className="flex items-center justify-between mb-1">
              <span className="text-sm text-muted-foreground">SSH 连接地址</span>
              <button onClick={() => copyToClipboard(`${sshAddress}:${sshPort}`)} className="text-muted hover:text-foreground">
                <Copy size={14} />
              </button>
            </div>
            <div className="font-mono text-sm bg-gray-50 px-3 py-2 rounded-lg">{sshAddress ? `${sshAddress}:${sshPort}` : '-'}</div>
          </div>

          <div>
            <div className="flex items-center justify-between mb-1">
              <span className="text-sm text-muted-foreground">用户名</span>
              <button onClick={() => copyToClipboard('root')} className="text-muted hover:text-foreground">
                <Copy size={14} />
              </button>
            </div>
            <div className="font-mono text-sm bg-gray-50 px-3 py-2 rounded-lg">root</div>
          </div>

          <div>
            <div className="flex items-center justify-between mb-1">
              <span className="text-sm text-muted-foreground">SSH 密码</span>
              <div className="flex items-center gap-2">
                <button onClick={() => setShowPassword(!showPassword)} className="text-muted hover:text-foreground">
                  {showPassword ? <EyeOff size={14} /> : <Eye size={14} />}
                </button>
                {instance.ssh_password && (
                  <button onClick={() => copyToClipboard(instance.ssh_password!)} className="text-muted hover:text-foreground">
                    <Copy size={14} />
                  </button>
                )}
                <button
                  onClick={onResetPassword}
                  disabled={isBusy}
                  className="text-xs text-blue-600 hover:text-blue-800 disabled:text-muted"
                >
                  <Lock size={12} className="inline mr-1" />重置
                </button>
              </div>
            </div>
            <div className="font-mono text-sm bg-gray-50 px-3 py-2 rounded-lg">
              {showPassword ? (instance.ssh_password || '-') : '--------'}
            </div>
          </div>

          <div className="pt-3 border-t border-gray-100">
            <div className="grid grid-cols-2 gap-y-2 text-sm">
              <div className="text-muted-foreground">内网 IPv4</div>
              <div className="font-mono">{stripCIDR(instance.internal_ipv4 || '') || '-'}</div>
              <div className="text-muted-foreground">公网 IPv4 (EIP)</div>
              <div className="font-mono">{stripCIDR(instance.ipv4_eip || '') || '-'}{instance.ipv4_eip_alias ? ` (${instance.ipv4_eip_alias})` : ''}</div>
              <div className="text-muted-foreground">公网 IPv6 (EIP)</div>
              <div className="font-mono">{stripCIDR(instance.ipv6_eip || '') || '-'}{instance.ipv6_eip_alias ? ` (${instance.ipv6_eip_alias})` : ''}</div>
            </div>
          </div>

          {instance.bridge_id && (
            <div className="pt-3 border-t border-gray-100">
              <div className="text-sm font-medium mb-2">Bridge 网络</div>
              <div className="grid grid-cols-2 gap-y-2 text-sm">
                <div className="text-muted-foreground">名称</div>
                <div>{instance.bridge_name || '-'}</div>
                <div className="text-muted-foreground">接口</div>
                <div className="font-mono">{instance.bridge_iface || '-'}</div>
                <div className="text-muted-foreground">CIDR</div>
                <div className="font-mono">{instance.bridge_cidr || '-'}</div>
                <div className="text-muted-foreground">网关</div>
                <div className="font-mono">{instance.bridge_gateway || '-'}</div>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
