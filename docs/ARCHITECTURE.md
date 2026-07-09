# AgentPulse Architecture

> Last updated: 2026-07-09 · MVP v0.1.0

## 1. 概述

AgentPulse 是一个面向 LLM Agent 的运维（AgentOps）平台。它为运行中的 Agent 系统提供可观测性、成本归因、质量评估、失败分析等能力。

## 2. 设计原则

| 原则 | 含义 |
|------|------|
| **接口与实现分离** | 所有持久化与外部依赖都通过 domain 接口抽象，业务代码不直接依赖 ClickHouse/PG/Chroma |
| **显式依赖注入** | 没有全局变量/单例；所有依赖通过构造函数注入 |
| **优雅关闭** | 服务按依赖顺序反向关闭，超时可控 |
| **接口预留可扩展** | Judge/Vector/Repository 都用接口抽象，便于 Phase 2 替换实现 |
| **OTel 标准兼容** | Trace 通过 OpenTelemetry 协议上报，可对接 Langfuse/Jaeger 等生态 |

## 3. 系统架构

```
┌────────────────────────────────────────────────────────────────┐
│                AgentPulse 整体架构                               │
└────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────┐
│  Layer 0: 用户/SDK                                              │
│  ┌──────────────┐ ┌──────────────┐ ┌────────────────────────┐  │
│  │ Python SDK   │ │ Go SDK       │ │ 任意 OTLP 客户端       │  │
│  │ (装饰器)     │ │ (装饰器)     │ │ (LangChain/...)        │  │
│  └──────────────┘ └──────────────┘ └────────────────────────┘  │
└────────────────────────────┬───────────────────────────────────┘
                             │ OTLP/HTTP protobuf
                             ▼
┌────────────────────────────────────────────────────────────────┐
│  Layer 1: 接收层                                                │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  OTLP HTTP Receiver (Go :4318)                              ││
│  │  - 解析 OTLP protobuf                                        ││
│  │  - 映射 OTel GenAI 语义约定 + ap.* 自定义属性                ││
│  │  - 异步批量写入                                              ││
│  └─────────────────────────────────────────────────────────────┘│
└────────────────────────────┬───────────────────────────────────┘
                             │ domain.Span
                             ▼
┌────────────────────────────────────────────────────────────────┐
│  Layer 2: 业务服务 (Service)                                     │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────────────────┐ │
│  │ SpanService  │ │ CostService  │ │ EvalService              │ │
│  │ - 异步批写    │ │ - 6维归因    │ │ - LLM-as-Judge          │ │
│  │ - 自动算成本  │ │ - 时间序列   │ │ - 异步队列 + 采样        │ │
│  └──────────────┘ └──────────────┘ └──────────────────────────┘ │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────────────────┐ │
│  │ ClusterSvc   │ │ MetadataSvc  │ │ PricingSvc (internal)    │ │
│  │ - 规则聚类    │ │ - Harness/AB │ │ - 时间窗口价格匹配        │ │
│  └──────────────┘ └──────────────┘ └──────────────────────────┘ │
└────────────────────────────┬───────────────────────────────────┘
                             │ domain.Repository 接口
                             ▼
┌────────────────────────────────────────────────────────────────┐
│  Layer 3: 仓储 (Repository)                                     │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────────────────┐ │
│  │ ClickHouse   │ │ PostgreSQL   │ │ Chroma (optional)        │ │
│  │ Span Repo    │ │ Eval Repo    │ │ Vector Repo              │ │
│  │              │ │ Metadata Repo│ │                          │ │
│  │              │ │ Pricing Repo │ │                          │ │
│  └──────────────┘ └──────────────┘ └──────────────────────────┘ │
└────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────┐
│  Layer 4: API 入口                                              │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  HTTP REST API (Gin :8080)                                  ││
│  │  /api/v1/traces/*    /api/v1/cost/*                         ││
│  │  /api/v1/eval/*      /api/v1/clusters/*                     ││
│  │  /api/v1/harness/*   /api/v1/abtests/*                      ││
│  └─────────────────────────────────────────────────────────────┘│
└────────────────────────────────────────────────────────────────┘
```

## 4. 数据流

### 4.1 写入流（Trace 上报）

```
Agent 业务调用
    │
    ▼
SDK 装饰器拦截 → 包装 OpenTelemetry Span
    │
    ▼
OTLP/HTTP POST /v1/traces  (protobuf)
    │
    ▼
collector/otlp_http.go: parse OTLP → domain.Span
    │
    ▼
service.IngestSpans()
    │
    ▼
spanService.fillMissingCost()  // 用 pricing 表补全 cost_usd
    │
    ▼
异步队列 + Worker Pool（批量 100 / 5s 刷新）
    │
    ▼
ClickHouse: agent_spans 表
```

### 4.2 评估触发流

```
Span 写入完成后 → service 决定是否触发评估
    │
    ▼
根据 sample_rate 采样（默认 100%）
    │
    ▼
EvalService 异步队列 → Worker
    │
    ▼
构建 Judge Prompt（system + user_input + agent_output + tool_calls）
    │
    ▼
OpenAI API 调用（gpt-4o-mini 等）
    │
    ▼
解析 JSON 五维分数
    │
    ▼
PostgreSQL: evaluations 表
```

### 4.3 查询流（Dashboard）

```
Web 前端 (Next.js) → fetch /api/backend/cost/breakdown
    │
    ▼
Next.js rewrites → http://localhost:8080/api/v1/cost/breakdown
    │
    ▼
Gin Router → api.CostHandler.Breakdown
    │
    ▼
cost_service.Breakdown → ClickHouse SQL（6维 GROUP BY）
    │
    ▼
JSON 响应 → 前端渲染（Recharts/Table）
```

## 5. 关键设计决策

### 5.1 为什么 ClickHouse？

| 数据特征 | 适配存储 |
|---------|---------|
| Trace 高吞吐写入（10k+ spans/sec） | ClickHouse 写入性能远超 PG |
| 时间窗口聚合查询（成本时间序列） | ClickHouse SummingMergeTree + toStartOfHour |
| 列式存储适合按字段聚合 | ClickHouse 列存优势 |
| 90 天 TTL 自动清理 | ClickHouse TTL 原生支持 |

### 5.2 为什么 PostgreSQL？

| 数据特征 | 适配存储 |
|---------|---------|
| 评估结果（结构化 JSON 字段） | PG JSONB |
| Harness 版本（关系查询 + 事务） | PG 强事务 |
| A/B 测试状态机（强一致性） | PG 关系建模 |
| 模型价格（时间窗口查询） | PG 索引支持 |

### 5.3 为什么 LangGraph 没有用 LangGraph？

LangGraph 内部使用 LangChain Callback 机制（`langchain_core.callbacks.BaseCallbackHandler`），所以 `AgentPulseCallback` 可以直接复用。我们提供 `create_agentpulse_tracer` 作为语义化的便捷包装。

### 5.4 为什么 OpenTelemetry？

- **行业标准**：CNCF 毕业项目，生态成熟
- **避免重复造轮子**：无需自己实现 protobuf 序列化、批量上报、重试
- **可对接现有生态**：用户已有 OTel Collector/Jaeger/Tempo 可直接复用
- **协议稳定**：跨语言 SDK 充足（Python/Go/Java/JS/Rust...）

### 5.5 为什么不用 Tailwind？

前端采用原生 CSS + utility classes：
- 减少依赖体积（Next.js 项目里 Tailwind 占 ~30% 体积）
- 减少配置复杂度
- 便于后续 SSR 性能优化

## 6. 扩展点（Phase 2 预留）

### 6.1 Judge 接口

```go
type Judge interface {
    Name() string
    Evaluate(ctx context.Context, input *JudgeInput) (*JudgeOutput, error)
    Close() error
}
```

可扩展实现：
- AnthropicJudge（Claude 3.5）
- DeepSeekJudge（DeepSeek V3）
- CustomJudge（自研评估 Prompt）
- EnsembleJudge（多 Judge 投票）

注册机制（Phase 2）：通过 `JudgeRegistry` 维护多个 Judge，按配置路由。

### 6.2 Vector Repository

```go
type VectorRepository interface {
    Upsert(ctx, collection, id, embedding, metadata) error
    Query(ctx, collection, embedding, topK) ([]VectorMatch, error)
    Delete(ctx, collection, id) error
}
```

可扩展实现：
- QdrantVectorRepo（Phase 2 备选）
- MilvusVectorRepo（大规模场景）
- LocalEmbeddingVectorRepo（无需外部服务，本地 Embedding）

### 6.3 Pricing 模型

```go
type Pricing struct {
    Model           string
    PromptPrice     float64
    CompletionPrice float64
    Currency        string
    EffectiveAt     time.Time
    ExpiredAt       *time.Time
}
```

支持时间窗口定价（应对 LLM 厂商调价）：
- 多版本同模型不同价
- 旧 Span 用旧价（按 Span 时间戳匹配）
- 新 Span 用新价

### 6.4 Harness Config

YAML-based，可任意扩展字段。当前结构：

```yaml
agent_name: interview-agent
version: 2
system_prompt: |
  ...
tools:
  - web_search
  - code_executor
max_steps: 10
temperature: 0.7
model: gpt-4o
memory_strategy: hybrid
verification:
  enabled: true
  levels: [L1, L2, L3]
reflection:
  enabled: true
  max_retries: 3
```

通过 `metadata` 字段透传任意自定义配置，无需改表结构。

## 7. 性能指标

| 指标 | 当前实现 | 目标 |
|------|---------|------|
| Trace 写入吞吐 | 单节点 ~5k spans/sec | 10k spans/sec（优化后） |
| OTLP 接收延迟 | P95 < 50ms | < 100ms |
| 成本归因查询 | P95 < 500ms (24h 窗口) | < 1s |
| 评估异步延迟 | P95 < 5s（含 LLM 调用） | < 10s |
| Dashboard 首屏 | < 2s（Next.js） | < 3s |

## 8. 安全考虑

> **当前阶段 (v0.1.x) 安全基线**: 已实现 X-AgentPulse-Key 头校验(基于配置文件白名单 + SHA-256 比对),OTLP 接收端加 body size 限制与 Key 校验,DSN 密码不在日志中明文输出,默认密码启动校验。

| 场景 | 当前实现 | 未来改进 |
|------|---------|---------|
| API 鉴权 | `X-AgentPulse-Key` 头(配置文件白名单,SHA-256) | DB/API Key CRUD + JWT/OAuth2 |
| OTLP 接入 | `X-AgentPulse-Key` + `MaxBytesReader`(默认 10MB) + Read/Write Timeout | mTLS / IP 白名单 / 速率限制 |
| 数据库密码 | `MaskedDSN()` 输出脱敏 + Mode=release 强制非默认密码 | Kubernetes Secrets 集成 |
| PII 数据 | input/output preview 截断 + evaluate 路径 regex 脱敏 | LLM 自动检测 + 完整脱敏流水线 |
| 速率限制 | 无 | Token Bucket + 维度限流 (per-key/per-ip) |
| 错误响应 | 内部 err 写日志,客户端只回通用消息 + request_id | 结构化错误码体系 |

## 9. 部署架构

### 9.1 开发环境

```
docker compose (ClickHouse + PG + Chroma)
    │
    ▼
go run cmd/server/main.go  (本地 :8080 + :4318)
    │
    ▼
cd sdk-python && python examples/basic_usage.py
    │
    ▼
npm run dev  (本地 :3000)
```

### 9.2 生产环境（建议）

```
┌─────────────────────────────────────────────┐
│  Kubernetes Cluster                          │
│                                              │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐│
│  │ AgentPulse│   │ ClickHouse│   │PostgreSQL││
│  │ (3 pods)  │   │ (3 shards)│   │ (主从)   ││
│  │ :8080     │   │           │   │          ││
│  │ :4318     │   │           │   │          ││
│  └──────┬────┘   └──────────┘   └──────────┘│
│         │                                    │
│         ▼                                    │
│  ┌──────────┐                                │
│  │ Next.js  │                                │
│  │ (CDN)    │                                │
│  └──────────┘                                │
└─────────────────────────────────────────────┘
```

详见 `deploy/README.md`。

## 10. 测试策略

| 层 | 测试方法 | 工具 |
|----|---------|------|
| Domain | 单元测试（纯逻辑） | Go testing |
| Repository | 集成测试（testcontainers） | testcontainers-go |
| Service | Mock Repository | testify/mock |
| API | HTTP 集成测试 | httptest + Gin |
| SDK (Python) | pytest | pytest |
| E2E | docker compose up + 完整流程 | Manual / Playwright |

当前 MVP 已覆盖：
- ✅ Go 后端 `go build ./...` 通过
- ✅ Python SDK pytest 7/7 通过
- ⏳ 集成测试（Phase 2 待补）
- ⏳ E2E（Phase 2 待补）

## 11. 演进路线

| Phase | 内容 | 状态 |
|-------|------|------|
| Phase 1 | MVP:Trace + Cost + Eval + 基础 Cluster + Harness/ABTest API | ✅ |
| Phase 2 | EvalLoop 迭代工作流 + LLM 标注聚类 + 漂移检测 + 告警 | 待开始 |
| Phase 3 | Go SDK + Trace 火焰图前端 + EvalLoop UI | 待开始 |
| Phase 4 | Kubernetes 部署 + Prometheus 监控 + OpenTelemetry 互操作 | 待开始 |
| Phase 5 | 多租户 + SaaS 化 + 计费 | 待开始 |

> **Phase 1 已实现接口**: Trace (按 trace_id/session/user/agent 查询), Cost (breakdown/timeline/total), Eval (按 agent/scores/手动触发), Cluster (列表/详情/手动聚类), Harness (CRUD/promote/diff), ABTest (CRUD)。
> **OTLP 接收**: 当前仅 HTTP/protobuf (端口 4318),**未实现 gRPC 接收器** (`otlp.grpc_port` 配置项保留为未来 Phase)。

## 12. 参考论文

> 以下 arxiv 编号为占位引用,实际链接可能在未来可访问。AgentPulse 实现以方法论为参考,不保证与具体论文完全一致。

| 参考方法论 | 引用位置 |
|------|---------|
| EvalLoop 迭代工作流 | EvalLoop 模块 (Phase 2) |
| Harness Engineering | Harness 版本化与灰度 |
| 自进化 Harness | Harness (Phase 3 扩展) |
| [OTel GenAI SemConv](https://opentelemetry.io/docs/specs/semconv/gen-ai/) | 语义约定映射(已实现) |
| Trace 完整性审计 | Phase 2 计划 |