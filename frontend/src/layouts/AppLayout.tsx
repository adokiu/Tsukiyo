import { useState } from 'react'
import { Outlet, NavLink, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import {
  LayoutDashboard,
  Server,
  Boxes,
  HardDrive,
  Network,
  Shield,
  Users,
  ClipboardList,
  LogOut,
  Moon,
  Sun,
  Monitor,
  ChevronLeft,
  ChevronRight,
} from 'lucide-react'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import { ToastContainer } from '@/components/Toast/Toast'

const navGroups = [
  {
    items: [
      { path: '/admin/dashboard', label: 'nav.dashboard', icon: LayoutDashboard },
    ],
  },
  {
    items: [
      { path: '/admin/nodes', label: 'nav.nodes', icon: Server },
      { path: '/admin/instances', label: 'nav.instances', icon: Boxes },
      { path: '/admin/images', label: 'nav.images', icon: HardDrive },
    ],
  },
  {
    items: [
      { path: '/admin/network', label: 'nav.network', icon: Network },
      { path: '/admin/security', label: 'nav.securityManagement', icon: Shield },
    ],
  },
  {
    items: [
      { path: '/admin/users', label: 'nav.users', icon: Users },
      { path: '/admin/audit-logs', label: 'nav.auditLogs', icon: ClipboardList },
    ],
  },
]

export default function AppLayout() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const logout = useAuthStore((s) => s.logout)
  const { theme, setTheme } = useThemeStore()
  const [collapsed, setCollapsed] = useState(false)

  const handleLogout = () => {
    logout()
    navigate('/login', { replace: true })
  }

  const cycleTheme = () => {
    const next = theme === 'light' ? 'dark' : theme === 'dark' ? 'system' : 'light'
    setTheme(next)
  }

  const ThemeIcon = theme === 'light' ? Sun : theme === 'dark' ? Moon : Monitor

  return (
    <div className="flex h-screen overflow-hidden bg-background">
      {/* Sidebar */}
      <aside
        className={`flex flex-col border-r border-border bg-card transition-all duration-300 ${
          collapsed ? 'w-16' : 'w-64'
        }`}
      >
        {/* Logo */}
        <div className="flex h-16 items-center justify-between px-4 border-b border-border">
          {!collapsed && (
            <span className="text-lg font-display font-semibold tracking-tight">{t('appName')}</span>
          )}
          <button
            onClick={() => setCollapsed(!collapsed)}
            className="rounded-lg p-1.5 text-muted-foreground hover:bg-accent transition-colors"
          >
            {collapsed ? <ChevronRight size={18} /> : <ChevronLeft size={18} />}
          </button>
        </div>

        {/* Nav */}
        <nav className="flex-1 overflow-y-auto py-4 space-y-4">
          {navGroups.map((group, gi) => (
            <div key={gi} className="px-3 space-y-1">
              {group.items.map((item) => (
                <NavLink
                  key={item.path}
                  to={item.path}
                  className={({ isActive }) =>
                    `flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium transition-all duration-200 ${
                      isActive
                        ? 'bg-apple-blue/10 text-apple-blue'
                        : 'text-muted-foreground hover:bg-accent hover:text-foreground'
                    }`
                  }
                >
                  <item.icon size={18} />
                  {!collapsed && <span>{t(item.label)}</span>}
                </NavLink>
              ))}
            </div>
          ))}
        </nav>

        {/* Bottom */}
        <div className="border-t border-border p-3 space-y-1">
          <button
            onClick={cycleTheme}
            className="flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-muted-foreground hover:bg-accent hover:text-foreground transition-all"
          >
            <ThemeIcon size={18} />
            {!collapsed && <span>{t('common.theme')}</span>}
          </button>
          <button
            onClick={handleLogout}
            className="flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-muted-foreground hover:bg-apple-red/10 hover:text-apple-red transition-all"
          >
            <LogOut size={18} />
            {!collapsed && <span>{t('common.logout')}</span>}
          </button>
        </div>
      </aside>

      {/* Main */}
      <main className="flex-1 overflow-y-auto">
        <Outlet />
      </main>
      <ToastContainer />
    </div>
  )
}
