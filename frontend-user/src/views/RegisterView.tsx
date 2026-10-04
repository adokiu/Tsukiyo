import { Link, useNavigate } from 'react-router-dom'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { authApi } from '@/api/auth'
import { useToastStore } from '@/stores/toast'
import { Loader2 } from 'lucide-react'

export function RegisterView() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const toast = useToastStore()
  const [loading, setLoading] = useState(false)
  const [form, setForm] = useState({ username: '', email: '', password: '' })

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    try {
      await authApi.register(form)
      toast.success(t('auth.registerSuccess'))
      navigate('/login')
    } catch (err: any) {
      toast.error(err.response?.data?.error || t('auth.registerFailed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center px-4">
      <div className="w-full max-w-md card p-8">
        <h1 className="text-2xl font-semibold text-center mb-2 text-primary">{t('auth.registerTitle')}</h1>
        <p className="text-sm text-tertiary text-center mb-6">{t('auth.registerSubtitle')}</p>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="apple-label">{t('auth.username')}</label>
            <input
              type="text"
              value={form.username}
              onChange={(e) => setForm({ ...form, username: e.target.value })}
              className="apple-input w-full"
              placeholder="3-64"
              required
              minLength={3}
              maxLength={64}
            />
          </div>
          <div>
            <label className="apple-label">{t('auth.email')}</label>
            <input
              type="email"
              value={form.email}
              onChange={(e) => setForm({ ...form, email: e.target.value })}
              className="apple-input w-full"
              placeholder={t('auth.emailPlaceholder')}
              required
            />
          </div>
          <div>
            <label className="apple-label">{t('auth.password')}</label>
            <input
              type="password"
              value={form.password}
              onChange={(e) => setForm({ ...form, password: e.target.value })}
              className="apple-input w-full"
              placeholder={t('auth.passwordPlaceholder')}
              required
              minLength={8}
            />
          </div>
          <button
            type="submit"
            disabled={loading}
            className="btn btn--primary btn--md w-full disabled:opacity-50"
          >
            {loading ? <Loader2 size={18} className="btn__spinner" /> : null}
            <span>{t('auth.registerButton')}</span>
          </button>
        </form>

        <p className="text-sm text-tertiary text-center mt-6">
          {t('auth.hasAccount')}{' '}
          <Link to="/login" className="text-secondary hover:text-primary transition-colors">
            {t('auth.goLogin')}
          </Link>
        </p>
      </div>
    </div>
  )
}
