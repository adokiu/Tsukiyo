import { useQuery } from '@tanstack/react-query'
import { instancesApi } from '@/api/instances'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth'
import { Server, Plus } from 'lucide-react'

export function HomeView() {
  const { t } = useTranslation()
  const { user } = useAuthStore()
  const { data } = useQuery({
    queryKey: ['dashboard'],
    queryFn: () => instancesApi.dashboard(),
  })

  const hour = new Date().getHours()
  const greeting = hour < 6 ? '夜深了' : hour < 12 ? '早上好' : hour < 18 ? '稍微休息一下眼睛吧' : '晚上好'

  return (
    <main className="flex-1 overflow-y-auto overflow-x-hidden p-4 sm:p-6 md:p-8">
      <div className="mx-auto max-w-6xl w-full">
        <div className="space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-700">

          <div className="flex flex-col md:flex-row md:items-end justify-between mb-8 gap-4">
            <div>
              <h2 className="text-3xl font-medium mb-1 tracking-tight">{greeting}，<span className="font-semibold">{user?.username || 'User'}</span></h2>
              <p className="text-muted-foreground">{t('dashboard.welcomeSubtitle2')}</p>
            </div>
          </div>

          <div className="bg-card border border-border rounded-xl p-6 shadow-sm flex flex-col md:flex-row gap-6 items-center justify-between">
            <div>
              <h3 className="text-lg font-medium text-foreground tracking-tight">{t('dashboard.computeResource')}</h3>
              <p className="text-sm text-muted-foreground mt-1">{t('dashboard.computeResourceDesc')}</p>
            </div>
            <div className="flex flex-wrap gap-3 shrink-0">
              <Link to="/instances" className="bg-secondary text-secondary-foreground hover:bg-secondary/80 px-4 py-2 rounded-md text-sm font-medium transition-colors border border-border">
                {t('dashboard.manageInstances')}
              </Link>
              <Link to="/instances" className="bg-primary text-primary-foreground hover:bg-primary/90 px-4 py-2 rounded-md text-sm font-medium transition-colors shadow-sm">
                {t('dashboard.createInstance')}
              </Link>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="bg-card border border-border rounded-xl p-6 shadow-sm overflow-hidden flex flex-col justify-between">
              <h3 className="text-sm font-medium text-muted-foreground mb-4 uppercase tracking-wider">{t('dashboard.currentBalance')}</h3>
              <div className="flex items-baseline gap-2">
                <span className="text-4xl font-semibold tracking-tight font-number">¥{((user?.balance_cents || 0) / 100).toFixed(2)}</span>
                <span className="text-sm text-emerald-500 font-medium">CNY</span>
              </div>
            </div>
            <div className="bg-card border border-border rounded-xl p-6 shadow-sm overflow-hidden flex flex-col justify-between">
              <h3 className="text-sm font-medium text-muted-foreground mb-4 uppercase tracking-wider">{t('dashboard.monthlySpent')}</h3>
              <div className="flex items-baseline gap-2">
                <span className="text-4xl font-semibold tracking-tight font-number">¥0.00</span>
                <span className="text-sm text-muted-foreground font-medium">CNY</span>
              </div>
            </div>
          </div>

          <div className="mt-8">
            <h3 className="text-xl font-medium mb-6 flex items-center gap-3">
              <div className="w-1.5 h-6 bg-primary rounded-full" />
              {t('dashboard.myInstances')}
            </h3>
            {data?.data.recent_instances && data.data.recent_instances.length > 0 ? (
              <div className="grid grid-cols-1 gap-4">
                {data.data.recent_instances.map((inst) => (
                  <Link
                    key={inst.id}
                    to={`/instances/${inst.id}`}
                    className="bg-card border border-border rounded-xl p-5 shadow-sm flex items-center justify-between hover:shadow-md transition-shadow"
                  >
                    <div className="flex items-center gap-3">
                      <div className="w-10 h-10 rounded-lg bg-secondary flex items-center justify-center">
                        <Server className="w-5 h-5 text-muted-foreground" />
                      </div>
                      <div>
                        <p className="font-medium text-foreground">{inst.name}</p>
                        <p className="text-sm text-muted-foreground">{inst.type} - {inst.node_name}</p>
                      </div>
                    </div>
                    <span className={
                      inst.status === 'running' ? 'text-sm font-medium text-emerald-500' :
                      inst.status === 'stopped' ? 'text-sm font-medium text-muted-foreground' :
                      'text-sm font-medium text-muted-foreground'
                    }>
                      {t(`common.${inst.status}`, inst.status)}
                    </span>
                  </Link>
                ))}
              </div>
            ) : (
              <div className="grid grid-cols-1 gap-4">
                <div className="p-8 text-center border border-dashed border-border rounded-xl bg-card/50">
                  <p className="text-muted-foreground">{t('dashboard.noInstances')}</p>
                  <Link to="/instances" className="inline-flex items-center gap-1.5 mt-4 text-primary hover:text-primary/80 text-sm font-medium transition-colors">
                    <Plus className="w-4 h-4" />
                    {t('dashboard.createFirst')}
                  </Link>
                </div>
              </div>
            )}
          </div>

          <div className="mt-8">
            <h3 className="text-xl font-medium mb-6 flex items-center gap-3">
              <div className="w-1.5 h-6 bg-primary rounded-full" />
              {t('dashboard.announcements')}
            </h3>
            <div className="grid grid-cols-1 gap-4">
              <div className="p-8 text-center border border-dashed border-border rounded-xl bg-card/50">
                <p className="text-muted-foreground">{t('dashboard.noAnnouncements')}</p>
              </div>
            </div>
          </div>

        </div>
      </div>
    </main>
  )
}
