import { useEffect, type ReactNode } from 'react'
import { useAppStore } from '@/stores/app'

export function ThemeProvider({ children }: { children: ReactNode }) {
  const applyTheme = useAppStore((s) => s.applyTheme)
  const themeMode = useAppStore((s) => s.themeMode)

  useEffect(() => {
    applyTheme()
  }, [applyTheme, themeMode])

  useEffect(() => {
    const mql = window.matchMedia('(prefers-color-scheme: dark)')
    const handler = () => {
      if (useAppStore.getState().themeMode === 'auto') {
        useAppStore.getState().applyTheme()
      }
    }
    mql.addEventListener('change', handler)
    return () => mql.removeEventListener('change', handler)
  }, [])

  return <>{children}</>
}
