import { Link, useNavigate } from 'react-router-dom'
import { useState, useEffect, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useAppStore } from '@/stores/app'
import { authApi } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toast'
import { Loader2 } from 'lucide-react'
import { LandingHeader } from '@/components/LandingHeader'
import './LandingView.css'
import './LoginView.css'

interface CtaItem {
  label: string
  href: string
  variant: string
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

function parseCtaItems(raw: string): CtaItem[] {
  return parseJsonArray<CtaItem>(
    raw,
    (item) => ({ label: String(item.label ?? ''), href: String(item.href ?? ''), variant: String(item.variant ?? 'ghost'), external: !!item.external }),
    (item) => !!item.label && !!item.href
  )
}

interface LandingViewProps {
  mode?: 'login' | 'register'
}

export function LandingView({ mode }: LandingViewProps = {}) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { getThemeConfig, fetchPublicSettings } = useAppStore()
  const { setToken, setUser } = useAuthStore()
  const toast = useToastStore()
  const [loginLoading, setLoginLoading] = useState(false)
  const [registerLoading, setRegisterLoading] = useState(false)
  const [sendCodeLoading, setSendCodeLoading] = useState(false)
  const [codeCountdown, setCodeCountdown] = useState(0)
  const [loginForm, setLoginForm] = useState({ username: '', password: '' })
  const [registerForm, setRegisterForm] = useState({ username: '', email: '', password: '', code: '' })

  const siteName = getThemeConfig('siteName')
  const footerText = getThemeConfig('footerText')
  const showRegisterButton = getThemeConfig('showRegisterButton')

  const enableRegister = useAppStore((s) => s.publicSettings?.enable_register ?? true)
  const forceEmailVerify = useAppStore((s) => s.publicSettings?.force_email_verify ?? false)

  const heroTitle = getThemeConfig('heroTitle')
  const heroDescription = getThemeConfig('heroDescription')
  const heroCtaItemsRaw = getThemeConfig('heroCtaItems')

  const heroCharacterUrl = getThemeConfig('heroCharacterUrl')

  const ctaItems = useMemo(() => parseCtaItems(heroCtaItemsRaw), [heroCtaItemsRaw])

  useEffect(() => {
    const POLL_INTERVAL = 10000
    let timer: ReturnType<typeof setInterval> | null = null

    const startPolling = () => {
      if (timer) return
      timer = setInterval(() => {
        fetchPublicSettings()
      }, POLL_INTERVAL)
    }

    const stopPolling = () => {
      if (timer) {
        clearInterval(timer)
        timer = null
      }
    }

    const onVisibilityChange = () => {
      if (document.hidden) {
        stopPolling()
      } else {
        fetchPublicSettings()
        startPolling()
      }
    }

    startPolling()
    document.addEventListener('visibilitychange', onVisibilityChange)

    return () => {
      stopPolling()
      document.removeEventListener('visibilitychange', onVisibilityChange)
    }
  }, [fetchPublicSettings])

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoginLoading(true)
    try {
      const res = await authApi.login(loginForm)
      setToken(res.data.token)
      setUser(res.data.user)
      toast.success(t('auth.loginSuccess'))
      navigate('/dashboard')
    } catch (err: any) {
      toast.error(err.response?.data?.error || t('auth.loginFailed'))
    } finally {
      setLoginLoading(false)
    }
  }

  const handleSendCode = async () => {
    if (!registerForm.email) {
      toast.error(t('auth.emailRequired'))
      return
    }
    setSendCodeLoading(true)
    try {
      await authApi.sendRegisterCode(registerForm.email)
      toast.success(t('auth.codeSent'))
      setCodeCountdown(60)
      const timer = setInterval(() => {
        setCodeCountdown((prev) => {
          if (prev <= 1) {
            clearInterval(timer)
            return 0
          }
          return prev - 1
        })
      }, 1000)
    } catch (err: any) {
      toast.error(err.response?.data?.error || t('auth.codeSendFailed'))
    } finally {
      setSendCodeLoading(false)
    }
  }

  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault()
    setRegisterLoading(true)
    try {
      await authApi.register(registerForm)
      toast.success(t('auth.registerSuccess'))
      navigate('/login')
    } catch (err: any) {
      toast.error(err.response?.data?.error || t('auth.registerFailed'))
    } finally {
      setRegisterLoading(false)
    }
  }

  return (
    <main className="landing-page min-h-screen">
      {/* 贝塞尔曲线背景 */}
      <div className="landing-curve-bg">
        <svg className="landing-curve-bottom" viewBox="0 0 1440 600" preserveAspectRatio="none">
          <path d="M0,400 Q360,280 720,340 T1440,290 L1440,600 L0,600 Z" fill="hsl(210 80% 60%)" opacity={0.15} />
          <path d="M0,460 Q480,360 960,430 T1440,390 L1440,600 L0,600 Z" fill="hsl(210 80% 60%)" opacity={0.08} />
        </svg>
        <svg className="landing-curve-top" viewBox="0 0 1440 300" preserveAspectRatio="none">
          <path d="M0,80 Q720,180 1440,60 L1440,0 L0,0 Z" fill="hsl(210 80% 60%)" opacity={0.1} />
        </svg>
      </div>
      {/* 导航栏 - 登录/注册模式不显示 */}
      {!mode && <LandingHeader />}

      {/* Hero 区域 / 登录注册卡片 */}
      <section className="landing-hero">
        {mode === 'login' ? (
          <div className="landing-hero-content" style={{ alignItems: 'center', justifyContent: 'center' }}>
            <div className="login-anim" style={{ width: '100%', maxWidth: '440px', paddingLeft: '1.5rem', paddingRight: '1.5rem' }}>
              <div className="login-card">
                <div className="login-card-header">
                  <div className="login-card-title">{t('auth.loginTitle')}</div>
                  <div className="login-card-subtitle">{t('auth.loginSubtitle')}</div>
                </div>
                <div className="login-card-body">
                  <form onSubmit={handleLogin} className="login-field-group">
                    <div className="login-field">
                      <label className="login-label" htmlFor="username">{t('auth.username')}</label>
                      <input
                        id="username"
                        type="text"
                        value={loginForm.username}
                        onChange={(e) => setLoginForm({ ...loginForm, username: e.target.value })}
                        className="login-input"
                        placeholder={t('auth.usernamePlaceholder')}
                        required
                      />
                    </div>
                    <div className="login-field">
                      <div className="login-field-row">
                        <label className="login-label" htmlFor="password">{t('auth.password')}</label>
                      </div>
                      <input
                        id="password"
                        type="password"
                        value={loginForm.password}
                        onChange={(e) => setLoginForm({ ...loginForm, password: e.target.value })}
                        className="login-input"
                        placeholder={t('auth.passwordPlaceholder')}
                        required
                      />
                    </div>
                    <button type="submit" disabled={loginLoading} className="login-btn-primary">
                      {loginLoading ? <Loader2 size={18} className="animate-spin" /> : null}
                      <span>{t('auth.loginButton')}</span>
                    </button>
                  </form>
                  {showRegisterButton && enableRegister && (
                    <div className="login-footer">
                      {t('auth.noAccount')} <Link to="/register">{t('auth.goRegister')}</Link>
                    </div>
                  )}
                </div>
              </div>
            </div>
          </div>
        ) : mode === 'register' ? (
          <div className="landing-hero-content" style={{ alignItems: 'center', justifyContent: 'center' }}>
            <div className="login-anim" style={{ width: '100%', maxWidth: '440px', paddingLeft: '1.5rem', paddingRight: '1.5rem' }}>
              <div className="login-card">
                <div className="login-card-header">
                  <div className="login-card-title">{t('auth.registerTitle')}</div>
                  <div className="login-card-subtitle">{t('auth.registerSubtitle')}</div>
                </div>
                <div className="login-card-body">
                  {!enableRegister ? (
                    <div style={{ textAlign: 'center', padding: '2rem 0' }}>
                      <p style={{ color: 'var(--color-gray-600)', marginBottom: '1rem' }}>{t('auth.registrationClosed')}</p>
                      <Link to="/login" className="login-btn-primary" style={{ display: 'inline-block', textDecoration: 'none' }}>
                        {t('auth.backToLogin')}
                      </Link>
                    </div>
                  ) : (
                  <>
                  <form onSubmit={handleRegister} className="login-field-group">
                    <div className="login-field">
                      <label className="login-label" htmlFor="reg-username">{t('auth.username')}</label>
                      <input
                        id="reg-username"
                        type="text"
                        value={registerForm.username}
                        onChange={(e) => setRegisterForm({ ...registerForm, username: e.target.value })}
                        className="login-input"
                        placeholder="3-64"
                        required
                        minLength={3}
                        maxLength={64}
                      />
                    </div>
                    <div className="login-field">
                      <label className="login-label" htmlFor="reg-email">{t('auth.email')}</label>
                      <input
                        id="reg-email"
                        type="email"
                        value={registerForm.email}
                        onChange={(e) => setRegisterForm({ ...registerForm, email: e.target.value })}
                        className="login-input"
                        placeholder={t('auth.emailPlaceholder')}
                        required
                      />
                    </div>
                    {forceEmailVerify && (
                    <div className="login-field">
                      <label className="login-label" htmlFor="reg-code">{t('auth.verifyCode')}</label>
                      <div style={{ display: 'flex', gap: '8px' }}>
                        <input
                          id="reg-code"
                          type="text"
                          value={registerForm.code}
                          onChange={(e) => setRegisterForm({ ...registerForm, code: e.target.value })}
                          className="login-input"
                          placeholder={t('auth.verifyCodePlaceholder')}
                          required
                          maxLength={6}
                          style={{ flex: 1 }}
                        />
                        <button
                          type="button"
                          onClick={handleSendCode}
                          disabled={sendCodeLoading || codeCountdown > 0 || !registerForm.email}
                          className="login-btn-primary"
                          style={{ flexShrink: 0, padding: '0 16px', fontSize: '13px', whiteSpace: 'nowrap' }}
                        >
                          {sendCodeLoading ? <Loader2 size={16} className="animate-spin" /> : null}
                          <span>{codeCountdown > 0 ? `${codeCountdown}s` : t('auth.sendCode')}</span>
                        </button>
                      </div>
                    </div>
                    )}
                    <div className="login-field">
                      <label className="login-label" htmlFor="reg-password">{t('auth.password')}</label>
                      <input
                        id="reg-password"
                        type="password"
                        value={registerForm.password}
                        onChange={(e) => setRegisterForm({ ...registerForm, password: e.target.value })}
                        className="login-input"
                        placeholder={t('auth.passwordPlaceholder')}
                        required
                        minLength={8}
                      />
                    </div>
                    <button type="submit" disabled={registerLoading} className="login-btn-primary">
                      {registerLoading ? <Loader2 size={18} className="animate-spin" /> : null}
                      <span>{t('auth.registerButton')}</span>
                    </button>
                  </form>
                  <div className="login-footer">
                    {t('auth.hasAccount')} <Link to="/login">{t('auth.goLogin')}</Link>
                  </div>
                  </>
                  )}
                </div>
              </div>
            </div>
          </div>
        ) : (
          <div className="landing-hero-content">
            <div className="landing-hero-text">
              <h1 className="landing-hero-title">{heroTitle}</h1>
              <p className="landing-hero-description">{heroDescription}</p>
              <div className="landing-hero-cta">
                {ctaItems.map((item, idx) => (
                  <a
                    key={idx}
                    className={`landing-btn ${item.variant === 'accent' ? 'landing-btn-accent' : 'landing-btn-ghost'}`}
                    href={item.href}
                    onClick={(e) => {
                      e.preventDefault()
                      if (item.external) {
                        window.open(item.href, '_blank')
                      } else if (item.href.startsWith('/')) {
                        navigate(item.href)
                      } else {
                        window.location.href = item.href
                      }
                    }}
                  >
                    {item.label}
                  </a>
                ))}
              </div>
            </div>
          </div>
        )}

        {/* 背景装饰 */}
        <div className="landing-hero-bg">
          {heroCharacterUrl && (
            <div className="landing-hero-character-wrapper">
              <img
                src={heroCharacterUrl}
                alt="Character"
                className="landing-hero-character"
              />
            </div>
          )}
        </div>
      </section>

      {/* 页脚 - 登录/注册模式不显示 */}
      {!mode && (
      <div className="landing-footer">
        <div className="landing-footer-inner">
          {footerText || `${siteName} - Powered by Tsukiyo`}
        </div>
      </div>
      )}
    </main>
  )
}
