"""AutoGen 集成。

包装 Agent 的 send/receive 及 a_send/a_receive 异步路径。
"""

from __future__ import annotations

import inspect
import logging
from typing import Any, Callable

from agentpulse.decorators import _safe_serialize
from agentpulse.spans import trace

logger = logging.getLogger(__name__)


class AgentPulseAutoGenHook:
    """AutoGen 回调钩子。

    提供 wrap_agent 装饰器，自动追踪 Agent 的 send/recv 及异步等价方法。

    用法::

        hook = AgentPulseAutoGenHook(agent_name="my-autogen-agent")
        wrapped = hook.wrap_agent(my_agent)
    """

    def __init__(self, agent_name: str = ""):
        self.agent_name = agent_name

    def wrap_agent(self, agent: Any) -> Any:
        """包装 AutoGen Agent，自动追踪其 send/recv 事件（含异步路径）。"""
        for method_name in ("send", "receive", "a_send", "a_receive"):
            original = getattr(agent, method_name, None)
            if original is None or not callable(original):
                continue
            span_name = f"autogen.{method_name.lstrip('a_')}"
            if inspect.iscoroutinefunction(original):
                setattr(agent, method_name, self._wrap_async(original, span_name))
            else:
                setattr(agent, method_name, self._wrap_sync(original, span_name))
        return agent

    def _wrap_sync(self, original: Callable[..., Any], span_name: str) -> Callable[..., Any]:
        def wrapper(*args: Any, **kwargs: Any) -> Any:
            with trace(span_name, span_type="agent", agent_name=self.agent_name) as t:
                t.set_input(_safe_serialize({"args": args, "kwargs": kwargs}, max_length=500))
                try:
                    result = original(*args, **kwargs)
                except Exception as exc:
                    t.record_exception(exc)
                    raise
                t.set_output(_safe_serialize(result, max_length=500))
                t.set_status_ok()
                return result

        return wrapper

    def _wrap_async(self, original: Callable[..., Any], span_name: str) -> Callable[..., Any]:
        async def wrapper(*args: Any, **kwargs: Any) -> Any:
            with trace(span_name, span_type="agent", agent_name=self.agent_name) as t:
                t.set_input(_safe_serialize({"args": args, "kwargs": kwargs}, max_length=500))
                try:
                    result = await original(*args, **kwargs)
                except Exception as exc:
                    t.record_exception(exc)
                    raise
                t.set_output(_safe_serialize(result, max_length=500))
                t.set_status_ok()
                return result

        return wrapper


__all__ = ["AgentPulseAutoGenHook"]
