import { Globe } from 'lucide-react'
import { useTranslation } from 'react-i18next'

export default function SecurityURLFilterPage() {
  const { t } = useTranslation()
  return (
    <div className="page-container">
      <div className="page-header">
        <div className="flex items-center gap-3">
          <Globe size={20} className="text-[#087ed1]" />
          <h1 className="page-title">{t('nav.urlFilter')}</h1>
        </div>
      </div>
      <div className="page-card p-8 text-center text-tertiary">
        {t('common.noData')}
      </div>
    </div>
  )
}
