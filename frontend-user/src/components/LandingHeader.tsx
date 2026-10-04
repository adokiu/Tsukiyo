import { Link, useNavigate } from 'react-router-dom'
import { useState, useEffect, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useAppStore, type ThemeMode } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useCartStore } from '@/stores/cart'
import { Moon, Sun, Monitor, Globe, Menu, X, LayoutDashboard, ShoppingCart, User, LogOut } from 'lucide-react'
import '../views/LandingView.css'

interface NavItem {
  label: string
  href: string
  external?: boolean
}

function parseJsonArray<T>(raw: string, mapper: (item: any) => T, filterFn: (item: T) => boolean): T[] {
  try {
    const parsed = JSON.parse(raw)
    if (Array.isArray(parsed)) {
      return parsed.map(mapper).filter(filterFn)
    }
  } catch {
  }
  return []
}

function parseNavItems(raw: string): NavItem[] {
  return parseJsonArray<NavItem>(
    raw,
    (item) => ({ label: String(item.label ?? ''), href: String(item.href ?? ''), external: !!item.external }),
    (item) => !!item.label && !!item.href
  )
}

export function LandingHeader() {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const { themeMode, setThemeMode, getThemeConfig } = useAppStore()
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const user = useAuthStore((s) => s.user)
  const logout = useAuthStore((s) => s.logout)
  const cartCount = useCartStore((s) => s.items.reduce((sum, item) => sum + item.quantity, 0))
  const fetchCart = useCartStore((s) => s.fetchCart)

  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const [langMenuOpen, setLangMenuOpen] = useState(false)
  const [scrolled, setScrolled] = useState(false)

  const siteName = getThemeConfig('siteName')
  const showLoginButton = getThemeConfig('showLoginButton')
  const showRegisterButton = getThemeConfig('showRegisterButton')
  const navItemsRaw = getThemeConfig('navItems')

  const navItems = useMemo(() => parseNavItems(navItemsRaw), [navItemsRaw])

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

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 10)
    window.addEventListener('scroll', onScroll)
    return () => window.removeEventListener('scroll', onScroll)
  }, [])

  useEffect(() => {
    if (isAuthenticated) {
      fetchCart().catch(() => {})
    }
  }, [isAuthenticated, fetchCart])

  const handleNavClick = (item: NavItem) => {
    setMobileMenuOpen(false)
    if (item.external) {
      window.open(item.href, '_blank')
    } else if (item.href.startsWith('/')) {
      navigate(item.href)
    } else {
      window.location.href = item.href
    }
  }

  const changeLanguage = (lang: string) => {
    i18n.changeLanguage(lang)
    setLangMenuOpen(false)
  }

  const handleLogout = () => {
    setMobileMenuOpen(false)
    logout()
    navigate('/login', { replace: true })
  }

  const authActionsDesktop = isAuthenticated ? (
    <>
      <Link to="/dashboard" className="landing-btn landing-btn-ghost">
        <LayoutDashboard size={16} />
        {t('landing.console')}
      </Link>
      <Link to="/cart" className="landing-btn landing-btn-ghost landing-cart-link">
        <ShoppingCart size={16} />
        {t('nav.cart')}
        {cartCount > 0 && <span className="landing-cart-badge">{cartCount > 99 ? '99+' : cartCount}</span>}
      </Link>
      <Link to="/profile" className="landing-user-chip" title={t('nav.profile')}>
        <User size={16} />
        <span className="landing-user-name">{user?.username || t('nav.profile')}</span>
      </Link>
      <button type="button" className="landing-icon-btn" onClick={handleLogout} title={t('common.logout')}>
        <LogOut size={18} />
      </button>
    </>
  ) : (
    <>
      {showLoginButton && (
        <Link to="/login" className="landing-btn landing-btn-ghost">
          {t('landing.login')}
        </Link>
      )}
      {showRegisterButton && (
        <Link to="/register" className="landing-btn landing-btn-accent">
          {t('landing.register')}
        </Link>
      )}
    </>
  )

  const authActionsMobile = isAuthenticated ? (
    <>
      <Link to="/dashboard" className="landing-mobile-link" onClick={() => setMobileMenuOpen(false)}>
        {t('landing.console')}
      </Link>
      <Link to="/cart" className="landing-mobile-link" onClick={() => setMobileMenuOpen(false)}>
        {t('nav.cart')}{cartCount > 0 ? ` (${cartCount})` : ''}
      </Link>
      <Link to="/profile" className="landing-mobile-link" onClick={() => setMobileMenuOpen(false)}>
        {t('nav.profile')} · {user?.username}
      </Link>
      <button type="button" className="landing-mobile-link landing-mobile-logout" onClick={handleLogout}>
        {t('common.logout')}
      </button>
    </>
  ) : (
    <>
      {showLoginButton && (
        <Link to="/login" className="landing-mobile-link" onClick={() => setMobileMenuOpen(false)}>
          {t('landing.login')}
        </Link>
      )}
      {showRegisterButton && (
        <Link to="/register" className="landing-mobile-link" onClick={() => setMobileMenuOpen(false)}>
          {t('landing.register')}
        </Link>
      )}
    </>
  )

  return (
    <header className={`landing-header ${scrolled ? 'scrolled' : ''}`}>
      <nav className="landing-nav">
        <Link to={isAuthenticated ? '/dashboard' : '/'} className="landing-logo">
          <span className="landing-logo-text">{siteName}</span>
        </Link>

        <div className="landing-nav-right">
          <nav className="landing-nav-desktop">
            {navItems.map((item, idx) => (
              <a
                key={idx}
                className="landing-nav-link"
                href={item.href}
                onClick={(e) => {
                  e.preventDefault()
                  handleNavClick(item)
                }}
              >
                {item.label}
              </a>
            ))}
          </nav>
          <div className="landing-nav-divider" />
          <div className="landing-nav-actions">
            <button
              type="button"
              className="landing-icon-btn"
              onClick={() => setLangMenuOpen(!langMenuOpen)}
              onBlur={() => setTimeout(() => setLangMenuOpen(false), 200)}
              title={t('landing.switchLanguage')}
            >
              <Globe size={18} />
              {langMenuOpen && (
                <div className="landing-dropdown-menu">
                  <button onClick={() => changeLanguage('zh')}>中文</button>
                  <button onClick={() => changeLanguage('en')}>English</button>
                </div>
              )}
            </button>
            <button
              type="button"
              className="landing-icon-btn"
              onClick={cycleTheme}
              title={t('landing.switchTheme')}
            >
              <ThemeIcon size={18} />
            </button>
            {authActionsDesktop}
          </div>
        </div>

        <button className="landing-mobile-toggle" onClick={() => setMobileMenuOpen(!mobileMenuOpen)}>
          {mobileMenuOpen ? <X size={22} /> : <Menu size={22} />}
        </button>
      </nav>

      {mobileMenuOpen && (
        <div className="landing-mobile-menu">
          {navItems.map((item, idx) => (
            <a
              key={idx}
              className="landing-mobile-link"
              href={item.href}
              onClick={(e) => {
                e.preventDefault()
                handleNavClick(item)
              }}
            >
              {item.label}
            </a>
          ))}
          <div className="landing-mobile-divider" />
          {authActionsMobile}
        </div>
      )}
    </header>
  )
}
