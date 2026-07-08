"""tests/test_basic.py - SDK 基础测试。"""

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


def test_decorators():
    """测试装饰器能正常包装函数（不强制 OTLP 接收）。"""
    from agentpulse import observe

    @observe(agent_name="test-agent")
    def my_func(x: int) -> int:
        return x * 2

    # 不实际执行（避免 OTLP 网络调用）
    assert my_func.__name__ == "my_func"


def test_safe_serialize():
    """测试安全序列化。"""
    from agentpulse.decorators import _safe_serialize

    assert _safe_serialize("hello") == "hello"
    assert _safe_serialize(123) == "123"
    assert _safe_serialize({"a": 1}) == '{"a": 1}'
    assert _safe_serialize([1, 2, 3]) == "[1, 2, 3]"
    assert _safe_serialize(None) == ""
    # 长字符串截断
    long_str = "x" * 2000
    assert len(_safe_serialize(long_str, max_length=100)) == 100


def test_otlp_endpoint_building():
    """测试 OTLP endpoint 构造逻辑。"""
    client = Client(ClientConfig(endpoint="http://localhost:8080"))
    endpoint = client._build_otlp_endpoint()
    assert ":4318" in endpoint
    assert endpoint.endswith("/v1/traces")

    # 自定义端口
    client2 = Client(ClientConfig(endpoint="http://otel-collector:4318"))
    endpoint2 = client2._build_otlp_endpoint()
    assert endpoint2.endswith("/v1/traces")


if __name__ == "__main__":
    pytest.main([__file__, "-v"])