import { create } from 'zustand'
import { persist } from 'zustand/middleware'

type Theme = 'light' | 'dark' | 'system'

interface ThemeState {
  theme: Theme
  setTheme: (theme: Theme) => void
  resolved: 'light' | 'dark'
  apply: () => void
}

function resolveTheme(theme: Theme): 'light' | 'dark' {
  if (theme !== 'system') return theme
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

export const useThemeStore = create<ThemeState>()(
  persist(
    (set, get) => ({
      theme: 'system',
      resolved: resolveTheme('system'),
      setTheme: (theme) => {
        const resolved = resolveTheme(theme)
        set({ theme, resolved })
        document.documentElement.classList.toggle('dark', resolved === 'dark')
      },
      apply: () => {
        const resolved = resolveTheme(get().theme)
        set({ resolved })
        document.documentElement.classList.toggle('dark', resolved === 'dark')
      },
    }),
    { name: 'tsukiyo-theme' }
  )
)
