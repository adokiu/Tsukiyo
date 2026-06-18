import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Check, ChevronRight } from 'lucide-react'
import apiClient from '@/api/client'

interface InitForm {
  siteName: string
  siteSubtitle: string
  siteDescription: string
  adminUsername: string
  adminEmail: string
  adminPassword: string
  adminPasswordConfirm: string
}

export default function InitPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [step, setStep] = useState(0)
  const [checking, setChecking] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [form, setForm] = useState<InitForm>({
    siteName: '',
    siteSubtitle: '',
    siteDescription: '',
    adminUsername: '',
    adminEmail: '',
    adminPassword: '',
    adminPasswordConfirm: '',
  })

  useEffect(() => {
    apiClient
      .get('/init/status')
      .then((res) => {
        if (res.data.initialized) {
          navigate('/login', { replace: true })
        }
      })
      .catch(() => {})
      .finally(() => setChecking(false))
  }, [navigate])

  const update = (k: keyof InitForm, v: string) => {
    setForm((p) => ({ ...p, [k]: v }))
    setError('')
  }

  const next = () => {
    if (step === 1 && form.adminPassword !== form.adminPasswordConfirm) {
      setError(t('init.passwordMismatch'))
      return
    }
    if (step < 2) setStep(step + 1)
  }

  const submit = async () => {
    if (form.adminPassword !== form.adminPasswordConfirm) {
      setError(t('init.passwordMismatch'))
      return
    }
    setSubmitting(true)
    try {
      await apiClient.post('/init/setup', {
        site_name: form.siteName,
        site_subtitle: form.siteSubtitle || undefined,
        site_description: form.siteDescription || undefined,
        admin_username: form.adminUsername,
        admin_email: form.adminEmail,
        admin_password: form.adminPassword,
      })
      setStep(2)
      setTimeout(() => navigate('/login', { replace: true }), 2000)
    } catch (e: any) {
      setError(e.response?.data?.error || t('common.error'))
    } finally {
      setSubmitting(false)
    }
  }

  if (checking) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background">
        <div className="text-center">
          <div className="mx-auto mb-4 h-8 w-8 animate-spin rounded-full border-2 border-apple-blue border-t-transparent" />
          <p className="text-sm text-muted-foreground">{t('init.checking')}</p>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-background flex items-center justify-center p-6">
      <div className="w-full max-w-md">
        <div className="text-center mb-10">
          <h1 className="text-3xl font-display font-semibold tracking-tight mb-2">{t('init.title')}</h1>
          <p className="text-muted-foreground text-sm">{t('init.subtitle')}</p>
        </div>

        <div className="glass-card p-8">
          {step === 0 && (
            <div className="space-y-5">
              <div>
                <label className="apple-label">{t('init.siteName')}</label>
                <input
                  className="apple-input w-full"
                  placeholder={t('init.siteNamePlaceholder')}
                  value={form.siteName}
                  onChange={(e) => update('siteName', e.target.value)}
                />
              </div>
              <div>
                <label className="apple-label">{t('init.siteSubtitle')}</label>
                <input
                  className="apple-input w-full"
                  placeholder={t('init.siteSubtitlePlaceholder')}
                  value={form.siteSubtitle}
                  onChange={(e) => update('siteSubtitle', e.target.value)}
                />
              </div>
              <div>
                <label className="apple-label">{t('init.siteDescription')}</label>
                <textarea
                  className="apple-input w-full min-h-[80px] resize-none"
                  placeholder={t('init.siteDescriptionPlaceholder')}
                  value={form.siteDescription}
                  onChange={(e) => update('siteDescription', e.target.value)}
                />
              </div>
            </div>
          )}

          {step === 1 && (
            <div className="space-y-5">
              <div>
                <label className="apple-label">{t('init.adminUsername')}</label>
                <input
                  className="apple-input w-full"
                  placeholder={t('init.adminUsernamePlaceholder')}
                  value={form.adminUsername}
                  onChange={(e) => update('adminUsername', e.target.value)}
                />
              </div>
              <div>
                <label className="apple-label">{t('init.adminEmail')}</label>
                <input
                  className="apple-input w-full"
                  placeholder={t('init.adminEmailPlaceholder')}
                  value={form.adminEmail}
                  onChange={(e) => update('adminEmail', e.target.value)}
                />
              </div>
              <div>
                <label className="apple-label">{t('init.adminPassword')}</label>
                <input
                  type="password"
                  className="apple-input w-full"
                  placeholder={t('init.adminPasswordPlaceholder')}
                  value={form.adminPassword}
                  onChange={(e) => update('adminPassword', e.target.value)}
                />
              </div>
              <div>
                <label className="apple-label">{t('init.adminPasswordConfirm')}</label>
                <input
                  type="password"
                  className="apple-input w-full"
                  placeholder={t('init.adminPasswordConfirmPlaceholder')}
                  value={form.adminPasswordConfirm}
                  onChange={(e) => update('adminPasswordConfirm', e.target.value)}
                />
              </div>
            </div>
          )}

          {step === 2 && (
            <div className="text-center py-4">
              <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-apple-green/10">
                <Check className="text-apple-green" size={28} />
              </div>
              <h3 className="text-lg font-semibold mb-1">{t('init.setupComplete')}</h3>
              <p className="text-sm text-muted-foreground">{t('init.setupCompleteDesc')}</p>
            </div>
          )}

          {error && (
            <div className="mt-5 rounded-xl bg-apple-red/10 px-4 py-3 text-sm text-apple-red">
              {error}
            </div>
          )}

          {step < 2 && (
            <div className="mt-8 flex gap-3">
              {step > 0 && (
                <button className="apple-button-secondary flex-1" onClick={() => setStep(step - 1)}>
                  {t('common.back')}
                </button>
              )}
              {step === 0 ? (
                <button className="apple-button flex-1" onClick={next}>
                  {t('common.next')}
                  <ChevronRight size={16} className="ml-1" />
                </button>
              ) : (
                <button className="apple-button flex-1" onClick={submit} disabled={submitting}>
                  {submitting ? t('common.loading') : t('common.submit')}
                </button>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
