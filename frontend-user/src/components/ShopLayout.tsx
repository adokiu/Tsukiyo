import type { ReactNode } from 'react'
import { LandingHeader } from '@/components/LandingHeader'
import '@/views/LandingView.css'

/** 商品/购物车等对外页面：统一顶栏 + 内容区 */
export function ShopLayout({ children }: { children: ReactNode }) {
  return (
    <div className="landing-page min-h-screen flex flex-col">
      <div className="landing-curve-bg" aria-hidden>
        <svg className="landing-curve-bottom" viewBox="0 0 1440 600" preserveAspectRatio="none">
          <path d="M0,400 Q360,280 720,340 T1440,290 L1440,600 L0,600 Z" fill="hsl(210 80% 60%)" opacity={0.15} />
          <path d="M0,460 Q480,360 960,430 T1440,390 L1440,600 L0,600 Z" fill="hsl(210 80% 60%)" opacity={0.08} />
        </svg>
        <svg className="landing-curve-top" viewBox="0 0 1440 300" preserveAspectRatio="none">
          <path d="M0,80 Q720,180 1440,60 L1440,0 L0,0 Z" fill="hsl(210 80% 60%)" opacity={0.1} />
        </svg>
      </div>
      <LandingHeader />
      {children}
    </div>
  )
}
