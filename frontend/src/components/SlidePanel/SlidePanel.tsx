import { useEffect, useRef, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { X } from 'lucide-react'
import './SlidePanel.css'

interface SlidePanelProps {
  open: boolean
  onClose: () => void
  title: string
  children: ReactNode
  footer?: ReactNode
  width?: number
}

export function SlidePanel({ open, onClose, title, children, footer, width = 480 }: SlidePanelProps) {
  const overlayRef = useRef<HTMLDivElement>(null)
  const panelRef = useRef<HTMLDivElement>(null)
  const downOnOverlayRef = useRef(false)

  useEffect(() => {
    if (!open) return
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [open, onClose])

  useEffect(() => {
    if (open) {
      document.body.style.overflow = 'hidden'
      requestAnimationFrame(() => {
        overlayRef.current?.classList.add('slide-overlay--visible')
        panelRef.current?.classList.add('slide-panel--visible')
      })
    }
    return () => {
      document.body.style.overflow = ''
    }
  }, [open])

  if (!open) return null

  const handleOverlayMouseDown = (e: React.MouseEvent) => {
    downOnOverlayRef.current = e.target === overlayRef.current
  }
  const handleOverlayMouseUp = (e: React.MouseEvent) => {
    if (downOnOverlayRef.current && e.target === overlayRef.current) {
      onClose()
    }
    downOnOverlayRef.current = false
  }

  return createPortal(
    <div
      ref={overlayRef}
      className="slide-overlay"
      onMouseDown={handleOverlayMouseDown}
      onMouseUp={handleOverlayMouseUp}
    >
      <div ref={panelRef} className="slide-panel" style={{ width }} onMouseDown={(e) => e.stopPropagation()}>
        <div className="slide-panel__header">
          <h2 className="slide-panel__title">{title}</h2>
          <button className="slide-panel__close" onClick={onClose} aria-label="关闭">
            <X size={18} />
          </button>
        </div>
        <div className="slide-panel__body">{children}</div>
        {footer && <div className="slide-panel__footer">{footer}</div>}
      </div>
    </div>,
    document.body
  )
}
