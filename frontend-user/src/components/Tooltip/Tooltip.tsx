import { useState, useRef, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import './Tooltip.css'

interface TooltipProps {
  content: ReactNode
  children: ReactNode
  placement?: 'top' | 'bottom' | 'left' | 'right'
  delay?: number
}

export function Tooltip({ content, children, placement = 'top', delay = 200 }: TooltipProps) {
  const [visible, setVisible] = useState(false)
  const [coords, setCoords] = useState({ x: 0, y: 0 })
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const targetRef = useRef<HTMLSpanElement>(null)

  const show = () => {
    if (timerRef.current) clearTimeout(timerRef.current)
    timerRef.current = setTimeout(() => {
      if (!targetRef.current) return
      const rect = targetRef.current.getBoundingClientRect()
      let x = rect.left + rect.width / 2
      let y = rect.top
      switch (placement) {
        case 'top':
          y = rect.top - 6
          break
        case 'bottom':
          y = rect.bottom + 6
          break
        case 'left':
          x = rect.left - 6
          y = rect.top + rect.height / 2
          break
        case 'right':
          x = rect.right + 6
          y = rect.top + rect.height / 2
          break
      }
      setCoords({ x, y })
      setVisible(true)
    }, delay)
  }

  const hide = () => {
    if (timerRef.current) clearTimeout(timerRef.current)
    setVisible(false)
  }

  return (
    <>
      <span
        ref={targetRef}
        onMouseEnter={show}
        onMouseLeave={hide}
        className="tooltip-trigger"
      >
        {children}
      </span>
      {visible && createPortal(
        <div
          className={`tooltip tooltip--${placement}`}
          style={{
            left: coords.x,
            top: coords.y,
          }}
        >
          {content}
        </div>,
        document.body
      )}
    </>
  )
}
