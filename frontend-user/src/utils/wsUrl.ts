import { useAuthStore } from '@/stores/auth'

const DEV_FRONTEND_PORTS = new Set(['5174', '5173'])

function useDirectMasterWebSocket(): boolean {
  if (import.meta.env.VITE_WS_DIRECT === '0') return false
  if (import.meta.env.VITE_WS_DIRECT === '1') return true
  if (import.meta.env.PROD) return false
  if (import.meta.env.DEV) return true
  if (typeof window !== 'undefined' && DEV_FRONTEND_PORTS.has(window.location.port)) {
    return true
  }
  return false
}

/** 开发环境 WebSocket 直连 Master，避免 Vite /ws 代理 ECONNRESET 日志 */
export function getWebSocketHost(): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  if (useDirectMasterWebSocket()) {
    const origin = import.meta.env.VITE_MASTER_WS_ORIGIN || 'localhost:8080'
    return `${proto}//${origin.replace(/^wss?:\/\//, '')}`
  }
  return `${proto}//${window.location.host}`
}

export function getAuthToken(): string | null {
  return useAuthStore.getState().token
}

export function buildWebSocketUrl(path: string, query?: Record<string, string>): string {
  const host = getWebSocketHost()
  const normalized = path.startsWith('/') ? path : `/${path}`
  let url = `${host}${normalized}`
  if (query && Object.keys(query).length > 0) {
    const params = new URLSearchParams(query)
    url += `?${params.toString()}`
  }
  return url
}

export function buildAuthenticatedWebSocketUrl(path: string, token?: string | null): string {
  const t = token ?? getAuthToken()
  if (!t) {
    return buildWebSocketUrl(path)
  }
  return buildWebSocketUrl(path, { token: t })
}
