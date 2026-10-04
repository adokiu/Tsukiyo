import { Link, useNavigate } from 'react-router-dom'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { Moon, Sun, Monitor, LogOut, User, LayoutGrid, Server } from 'lucide-react'
import { useState } from 'react'
import type { ThemeMode } from '@/stores/app'

export function Header() {
  const navigate = useNavigate()
  const { themeMode, setThemeMode } = useAppStore()
  const { isAuthenticated, user, logout } = useAuthStore()
  const [menuOpen, setMenuOpen] = useState(false)

  const siteName = useAppStore((s) => s.getThemeConfig('siteName'))

  const themeIcons: Record<ThemeMode, typeof Sun> = {
    light: Sun,
    dark: Moon,
    auto: Monitor,
  }
  const ThemeIcon = themeIcons[themeMode]

  const cycleTheme = () => {
    const next: ThemeMode = themeMode === 'light' ? 'dark' : themeMode === 'dark' ? 'auto' : 'light'
    setThemeMode(next)
  }

  const handleLogout = () => {
    logout()
    navigate('/')
  }

  return (
    <header className="sticky top-0 z-50 glass border-b border-border">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <div className="flex h-16 items-center justify-between">
          <div className="flex items-center gap-6">
            <Link to="/" className="text-lg font-semibold tracking-tight">
              {siteName}
            </Link>
            {isAuthenticated && (
              <nav className="hidden sm:flex items-center gap-4">
                <Link to="/" className="text-sm text-muted-foreground hover:text-foreground transition-colors flex items-center gap-1.5">
                  <LayoutGrid size={16} />
                  概览
                </Link>
                <Link to="/instances" className="text-sm text-muted-foreground hover:text-foreground transition-colors flex items-center gap-1.5">
                  <Server size={16} />
                  实例
                </Link>
              </nav>
            )}
          </div>

          <div className="flex items-center gap-3">
            <button
              onClick={cycleTheme}
              className="p-2 rounded-lg hover:bg-accent transition-colors"
              title="切换主题"
            >
              <ThemeIcon size={18} />
            </button>

            {isAuthenticated ? (
              <div className="relative">
                <button
                  onClick={() => setMenuOpen(!menuOpen)}
                  className="flex items-center gap-2 px-3 py-1.5 rounded-lg hover:bg-accent transition-colors"
                >
                  <div className="w-8 h-8 rounded-full bg-primary/10 flex items-center justify-center">
                    <User size={16} />
                  </div>
                  <span className="text-sm font-medium hidden sm:block">{user?.username}</span>
                </button>
                {menuOpen && (
                  <>
                    <div className="fixed inset-0 z-40" onClick={() => setMenuOpen(false)} />
                    <div className="absolute right-0 mt-2 w-48 rounded-xl glass-card shadow-lg z-50 py-1">
                      <Link
                        to="/profile"
                        onClick={() => setMenuOpen(false)}
                        className="flex items-center gap-2 px-4 py-2 text-sm hover:bg-accent transition-colors"
                      >
                        <User size={16} />
                        个人设置
                      </Link>
                      <button
                        onClick={handleLogout}
                        className="flex items-center gap-2 px-4 py-2 text-sm text-destructive hover:bg-accent transition-colors w-full"
                      >
                        <LogOut size={16} />
                        退出登录
                      </button>
                    </div>
                  </>
                )}
              </div>
            ) : (
              <Link
                to="/login"
                className="apple-button text-sm"
              >
                登录
              </Link>
            )}
          </div>
        </div>
      </div>
    </header>
  )
}
