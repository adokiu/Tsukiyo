import apiClient from './client'

export interface CartItem {
  id: string
  user_id: number
  product_id: string
  product?: {
    id: string
    name: string
    type: string
    status: string
    base_price_cents: number
  }
  config: {
    name?: string
    vcpu?: number
    memory_mb?: number
    disk_mb?: number
    data_disk_mb?: number
    payment_period?: string
    template_id?: string
    image_key?: string
    node_id?: string
    login_method?: string
    trial?: boolean
  }
  unit_price_cents: number
  quantity: number
  created_at: string
  updated_at: string
}

export interface AddCartRequest {
  product_id: string
  config: Record<string, unknown>
  quantity?: number
}

export interface CouponValidateRequest {
  code: string
  amount_cents: number
}

export interface CouponValidateResult {
  valid: boolean
  discount_cents: number
  message?: string
  coupon_id?: string
  coupon_name?: string
  coupon_type?: string
  coupon_value?: number
}

export interface CheckoutItem {
  product_id: string
  config: Record<string, unknown>
  quantity: number
}

export interface CheckoutRequest {
  items: CheckoutItem[]
  coupon_code?: string
}

export interface CheckoutResult {
  order_id: string
  order_no: string
  subtotal_cents: number
  discount_cents: number
  total_cents: number
}

export interface PayOrderRequest {
  order_id: string
  method: 'balance' | 'channel'
  channel_id?: number
}

export interface PayOrderResult {
  order_id: string
  order_no: string
  status: string
  pay_url?: string
}

export interface Order {
  id: string
  order_no: string
  user_id: number
  subtotal_cents: number
  discount_cents: number
  total_cents: number
  coupon_code?: string
  status: string
  payment_method?: string
  description?: string
  expires_at?: string
  created_at: string
  items?: OrderItem[]
}

export interface OrderItem {
  id: string
  order_id: string
  product_id: string
  product?: { name: string; type: string }
  config: Record<string, unknown>
  unit_price_cents: number
  quantity: number
  subtotal_cents: number
  instance_id?: string
}

export const cartApi = {
  list: () => apiClient.get<{ data: CartItem[] }>('/user/cart'),
  add: (req: AddCartRequest) => apiClient.post<CartItem>('/user/cart', req),
  update: (id: string, req: { quantity?: number; config?: Record<string, unknown> }) =>
    apiClient.put(`/user/cart/${id}`, req),
  remove: (id: string) => apiClient.delete(`/user/cart/${id}`),
  clear: () => apiClient.delete('/user/cart'),
}

export const couponApi = {
  validate: (req: CouponValidateRequest) => apiClient.post<CouponValidateResult>('/user/coupon/validate', req),
}

export const orderApi = {
  checkout: (req: CheckoutRequest) => apiClient.post<CheckoutResult>('/user/checkout', req),
  pay: (req: PayOrderRequest) => apiClient.post<PayOrderResult>(`/user/orders/${req.order_id}/pay`, req),
  get: (id: string) => apiClient.get<Order>(`/user/orders/${id}`),
  list: () => apiClient.get<{ data: Order[] }>('/user/orders'),
}
