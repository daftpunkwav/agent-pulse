# AgentPulse 部署文件

本目录包含 AgentPulse 的所有部署相关配置。

## 文件说明

```
deploy/
├── docker-compose.yml             # 本地开发依赖（ClickHouse/PG/Chroma）
├── init/
│   ├── clickhouse/01-init.sql     # ClickHouse 表结构与初始化
│   └── postgres/01-init.sql       # PostgreSQL 表结构与初始化数据
└── README.md                      # 本文件
```

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

## 数据保留策略

| 数据 | 存储 | TTL |
|------|------|-----|
| Trace 原始数据 | ClickHouse | 90 天 |
| 会话汇总 | ClickHouse | 365 天 |
| 评估结果 | PostgreSQL | 永久（可配置） |
| 失败聚类 | PostgreSQL | 永久 |
| Harness 配置 | PostgreSQL | 永久 |

修改 TTL：编辑 `init/clickhouse/01-init.sql` 中的 `INTERVAL 90 DAY` 等。

## 生产部署

生产环境建议：

- **ClickHouse**：集群部署（Zookeeper + 多副本）
- **PostgreSQL**：主从复制 + 定期备份
- **Chroma**：单实例足够（数据量不大），或迁移到专用向量数据库（Qdrant/Milvus）
- **后端服务**：Kubernetes 部署，水平扩展
- **负载均衡**：Nginx/Traefik 代理 API
- **HTTPS**：Let's Encrypt 自动证书

K8s manifests 后续在 `deploy/k8s/` 目录下提供。

## 添加新依赖

如需添加新服务（如 Redis、RabbitMQ）：

1. 在 `docker-compose.yml` 中添加 service
2. 如需初始化脚本，在 `init/` 下创建对应目录
3. 更新本 README 的端口表
4. 更新根目录 `.env.example` 中的相关配置