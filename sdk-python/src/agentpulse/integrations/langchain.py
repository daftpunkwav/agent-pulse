"""LangChain 集成（占位导入）。

通过 `agentpulse.integrations.langchain` 直接访问 AgentPulseCallback。
"""

from agentpulse.integrations.langchain_callback import (  # noqa: F401
    AgentPulseCallback,
    LANGCHAIN_AVAILABLE,
)

__all__ = ["AgentPulseCallback", "LANGCHAIN_AVAILABLE"]