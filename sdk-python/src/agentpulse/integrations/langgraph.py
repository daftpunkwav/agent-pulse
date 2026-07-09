"""LangGraph 集成。

LangGraph 内部使用 LangChain Callback 机制，直接复用 AgentPulseCallback。
"""

from __future__ import annotations

import logging
from typing import TYPE_CHECKING

from agentpulse.integrations.langchain_callback import LANGCHAIN_AVAILABLE

if TYPE_CHECKING:
    from agentpulse.integrations.langchain_callback import AgentPulseCallback

logger = logging.getLogger(__name__)


def create_agentpulse_tracer(agent_name: str = "") -> "AgentPulseCallback":
    """创建 LangGraph 兼容的 Callback Handler。

    LangGraph 通过 `config={"callbacks": [handler]}` 传入回调列表。

    Raises:
        ImportError: langchain-core 未安装时抛出。
    """
    if not LANGCHAIN_AVAILABLE:
        raise ImportError(
            "langchain-core is required for LangGraph integration. "
            "Install with: pip install agentpulse[langchain]"
        )
    from agentpulse.integrations.langchain_callback import AgentPulseCallback

    return AgentPulseCallback(agent_name=agent_name)


__all__ = ["create_agentpulse_tracer"]
