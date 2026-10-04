import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Mail, Save, Loader2, Send } from 'lucide-react'
import apiClient from '@/api/client'

interface SMTPConfig {
  id?: string
  host: string
  port: number
  username: string
  password?: string
  from_name: string
  from_email: string
  encryption: string
  enabled: boolean
  verify_email: boolean
}

export default function PushSettingsPage() {
  const { t } = useTranslation()
  const [config, setConfig] = useState<SMTPConfig | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [testEmail, setTestEmail] = useState('')
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null)

  useEffect(() => {
    apiClient.get('/settings/smtp').then((res) => setConfig(res.data)).finally(() => setLoading(false))
  }, [])

  const handleSave = async () => {
    if (!config) return
    setSaving(true)
    try {
      await apiClient.put('/settings/smtp', {
        host: config.host,
        port: config.port,
        username: config.username,
        password: config.password || undefined,
        from_name: config.from_name,
        from_email: config.from_email,
        encryption: config.encryption,
        enabled: config.enabled,
        verify_email: config.verify_email,
      })
      setTestResult({ success: true, message: '保存成功' })
      setConfig((prev) => prev ? { ...prev, password: '' } : prev)
    } catch (error: any) {
      setTestResult({ success: false, message: error.response?.data?.error || '保存失败' })
    } finally {
      setSaving(false)
    }
  }

  const handleTest = async () => {
    if (!testEmail) return
    setTesting(true)
    setTestResult(null)
    try {
      await apiClient.post('/settings/smtp/test', { to: testEmail })
      setTestResult({ success: true, message: '测试邮件已发送' })
    } catch (error: any) {
      setTestResult({ success: false, message: error.response?.data?.error || '发送失败' })
    } finally {
      setTesting(false)
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
        <Mail size={24} />
        {t('nav.pushSettings')}
      </h1>

      {testResult && (
        <div className={`mb-4 p-3 rounded-lg text-sm ${testResult.success ? 'bg-green-50 text-green-700' : 'bg-red-50 text-red-700'}`}>
          {testResult.message}
        </div>
      )}

      <div className="glass-card p-6 max-w-2xl space-y-6">
        <div className="flex items-center gap-3">
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              checked={config?.enabled ?? false}
              onChange={(e) => setConfig(prev => prev ? { ...prev, enabled: e.target.checked } : prev)}
              className="w-4 h-4 rounded"
            />
            <span className="text-sm font-medium">启用 SMTP 邮件推送</span>
          </label>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium mb-1">SMTP 服务器</label>
            <input
              type="text"
              value={config?.host ?? ''}
              onChange={(e) => setConfig(prev => prev ? { ...prev, host: e.target.value } : prev)}
              placeholder="smtp.example.com"
              className="w-full px-3 py-2 rounded-lg border border-border bg-input text-sm"
            />
          </div>
          <div>
            <label className="block text-sm font-medium mb-1">端口</label>
            <input
              type="number"
              value={config?.port ?? 587}
              onChange={(e) => setConfig(prev => prev ? { ...prev, port: parseInt(e.target.value) || 587 } : prev)}
              className="w-full px-3 py-2 rounded-lg border border-border bg-input text-sm"
            />
          </div>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium mb-1">用户名</label>
            <input
              type="text"
              value={config?.username ?? ''}
              onChange={(e) => setConfig(prev => prev ? { ...prev, username: e.target.value } : prev)}
              placeholder="user@example.com"
              className="w-full px-3 py-2 rounded-lg border border-border bg-input text-sm"
            />
          </div>
          <div>
            <label className="block text-sm font-medium mb-1">密码 / 授权码</label>
            <input
              type="password"
              value={config?.password ?? ''}
              onChange={(e) => setConfig(prev => prev ? { ...prev, password: e.target.value } : prev)}
              placeholder="留空则不修改"
              className="w-full px-3 py-2 rounded-lg border border-border bg-input text-sm"
            />
          </div>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium mb-1">发件人名称</label>
            <input
              type="text"
              value={config?.from_name ?? ''}
              onChange={(e) => setConfig(prev => prev ? { ...prev, from_name: e.target.value } : prev)}
              placeholder="Tsukiyo"
              className="w-full px-3 py-2 rounded-lg border border-border bg-input text-sm"
            />
          </div>
          <div>
            <label className="block text-sm font-medium mb-1">发件人邮箱</label>
            <input
              type="email"
              value={config?.from_email ?? ''}
              onChange={(e) => setConfig(prev => prev ? { ...prev, from_email: e.target.value } : prev)}
              placeholder="noreply@example.com"
              className="w-full px-3 py-2 rounded-lg border border-border bg-input text-sm"
            />
          </div>
        </div>

        <div>
          <label className="block text-sm font-medium mb-1">加密方式</label>
          <select
            value={config?.encryption ?? 'starttls'}
            onChange={(e) => setConfig(prev => prev ? { ...prev, encryption: e.target.value } : prev)}
            className="w-full px-3 py-2 rounded-lg border border-border bg-input text-sm"
          >
            <option value="none">无加密</option>
            <option value="starttls">STARTTLS</option>
            <option value="ssl">SSL/TLS</option>
          </select>
        </div>

        <div className="flex items-center gap-3 pt-2 border-t border-border">
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              checked={config?.verify_email ?? false}
              onChange={(e) => setConfig(prev => prev ? { ...prev, verify_email: e.target.checked } : prev)}
              className="w-4 h-4 rounded"
            />
            <span className="text-sm font-medium">启用注册邮箱验证</span>
          </label>
          <span className="text-xs text-muted-foreground">启用后，新注册用户需点击邮件中的链接验证邮箱才能登录</span>
        </div>

        <div className="flex items-center gap-3 pt-4">
          <button
            onClick={handleSave}
            disabled={saving}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {saving ? <Loader2 size={18} className="animate-spin" /> : <Save size={18} />}
            {saving ? '保存中...' : '保存'}
          </button>
        </div>

        <div className="pt-4 border-t border-border">
          <h3 className="text-sm font-medium mb-3">发送测试邮件</h3>
          <div className="flex items-center gap-2">
            <input
              type="email"
              value={testEmail}
              onChange={(e) => setTestEmail(e.target.value)}
              placeholder="收件人邮箱"
              className="flex-1 px-3 py-2 rounded-lg border border-border bg-input text-sm"
            />
            <button
              onClick={handleTest}
              disabled={testing || !testEmail}
              className="flex items-center gap-2 px-4 py-2 bg-gray-700 text-white rounded-lg hover:bg-gray-800 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {testing ? <Loader2 size={18} className="animate-spin" /> : <Send size={18} />}
              {testing ? '发送中...' : '发送测试'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
