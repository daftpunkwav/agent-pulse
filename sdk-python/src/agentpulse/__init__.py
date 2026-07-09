"""AgentPulse Python SDK.

面向 LLM Agent 的可观测性 SDK。

通过 OpenTelemetry 协议上报 Trace 到 AgentPulse 后端，
支持装饰器、Context Manager、LangChain/LangGraph/AutoGen 适配器。

基本用法：

    from agentpulse import init, observe, session, trace

    # 初始化（应用启动时调用一次）
    init(
        api_key="ap-your-key",
        endpoint="http://localhost:8080",
        service_name="my-agent-app",
    )

    # 装饰器方式
    @observe(agent_name="interview-agent")
    def answer_question(question: str) -> str:
        return llm.invoke(question)

    # Context Manager 方式
    with session(user_id="u-123") as s:
        with trace("llm_call", model="gpt-4o") as t:
            t.set_input(question)
            result = llm.invoke(question)
            t.set_output(result)
"""

from agentpulse._version import __version__
from agentpulse.client import Client, init, get_client, shutdown
from agentpulse.decorators import observe, observe_async
from agentpulse.spans import session, trace, span, current_span

__all__ = [
    "Client",
    "init",
    "get_client",
    "shutdown",
    "observe",
    "observe_async",
    "session",
    "trace",
    "span",
    "current_span",
    "__version__",
]