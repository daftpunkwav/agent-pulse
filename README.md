# AgentPulse

> The heartbeat monitor for AI Agents in production.

**AgentPulse** 是一个面向 LLM Agent 的运维（AgentOps）平台。它为生产环境中的 Agent 接入"心电监护仪"——自动采集调用 Trace、归因成本、在线评估质量、聚类失败模式，让 Agent 上线后**可观测、可量化、可控**。

类比 DevOps 管代码部署，AgentPulse 管 Agent 部署后的运行健康。

---

## 为什么需要 AgentPulse

Agent Demo 跑通很容易，但上线后立刻面对一堆新问题：

| 痛点 | 后果 |
|------|------|
| 不知道花了多少钱 | 月底账单爆炸，不知道哪个用户/任务烧的 |
| 不知道错在哪里 | 用户投诉答错，几万条 Trace 找不到问题 |
| 不知道线上效果 | Demo 100 个都对，线上 1 万个效果如何无量化 |
| 不知道何时回归 | 改了一个字，成功率掉 5% 没人发现 |
| 不知道如何回滚 | Harness 升级后变差，没有版本控制 |
| 不知道如何 AB | 想测新 Prompt vs 旧 Prompt，只能凭感觉切流量 |

AgentPulse 给每个 Agent 接上"心电监护仪"——实时监控、量化评估、可控变更。

---

## 核心功能

### 1. Trace 全链路采集

通过 OpenTelemetry 协议自动采集 Agent 调用的完整链路：

```
Session (会话级)
├── Span: AgentCall (Agent 调用)
│   ├── Span: LLMCall (LLM 调用，含 token/成本/延迟)
│   ├── Span: ToolCall (工具调用，含参数/结果/耗时)
│   ├── Span: ReasoningStep (推理步骤，含 Thought/Action)
│   └── Span: Evaluation (评估，含五维评分)
```

### 2. 五维成本归因

把 token 消耗精确拆解到五个维度：

- **用户维度**：谁花得最多
- **会话维度**：哪些会话最贵
- **Agent 维度**：哪个 Agent 烧钱最快
- **工具维度**：哪些工具调用最频繁
- **推理步骤维度**：哪一步反思循环最贵

可视化：**成本桑基图** 展示维度间的资金流向。

### 3. 五维在线评估

每次 Agent 输出后，LLM-as-Judge 自动打分：

| 维度 | 评分标准 |
|------|---------|
| Accuracy | 事实是否正确 |
| Completeness | 是否覆盖问题所有要点 |
| Tool Selection | 工具选择是否合理 |
| Reasoning Depth | 推理链是否充分 |
| Helpfulness | 对用户是否有实际帮助 |

### 4. 失败模式聚类

自动从失败 Trace 中聚类出失败模式：

```
50% 失败：工具参数错误（web_search query 格式不对）
25% 失败：推理链断裂（Step 3 后失去上下文）
15% 失败：超时（超过 30s）
10% 失败：幻觉（编造事实）
```

自动生成改进建议：**"建议在 Prompt 中加入工具参数示例"**。

### 5. Harness 版本化与灰度

- Prompt/Harness 配置 git 化版本管理
- 支持按比例灰度（10% → 50% → 100%）
- 一键回滚到任意历史版本
- 同一任务路由到不同配置，自动对比指标

### 6. EvalLoop 迭代工作流

参考 arXiv:2607.05638 的 EvalLoop 方法论：

```
1. 维度化打分 → 2. 识别弱维度 → 3. 假设失败原因
        ↓
4. 单变量修改 Prompt/Harness → 5. A/B 验证 → 6. 应用/回滚
```

避免"瞎改 Prompt 看运气"的迭代方式。

---

## 与其他三个项目的关系

```
InterviewOS ─┐
RepoPilot   ├─ 开发期：怎么写 Agent / 怎么学开源
AgentPrism  ─┘
                              ↓
                        部署上线
                              ↓
AgentPulse ────────────── 运营期：怎么监控、量化、迭代
```

| 生命周期 | 对应项目 | 核心问题 |
|---------|---------|---------|
| 设计期 | InterviewOS | 怎么设计交互式 Agent |
| 学习期 | RepoPilot | 怎么用 Agent 学开源 |
| 实验期 | AgentPrism | 怎么对比 Agent 配置 |
| **运营期** | **AgentPulse** | **上线后怎么监控、量化、迭代** |

四个项目覆盖 Agent 的完整生命周期，与你的 Agent 开发知识图谱形成闭环。

---

## 技术栈

| 层 | 技术 | 选型理由 |
|----|------|---------|
| 后端 | **Go + Gin** | 高并发，Trace 场景吞吐优先 |
| Trace 存储 | **ClickHouse** | OLAP，适合 Trace 高吞吐写入与查询 |
| 元数据 | **PostgreSQL** | 强事务，关系查询 |
| 向量 | **Chroma** | 失败模式聚类辅助 |
| Trace 协议 | **OpenTelemetry (OTLP)** | 行业标准，兼容 Langfuse 等生态 |
| LLM Judge | 多模型（GPT-4o/Claude/DeepSeek） | BYOK |
| Go SDK | OpenTelemetry Go SDK | 装饰器模式自动上报 |
| Python SDK | OpenTelemetry Python SDK + LangChain/LangGraph 适配器 | 兼容前三个项目 |
| 前端 | **Next.js 15 + Recharts + D3.js** | 与其他项目保持一致 |
| 部署 | Docker Compose（本地）/ Kubernetes（生产） | 标准方案 |

---

## 差异化定位

市面上已有 Langfuse、LangSmith、Helicone 等类似产品，AgentPulse 的差异化在于：

| 差异化方向 | Langfuse | LangSmith | AgentPulse |
|------------|----------|-----------|------------|
| 五维成本归因 | 部分 | 部分 | ✅ 完整 |
| 失败模式自动聚类 | ❌ | ❌ | ✅ |
| EvalLoop 迭代工作流 | ❌ | ❌ | ✅ |
| Harness 版本化与灰度 | ❌ | 部分 | ✅ |
| 双语言 SDK | ✅ | ✅ | ✅ |

主攻 **成本归因 + 失败聚类 + EvalLoop 迭代** 三个差异化能力。

---

## 仓库结构

```
AgentPulse/
├── backend/            # Go 后端服务（cmd + internal + pkg 标准 layout）
├── web/                # Next.js 前端（Trace 火焰图 / 成本桑基图 / 漂移曲线）
├── sdk-go/             # Go SDK（装饰器 + OTLP exporter）
├── sdk-python/         # Python SDK（含 LangChain/LangGraph 适配器）
├── docs/               # PRD/架构/API 文档
├── deploy/             # docker-compose.yml / k8s manifests
├── .env.example
├── .gitignore
└── README.md
```

**结构说明**：

- **不使用 monorepo**——Go/Python/前端语言不同，耦合度低，单仓多目录结构更清晰
- 与你已有的 InterviewOS / RepoPilot / AgentPrism 项目保持一致风格
- Go 后端采用社区标准 `cmd + internal + pkg` 分层
- Python SDK 采用现代 `src/` layout（避免 import 路径混乱）

---

## 快速开始

### 1. 启动基础设施

```bash
cd deploy
docker compose up -d  # 启动 ClickHouse + PostgreSQL + Chroma
```

### 2. 启动后端

```bash
cd backend
go run cmd/server/main.go
```

### 3. 安装 Python SDK

```bash
cd sdk-python
source .venv/bin/activate  # Linux/macOS
# 或 .venv\Scripts\activate  # Windows
pip install -e .
```

### 4. 在你的 Agent 中集成

```python
from agentpulse import init, wrap

init(api_key="ap-your-key", endpoint="http://localhost:8080")

@wrap(session_id="user-123")
def my_agent(query: str) -> str:
    return llm.invoke(query)  # 自动上报 Trace
```

### 5. 打开前端

```bash
cd web
npm install
npm run dev
# 访问 http://localhost:3000
```

---

## 前沿论文对应

| 论文 | 对应模块 |
|------|---------|
| EvalLoop (arXiv:2607.05638) | EvalLoop 迭代工作流 |
| HarnessX (arXiv:2606.14249) | Harness 版本化与灰度 |
| CXI (arXiv:2607.06000) | Trace 完整性审计 |
| AgentDojo (arXiv 系列) | 失败模式测试集 |

---

## MVP 范围

### Phase 1：Trace + 成本归因 + 在线评估（核心三模块）

- Trace OTLP 接入 + ClickHouse 写入
- 五维成本归因 API + 桑基图
- LLM-as-Judge 五维评分 + 维度雷达图

### Phase 2：失败聚类 + Harness 版本化

- 失败模式自动聚类 + 改进建议
- Harness Config git 化 + 灰度发布 + 回滚

### Phase 3：EvalLoop 迭代 + A/B 测试

- 维度诊断 → 单变量修改 → 验证闭环
- 同任务路由不同配置，自动对比

### Phase 4：SDK 完善 + 文档

- LangChain / LangGraph / AutoGen 适配器
- 完整使用文档 + 示例

---

## License

MIT