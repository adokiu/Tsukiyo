import { Link, useNavigate } from 'react-router-dom'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useAppStore } from '@/stores/app'
import { authApi } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toast'
import { Loader2 } from 'lucide-react'
import './LoginView.css'
import './LandingView.css'

export function LoginView() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { setToken, setUser } = useAuthStore()
  const toast = useToastStore()
  const showRegisterButton = useAppStore((s) => s.getThemeConfig('showRegisterButton'))
  const [loading, setLoading] = useState(false)
  const [form, setForm] = useState({ username: '', password: '' })

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    try {
      const res = await authApi.login(form)
      setToken(res.data.token)
      setUser(res.data.user)
      toast.success(t('auth.loginSuccess'))
      navigate('/dashboard')
    } catch (err: any) {
      toast.error(err.response?.data?.error || t('auth.loginFailed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="login-page">
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
      <div className="login-anim w-full max-w-[440px]" style={{ paddingLeft: '1.5rem', paddingRight: '1.5rem' }}>
        <div className="login-card">
          <div className="login-card-header">
            <div className="login-card-title">{t('auth.loginTitle')}</div>
            <div className="login-card-subtitle">{t('auth.loginSubtitle')}</div>
          </div>
          <div className="login-card-body">
            <form onSubmit={handleSubmit} className="login-field-group">
              <div className="login-field">
                <label className="login-label" htmlFor="username">{t('auth.username')}</label>
                <input
                  id="username"
                  type="text"
                  value={form.username}
                  onChange={(e) => setForm({ ...form, username: e.target.value })}
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
                  value={form.password}
                  onChange={(e) => setForm({ ...form, password: e.target.value })}
                  className="login-input"
                  placeholder={t('auth.passwordPlaceholder')}
                  required
                />
              </div>
              <button
                type="submit"
                disabled={loading}
                className="login-btn-primary"
              >
                {loading ? <Loader2 size={18} className="animate-spin" /> : null}
                <span>{t('auth.loginButton')}</span>
              </button>
            </form>

            {showRegisterButton && (
              <div className="login-footer">
                {t('auth.noAccount')} <Link to="/register">{t('auth.goRegister')}</Link>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
