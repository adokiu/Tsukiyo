export interface ApiResponse<T> {
  data?: T
  error?: string
  message?: string
}

export interface User {
  id: string
  username: string
  email: string
  status: string
  groups: UserGroup[]
  created_at: string
  updated_at: string
}

export interface UserGroup {
  id: string
  name: string
  description: string
}

export interface Node {
  id: string
  name: string
  hostname: string
  ip_address: string
  status: string
  total_cpu: number
  used_cpu: number
  total_memory: number
  used_memory: number
  total_disk: number
  used_disk: number
  instance_count: number
  running_count: number
  region: string
  last_heartbeat: string
}

export interface Instance {
  id: string
  name: string
  incus_name: string
  type: string
  status: string
  user_id: string
  node_id: string
  node_name?: string
  template_id: string
  template_name?: string
  vcpu: number
  memory_mb: number
  disk_gb: number
  storage_pool: string
  public_ipv4: string
  public_ipv6: string
  network_down_mbps: number
  network_up_mbps: number
  io_read_mbps: number
  io_write_mbps: number
  monthly_traffic_gb: number
  current_traffic_gb: number
  traffic_mode: string
  snapshot_limit: number
  ssh_password: string
  ssh_public_key: string
  expires_at: string
  created_at: string
  data_disks?: DataDisk[]
  nat_configs?: NATConfig[]
}

export interface DataDisk {
  id: string
  name: string
  size_gb: number
  storage_pool: string
  mount_point: string
}

export interface NATConfig {
  id: string
  internal_ip: string
  external_ip: string
  internal_port: number
  external_port: number
  protocol: string
}

export interface ImageTemplate {
  id: string
  name: string
  type: string
  distro: string
  version: string
  architecture: string
  size_mb: number
  enabled: boolean
  node_ids: string[]
  created_at: string
}

export interface IPPool {
  id: string
  node_id: string
  ip_address: string
  gateway: string
  netmask: string
  type: string
  status: string
}

export interface IPv6Prefix {
  id: string
  node_id: string
  prefix: string
  gateway: string
  subnet_mask: number
}

export interface FirewallRule {
  id: string
  instance_id: string
  direction: string
  protocol: string
  port_start: number
  port_end: number
  source_ip: string
  action: string
  priority: number
}

export interface Task {
  id: string
  type: string
  status: string
  node_id: string
  instance_id?: string
  payload: string
  result?: string
  error_message?: string
  retry_count: number
  max_retries: number
  created_at: string
  started_at?: string
  completed_at?: string
}

export interface MetricPoint {
  timestamp: string
  cpu: number
  mem_used: number
  mem_total: number
  disk_read: number
  disk_write: number
  net_in: number
  net_out: number
  net_in_total: number
  net_out_total: number
}

export interface AuditLog {
  id: string
  user_id: string
  action: string
  resource_type: string
  resource_id: string
  details: string
  ip_address: string
  created_at: string
}

export interface DashboardStats {
  total_users: number
  total_nodes: number
  total_instances: number
  running_instances: number
  recent_tasks: Task[]
  node_resources: Node[]
}
