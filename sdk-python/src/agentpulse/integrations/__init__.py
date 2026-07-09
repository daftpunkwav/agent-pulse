"""框架集成子包。

子模块：
- langchain_callback: LangChain CallbackHandler (`AgentPulseCallback`)
- langgraph: LangGraph 集成 (`create_agentpulse_callback`)
- autogen: AutoGen 集成 (`AgentPulseAutoGenHook`)
"""

from agentpulse.integrations.autogen import AgentPulseAutoGenHook
from agentpulse.integrations.langchain_callback import AgentPulseCallback, LANGCHAIN_AVAILABLE
from agentpulse.integrations.langgraph import create_agentpulse_callback

__all__ = [
    "AgentPulseCallback",
    "LANGCHAIN_AVAILABLE",
    "create_agentpulse_callback",
    "AgentPulseAutoGenHook",
]
