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
import threading
from dataclasses import dataclass, field
from typing import Any, Optional

from opentelemetry import trace as otel_trace
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter

logger = logging.getLogger(__name__)


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
        - AGENTPULSE_ENDPOINT
        - AGENTPULSE_SERVICE_NAME
        - AGENTPULSE_ENVIRONMENT
        """
        return cls(
            api_key=os.environ.get("AGENTPULSE_API_KEY", ""),
            endpoint=os.environ.get("AGENTPULSE_ENDPOINT", "http://localhost:8080"),
            service_name=os.environ.get("AGENTPULSE_SERVICE_NAME", "agent-app"),
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
        self._tracer = None

    @classmethod
    def instance(cls) -> "Client":
        """获取全局单例（未初始化时自动创建默认实例）。"""
        if cls._instance is None:
            with cls._lock:
                if cls._instance is None:
                    cls._instance = cls(ClientConfig())
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

    def get_tracer(self):
        """获取 OpenTelemetry Tracer。"""
        if self._tracer is None:
            self.initialize()
        return self._tracer

    def _build_otlp_endpoint(self) -> str:
        """构造 OTLP HTTP endpoint URL。

        默认端口为 4318（与 AgentPulse 后端配置一致）。
        """
        endpoint = self.config.endpoint.rstrip("/")
        # 如果 endpoint 没有指定端口，使用默认 OTLP HTTP 端口 4318
        if not endpoint.endswith(":4318") and ":4317" not in endpoint:
            # 如果只有 host:port（如 :8080），替换为 :4318
            if ":" in endpoint.split("//")[-1]:
                # 有端口，但可能是 8080（API 端口），不是 OTLP 端口
                host_part = endpoint.split("//")[-1]
                if ":" in host_part:
                    host = host_part.rsplit(":", 1)[0]
                    endpoint = f"{endpoint.split('://')[0]}://{host}:4318"
            else:
                endpoint = f"{endpoint}:4318"
        return f"{endpoint}/v1/traces"


# 全局函数式 API

def init(
    api_key: str = "",
    endpoint: str = "http://localhost:8080",
    service_name: str = "agent-app",
    environment: str = "production",
    sample_rate: float = 1.0,
    flush_interval_seconds: float = 5.0,
    headers: Optional[dict[str, str]] = None,
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

    Returns:
        初始化后的 Client 实例。
    """
    config = ClientConfig(
        api_key=api_key,
        endpoint=endpoint,
        service_name=service_name,
        environment=environment,
        sample_rate=sample_rate,
        flush_interval_seconds=flush_interval_seconds,
        headers=headers or {},
    )
    client = Client(config)
    client.initialize()
    Client.set_instance(client)
    return client


def get_client() -> Client:
    """获取全局客户端实例。

    若未初始化则使用默认配置创建。
    """
    return Client.instance()


def shutdown() -> None:
    """关闭全局客户端，刷新所有缓冲 Span。"""
    client = Client.instance()
    client.shutdown()