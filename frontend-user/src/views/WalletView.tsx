import { useState, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toast'
import { paymentApi, type PaymentChannel } from '@/api/payment'
import { Loader2, Wallet, ArrowUpCircle, CheckCircle2, XCircle } from 'lucide-react'

export function WalletView() {
  const { t } = useTranslation()
  const toast = useToastStore()
  const { user, setUser } = useAuthStore()
  const [channels, setChannels] = useState<PaymentChannel[]>([])
  const [selectedChannel, setSelectedChannel] = useState<PaymentChannel | null>(null)
  const [amount, setAmount] = useState('')
  const [loading, setLoading] = useState(false)
  const [channelsLoading, setChannelsLoading] = useState(true)
  const [payUrl, setPayUrl] = useState<string | null>(null)
  const [billNo, setBillNo] = useState<string | null>(null)
  const [payStatus, setPayStatus] = useState<'idle' | 'pending' | 'paid' | 'failed'>('idle')
  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null)

  useEffect(() => {
    fetchChannels()
    return () => stopPolling()
  }, [])

  const fetchChannels = async () => {
    setChannelsLoading(true)
    try {
      const res = await paymentApi.listChannels()
      setChannels(res.data.data || [])
      if (res.data.data && res.data.data.length > 0) {
        setSelectedChannel(res.data.data[0])
      }
    } catch {
      toast.error(t('common.error'))
    } finally {
      setChannelsLoading(false)
    }
  }

  const stopPolling = () => {
    if (pollingRef.current) {
      clearInterval(pollingRef.current)
      pollingRef.current = null
    }
  }

  const startPolling = (no: string) => {
    stopPolling()
    pollingRef.current = setInterval(async () => {
      try {
        const res = await paymentApi.getStatus(no)
        const status = res.data.status
        if (status === 'paid') {
          setPayStatus('paid')
          toast.success(t('wallet.rechargeSuccess'))
          stopPolling()
          refreshUser()
        } else if (status === 'failed' || status === 'cancelled') {
          setPayStatus('failed')
          toast.error(t('wallet.rechargeFailed'))
          stopPolling()
        }
      } catch {
        // 静默失败
      }
    }, 3000)
  }

  const refreshUser = async () => {
    try {
      const { authApi } = await import('@/api/auth')
      const res = await authApi.me()
      if (res.data) {
        setUser(res.data)
      }
    } catch {
      // 静默失败
    }
  }

  const formatBalance = (cents: number) => {
    return (cents / 100).toFixed(2)
  }

  const formatFee = (channel: PaymentChannel) => {
    if (channel.fee_type === 'fixed') {
      return `${(channel.fee_fixed_cents / 100).toFixed(2)} ${channel.fee_bearer === 'user' ? t('wallet.feeBearerUser') : t('wallet.feeBearerMerchant')}`
    } else if (channel.fee_type === 'percent') {
      return `${channel.fee_percent}% ${channel.fee_bearer === 'user' ? t('wallet.feeBearerUser') : t('wallet.feeBearerMerchant')}`
    } else if (channel.fee_type === 'fixed_plus_percent') {
      return `${(channel.fee_fixed_cents / 100).toFixed(2)} + ${channel.fee_percent}% ${channel.fee_bearer === 'user' ? t('wallet.feeBearerUser') : t('wallet.feeBearerMerchant')}`
    }
    return '-'
  }

  const handleRecharge = async () => {
    if (!selectedChannel) {
      toast.error(t('wallet.selectChannel'))
      return
    }
    const value = parseFloat(amount)
    if (!value || value <= 0) {
      toast.error(t('wallet.invalidAmount'))
      return
    }
    const minAmount = selectedChannel.min_amount / 100
    if (selectedChannel.min_amount > 0 && value < minAmount) {
      toast.error(t('wallet.minAmountError', { min: minAmount.toFixed(2) }))
      return
    }
    const maxAmount = selectedChannel.max_amount / 100
    if (selectedChannel.max_amount > 0 && value > maxAmount) {
      toast.error(t('wallet.maxAmountError', { max: maxAmount.toFixed(2) }))
      return
    }

    setLoading(true)
    setPayStatus('idle')
    try {
      const res = await paymentApi.createRecharge(selectedChannel.id, amount)
      setBillNo(res.data.bill_no)
      setPayUrl(res.data.pay_url)
      setPayStatus('pending')

      if (res.data.pay_type === 1 && res.data.pay_url) {
        window.open(res.data.pay_url, '_blank')
      }

      startPolling(res.data.bill_no)
    } catch (err: any) {
      toast.error(err.response?.data?.error || t('wallet.rechargeFailed'))
    } finally {
      setLoading(false)
    }
  }

  const resetRecharge = () => {
    setPayUrl(null)
    setBillNo(null)
    setPayStatus('idle')
    setAmount('')
    stopPolling()
  }

  return (
    <main className="flex-1 overflow-y-auto overflow-x-hidden p-4 sm:p-6 md:p-8">
      <div className="mx-auto max-w-4xl w-full space-y-6" style={{ animation: 'page-enter 0.25s ease' }}>

        <div className="page-header">
          <h1 className="page-title">{t('wallet.title')}</h1>
        </div>

        {/* 余额卡片 */}
        <div className="card p-6">
          <div className="flex items-center gap-4">
            <div className="w-12 h-12 rounded-xl bg-surface-secondary flex items-center justify-center">
              <Wallet size={24} className="text-tertiary" />
            </div>
            <div>
              <p className="text-sm text-tertiary">{t('wallet.currentBalance')}</p>
              <p className="text-3xl font-semibold text-primary font-number">
                ¥{formatBalance(user?.balance_cents || 0)}
              </p>
            </div>
          </div>
        </div>

        {/* 充值区域 */}
        {payStatus === 'pending' || payStatus === 'paid' || payStatus === 'failed' ? (
          <div className="card p-6 space-y-4">
            <div className="flex flex-col items-center py-8 space-y-3">
              {payStatus === 'pending' && (
                <>
                  <Loader2 size={48} className="animate-spin text-primary" />
                  <p className="text-lg font-medium text-primary">{t('wallet.waitingPayment')}</p>
                  <p className="text-sm text-tertiary">{t('wallet.waitingPaymentDesc')}</p>
                  {payUrl && (
                    <a href={payUrl} target="_blank" rel="noopener noreferrer"
                       className="btn btn--primary btn--md mt-2">
                      {t('wallet.openPayPage')}
                    </a>
                  )}
                </>
              )}
              {payStatus === 'paid' && (
                <>
                  <CheckCircle2 size={48} className="text-green-500" />
                  <p className="text-lg font-medium text-primary">{t('wallet.rechargeSuccess')}</p>
                </>
              )}
              {payStatus === 'failed' && (
                <>
                  <XCircle size={48} className="text-red-500" />
                  <p className="text-lg font-medium text-primary">{t('wallet.rechargeFailed')}</p>
                </>
              )}
              <button onClick={resetRecharge} className="btn btn--ghost btn--md mt-4">
                {t('wallet.rechargeAgain')}
              </button>
            </div>
          </div>
        ) : (
          <div className="card p-6 space-y-5">
            <h2 className="text-lg font-semibold text-primary">{t('wallet.recharge')}</h2>

            {/* 选择支付渠道 */}
            <div>
              <label className="apple-label mb-3 block">{t('wallet.selectChannel')}</label>
              {channelsLoading ? (
                <div className="flex items-center gap-2 text-tertiary">
                  <Loader2 size={16} className="animate-spin" />
                  <span className="text-sm">{t('common.loading')}</span>
                </div>
              ) : channels.length === 0 ? (
                <p className="text-sm text-tertiary">{t('wallet.noChannels')}</p>
              ) : (
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  {channels.map((ch) => (
                    <button
                      key={ch.id}
                      onClick={() => setSelectedChannel(ch)}
                      className={`p-4 rounded-xl border text-left transition-all ${
                        selectedChannel?.id === ch.id
                          ? 'border-primary bg-surface-secondary'
                          : 'border-border hover:border-primary/50'
                      }`}
                    >
                      <div className="flex items-center gap-3">
                        {ch.icon && <img src={ch.icon} alt="" className="w-8 h-8 rounded" />}
                        <div>
                          <p className="font-medium text-primary text-sm">{ch.name}</p>
                          {ch.description && <p className="text-xs text-tertiary mt-0.5">{ch.description}</p>}
                        </div>
                      </div>
                      <p className="text-xs text-tertiary mt-2">{formatFee(ch)}</p>
                    </button>
                  ))}
                </div>
              )}
            </div>

            {/* 金额输入 */}
            <div>
              <label className="apple-label mb-2 block">{t('wallet.amount')}</label>
              <div className="relative">
                <span className="absolute left-3 top-1/2 -translate-y-1/2 text-lg text-tertiary">¥</span>
                <input
                  type="number"
                  step="0.01"
                  min="0"
                  value={amount}
                  onChange={(e) => setAmount(e.target.value)}
                  className="apple-input w-full pl-8 text-lg font-number"
                  placeholder="0.00"
                />
              </div>
              {selectedChannel && (
                <p className="text-xs text-tertiary mt-2">
                  {selectedChannel.min_amount > 0 && t('wallet.minAmount', { min: (selectedChannel.min_amount / 100).toFixed(2) })}
                  {selectedChannel.min_amount > 0 && selectedChannel.max_amount > 0 && ' / '}
                  {selectedChannel.max_amount > 0 && t('wallet.maxAmount', { max: (selectedChannel.max_amount / 100).toFixed(2) })}
                </p>
              )}
            </div>

            {/* 快捷金额 */}
            <div className="flex flex-wrap gap-2">
              {[10, 50, 100, 500].map((v) => (
                <button
                  key={v}
                  onClick={() => setAmount(String(v))}
                  className="px-4 py-2 rounded-lg border border-border text-sm text-secondary hover:border-primary/50 transition-all"
                >
                  ¥{v}
                </button>
              ))}
            </div>

            {/* 提交按钮 */}
            <button
              onClick={handleRecharge}
              disabled={loading || !selectedChannel || !amount}
              className="btn btn--primary btn--lg w-full disabled:opacity-50"
            >
              {loading ? <Loader2 size={18} className="btn__spinner" /> : <ArrowUpCircle size={18} />}
              <span>{t('wallet.confirmRecharge')}</span>
            </button>
          </div>
        )}
      </div>
    </main>
  )
}
