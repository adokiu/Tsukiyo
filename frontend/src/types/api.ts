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
  swap_mb: number
  disk_mb: number
  storage_pool: string
  public_ipv4: string
  public_ipv6: string
  network_down_mbps: number
  network_up_mbps: number
  io_read_iops: number
  io_write_iops: number
  monthly_traffic_gb: number
  current_traffic_gb: number
  traffic_mode: string
  snapshot_limit: number
  port_mapping_limit?: number
  ssh_password: string
  ssh_public_key: string
  expires_at: string
  expired_at?: string
  created_at: string
  data_disks?: DataDisk[]
  port_mappings?: PortMapping[]
  bridge_id?: string
  bridge_name?: string
  internal_ipv4?: string
  internal_ipv6?: string
  ipv4_eip?: string
  ipv6_eip?: string
}

export interface DataDisk {
  id: string
  name: string
  size_mb: number
  storage_pool: string
  mount_point: string
  status?: string
  updated_at?: string
}

export interface PortMapping {
  id: string
  instance_id: string
  bridge_id: string
  ip_version: string
  egress_allocation_id: string
  container_port: number
  host_port: number
  protocol: string
  description?: string
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
  dns_servers: string[]
  nat_egress_ipv4_id?: string
  ipv6_eip_pool_id?: string
  port_range_start: number
  port_range_end: number
  status: string
  instance_count?: number
  created_at: string
}

export interface EIPPool {
  id: string
  node_id: string
  ip_version: string
  cidr: string
  interface: string
  gateway: string
  prefix_len: number
  netmask_prefix: number
  alias: string
  pool_type: string
  status: string
  created_at: string
}

export interface EIPAllocation {
  id: string
  pool_id: string
  node_id: string
  cidr: string
  prefix_len: number
  ip_version: string
  usage: string
  bridge_id?: string
  instance_id?: string
  status: string
  allocated_at: string
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

export interface FirewallRule {
  id: string
  instance_id: string
  node_id: string
  network: string
  direction: string
  protocol: string
  port: string
  source_ip: string
  action: string
  description?: string
  enabled: boolean
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
