export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  if (!bytes || bytes < 0) return '-'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`
}

export function formatSpeed(bytes: number): string {
  return `${formatBytes(bytes)}/s`
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

export { stripCIDR, formatImageName, formatTrafficMode }
