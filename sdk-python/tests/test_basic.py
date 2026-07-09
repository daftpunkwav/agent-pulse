"""tests/test_basic.py - SDK 基础测试。"""

import asyncio

import pytest

from agentpulse import init, shutdown
from agentpulse.client import Client, ClientConfig
from agentpulse.spans import Session, current_span, session, span


def test_client_config_default():
    """测试默认配置。"""
    cfg = ClientConfig()
    assert cfg.endpoint == "http://localhost:8080"
    assert cfg.service_name == "agent-app"


def test_client_init():
    """测试客户端初始化。"""
    client = init(
        endpoint="http://localhost:8080",
        service_name="test-app",
    )
    assert client._initialized is True
    shutdown()


def test_instance_raises_without_init():
    """未 init 时 instance() 应抛 RuntimeError。"""
    shutdown()
    with pytest.raises(RuntimeError, match="not initialized"):
        Client.instance()


def test_session_creation():
    """测试 Session 创建与属性。"""
    sess = Session(user_id="u-1", agent_name="test")
    assert sess.user_id == "u-1"
    assert sess.agent_name == "test"
    assert sess.session_id != ""

    attrs = sess.to_attributes()
    assert attrs["ap.session_id"] == sess.session_id
    assert attrs["ap.user_id"] == "u-1"
    assert attrs["ap.agent_name"] == "test"


def test_session_context_manager():
    """测试 Session 上下文管理器。"""
    from agentpulse.spans import get_current_session
    assert get_current_session() is None

    with session(user_id="u-2") as s:
        assert get_current_session() == s
        assert get_current_session().user_id == "u-2"

    assert get_current_session() is None


def test_decorators_wrap():
    """测试装饰器能正常包装函数（不强制 OTLP 接收）。"""
    from agentpulse import observe

    @observe(agent_name="test-agent")
    def my_func(x: int) -> int:
        return x * 2

    assert my_func.__name__ == "my_func"


def test_decorators_async_execution():
    """测试异步装饰器实际执行。"""
    from agentpulse import observe

    init(endpoint="http://localhost:8080", service_name="async-test")

    @observe(agent_name="test-agent", capture_args=True, capture_result=True)
    async def async_func(x: int) -> int:
        return x + 1

    result = asyncio.run(async_func(41))
    assert result == 42
    shutdown()


def test_safe_serialize():
    """测试安全序列化与截断标记。"""
    from agentpulse.decorators import _safe_serialize

    assert _safe_serialize("hello") == "hello"
    assert _safe_serialize(123) == "123"
    assert _safe_serialize({"a": 1}) == '{"a": 1}'
    assert _safe_serialize([1, 2, 3]) == "[1, 2, 3]"
    assert _safe_serialize(None) == ""
    long_str = "x" * 2000
    truncated = _safe_serialize(long_str, max_length=100)
    assert truncated.endswith("...[truncated]")
    assert len(truncated) == 100


def test_redact_sensitive_keys():
    """测试敏感键名脱敏。"""
    from agentpulse.decorators import _safe_serialize

    result = _safe_serialize({"password": "secret123", "name": "alice"})
    assert "***REDACTED***" in result
    assert "secret123" not in result
    assert "alice" in result


def test_otlp_endpoint_building():
    """测试 OTLP endpoint 构造逻辑。"""
    client = Client(ClientConfig(endpoint="http://localhost:8080"))
    endpoint = client._build_otlp_endpoint()
    assert endpoint == "http://localhost:4318/v1/traces"

    client2 = Client(ClientConfig(endpoint="http://otel-collector:4318"))
    assert client2._build_otlp_endpoint() == "http://otel-collector:4318/v1/traces"

    client3 = Client(ClientConfig(endpoint="http://[::1]:8080"))
    assert client3._build_otlp_endpoint() == "http://[::1]:4318/v1/traces"

    client4 = Client(ClientConfig(endpoint="http://collector:4318/v1/traces"))
    assert client4._build_otlp_endpoint() == "http://collector:4318/v1/traces"


def test_otlp_endpoint_rejects_duplicate_path():
    """已含 /v1/traces 中间路径的 endpoint 应拒绝。"""
    client = Client(ClientConfig(endpoint="http://host/api/v1/traces/extra"))
    with pytest.raises(ValueError, match="/v1/traces"):
        client._build_otlp_endpoint()


def test_api_key_validation():
    """测试 API Key 格式校验。"""
    with pytest.raises(ValueError, match="api_key"):
        init(api_key="short")

    client = init(api_key="ap-test-key-1234567", endpoint="http://localhost:8080")
    assert client.config.api_key == "ap-test-key-1234567"
    shutdown()


def test_set_ap_attribute_preserves_numeric_types():
    """测试 ap.* 属性保留数值类型。"""
    from unittest.mock import MagicMock

    from agentpulse.spans import _set_ap_attribute

    span = MagicMock()
    _set_ap_attribute(span, "prompt_tokens", 42)
    span.set_attribute.assert_called_with("ap.prompt_tokens", 42)

    span.reset_mock()
    _set_ap_attribute(span, "ap.model", "gpt-4o")
    span.set_attribute.assert_called_with("ap.model", "gpt-4o")


def test_trace_sets_span_type():
    """测试 trace() 写入 ap.span_type。"""
    from unittest.mock import MagicMock, patch

    from agentpulse.spans import trace as trace_cm

    init(endpoint="http://localhost:8080", service_name="test-span-type")
    mock_span = MagicMock()
    mock_span.is_recording.return_value = True
    mock_span.__enter__ = MagicMock(return_value=mock_span)
    mock_span.__exit__ = MagicMock(return_value=False)

    mock_tracer = MagicMock()
    mock_tracer.start_as_current_span.return_value = mock_span

    with patch("agentpulse.spans.get_client") as mock_get_client:
        mock_get_client.return_value.get_tracer.return_value = mock_tracer
        with trace_cm("llm-call", span_type="llm"):
            pass

    mock_span.set_attribute.assert_any_call("ap.span_type", "llm")
    shutdown()


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
