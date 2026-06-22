import { SlidersHorizontal } from 'lucide-react'
import { useTranslation } from 'react-i18next'

export default function SecurityACLPage() {
  const { t } = useTranslation()
  return (
    <div className="page-container">
      <div className="page-header">
        <div className="flex items-center gap-3">
          <SlidersHorizontal size={20} className="text-[#087ed1]" />
          <h1 className="page-title">{t('nav.aclRules')}</h1>
        </div>
      </div>
      <div className="page-card p-8 text-center text-tertiary">
        {t('common.noData')}
      </div>
    </div>
  )
}
