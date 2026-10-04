import { useTranslation } from 'react-i18next'
import { Construction } from 'lucide-react'
import { PageLayout } from '@/components/PageLayout/PageLayout'

interface Props {
  titleKey: string
}

export default function CommercePlaceholderPage({ titleKey }: Props) {
  const { t } = useTranslation()
  return (
    <PageLayout
      leftSlot={<span className="text-sm font-medium text-primary">{t(titleKey)}</span>}
    >
      <div className="page-transition__content flex flex-col items-center justify-center text-center gap-4" style={{ flex: 1 }}>
        <Construction size={48} className="text-tertiary" />
        <p className="text-tertiary text-sm">{t('common.comingSoon')}</p>
      </div>
    </PageLayout>
  )
}
