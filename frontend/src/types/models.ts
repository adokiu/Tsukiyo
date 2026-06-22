export interface IPProbe {
  interface: string
  address: string
  prefix_len: number
  gateway?: string
}

export interface IPv4Prefix {
  interface: string
  address: string
  prefix: string
  source: string
}

export interface IPv6Prefix {
  address: string
  prefix_len: number
  interface: string
  gateway: string
}

export interface GatewayProbe {
  family: string
  interface: string
  gateway: string
}

export interface GPUInfo {
  name: string
  vendor: string
  driver: string
  type: string
}

export interface MemModule {
  locator: string
  size: string
  type: string
  speed: string
  manufacturer: string
  serial_number: string
}

export interface DiskSMART {
  available: boolean
  life_used_percent?: number
  power_on_hours?: number
  temperature?: number
  health_status?: string
  model?: string
  serial?: string
  firmware?: string
  media_errors?: number
}

export interface EnvCheck {
  key: string
  label: string
  ok: boolean
  detail: string
}

export interface SystemInfo {
  generated_at?: string
  hostname?: string
  os?: string
  kernel?: string
  arch?: string
  cpu_model?: string
  cpu_sockets?: number
  cpu_cores?: number
  cpu_threads?: number
  cpu_freq?: string
  memory_total?: string
  memory_used?: string
  memory_free?: string
  swap_total?: string
  swap_used?: string
  uptime?: string
  load_avg?: string
  disk_total?: string
  disk_used?: string
  disk_free?: string
  gpus?: GPUInfo[]
  memory_modules?: MemModule[]
  disk_smart?: DiskSMART[]
  environment?: EnvCheck[]
}

export interface NodeNetwork {
  name: string
  state: string
  mac: string
  mtu: number
  ipv4: IPProbe[]
  ipv6: IPProbe[]
}

export interface NodeListItem {
  id: string
  name: string
  token: string
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
  system_info?: SystemInfo
  network_interfaces?: NodeNetwork[]
}

export interface InstalledImage {
  id: string
  fingerprint: string
  alias: string
  type: string
  size: number
  architecture: string
  created_at: string
  category_id?: string
  category_name?: string
  install_ssh: boolean
}

export interface ImageCategory {
  id: string
  name: string
  image_type: string
  sort_order: number
}

export interface StoragePoolInfo {
  name: string
  driver: string
  size: number
  used: number
}

export interface BridgeListItem {
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

export interface EIPPoolListItem {
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

export interface EIPAllocationListItem {
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

export interface SnapshotInfo {
  id: string
  name: string
  created_at: string
  size?: number
}

export interface InstanceMetrics {
  cpu_usage?: number
  memory_usage?: number
  memory_total?: number
  memory_used?: number
  disk_read?: number
  disk_write?: number
  net_in?: number
  net_out?: number
  net_in_total?: number
  net_out_total?: number
  monthly_traffic?: number
}

export interface InstanceMetricPoint {
  timestamp: string
  cpu: number
  cpu_max?: number
  mem_used: number
  mem_total: number
  disk_read: number
  disk_write: number
  net_in: number
  net_out: number
  net_in_total: number
  net_out_total: number
  net_in_max?: number
  net_out_max?: number
  net_in_min?: number
  net_out_min?: number
}

export interface EIPPoolDraftItem {
  id: string
  cidr: string
  cidrManual: boolean
}
