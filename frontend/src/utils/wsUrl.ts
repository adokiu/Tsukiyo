/** 从持久化 auth store 读取 token，供管理端 WebSocket 鉴权 */
export function getAuthToken(): string | null {
  try {
    const raw = localStorage.getItem('tsukiyo-auth')
    if (!raw) return null
    const parsed = JSON.parse(raw) as { state?: { token?: string | null } }
    return parsed.state?.token ?? null
  } catch {
    return null
  }
}

export function buildAuthenticatedWebSocketUrl(path: string): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const base = path.startsWith('ws') ? path : `${proto}//${window.location.host}${path}`
  const token = getAuthToken()
  if (!token) return base
  const sep = base.includes('?') ? '&' : '?'
  return `${base}${sep}token=${encodeURIComponent(token)}`
}
