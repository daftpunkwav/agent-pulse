"""integrations 回归测试（不依赖 langchain-core 安装时仍覆盖结构与泄漏逻辑）。"""

from __future__ import annotations

import uuid
from typing import Any
from unittest.mock import MagicMock, patch

import pytest

from agentpulse import init, shutdown
from agentpulse.integrations.autogen import AgentPulseAutoGenHook
from agentpulse.integrations.langchain_callback import LANGCHAIN_AVAILABLE


def setup_module() -> None:
    init(endpoint="http://localhost:8080", service_name="test-integrations")


def teardown_module() -> None:
    shutdown()


def test_autogen_default_no_content_capture() -> None:
    hook = AgentPulseAutoGenHook(agent_name="a")
    assert hook.capture_content is False

    calls: list[tuple[str, Any]] = []

    class Dummy:
        def send(self, msg: str) -> str:
            return f"echo:{msg}"

    agent = Dummy()
    wrapped = hook.wrap_agent(agent)

    with patch("agentpulse.spans.SpanWrapper.set_input", side_effect=lambda *a, **k: calls.append(("in", a))):
        with patch("agentpulse.spans.SpanWrapper.set_output", side_effect=lambda *a, **k: calls.append(("out", a))):
            result = wrapped.send("secret-pii")

    assert result == "echo:secret-pii"
    assert calls == [], "default must not capture input/output"


def test_autogen_capture_content_opt_in() -> None:
    hook = AgentPulseAutoGenHook(agent_name="a", capture_content=True)

    class Dummy:
        def send(self, msg: str) -> str:
            return "ok"

    wrapped = hook.wrap_agent(Dummy())
    # 仅验证可运行且 opt-in 不抛错
    assert wrapped.send("x") == "ok"


@pytest.mark.skipif(not LANGCHAIN_AVAILABLE, reason="langchain-core not installed")
def test_langchain_start_span_no_leak_on_overwrite() -> None:
    from agentpulse.integrations.langchain_callback import AgentPulseCallback

    cb = AgentPulseCallback(agent_name="lc", capture_content=False)
    run_id = uuid.uuid4()
    parent = uuid.uuid4()

    # 第一次 start
    w1 = cb._start_span(run_id, "chain", "agent")
    assert run_id in cb._spans
    # 第二次同 id：应关闭旧 span 并替换
    w2 = cb._start_span(run_id, "chain2", "agent")
    assert cb._spans[run_id] is w2
    assert w1 is not w2
    # end 后清空
    cb._end_span(run_id)
    assert run_id not in cb._spans


@pytest.mark.skipif(not LANGCHAIN_AVAILABLE, reason="langchain-core not installed")
def test_langchain_agent_action_uses_unique_id() -> None:
    from agentpulse.integrations.langchain_callback import AgentPulseCallback
    from langchain_core.agents import AgentAction

    cb = AgentPulseCallback(agent_name="lc", capture_content=False)
    run_id = uuid.uuid4()
    chain = cb._start_span(run_id, "agent", "agent")
    assert cb._spans[run_id] is chain

    action = AgentAction(tool="search", tool_input="q", log="think")
    cb.on_agent_action(action, run_id=run_id)

    # agent_action 不得覆盖 chain span
    assert cb._spans.get(run_id) is chain
    # action span 已即时结束，不应残留在 map（除 chain 外）
    assert len(cb._spans) == 1

    cb._end_span(run_id)


@pytest.mark.skipif(not LANGCHAIN_AVAILABLE, reason="langchain-core not installed")
def test_langchain_capture_content_default_false() -> None:
    from agentpulse.integrations.langchain_callback import AgentPulseCallback

    cb = AgentPulseCallback()
    assert cb.capture_content is False
