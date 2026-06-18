# Tsukiyo

基于 Incus 的多节点虚拟化管理平台，支持 LXC 容器和 QEMU/KVM 虚拟机。

## 架构

Tsukiyo 采用 Agent/Master 架构：

- **Master**: 部署在主控机，负责全局资源调度、用户管理、实例编排、任务下发
- **Agent**: 部署在计算节点，负责本地 Incus 实例生命周期管理、网络配置、监控上报
- **Frontend**: React + TypeScript + Tailwind CSS 管理面板

```
[用户浏览器] -> [Nginx] -> [Master]
                              |
                        [PostgreSQL]
                        [Redis]
                              |
                    [Agent Node 1] (Incus)
                    [Agent Node 2] (Incus)
```

## 技术栈

| 组件 | 技术 |
|------|------|
| Master | Go 1.24 + Gin + PostgreSQL + Redis |
| Agent | Go 1.24 + Incus API + WebSocket |
| Frontend | React 18 + TypeScript + Vite + Tailwind CSS |
| 虚拟化 | Incus (LXC 容器 + QEMU/KVM 虚拟机) |

## 目录结构

```
master/          # 主控后端
agent/           # 节点 Agent
frontend/        # 管理面板
docs/            # 架构文档
```

## 核心功能

- 多节点集群管理（Agent 主动注册）
- VPC 网络隔离（自定义 CIDR、SNAT 出口 IP）
- 端口映射（Incus proxy device，TCP/UDP/both）
- 实例生命周期（创建、启动、停止、删除、重装、快照）
- 监控指标（CPU、内存、磁盘、网络流量）
- 镜像管理（容器模板 + VM cloud image）

## 快速开始

```bash
# 编译 Master
cd master && go build -o tsukiyo-master ./cmd/master

# 编译 Agent（Linux 交叉编译）
cd agent && GOOS=linux GOARCH=amd64 go build -ldflags='-s -w' -o tsukiyo-agent-linux-amd64 .

# 编译前端
cd frontend && npm install && npm run build
```

## License

MIT
