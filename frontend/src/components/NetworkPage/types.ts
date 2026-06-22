export interface Node {
  id: string
  name: string
  status: string
}

export interface IPProbe {
  interface: string
  address: string
  prefix_len: number
  scope: string
  gateway?: string
}

export interface NodeNetwork {
  name: string
  state: string
  mac: string
  ipv4: IPProbe[]
  ipv6: IPProbe[]
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
  nat_egress_ipv4_addr?: string
  port_range_start: number
  port_range_end: number
  port_used: number
  port_total: number
  status: string
  instance_count: number
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
  used_count: number
  total_ips: number
  created_at: string
}

export interface EIPAllocation {
  id: string
  pool_id: string
  node_id: string
  cidr: string
  prefix_len: number
  ip_version: string
  alias: string
  usage: string
  bridge_id?: string
  instance_id?: string
  status: string
  allocated_at: string
}

export interface EIPPoolDraftItem {
  id: string
  cidr: string
  cidrManual: boolean
  gateway: string
  gatewayManual: boolean
  interface: string
  prefix: string
  hostAddr: string
  netmask: string
  alias: string
  poolType: 'host' | 'eip'
  detecting: boolean
}

export function makeDraftItem(): EIPPoolDraftItem {
  return { id: Math.random().toString(36).slice(2), cidr: '', cidrManual: false, gateway: '', gatewayManual: false, interface: '', prefix: '', hostAddr: '', netmask: '', alias: '', poolType: 'eip', detecting: false }
}
