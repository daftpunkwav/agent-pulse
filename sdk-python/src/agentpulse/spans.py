"""AgentPulse Span 上下文管理。

提供三种使用方式：

1. 装饰器（自动追踪整个函数）::

    @observe(agent_name="my-agent")
    def my_function(x):
        return llm.invoke(x)

2. Context Manager（手动控制）::

    with session(user_id="u-123") as s:
        with trace("llm_call", agent_name="my-agent") as t:
            t.set_input("hello")
            result = llm.invoke("hello")
            t.set_output(result)

3. 直接获取当前 Span（高级用法）::

    span = current_span()
    if span:
        span.set_attribute("key", "value")
"""

from __future__ import annotations

import contextlib
import logging
import time
import uuid
from contextvars import ContextVar
from typing import Any, Optional

from opentelemetry import trace as otel_trace
from opentelemetry.trace import Span, Status, StatusCode

from agentpulse.client import get_client

logger = logging.getLogger(__name__)

# 当前活跃 Session 的 ContextVar（用于跨函数追踪用户/会话上下文）
_current_session: ContextVar[Optional["Session"]] = ContextVar(
    "_current_session", default=None
)


# =============================================================================
# Session - 跨 Span 共享的会话级上下文
# =============================================================================

class Session:
    """会话上下文。

    用于跨 Span 共享 user_id、session_id、agent_name 等元数据。
    通常由外层调用方创建，所有内层 Span 自动继承。
    """

    def __init__(
        self,
        user_id: str = "",
        session_id: Optional[str] = None,
        agent_name: str = "",
        metadata: Optional[dict[str, Any]] = None,
    ):
        self.user_id = user_id
        self.session_id = session_id or str(uuid.uuid4())
        self.agent_name = agent_name
        self.metadata = metadata or {}
        self._token = None

    def to_attributes(self) -> dict[str, Any]:
        """转为 Span attribute 字典。"""
        attrs = {
            "ap.session_id": self.session_id,
            "ap.user_id": self.user_id,
            "ap.agent_name": self.agent_name,
        }
        # 合并 metadata，保留原生数值类型
        for key, value in self.metadata.items():
            attr_key = f"ap.session.metadata.{key}"
            if isinstance(value, (bool, int, float, str)):
                attrs[attr_key] = value
            elif isinstance(value, (list, tuple)):
                attrs[attr_key] = [str(v) for v in value]
            else:
                attrs[attr_key] = str(value)
        return attrs

    def __enter__(self) -> "Session":
        self._token = _current_session.set(self)
        return self

    def __exit__(self, exc_type, exc_val, exc_tb) -> None:
        if self._token is not None:
            _current_session.reset(self._token)


def get_current_session() -> Optional[Session]:
    """获取当前活跃 Session。"""
    return _current_session.get()


# =============================================================================
# Span - 单次追踪的 Span 包装
# =============================================================================

class SpanWrapper:
    """Span 包装，提供 AgentPulse 增强 API。

    内部包装 OpenTelemetry Span，提供：
    - 类型化的字段（model/tokens/cost）
    - 智能默认值（自动时间戳、自动完成）
    - AgentPulse 语义约定的便捷方法
    """

    def __init__(self, span: Span, span_type: str = "agent", name: str = ""):
        self._span = span
        self._span_type = span_type
        self._name = name
        self._start_time = time.time()
        self._completed = False

    @property
    def otel_span(self) -> Span:
        """获取底层 OpenTelemetry Span。"""
        return self._span

    @property
    def span_id(self) -> str:
        """获取 Span ID（十六进制字符串）。"""
        return format(self._span.get_span_context().span_id, "016x")

    @property
    def trace_id(self) -> str:
        """获取 Trace ID（十六进制字符串）。"""
        return format(self._span.get_span_context().trace_id, "032x")

    def set_attribute(self, key: str, value: Any) -> "SpanWrapper":
        """设置 Span 属性。"""
        self._span.set_attribute(key, value)
        return self

    def set_input(self, value: str, max_length: int = 4000) -> "SpanWrapper":
        """设置输入预览（用于后端 trace 回放）。"""
        preview = value if len(value) <= max_length else value[:max_length]
        self._span.set_attribute("ap.input_preview", preview)
        return self

    def set_output(self, value: str, max_length: int = 4000) -> "SpanWrapper":
        """设置输出预览。"""
        preview = value if len(value) <= max_length else value[:max_length]
        self._span.set_attribute("ap.output_preview", preview)
        return self

    # ---- LLM 专用字段 ----

    def set_llm(
        self,
        model: str,
        prompt_tokens: int = 0,
        completion_tokens: int = 0,
        cost_usd: Optional[float] = None,
        finish_reason: str = "",
    ) -> "SpanWrapper":
        """设置 LLM 调用信息。

        Args:
            model: 模型名（如 "gpt-4o"）。
            prompt_tokens: 输入 token 数。
            completion_tokens: 输出 token 数。
            cost_usd: 成本（USD）。None 时由后端自动计算。
            finish_reason: 结束原因。
        """
        self._span.set_attribute("ap.span_type", "llm")
        self._span.set_attribute("ap.model", model)
        self._span.set_attribute("ap.prompt_tokens", prompt_tokens)
        self._span.set_attribute("ap.completion_tokens", completion_tokens)
        self._span.set_attribute("ap.total_tokens", prompt_tokens + completion_tokens)
        if cost_usd is not None:
            self._span.set_attribute("ap.cost_usd", cost_usd)
        if finish_reason:
            self._span.set_attribute("ap.finish_reason", finish_reason)
        return self

    # ---- Tool 专用字段 ----

    def set_tool(self, tool_name: str, args: Optional[dict] = None, result_preview: str = "") -> "SpanWrapper":
        """设置工具调用信息。"""
        self._span.set_attribute("ap.span_type", "tool")
        self._span.set_attribute("ap.tool_name", tool_name)
        if args is not None:
            import json
            try:
                self._span.set_attribute("ap.tool_args", json.dumps(args, default=str, ensure_ascii=False)[:1000])
            except (TypeError, ValueError):
                # 工具参数不可 JSON 序列化时跳过，不应阻塞主流程
                pass
        if result_preview:
            self._span.set_attribute("ap.tool_result_preview", result_preview[:1000])
        return self

    # ---- Reasoning 专用字段 ----

    def set_reasoning(self, step: int, thought: str = "", action: str = "") -> "SpanWrapper":
        """设置推理步骤信息。"""
        self._span.set_attribute("ap.span_type", "reasoning")
        self._span.set_attribute("ap.reasoning_step", step)
        if thought:
            self._span.set_attribute("ap.reasoning_thought", thought[:2000])
        if action:
            self._span.set_attribute("ap.reasoning_action", action[:1000])
        return self

    # ---- 状态控制 ----

    def set_status_ok(self) -> "SpanWrapper":
        """标记 Span 为成功状态。"""
        self._span.set_status(Status(StatusCode.OK))
        return self

    def set_status_error(self, error: str) -> "SpanWrapper":
        """标记 Span 为失败状态。"""
        self._span.set_status(Status(StatusCode.ERROR, error))
        self._span.set_attribute("ap.error_message", error[:2000])
        return self

    def record_exception(self, exception: BaseException) -> "SpanWrapper":
        """记录异常。"""
        self._span.record_exception(exception)
        self.set_status_error(str(exception))
        return self

    def end(self) -> None:
        """显式结束 Span。

        通常不需要调用，Context Manager / 装饰器会自动结束。
        """
        if not self._completed:
            self._span.end()
            self._completed = True


# =============================================================================
# Context Manager - 上下文管理器
# =============================================================================

@contextlib.contextmanager
def session(
    user_id: str = "",
    session_id: Optional[str] = None,
    agent_name: str = "",
    metadata: Optional[dict[str, Any]] = None,
):
    """会话上下文。

    用法::

        with session(user_id="u-123", agent_name="my-agent") as s:
            # 内部所有 Span 自动继承 session_id/user_id/agent_name
            with trace("llm_call") as t:
                ...
    """
    sess = Session(
        user_id=user_id,
        session_id=session_id,
        agent_name=agent_name,
        metadata=metadata,
    )
    with sess:
        yield sess


def _set_ap_attribute(otel_span: Span, key: str, value: Any) -> None:
    """写入 ap.* 属性，保留原生数值类型，避免重复 ap. 前缀。"""
    attr_key = key if key.startswith("ap.") else f"ap.{key}"
    if isinstance(value, bool):
        otel_span.set_attribute(attr_key, value)
    elif isinstance(value, int):
        otel_span.set_attribute(attr_key, value)
    elif isinstance(value, float):
        otel_span.set_attribute(attr_key, value)
    elif isinstance(value, str):
        otel_span.set_attribute(attr_key, value)
    elif isinstance(value, (list, tuple)):
        otel_span.set_attribute(attr_key, [str(v) for v in value])
    else:
        raise TypeError(
            f"unsupported attribute type {type(value).__name__} for key={attr_key}; "
            "supported: bool, int, float, str, list, tuple"
        )


@contextlib.contextmanager
def trace(
    name: str,
    span_type: str = "agent",
    **attrs: Any,
):
    """Span 上下文管理器。

    用法::

        with trace("llm_call", span_type="llm", model="gpt-4o") as t:
            t.set_input("hello")
            result = llm.invoke("hello")
            t.set_output(result)
            t.set_llm("gpt-4o", prompt_tokens=10, completion_tokens=20)
    """
    tracer = get_client().get_tracer()
    with tracer.start_as_current_span(name) as otel_span:
        # 写入 span 类型，供后端 collector 正确映射
        otel_span.set_attribute("ap.span_type", span_type)

        # 应用 Session 上下文
        sess = get_current_session()
        if sess:
            for key, value in sess.to_attributes().items():
                otel_span.set_attribute(key, value)

        # 应用显式传入的 attrs（如 model、agent_name 等）
        for key, value in attrs.items():
            _set_ap_attribute(otel_span, key, value)

        wrapper = SpanWrapper(otel_span, span_type=span_type, name=name)
        try:
            yield wrapper
        except Exception as exc:
            wrapper.record_exception(exc)
            raise
        finally:
            wrapper.end()


@contextlib.contextmanager
def span(name: str, **attrs: Any):
    """trace() 的别名。"""
    with trace(name, **attrs) as t:
        yield t


def current_span() -> Optional[SpanWrapper]:
    """获取当前活跃 Span（高级用法）。

    用法::

        sp = current_span()
        if sp:
            sp.set_attribute("custom.key", "value")
    """
    otel_span = otel_trace.get_current_span()
    if otel_span is None or not otel_span.get_span_context().is_valid:
        return None
    return SpanWrapper(otel_span)