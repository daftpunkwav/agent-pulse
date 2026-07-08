# AgentPulse Python SDK

Python SDK for AgentPulse — observability and operations for LLM Agents.

通过 OpenTelemetry 协议将 Agent 调用 Trace 上报到 AgentPulse 后端。

## 安装

```bash
pip install agentpulse
```

使用虚拟环境（推荐，避免污染全局）：

```bash
python -m venv .venv
source .venv/bin/activate  # Linux/macOS
.venv\Scripts\activate     # Windows
pip install -e ".[dev]"
```

带框架集成：

```bash
pip install agentpulse[langchain]
pip install agentpulse[langgraph]
pip install agentpulse[autogen]
```

## 快速开始

### 1. 初始化

应用启动时调用一次：

```python
from agentpulse import init

init(
    api_key="ap-your-key",
    endpoint="http://localhost:8080",
    service_name="my-agent-app",
    environment="production",
)
```

或通过环境变量：

```bash
export AGENTPULSE_API_KEY=ap-your-key
export AGENTPULSE_ENDPOINT=http://localhost:8080
export AGENTPULSE_SERVICE_NAME=my-agent-app
```

```python
from agentpulse import init

init()  # 自动读取环境变量
```

### 2. 装饰器方式

```python
from agentpulse import observe

@observe(agent_name="interview-agent")
def answer_question(question: str) -> str:
    return llm.invoke(question)
```

### 3. Context Manager 方式

```python
from agentpulse import session, trace

with session(user_id="u-123") as sess:
    with trace("llm_call", model="gpt-4o") as t:
        t.set_input(question)
        result = llm.invoke(question)
        t.set_output(result)
        # 自动记录 token 与成本
        t.set_llm("gpt-4o", prompt_tokens=100, completion_tokens=50, cost_usd=0.005)
```

### 4. LangChain 集成

```python
from agentpulse.integrations.langchain import AgentPulseCallback

callback = AgentPulseCallback()
result = chain.invoke(input, config={"callbacks": [callback]})
```

### 5. LangGraph 集成

```python
from agentpulse.integrations.langgraph import create_agentpulse_tracer

tracer = create_agentpulse_tracer(agent_name="my-graph-agent")
app = workflow.compile()
result = app.invoke(input, config={"callbacks": [tracer]})
```

### 6. AutoGen 集成

```python
from agentpulse.integrations.autogen import AgentPulseAutoGenHook

hook = AgentPulseAutoGenHook(agent_name="my-autogen-agent")
wrapped_agent = hook.wrap_agent(my_agent)
```

### 7. 关闭

应用退出前调用：

```python
from agentpulse import shutdown

shutdown()  # 刷新所有缓冲 Span
```

## Span 类型与字段

| Span 类型 | 必填字段 | 设置方法 |
|----------|---------|---------|
| `agent` | name | `@observe()` 或 `trace("name", span_type="agent")` |
| `llm` | model, prompt_tokens, completion_tokens | `t.set_llm(model, prompt_tokens, completion_tokens)` |
| `tool` | tool_name, args | `t.set_tool(tool_name, args={...})` |
| `reasoning` | step | `t.set_reasoning(step=1, thought="...")` |
| `evaluation` | 五维评分 | Phase 2 支持 |

## 核心 API 参考

### `init()`

```python
def init(
    api_key: str = "",
    endpoint: str = "http://localhost:8080",
    service_name: str = "agent-app",
    environment: str = "production",
    sample_rate: float = 1.0,
    flush_interval_seconds: float = 5.0,
    headers: Optional[dict[str, str]] = None,
) -> Client
```

### `observe()`

```python
def observe(
    agent_name: str = "",
    span_type: str = "agent",
    name: Optional[str] = None,
    capture_args: bool = True,
    capture_result: bool = True,
) -> Callable
```

### `session()`

```python
@contextmanager
def session(
    user_id: str = "",
    session_id: Optional[str] = None,
    agent_name: str = "",
    metadata: Optional[dict] = None,
) -> Session
```

### `trace()`

```python
@contextmanager
def trace(
    name: str,
    span_type: str = "agent",
    **attrs: Any,
) -> SpanWrapper
```

## 开发

```bash
# 激活虚拟环境
source .venv/bin/activate  # Linux/macOS
.venv\Scripts\activate     # Windows

# 以开发模式安装（已自动）
pip install -e ".[dev]"

# 运行测试
pytest

# Lint
ruff check src/

# 类型检查
mypy src/
```

## 示例

参见 `examples/` 目录：

- `basic_usage.py` — 装饰器 + Context Manager 基础用法
- `langchain_example.py` — LangChain Callback 集成（Phase 2）

## License

MIT