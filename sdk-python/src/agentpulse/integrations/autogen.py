"""AutoGen 集成（基础支持）。

AutoGen 通过事件回调机制工作。
完整适配器待 Phase 2 实现。
"""

from __future__ import annotations

import logging
from typing import Any, Callable, Optional

from agentpulse.spans import get_client, trace

logger = logging.getLogger(__name__)


class AgentPulseAutoGenHook:
    """AutoGen 回调钩子（基础实现）。

    AutoGen 使用 on_send / on_receive 钩子函数。
    本类提供 wrap_agent 装饰器，自动追踪 Agent 的 send/recv 事件。

    用法::

        hook = AgentPulseAutoGenHook(agent_name="my-autogen-agent")
        wrapped = hook.wrap_agent(my_agent)
    """

    def __init__(self, agent_name: str = ""):
        self.agent_name = agent_name
        self._span_active = False

    def wrap_agent(self, agent: Any) -> Any:
        """包装 AutoGen Agent，自动追踪其 send/recv 事件。"""
        original_send = getattr(agent, "send", None)
        original_receive = getattr(agent, "receive", None)

        if original_send is not None:
            agent.send = self._wrap_send(original_send)
        if original_receive is not None:
            agent.receive = self._wrap_receive(original_receive)

        return agent

    def _wrap_send(self, original: Callable) -> Callable:
        def wrapper(*args, **kwargs):
            with trace("autogen.send", span_type="agent", agent_name=self.agent_name) as t:
                t.set_input(str(args)[:500])
                result = original(*args, **kwargs)
                t.set_output(str(result)[:500])
                t.set_status_ok()
                return result
        return wrapper

    def _wrap_receive(self, original: Callable) -> Callable:
        def wrapper(*args, **kwargs):
            with trace("autogen.receive", span_type="agent", agent_name=self.agent_name) as t:
                t.set_input(str(args)[:500])
                result = original(*args, **kwargs)
                t.set_output(str(result)[:500])
                t.set_status_ok()
                return result
        return wrapper


__all__ = ["AgentPulseAutoGenHook"]