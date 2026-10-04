import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api/auth'
import { useTranslation } from 'react-i18next'
import { useState } from 'react'
import { Loader2, User, Mail, Lock } from 'lucide-react'
import { useToastStore } from '@/stores/toast'

export function ProfileView() {
  const { t } = useTranslation()
  const toast = useToastStore()
  const { user } = useAuthStore()
  const [loading, setLoading] = useState(false)
  const [form, setForm] = useState({ old_password: '', new_password: '' })

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    try {
      await authApi.changePassword(form)
      toast.success(t('auth.passwordChanged'))
      setForm({ old_password: '', new_password: '' })
    } catch (err: any) {
      toast.error(err.response?.data?.error || t('common.error'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="page-container">
      <div className="max-w-2xl mx-auto space-y-4" style={{ animation: 'page-enter 0.25s ease' }}>
        <div className="page-header">
          <h1 className="page-title">{t('profile.title')}</h1>
        </div>

        <div className="card p-6">
          <h2 className="text-lg font-semibold mb-4 text-primary">{t('profile.accountInfo')}</h2>
          <div className="space-y-3">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-full bg-surface-secondary flex items-center justify-center">
                <User size={18} className="text-tertiary" />
              </div>
              <div>
                <p className="text-sm text-tertiary">{t('profile.username')}</p>
                <p className="font-medium text-primary">{user?.username}</p>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-full bg-surface-secondary flex items-center justify-center">
                <Mail size={18} className="text-tertiary" />
              </div>
              <div>
                <p className="text-sm text-tertiary">{t('profile.email')}</p>
                <p className="font-medium text-primary">{user?.email}</p>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-full bg-surface-secondary flex items-center justify-center">
                <Lock size={18} className="text-tertiary" />
              </div>
              <div>
                <p className="text-sm text-tertiary">{t('profile.status')}</p>
                <p className="font-medium text-primary">{user?.status}</p>
              </div>
            </div>
          </div>
        </div>

        <div className="card p-6">
          <h2 className="text-lg font-semibold mb-4 text-primary">{t('profile.changePassword')}</h2>
          <form onSubmit={handleChangePassword} className="space-y-4">
            <div>
              <label className="apple-label">{t('profile.oldPassword')}</label>
              <input
                type="password"
                value={form.old_password}
                onChange={(e) => setForm({ ...form, old_password: e.target.value })}
                className="apple-input w-full"
                placeholder={t('profile.oldPasswordPlaceholder')}
                required
              />
            </div>
            <div>
              <label className="apple-label">{t('profile.newPassword')}</label>
              <input
                type="password"
                value={form.new_password}
                onChange={(e) => setForm({ ...form, new_password: e.target.value })}
                className="apple-input w-full"
                placeholder={t('profile.newPasswordPlaceholder')}
                required
                minLength={8}
              />
            </div>
            <button type="submit" disabled={loading} className="btn btn--primary btn--md disabled:opacity-50">
              {loading ? <Loader2 size={16} className="btn__spinner" /> : null}
              <span>{t('profile.submit')}</span>
            </button>
          </form>
        </div>
      </div>
    </div>
  )
}
