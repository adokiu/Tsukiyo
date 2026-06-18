import { useEffect, useRef } from 'react'
import { CheckCircle, XCircle, AlertTriangle, Info, X } from 'lucide-react'
import { useToastStore } from '@/stores/toast'
import type { ToastType } from '@/stores/toast'
import './Toast.css'

const iconMap: Record<ToastType, React.ReactNode> = {
  success: <CheckCircle size={18} />,
  error: <XCircle size={18} />,
  warning: <AlertTriangle size={18} />,
  info: <Info size={18} />,
}

export function ToastContainer() {
  const toasts = useToastStore((s) => s.toasts)
  const remove = useToastStore((s) => s.remove)

  return (
    <div className="toast-container">
      {toasts.map((t) => (
        <ToastItem key={t.id} id={t.id} type={t.type} message={t.message} onClose={remove} />
      ))}
    </div>
  )
}

function ToastItem({
  id,
  type,
  message,
  onClose,
}: {
  id: string
  type: ToastType
  message: string
  onClose: (id: string) => void
}) {
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    requestAnimationFrame(() => {
      ref.current?.classList.add('toast-item--visible')
    })
  }, [])

  const handleClose = () => {
    ref.current?.classList.remove('toast-item--visible')
    ref.current?.classList.add('toast-item--exit')
    setTimeout(() => onClose(id), 200)
  }

  return (
    <div ref={ref} className={`toast-item toast-item--${type}`} role="alert">
      <span className="toast-item__icon">{iconMap[type]}</span>
      <span className="toast-item__message">{message}</span>
      <button className="toast-item__close" onClick={handleClose} aria-label="关闭">
        <X size={14} />
      </button>
    </div>
  )
}
