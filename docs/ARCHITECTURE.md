# Tsukiyo 系统架构设计

## 1. 系统概述

Tsukiyo 是一个基于 Incus 的多节点虚拟化管理平台，采用 Agent/Master 架构。
Master 部署在主控机上，负责全局资源调度、用户管理、实例编排。
Agent 部署在计算节点上，负责本地实例生命周期管理、网络配置、监控上报。
Agent 通过 WebSocket 长连接主动向 Master 注册并上报状态。

## 2. 技术栈

- **Master**: Go 1.24 + PostgreSQL 15+ + Redis 7+ + WebSocket + REST API + JWT
- **Agent**: Go 1.24 + Incus API (Unix Socket / REST) + WebSocket Client
- **共享协议**: JSON over WebSocket
- **前端**: React 18 + TypeScript + Vite + Tailwind CSS
- **虚拟化**: Incus (LXC 容器 + QEMU/KVM 虚拟机)
- **存储**: Incus 内置存储池 (ZFS/Btrfs/Dir/LVM)
- **网络**: Incus 托管桥接 + 公网 IP 池管理

## 3. 架构模式

```
                    [用户浏览器]
                         |
                    [Nginx/Traefik]
                         |
        +----------------+---------------+
        |                                |
   [Master Web]                    [Master WS]
   REST API                        Agent 管理
        |                                |
   [PostgreSQL]  <----------->  [Agent Node 1] (Incus)
   [Redis]                     [Agent Node 2] (Incus)
   [Task Queue]                [Agent Node 3] (Incus)
```

### 3.1 Master 职责

- 用户认证与授权 (RBAC)
- 节点注册与管理
- 实例元数据管理 (CRUD)
- 全局 IP 池分配 (公网 IPv4 / IPv6)
- 镜像模板管理
- 任务队列调度
- 监控数据聚合
- 审计日志
- WebSSH / WebVNC 代理
- Redis 缓存管理

### 3.2 Agent 职责

- 向 Master 发起 WebSocket 注册
- 心跳与资源上报 (每秒)
- 执行 Master 下发的实例操作指令
- 本地网络配置 (IP 绑定、端口映射、防火墙)
- 本地镜像缓存与下载
- 实例监控数据采集与上报
- WebSSH / WebVNC 本地连接中转
- 本地 Incus 存储池管理 (由 Master 统一下发初始化配置)
- 本地 Incus 网络管理 (由 Master 统一下发初始化配置)

## 4. Redis 缓存设计

### 4.1 缓存用途

| 用途 | Key 模式 | TTL | 说明 |
|------|---------|-----|------|
| 用户会话 | `session:{user_id}` | 24h | JWT 黑名单、登录状态 |
| Agent 连接状态 | `agent:{node_id}` | 60s | 在线状态、最后心跳 |
| 实例状态缓存 | `instance:{id}:status` | 30s | 减少数据库查询 |
| 实例指标缓存 | `instance:{id}:metrics` | 10s | 最新监控指标 |
| 用户权限缓存 | `user:{id}:perms` | 5min | RBAC 权限列表 |
| API 限流计数 | `rate_limit:{ip}` | 1min | 请求频率控制 |
| 任务队列 | `task_queue` | - | 待执行任务列表 |
| 镜像下载进度 | `image_dl:{image_id}` | 10min | 下载状态缓存 |
| 节点资源缓存 | `node:{id}:resources` | 10s | CPU/内存/磁盘实时数据 |

### 4.2 缓存一致性策略

采用 **Cache-Aside + 延迟双删** 策略：

1. **读流程**: 先查缓存，命中返回；未命中查数据库，回填缓存
2. **写流程**: 先更新数据库，成功后再删除缓存；延迟 500ms 后二次删除缓存
3. **删除流程**: 先删除数据库，再立即删除缓存

**关键保证**:
- 所有缓存数据均为非关键数据，缓存 miss 可安全回查数据库
- 用户权限缓存 TTL 短 (5min)，权限变更后最多延迟 5min 生效
- 实例状态缓存 TTL 30s，状态变更通过 Agent 主动推送更新
- 数据库事务成功后才删除缓存，避免脏数据

### 4.3 缓存穿透/击穿/雪崩防护

- **穿透**: 对不存在 key 设置空值缓存 (TTL 60s)
- **击穿**: 热点 key 永不过期 + 定时异步刷新
- **雪崩**: 随机 TTL 偏移 (基础 TTL + 0~30s 随机值)

## 5. 数据库设计 (PostgreSQL)

### 5.1 用户表 (users)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID PK | 用户 ID |
| username | VARCHAR(64) UNIQUE | 用户名 |
| email | VARCHAR(255) UNIQUE | 邮箱 |
| password_hash | VARCHAR(255) | bcrypt 哈希 |
| status | VARCHAR(16) | active / suspended / deleted |
| created_at | TIMESTAMPTZ | 创建时间 |
| updated_at | TIMESTAMPTZ | 更新时间 |

### 5.2 用户组表 (user_groups)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID PK | 组 ID |
| name | VARCHAR(64) UNIQUE | 组名称 |
| description | TEXT | 描述 |
| is_builtin | BOOLEAN | 是否为内置组 |
| created_at | TIMESTAMPTZ | 创建时间 |

### 5.3 用户与组关联表 (user_group_members)

| 字段 | 类型 | 说明 |
|------|------|------|
| user_id | UUID FK | 用户 ID |
| group_id | UUID FK | 组 ID |
| assigned_at | TIMESTAMPTZ | 加入时间 |
| assigned_by | UUID FK | 分配者 |

### 5.4 权限定义表 (permissions)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | VARCHAR(64) PK | 权限标识 |
| name | VARCHAR(128) | 权限名称 |
| resource | VARCHAR(32) | 资源类型 |
| action | VARCHAR(32) | 操作类型 |
| description | TEXT | 描述 |

### 5.5 组权限关联表 (group_permissions)

| 字段 | 类型 | 说明 |
|------|------|------|
| group_id | UUID FK | 组 ID |
| permission_id | VARCHAR(64) FK | 权限 ID |
| scope | VARCHAR(16) | all / own / group / node |
| scope_target | UUID | 作用域目标 |
| created_at | TIMESTAMPTZ | 创建时间 |

### 5.6 节点表 (nodes)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID PK | 节点 ID |
| name | VARCHAR(64) | 节点名称 |
| token | VARCHAR(255) | 注册 Token |
| hostname | VARCHAR(255) | Agent 主机名 |
| ip_address | INET | Agent 外网 IP |
| status | VARCHAR(16) | online / offline / maintenance |
| incus_version | VARCHAR(32) | Incus 版本 |
| total_cpu | FLOAT | 总 CPU |
| total_memory | BIGINT | 总内存 (MB) |
| total_disk | BIGINT | 总磁盘 (GB) |
| last_seen | TIMESTAMPTZ | 最后心跳时间 |
| created_at | TIMESTAMPTZ | 创建时间 |

### 5.7 实例表 (instances)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID PK | 实例 ID |
| name | VARCHAR(64) | 实例名称 |
| user_id | UUID FK | 所属用户 |
| node_id | UUID FK | 所在节点 |
| type | VARCHAR(16) | container / vm |
| status | VARCHAR(16) | running / stopped / creating / error |
| incus_name | VARCHAR(64) | Incus 内部名称 |
| template_id | VARCHAR(64) | 镜像模板 ID |
| vcpu | FLOAT | CPU 核心数 |
| memory_mb | INT | 内存 (MB) |
| disk_gb | INT | 磁盘 (GB) |
| ipv4_address | INET | 内网 IPv4 |
| ipv6_address | INET | 内网 IPv6 |
| ssh_port | INT | SSH 映射端口 |
| created_at | TIMESTAMPTZ | 创建时间 |
| expires_at | TIMESTAMPTZ | 到期时间 |

### 5.8 公网 IP 池表 (public_ip_pools)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID PK | IP ID |
| node_id | UUID FK | 所属节点 |
| address | INET | IP 地址 |
| gateway | INET | 网关 |
| prefix_len | INT | 前缀长度 |
| interface | VARCHAR(32) | 绑定网卡 |
| status | VARCHAR(16) | free / assigned / reserved |
| instance_id | UUID FK | 绑定实例 |
| assigned_at | TIMESTAMPTZ | 分配时间 |

### 5.9 IPv6 前缀表 (ipv6_prefixes)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID PK | 前缀 ID |
| node_id | UUID FK | 所属节点 |
| prefix | CIDR | IPv6 前缀 |
| prefix_len | INT | 前缀长度 |
| interface | VARCHAR(32) | 关联网卡 |
| gateway | INET | 网关 |
| status | VARCHAR(16) | active / inactive |

### 5.10 端口映射表 (port_mappings)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID PK | 映射 ID |
| instance_id | UUID FK | 实例 ID |
| node_id | UUID FK | 节点 ID |
| container_port | INT | 容器端口 |
| host_port | INT | 宿主机端口 |
| protocol | VARCHAR(8) | tcp / udp |
| host_ip | INET | 宿主机绑定 IP |

### 5.11 镜像模板表 (image_templates)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | VARCHAR(64) PK | 模板 ID |
| name | VARCHAR(128) | 显示名称 |
| type | VARCHAR(16) | container / vm |
| distro | VARCHAR(32) | 发行版 |
| release | VARCHAR(32) | 版本 |
| arch | VARCHAR(16) | 架构 |
| url | VARCHAR(512) | 下载地址 |
| description | TEXT | 描述 |
| enabled | BOOLEAN | 是否启用 |
| desktop | VARCHAR(32) | 桌面环境 |

### 5.12 审计日志表 (audit_logs)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID PK | 日志 ID |
| user_id | UUID FK | 操作用户 |
| action | VARCHAR(64) | 操作类型 |
| target | VARCHAR(64) | 操作对象 |
| detail | TEXT | 详情 |
| ip_address | INET | 来源 IP |
| success | BOOLEAN | 是否成功 |
| created_at | TIMESTAMPTZ | 时间 |

### 5.13 任务队列表 (tasks)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID PK | 任务 ID |
| type | VARCHAR(32) | 任务类型 |
| node_id | UUID FK | 目标节点 |
| instance_id | UUID FK | 目标实例 |
| status | VARCHAR(16) | pending / running / completed / failed |
| payload | JSONB | 任务参数 |
| result | JSONB | 执行结果 |
| error | TEXT | 错误信息 |
| created_at | TIMESTAMPTZ | 创建时间 |
| completed_at | TIMESTAMPTZ | 完成时间 |

### 5.14 监控指标表 (instance_metrics)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGSERIAL PK | 自增 ID |
| instance_id | UUID | 实例 ID |
| node_id | UUID | 节点 ID |
| timestamp | TIMESTAMPTZ | 采集时间 |
| cpu_percent | FLOAT | CPU 使用率 |
| mem_used | BIGINT | 内存已用 (MB) |
| mem_total | BIGINT | 内存总量 (MB) |
| disk_read_bps | BIGINT | 磁盘读速度 (B/s) |
| disk_write_bps | BIGINT | 磁盘写速度 (B/s) |
| net_in_bps | BIGINT | 网络入速度 (B/s) |
| net_out_bps | BIGINT | 网络出速度 (B/s) |
| net_in_total | BIGINT | 网络入累计 (Bytes) |
| net_out_total | BIGINT | 网络出累计 (Bytes) |

## 6. 用户系统设计

### 6.1 用户与实例完全解耦

用户和实例是独立的一级资源。用户可以在没有任何实例的情况下存在。
创建用户时不分配任何实例。管理员可以在任何时候为用户分配实例。
用户可以拥有多个实例，实例也可以在用户之间迁移。

### 6.2 用户组与权限体系

所有用户没有本质区别，不通过角色字段区分身份。
权限完全由所属的用户组决定，支持多对多关联。

**默认用户组**:
- **管理员组 (admin)**: 默认拥有所有权限
- **普通用户组 (user)**: 默认只能管理自己的实例

### 6.3 权限模型

- **实例权限**: `instance:create`, `instance:read`, `instance:update`, `instance:delete`, `instance:start`, `instance:stop`, `instance:restart`, `instance:console`, `instance:snapshot`, `instance:reinstall`, `instance:migrate`
- **用户权限**: `user:create`, `user:read`, `user:update`, `user:delete`, `user:group_manage`
- **节点权限**: `node:create`, `node:read`, `node:update`, `node:delete`, `node:monitor`
- **网络权限**: `network:manage`, `network:ip_allocate`, `network:port_forward`
- **镜像权限**: `image:create`, `image:read`, `image:update`, `image:delete`, `image:download`
- **审计权限**: `audit:read`, `audit:manage`
- **系统权限**: `system:config`, `system:log`, `system:backup`

## 7. 网络架构设计（VPC 模型）

### 7.1 核心原则

- 宿主机始终作为流量网关和路由器，实例不直连上层物理网络。
- **IPv4**：实例仅持内网 IP，公网 IPv4 通过宿主机做 **1:1 NAT（DMZ）** 映射，出站共享或独立公网 IP。
- **IPv6**：实例直接持有 IPv6 地址（内网 ULA 或公网 GUA），**无需 NAT**，通过宿主机做 IPv6 路由转发，流量必须经过宿主机网关。

系统采用**云厂商 VPC 模型**：管理员在 Master Web 端创建多个 VPC 网络，每个 VPC 是一个独立的 Incus `bridge`，拥有独立的 IPv4 内网段、IPv6 内网段、IPv6 公网段，以及绑定的宿主机 IPv4 出口地址池。

### 7.2 VPC 网络（Bridge）—— 实例唯一的网络层

管理员创建 VPC 时配置以下参数：

| 配置项 | 说明 |
|--------|------|
| `ipv4_cidr` | IPv4 内网段，如 `10.10.1.0/24` |
| `ipv6_ula_cidr` | IPv6 ULA 内网段，如 `fd00:1::/64`（可选） |
| `ipv6_gua_cidr` | IPv6 公网段，如 `2001:db8:1::/64`（可选） |
| `default_gateway_v4` | 该 VPC 的 IPv4 默认网关地址（bridge 网关 IP） |
| `default_gateway_v6` | 该 VPC 的 IPv6 默认网关地址（bridge 网关 IP） |
| `egress_v4_primary` | 宿主机上绑定的**默认出口 IPv4**（SNAT 共享上网用） |
| `egress_v4_extra` | 宿主机上绑定的**额外 IPv网 IP 地址池**（给实例分配独立公网 IP 用） |
| `port_range_start` | 端口映射**起始端口**，如 `10000` |
| `port_range_end` | 端口映射**结束端口**，如 `65535` |
| `address_aliases` | 地址别名映射（JSON），键为宿主机网卡上的真实IP，值为展示给用户的别名IP |
| `parent_iface` | 宿主机物理父网卡，如 `eth0` |

- 每个 VPC 对应一个 Incus `bridge` 网络，独立子网、独立路由表。
- 一个 Agent 节点可创建**多个 VPC**，实现多租户网络隔离。
- 创建 VM/LXC 时**必须选择所属 VPC**，不允许游离在网络之外的实例。

### 7.3 IPv4 地址分配策略

**内网 IP**
- 所有实例均从 VPC 的 `ipv4_cidr` 中由 Master IP 池**静态分配**内网 IP。
- 通过 `ipv4.address` 硬编码到实例 nic，关闭 bridge 的 DHCP，防止地址冲突。
- 启用 `security.ipv4_filter=true` + `security.mac_filter=true`，防止用户篡改 IP/MAC。

**公网 IPv4（1:1 NAT / DMZ）**
- 实例本身**不直接配置公网 IPv4**，公网 IP 资源绑定在宿主机层面。
- **共享出口**：无独立公网 IP 的实例，出站通过 VPC 的 `egress_v4_primary` 做 SNAT 共享上网。
- **独立公网 IP**：从 VPC 绑定的 `egress_v4_extra` 地址池中选一个地址，通过 Incus `network forward` 做 1:1 NAT 映射到实例内网 IP：
  - DNAT：公网 IP 全部入站流量 -> 实例内网 IP
  - SNAT：该实例所有出站流量源地址改写为绑定的公网 IP
- 一个实例可以绑定**多个**独立公网 IPv4（多 IP 配置）。

**端口映射（NAT 端口转发）**
- 端口映射是**实例级别**配置，每个实例可独立开启或关闭。
- 映射使用的外部 IP 地址固定为所属 VPC 的 `egress_v4_primary`（默认网关地址），即所有实例的端口映射都共享同一个出口 IP，通过不同端口号区分。
- 外部端口从 VPC 设定的 `port_range_start` ~ `port_range_end` 范围内由 Master 自动分配，防止冲突。
- 实现方式：Incus `proxy` device 或 `network forward`，将 `egress_v4_primary:外部端口` 转发到 `实例内网IP:内部端口`。

### 7.4 IPv6 地址分配策略

**核心设计：IPv6 无 NAT，宿主机做路由转发**

- 实例直接持有 IPv6 地址，可以是 **ULA 内网地址**（`fd00::/8`）或 **GUA 公网地址**（`2000::/3`）。
- 创建实例时可配置：是否启用 IPv6、IPv6 地址数量、每个地址的前缀长度。

**内网 IPv6（ULA）**
- 从 VPC 的 `ipv6_ula_cidr` 中静态分配，如 `fd00:1::10/64`。
- 仅用于 VPC 内部通信，不对外路由。

**公网 IPv6（GUA）**
- 从 VPC 的 `ipv6_gua_cidr` 中静态分配，如 `2001:db8:1::10/64`。
- **路由模式**：宿主机在上层网络中作为该 `/64` 或 `/120` 前缀的下一跳路由器：
  - 宿主机物理接口持有该段的路由锚点地址（如 `2001:db8:1::1/64`）。
  - 上层路由器配置静态路由：`ipv6_gua_cidr` 的下一跳为宿主机物理接口的链路本地地址或 GUA 地址。
  - 或者宿主机启用 **ND Proxy**（`ndppd` / `kernel proxy_ndp`），代替实例响应上层网络的 NDP 邻居发现请求。
- 实例网关指向 bridge 的 IPv6 地址（如 `2001:db8:1::1`），所有出入 IPv6 流量必须经过宿主机 bridge 网关转发。
- **防篡改**：`security.ipv6_filter=true`（Incus 原生）防止用户私自修改 IPv6 地址。

### 7.5 宿主机出口地址管理

宿主机网卡上的 IPv4/IPv6 地址由管理员在 Master Web 端登记为**节点地址池**：

```
宿主机 eth0 地址池示例：
- 203.0.113.1/24     -> 主地址，可作为默认网关/SNAT 出口
- 203.0.113.2/32     -> 额外地址 #1，可分配给实例做 1:1 NAT
- 203.0.113.3/32     -> 额外地址 #2
- 2001:db8::1/64     -> IPv6 主地址/路由锚点
- 2001:db8:1::/64    -> 可分配给 VPC-1 的 IPv6 GUA 段
- 2001:db8:2::/64    -> 可分配给 VPC-2 的 IPv6 GUA 段
```

- 创建 VPC 时，管理员从节点地址池中选择：
  - `egress_v4_primary`：该 VPC 所有无独立公网 IP 的实例共享此地址做 SNAT 上网
  - `egress_v4_extra[]`：该 VPC 可分配给实例的独立公网 IPv4 地址列表
  - `ipv6_gua_cidr`：该 VPC 的 IPv6 公网段（从宿主机持有的更大段中划分）
- 地址分配关系持久化到数据库，Agent 启动/重建时自动按 Master 下发的规则重新配置。

### 7.6 多网络共存与流量隔离模型

```
+----------------------------------------------------------+
|                      Agent 宿主机                         |
|  宿主机作为 IPv4 NAT 网关 + IPv6 路由器                   |
|                                                          |
|  eth0: 203.0.113.1/24                                    |
|  eth0:0: 203.0.113.2/32, 203.0.113.3/32 ... 别名IP       |
|  eth0: 2001:db8::1/64  (IPv6 路由锚点)                   |
|                                                          |
|  +------------------------+  +------------------------+  |
|  | VPC-1: bridge-nat1     |  | VPC-2: bridge-nat2     |  |
|  |  v4: 10.10.1.0/24      |  |  v4: 10.20.1.0/24      |  |
|  |  v6 ULA: fd00:1::/64   |  |  v6 ULA: fd00:2::/64   |  |
|  |  v6 GUA: 2001:db8:1::/64|  |  v6 GUA: 2001:db8:2::/64|  |
|  |  egress_v4_primary:    |  |  egress_v4_primary:    |  |
|  |    203.0.113.1         |  |    203.0.113.1         |  |
|  |  egress_v4_extra:      |  |  egress_v4_extra:      |  |
|  |    203.0.113.2         |  |    203.0.113.3         |  |
|  +--------+---------------+  +--------+---------------+  |
|           |                          |                  |
|  +--------v----------+      +--------v----------+      |
|  | VM-A (VPC-1)      |      | VM-B (VPC-2)      |      |
|  | v4: 10.10.1.10/24 |      | v4: 10.20.1.20/24 |      |
|  | v6: 2001:db8:1::10/64   |      | v6: 2001:db8:2::20/64   |      |
|  | pub_v4: 203.0.113.2/32  |      | pub_v4: 203.0.113.3/32  |      |
|  +-------------------+      +-------------------+      |
|                                                          |
|  流量路径：                                               |
|  v4 入站: 203.0.113.2 -> [宿主机 DNAT] -> 10.10.1.10    |
|  v4 出站: 10.10.1.10 -> [宿主机 SNAT] -> 203.0.113.2    |
|  v6 入站: 2001:db8:1::10 -> [宿主机路由转发] -> VM-A    |
|  v6 出站: VM-A -> [宿主机网关 fd00:1::1] -> 上层路由    |
+----------------------------------------------------------+
```

- **纯内网实例**：仅分配 v4 内网 + v6 ULA，v4 出站共享 `egress_v4_primary` 的 SNAT。
- **公网 v4 实例**：从内网 IP + `egress_v4_extra` 中分配的独立公网 IP 做 1:1 NAT。
- **公网 v6 实例**：直接分配 GUA 地址，宿主机做 IPv6 路由转发，无需 NAT。
- **多网卡实例**：可接入多个 VPC（多网卡），每个网卡独立配置 v4/v6 地址。
- **VM 与 LXC 共存**：同一 VPC 内 VM 和 LXC 共享同一 bridge，Incus 自动处理网络设备差异。
- Agent 配置文件**仅包含 token 和 master 地址**，所有 VPC、地址池、NAT、路由规则均由 Master Web 端配置后通过 WebSocket 指令下发。

## 8. 流量控制

支持四种流量计算模式: `total`, `outbound`, `inbound`, `max`。
超额处理策略: `shutdown` (停止实例) / `throttle` (限速到 1Mbps)。

## 9. 监控与告警

Agent 每 1 秒采集并上报，Master 持久化到时序表，支持 30d/7d/24h/1h/15min 回放。

---

文档版本: v1.0
