import {
  AreaChart, Area, LineChart, Line, XAxis, YAxis,
  CartesianGrid, Tooltip, ResponsiveContainer
} from 'recharts'
import { formatBytes, formatSpeed, formatTimeByPeriod } from '@/utils/format'
import { InstanceMetricsData, MetricPoint } from '@/api/instances'

const TIME_RANGES = [
  { key: '1m', label: '实时' },
  { key: '15m', label: '15分钟' },
  { key: '1h', label: '1小时' },
  { key: '1d', label: '1天' },
  { key: '7d', label: '7天' },
]

const tooltipStyle = {
  backgroundColor: 'rgba(255,255,255,0.98)',
  border: '1px solid #e5e7eb',
  borderRadius: '8px',
  fontSize: '12px',
  boxShadow: '0 2px 8px rgba(0,0,0,0.08)',
}

interface MonitoringTabProps {
  metrics: InstanceMetricsData | null
  metricHistory: MetricPoint[]
  metricPeriod: string
  setMetricPeriod: (p: string) => void
  metricLoading: boolean
  cpuPercent: number
  memPercent: number
}

export function MonitoringTab({
  metrics, metricHistory, metricPeriod, setMetricPeriod, metricLoading,
  cpuPercent, memPercent,
}: MonitoringTabProps) {
  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div className="card p-4">
          <div className="text-xs uppercase tracking-widest text-muted-foreground font-semibold">CPU 使用率</div>
          <div className="text-2xl font-bold mt-2 font-number">{cpuPercent.toFixed(1)}%</div>
          <div className="h-1.5 bg-gray-100 rounded-full overflow-hidden mt-3">
            <div className="h-full bg-gradient-to-r from-teal-500 to-orange-500 rounded-full" style={{ width: `${Math.min(cpuPercent, 100)}%` }} />
          </div>
        </div>
        <div className="card p-4">
          <div className="text-xs uppercase tracking-widest text-muted-foreground font-semibold">内存使用率</div>
          <div className="text-2xl font-bold mt-2 font-number">{memPercent.toFixed(1)}%</div>
          <div className="text-xs text-muted-foreground mt-1">{formatBytes(metrics?.memory_usage || 0)} / {formatBytes(metrics?.memory_total || 0)}</div>
          <div className="h-1.5 bg-gray-100 rounded-full overflow-hidden mt-3">
            <div className="h-full bg-gradient-to-r from-teal-500 to-orange-500 rounded-full" style={{ width: `${Math.min(memPercent, 100)}%` }} />
          </div>
        </div>
        <div className="card p-4">
          <div className="text-xs uppercase tracking-widest text-muted-foreground font-semibold">磁盘 IO</div>
          <div className="flex items-end gap-4 mt-2">
            <div>
              <div className="text-xs text-muted-foreground">读</div>
              <div className="text-lg font-bold font-number">{formatSpeed(metrics?.disk_read_bps || 0)}</div>
              <div className="text-xs text-muted-foreground">{metrics?.disk_read_iops || 0} IOPS</div>
            </div>
            <div>
              <div className="text-xs text-muted-foreground">写</div>
              <div className="text-lg font-bold font-number">{formatSpeed(metrics?.disk_write_bps || 0)}</div>
              <div className="text-xs text-muted-foreground">{metrics?.disk_write_iops || 0} IOPS</div>
            </div>
          </div>
        </div>
        <div className="card p-4">
          <div className="text-xs uppercase tracking-widest text-muted-foreground font-semibold">网络 IO</div>
          <div className="text-2xl font-bold mt-2 font-number">{formatSpeed(metrics?.network_rx || 0)}</div>
          <div className="text-xs text-muted-foreground mt-1">上传: {formatSpeed(metrics?.network_tx || 0)}</div>
        </div>
      </div>

      <div className="card p-5">
        <div className="flex items-center justify-between mb-4">
          <div className="text-sm font-bold">资源使用图表</div>
          <div className="flex gap-1 bg-gray-100 rounded-full p-1">
            {TIME_RANGES.map(p => (
              <button
                key={p.key}
                onClick={() => setMetricPeriod(p.key)}
                className={`px-3 py-1 text-xs font-semibold rounded-full transition-colors ${
                  metricPeriod === p.key
                    ? 'bg-blue-600 text-white'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
              >
                {p.label}
              </button>
            ))}
          </div>
        </div>

        {metricLoading && metricHistory.length === 0 ? (
          <div className="h-64 flex items-center justify-center">
            <div className="animate-spin rounded-full h-6 w-6 border-2 border-blue-600 border-t-transparent" />
          </div>
        ) : metricHistory.length === 0 ? (
          <div className="h-64 flex items-center justify-center text-muted-foreground text-sm">
            暂无监控数据
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="card p-4">
              <div className="text-sm font-semibold mb-2">CPU 使用率</div>
              <div className="h-44">
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart data={metricHistory} margin={{ top: 5, right: 5, bottom: 0, left: 0 }}>
                    <defs>
                      <linearGradient id="cpuGradient" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.3} />
                        <stop offset="95%" stopColor="#3b82f6" stopOpacity={0} />
                      </linearGradient>
                    </defs>
                    <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
                    <XAxis dataKey="timestamp" tickFormatter={(t) => formatTimeByPeriod(t, metricPeriod)} tick={{ fontSize: 10 }} stroke="#888" />
                    <YAxis domain={[0, 100]} tickFormatter={(v) => `${v}%`} tick={{ fontSize: 10 }} stroke="#888" width={40} />
                    <Tooltip
                      formatter={(value: any) => [`${Number(value).toFixed(1)}%`, 'CPU']}
                      labelFormatter={(label) => new Date(label).toLocaleString('zh-CN')}
                      contentStyle={tooltipStyle}
                    />
                    <Area type="monotone" dataKey="cpu" stroke="#3b82f6" fill="url(#cpuGradient)" strokeWidth={2} />
                  </AreaChart>
                </ResponsiveContainer>
              </div>
            </div>

            <div className="card p-4">
              <div className="text-sm font-semibold mb-2">内存使用</div>
              <div className="h-44">
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart data={metricHistory} margin={{ top: 5, right: 5, bottom: 0, left: 0 }}>
                    <defs>
                      <linearGradient id="memGradient" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="5%" stopColor="#8b5cf6" stopOpacity={0.3} />
                        <stop offset="95%" stopColor="#8b5cf6" stopOpacity={0} />
                      </linearGradient>
                    </defs>
                    <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
                    <XAxis dataKey="timestamp" tickFormatter={(t) => formatTimeByPeriod(t, metricPeriod)} tick={{ fontSize: 10 }} stroke="#888" />
                    <YAxis domain={[0, 'auto']} tickFormatter={(v) => formatBytes(v)} tick={{ fontSize: 10 }} stroke="#888" width={50} />
                    <Tooltip
                      formatter={(value: any) => [formatBytes(Number(value)), '内存']}
                      labelFormatter={(label) => new Date(label).toLocaleString('zh-CN')}
                      contentStyle={tooltipStyle}
                    />
                    <Area type="monotone" dataKey="mem_used" stroke="#8b5cf6" fill="url(#memGradient)" strokeWidth={2} />
                  </AreaChart>
                </ResponsiveContainer>
              </div>
            </div>

            <div className="card p-4">
              <div className="flex items-center justify-between mb-2">
                <span className="text-sm font-semibold">磁盘 IO 速度</span>
                <div className="flex gap-2 text-xs text-muted-foreground">
                  <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-blue-500" />读</span>
                  <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-orange-500" />写</span>
                </div>
              </div>
              <div className="h-32">
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={metricHistory} margin={{ top: 5, right: 5, bottom: 0, left: 0 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
                    <XAxis dataKey="timestamp" tickFormatter={(t) => formatTimeByPeriod(t, metricPeriod)} tick={{ fontSize: 10 }} stroke="#888" />
                    <YAxis domain={[0, 'auto']} tickFormatter={(v) => formatSpeed(v)} tick={{ fontSize: 10 }} stroke="#888" width={55} />
                    <Tooltip
                      formatter={(value: any, name: any) => [formatSpeed(Number(value)), name === 'disk_read_bps' ? '读速度' : '写速度']}
                      labelFormatter={(label) => new Date(label).toLocaleString('zh-CN')}
                      contentStyle={tooltipStyle}
                    />
                    <Line type="monotone" dataKey="disk_read_bps" stroke="#3b82f6" strokeWidth={2} dot={false} />
                    <Line type="monotone" dataKey="disk_write_bps" stroke="#f97316" strokeWidth={2} dot={false} />
                  </LineChart>
                </ResponsiveContainer>
              </div>
              <div className="flex items-center justify-between mb-2 mt-3">
                <span className="text-sm font-semibold">磁盘 IOPS</span>
                <div className="flex gap-2 text-xs text-muted-foreground">
                  <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-blue-500" />读</span>
                  <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-orange-500" />写</span>
                </div>
              </div>
              <div className="h-32">
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={metricHistory} margin={{ top: 5, right: 5, bottom: 0, left: 0 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
                    <XAxis dataKey="timestamp" tickFormatter={(t) => formatTimeByPeriod(t, metricPeriod)} tick={{ fontSize: 10 }} stroke="#888" />
                    <YAxis domain={[0, 'auto']} tickFormatter={(v) => `${v} IOPS`} tick={{ fontSize: 10 }} stroke="#888" width={55} />
                    <Tooltip
                      formatter={(value: any, name: any) => [`${Number(value)} IOPS`, name === 'disk_read_iops' ? '读IOPS' : '写IOPS']}
                      labelFormatter={(label) => new Date(label).toLocaleString('zh-CN')}
                      contentStyle={tooltipStyle}
                    />
                    <Line type="monotone" dataKey="disk_read_iops" stroke="#3b82f6" strokeWidth={2} dot={false} />
                    <Line type="monotone" dataKey="disk_write_iops" stroke="#f97316" strokeWidth={2} dot={false} />
                  </LineChart>
                </ResponsiveContainer>
              </div>
            </div>

            <div className="card p-4">
              <div className="flex items-center justify-between mb-2">
                <span className="text-sm font-semibold">网络 IO</span>
                <div className="flex gap-2 text-xs text-muted-foreground">
                  <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-teal-500" />下载</span>
                  <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-orange-500" />上传</span>
                </div>
              </div>
              <div className="h-44">
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={metricHistory} margin={{ top: 5, right: 5, bottom: 0, left: 0 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
                    <XAxis dataKey="timestamp" tickFormatter={(t) => formatTimeByPeriod(t, metricPeriod)} tick={{ fontSize: 10 }} stroke="#888" />
                    <YAxis domain={[0, 'auto']} tickFormatter={(v) => formatSpeed(v)} tick={{ fontSize: 10 }} stroke="#888" width={55} />
                    <Tooltip
                      formatter={(value: any, name: any) => [formatSpeed(Number(value)), name === 'net_in' ? '下载' : '上传']}
                      labelFormatter={(label) => new Date(label).toLocaleString('zh-CN')}
                      contentStyle={tooltipStyle}
                    />
                    <Line type="monotone" dataKey="net_in" stroke="#14b8a6" strokeWidth={2} dot={false} />
                    <Line type="monotone" dataKey="net_out" stroke="#f97316" strokeWidth={2} dot={false} />
                  </LineChart>
                </ResponsiveContainer>
              </div>
            </div>
          </div>
        )}
        <div className="text-right text-xs text-muted-foreground mt-3">
          {metricPeriod === '1m' ? 'WebSocket 实时刷新（每秒）' : '30秒自动刷新'} · 最后更新: {new Date().toLocaleTimeString()}
        </div>
      </div>
    </div>
  )
}
