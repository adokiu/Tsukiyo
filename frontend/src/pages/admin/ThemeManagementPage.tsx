import { useState, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import apiClient from '@/api/client'
import { PageLayout } from '@/components/PageLayout/PageLayout'
import { Button } from '@/components/Button/Button'
import { Input } from '@/components/Input/Input'
import { Modal } from '@/components/Modal/Modal'
import { useToastStore } from '@/stores/toast'
import { Upload, Trash2, Check, Download, Loader2, Palette, ExternalLink, ImageIcon } from 'lucide-react'

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
    data?: any[]
    pages?: any[]
  }
}

export default function ThemeManagementPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const toast = useToastStore()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [importUrl, setImportUrl] = useState('')
  const [deleteTheme, setDeleteTheme] = useState<Theme | null>(null)

  const { data: themes, isLoading } = useQuery({
    queryKey: ['themes'],
    queryFn: async () => {
      const res = await apiClient.get('/theme/list')
      return res.data as Theme[]
    },
  })

  const { data: siteConfig } = useQuery({
    queryKey: ['site-config'],
    queryFn: async () => {
      const res = await apiClient.get('/settings/site')
      return res.data
    },
  })

  const uploadMutation = useMutation({
    mutationFn: async (file: File) => {
      const formData = new FormData()
      formData.append('file', file)
      const res = await apiClient.put('/theme/upload', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      })
      return res.data
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['themes'] })
      toast.success(t('theme.uploadSuccess'))
    },
    onError: (e: any) => {
      toast.error(e?.response?.data?.error || t('theme.uploadFailed'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (short: string) => apiClient.post('/theme/delete', { short }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['themes'] })
      toast.success(t('theme.deleteSuccess'))
      setDeleteTheme(null)
    },
    onError: (e: any) => {
      toast.error(e?.response?.data?.error || t('theme.deleteFailed'))
    },
  })

  const setThemeMutation = useMutation({
    mutationFn: (theme: string) => apiClient.get('/theme/set', { params: { theme } }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['site-config'] })
      queryClient.invalidateQueries({ queryKey: ['themes'] })
      toast.success(t('theme.switchSuccess'))
    },
    onError: (e: any) => {
      toast.error(e?.response?.data?.error || t('theme.switchFailed'))
    },
  })

  const importMutation = useMutation({
    mutationFn: (url: string) => apiClient.post('/theme/import', { url }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['themes'] })
      setImportUrl('')
      toast.success(t('theme.importSuccess'))
    },
    onError: (e: any) => {
      toast.error(e?.response?.data?.error || t('theme.importFailed'))
    },
  })

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) {
      uploadMutation.mutate(file)
    }
    e.target.value = ''
  }

  const currentTheme = siteConfig?.theme || 'default'

  const handleConfig = (theme: Theme) => {
    const firstPage = theme.configuration?.pages?.[0]
    if (firstPage) {
      navigate(`/admin/themeSettings/${firstPage.key}`)
    }
  }

  return (
    <PageLayout
      rightSlot={
        <div className="flex gap-2">
          <input
            ref={fileInputRef}
            type="file"
            accept=".zip"
            className="hidden"
            onChange={handleFileSelect}
          />
          <Button
            variant="primary"
            size="sm"
            icon={<Upload size={16} />}
            loading={uploadMutation.isPending}
            onClick={() => fileInputRef.current?.click()}
          >
            {t('theme.upload')}
          </Button>
        </div>
      }
    >
      <div className="page-transition__content" style={{ flex: 1, overflow: 'auto', padding: '24px' }}>
        {/* 导入区域 */}
        <div className="flex gap-2 mb-6">
          <Input
            className="flex-1"
            value={importUrl}
            onChange={(e) => setImportUrl(e.target.value)}
            placeholder={t('theme.importUrlPlaceholder')}
          />
          <Button
            variant="secondary"
            size="md"
            icon={<Download size={16} />}
            loading={importMutation.isPending}
            disabled={!importUrl}
            onClick={() => importUrl && importMutation.mutate(importUrl)}
          >
            {t('theme.import')}
          </Button>
        </div>

        {/* 主题卡片网格 */}
        {isLoading ? (
          <div className="flex justify-center py-12">
            <Loader2 className="animate-spin text-tertiary" size={24} />
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {themes?.map((theme) => (
              <div
                key={theme.short}
                className={`rounded-xl border p-5 transition-all ${
                  currentTheme === theme.short
                    ? 'border-[#087ed1] ring-2 ring-[#087ed1]/20'
                    : 'border-gray-200 dark:border-gray-700 hover:border-gray-400 dark:hover:border-gray-500'
                }`}
              >
                {/* 预览图 */}
                <div className="mb-4 h-32 rounded-lg overflow-hidden bg-gray-100 dark:bg-gray-800 flex items-center justify-center">
                  {theme.preview ? (
                    <img src={theme.preview} alt={theme.name} className="w-full h-full object-cover" />
                  ) : (
                    <ImageIcon size={32} className="text-tertiary" />
                  )}
                </div>

                {/* 主题信息 */}
                <div className="flex items-start justify-between mb-3">
                  <div className="flex items-center gap-2">
                    <Palette size={20} className="text-tertiary" />
                    <div>
                      <p className="font-medium">{theme.name}</p>
                      <p className="text-xs text-tertiary">v{theme.version} - {theme.author}</p>
                    </div>
                  </div>
                  {currentTheme === theme.short && (
                    <span className="inline-flex items-center gap-1 text-xs text-[#087ed1] font-medium">
                      <Check size={14} /> {t('theme.current')}
                    </span>
                  )}
                </div>

                <p className="text-sm text-tertiary mb-3 line-clamp-2">{theme.description}</p>

                {theme.url && (
                  <a
                    href={theme.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1 text-xs text-[#087ed1] hover:underline mb-3"
                  >
                    <ExternalLink size={12} />
                    {theme.url}
                  </a>
                )}

                {/* 操作按钮 */}
                <div className="flex gap-2">
                  {currentTheme !== theme.short && (
                    <Button
                      variant="primary"
                      size="sm"
                      icon={<Check size={14} />}
                      loading={setThemeMutation.isPending}
                      onClick={() => setThemeMutation.mutate(theme.short)}
                    >
                      {t('theme.activate')}
                    </Button>
                  )}
                  <Button
                    variant="secondary"
                    size="sm"
                    icon={<Palette size={14} />}
                    onClick={() => handleConfig(theme)}
                  >
                    {t('theme.config')}
                  </Button>
                  {theme.short !== 'default' && (
                    <Button
                      variant="danger"
                      size="sm"
                      icon={<Trash2 size={14} />}
                      loading={deleteMutation.isPending}
                      onClick={() => setDeleteTheme(theme)}
                    >
                      {t('theme.delete')}
                    </Button>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* 删除确认弹窗 */}
      <Modal
        open={!!deleteTheme}
        onClose={() => setDeleteTheme(null)}
        title={t('theme.deleteTitle')}
        confirmMode
        confirmText={t('theme.delete')}
        cancelText={t('theme.cancel')}
        confirmVariant="danger"
        onConfirm={() => deleteTheme && deleteMutation.mutate(deleteTheme.short)}
      >
        {t('theme.deleteConfirm', { name: deleteTheme?.name })}
      </Modal>
    </PageLayout>
  )
}
