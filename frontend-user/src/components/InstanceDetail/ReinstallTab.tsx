import { Category, InstalledImage } from '@/api/instances'
import { generateRandomPassword } from '@/utils/format'

interface ReinstallTabProps {
  templateId: string
  categories: Category[]
  images: InstalledImage[]
  category: string
  setCategory: (v: string) => void
  image: string
  setImage: (v: string) => void
  loginMode: 'auto' | 'password' | 'sshkey'
  setLoginMode: (v: 'auto' | 'password' | 'sshkey') => void
  password: string
  setPassword: (v: string) => void
  sshKey: string
  setSSHKey: (v: string) => void
  formatDisks: boolean
  setFormatDisks: (v: boolean) => void
  onConfirm: () => void
  isBusy: boolean
}

export function ReinstallTab({
  templateId, categories, images, category, setCategory,
  image, setImage, loginMode, setLoginMode,
  password, setPassword, sshKey, setSSHKey,
  formatDisks, setFormatDisks, onConfirm, isBusy,
}: ReinstallTabProps) {
  const filteredImages = category ? images.filter(img => img.category_id === category) : images

  const selectLoginMode = (mode: 'auto' | 'password' | 'sshkey') => {
    setLoginMode(mode)
    if (mode === 'password' && !password.trim()) {
      setPassword(generateRandomPassword())
    }
  }

  return (
    <div className="space-y-4">
      <div className="card p-5">
        <h3 className="text-sm font-semibold mb-4">重装系统</h3>

        <div className="space-y-4">
          <div>
            <label className="text-sm text-muted-foreground mb-1 block">镜像分类</label>
            <select
              value={category}
              onChange={(e) => {
                setCategory(e.target.value)
                setImage('')
              }}
              className="w-full px-3 py-2 border border-gray-200 rounded-lg text-sm"
            >
              <option value="">全部分类</option>
              {categories.map(cat => (
                <option key={cat.id} value={cat.id}>{cat.name}</option>
              ))}
            </select>
          </div>

          <div>
            <label className="text-sm text-muted-foreground mb-1 block">选择镜像</label>
            <select
              value={image}
              onChange={(e) => setImage(e.target.value)}
              className="w-full px-3 py-2 border border-gray-200 rounded-lg text-sm"
            >
              <option value="">请选择镜像</option>
              {filteredImages.map(img => (
                <option key={img.id} value={img.id}>{img.display_name || img.alias}</option>
              ))}
            </select>
            <p className="text-xs text-muted-foreground mt-1">
              当前系统: <span className="font-mono">{templateId || '-'}</span>
            </p>
          </div>

          <div className="pt-2 border-t border-gray-100">
            <label className="text-sm font-semibold text-foreground mb-2 block">密码设置</label>
            <div className="flex flex-wrap gap-2 mb-3">
              {[
                { key: 'password' as const, label: '自定义密码' },
                { key: 'auto' as const, label: '自动生成' },
                { key: 'sshkey' as const, label: 'SSH 密钥' },
              ].map(mode => (
                <button
                  key={mode.key}
                  type="button"
                  onClick={() => selectLoginMode(mode.key)}
                  className={`px-3 py-1.5 text-sm rounded-lg border transition-colors ${
                    loginMode === mode.key
                      ? 'border-gray-900 bg-gray-900 text-white'
                      : 'border-gray-200 bg-white text-gray-700 hover:border-gray-400'
                  }`}
                >
                  {mode.label}
                </button>
              ))}
            </div>

            {loginMode === 'auto' && (
              <p className="text-xs text-muted-foreground">
                系统将自动生成随机密码，重装完成后可在实例详情中查看。
              </p>
            )}

            {loginMode === 'password' && (
              <div className="flex gap-2">
                <input
                  type="text"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="输入 SSH 登录密码"
                  className="flex-1 px-3 py-2 border border-gray-200 rounded-lg text-sm font-mono"
                />
                <button
                  type="button"
                  onClick={() => setPassword(generateRandomPassword())}
                  className="px-3 py-2 text-xs border border-gray-200 rounded-lg hover:bg-gray-50 whitespace-nowrap"
                >
                  随机生成
                </button>
              </div>
            )}

            {loginMode === 'sshkey' && (
              <textarea
                value={sshKey}
                onChange={(e) => setSSHKey(e.target.value)}
                placeholder="粘贴 SSH 公钥"
                rows={3}
                className="w-full px-3 py-2 border border-gray-200 rounded-lg text-sm font-mono"
              />
            )}
          </div>

          <div className="pt-2 border-t border-gray-100">
            <label className="flex items-center gap-2 text-sm text-foreground cursor-pointer">
              <input
                type="checkbox"
                checked={formatDisks}
                onChange={(e) => setFormatDisks(e.target.checked)}
              />
              格式化所有数据盘
            </label>
          </div>

          <div className="flex justify-end pt-2">
            <button
              type="button"
              onClick={onConfirm}
              disabled={!image || isBusy}
              className="btn btn--danger btn--md disabled:opacity-50"
            >
              确认重装
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
