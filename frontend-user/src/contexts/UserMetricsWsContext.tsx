import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { useAuthStore } from '@/stores/auth'
import { instancesApi, type BatchMetricsItem } from '@/api/instances'
import { buildAuthenticatedWebSocketUrl } from '@/utils/wsUrl'

type MetricsMap = Record<string, BatchMetricsItem>

interface WsMessage {
  type: string
  timestamp: number
  items: BatchMetricsItem[]
}

interface UserMetricsWsContextValue {
  metrics: MetricsMap
  connected: boolean
}

const UserMetricsWsContext = createContext<UserMetricsWsContextValue>({
  metrics: {},
  connected: false,
})

const RECONNECT_BASE_MS = 5000
const RECONNECT_MAX_MS = 60000

export function UserMetricsWsProvider({ children }: { children: ReactNode }) {
  const token = useAuthStore((s) => s.token)
  const [metrics, setMetrics] = useState<MetricsMap>({})
  const [connected, setConnected] = useState(false)
  const wsRef = useRef<WebSocket | null>(null)
  const pollTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const reconnectAttemptRef = useRef(0)
  const intentionalCloseRef = useRef(false)

  const stopPolling = useCallback(() => {
    if (pollTimerRef.current) {
      clearInterval(pollTimerRef.current)
      pollTimerRef.current = null
    }
  }, [])

  const startPolling = useCallback(() => {
    if (pollTimerRef.current) return

    const poll = async () => {
      try {
        const res = await instancesApi.batchMetrics()
        const map: MetricsMap = {}
        for (const item of res.data.items) {
          map[item.instance_id] = item
        }
        setMetrics(map)
      } catch {
        // 静默失败
      }
    }

    poll()
    pollTimerRef.current = setInterval(poll, 5000)
  }, [])

  const scheduleReconnect = useCallback((connect: () => void) => {
    if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current)
    const attempt = reconnectAttemptRef.current
    const delay = Math.min(RECONNECT_BASE_MS * 2 ** attempt, RECONNECT_MAX_MS)
    reconnectTimerRef.current = setTimeout(() => {
      reconnectAttemptRef.current = attempt + 1
      connect()
    }, delay)
  }, [])

  const closeWs = useCallback((intentional = true) => {
    const ws = wsRef.current
    if (!ws) return
    if (intentional) intentionalCloseRef.current = true
    ws.onclose = null
    ws.onerror = null
    ws.close()
    wsRef.current = null
  }, [])

  useEffect(() => {
    intentionalCloseRef.current = false
    reconnectAttemptRef.current = 0

    if (!token) {
      closeWs(true)
      setConnected(false)
      startPolling()
      return () => {
        closeWs()
        stopPolling()
        if (reconnectTimerRef.current) {
          clearTimeout(reconnectTimerRef.current)
          reconnectTimerRef.current = null
        }
      }
    }

    const connect = () => {
      if (intentionalCloseRef.current || !token) return

      closeWs(true)
      intentionalCloseRef.current = false

      const wsUrl = buildAuthenticatedWebSocketUrl('/ws/user/metrics', token)
      let ws: WebSocket
      try {
        ws = new WebSocket(wsUrl)
      } catch {
        startPolling()
        scheduleReconnect(connect)
        return
      }

      wsRef.current = ws
      let opened = false

      ws.onopen = () => {
        opened = true
        reconnectAttemptRef.current = 0
        setConnected(true)
        stopPolling()
      }

      ws.onmessage = (event) => {
        try {
          const msg: WsMessage = JSON.parse(event.data)
          if (msg.type === 'instance_metrics_batch' && msg.items) {
            const map: MetricsMap = {}
            for (const item of msg.items) {
              map[item.instance_id] = item
            }
            setMetrics(map)
          }
        } catch {
          // 忽略
        }
      }

      ws.onclose = () => {
        setConnected(false)
        wsRef.current = null
        if (intentionalCloseRef.current) return
        startPolling()
        // 从未连上过（如误走 Vite 代理）时降低 WS 重试频率，避免 dev 终端刷屏
        if (!opened) {
          reconnectAttemptRef.current = Math.max(reconnectAttemptRef.current, 3)
        }
        if (document.visibilityState === 'visible') {
          scheduleReconnect(connect)
        }
      }

      ws.onerror = () => {
        // onclose 会处理重连
      }
    }

    connect()

    return () => {
      intentionalCloseRef.current = true
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current)
        reconnectTimerRef.current = null
      }
      closeWs()
      stopPolling()
    }
  }, [token, closeWs, startPolling, stopPolling, scheduleReconnect])

  const value = useMemo(
    () => ({ metrics, connected }),
    [metrics, connected],
  )

  return (
    <UserMetricsWsContext.Provider value={value}>
      {children}
    </UserMetricsWsContext.Provider>
  )
}

export function useUserMetricsWs() {
  return useContext(UserMetricsWsContext)
}

/** @deprecated 请用 useUserMetricsWs；保留别名避免大范围改动 */
export function useInstanceMetrics() {
  return useUserMetricsWs()
}
