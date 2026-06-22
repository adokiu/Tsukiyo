import { useEffect, useRef, useState, useCallback } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useSpice } from 'spice-html5-react'
import apiClient from '@/api/client'
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

  // NumLock 修复：浏览器在 NumLock 开启时对小键盘按键的 keyCode 和 code 会变化
  // spice-html5-react 的 scancode 映射表 ji (code -> scancode) 缺少部分小键盘映射
  // 这里在 keydown/keyup 时检测小键盘按键，强制使用正确的 code 进行映射
  const numpadCodeFix = (e: KeyboardEvent): string => {
    const codeMap: Record<number, string> = {
      96: 'Numpad0', 97: 'Numpad1', 98: 'Numpad2', 99: 'Numpad3',
      100: 'Numpad4', 101: 'Numpad5', 102: 'Numpad6', 103: 'Numpad7',
      104: 'Numpad8', 105: 'Numpad9', 106: 'NumpadMultiply',
      107: 'NumpadAdd', 109: 'NumpadSubtract', 110: 'NumpadDecimal',
      111: 'NumpadDivide', 13: 'NumpadEnter',
    }
    if (e.location === 3 || (e.keyCode >= 96 && e.keyCode <= 111)) {
      const fixed = codeMap[e.keyCode]
      if (fixed) return fixed
    }
    return e.code
  }

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

  // 文本转 scancode 映射表（AT scancode）
  const charToScancode = (ch: string): { make: number; shift: boolean } | null => {
    const map: Record<string, { make: number; shift: boolean }> = {
      'a': { make: 30, shift: false }, 'b': { make: 48, shift: false }, 'c': { make: 46, shift: false },
      'd': { make: 32, shift: false }, 'e': { make: 18, shift: false }, 'f': { make: 33, shift: false },
      'g': { make: 34, shift: false }, 'h': { make: 35, shift: false }, 'i': { make: 23, shift: false },
      'j': { make: 36, shift: false }, 'k': { make: 37, shift: false }, 'l': { make: 38, shift: false },
      'm': { make: 50, shift: false }, 'n': { make: 49, shift: false }, 'o': { make: 24, shift: false },
      'p': { make: 25, shift: false }, 'q': { make: 16, shift: false }, 'r': { make: 19, shift: false },
      's': { make: 31, shift: false }, 't': { make: 20, shift: false }, 'u': { make: 22, shift: false },
      'v': { make: 47, shift: false }, 'w': { make: 17, shift: false }, 'x': { make: 45, shift: false },
      'y': { make: 21, shift: false }, 'z': { make: 44, shift: false },
      'A': { make: 30, shift: true }, 'B': { make: 48, shift: true }, 'C': { make: 46, shift: true },
      'D': { make: 32, shift: true }, 'E': { make: 18, shift: true }, 'F': { make: 33, shift: true },
      'G': { make: 34, shift: true }, 'H': { make: 35, shift: true }, 'I': { make: 23, shift: true },
      'J': { make: 36, shift: true }, 'K': { make: 37, shift: true }, 'L': { make: 38, shift: true },
      'M': { make: 50, shift: true }, 'N': { make: 49, shift: true }, 'O': { make: 24, shift: true },
      'P': { make: 25, shift: true }, 'Q': { make: 16, shift: true }, 'R': { make: 19, shift: true },
      'S': { make: 31, shift: true }, 'T': { make: 20, shift: true }, 'U': { make: 22, shift: true },
      'V': { make: 47, shift: true }, 'W': { make: 17, shift: true }, 'X': { make: 45, shift: true },
      'Y': { make: 21, shift: true }, 'Z': { make: 44, shift: true },
      '0': { make: 11, shift: false }, '1': { make: 2, shift: false }, '2': { make: 3, shift: false },
      '3': { make: 4, shift: false }, '4': { make: 5, shift: false }, '5': { make: 6, shift: false },
      '6': { make: 7, shift: false }, '7': { make: 8, shift: false }, '8': { make: 9, shift: false },
      '9': { make: 10, shift: false },
      ')': { make: 11, shift: true }, '!': { make: 2, shift: true }, '@': { make: 3, shift: true },
      '#': { make: 4, shift: true }, '$': { make: 5, shift: true }, '%': { make: 6, shift: true },
      '^': { make: 7, shift: true }, '&': { make: 8, shift: true }, '*': { make: 9, shift: true },
      '(': { make: 10, shift: true },
      '-': { make: 12, shift: false }, '_': { make: 12, shift: true },
      '=': { make: 13, shift: false }, '+': { make: 13, shift: true },
      '[': { make: 26, shift: false }, '{': { make: 26, shift: true },
      ']': { make: 27, shift: false }, '}': { make: 27, shift: true },
      ';': { make: 39, shift: false }, ':': { make: 39, shift: true },
      "'": { make: 40, shift: false }, '"': { make: 40, shift: true },
      '`': { make: 41, shift: false }, '~': { make: 41, shift: true },
      '\\': { make: 43, shift: false }, '|': { make: 43, shift: true },
      ',': { make: 51, shift: false }, '<': { make: 51, shift: true },
      '.': { make: 52, shift: false }, '>': { make: 52, shift: true },
      '/': { make: 53, shift: false }, '?': { make: 53, shift: true },
      ' ': { make: 57, shift: false },
      '\n': { make: 28, shift: false },
      '\t': { make: 15, shift: false },
    }
    return map[ch] ?? null
  }

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
