"""AgentPulse 客户端。

负责：
- 初始化 OpenTelemetry Tracer Provider 与 OTLP Exporter
- 管理全局配置
- 提供获取当前 Span 的接口
- 优雅关闭
"""

from __future__ import annotations

import logging
import os
import re
import threading
from dataclasses import dataclass, field
from typing import Any, Optional
from urllib.parse import urlparse, urlunparse

from opentelemetry.trace import Tracer

from opentelemetry import trace as otel_trace
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter

logger = logging.getLogger(__name__)

_OTLP_HTTP_PORT = 4318
_OTLP_TRACES_SUFFIX = "/v1/traces"
_API_KEY_PATTERN = re.compile(r"^ap-[a-zA-Z0-9_-]{13,}$")


@dataclass
class ClientConfig:
    """客户端配置。"""

    api_key: str = ""
    endpoint: str = "http://localhost:8080"
    service_name: str = "agent-app"
    environment: str = "production"
    sample_rate: float = 1.0
    flush_interval_seconds: float = 5.0
    max_queue_size: int = 2048
    # 透传给后端的 headers（用于 API Key 鉴权等）
    headers: dict[str, str] = field(default_factory=dict)
    # 自定义资源属性
    resource_attributes: dict[str, str] = field(default_factory=dict)

    @classmethod
    def from_env(cls) -> "ClientConfig":
        """从环境变量构造配置。

        支持的环境变量：
        - AGENTPULSE_API_KEY
        - AGENTPULSE_ENDPOINT / OTEL_EXPORTER_OTLP_ENDPOINT
        - AGENTPULSE_SERVICE_NAME / OTEL_SERVICE_NAME
        - AGENTPULSE_ENVIRONMENT
        """
        endpoint = (
            os.environ.get("AGENTPULSE_ENDPOINT")
            or os.environ.get("OTEL_EXPORTER_OTLP_ENDPOINT")
            or "http://localhost:8080"
        )
        service_name = (
            os.environ.get("AGENTPULSE_SERVICE_NAME")
            or os.environ.get("OTEL_SERVICE_NAME")
            or "agent-app"
        )
        return cls(
            api_key=os.environ.get("AGENTPULSE_API_KEY", ""),
            endpoint=endpoint,
            service_name=service_name,
            environment=os.environ.get("AGENTPULSE_ENVIRONMENT", "production"),
        )


class Client:
    """AgentPulse SDK 主客户端。

    单例模式：通过 init() 全局初始化，通过 get_client() 获取实例。

    主要职责：
    - 设置 OpenTelemetry TracerProvider + OTLP Exporter
    - 提供 agentpulse.* 语义约定的便捷方法
    """

    _instance: Optional["Client"] = None
    _lock = threading.Lock()

    def __init__(self, config: ClientConfig):
        self.config = config
        self._initialized = False
        self._tracer_provider: Optional[TracerProvider] = None
        self._tracer: Optional[Tracer] = None

    @classmethod
    def instance(cls) -> "Client":
        """获取全局单例。

        Raises:
            RuntimeError: 未调用 init() 初始化时抛出。
        """
        if cls._instance is None:
            raise RuntimeError(
                "AgentPulse client not initialized; call agentpulse.init() first"
            )
        return cls._instance

    @classmethod
    def set_instance(cls, client: "Client") -> None:
        """设置全局单例（由 init() 调用）。"""
        with cls._lock:
            cls._instance = client

    def initialize(self) -> None:
        """初始化 OpenTelemetry 管道。

        多次调用是幂等的。
        """
        if self._initialized:
            return

        # 构造 OTLP endpoint
        # AgentPulse 后端 OTLP HTTP 接收路径：:4318/v1/traces
        otlp_endpoint = self._build_otlp_endpoint()

        # 构造资源
        resource_attrs = {
            "service.name": self.config.service_name,
            "deployment.environment": self.config.environment,
            **self.config.resource_attributes,
        }
        resource = Resource.create(resource_attrs)

        # 构造 Exporter
        headers = dict(self.config.headers)
        if self.config.api_key:
            headers["X-AgentPulse-Key"] = self.config.api_key

        exporter = OTLPSpanExporter(
            endpoint=otlp_endpoint,
            headers=headers or None,
        )

        # 构造 Provider
        provider = TracerProvider(resource=resource)

        # 使用 BatchSpanProcessor 异步批处理
        processor = BatchSpanProcessor(
            exporter,
            max_queue_size=self.config.max_queue_size,
            schedule_delay_millis=int(self.config.flush_interval_seconds * 1000),
        )
        provider.add_span_processor(processor)

        # 设置全局 TracerProvider
        otel_trace.set_tracer_provider(provider)

        self._tracer_provider = provider
        self._tracer = provider.get_tracer(
            instrumenting_module_name="agentpulse",
            instrumenting_library_version="0.1.0",
        )

        self._initialized = True
        logger.info(
            "AgentPulse client initialized: endpoint=%s service=%s",
            self.config.endpoint,
            self.config.service_name,
        )

    def shutdown(self) -> None:
        """关闭客户端，刷新所有缓冲 Span。"""
        if not self._initialized or self._tracer_provider is None:
            return
        try:
            self._tracer_provider.shutdown()
            logger.debug("AgentPulse client shutdown complete")
        except Exception as exc:  # pylint: disable=broad-except
            logger.warning("Error during AgentPulse shutdown: %s", exc)

    def get_tracer(self) -> Tracer:
        """获取 OpenTelemetry Tracer。"""
        if self._tracer is None:
            self.initialize()
        assert self._tracer is not None
        return self._tracer

    def _build_otlp_endpoint(self) -> str:
        """构造 OTLP HTTP endpoint URL。

        使用 urllib.parse 解析，支持 IPv6；拒绝已含 /v1/traces 的输入。
        非 OTLP 端口（非 4318）时默认替换为 4318。
        """
        raw = self.config.endpoint.strip()
        if not raw:
            raise ValueError("endpoint must not be empty")

        if "://" not in raw:
            raw = f"http://{raw}"

        parsed = urlparse(raw)
        if not parsed.hostname:
            raise ValueError(f"invalid endpoint: {self.config.endpoint!r}")

        path = parsed.path or ""
        normalized_path = path.rstrip("/")

        if normalized_path.endswith(_OTLP_TRACES_SUFFIX):
            return urlunparse(parsed._replace(path=_OTLP_TRACES_SUFFIX))

        if _OTLP_TRACES_SUFFIX in normalized_path:
            raise ValueError(
                "endpoint must be a base URL without /v1/traces; "
                f"got path {path!r}"
            )

        port = parsed.port
        if port is None:
            port = _OTLP_HTTP_PORT
        elif port != _OTLP_HTTP_PORT:
            port = _OTLP_HTTP_PORT

        userinfo = ""
        if parsed.username:
            userinfo = parsed.username
            if parsed.password:
                userinfo += f":{parsed.password}"
            userinfo += "@"

        hostname = parsed.hostname
        if ":" in hostname and not hostname.startswith("["):
            hostname = f"[{hostname}]"

        netloc = f"{userinfo}{hostname}:{port}"
        new_path = f"{normalized_path}{_OTLP_TRACES_SUFFIX}" if normalized_path else _OTLP_TRACES_SUFFIX
        return urlunparse(parsed._replace(netloc=netloc, path=new_path))


# 全局函数式 API

def _validate_api_key(api_key: str) -> None:
    """校验 API Key 格式（非空时必须符合 ap- 前缀 + 长度要求）。"""
    if not api_key:
        return
    if not _API_KEY_PATTERN.match(api_key):
        raise ValueError(
            "api_key must start with 'ap-' and be at least 16 characters "
            f"(got length {len(api_key)})"
        )


def init(
    api_key: str = "",
    endpoint: str = "http://localhost:8080",
    service_name: str = "agent-app",
    environment: str = "production",
    sample_rate: float = 1.0,
    flush_interval_seconds: float = 5.0,
    max_queue_size: int = 2048,
    headers: Optional[dict[str, str]] = None,
    resource_attributes: Optional[dict[str, str]] = None,
    **kwargs: Any,
) -> Client:
    """初始化 AgentPulse 客户端（全局）。

    应在应用启动时调用一次。

    Args:
        api_key: API Key（用于后端鉴权）。
        endpoint: AgentPulse 后端地址。
        service_name: 服务名（用于多服务区分）。
        environment: 部署环境（production/staging/dev）。
        sample_rate: 采样率，0-1 之间。
        flush_interval_seconds: 批量上报间隔。
        headers: 自定义 HTTP headers。
        max_queue_size: Span 队列最大长度。
        resource_attributes: 附加 OTel 资源属性。
        **kwargs: 保留给未来扩展，当前忽略未知键。

    Returns:
        初始化后的 Client 实例。
    """
    _validate_api_key(api_key)
    if kwargs:
        logger.debug("init() received unused kwargs: %s", list(kwargs.keys()))

    config = ClientConfig(
        api_key=api_key,
        endpoint=endpoint,
        service_name=service_name,
        environment=environment,
        sample_rate=sample_rate,
        flush_interval_seconds=flush_interval_seconds,
        max_queue_size=max_queue_size,
        headers=headers or {},
        resource_attributes=resource_attributes or {},
    )
    client = Client(config)
    client.initialize()
    Client.set_instance(client)
    return client


def get_client() -> Client:
    """获取全局客户端实例。

    Raises:
        RuntimeError: 未调用 init() 初始化时抛出。
    """
    return Client.instance()


def shutdown() -> None:
    """关闭全局客户端，刷新所有缓冲 Span。"""
    if Client._instance is None:
        return
    Client._instance.shutdown()
    with Client._lock:
        Client._instance = None