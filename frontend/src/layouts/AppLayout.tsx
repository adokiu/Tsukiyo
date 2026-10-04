import { useMemo, useState } from 'react'
import { Outlet, NavLink, useLocation, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import apiClient from '@/api/client'
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
  Palette,
  Users,
  UsersRound,
  Wallet,
  Receipt,
  CreditCard,
  ShoppingCart,
  FolderTree,
  Package,
  Ticket,
  QrCode,
  Mail,
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
    id: 'user',
    labelKey: 'nav.userManagement',
    icon: Users,
    children: [
      { path: '/admin/userManagement/users', labelKey: 'nav.users', icon: Users },
      { path: '/admin/userManagement/groups', labelKey: 'nav.userGroups', icon: UsersRound },
    ],
  },
  {
    id: 'finance',
    labelKey: 'nav.financeManagement',
    icon: Wallet,
    children: [
      { path: '/admin/financeManagement/overview', labelKey: 'nav.financeOverview', icon: Wallet },
      { path: '/admin/financeManagement/bills', labelKey: 'nav.billManagement', icon: Receipt },
      { path: '/admin/financeManagement/channels', labelKey: 'nav.paymentChannels', icon: CreditCard },
    ],
  },
  {
    id: 'commerce',
    labelKey: 'nav.commerceManagement',
    icon: ShoppingCart,
    children: [
      { path: '/admin/commerceManagement/categories', labelKey: 'nav.productCategories', icon: FolderTree },
      { path: '/admin/commerceManagement/products', labelKey: 'nav.products', icon: Package },
      { path: '/admin/commerceManagement/coupons', labelKey: 'nav.coupons', icon: Ticket },
      { path: '/admin/commerceManagement/promo-codes', labelKey: 'nav.promoCodes', icon: QrCode },
    ],
  },
  {
    id: 'ticket',
    labelKey: 'nav.ticketManagement',
    icon: Ticket,
    children: [
      { path: '/admin/ticketManagement/tickets', labelKey: 'nav.tickets', icon: Ticket },
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
      { path: '/admin/systemManagement/pushSettings', labelKey: 'nav.pushSettings', icon: Mail },
      { path: '/admin/systemManagement/themes', labelKey: 'nav.themeManagement', icon: Palette },
    ],
  },
  {
    id: 'themeSettings',
    labelKey: 'nav.themeSettings',
    icon: Palette,
    children: [],
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
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const logout = useAuthStore((s) => s.logout)
  const { theme, setTheme } = useThemeStore()
  const [collapsed, setCollapsed] = useState(false)

  const lang = i18n.language?.startsWith('zh') ? 'zh' : 'en'

  const { data: siteConfig } = useQuery({
    queryKey: ['site-config'],
    queryFn: async () => {
      const res = await apiClient.get('/settings/site')
      return res.data
    },
  })

  const { data: themes } = useQuery({
    queryKey: ['themes'],
    queryFn: async () => {
      const res = await apiClient.get('/theme/list')
      return res.data as any[]
    },
  })

  const dynamicMenuConfig = useMemo(() => {
    const config = [...menuConfig]
    const currentThemeShort = siteConfig?.theme || 'default'
    const currentTheme = themes?.find((th: any) => th.short === currentThemeShort)
    const pages = currentTheme?.configuration?.pages
    if (pages && Array.isArray(pages)) {
      const themeSettingsIdx = config.findIndex((g) => g.id === 'themeSettings')
      if (themeSettingsIdx >= 0) {
        config[themeSettingsIdx] = {
          ...config[themeSettingsIdx],
          children: pages.map((page: any) => {
            const pageName = typeof page.name === 'string' ? page.name : (page.name?.[lang] || page.name?.['en'] || page.name?.['zh'] || page.key)
            return {
              path: `/admin/themeSettings/${page.key}`,
              labelKey: pageName,
              icon: Palette,
            }
          }),
        }
      }
    }
    return config
  }, [siteConfig, themes, lang])

  const activeGroup = useMemo(
    () => dynamicMenuConfig.find((g) => matchGroup(location.pathname, g)) ?? dynamicMenuConfig[0],
    [location.pathname, dynamicMenuConfig]
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
              {dynamicMenuConfig.map((group) => {
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
