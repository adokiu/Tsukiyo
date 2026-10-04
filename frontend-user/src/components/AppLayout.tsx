import { Outlet, NavLink, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import {
  LayoutDashboard,
  Server,
  User,
  Wallet,
  LogOut,
  Moon,
  Sun,
  Monitor,
  ChevronLeft,
  ChevronRight,
  Package,
  ShoppingCart,
  Receipt,
  TicketIcon,
} from 'lucide-react'
import { useState } from 'react'
import { useAuthStore } from '@/stores/auth'
import { useAppStore, type ThemeMode } from '@/stores/app'
import { ToastContainer } from '@/components/Toast/Toast'
import { Background } from '@/components/Background'
import { UserMetricsWsProvider } from '@/contexts/UserMetricsWsContext'
import './AppLayout.css'

interface MenuItem {
  path: string
  labelKey: string
  icon: React.ComponentType<{ size?: number | string; className?: string; strokeWidth?: number | string }>
}

const menuConfig: MenuItem[] = [
  { path: '/dashboard', labelKey: 'nav.overview', icon: LayoutDashboard },
  { path: '/products', labelKey: 'nav.products', icon: Package },
  { path: '/cart', labelKey: 'nav.cart', icon: ShoppingCart },
  { path: '/instances', labelKey: 'nav.instances', icon: Server },
  { path: '/wallet', labelKey: 'nav.wallet', icon: Wallet },
  { path: '/bills', labelKey: 'nav.bills', icon: Receipt },
  { path: '/tickets', labelKey: 'nav.tickets', icon: TicketIcon },
  { path: '/profile', labelKey: 'nav.profile', icon: User },
]

export function AppLayout() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { logout } = useAuthStore()
  const { themeMode, setThemeMode } = useAppStore()
  const [collapsed, setCollapsed] = useState(false)

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
    navigate('/login', { replace: true })
  }

  return (
    <div className="app-shell">
      <Background />
      <aside className={`sidebar-container ${collapsed ? 'collapsed' : 'expanded'}`}>
        <div className={`sidebar-header ${collapsed ? 'collapsed-header' : ''}`}>
          {!collapsed ? (
            <div className="sidebar-header-top">
              <span className="sidebar-logo-text">{siteName}</span>
            </div>
          ) : (
            <span className="sidebar-logo-text" style={{ fontSize: 14, writingMode: 'vertical-rl' }}>{siteName}</span>
          )}
        </div>

        <div className="sidebar-menus">
          <nav className="sidebar-primary">
            <ul className="menu-list">
              {menuConfig.map((item) => {
                const Icon = item.icon
                return (
                  <li key={item.path}>
                    <NavLink
                      to={item.path}
                      end={item.path === '/'}
                      className={({ isActive }) => `menu-item ${isActive ? 'active' : ''}`}
                      title={t(item.labelKey)}
                    >
                      <Icon size={18} strokeWidth={2.5} className="menu-item-icon" />
                      {!collapsed && <span className="menu-item-label">{t(item.labelKey)}</span>}
                    </NavLink>
                  </li>
                )
              })}
            </ul>

            <div style={{ flex: 1 }} />

            <ul className="menu-list">
              <li>
                <button type="button" className="menu-item" onClick={cycleTheme} title={t('common.theme')}>
                  <ThemeIcon size={18} strokeWidth={2.5} className="menu-item-icon" />
                  {!collapsed && <span className="menu-item-label">{t('common.theme')}</span>}
                </button>
              </li>
              <li>
                <button type="button" className="menu-item" onClick={handleLogout} title={t('common.logout')}>
                  <LogOut size={18} strokeWidth={2.5} className="menu-item-icon" />
                  {!collapsed && <span className="menu-item-label">{t('common.logout')}</span>}
                </button>
              </li>
            </ul>
          </nav>
        </div>

        <button
          type="button"
          className="sidebar-collapse-btn"
          onClick={() => setCollapsed(!collapsed)}
          title={collapsed ? t('common.expand') : t('common.collapse')}
        >
          {collapsed ? <ChevronRight size={20} /> : <ChevronLeft size={20} />}
        </button>
      </aside>

      <div className="app-main">
        <div className="app-content">
          <UserMetricsWsProvider>
            <Outlet />
          </UserMetricsWsProvider>
        </div>
      </div>
      <ToastContainer />
    </div>
  )
}
