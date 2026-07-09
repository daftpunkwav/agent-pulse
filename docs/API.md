# AgentPulse API Reference

> Auto-generated from internal/handlers · v0.1.0

所有 API 前缀：`/api/v1`

公共响应头：`X-Request-ID`

## 1. Trace API

### GET /traces/:trace_id

查询完整 Trace 调用树。

**Path 参数**：
- `trace_id` (string, required) — Trace ID

**响应**：

```json
{
  "trace": {
    "trace_id": "abc123...",
    "session_id": "session-xyz",
    "user_id": "u-001",
    "start_time": "2026-07-09T10:00:00Z",
    "end_time": "2026-07-09T10:00:05Z",
    "depth": 5,
    "all_spans": [
      {
        "id": "span-001",
        "trace_id": "abc123",
        "parent_span_id": "",
        "type": "agent",
        "name": "interview-agent",
        "status": "ok",
        "start_time": "...",
        "latency_ms": 5000,
        "attributes": {}
      }
    ],
    "root": { ... }
  }
}
```

### GET /sessions/:session_id/spans

查询会话下所有 Span。

**Query 参数**：
- `from` (RFC3339, optional)
- `to` (RFC3339, optional)
- `limit` (int, default 100)
- `offset` (int, default 0)
- `status` (string, optional: ok/error/timeout)
- `type` (string, optional: agent/llm/tool/reasoning/evaluation)

### GET /users/:user_id/spans

按用户查询 Span。

### GET /agents/:agent_name/spans

按 Agent 查询 Span。

## 2. Cost API

### GET /cost/breakdown

五/六维成本归因。

**Query 参数**：
- `from` (RFC3339, required)
- `to` (RFC3339, required)
- `dimensions` (string, optional, comma-separated, default: `user,agent,tool,model`)
  - 支持：`user`, `session`, `agent`, `tool`, `reasoning_step`, `model`
- `limit` (int, default 100)

**响应**：

```json
{
  "window": {
    "from": "2026-07-01T00:00:00Z",
    "to": "2026-07-09T00:00:00Z"
  },
  "breakdowns": [
    {
      "dimension": "user",
      "total_usd": 12.34,
      "total_tokens": 567890,
      "items": [
        {
          "key": "user-001",
          "cost_usd": 5.67,
          "tokens": 234567,
          "call_count": 123,
          "rank": 1
        }
      ]
    }
  ]
}
```

### GET /cost/timeline

成本时间序列。

**Query 参数**：
- `from`, `to` (RFC3339, required)
- `granularity` (string: `hour` | `day`, default `hour`)

**响应**：

```json
{
  "window": { ... },
  "granularity": "day",
  "points": [
    {
      "bucket": "2026-07-08T00:00:00Z",
      "cost_usd": 1.23,
      "tokens": 12345,
      "call_count": 56
    }
  ]
}
```

### GET /cost/total

时间窗口内总成本。

**Query 参数**：`from`, `to`

**响应**：

```json
{
  "window": { ... },
  "total_usd": 12.34,
  "total_tokens": 567890
}
```

## 3. Evaluation API

### GET /eval/agents/:agent_name/scores

Agent 五维平均分。

**Query 参数**：`from`, `to`

**响应**：

```json
{
  "agent": "interview-agent",
  "window": { ... },
  "scores": {
    "accuracy": 0.92,
    "completeness": 0.85,
    "tool_selection": 0.78,
    "reasoning_depth": 0.88,
    "helpfulness": 0.90
  }
}
```

### GET /eval/spans/:span_id

根据 Span ID 查询评估。

### GET /eval/agents/:agent_name/list

列出 Agent 所有评估（最多 100 条）。

### POST /eval/spans/:span_id

立即触发同步评估。

**响应**：返回完整 Evaluation 对象。

## 4. Failure Cluster API

### GET /clusters

列出聚类结果。

**Query 参数**：
- `active_only` (bool, default `true`)

**响应**：

```json
{
  "clusters": [
    {
      "id": "uuid",
      "name": "Tool 参数错误",
      "description": "...",
      "trace_count": 145,
      "percentage": 0.35,
      "common_pattern": "[...]",
      "suggestion": "在 Prompt 中加入工具参数示例",
      "is_active": true,
      "created_at": "..."
    }
  ],
  "count": 5
}
```

### GET /clusters/:cluster_id

查询单个聚类。

### POST /clusters/run

手动触发聚类分析。

**Query 参数**：`from`, `to`

## 5. Harness API

### GET /harness/:agent_name/versions

列出 Agent 所有 Harness 版本。

### GET /harness/:agent_name/versions/:version

查询指定版本详情。

### POST /harness/:agent_name/versions

创建新版本。

**Body**：

```json
{
  "config_yaml": "agent_name: interview-agent\n...",
  "notes": "优化 prompt",
  "created_by": "user@example.com"
}
```

**响应**：返回新版本对象（含自动分配的 version）。

### POST /harness/:agent_name/versions/:version/promote

提升版本到 production（自动降级旧 production 版本）。

### GET /harness/:agent_name/diff/:v1/:v2

对比两个版本的 Config Hash。

**响应**：

```json
{
  "v1": { ... },
  "v2": { ... },
  "same": false
}
```

## 6. A/B Test API

### GET /abtests

列出所有 A/B 测试（按创建时间倒序，最多 100）。

### GET /abtests/:test_id

查询单个 A/B 测试。

### POST /abtests

创建 A/B 测试。

**Body**：

```json
{
  "name": "测试新 Prompt",
  "agent_name": "interview-agent",
  "control_version": 1,
  "treatment_version": 2,
  "traffic_percent": 20
}
```

## 7. OTLP 接收

### POST /v1/traces  (端口 4318)

OpenTelemetry OTLP/HTTP 接收端点。

- Content-Type: `application/x-protobuf`
- 协议: [OTLP Trace Export](https://opentelemetry.io/docs/specs/otlp/#trace-export)
- 兼容: 所有 OpenTelemetry 官方 SDK
- 鉴权: `X-AgentPulse-Key` 头(必须,与 API Key 同一组)
- Body 限制: 默认 10MB,可通过 `AGENTPULSE_OTLP_MAX_BODY_SIZE` 调整
- 超时: ReadHeader 5s / Read 30s / Write 30s

支持 OTel GenAI 语义约定属性 + AgentPulse 自定义属性(`ap.*`)。

> **gRPC 接收 (端口 4317)**: 当前版本未实现,配置项保留为 Phase 2 扩展。

## 8. 健康检查

### GET /healthz  (端口 8080)

返回服务状态。

```json
{
  "status": "ok",
  "version": "0.1.0",
  "timestamp": "2026-07-09T10:00:00Z"
}
```

### GET /readyz

就绪探针：调用依赖 `HealthCheck()`（ClickHouse + PostgreSQL），全部可用时返回 `200`。

```json
{
  "status": "ready",
  "version": "0.1.0",
  "timestamp": "2026-07-09T10:00:00Z"
}
```

依赖不可用时返回 `503`：

```json
{
  "status": "not_ready",
  "error": "clickhouse: connection refused",
  "timestamp": "2026-07-09T10:00:00Z"
}
```

> `/healthz` 仅表示进程存活；`/readyz` 表示可接受流量。K8s 应分别配置 liveness 与 readiness。

## 9. 错误响应

统一格式：

```json
{
  "error": "internal_error",
  "message": "an internal error occurred, please retry with the request_id for support",
  "request_id": "uuid"
}
```

错误码：
- `400 bad_request` — 参数错误
- `401 unauthorized` — 鉴权失败(MVP 已启用,`X-AgentPulse-Key` 缺失或无效)
- `404 not_found` — 资源不存在
- `405 method_not_allowed` — 方法不允许
- `413 payload_too_large` — Body 超过限制
- `500 internal_error` — 服务内部错误(详细错误仅写日志,客户端只见通用消息 + request_id)
- `503 service_unavailable` — 依赖不可用(如 ClickHouse/Postgres 离线)

## 10. 限流（Phase 2）

预留维度：
- 按 API Key 限流（per-minute）
- 按 IP 限流
- 按 Agent 限流

Token Bucket 算法。