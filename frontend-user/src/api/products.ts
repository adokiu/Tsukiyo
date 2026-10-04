import apiClient from './client'

export interface ProductCategory {
  id: string
  parent_id: string | null
  name: string
  sort: number
  status: string
  description?: string
  children?: ProductCategory[]
}

export interface Product {
  id: string
  category_id: string | null
  name: string
  type: string
  status: string
  sort: number
  node_ids: string[]
  vcpu_min: number
  vcpu_max: number
  vcpu_custom_mode: string
  vcpu_tiers: number[]
  vcpu_unit_price_cents: number
  memory_min_mb: number
  memory_max_mb: number
  memory_custom_mode: string
  memory_tiers: number[]
  memory_unit_price_cents: number
  memory_unit: string
  disk_min_mb: number
  disk_max_mb: number
  disk_custom_mode: string
  disk_tiers: number[]
  disk_unit_price_cents: number
  disk_storage_pools: Record<string, string>
  data_disk_allow: boolean
  data_disk_min_mb: number
  data_disk_max_mb: number
  data_disk_custom_mode: string
  data_disk_tiers: number[]
  data_disk_unit_price_cents: number
  data_disk_storage_pools: Record<string, string>
  network_down_min_mbps: number
  network_down_max_mbps: number
  network_down_custom_mode: string
  network_down_tiers: number[]
  network_up_min_mbps: number
  network_up_max_mbps: number
  network_up_custom_mode: string
  network_up_tiers: number[]
  network_down_unit_price_cents: number
  network_up_unit_price_cents: number
  traffic_min_gb: number
  traffic_max_gb: number
  traffic_calc_mode: string
  traffic_custom_mode: string
  traffic_tiers: number[]
  traffic_unit_price_cents: number
  node_bridges: Record<string, string>
  ipv4_mode: string
  ipv4_eip_pools: Record<string, string[]>
  ipv4_eip_min: number
  ipv4_eip_max: number
  ipv4_eip_custom_mode: string
  ipv4_eip_tiers: number[]
  ipv4_eip_unit_price_cents: number
  ipv4_eip_allow_change: boolean
  ipv4_eip_change_price_cents: number
  nat_port_min: number
  nat_port_max: number
  nat_port_custom_mode: string
  nat_port_tiers: number[]
  nat_port_unit_price_cents: number
  ipv6_enabled: boolean
  ipv6_configs: IPv6ConfigItem[]
  ipv6_eip_pools: Record<string, string[]>
  stock: number
  base_price_cents: number
  min_payment_period: string
  trial_enabled: boolean
  trial_hours: number
  trial_price_cents: number
  quarterly_discount: number
  half_yearly_discount: number
  yearly_discount: number
  created_at: string
  updated_at: string
}

export interface IPv6ConfigItem {
  prefix_len: number
  min: number
  max: number
  custom_mode: string
  tiers: number[]
  unit_price_cents: number
}

export interface NodeImage {
  id: string
  fingerprint: string
  alias: string
  display_name: string
  type: string
  architecture: string
  size: number
  description: string
  image_source: string
  upload_date: string
  category_id?: string | null
  category_name?: string
  install_ssh: boolean
}

export interface Bridge {
  id: string
  node_id: string
  name: string
  bridge_name: string
  ipv4_enabled: boolean
  ipv4_cidr: string
  ipv4_gateway: string
  ipv6_enabled: boolean
  ipv6_cidr: string
  ipv6_gateway: string
  status: string
}

export interface OrderRequest {
  product_id: string
  name: string
  vcpu: number
  memory_mb: number
  disk_mb: number
  payment_period: string
  template_id: string
  image_key?: string
  node_id?: string
  bridge_id?: string
  login_method?: string
  ssh_password?: string
  ssh_public_key?: string
  data_disk_mb?: number
  network_down_mbps?: number
  network_up_mbps?: number
  nat_port_count?: number
  ipv4_eip_count?: number
  ipv6_items?: { prefix_len: number; count: number }[]
  trial?: boolean
}

export interface OrderResult {
  instance_id: string
  task_id: string
  bill_no: string
}

export const productsApi = {
  list: (categoryId?: string) =>
    apiClient.get<{ data: Product[] }>('/public/products', { params: categoryId ? { category_id: categoryId } : undefined }),
  get: (id: string) => apiClient.get<Product>(`/public/products/${id}`),
  categories: () => apiClient.get<{ data: ProductCategory[] }>('/public/product-categories'),
  images: (productId: string) => apiClient.get<{ data: NodeImage[]; node_id: string }>(`/public/products/${productId}/images`),
  bridges: (productId: string) => apiClient.get<{ data: Bridge[]; node_id: string }>(`/public/products/${productId}/bridges`),
  order: (productId: string, req: OrderRequest) => apiClient.post<OrderResult>(`/user/products/${productId}/order`, req),
}
