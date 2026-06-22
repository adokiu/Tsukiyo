import { useEffect, useRef, useCallback } from 'react'

export type WsMessageHandler = (msg: any) => void

interface UseWebSocketOptions {
  url: string
  onMessage: WsMessageHandler
  reconnectInterval?: number
  enabled?: boolean
}

export function useWebSocket({ url, onMessage, reconnectInterval = 3000, enabled = true }: UseWebSocketOptions) {
  const wsRef = useRef<WebSocket | null>(null)
  const handlerRef = useRef(onMessage)
  const reconnectTimer = useRef<ReturnType<typeof setTimeout>>()
  const manualClose = useRef(false)

  useEffect(() => {
    handlerRef.current = onMessage
  }, [onMessage])

  const connect = useCallback(() => {
    if (!enabled) return
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const fullUrl = url.startsWith('ws') ? url : `${proto}//${window.location.host}${url}`
    const ws = new WebSocket(fullUrl)
    wsRef.current = ws

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        handlerRef.current(msg)
      } catch {
        // ignore
      }
    }

    ws.onclose = () => {
      wsRef.current = null
      if (!manualClose.current && enabled) {
        reconnectTimer.current = setTimeout(connect, reconnectInterval)
      }
    }

    ws.onerror = () => {
      ws.close()
    }
  }, [url, reconnectInterval, enabled])

  useEffect(() => {
    manualClose.current = false
    connect()
    return () => {
      manualClose.current = true
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current)
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
    }
  }, [connect])

  return wsRef
}
