"""LangChain CallbackHandler 集成。

实现 `BaseCallbackHandler`，自动追踪 LangChain 的 chain/llm/tool/agent 事件。
缺 `langchain-core` 时 `LANGCHAIN_AVAILABLE=False`，实例化会抛 ImportError。
"""

from __future__ import annotations

import logging
import uuid
from typing import Any, Optional, Union

from agentpulse.decorators import _safe_serialize
from agentpulse.spans import SpanWrapper, get_current_session

logger = logging.getLogger(__name__)

LANGCHAIN_AVAILABLE = False

try:
    from langchain_core.agents import AgentAction, AgentFinish
    from langchain_core.callbacks import BaseCallbackHandler
    from langchain_core.outputs import LLMResult

    LANGCHAIN_AVAILABLE = True
except ImportError:
    BaseCallbackHandler = None  # type: ignore[misc, assignment]
    AgentAction = None  # type: ignore[misc, assignment]
    AgentFinish = None  # type: ignore[misc, assignment]
    LLMResult = None  # type: ignore[misc, assignment]


def _make_callback_class() -> type:
    """根据 langchain-core 是否可用动态构建 Callback 类。"""

    if not LANGCHAIN_AVAILABLE or BaseCallbackHandler is None:
        class _UnavailableCallback:  # noqa: N801
            def __init__(self, *args: Any, **kwargs: Any) -> None:
                raise ImportError(
                    "langchain-core is required for AgentPulseCallback. "
                    "Install with: pip install agentpulse[langchain]"
                )

        return _UnavailableCallback

    class AgentPulseCallback(BaseCallbackHandler):  # type: ignore[misc, valid-type]
        """LangChain 回调处理器，将 chain/llm/tool/agent 事件映射为 AgentPulse Span。"""

        def __init__(self, agent_name: str = "", **kwargs: Any) -> None:
            super().__init__(**kwargs)
            self.agent_name = agent_name
            self._spans: dict[uuid.UUID, SpanWrapper] = {}

        def _resolve_agent_name(self) -> str:
            sess = get_current_session()
            if sess and sess.agent_name:
                return sess.agent_name
            return self.agent_name

        def _start_span(
            self,
            run_id: uuid.UUID,
            name: str,
            span_type: str,
            parent_run_id: Optional[uuid.UUID] = None,
        ) -> SpanWrapper:
            from opentelemetry import trace as otel_trace
            from opentelemetry.trace import set_span_in_context

            from agentpulse.client import get_client

            tracer = get_client().get_tracer()
            parent = self._spans.get(parent_run_id) if parent_run_id else None
            if parent is not None:
                ctx = set_span_in_context(parent.otel_span)
                otel_span = tracer.start_span(name, context=ctx)
            else:
                otel_span = tracer.start_span(name)

            wrapper = SpanWrapper(otel_span, span_type=span_type, name=name)
            wrapper.set_attribute("ap.span_type", span_type)
            agent = self._resolve_agent_name()
            if agent:
                wrapper.set_attribute("ap.agent_name", agent)

            sess = get_current_session()
            if sess:
                for key, value in sess.to_attributes().items():
                    wrapper.set_attribute(key, value)

            self._spans[run_id] = wrapper
            return wrapper

        def _end_span(
            self,
            run_id: uuid.UUID,
            *,
            error: Optional[BaseException] = None,
        ) -> None:
            wrapper = self._spans.pop(run_id, None)
            if wrapper is None:
                return
            if error is not None:
                wrapper.record_exception(error)
            else:
                wrapper.set_status_ok()
            wrapper.end()

        # ---- Chain ----

        def on_chain_start(
            self,
            serialized: dict[str, Any],
            inputs: dict[str, Any],
            *,
            run_id: uuid.UUID,
            parent_run_id: Optional[uuid.UUID] = None,
            tags: Optional[list[str]] = None,
            metadata: Optional[dict[str, Any]] = None,
            **kwargs: Any,
        ) -> None:
            chain_name = serialized.get("name", serialized.get("id", ["chain"])[-1])
            wrapper = self._start_span(run_id, str(chain_name), "agent", parent_run_id)
            wrapper.set_input(_safe_serialize(inputs, max_length=2000))

        def on_chain_end(
            self,
            outputs: dict[str, Any],
            *,
            run_id: uuid.UUID,
            parent_run_id: Optional[uuid.UUID] = None,
            **kwargs: Any,
        ) -> None:
            wrapper = self._spans.get(run_id)
            if wrapper:
                wrapper.set_output(_safe_serialize(outputs, max_length=2000))
            self._end_span(run_id)

        def on_chain_error(
            self,
            error: BaseException,
            *,
            run_id: uuid.UUID,
            parent_run_id: Optional[uuid.UUID] = None,
            **kwargs: Any,
        ) -> None:
            self._end_span(run_id, error=error)

        # ---- LLM ----

        def on_llm_start(
            self,
            serialized: dict[str, Any],
            prompts: list[str],
            *,
            run_id: uuid.UUID,
            parent_run_id: Optional[uuid.UUID] = None,
            **kwargs: Any,
        ) -> None:
            wrapper = self._start_span(run_id, "llm", "llm", parent_run_id)
            wrapper.set_input("\n".join(prompts)[:4000])
            model = ""
            if serialized:
                model = str(
                    serialized.get("kwargs", {}).get("model_name")
                    or serialized.get("name", "")
                )
            if model:
                wrapper.set_attribute("ap.model", model)

        def on_llm_end(
            self,
            response: LLMResult,
            *,
            run_id: uuid.UUID,
            parent_run_id: Optional[uuid.UUID] = None,
            **kwargs: Any,
        ) -> None:
            wrapper = self._spans.get(run_id)
            if wrapper and response.generations:
                texts: list[str] = []
                for gen_list in response.generations:
                    for gen in gen_list:
                        if gen.text:
                            texts.append(gen.text)
                if texts:
                    wrapper.set_output("\n".join(texts)[:4000])
                if response.llm_output:
                    usage = response.llm_output.get("token_usage", {})
                    prompt_tok = int(usage.get("prompt_tokens", 0) or 0)
                    completion_tok = int(usage.get("completion_tokens", 0) or 0)
                    model = str(response.llm_output.get("model_name", ""))
                    wrapper.set_llm(
                        model=model or "unknown",
                        prompt_tokens=prompt_tok,
                        completion_tokens=completion_tok,
                    )
            self._end_span(run_id)

        def on_llm_error(
            self,
            error: BaseException,
            *,
            run_id: uuid.UUID,
            parent_run_id: Optional[uuid.UUID] = None,
            **kwargs: Any,
        ) -> None:
            self._end_span(run_id, error=error)

        # ---- Tool ----

        def on_tool_start(
            self,
            serialized: dict[str, Any],
            input_str: str,
            *,
            run_id: uuid.UUID,
            parent_run_id: Optional[uuid.UUID] = None,
            **kwargs: Any,
        ) -> None:
            tool_name = str(serialized.get("name", "tool"))
            wrapper = self._start_span(run_id, tool_name, "tool", parent_run_id)
            wrapper.set_tool(tool_name, result_preview="")
            wrapper.set_input(input_str[:2000])

        def on_tool_end(
            self,
            output: str,
            *,
            run_id: uuid.UUID,
            parent_run_id: Optional[uuid.UUID] = None,
            **kwargs: Any,
        ) -> None:
            wrapper = self._spans.get(run_id)
            if wrapper:
                wrapper.set_output(output[:2000])
            self._end_span(run_id)

        def on_tool_error(
            self,
            error: BaseException,
            *,
            run_id: uuid.UUID,
            parent_run_id: Optional[uuid.UUID] = None,
            **kwargs: Any,
        ) -> None:
            self._end_span(run_id, error=error)

        # ---- Agent ----

        def on_agent_action(
            self,
            action: AgentAction,
            *,
            run_id: uuid.UUID,
            parent_run_id: Optional[uuid.UUID] = None,
            **kwargs: Any,
        ) -> None:
            wrapper = self._start_span(run_id, "agent_action", "reasoning", parent_run_id)
            wrapper.set_reasoning(
                step=0,
                thought=str(action.log)[:2000] if action.log else "",
                action=f"{action.tool}: {action.tool_input}"[:1000],
            )

        def on_agent_finish(
            self,
            finish: AgentFinish,
            *,
            run_id: uuid.UUID,
            parent_run_id: Optional[uuid.UUID] = None,
            **kwargs: Any,
        ) -> None:
            wrapper = self._spans.get(run_id)
            if wrapper:
                wrapper.set_output(_safe_serialize(finish.return_values, max_length=2000))
            else:
                wrapper = self._start_span(run_id, "agent_finish", "agent", parent_run_id)
                wrapper.set_output(_safe_serialize(finish.return_values, max_length=2000))
            self._end_span(run_id)

    return AgentPulseCallback


AgentPulseCallback = _make_callback_class()

__all__ = ["AgentPulseCallback", "LANGCHAIN_AVAILABLE"]
