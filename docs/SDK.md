# AgentPulse SDK Guide

> Python & Go SDK reference for AgentPulse v0.1.0

## 1. 设计目标

- **零侵入**：装饰器/Context Manager 风格，业务代码改动最小
- **OpenTelemetry 原生**：复用 OTel SDK，避免重复造轮子
- **多框架支持**：原生 Python + LangChain + LangGraph + AutoGen
- **异步友好**：同步/异步统一 API

## 2. Python SDK

详见 [`sdk-python/README.md`](../sdk-python/README.md)

### 2.1 安装

```bash
pip install agentpulse
# 或（虚拟环境，推荐）
python -m venv .venv
source .venv/bin/activate
pip install agentpulse
```

### 2.2 核心 API 速查

| API | 用途 | 示例 |
|-----|------|------|
| `init()` | 初始化客户端 | `init(endpoint=..., service_name=...)` |
| `shutdown()` | 关闭并刷新 | 应用退出前调用 |
| `@observe()` | 装饰器自动追踪 | `@observe(agent_name="...")` |
| `session()` | 用户/会话上下文 | `with session(user_id="u1")` |
| `trace()` | Span 上下文 | `with trace("llm_call")` |
| `current_span()` | 获取当前 Span | `sp = current_span()` |

### 2.3 Span 类型与字段

| Span Type | 必填字段 | 设置方法 |
|----------|---------|---------|
| `agent` | `name` | 装饰器或 `trace("name", span_type="agent")` |
| `llm` | `model`, `prompt_tokens`, `completion_tokens` | `t.set_llm(model, prompt_tokens, completion_tokens, cost_usd)` |
| `tool` | `tool_name`, `args` | `t.set_tool(tool_name, args={...})` |
| `reasoning` | `step` | `t.set_reasoning(step, thought="...")` |

### 2.4 自定义属性透传

任何 `ap.*` 属性都会自动从 OTLP 透传到后端：

```python
t.set_attribute("ap.custom.business_metric", 42)
# 后端存储到 agent_spans.attributes JSON 字段
```

### 2.5 集成示例

**LangChain**：
```python
from agentpulse.integrations.langchain import AgentPulseCallback

callback = AgentPulseCallback()
chain.invoke(input, config={"callbacks": [callback]})
```

**LangGraph**：
```python
from agentpulse.integrations.langgraph import create_agentpulse_tracer

tracer = create_agentpulse_tracer(agent_name="my-graph")
app = workflow.compile()
app.invoke(input, config={"callbacks": [tracer]})
```

**AutoGen**：
```python
from agentpulse.integrations.autogen import AgentPulseAutoGenHook

hook = AgentPulseAutoGenHook(agent_name="my-autogen")
hook.wrap_agent(my_agent)
```

## 3. Go SDK（Phase 3 实现）

### 3.1 计划 API

```go
import "github.com/agentpulse/sdk-go"

func main() {
    ap.Init(ap.Config{
        APIKey:      "ap-key",
        Endpoint:    "http://localhost:8080",
        ServiceName: "my-service",
    })
    defer ap.Shutdown()

    // 装饰器
    agent := ap.Wrap(myAgent, ap.WithAgentName("interview-agent"))
    result, _ := agent.Run(ctx, "...")
}
```

## 4. 通用 OTLP 接入

任何支持 OTLP 的客户端都可以直接接入 AgentPulse：

```bash
# 环境变量配置
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf

# 启动你的应用（任意语言）
python my_agent.py
```

Trace 会自动上报到 AgentPulse。配合 `ap.*` 属性自定义业务字段。

## 5. OpenTelemetry 语义约定映射

AgentPulse 同时支持两种属性命名：

### 5.1 OTel GenAI 语义约定（标准）

| 属性 | 含义 | AgentPulse 字段 |
|------|------|----------------|
| `gen_ai.system` | 系统名 | `ap.agent_name` |
| `gen_ai.request.model` | 请求模型 | `ap.model` |
| `gen_ai.response.model` | 响应模型 | `ap.model` |
| `gen_ai.usage.input_tokens` | 输入 tokens | `ap.prompt_tokens` |
| `gen_ai.usage.output_tokens` | 输出 tokens | `ap.completion_tokens` |
| `gen_ai.response.finish_reasons` | 结束原因 | `ap.finish_reason` |

### 5.2 AgentPulse 自定义属性（推荐）

| 属性 | 含义 |
|------|------|
| `ap.session_id` | 会话 ID（自动继承） |
| `ap.user_id` | 用户 ID（自动继承） |
| `ap.agent_name` | Agent 名称 |
| `ap.span_type` | Span 类型（agent/llm/tool/reasoning/evaluation） |
| `ap.model` | 模型名 |
| `ap.prompt_tokens` | 输入 tokens |
| `ap.completion_tokens` | 输出 tokens |
| `ap.cost_usd` | 成本（USD） |
| `ap.tool_name` | 工具名 |
| `ap.reasoning_step` | 推理步骤编号 |
| `ap.input_preview` | 输入预览 |
| `ap.output_preview` | 输出预览 |
| `ap.error_message` | 错误信息 |

`ap.*` 优先级高于 OTel 约定，便于业务自定义。

## 6. 性能开销

| 操作 | 开销 |
|------|------|
| 装饰器包装 | < 0.1ms |
| Span 创建 | < 1ms |
| OTLP 异步上报 | 不阻塞业务（BatchSpanProcessor 缓冲） |
| 批量大小 | 默认 2048 spans |
| 刷新间隔 | 默认 5 秒 |

可通过 `flush_interval_seconds` 与 `max_queue_size` 调整。

## 7. 调试模式

启用 DEBUG 日志：

```python
import logging
logging.basicConfig(level=logging.DEBUG)

from agentpulse import init
init(...)
```

## 8. 常见问题

### Q1: Trace 找不到？

检查：
1. `init()` 是否被调用
2. OTLP endpoint 是否可达（默认 `http://localhost:4318`）
3. 服务端是否监听 4318 端口

### Q2: 评估没触发？

检查：
1. `sample_rate` 配置（默认 1.0 = 全量）
2. `judge_client` 是否正确初始化（API Key 等）
3. 后端 Judge 配置正确

### Q3: Span 父子关系错乱？

确保在父 Span 的 `with trace()` 内创建子 Span。SDK 会自动维护 OTel Context。

### Q4: 如何关闭采样？

```python
init(sample_rate=0.1)  # 10% 采样
```

## 9. 最佳实践

1. **应用启动时调用一次 `init()`**，不要每次请求都调用
2. **使用 Session 上下文**共享 user_id/session_id，避免每个 Span 重复设置
3. **关键 LLM 调用用 `@observe(span_type="llm")`**，便于后端精准归因
4. **异常时调用 `record_exception()`**，自动设置 error_message
5. **应用退出前调用 `shutdown()`**，确保所有 Span 上报完成