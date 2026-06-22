import { useEffect, useRef, useState, useCallback } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useSpice } from 'spice-html5-react'
import apiClient from '@/api/client'
import { charToScancode, numpadCodeFix } from '@/utils/scancode'
import './VNC.css'

export default function VNCPage() {
  const [searchParams] = useSearchParams()
  const containerRef = useRef<HTMLDivElement>(null)
  const connectedRef = useRef(false)
  const modalOpenRef = useRef(false)
  const sendingRef = useRef(false)
  const controlsRef = useRef<{ sendKeyDown: (s: number) => void; sendKeyUp: (s: number) => void; sendCtrlAltDel: () => void; connect: (c: { uri: string; password?: string; screenId?: string }) => void; disconnect: () => void; getConnection: () => unknown } | null>(null)
  const [status, setStatus] = useState<'connecting' | 'connected' | 'disconnected' | 'error'>('connecting')
  const [errorMsg, setErrorMsg] = useState('')
  const [showClipboardModal, setShowClipboardModal] = useState(false)
  const [clipboardText, setClipboardText] = useState('')
  const [pasteStatus, setPasteStatus] = useState<'idle' | 'loading' | 'success' | 'error'>('idle')

  const token = searchParams.get('token')

  const { status: spiceStatus, error: spiceError, controls } = useSpice({
    canvasRef: containerRef as React.RefObject<HTMLElement | null>,
    callbacks: {
      onConnect: () => {
        setStatus('connected')
      },
      onDisconnect: () => {
        setStatus('disconnected')
      },
      onError: (e: Error) => {
        setStatus('error')
        setErrorMsg(e.message)
      },
    },
  })

  useEffect(() => {
    controlsRef.current = controls
  }, [controls])

  useEffect(() => {
    if (spiceStatus === 'connected') setStatus('connected')
    else if (spiceStatus === 'error') {
      setStatus('error')
      const errMsg = spiceError?.message || 'SPICE 连接错误'
      setErrorMsg(errMsg)
    } else if (spiceStatus === 'disconnected') setStatus('disconnected')
  }, [spiceStatus, spiceError])

  useEffect(() => {
    if (!token || connectedRef.current) return
    connectedRef.current = true
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/api/v1/console/vnc?token=${token}`
    controls.connect({ uri: wsUrl, password: '', screenId: 'spice-screen' })
  }, [token])


  // 手动绑定键盘和鼠标事件，转发给 SPICE inputs 通道
  useEffect(() => {
    const container = containerRef.current
    if (!container) return

    const getInputs = () => {
      const conn = controlsRef.current?.getConnection() as { channels?: { inputs?: { handleKeyDown: (e: KeyboardEvent) => void; handleKeyUp: (e: KeyboardEvent) => void; handleMouseMove: (e: MouseEvent) => void; handleMouseDown: (e: MouseEvent) => void; handleMouseUp: (e: MouseEvent) => void; handleMouseWheel: (e: WheelEvent) => void; handleContextMenu: (e: MouseEvent) => void } | null } } | null
      return conn?.channels?.inputs ?? null
    }

    const onKeyDown = (e: KeyboardEvent) => {
      if (modalOpenRef.current) return
      const inputs = getInputs()
      if (!inputs) return
      e.preventDefault()
      const fixedCode = numpadCodeFix(e)
      if (fixedCode !== e.code) {
        const fixedEvent = new KeyboardEvent('keydown', {
          key: e.key, code: fixedCode, keyCode: e.keyCode,
          altKey: e.altKey, ctrlKey: e.ctrlKey, shiftKey: e.shiftKey, metaKey: e.metaKey,
          location: e.location, repeat: e.repeat,
        })
        inputs.handleKeyDown(fixedEvent)
      } else {
        inputs.handleKeyDown(e)
      }
    }
    const onKeyUp = (e: KeyboardEvent) => {
      if (modalOpenRef.current) return
      const inputs = getInputs()
      if (!inputs) return
      e.preventDefault()
      const fixedCode = numpadCodeFix(e)
      if (fixedCode !== e.code) {
        const fixedEvent = new KeyboardEvent('keyup', {
          key: e.key, code: fixedCode, keyCode: e.keyCode,
          altKey: e.altKey, ctrlKey: e.ctrlKey, shiftKey: e.shiftKey, metaKey: e.metaKey,
          location: e.location, repeat: e.repeat,
        })
        inputs.handleKeyUp(fixedEvent)
      } else {
        inputs.handleKeyUp(e)
      }
    }
    const onMouseMove = (e: MouseEvent) => getInputs()?.handleMouseMove(e)
    const onMouseDown = (e: MouseEvent) => getInputs()?.handleMouseDown(e)
    const onMouseUp = (e: MouseEvent) => getInputs()?.handleMouseUp(e)
    const onWheel = (e: WheelEvent) => {
      const inputs = getInputs()
      if (!inputs) return
      e.preventDefault()
      inputs.handleMouseWheel(e)
    }
    const onContextMenu = (e: MouseEvent) => getInputs()?.handleContextMenu(e)

    window.addEventListener('keydown', onKeyDown)
    window.addEventListener('keyup', onKeyUp)
    container.addEventListener('mousemove', onMouseMove)
    container.addEventListener('mousedown', onMouseDown)
    container.addEventListener('mouseup', onMouseUp)
    container.addEventListener('wheel', onWheel, { passive: false })
    container.addEventListener('contextmenu', onContextMenu)

    return () => {
      window.removeEventListener('keydown', onKeyDown)
      window.removeEventListener('keyup', onKeyUp)
      container.removeEventListener('mousemove', onMouseMove)
      container.removeEventListener('mousedown', onMouseDown)
      container.removeEventListener('mouseup', onMouseUp)
      container.removeEventListener('wheel', onWheel)
      container.removeEventListener('contextmenu', onContextMenu)
    }
  }, [])

  // 向 SPICE 发送文本字符串（异步逐字符模拟按键，使用 controlsRef 确保引用最新）
  // 注意：sendKeyUp 不会自动加 break bit，必须手动 | 128
  // shift down 和字符 down 之间需要延迟，让 SPICE 服务端处理 modifier 状态变化
  const sendTextToGuest = useCallback((text: string) => {
    if (sendingRef.current) return
    sendingRef.current = true
    const chars = Array.from(text)
    let i = 0
    const sendNext = () => {
      if (i >= chars.length) {
        sendingRef.current = false
        modalOpenRef.current = false
        return
      }
      const ctl = controlsRef.current
      if (!ctl) {
        sendingRef.current = false
        modalOpenRef.current = false
        return
      }
      const ch = chars[i++]
      const mapping = charToScancode(ch)
      if (!mapping) {
        setTimeout(sendNext, 10)
        return
      }
      if (mapping.shift) {
        ctl.sendKeyDown(42)
        setTimeout(() => {
          ctl.sendKeyDown(mapping.make)
          setTimeout(() => {
            ctl.sendKeyUp(mapping.make | 128)
            setTimeout(() => {
              ctl.sendKeyUp(42 | 128)
              setTimeout(sendNext, 20)
            }, 15)
          }, 30)
        }, 15)
      } else {
        ctl.sendKeyDown(mapping.make)
        setTimeout(() => {
          ctl.sendKeyUp(mapping.make | 128)
          setTimeout(sendNext, 20)
        }, 30)
      }
    }
    sendNext()
  }, [])

  // Ctrl+Alt+Delete 按钮
  const handleCtrlAltDel = () => {
    controlsRef.current?.sendCtrlAltDel()
  }

  // 粘贴密码按钮
  const handlePastePassword = async () => {
    if (!token || sendingRef.current) return
    setPasteStatus('loading')
    try {
      const res = await apiClient.get('/console/credentials', { params: { token } })
      const password = res.data?.password
      if (password) {
        sendTextToGuest(password)
        setPasteStatus('success')
        setTimeout(() => setPasteStatus('idle'), 3000)
      } else {
        setPasteStatus('error')
        setTimeout(() => setPasteStatus('idle'), 2000)
      }
    } catch {
      setPasteStatus('error')
      setTimeout(() => setPasteStatus('idle'), 2000)
    }
  }

  // 剪贴板粘贴按钮
  const handlePasteClipboard = () => {
    setShowClipboardModal(false)
    setClipboardText('')
    if (clipboardText) {
      sendTextToGuest(clipboardText)
    }
  }

  return (
    <div className="vnc-page">
      {/* 顶部工具栏 */}
      <div className="vnc-toolbar">
        <button className="vnc-toolbar__btn" onClick={handleCtrlAltDel}>
          Ctrl+Alt+Delete
        </button>
        <button
          className="vnc-toolbar__btn"
          onClick={handlePastePassword}
          disabled={pasteStatus === 'loading'}
        >
          {pasteStatus === 'loading' ? '粘贴中...' : pasteStatus === 'success' ? '已粘贴' : pasteStatus === 'error' ? '获取失败' : '粘贴密码'}
        </button>
        <button className="vnc-toolbar__btn" onClick={() => { modalOpenRef.current = true; setShowClipboardModal(true) }}>
          剪贴板
        </button>
      </div>

      {/* SPICE 画面区域 - letterbox 居中 */}
      <div ref={containerRef} className="vnc-container" id="spice-screen" />

      {/* 连接状态遮罩 */}
      {status === 'connecting' && (
        <div className="vnc-overlay">
          <div className="vnc-overlay__spinner" />
          <div className="vnc-overlay__text">正在连接控制台...</div>
        </div>
      )}
      {status === 'error' && (
        <div className="vnc-overlay">
          <div className="vnc-overlay__text vnc-overlay__text--error">{errorMsg || '控制台连接失败'}</div>
        </div>
      )}
      {status === 'disconnected' && (
        <div className="vnc-overlay">
          <div className="vnc-overlay__text">控制台连接已断开</div>
        </div>
      )}

      {/* 剪贴板弹窗 */}
      {showClipboardModal && (
        <div className="vnc-modal-overlay" onClick={() => { setShowClipboardModal(false); modalOpenRef.current = false }}>
          <div className="vnc-modal" onClick={(e) => e.stopPropagation()}>
            <div className="vnc-modal__title">输入要粘贴的文本</div>
            <textarea
              className="vnc-modal__textarea"
              value={clipboardText}
              onChange={(e) => setClipboardText(e.target.value)}
              placeholder="在此输入要发送到虚拟机的文本..."
              autoFocus
              rows={6}
            />
            <div className="vnc-modal__actions">
              <button className="vnc-modal__btn vnc-modal__btn--cancel" onClick={() => { setShowClipboardModal(false); modalOpenRef.current = false; setClipboardText('') }}>
                取消
              </button>
              <button className="vnc-modal__btn vnc-modal__btn--confirm" onClick={handlePasteClipboard} disabled={!clipboardText}>
                粘贴
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
