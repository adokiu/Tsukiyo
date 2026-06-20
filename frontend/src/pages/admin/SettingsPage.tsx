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
}

export default function SettingsPage() {
  const { t } = useTranslation()
  const [config, setConfig] = useState<SiteConfig | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    apiClient.get('/settings/site').then((res) => setConfig(res.data)).finally(() => setLoading(false))
  }, [])

  const handleSave = async () => {
    if (!config) return
    setSaving(true)
    try {
      await apiClient.put('/settings/site', {
        site_name: config.site_name,
        site_subtitle: config.site_subtitle,
        site_description: config.site_description,
        site_url: config.site_url,
        contact_email: config.contact_email,
      })
      alert('保存成功')
    } catch (error) {
      alert('保存失败')
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

      <div className="glass-card p-6 max-w-2xl">
        <div className="space-y-6">
          <p className="text-sm text-muted-foreground">镜像源配置已移至「实例模板管理」页面</p>

          <div className="pt-4">
            <button
              onClick={handleSave}
              disabled={saving}
              className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {saving ? <Loader2 size={18} className="animate-spin" /> : <Save size={18} />}
              {saving ? '保存中...' : '保存'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
