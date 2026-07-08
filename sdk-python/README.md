# AgentPulse Python SDK

Python SDK for AgentPulse — observability and operations for LLM Agents.

## Installation

```bash
pip install agentpulse
```

With framework integrations:

```bash
pip install agentpulse[langchain]
pip install agentpulse[langgraph]
pip install agentpulse[autogen]
```

## Quick Start

```python
from agentpulse import init, session, trace, observe

# 1. Initialize once at app startup
init(
    api_key="ap-your-key",
    endpoint="http://localhost:8080",
    service_name="my-agent-app",
)

# 2. Decorator pattern
@observe(agent_name="interview-agent")
def answer_question(question: str) -> str:
    return llm.invoke(question)

# 3. Context manager pattern
def run_agent(question: str):
    with session(user_id="u-123") as s:
        with trace("llm_call", model="gpt-4o") as t:
            t.set_input(question)
            result = llm.invoke(question)
            t.set_output(result)
            t.set_tokens(prompt=100, completion=50)
            t.set_cost(0.005)
        return result
```

## Framework Integrations

### LangChain

```python
from agentpulse.integrations.langchain import AgentPulseCallback

callback = AgentPulseCallback(api_key="ap-your-key")
chain.invoke(input, config={"callbacks": [callback]})
```

### LangGraph

```python
from agentpulse.integrations.langgraph import AgentPulseTracer

tracer = AgentPulseTracer(api_key="ap-your-key")
app = workflow.compile(tracers=[tracer])
```

## Development

```bash
# Create virtual environment (already done)
source .venv/bin/activate  # Linux/macOS
.venv\Scripts\activate     # Windows

# Install in editable mode with dev dependencies
pip install -e ".[dev]"

# Run tests
pytest

# Lint
ruff check src/

# Type check
mypy src/
```

## License

MIT