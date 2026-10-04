import apiClient from './client'

export interface InstanceMetrics {
  cpu_usage: number
  memory_used: number
  memory_total: number
  disk_used: number
  disk_total: number
  net_in_bps: number
  net_out_bps: number
  net_in_total: number
  net_out_total: number
}

export interface InstanceCounts {
  all: number
  running: number
  stopped: number
  creating: number
  error: number
}

export interface InstanceListResponse {
  data: InstanceItem[]
  total: number
  page: number
  per_page: number
  counts: InstanceCounts
}

export interface InstanceItem {
  id: string
  name: string
  user_id: number
  node_id: string
  type: string
  status: string
  vcpu: number
  memory_mb: number
  disk_mb: number
  incus_name: string
  ipv4_addr: string
  ipv6_addr: string
  traffic_used_gb: number
  monthly_traffic_gb: number
  created_at: string
  expires_at: string | null
  metrics?: InstanceMetrics | null
}

export interface BatchMetricsItem {
  instance_id: string
  cpu_usage: number
  memory_used: number
  memory_total: number
  disk_used: number
  disk_total: number
  net_in_bps: number
  net_out_bps: number
  net_in_total: number
  net_out_total: number
}

export interface DataDisk {
  id: string
  name: string
  size_mb: number
  mount_point: string
  storage_pool?: string
  status?: string
  updated_at?: string
}

export interface PortMapping {
  id: string
  host_port: number
  container_port: number
  protocol: string
  description?: string
}

export interface Snapshot {
  id: string
  name: string
  created_at: string
  size?: number
}

export interface InstanceDetail {
  id: string
  name: string
  type: string
  status: string
  node_id: string
  node_name?: string
  user_id: number
  incus_name: string
  template_id: string
  vcpu: number
  memory_mb: number
  swap_mb: number
  disk_mb: number
  storage_pool: string
  internal_ipv4?: string
  internal_ipv6?: string
  login_method: string
  ssh_port?: number
  ssh_password?: string
  ssh_public_key?: string
  network_down?: number
  network_up?: number
  io_read_iops?: number
  io_write_iops?: number
  monthly_traffic?: number
  traffic_used_gb?: number
  traffic_mode: string
  over_limit_action?: string
  throttle_mbps?: number
  is_over_limit?: boolean
  snapshot_limit: number
  port_mapping_limit?: number
  bridge_id?: string
  bridge_name?: string
  bridge_iface?: string
  bridge_cidr?: string
  bridge_gateway?: string
  ipv4_eip?: string
  ipv4_eip_alias?: string
  ipv6_eip?: string
  ipv6_eip_alias?: string
  has_eip?: boolean
  data_disks?: DataDisk[]
  port_mappings?: PortMapping[]
  expires_at?: string
  created_at: string
  ipv4_mode?: string
  ipv6_mode?: string
}

export interface InstanceMetricsData {
  cpu_usage?: number
  memory_usage?: number
  memory_total?: number
  memory_used?: number
  disk_used?: number
  disk_total?: number
  disk_read_bps?: number
  disk_write_bps?: number
  disk_read_iops?: number
  disk_write_iops?: number
  network_rx?: number
  network_tx?: number
  traffic_used_gb?: number
  monthly_traffic?: number
}

export interface MetricPoint {
  timestamp: string
  cpu: number
  cpu_max?: number
  cpu_min?: number
  mem_used: number
  mem_used_max?: number
  mem_used_min?: number
  mem_total: number
  disk_used: number
  disk_used_max?: number
  disk_used_min?: number
  disk_total: number
  disk_read_bps: number
  disk_read_max?: number
  disk_write_bps: number
  disk_write_max?: number
  disk_read_iops?: number
  disk_write_iops?: number
  net_in: number
  net_in_max?: number
  net_in_min?: number
  net_out: number
  net_out_max?: number
  net_out_min?: number
}

export interface InstalledImage {
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

export interface Category {
  id: string
  name: string
  image_type: string
  sort_order: number
}

export interface DashboardData {
  total_instances: number
  running_instances: number
  stopped_instances: number
  recent_instances: InstanceItem[]
}

export const instancesApi = {
  list: (params?: { page?: number; per_page?: number; search?: string; type?: string; filter_status?: string }) =>
    apiClient.get<InstanceListResponse>('/user/instances', { params }),
  batchMetrics: () => apiClient.get<{ items: BatchMetricsItem[] }>('/user/instances/metrics'),
  get: (id: string) => apiClient.get<InstanceDetail>(`/user/instances/${id}`),
  start: (id: string) => apiClient.post(`/user/instances/${id}/start`),
  stop: (id: string) => apiClient.post(`/user/instances/${id}/stop`),
  restart: (id: string) => apiClient.post(`/user/instances/${id}/restart`),
  delete: (id: string) => apiClient.delete(`/user/instances/${id}`),
  console: (id: string, type?: string) => apiClient.get(`/user/instances/${id}/console`, { params: type ? { type } : undefined }),
  metrics: (id: string) => apiClient.get<InstanceMetricsData>(`/user/instances/${id}/metrics`),
  metricsHistory: (id: string, period: string) => apiClient.get<{ data: MetricPoint[] }>(`/user/instances/${id}/metrics/history`, { params: { period } }),
  resetPassword: (id: string, data: { password?: string }) => apiClient.post(`/user/instances/${id}/reset-password`, data),
  reinstall: (id: string, data: Record<string, any>) => apiClient.post(`/user/instances/${id}/reinstall`, data),
  snapshots: (id: string) => apiClient.get<{ data: Snapshot[] }>(`/user/instances/${id}/snapshots`),
  createSnapshot: (id: string, data: { name: string }) => apiClient.post(`/user/instances/${id}/snapshots`, data),
  restoreSnapshot: (id: string, name: string) => apiClient.post(`/user/instances/${id}/snapshots/${name}/restore`),
  deleteSnapshot: (id: string, name: string) => apiClient.delete(`/user/instances/${id}/snapshots/${name}`),
  dashboard: () => apiClient.get<DashboardData>('/user/dashboard'),
}
