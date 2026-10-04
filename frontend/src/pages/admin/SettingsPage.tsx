import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Settings, Save, Loader2 } from 'lucide-react'
import apiClient from '@/api/client'

interface SiteConfig {
  id: string
  site_name: string
  site_subtitle: string
  site_description: string
  site_url: string
  contact_email: string
  incus_remote_url: string
  auto_release_days: number
  allow_registration: boolean
  force_email_verify: boolean
}

export default function SettingsPage() {
  const { t } = useTranslation()
  const [config, setConfig] = useState<SiteConfig | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [saveResult, setSaveResult] = useState<{ success: boolean; message: string } | null>(null)

  useEffect(() => {
    apiClient.get('/settings/site').then((res) => setConfig(res.data)).finally(() => setLoading(false))
  }, [])

  const handleSave = async () => {
    if (!config) return
    setSaving(true)
    setSaveResult(null)
    try {
      await apiClient.put('/settings/site', {
        site_name: config.site_name,
        site_subtitle: config.site_subtitle,
        site_description: config.site_description,
        site_url: config.site_url,
        contact_email: config.contact_email,
        incus_remote_url: config.incus_remote_url,
        allow_registration: config.allow_registration,
        force_email_verify: config.force_email_verify,
      })
      setSaveResult({ success: true, message: t('settings.saveSuccess') })
    } catch (error: any) {
      setSaveResult({ success: false, message: error.response?.data?.error || t('settings.saveFailed') })
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return (
      <div className="p-8">
        <div className="flex items-center gap-2 text-muted-foreground">
          <Loader2 size={18} className="animate-spin" />
          <span className="text-sm">加载中...</span>
        </div>
      </div>
    )
  }

  return (
    <div className="p-8">
      <h1 className="text-2xl font-display font-semibold tracking-tight mb-8 flex items-center gap-2">
        <Settings size={24} />
        {t('settings.title')}
      </h1>

      {saveResult && (
        <div className={`mb-4 p-3 rounded-lg text-sm ${saveResult.success ? 'bg-green-50 text-green-700' : 'bg-red-50 text-red-700'}`}>
          {saveResult.message}
        </div>
      )}

      <div className="glass-card p-6 max-w-2xl space-y-6">
        {/* 站点基本信息 */}
        <div>
          <h3 className="text-sm font-medium mb-4 text-muted-foreground">{t('settings.siteInfo')}</h3>
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium mb-1">{t('settings.siteName')}</label>
              <input
                type="text"
                value={config?.site_name ?? ''}
                onChange={(e) => setConfig(prev => prev ? { ...prev, site_name: e.target.value } : prev)}
                className="w-full px-3 py-2 rounded-lg border border-border bg-input text-sm"
                placeholder="Tsukiyo"
              />
            </div>
            <div>
              <label className="block text-sm font-medium mb-1">{t('settings.siteSubtitle')}</label>
              <input
                type="text"
                value={config?.site_subtitle ?? ''}
                onChange={(e) => setConfig(prev => prev ? { ...prev, site_subtitle: e.target.value } : prev)}
                className="w-full px-3 py-2 rounded-lg border border-border bg-input text-sm"
                placeholder={t('settings.siteSubtitlePlaceholder')}
              />
            </div>
            <div>
              <label className="block text-sm font-medium mb-1">{t('settings.siteDescription')}</label>
              <textarea
                value={config?.site_description ?? ''}
                onChange={(e) => setConfig(prev => prev ? { ...prev, site_description: e.target.value } : prev)}
                className="w-full px-3 py-2 rounded-lg border border-border bg-input text-sm min-h-[80px]"
                placeholder={t('settings.siteDescriptionPlaceholder')}
              />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium mb-1">{t('settings.siteURL')}</label>
                <input
                  type="text"
                  value={config?.site_url ?? ''}
                  onChange={(e) => setConfig(prev => prev ? { ...prev, site_url: e.target.value } : prev)}
                  className="w-full px-3 py-2 rounded-lg border border-border bg-input text-sm"
                  placeholder="https://example.com"
                />
              </div>
              <div>
                <label className="block text-sm font-medium mb-1">{t('settings.contactEmail')}</label>
                <input
                  type="email"
                  value={config?.contact_email ?? ''}
                  onChange={(e) => setConfig(prev => prev ? { ...prev, contact_email: e.target.value } : prev)}
                  className="w-full px-3 py-2 rounded-lg border border-border bg-input text-sm"
                  placeholder="admin@example.com"
                />
              </div>
            </div>
          </div>
        </div>

        {/* 注册设置 */}
        <div className="pt-4 border-t border-border">
          <h3 className="text-sm font-medium mb-4 text-muted-foreground">{t('settings.registrationSettings')}</h3>
          <div className="space-y-4">
            <label className="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                checked={config?.allow_registration ?? true}
                onChange={(e) => setConfig(prev => prev ? { ...prev, allow_registration: e.target.checked } : prev)}
                className="w-4 h-4 rounded"
              />
              <div>
                <span className="text-sm font-medium">{t('settings.allowRegistration')}</span>
                <p className="text-xs text-muted-foreground mt-0.5">{t('settings.allowRegistrationDesc')}</p>
              </div>
            </label>
            <label className="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                checked={config?.force_email_verify ?? false}
                onChange={(e) => setConfig(prev => prev ? { ...prev, force_email_verify: e.target.checked } : prev)}
                className="w-4 h-4 rounded"
              />
              <div>
                <span className="text-sm font-medium">{t('settings.forceEmailVerify')}</span>
                <p className="text-xs text-muted-foreground mt-0.5">{t('settings.forceEmailVerifyDesc')}</p>
              </div>
            </label>
          </div>
        </div>

        {/* 保存按钮 */}
        <div className="pt-4 border-t border-border">
          <button
            onClick={handleSave}
            disabled={saving}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {saving ? <Loader2 size={18} className="animate-spin" /> : <Save size={18} />}
            {saving ? t('settings.saving') : t('settings.save')}
          </button>
        </div>
      </div>
    </div>
  )
}
