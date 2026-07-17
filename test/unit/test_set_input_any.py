"""单元：set_input/set_output 接受任意类型。"""

from __future__ import annotations

from agentpulse import init, shutdown
from agentpulse.spans import trace


def setup_function() -> None:
    shutdown()
    init(endpoint="http://localhost:8080", service_name="unit-set-input")


def teardown_function() -> None:
    shutdown()


def test_set_input_dict() -> None:
    with trace("t", span_type="tool") as t:
        t.set_input({"tool": "search", "q": "hello"})
        t.set_output({"ok": True})
        t.set_status_ok()


def test_set_input_str() -> None:
    with trace("t2", span_type="llm") as t:
        t.set_input("plain")
        t.set_output("out")
