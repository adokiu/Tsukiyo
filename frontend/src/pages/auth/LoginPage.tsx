import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { LogIn } from 'lucide-react'
import apiClient from '@/api/client'
import { useAuthStore } from '@/stores/auth'

export default function LoginPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const setToken = useAuthStore((s) => s.setToken)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError('')
    try {
      const res = await apiClient.post('/auth/login', { username, password })
      setToken(res.data.token)
      navigate('/admin/systemOverview', { replace: true })
    } catch (err: any) {
      setError(err.response?.data?.error || t('auth.loginFailed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen bg-background flex items-center justify-center p-6">
      <div className="w-full max-w-sm">
        <div className="text-center mb-10">
          <div className="mx-auto mb-5 flex h-14 w-14 items-center justify-center rounded-2xl bg-apple-blue/10">
            <LogIn className="text-apple-blue" size={28} />
          </div>
          <h1 className="text-2xl font-display font-semibold tracking-tight mb-1">{t('auth.loginTitle')}</h1>
          <p className="text-sm text-muted-foreground">{t('auth.loginSubtitle')}</p>
        </div>

        <form onSubmit={handleLogin} className="glass-card p-8 space-y-5">
          <div>
            <label className="apple-label">{t('auth.username')}</label>
            <input
              className="apple-input w-full"
              placeholder={t('auth.usernamePlaceholder')}
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
              autoFocus
            />
          </div>
          <div>
            <label className="apple-label">{t('auth.password')}</label>
            <input
              type="password"
              className="apple-input w-full"
              placeholder={t('auth.passwordPlaceholder')}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </div>

          {error && (
            <div className="rounded-xl bg-apple-red/10 px-4 py-3 text-sm text-apple-red">
              {error}
            </div>
          )}

          <button
            type="submit"
            className="apple-button w-full"
            disabled={loading}
          >
            {loading ? t('common.loading') : t('auth.loginButton')}
          </button>
        </form>
      </div>
    </div>
  )
}
