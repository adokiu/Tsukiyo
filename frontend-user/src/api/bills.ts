import apiClient from './client'

export interface Bill {
  id: number
  bill_no: string
  user_id: number
  username: string
  type: string
  status: string
  amount_cents: number
  balance_before: number
  balance_after: number
  payment_channel_id?: number
  payment_channel_name?: string
  transaction_no?: string
  description?: string
  expires_at?: string
  created_at: string
  updated_at: string
}

export interface WalletTransaction {
  id: number
  user_id: number
  amount_cents: number
  type: string
  bill_id?: number
  description?: string
  created_at: string
}

export interface InvoiceItemData {
  order_item_id: string
  product_name: string
  quantity: number
  unit_price_cents: number
  subtotal_cents: number
  vcpu: number
  memory_mb: number
  disk_mb: number
  data_disk_mb: number
  payment_period: string
  template_id: string
  image_key: string
  login_method: string
  instance_id?: string
  instance_name?: string
  internal_ipv4?: string
  internal_ipv6?: string
  network_down_mbps: number
  network_up_mbps: number
  monthly_traffic_gb: number
  ipv4_mode?: string
  ipv6_mode?: string
  expires_at?: string
}

export interface InvoiceData {
  site_name: string
  site_url: string
  contact_email: string
  username: string
  email: string
  order_id: string
  order_no: string
  status: string
  subtotal_cents: number
  discount_cents: number
  total_cents: number
  coupon_code?: string
  payment_method?: string
  created_at: string
  paid_at?: string
  bill_no?: string
  bill_status?: string
  payment_channel_name?: string
  items: InvoiceItemData[]
}

export interface ListResult<T> {
  data: T[]
  total: number
  page: number
  page_size: number
}

export const billsApi = {
  listBills: (page = 1, pageSize = 20) =>
    apiClient.get<ListResult<Bill>>('/user/bills', { params: { page, page_size: pageSize } }),
  getBill: (id: number) =>
    apiClient.get<Bill>(`/user/bills/${id}`),
  listTransactions: (page = 1, pageSize = 20) =>
    apiClient.get<ListResult<WalletTransaction>>('/user/wallet/transactions', { params: { page, page_size: pageSize } }),
  getInvoice: (orderId: string) =>
    apiClient.get<InvoiceData>(`/user/orders/${orderId}/invoice`),
}
