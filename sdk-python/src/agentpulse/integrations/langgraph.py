"""LangGraph 集成。

LangGraph 通过 Checkpoint + Stream 实现追踪，与 LangChain 类似。
"""

from __future__ import annotations

import logging
from typing import Any, Optional

logger = logging.getLogger(__name__)


# LangGraph 提供了 langchain_core.callbacks.BaseCallbackHandler 兼容接口
# 因此 AgentPulseCallback 可以直接复用
def create_agentpulse_tracer(agent_name: str = "") -> Any:
    """创建 LangGraph 兼容的 Tracer。

    LangGraph 内部使用 LangChain Callback 机制，所以可以直接复用
    AgentPulseCallback。
    """
    try:
        from agentpulse.integrations import AgentPulseCallback
        return AgentPulseCallback(agent_name=agent_name)
    except ImportError as exc:
        logger.warning("Cannot create AgentPulse Tracer: %s", exc)
        return None


__all__ = ["create_agentpulse_tracer"]