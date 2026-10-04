import apiClient from './client'

export interface PaymentChannel {
  id: number
  name: string
  type: string
  icon: string
  description: string
  fee_bearer: string
  fee_type: string
  fee_fixed_cents: number
  fee_percent: number
  min_amount: number
  max_amount: number
  settlement_currency: string
  status: string
}

export interface RechargeResult {
  bill_no: string
  pay_type: number
  pay_url: string
  status: string
}

export interface RechargeStatus {
  bill_no: string
  status: string
  amount_cents: number
}

export const paymentApi = {
  listChannels: () => apiClient.get<{ data: PaymentChannel[] }>('/payment/channels'),
  createRecharge: (channelId: number, amount: string) =>
    apiClient.post<RechargeResult>('/payment/recharge', { channel_id: channelId, amount }),
  getStatus: (billNo: string) => apiClient.get<RechargeStatus>(`/payment/recharge/${billNo}/status`),
}
