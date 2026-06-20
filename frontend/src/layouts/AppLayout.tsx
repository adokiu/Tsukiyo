import { useMemo, useState } from 'react'
import { Outlet, NavLink, useLocation, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import {
  LayoutDashboard,
  Server,
  Boxes,
  HardDrive,
  ListTodo,
  Network,
  Shield,
  Settings,
  LogOut,
  Moon,
  Sun,
  Monitor,
  ChevronLeft,
  ChevronRight,
  Search,
  Container,
  MonitorDot,
  Flame,
  ShieldCheck,
  Globe,
  SlidersHorizontal,
} from 'lucide-react'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import { ToastContainer } from '@/components/Toast/Toast'
import './AppLayout.css'

interface SubMenuItem {
  path: string
  labelKey: string
  icon: React.ComponentType<{ size?: number | string; className?: string; strokeWidth?: number | string }>
}

interface MenuGroup {
  id: string
  labelKey: string
  icon: React.ComponentType<{ size?: number | string; className?: string; strokeWidth?: number | string }>
  path?: string
  children?: SubMenuItem[]
}

const menuConfig: MenuGroup[] = [
  {
    id: 'overview',
    labelKey: 'nav.systemOverview',
    icon: LayoutDashboard,
    path: '/admin/systemOverview',
  },
  {
    id: 'host',
    labelKey: 'nav.hostManagement',
    icon: Server,
    children: [
      { path: '/admin/hostManagement/nodes', labelKey: 'nav.hostList', icon: Server },
      { path: '/admin/hostManagement/images', labelKey: 'nav.templateManagement', icon: HardDrive },
      { path: '/admin/hostManagement/network', labelKey: 'nav.networkManagement', icon: Network },
      { path: '/admin/hostManagement/storage', labelKey: 'nav.storageManagement', icon: HardDrive },
      { path: '/admin/hostManagement/tasks', labelKey: 'nav.taskList', icon: ListTodo },
    ],
  },
  {
    id: 'instance',
    labelKey: 'nav.instanceManagement',
    icon: Boxes,
    children: [
      { path: '/admin/instanceManagement/vm', labelKey: 'nav.virtualMachines', icon: MonitorDot },
      { path: '/admin/instanceManagement/container', labelKey: 'nav.containers', icon: Container },
    ],
  },
  {
    id: 'security',
    labelKey: 'nav.securityManagement',
    icon: Shield,
    children: [
      { path: '/admin/securityManagement/security', labelKey: 'nav.securityOverview', icon: ShieldCheck },
      { path: '/admin/securityManagement/firewall', labelKey: 'nav.firewallManagement', icon: Flame },
      { path: '/admin/securityManagement/acl', labelKey: 'nav.aclRules', icon: SlidersHorizontal },
      { path: '/admin/securityManagement/url-filter', labelKey: 'nav.urlFilter', icon: Globe },
    ],
  },
  {
    id: 'system',
    labelKey: 'nav.systemManagement',
    icon: Settings,
    children: [
      { path: '/admin/systemManagement/settings', labelKey: 'nav.generalSettings', icon: Settings },
    ],
  },
]

function matchGroup(locationPath: string, group: MenuGroup): boolean {
  if (group.path && locationPath.startsWith(group.path)) return true
  if (group.children) {
    return group.children.some(
      (c) => locationPath === c.path || locationPath.startsWith(c.path + '/')
    )
  }
  return false
}

export default function AppLayout() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const logout = useAuthStore((s) => s.logout)
  const { theme, setTheme } = useThemeStore()
  const [collapsed, setCollapsed] = useState(false)

  const activeGroup = useMemo(
    () => menuConfig.find((g) => matchGroup(location.pathname, g)) ?? menuConfig[0],
    [location.pathname]
  )

  const handleLogout = () => {
    logout()
    navigate('/login', { replace: true })
  }

  const cycleTheme = () => {
    const next = theme === 'light' ? 'dark' : theme === 'dark' ? 'system' : 'light'
    setTheme(next)
  }

  const ThemeIcon = theme === 'light' ? Sun : theme === 'dark' ? Moon : Monitor

  const handlePrimaryClick = (group: MenuGroup) => {
    if (group.path) {
      navigate(group.path)
      return
    }
    if (group.children?.length) {
      navigate(group.children[0].path)
    }
  }

  return (
    <div className="app-shell">
      <aside className={`sidebar-container ${collapsed ? 'collapsed' : 'expanded'} ${!collapsed && (!activeGroup.children || activeGroup.children.length === 0) ? 'no-secondary' : ''}`}>
        <div className={`sidebar-header ${collapsed ? 'collapsed-header' : ''}`}>
          {!collapsed ? (
            <>
              <div className="sidebar-header-top">
                <span className="sidebar-logo-text">{t('appName')}</span>
              </div>
              <div className="sidebar-search">
                <Search size={16} />
                <span>{t('common.search')}</span>
                <kbd>Ctrl+K</kbd>
              </div>
            </>
          ) : (
            <span className="sidebar-logo-text" style={{ fontSize: 14, writingMode: 'vertical-rl' }}>{t('appName')}</span>
          )}
        </div>

        <div className="sidebar-menus">
          <nav className="sidebar-primary">
            <ul className="menu-list">
              {menuConfig.map((group) => {
                const Icon = group.icon
                const isActive = activeGroup.id === group.id
                return (
                  <li key={group.id}>
                    <button
                      type="button"
                      className={`menu-item ${isActive ? 'active' : ''}`}
                      onClick={() => handlePrimaryClick(group)}
                      title={t(group.labelKey)}
                    >
                      <Icon size={18} strokeWidth={2.5} className="menu-item-icon" />
                      {!collapsed && <span className="menu-item-label">{t(group.labelKey)}</span>}
                    </button>
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

          {!collapsed && activeGroup.children && activeGroup.children.length > 0 && (
            <nav className="sidebar-secondary">
              <ul className="secondary-menu-list">
                {activeGroup.children.map((item) => {
                  const Icon = item.icon
                  return (
                    <li key={item.path}>
                      <NavLink
                        to={item.path}
                        end={item.path === '/admin/securityManagement/security'}
                        className={({ isActive }) =>
                          `secondary-menu-item ${isActive ? 'active' : ''}`
                        }
                      >
                        <Icon size={16} strokeWidth={2.5} className="secondary-menu-item-icon" />
                        <span>{t(item.labelKey)}</span>
                      </NavLink>
                    </li>
                  )
                })}
              </ul>
            </nav>
          )}
        </div>

        <button
          type="button"
          className="sidebar-collapse-btn"
          onClick={() => setCollapsed(!collapsed)}
          title={collapsed ? '展开' : '收起'}
        >
          {collapsed ? <ChevronRight size={20} /> : <ChevronLeft size={20} />}
        </button>
      </aside>

      <div className="app-main">
        <div className="app-content">
          <Outlet />
        </div>
      </div>
      <ToastContainer />
    </div>
  )
}
