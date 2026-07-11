# AgentPulse PRD — 产品需求文档

## 1. 产品概述

### 1.1 产品定义

AgentPulse 是一个面向 LLM Agent 的生产运维（AgentOps）平台。它为运行中的 Agent 系统提供：

1. **可观测性**——基于 OpenTelemetry 的全链路 Trace 采集
2. **可量化**——五维成本归因 + 五维质量评估
3. **可控制**——Harness 版本化 + 灰度发布 + 一键回滚
4. **可改进**——失败模式聚类 + EvalLoop 迭代工作流

类比 DevOps（Development + Operations）解决了代码开发与部署的协同问题，AgentOps（Agent + Operations）解决 Agent 开发与运维的协同问题。

### 1.2 目标用户

| 用户类型 | 痛点 | AgentPulse 如何解决 |
|---------|------|-------------------|
| **Agent 开发者** | Agent 上线后出问题找不到原因 | Trace 全链路 + 自动聚类失败模式 |
| **AI 产品经理** | 不知道线上效果如何，不知道改什么 | 五维评分 + EvalLoop 迭代闭环 |
| **运维/平台团队** | Token 费用失控，不知钱花在哪 | 五维成本归因 + 漂移检测 |
| **CTO/技术决策者** | 想做 A/B 测试没有基础设施 | Harness 灰度 + 自动对比 |

### 1.3 核心价值主张

> **让 Agent 上线后不再黑盒——可观测、可量化、可控、可改进。**

### 1.4 与市面产品的差异化

| 能力 | Langfuse | LangSmith | Helicone | **AgentPulse** |
|------|----------|-----------|----------|---------------|
| Trace 采集 | ✅ | ✅ | ✅ | ✅ |
| 五维成本归因 | 部分 | 部分 | 部分 | ✅ 完整 |
| LLM-as-Judge | ✅ | ✅ | ❌ | ✅ |
| 失败模式自动聚类 | ❌ | ❌ | ❌ | ✅ |
| EvalLoop 迭代工作流 | ❌ | ❌ | ❌ | ✅ |
| Harness 版本化与灰度 | ❌ | 部分 | ❌ | ✅ |
| 多语言 SDK | ✅ | ✅ | ✅ | ✅ |

主攻 **成本归因 + 失败聚类 + EvalLoop 迭代** 三个差异化能力。

---

## 2. 核心功能

### 2.1 Trace 全链路采集

#### 2.1.1 Trace Span 模型

```
Session (会话级)
├── Span: AgentCall (Agent 调用)
│   ├── Span: LLMCall (LLM 调用)
│   │   ├── model
│   │   ├── prompt_tokens
│   │   ├── completion_tokens
│   │   ├── cost_usd
│   │   ├── latency_ms
│   │   └── finish_reason
│   ├── Span: ToolCall (工具调用)
│   │   ├── tool_name
│   │   ├── args
│   │   ├── result_preview
│   │   └── latency_ms
│   ├── Span: ReasoningStep (推理步骤)
│   │   ├── step_index
│   │   ├── thought
│   │   └── action
│   └── Span: Evaluation (评估)
│       ├── accuracy
│       ├── completeness
│       ├── tool_selection
│       ├── reasoning_depth
│       ├── helpfulness
│       └── judge_model
```

#### 2.1.2 采集协议

- **OTLP gRPC**（端口 4317）：高性能，适合服务端 Agent
- **OTLP HTTP**（端口 4318）：兼容性好，适合边缘节点
- **SDK 直推**：Python/Go SDK 装饰 Agent 自动上报
- **批量异步**：SDK 端攒批异步上报，不影响业务延迟

#### 2.1.3 存储设计

| 数据 | 存储 | 索引 |
|------|------|------|
| Span 原始数据 | ClickHouse（OLAP） | 按时间 + session_id + user_id |
| 会话/Agent 元数据 | PostgreSQL | 按 ID |
| Trace 嵌入向量 | Chroma（可选） | 用于相似度检索 |
| 失败聚类结果 | PostgreSQL | 按 cluster_id |

**ClickHouse 表结构**：

```sql
CREATE TABLE agent_spans (
    timestamp       DateTime64(9),
    trace_id        String,
    span_id         String,
    parent_span_id  String,
    session_id      String,
    user_id         String,
    agent_name      String,
    span_type       Enum8('llm'=1, 'tool'=2, 'reasoning'=3, 'evaluation'=4, 'agent'=5),
    model           String,
    prompt_tokens   UInt32,
    completion_tokens UInt32,
    cost_usd        Decimal(10, 6),
    latency_ms      UInt32,
    tool_name       String,
    reasoning_step  UInt16,
    attributes      JSON,
    status          Enum8('ok'=1, 'error'=2, 'timeout'=3)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (user_id, session_id, timestamp);
```

### 2.2 五维成本归因

#### 2.2.1 五个归因维度

| 维度 | 拆分依据 | 业务问题 |
|------|---------|---------|
| **用户** | `user_id` | 谁花得最多？是否要限流？ |
| **会话** | `session_id` | 哪些会话异常贵？ |
| **Agent** | `agent_name` | 哪个 Agent 烧钱最快？ |
| **工具** | `tool_name` | web_search 还是 code_exec？ |
| **推理步骤** | `reasoning_step` | 反思循环是不是太贵？ |

#### 2.2.2 价格表管理

维护各模型的实时价格表，自动计算成本：

```yaml
# configs/model_pricing.yaml
gpt-4o:
  prompt: 0.0025      # USD per 1k tokens
  completion: 0.01
gpt-4o-mini:
  prompt: 0.00015
  completion: 0.0006
claude-3.5-sonnet:
  prompt: 0.003
  completion: 0.015
deepseek-v3:
  prompt: 0.00014
  completion: 0.00028
```

支持价格版本化，自动按 Span 时间戳匹配生效价格。

#### 2.2.3 归因 API

```http
GET /api/v1/cost/breakdown
  ?from=2026-07-01&to=2026-07-09
  &dimensions=user,agent,tool
  &user_id=user-123

Response:
{
  "total_cost_usd": 1234.56,
  "total_tokens": 5678901,
  "by_user": [
    {"user_id": "u-001", "cost_usd": 234.56, "tokens": 1234567, "rank": 1},
    ...
  ],
  "by_agent": [...],
  "by_tool": [...]
}
```

#### 2.2.4 桑基图可视化

成本在不同维度间的流向：

```
[用户 A] ─┬─→ [Agent X] ─┬─→ [Tool: search] ─→ ¥120
          │              └─→ [Tool: code]    ─→ ¥80
          │
[用户 B] ─┴─→ [Agent Y] ────→ [Tool: llm]    ─→ ¥50
```

前端用 D3.js Sankey 展示资金流向。

### 2.3 五维在线评估

#### 2.3.1 五维评分体系

| 维度 | 含义 | 评分方法 |
|------|------|---------|
| **Accuracy** | 事实是否正确 | LLM 对比答案与 ground truth 或参考来源 |
| **Completeness** | 是否覆盖问题所有要点 | LLM 列举问题子点，逐项检查 |
| **Tool Selection** | 工具选择是否合理 | LLM 判断选的工具能否解决问题 |
| **Reasoning Depth** | 推理链是否充分 | LLM 评估 Thought-Action-Observation 的逻辑严密性 |
| **Helpfulness** | 对用户是否有实际帮助 | LLM 模拟用户，判断答案价值 |

每个维度 0-1 浮点分，加权平均得 Overall 分。

#### 2.3.2 评估触发

| 触发方式 | 场景 |
|---------|------|
| **同步评估** | 每次 Trace 上报后立即评估（高价值场景） |
| **采样评估** | 按比例采样评估（如 10%） |
| **离线评估** | 人工标注 + 批量回放评估 |
| **主动评估** | 用户反馈"答错"时触发深度评估 |

#### 2.3.3 LLM-as-Judge 实现

```go
type Judge struct {
    model    string
    client   *openai.Client
    template *JudgePromptTemplate
}

func (j *Judge) Evaluate(ctx context.Context, span *Span) (*Evaluation, error) {
    prompt := j.template.Render(map[string]any{
        "input":          span.Input,
        "output":         span.Output,
        "trace":          span.ReasoningTrace,
        "tool_calls":     span.ToolCalls,
    })
    
    resp := j.client.Chat(ctx, j.model, prompt, ResponseFormatJSON)
    
    var eval Evaluation
    json.Unmarshal(resp, &eval)
    
    return &eval, nil
}

type Evaluation struct {
    Accuracy       float64 `json:"accuracy"`
    Completeness   float64 `json:"completeness"`
    ToolSelection  float64 `json:"tool_selection"`
    ReasoningDepth float64 `json:"reasoning_depth"`
    Helpfulness    float64 `json:"helpfulness"`
    Overall        float64 `json:"overall"`
    Rationale      string  `json:"rationale"`
    JudgeModel     string  `json:"judge_model"`
}
```

### 2.4 失败模式自动聚类

#### 2.4.1 聚类流程

```
失败 Trace 收集
    ↓
失败原因 LLM 标注（每条 Trace 一个标签）
    ↓
向量化嵌入（Chroma）
    ↓
DBSCAN / K-Means 聚类
    ↓
每个 Cluster 命名 + 改进建议
    ↓
Dashboard 展示
```

#### 2.4.2 输出示例

```json
{
  "clusters": [
    {
      "cluster_id": "C001",
      "name": "Tool 参数格式错误",
      "count": 145,
      "percentage": 0.35,
      "example_traces": ["trace-id-001", "trace-id-002", ...],
      "common_pattern": "web_search 调用时 query 参数未加引号",
      "suggestion": "在 Prompt 中加入工具参数示例"
    },
    {
      "cluster_id": "C002",
      "name": "推理链断裂",
      "count": 89,
      "percentage": 0.21,
      "common_pattern": "Step 3 后 Agent 失去上下文",
      "suggestion": "增加上下文压缩策略或减少最大步数"
    }
  ]
}
```

#### 2.4.3 触发方式

- **定期任务**：每 6 小时聚类一次最近 24h 的失败 Trace
- **阈值触发**：失败率超过 20% 时立即聚类
- **手动触发**：用户在前端点击"立即分析"

### 2.5 Harness 版本化与灰度

#### 2.5.1 Harness Config 模型

```yaml
# harness-config-v2.yaml
version: 2
agent_name: InterviewAgent
system_prompt: |
  你是一位专业的面试官...
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

#### 2.5.2 版本管理

- Config 文件 git 化（提交到独立 `harness-configs/` 目录）
- 每次变更创建新版本（v1, v2, v3...）
- 版本对比 Diff 可视化
- 任一版本可标记为 `production` / `archived` / `canary`

#### 2.5.3 灰度发布

```http
POST /api/v1/harness/canary
{
  "agent_name": "InterviewAgent",
  "from_version": 2,
  "to_version": 3,
  "traffic_percent": 10,  // 10% 流量切到 v3
  "duration_minutes": 60,
  "auto_promote": true,
  "auto_rollback": true,
  "rollback_threshold": {
    "success_rate_drop": 0.05,
    "cost_increase": 0.3
  }
}
```

#### 2.5.4 自动回滚条件

| 指标 | 回滚阈值 |
|------|---------|
| 成功率下降 | > 5% |
| 成本增加 | > 30% |
| P99 延迟增加 | > 50% |
| 评估分下降 | > 0.1 |

### 2.6 EvalLoop 迭代工作流

参考 arXiv:2607.05638 EvalLoop 方法论：

```
┌─────────────────────────────────────────────────┐
│  EvalLoop Iteration                             │
│                                                  │
│  1. 维度化打分 (Multi-dimensional Scoring)        │
│     ↓                                            │
│  2. 维度诊断 (Dimensional Diagnosis)             │
│     识别最弱维度（如 Completeness 只有 0.6）       │
│     ↓                                            │
│  3. 假设失败原因 (Failure Hypothesis)             │
│     "Completeness 低是因为没要求 Agent 列点"       │
│     ↓                                            │
│  4. 单变量修改 (Single-variable Modification)    │
│     只改 Prompt 加一句"请用 bullet points 列出"  │
│     ↓                                            │
│  5. A/B 验证 (A/B Validation)                   │
│     新版 vs 旧版，5% 流量，统计显著性             │
│     ↓                                            │
│  6. 应用/回滚 (Apply/Rollback)                  │
│     通过 → 全量；失败 → 回滚                      │
└─────────────────────────────────────────────────┘
```

**核心原则**：每次只改一个变量，看因果，避免"瞎调运气"。

### 2.7 A/B 测试

| 能力 | 说明 |
|------|------|
| 同任务路由不同 Harness | 按 session_id hash 路由，保证同会话走同配置 |
| 自动指标对比 | 成功率、成本、延迟、评估分四维度对比 |
| 统计显著性检验 | 自动计算 p-value，给出置信度 |
| 自动获胜方判定 | 满足显著性 + 阈值时自动全量推广获胜方 |

### 2.8 漂移检测

| 检测项 | 算法 | 告警 |
|--------|------|------|
| 成功率下降 | 移动平均 vs 历史基线 | 偏离 > 阈值触发 |
| 成本上升 | 同上 | 同上 |
| P99 延迟恶化 | 同上 | 同上 |
| 评估分下降 | 同上 | 同上 |
| 工具调用模式变化 | 分布 KL 散度 | 分布偏移 > 阈值 |

---

## 3. 系统架构

### 3.1 整体架构

```
┌──────────────────────────────────────────────────────────────┐
│                Web Frontend (Next.js 15)                      │
│  Trace Viewer │ Cost Dashboard │ Eval Dashboard │ Harness Mgr │
└─────────────────────────┬────────────────────────────────────┘
                          │ REST + SSE
┌─────────────────────────┴────────────────────────────────────┐
│                  API Gateway (Gin)                            │
│  /api/v1/traces /cost /eval /cluster /harness /ab            │
└─────────────────────────┬────────────────────────────────────┘
                          │
┌─────────────────────────┴────────────────────────────────────┐
│                     Service Layer                             │
│                                                               │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────────────┐ │
│  │ Trace        │ │ Cost         │ │ Eval (LLM-as-Judge)  │ │
│  │ Service      │ │ Service      │ │ Service              │ │
│  └──────────────┘ └──────────────┘ └──────────────────────┘ │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────────────┐ │
│  │ Cluster      │ │ Harness      │ │ AB Test              │ │
│  │ Service      │ │ Service      │ │ Service              │ │
│  └──────────────┘ └──────────────┘ └──────────────────────┘ │
└─────────────────────────┬────────────────────────────────────┘
                          │
┌─────────────────────────┴────────────────────────────────────┐
│                    Storage Layer                              │
│                                                               │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────────────┐ │
│  │ ClickHouse   │ │ PostgreSQL   │ │ Chroma (optional)    │ │
│  │ (Trace/Span) │ │ (Metadata)   │ │ (Cluster vectors)    │ │
│  └──────────────┘ └──────────────┘ └──────────────────────┘ │
└──────────────────────────────────────────────────────────────┘

                          ▲
                          │ OTLP / SDK 直推
                          │
┌─────────────────────────┴────────────────────────────────────┐
│                   Agent Ecosystem                             │
│                                                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐│
│  │  sdk-go      │  │  sdk-python  │  │ OTLP direct (any    ││
│  │  (Decorator) │  │  (Decorator) │  │ language)           ││
│  └──────────────┘  └──────────────┘  └──────────────────────┘│
│                                                               │
│  用户的 Agent 系统（InterviewOS / RepoPilot / AgentPrism /...）│
└──────────────────────────────────────────────────────────────┘
```

### 3.2 数据流

**写入流**：

```
Agent 业务调用
    ↓
SDK 装饰器拦截 → 包装 Span
    ↓
批量异步发送（OTLP）
    ↓
API Gateway 接收
    ↓
Trace Service → ClickHouse 写入
    ↓
Eval Service（同步/采样/异步）→ PostgreSQL
    ↓
告警/通知（如有异常）
```

**查询流**：

```
Web Dashboard 查询
    ↓
API Gateway
    ↓
Service Layer 组合查询（CH + PG + Chroma）
    ↓
返回 JSON + 缓存
    ↓
前端渲染（火焰图 / 桑基图 / 雷达图）
```

### 3.3 关键技术决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 后端语言 | Go | 高并发，Trace 场景吞吐优先 |
| Trace 存储 | ClickHouse | OLAP 引擎，写入与聚合查询性能远超 PG |
| 元数据存储 | PostgreSQL | 强事务，会话/Harness/AB 测试等元数据需要关系建模 |
| OTLP 接收 | 自实现 OTLP receiver | 不引入完整 OTel Collector，减少依赖 |
| LLM Judge | 多模型支持 | BYOK，避免锁定 |
| 失败聚类 | LLM 标注 + 向量聚类 | 单纯向量聚类可解释性差 |
| 前端可视化 | Recharts + D3.js | Recharts 简单图表，D3 复杂可视化 |
| 部署 | Docker Compose + 可选 K8s | 开发友好，生产可扩展 |

---

## 4. 数据模型

### 4.1 ClickHouse Span 表（见 2.1.3）

### 4.2 PostgreSQL 表

```sql
-- 会话表
CREATE TABLE sessions (
    id            UUID PRIMARY KEY,
    user_id       VARCHAR(64) NOT NULL,
    agent_name    VARCHAR(64) NOT NULL,
    started_at    TIMESTAMPTZ NOT NULL,
    ended_at      TIMESTAMPTZ,
    total_cost    DECIMAL(10,6),
    total_tokens  INTEGER,
    status        VARCHAR(16) DEFAULT 'running',
    metadata      JSONB,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

-- 评估表
CREATE TABLE evaluations (
    id                UUID PRIMARY KEY,
    span_id           VARCHAR(64) NOT NULL,
    session_id        UUID NOT NULL,
    accuracy          DECIMAL(4,3),
    completeness      DECIMAL(4,3),
    tool_selection    DECIMAL(4,3),
    reasoning_depth   DECIMAL(4,3),
    helpfulness       DECIMAL(4,3),
    overall           DECIMAL(4,3),
    rationale         TEXT,
    judge_model       VARCHAR(64),
    created_at        TIMESTAMPTZ DEFAULT NOW()
);

-- 失败聚类表
CREATE TABLE failure_clusters (
    id              UUID PRIMARY KEY,
    cluster_name    VARCHAR(128) NOT NULL,
    description     TEXT,
    trace_count     INTEGER,
    percentage      DECIMAL(5,4),
    common_pattern  TEXT,
    suggestion      TEXT,
    example_traces  JSONB,
    metadata        JSONB,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Harness 版本表
CREATE TABLE harness_configs (
    id              UUID PRIMARY KEY,
    agent_name      VARCHAR(64) NOT NULL,
    version         INTEGER NOT NULL,
    config_yaml     TEXT NOT NULL,
    config_hash     VARCHAR(64) NOT NULL,
    status          VARCHAR(16) DEFAULT 'archived',  -- production | canary | archived
    traffic_percent INTEGER DEFAULT 0,
    created_by      VARCHAR(64),
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    promoted_at     TIMESTAMPTZ,
    UNIQUE(agent_name, version)
);

-- A/B 测试表
CREATE TABLE ab_tests (
    id              UUID PRIMARY KEY,
    name            VARCHAR(128) NOT NULL,
    agent_name      VARCHAR(64) NOT NULL,
    control_version INTEGER NOT NULL,
    treatment_version INTEGER NOT NULL,
    traffic_percent INTEGER NOT NULL,
    status          VARCHAR(16) DEFAULT 'running',  -- running | completed | aborted
    started_at      TIMESTAMPTZ,
    ended_at        TIMESTAMPTZ,
    result          JSONB,  -- 包含胜出方、统计显著性、指标对比
    created_at      TIMESTAMPTZ DEFAULT NOW()
);
```

### 4.3 Chroma Collection

```python
# 失败 Trace 嵌入（用于聚类）
collection.add(
    embeddings=[trace_embedding],
    documents=[trace_text],
    metadatas=[{"error_type": "...", "cluster_id": "..."}],
    ids=[trace_id]
)
```

---

## 5. API 设计

> **图例**: ✅ MVP 已实现 · ⏳ Phase 2/3 计划 · ❌ 已废弃
> 完整已实现接口以 `backend/internal/api/router.go` 为准,见 [docs/API.md](API.md)。

### 5.1 Trace API

```http
POST /api/v1/traces              # ❌ SDK 上报使用 OTLP 端点,非本路径
GET  /api/v1/traces              # ❌ 当前仅支持按 trace_id/session/user/agent 维度查询
GET  /api/v1/traces/{trace_id}   # ✅ 获取 Trace 详情(含 Span 树)
GET  /api/v1/traces/{trace_id}/replay  # ⏳ Phase 2
GET  /api/v1/sessions/{session_id}/spans  # ✅ 替代列表查询
GET  /api/v1/users/{user_id}/spans        # ✅
GET  /api/v1/agents/{agent_name}/spans    # ✅
```

### 5.2 Cost API

```http
GET /api/v1/cost/breakdown       # ✅ 五维归因
GET /api/v1/cost/timeline        # ✅ 时间序列
GET /api/v1/cost/total           # ✅ 总成本
GET /api/v1/cost/sankey          # ⏳ Phase 2(由 breakdown + timeline 派生)
GET /api/v1/cost/pricing         # ⏳ Phase 2(当前由 DB 维护,无 API 暴露)
PUT /api/v1/cost/pricing         # ⏳ Phase 2
```

### 5.3 Eval API

```http
GET  /api/v1/eval/spans/{span_id}                  # ✅ 单 Span 评估结果
GET  /api/v1/eval/agents/{agent_name}/scores       # ✅ Agent 维度平均分
GET  /api/v1/eval/agents/{agent_name}/list         # ✅ Agent 评估列表
POST /api/v1/eval/spans/{span_id}                  # ✅ 手动触发评估
GET  /api/v1/eval/dashboard                        # ⏳ Phase 2
GET  /api/v1/eval/radar                            # ⏳ Phase 2(由 scores 派生)
```

### 5.4 Cluster API

```http
GET  /api/v1/clusters                              # ✅ 列出聚类(默认最近)
GET  /api/v1/clusters/{cluster_id}                 # ✅ 聚类详情
POST /api/v1/clusters/run                          # ✅ 手动触发聚类
GET  /api/v1/clusters/recent                       # ⏳ Phase 2(由 list 替代)
GET  /api/v1/clusters/{cluster_id}/traces          # ⏳ Phase 2
```

### 5.5 Harness API

```http
GET    /api/v1/harness/{agent_name}/versions                  # ✅ 版本列表
GET    /api/v1/harness/{agent_name}/versions/{version}       # ✅ 版本详情
POST   /api/v1/harness/{agent_name}/versions                  # ✅ 创建新版本
POST   /api/v1/harness/{agent_name}/versions/{version}/promote  # ✅ 设为生产
GET    /api/v1/harness/{agent_name}/diff/{v1}/{v2}            # ✅ 版本对比
POST   /api/v1/harness/{agent_name}/canary                    # ⏳ Phase 2(可由 promote + traffic_percent 组合实现)
POST   /api/v1/harness/{agent_name}/rollback                  # ⏳ Phase 2(可由 promote 旧版本实现)
```

### 5.6 AB Test API

```http
POST /api/v1/abtests                # ✅ 创建 A/B 测试
GET  /api/v1/abtests                # ✅ 列出测试
GET  /api/v1/abtests/{test_id}      # ✅ 测试详情
GET  /api/v1/abtests/{id}/report    # ⏳ Phase 2
POST /api/v1/abtests/{id}/abort     # ⏳ Phase 2
```

### 5.7 OTLP 接收

```
gRPC: 0.0.0.0:4317  # ✅ OpenTelemetry OTLP/gRPC
HTTP: 0.0.0.0:4318  # ✅ OpenTelemetry OTLP/HTTP
```

**当前 OTLP 接收器要求**:
- `X-AgentPulse-Key` 头必须（当 `AGENTPULSE_AUTH_OTLP_REQUIRE_KEY=true` 时）
- 单次请求 body 上限 10MB（可通过 `AGENTPULSE_OTLP_MAX_BODY_SIZE` 调整）
- 异步写入 ClickHouse，返回 `ExportTraceServicePartialSuccess` 反馈

---

## 6. SDK 设计

### 6.1 Python SDK

#### 安装

```bash
pip install agentpulse
# 或带框架适配器
pip install agentpulse[langchain,langgraph,autogen]
```

**要求**: Python >= 3.11

#### 基础用法

```python
from agentpulse import init, session, trace, observe

# 1. 初始化（应用启动时一次）
init(
    api_key="ap-your-key",
    endpoint="http://localhost:8080",
    service_name="my-agent-app",
)

# 2. 装饰器方式
@observe(agent_name="interview-agent")
def answer_question(question: str) -> str:
    return llm.invoke(question)

# 3. Context Manager 方式
def run_agent(question: str):
    with session(user_id="u-123") as s:
        with trace("llm_call", model="gpt-4o") as t:
            t.set_input(question)
            result = llm.invoke(question)
            t.set_output(result)
            t.set_tokens(prompt=100, completion=50)
            t.set_cost(0.005)
        return result

# 4. LangChain 集成
from agentpulse.integrations.langchain import AgentPulseCallback

callback = AgentPulseCallback(api_key="ap-your-key")
chain.invoke(input, config={"callbacks": [callback]})
```

#### 核心 API

```python
class AgentPulseClient:
    def init(
        api_key: str,
        endpoint: str,
        service_name: str = "agent-app",
        environment: str = "production",
        sample_rate: float = 1.0,  # 采样率
        flush_interval: float = 5.0,  # 批量上报间隔（秒）
    ): ...
    
    def session(self, user_id: str = None, metadata: dict = None) -> Session: ...
    def trace(self, name: str, **attrs) -> Span: ...
    def observe(self, agent_name: str = None, **attrs) -> Callable: ...  # 装饰器
    
    def flush(self): ...  # 强制刷新缓冲区
    def shutdown(self): ...  # 关闭客户端
```

### 6.2 Go SDK（Phase 3 计划中）

> **状态**: `sdk-go/` 目录当前为空（Phase 3 占位）。以下 API 为设计草案，可能与最终实现有差异。

#### 安装

```bash
go get github.com/agentpulse/sdk-go
```

#### 基础用法

```go
import "github.com/agentpulse/sdk-go"

func main() {
    ap.Init(ap.Config{
        APIKey:      "ap-your-key",
        Endpoint:    "http://localhost:8080",
        ServiceName: "my-agent-app",
    })
    defer ap.Shutdown()
    
    // 装饰器方式
    agent := ap.Wrap(myAgent, ap.WithAgentName("interview-agent"))
    
    // 手动方式
    ctx := ap.ContextWithSession(context.Background(), "user-123")
    result, _ := agent.Run(ctx, "...")
}
```

### 6.3 自动 Span 类型

| Span 类型 | 自动捕获 | 手动设置 |
|-----------|---------|---------|
| LLMCall | model 名称、延迟 | tokens、cost |
| ToolCall | 工具名、参数、延迟、结果预览 | 错误信息 |
| ReasoningStep | step_index | thought、action |
| Evaluation | judge_model | 五维分数 |

---

## 7. 前端可视化

### 7.1 Trace Viewer（火焰图/甘特图）

展示单次调用的完整 Span 树，按时间排列，可点击查看 Span 详情。

### 7.2 Cost Dashboard

- **总览卡片**：今日/本周/本月总成本、token、调用次数
- **时间序列图**：成本随时间变化
- **五维归因表**：每个维度的 Top N
- **桑基图**：维度间资金流向

### 7.3 Eval Dashboard

- **五维雷达图**：所有 Agent 的评估分对比
- **维度时间序列**：每个维度得分随时间变化
- **Top 问题列表**：低分样本聚类
- **A/B 对比**：实验组 vs 对照组

### 7.4 Harness Manager

- **版本列表**：所有 Agent 的所有版本
- **版本对比**：Diff 视图
- **灰度控制**：滑块调整流量比例
- **回滚按钮**：一键回滚

### 7.5 失败聚类 Dashboard

- **聚类列表**：按 Trace 数排序
- **聚类详情**：样本 Trace、改进建议
- **趋势图**：聚类随时间变化

---

## 8. 非功能性需求

| 需求 | 指标 |
|------|------|
| Trace 写入吞吐 | 单节点 ≥ 10k spans/sec |
| 评估延迟 | 同步评估 ≤ 3s，采样评估 ≤ 10s |
| Dashboard 查询响应 | P95 ≤ 1s |
| 数据保留 | Trace 90 天，评估 1 年 |
| 部署资源（最小） | 2 CPU + 4GB RAM + 100GB 磁盘 |
| 多租户 | 支持多 API Key 数据隔离 |

---

## 9. MVP 范围

### Phase 1：Trace + 成本归因 + 在线评估（核心三模块）— 2 周

- [x] 项目骨架（Go + Gin + ClickHouse + PG + Chroma）
- [ ] OTLP HTTP 接收 + ClickHouse 写入
- [ ] Python SDK 基础版本（装饰器 + 自动上报）
- [ ] Go SDK 基础版本
- [ ] 五维成本归因 API + 桑基图
- [ ] LLM-as-Judge 五维评分 + 雷达图
- [ ] Web Dashboard 基础框架

### Phase 2：失败聚类 + Harness 版本化 — 2 周

- [ ] 失败模式自动聚类（LLM 标注 + 向量聚类）
- [ ] Harness Config git 化版本管理
- [ ] 灰度发布 + 自动回滚
- [ ] Web Harness Manager UI

### Phase 3：EvalLoop 迭代 + A/B 测试 — 2 周

- [ ] EvalLoop 工作流引擎
- [ ] A/B 测试 + 统计显著性检验
- [ ] 自动获胜方判定与全量推广

### Phase 4：漂移检测 + SDK 完善 — 1 周

- [ ] 漂移检测算法 + 告警
- [ ] LangChain / LangGraph / AutoGen 适配器
- [ ] 完整文档 + 教程

---

## 10. 关键设计决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 后端语言 | Go | 高并发，Trace 场景吞吐优先 |
| Trace 存储 | ClickHouse | OLAP，Trace 写入与聚合性能最佳 |
| 元数据存储 | PostgreSQL | 强事务，关系查询 |
| 协议 | OpenTelemetry OTLP | 行业标准，生态兼容 |
| LLM Judge | 多模型 BYOK | 不锁定，单点故障降级 |
| 失败聚类 | LLM 标注 + 向量聚类 | 可解释性 + 自动化 |
| 前端 | Next.js 15 + Recharts + D3.js | 与已有项目一致 |
| 不使用 monorepo | 单仓多目录 | 与 InterviewOS/RepoPilot/AgentPrism 风格一致 |
| Python SDK venv 隔离 | sdk-python/.venv | 不污染全局环境 |
| Go 标准 layout | cmd + internal + pkg | Go 社区最佳实践 |

---

## 11. 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| LLM Judge 评估成本高 | 用户不愿开启全量评估 | 支持采样评估 + 离线回放评估 |
| 失败聚类 LLM 标注成本 | 同上 | 采样标注 + 复用已有标注 |
| ClickHouse 单点故障 | 数据丢失 | 配置副本 + 定期备份 |
| OTLP 与业务 Trace 冲突 | SDK 兼容性 | 命名空间隔离 + 配置项 |
| Harness 灰度自动回滚误判 | 流量抖动导致误回滚 | 滑动窗口确认 + 阈值放宽 |
| Python SDK 性能开销 | 业务延迟增加 | 批量异步上报 + 采样率可配 |
| 用户 Agent 接入成本高 | 推广受阻 | 提供 LangChain/LangGraph/AutoGen 一行接入 |