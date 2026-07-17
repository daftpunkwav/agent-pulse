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


def create_agentpulse_callback(
    agent_name: str = "",
    *,
    capture_content: bool = False,
) -> "AgentPulseCallback":
    """创建 LangGraph 兼容的 Callback Handler。

    LangGraph 通过 `config={"callbacks": [handler]}` 传入回调列表。

    Args:
        agent_name: 默认 agent 名。
        capture_content: 是否上报 prompt/输出（默认 False，防 PII）。

    Raises:
        ImportError: langchain-core 未安装时抛出。
    """
    if not LANGCHAIN_AVAILABLE:
        raise ImportError(
            "langchain-core is required for LangGraph integration. "
            "Install with: pip install agentpulse[langchain]"
        )
    from agentpulse.integrations.langchain_callback import AgentPulseCallback

    return AgentPulseCallback(agent_name=agent_name, capture_content=capture_content)


__all__ = ["create_agentpulse_callback"]

# 向后兼容旧函数名
create_agentpulse_tracer = create_agentpulse_callback
