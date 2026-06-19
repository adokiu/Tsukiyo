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
        incus_remote_url: config.incus_remote_url,
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
          <div>
            <label className="block text-sm font-medium mb-2">Incus 镜像源</label>
            <input
              type="text"
              value={config?.incus_remote_url || ''}
              onChange={(e) => setConfig({ ...config!, incus_remote_url: e.target.value })}
              placeholder="images:"
              className="w-full px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <p className="text-xs text-muted-foreground mt-1">
              留空使用默认的 Incus 官方镜像源（images:）
            </p>
          </div>

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
