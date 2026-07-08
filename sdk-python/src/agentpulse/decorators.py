"""AgentPulse 装饰器。

提供 @observe 和 @observe_async 装饰器，自动追踪函数调用。

用法::

    @observe(agent_name="my-agent")
    def my_function(x: str) -> str:
        return llm.invoke(x)

    @observe(agent_name="my-agent", span_type="llm")
    async def async_llm_call(prompt: str) -> str:
        return await llm.ainvoke(prompt)

装饰器自动捕获：
- 函数名（作为 Span name）
- 输入参数（JSON 序列化预览）
- 返回值（预览）
- 异常
- 执行时长（自动）
"""

from __future__ import annotations

import functools
import inspect
import json
import logging
from typing import Any, Callable, Optional, TypeVar, Union

from agentpulse.spans import Session, get_current_session, get_client

logger = logging.getLogger(__name__)

F = TypeVar("F", bound=Callable[..., Any])


def _safe_serialize(value: Any, max_length: int = 1000) -> str:
    """安全地将 Python 对象序列化为字符串。

    - str/int/float/bool 直接 str()
    - dict/list 尝试 JSON 序列化
    - 其他 repr()
    """
    if value is None:
        return ""
    if isinstance(value, (str, int, float, bool)):
        s = str(value)
    elif isinstance(value, (dict, list, tuple)):
        try:
            s = json.dumps(value, default=str, ensure_ascii=False)
        except Exception:  # pylint: disable=broad-except
            s = repr(value)
    else:
        s = repr(value)
    return s[:max_length]


def _format_args(args: tuple, kwargs: dict, max_length: int = 1000) -> str:
    """格式化函数参数为字符串。"""
    parts = []
    for arg in args:
        parts.append(_safe_serialize(arg, max_length // 4))
    for key, value in kwargs.items():
        parts.append(f"{key}={_safe_serialize(value, max_length // 4)}")
    return ", ".join(parts)


def observe(
    agent_name: str = "",
    span_type: str = "agent",
    name: Optional[str] = None,
    capture_args: bool = True,
    capture_result: bool = True,
) -> Callable[[F], F]:
    """装饰器：自动追踪函数调用。

    Args:
        agent_name: Agent 名称（覆盖 Session 中的 agent_name）。
        span_type: Span 类型（agent/llm/tool/reasoning/evaluation）。
        name: 自定义 Span 名（默认使用函数名）。
        capture_args: 是否捕获函数参数。
        capture_result: 是否捕获返回值。

    用法::

        @observe(agent_name="interview-agent")
        def answer(question: str) -> str:
            return llm.invoke(question)
    """
    def decorator(func: F) -> F:
        is_async = inspect.iscoroutinefunction(func)
        span_name = name or func.__name__

        if is_async:
            @functools.wraps(func)
            async def async_wrapper(*args, **kwargs):
                return await _execute_traced_async(
                    func, args, kwargs, span_name, agent_name, span_type, capture_args, capture_result
                )
            return async_wrapper  # type: ignore[return-value]
        else:
            @functools.wraps(func)
            def sync_wrapper(*args, **kwargs):
                return _execute_traced_sync(
                    func, args, kwargs, span_name, agent_name, span_type, capture_args, capture_result
                )
            return sync_wrapper  # type: ignore[return-value]

    return decorator


def observe_async(
    agent_name: str = "",
    span_type: str = "agent",
    name: Optional[str] = None,
    capture_args: bool = True,
    capture_result: bool = True,
) -> Callable[[F], F]:
    """异步函数装饰器（observe 的别名，明确表示异步语义）。"""
    return observe(
        agent_name=agent_name,
        span_type=span_type,
        name=name,
        capture_args=capture_args,
        capture_result=capture_result,
    )


# ---------------------------------------------------------------------------
# 内部执行函数
# ---------------------------------------------------------------------------

def _execute_traced_sync(
    func: Callable,
    args: tuple,
    kwargs: dict,
    span_name: str,
    agent_name: str,
    span_type: str,
    capture_args: bool,
    capture_result: bool,
) -> Any:
    """同步函数追踪执行。"""
    from agentpulse.spans import trace as trace_cm  # 避免循环引用

    with trace_cm(span_name, span_type=span_type, agent_name=agent_name or "") as t:
        if capture_args:
            args_str = _format_args(args, kwargs)
            t.set_input(args_str)
            t.set_attribute("ap.function", func.__name__)
            t.set_attribute("ap.module", func.__module__)

        try:
            result = func(*args, **kwargs)
        except Exception as exc:
            t.record_exception(exc)
            raise

        if capture_result and result is not None:
            t.set_output(_safe_serialize(result))

        t.set_status_ok()
        return result


async def _execute_traced_async(
    func: Callable,
    args: tuple,
    kwargs: dict,
    span_name: str,
    agent_name: str,
    span_type: str,
    capture_args: bool,
    capture_result: bool,
) -> Any:
    """异步函数追踪执行。"""
    from agentpulse.spans import trace as trace_cm

    with trace_cm(span_name, span_type=span_type, agent_name=agent_name or "") as t:
        if capture_args:
            args_str = _format_args(args, kwargs)
            t.set_input(args_str)
            t.set_attribute("ap.function", func.__name__)
            t.set_attribute("ap.module", func.__module__)

        try:
            result = await func(*args, **kwargs)
        except Exception as exc:
            t.record_exception(exc)
            raise

        if capture_result and result is not None:
            t.set_output(_safe_serialize(result))

        t.set_status_ok()
        return result