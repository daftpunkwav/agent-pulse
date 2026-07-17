# AgentPulse 部署文件

本目录包含 AgentPulse 的所有部署相关配置。

## 文件说明

```
deploy/
├── docker/                         # Dockerfile（多阶段构建）
│   ├── backend.Dockerfile          # Go 后端镜像（distroless）
│   └── web.Dockerfile              # Next.js 前端镜像（standalone）
├── init/
│   ├── clickhouse/01-init.sql      # ClickHouse 表结构与初始化
│   └── postgres/01-init.sql        # PostgreSQL 表结构与初始化数据
├── k8s/                            # Kubernetes manifests
│   ├── base/                       # 基础资源（Deployment/Service/StatefulSet/Ingress/NetworkPolicy）
│   │   └── secrets.example.yaml    # Secret 模板（勿直接 apply 生产）
│   └── overlays/production/        # 生产 overlay（扩缩容 + secretGenerator + TLS）
│       └── secrets.env.example     # 复制为 secrets.env 后 kustomize build
├── docker-compose.yml              # 本地基础设施（端口仅绑 127.0.0.1）
└── README.md                       # 本文件
```

## Kubernetes 密钥与连接

**仓库内不提交真实 Secret。** base 中的 Backend 通过 `secretKeyRef` 引用 `agentpulse-secrets`，需先创建：

```bash
# 方式 A：示例文件改名后 apply（仅开发）
cp k8s/base/secrets.example.yaml /tmp/agentpulse-secrets.yaml
# 编辑 /tmp/agentpulse-secrets.yaml 填入强密钥
kubectl apply -f /tmp/agentpulse-secrets.yaml

# 方式 B：生产 overlay（推荐）
cd k8s/overlays/production
cp secrets.env.example secrets.env
# 编辑 secrets.env
kubectl apply -k .
```

Backend 注入的环境变量包括：

| 变量 | 用途 |
|------|------|
| `AGENTPULSE_AUTH_API_KEYS` | API 鉴权 |
| `AGENTPULSE_POSTGRES_*` | Postgres 连接 |
| `AGENTPULSE_CLICKHOUSE_*` | ClickHouse 连接 |
| `AGENTPULSE_CHROMA_*` | Chroma 连接与 token |
| `AGENTPULSE_JUDGE_API_KEY` | LLM Judge（release 必填） |

Chroma 使用 PVC 持久化 + Token 鉴权；探活请求携带 `Authorization: Bearer`。

## 启动本地基础设施

```bash
# 启动所有依赖
docker compose up -d

# 查看服务状态
docker compose ps

# 查看日志
docker compose logs -f clickhouse
docker compose logs -f postgres
docker compose logs -f chroma

# 停止
docker compose down

# 重置（删除所有数据）
docker compose down -v
```

## 服务端口

| 服务 | 端口 | 用途 |
|------|------|------|
| ClickHouse | 9000 | Native protocol（后端连接） |
| ClickHouse | 8123 | HTTP interface（调试） |
| PostgreSQL | 5432 | 元数据存储 |
| Chroma | 8000 | 向量存储 |
| AgentPulse 后端 | 8080 | REST API |
| AgentPulse 后端 | 4318 | OTLP HTTP |
| AgentPulse 后端 | 4317 | OTLP gRPC |
| Next.js 前端 | 3000 | Web Dashboard |

## 数据保留策略

| 数据 | 存储 | TTL |
|------|------|-----|
| Trace 原始数据 | ClickHouse | 90 天 |
| 会话汇总 | ClickHouse | 365 天 |
| 评估结果 | PostgreSQL | 永久（可配置） |
| 失败聚类 | PostgreSQL | 永久 |
| Harness 配置 | PostgreSQL | 永久 |

修改 TTL：编辑 `init/clickhouse/01-init.sql` 中的 `INTERVAL 90 DAY` 等。

## Docker 镜像

### 后端镜像 (`docker/backend.Dockerfile`)

多阶段构建：
- **Stage 1 (builder)**: `golang:1.25-alpine`，编译 `./cmd/server`，CGO_ENABLED=0
- **Stage 2 (runtime)**: `gcr.io/distroless/static-debian12:nonroot`，非 root 用户运行
- 暴露端口：`8080`（REST API）、`4318`（OTLP HTTP）、`4317`（OTLP gRPC）

### 前端镜像 (`docker/web.Dockerfile`)

三阶段构建：
- **deps**: `node:20-alpine`，安装依赖
- **builder**: `node:20-alpine`，构建 Next.js standalone 输出
- **runner**: `node:20-alpine`，`nextjs` 用户（UID 1001）运行
- 暴露端口：`3000`

## Kubernetes 部署

`k8s/base/` 包含完整 K8s 资源清单：

| 资源 | 说明 |
|------|------|
| `backend.yaml` | Deployment + Service（1 副本，100m-1 CPU / 256-512Mi） |
| `web.yaml` | Deployment + Service（1 副本，50-500m CPU / 128-256Mi） |
| `postgres.yaml` | StatefulSet + Service（5Gi PVC） |
| `clickhouse.yaml` | StatefulSet + Service（20Gi PVC） |
| `chroma.yaml` | Deployment + Service（持久化存储） |
| `ingress.yaml` | Nginx Ingress（`/` → web:3000, `/api` → backend:8080） |
| `networkpolicy.yaml` | Default-deny + 仅允许 `agentpulse` 和 `ingress-nginx` 命名空间 |

**生产扩缩容**: `k8s/overlays/production/` 将 backend 和 web 各扩至 2 副本。

应用方式：
```bash
kubectl apply -k k8s/base
# 或生产环境
kubectl apply -k k8s/overlays/production
```

## 生产部署

生产环境建议：

- **ClickHouse**：集群部署（Zookeeper + 多副本）
- **PostgreSQL**：主从复制 + 定期备份
- **Chroma**：单实例足够（数据量不大），或迁移到专用向量数据库（Qdrant/Milvus）
- **后端服务**：Kubernetes 部署，水平扩展
- **负载均衡**：Nginx/Traefik 代理 API
- **HTTPS**：Let's Encrypt 自动证书

K8s manifests 见 `deploy/k8s/base/`，生产扩缩容见 `deploy/k8s/overlays/production/`。

## 添加新依赖

如需添加新服务（如 Redis、RabbitMQ）：

1. 在 `docker-compose.yml` 中添加 service
2. 如需初始化脚本，在 `init/` 下创建对应目录
3. 更新本 README 的端口表
4. 更新根目录 `.env.example` 中的相关配置