import { useTranslation } from 'react-i18next'
import { Construction } from 'lucide-react'

interface Props {
  titleKey: string
}

export default function SecurityPlaceholderPage({ titleKey }: Props) {
  const { t } = useTranslation()
  return (
    <div className="page-container">
      <div className="page-header">
        <h1 className="page-title">{t(titleKey)}</h1>
      </div>
      <div className="page-card p-12 flex flex-col items-center justify-center text-center gap-4">
        <Construction size={48} className="text-[#8597ab]" />
        <p className="text-[#8597ab] text-sm">{t('common.noData')}</p>
        <p className="text-xs text-[#8597ab]">功能开发中</p>
      </div>
    </div>
  )
}
