import { useState, useEffect } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import apiClient from '@/api/client'
import { PageLayout } from '@/components/PageLayout/PageLayout'
import { Button } from '@/components/Button/Button'
import { Input } from '@/components/Input/Input'
import { Select } from '@/components/Select/Select'
import { useToastStore } from '@/stores/toast'
import { Save, Loader2 } from 'lucide-react'

interface ThemeConfigItem {
  key: string
  name: string | { [lang: string]: string }
  required?: boolean
  type: string
  options?: string
  default?: any
  help?: string | { [lang: string]: string }
}

interface ThemeConfigPage {
  key: string
  name: string | { [lang: string]: string }
  icon: string
  items: ThemeConfigItem[]
}

interface Theme {
  name: string
  short: string
  description: string
  version: string
  author: string
  url?: string
  preview?: string
  configuration: {
    type: string
    icon?: string
    name?: any
    data?: ThemeConfigItem[]
    pages?: ThemeConfigPage[]
  }
}

function getLocalizedText(value: string | { [lang: string]: string } | undefined, lang: string): string {
  if (!value) return ''
  if (typeof value === 'string') return value
  return value[lang] || value['en'] || value['zh'] || Object.values(value)[0] || ''
}

export default function ThemeSettingsPage() {
  const { pageKey } = useParams<{ pageKey: string }>()
  const { t, i18n } = useTranslation()
  const queryClient = useQueryClient()
  const toast = useToastStore()
  const [configValues, setConfigValues] = useState<Record<string, any>>({})

  const lang = i18n.language?.startsWith('zh') ? 'zh' : 'en'

  const { data: siteConfig } = useQuery({
    queryKey: ['site-config'],
    queryFn: async () => {
      const res = await apiClient.get('/settings/site')
      return res.data
    },
  })

  const { data: themes } = useQuery({
    queryKey: ['themes'],
    queryFn: async () => {
      const res = await apiClient.get('/theme/list')
      return res.data as Theme[]
    },
  })

  const currentThemeShort = siteConfig?.theme || 'default'
  const currentTheme = themes?.find((th) => th.short === currentThemeShort)

  const currentPage = currentTheme?.configuration?.pages?.find((p) => p.key === pageKey)

  const { data: savedSettings, isLoading } = useQuery({
    queryKey: ['theme-settings', currentThemeShort],
    queryFn: async () => {
      const res = await apiClient.get('/theme/settings', { params: { theme: currentThemeShort } })
      return res.data as Record<string, any>
    },
    enabled: !!currentThemeShort,
  })

  useEffect(() => {
    if (!currentPage || !savedSettings) return
    const merged: Record<string, any> = {}
    for (const item of currentPage.items) {
      merged[item.key] = savedSettings[item.key] !== undefined ? savedSettings[item.key] : item.default
    }
    setConfigValues(merged)
  }, [currentPage, savedSettings])

  const saveMutation = useMutation({
    mutationFn: ({ theme, settings }: { theme: string; settings: Record<string, any> }) =>
      apiClient.post('/theme/settings', settings, { params: { theme } }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['theme-settings'] })
      queryClient.invalidateQueries({ queryKey: ['site-config'] })
      toast.success(t('theme.configSaved'))
    },
    onError: (e: any) => {
      toast.error(e?.response?.data?.error || t('theme.configSaveFailed'))
    },
  })

  const handleSave = () => {
    saveMutation.mutate({ theme: currentThemeShort, settings: configValues })
  }

  const renderConfigItem = (item: ThemeConfigItem) => {
    const label = getLocalizedText(item.name, lang)
    const helpText = getLocalizedText(item.help, lang)
    const value = configValues[item.key]

    return (
      <div key={item.key} className="space-y-1.5">
        <label className="block text-sm font-medium text-secondary">
          {label || item.key}
          {item.required && <span className="text-red-500 ml-1">*</span>}
        </label>
        {helpText && <p className="text-xs text-tertiary">{helpText}</p>}
        {item.type === 'switch' ? (
          <button
            type="button"
            className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${value ? 'bg-[#087ed1]' : 'bg-gray-300 dark:bg-gray-600'}`}
            onClick={() => setConfigValues({ ...configValues, [item.key]: !value })}
          >
            <span className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${value ? 'translate-x-6' : 'translate-x-1'}`} />
          </button>
        ) : item.type === 'select' ? (
          <Select
            value={value ?? ''}
            options={(item.options || '').split(',').map((opt) => ({
              label: opt.trim(),
              value: opt.trim(),
            }))}
            onChange={(v) => setConfigValues({ ...configValues, [item.key]: String(v) })}
          />
        ) : item.type === 'number' ? (
          <Input
            type="number"
            value={value ?? 0}
            onChange={(e) => setConfigValues({ ...configValues, [item.key]: Number(e.target.value) })}
          />
        ) : (
          <Input
            type="text"
            value={value ?? ''}
            onChange={(e) => setConfigValues({ ...configValues, [item.key]: e.target.value })}
          />
        )}
      </div>
    )
  }

  if (isLoading) {
    return (
      <PageLayout>
        <div className="flex justify-center py-12">
          <Loader2 className="animate-spin text-tertiary" size={24} />
        </div>
      </PageLayout>
    )
  }

  if (!currentTheme) {
    return (
      <PageLayout>
        <div className="p-6 text-center text-tertiary">{t('theme.noTheme')}</div>
      </PageLayout>
    )
  }

  if (!currentPage) {
    return (
      <PageLayout>
        <div className="p-6 text-center text-tertiary">{t('theme.noConfig')}</div>
      </PageLayout>
    )
  }

  const pageTitle = getLocalizedText(currentPage.name, lang)

  return (
    <PageLayout
      leftSlot={<h2 className="text-lg font-semibold">{pageTitle}</h2>}
      rightSlot={
        <Button
          variant="primary"
          size="sm"
          icon={<Save size={16} />}
          loading={saveMutation.isPending}
          onClick={handleSave}
        >
          {t('theme.save')}
        </Button>
      }
    >
      <div className="page-transition__content" style={{ flex: 1, overflow: 'auto', padding: '24px' }}>
        <div className="max-w-2xl space-y-6">
          {currentPage.items.map((item) => renderConfigItem(item))}
        </div>
      </div>
    </PageLayout>
  )
}
