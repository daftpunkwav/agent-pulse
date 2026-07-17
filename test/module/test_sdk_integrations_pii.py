"""模块：集成默认不采集内容（PII 安全默认）。"""

from __future__ import annotations

from agentpulse import init, shutdown
from agentpulse.integrations.autogen import AgentPulseAutoGenHook
from agentpulse.integrations.langchain_callback import LANGCHAIN_AVAILABLE


def setup_module() -> None:
    init(endpoint="http://localhost:8080", service_name="mod-integrations")


def teardown_module() -> None:
    shutdown()


def test_autogen_default_capture_false() -> None:
    hook = AgentPulseAutoGenHook(agent_name="a")
    assert hook.capture_content is False

    class Dummy:
        def send(self, msg: str) -> str:
            return "ok"

    wrapped = hook.wrap_agent(Dummy())
    assert wrapped.send("secret") == "ok"


def test_langchain_default_capture_false() -> None:
    if not LANGCHAIN_AVAILABLE:
        return
    from agentpulse.integrations.langchain_callback import AgentPulseCallback

    cb = AgentPulseCallback(agent_name="lc")
    assert cb.capture_content is False
