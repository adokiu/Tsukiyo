import { AlertTriangle } from 'lucide-react'
import { generateRandomPassword } from '@/utils/format'

export interface CategoryItem {
  id: string
  name: string
  image_type: string
  sort_order: number
}

export interface ReinstallImageItem {
  id: string
  fingerprint: string
  alias: string
  type: string
  size: number
  architecture: string
  created_at: string
  category_id?: string
  category_name?: string
  install_ssh: boolean
  display_name?: string
}

interface ReinstallTabProps {
  templateId: string
  categories: CategoryItem[]
  images: ReinstallImageItem[]
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
  isBanned: boolean
  isExpired: boolean
  isBusy: boolean
}

export function ReinstallTab({
  templateId, categories, images, category, setCategory, image, setImage,
  loginMode, setLoginMode, password, setPassword, sshKey, setSSHKey,
  formatDisks, setFormatDisks, onConfirm, isBanned, isExpired, isBusy,
}: ReinstallTabProps) {
  return (
    <div className="space-y-4">
      {/* 警告 */}
      <div className="bg-red-50 border border-red-200 rounded-xl p-5">
        <div className="flex items-start gap-3">
          <AlertTriangle size={20} className="text-red-500 flex-shrink-0 mt-0.5" />
          <div>
            <h4 className="text-sm font-semibold text-red-700">危险操作</h4>
            <p className="text-sm text-red-600 mt-1">
              重装系统将删除容器内所有数据，此操作不可撤销！请提前备份重要数据。
            </p>
          </div>
        </div>
      </div>

      {/* 重装配置 */}
      <div className="bg-surface rounded-xl border border-surface p-5 space-y-4">
        {/* 选择镜像 - 两级选择 */}
        <div>
          <h4 className="text-sm font-semibold text-primary mb-2">选择目标系统</h4>
          <p className="text-xs text-tertiary mb-3">当前系统: <span className="font-mono">{templateId || '-'}</span></p>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs text-tertiary mb-1">分类</label>
              <select
                value={category}
                onChange={(e) => { setCategory(e.target.value); setImage('') }}
                className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm"
              >
                <option value="">全部分类</option>
                {categories.map(cat => (
                  <option key={cat.id} value={cat.id}>{cat.name}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-xs text-tertiary mb-1">镜像版本</label>
              <select
                value={image}
                onChange={(e) => setImage(e.target.value)}
                className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm"
              >
                <option value="">选择镜像...</option>
                {images
                  .filter(img => !category || img.category_id === category)
                  .map(img => (
                    <option key={img.id} value={img.alias}>
                      {img.display_name || img.alias} ({img.architecture})
                    </option>
                  ))
                }
              </select>
            </div>
          </div>
          {images.length === 0 && (
            <p className="text-xs text-muted mt-2">该节点暂无已安装的镜像</p>
          )}
        </div>

        {/* SSH 凭据 */}
        <div className="pt-4 border-t border-surface-light">
          <h4 className="text-sm font-semibold text-primary mb-3">SSH 登录凭据</h4>
          <div className="flex gap-2 mb-3">
            {[
              { key: 'auto', label: '自动生成' },
              { key: 'password', label: '随机密码' },
              { key: 'sshkey', label: '自定义 Key' },
            ].map(mode => (
              <button
                key={mode.key}
                onClick={() => {
                  setLoginMode(mode.key as any)
                  if (mode.key === 'password') {
                    setPassword(generateRandomPassword())
                  }
                }}
                className={`px-4 py-2 text-sm font-semibold rounded-full border transition-all ${
                  loginMode === mode.key
                    ? 'border-blue-600 bg-blue-600 text-white'
                    : 'border-surface text-tertiary hover:text-secondary hover:border-blue-300'
                }`}
              >
                {mode.label}
              </button>
            ))}
          </div>

          {loginMode === 'auto' && (
            <p className="text-xs text-tertiary">系统将自动生成随机密码，重装完成后可在实例详情页查看。</p>
          )}

          {loginMode === 'password' && (
            <div className="flex gap-2">
              <input
                type="text"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="密码"
                className="flex-1 px-3 py-2 border border-surface-strong rounded-lg text-sm font-mono"
              />
              <button
                onClick={() => setPassword(generateRandomPassword())}
                className="px-3 py-2 text-xs bg-surface-secondary hover:bg-surface-hover rounded-lg"
              >
                重新生成
              </button>
            </div>
          )}

          {loginMode === 'sshkey' && (
            <textarea
              value={sshKey}
              onChange={(e) => setSSHKey(e.target.value)}
              placeholder="粘贴 SSH 公钥 (ssh-ed25519 AAAA... 或 ssh-rsa AAAA...)"
              rows={4}
              className="w-full px-3 py-2 border border-surface-strong rounded-lg text-sm font-mono"
            />
          )}
        </div>

        {/* 格式化数据盘 */}
        <div className="pt-4 border-t border-surface-light">
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              checked={formatDisks}
              onChange={(e) => setFormatDisks(e.target.checked)}
              className="w-4 h-4 rounded"
            />
            <span className="text-sm text-secondary">同时格式化所有数据盘</span>
          </label>
        </div>

        {/* 确认按钮 */}
        <div className="pt-4 border-t border-surface-light">
          <button
            onClick={onConfirm}
            disabled={!image || isBanned || isExpired || isBusy}
            className="px-6 py-2.5 rounded-lg bg-red-600 text-white hover:bg-red-700 font-semibold text-sm disabled:opacity-50 disabled:cursor-not-allowed"
          >
            确认重装
          </button>
        </div>
      </div>
    </div>
  )
}
