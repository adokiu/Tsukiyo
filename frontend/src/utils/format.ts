export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`
}

export function formatSpeed(bytes: number): string {
  return `${formatBytes(bytes)}/s`
}

export function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
}

export function formatTimeByPeriod(ts: string, period: string): string {
  const d = new Date(ts)
  if (period === '1m' || period === '15m') {
    return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  }
  if (period === '1h' || period === '6h') {
    return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  }
  return d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

export function generateRandomPassword(length: number = 16): string {
  const chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*'
  let result = ''
  for (let i = 0; i < length; i++) {
    result += chars.charAt(Math.floor(Math.random() * chars.length))
  }
  return result
}

export function formatArch(arch: string): string {
  switch (arch) {
    case 'x86_64': return 'amd64'
    case 'aarch64': return 'arm64'
    default: return arch
  }
}

const STATUS_LABEL_KEYS: Record<string, string> = {
  running: 'common.running',
  stopped: 'common.stopped',
  starting: 'common.starting',
  stopping: 'common.stopping',
  restarting: 'common.restarting',
  creating: 'common.creating',
  deleting: 'common.deleting',
  deleted: 'common.deleted',
  error: 'common.error',
  reinstalling: 'common.reinstalling',
  resizing: 'common.resizing',
}

export function getStatusLabel(status: string, t: (key: string) => string): string {
  const key = STATUS_LABEL_KEYS[status]
  return key ? t(key) : status
}
