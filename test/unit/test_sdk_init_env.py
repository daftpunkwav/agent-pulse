"""单元：SDK init 环境变量合并与 sample_rate。"""

from __future__ import annotations

import os

from agentpulse import init, shutdown
from agentpulse.client import Client, ClientConfig


def setup_function() -> None:
    shutdown()


def teardown_function() -> None:
    shutdown()
    for k in (
        "AGENTPULSE_API_KEY",
        "AGENTPULSE_ENDPOINT",
        "AGENTPULSE_SERVICE_NAME",
        "AGENTPULSE_ENVIRONMENT",
    ):
        os.environ.pop(k, None)


def test_from_env_reads_variables() -> None:
    os.environ["AGENTPULSE_API_KEY"] = "ap-env-key-12345678"
    os.environ["AGENTPULSE_ENDPOINT"] = "http://env-host:9090"
    os.environ["AGENTPULSE_SERVICE_NAME"] = "from-env"
    cfg = ClientConfig.from_env()
    assert cfg.api_key == "ap-env-key-12345678"
    assert cfg.endpoint == "http://env-host:9090"
    assert cfg.service_name == "from-env"


def test_init_merges_env_when_args_omitted() -> None:
    os.environ["AGENTPULSE_API_KEY"] = "ap-env-key-12345678"
    os.environ["AGENTPULSE_ENDPOINT"] = "http://env-host:9090"
    client = init()  # 全部走 env
    assert client.config.api_key == "ap-env-key-12345678"
    assert client.config.endpoint == "http://env-host:9090"
    assert client._initialized is True


def test_init_explicit_overrides_env() -> None:
    os.environ["AGENTPULSE_API_KEY"] = "ap-env-key-12345678"
    client = init(api_key="ap-explicit-key-01", endpoint="http://explicit:1")
    assert client.config.api_key == "ap-explicit-key-01"
    assert client.config.endpoint == "http://explicit:1"


def test_sample_rate_clamped() -> None:
    client = init(sample_rate=2.5)
    assert 0.0 <= client.config.sample_rate <= 1.0
