import { type ReactNode } from 'react'
import { Loader2 } from 'lucide-react'
import './PageTransition.css'

interface PageTransitionProps {
  loading?: boolean
  empty?: boolean
  emptyText?: string
  emptyIcon?: ReactNode
  children: ReactNode
}

export function PageTransition({ loading, empty, emptyText = '暂无数据', emptyIcon, children }: PageTransitionProps) {
  if (loading) {
    return (
      <div className="page-transition__loading">
        <Loader2 size={24} className="page-transition__spinner" />
        <span>加载中...</span>
      </div>
    )
  }

  if (empty) {
    return (
      <div className="page-transition__empty">
        {emptyIcon}
        <p className="page-transition__empty-text">{emptyText}</p>
      </div>
    )
  }

  return (
    <div className="page-transition__content">
      {children}
    </div>
  )
}
