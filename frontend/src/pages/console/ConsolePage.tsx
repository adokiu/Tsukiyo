import { useEffect, useRef, useCallback } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { SearchAddon } from '@xterm/addon-search'
import '@xterm/xterm/css/xterm.css'
import './Terminal.css'

const TERMINAL_PADDING = 16

export default function ConsolePage() {
  const [searchParams] = useSearchParams()
  const terminalRef = useRef<HTMLDivElement>(null)
  const terminalInstance = useRef<Terminal | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const fitAddonRef = useRef<FitAddon | null>(null)
  const disposedRef = useRef(false)

  const resizeTerminal = useCallback(() => {
    fitAddonRef.current?.fit()
    const term = terminalInstance.current
    const ws = wsRef.current
    if (term && ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
    }
  }, [])

  useEffect(() => {
    const token = searchParams.get('token')
    if (!token || !terminalRef.current) return

    disposedRef.current = false

    const term = new Terminal({
      cursorBlink: true,
      macOptionIsMeta: true,
      fontFamily: "'Cascadia Mono', 'Noto Sans SC', monospace",
      fontSize: 16,
      scrollback: 5000,
      theme: {
        background: '#000000',
        foreground: '#ffffff',
        cursor: '#ffffff',
        selectionBackground: '#264f78',
      },
    })

    const fitAddon = new FitAddon()
    fitAddonRef.current = fitAddon
    const webLinksAddon = new WebLinksAddon()
    const searchAddon = new SearchAddon()

    term.loadAddon(fitAddon)
    term.loadAddon(webLinksAddon)
    term.loadAddon(searchAddon)

    term.open(terminalRef.current)
    terminalInstance.current = term

    const resizeObserver =
      typeof ResizeObserver !== 'undefined'
        ? new ResizeObserver(() => {
            if (!disposedRef.current) resizeTerminal()
          })
        : null

    if (resizeObserver && terminalRef.current) {
      resizeObserver.observe(terminalRef.current)
    }

    document.fonts?.ready?.then(() => {
      if (!disposedRef.current) resizeTerminal()
    })

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/api/v1/console/ssh?token=${token}`
    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onopen = () => {
      if (disposedRef.current) return
      resizeTerminal()
    }

    ws.onmessage = (event) => {
      if (disposedRef.current) return
      try {
        const msg = JSON.parse(event.data)
        if (msg.type === 'data') {
          term.write(msg.data)
        } else if (msg.type === 'error') {
          term.writeln(`\x1b[31m${msg.message}\x1b[0m`)
        } else if (msg.type === 'exit') {
          term.writeln('\r\n\x1b[33m控制台会话已结束\x1b[0m')
        }
      } catch {
        term.write(event.data)
      }
    }

    ws.onclose = () => {
      if (disposedRef.current) return
      term.write('\r\n\x1b[33m连接已断开\x1b[0m')
    }

    const termDataDisposable = term.onData((data) => {
      if (disposedRef.current) return
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'input', data }))
      }
    })

    const handleResize = () => resizeTerminal()
    window.addEventListener('resize', handleResize)

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.ctrlKey && (e.key === 'f' || e.key === 'd')) {
        searchAddon.findNext('')
        e.preventDefault()
      }
    }
    document.addEventListener('keydown', handleKeyDown)

    const handleContextMenu = (e: MouseEvent) => {
      if (e.ctrlKey || ws.readyState !== WebSocket.OPEN) return
      const selection = window.getSelection()
      const hasSelection = selection && selection.toString().length > 0
      if (hasSelection) {
        e.preventDefault()
        const selectedText = selection.toString()
        navigator.clipboard.writeText(selectedText).finally(() => {
          if (disposedRef.current) return
          term.focus()
          term.clearSelection()
        })
      } else {
        e.preventDefault()
        term.focus()
        navigator.clipboard.readText().then((text) => {
          if (disposedRef.current || ws.readyState !== WebSocket.OPEN) return
          ws.send(JSON.stringify({ type: 'input', data: text.replace(/\r?\n/g, '\r') }))
        })
      }
    }
    document.addEventListener('contextmenu', handleContextMenu)

    return () => {
      disposedRef.current = true
      ws.onopen = null
      ws.onmessage = null
      ws.onclose = null
      ws.onerror = null
      resizeObserver?.disconnect()
      termDataDisposable.dispose()
      term.dispose()
      if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
        ws.close()
      }
      terminalInstance.current = null
      wsRef.current = null
      fitAddonRef.current = null
      window.removeEventListener('resize', handleResize)
      document.removeEventListener('keydown', handleKeyDown)
      document.removeEventListener('contextmenu', handleContextMenu)
    }
  }, [searchParams, resizeTerminal])

  return (
    <div
      className="terminal-page h-screen w-screen overflow-hidden"
      style={{
        '--xterm-padding': `${TERMINAL_PADDING}px`,
        '--xterm-container-bg': '#000000',
        '--xterm-scrollbar-track': '#1e1e1e',
        '--xterm-scrollbar-thumb': '#555555',
        '--xterm-scrollbar-thumb-hover': '#777777',
      } as React.CSSProperties}
    >
      <div className="terminal-xterm-host" style={{ height: '100%', width: '100%' }}>
        <div ref={terminalRef} className="h-full w-full" />
      </div>
    </div>
  )
}
