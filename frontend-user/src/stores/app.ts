import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import apiClient from '@/api/client'

export type ThemeMode = 'auto' | 'light' | 'dark'

export interface PublicSettings {
  site_name: string
  site_subtitle: string
  site_description: string
  theme: string
  theme_settings: Record<string, any>
  enable_register: boolean
  force_email_verify: boolean
}

interface AppState {
  themeMode: ThemeMode
  publicSettings: PublicSettings | null
  loaded: boolean

  setThemeMode: (mode: ThemeMode) => void
  fetchPublicSettings: () => Promise<void>

  getIsDark: () => boolean
  getThemeConfig: (key: string) => any
  applyTheme: () => void
}

export const useAppStore = create<AppState>()(
  persist(
    (set, get) => ({
      themeMode: 'auto',
      publicSettings: null,
      loaded: false,

      setThemeMode: (mode) => {
        set({ themeMode: mode })
        get().applyTheme()
      },

      fetchPublicSettings: async () => {
        try {
          const res = await apiClient.get('/public/info')
          set({ publicSettings: res.data, loaded: true })
          get().applyTheme()
        } catch {
          set({ loaded: true })
        }
      },

      getIsDark: () => {
        const { themeMode } = get()
        if (themeMode === 'auto') {
          return window.matchMedia('(prefers-color-scheme: dark)').matches
        }
        return themeMode === 'dark'
      },

      getThemeConfig: (key) => {
        const { publicSettings } = get()
        return publicSettings?.theme_settings?.[key]
      },

      applyTheme: () => {
        const state = get()
        const isDark = state.getIsDark()
        const root = document.documentElement

        root.classList.toggle('dark', isDark)

        const hexToHsl = (hex?: string): string => {
          if (!hex || !hex.startsWith('#')) return ''
          const r = parseInt(hex.slice(1, 3), 16) / 255
          const g = parseInt(hex.slice(3, 5), 16) / 255
          const b = parseInt(hex.slice(5, 7), 16) / 255
          const max = Math.max(r, g, b)
          const min = Math.min(r, g, b)
          let h = 0, s = 0
          const l = (max + min) / 2
          if (max !== min) {
            const d = max - min
            s = l > 0.5 ? d / (2 - max - min) : d / (max + min)
            switch (max) {
              case r: h = ((g - b) / d + (g < b ? 6 : 0)) * 60; break
              case g: h = ((b - r) / d + 2) * 60; break
              case b: h = ((r - g) / d + 4) * 60; break
            }
          }
          return `${Math.round(h)} ${Math.round(s * 100)}% ${Math.round(l * 100)}%`
        }

        const primary = hexToHsl(state.getThemeConfig('primaryColor'))
        const primaryHover = hexToHsl(state.getThemeConfig('primaryColorHover'))
        const darkPrimary = hexToHsl(state.getThemeConfig('darkPrimaryColor'))
        const darkPrimaryHover = hexToHsl(state.getThemeConfig('darkPrimaryColorHover'))
        const borderRadius = state.getThemeConfig('borderRadius')

        if (isDark) {
          if (darkPrimary) root.style.setProperty('--primary', darkPrimary)
          if (darkPrimaryHover) root.style.setProperty('--primary-hover', darkPrimaryHover)
        } else {
          if (primary) root.style.setProperty('--primary', primary)
          if (primaryHover) root.style.setProperty('--primary-hover', primaryHover)
        }
        if (borderRadius) root.style.setProperty('--radius', borderRadius)
      },
    }),
    {
      name: 'tsukiyo-user-theme',
      partialize: (state) => ({ themeMode: state.themeMode }),
    }
  )
)
