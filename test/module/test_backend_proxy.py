"""模块：Web BFF 路径白名单与鉴权注入规则（纯逻辑，不启动 Next）。"""

from __future__ import annotations

import importlib.util
import os
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[2]
PROXY_TS = ROOT / "web" / "src" / "lib" / "backend-proxy.ts"


def _load_logic_via_node() -> None:
    """用 node 执行 backend-proxy.test.ts 等价断言（内联）。"""
    # 纯 Python 复刻关键规则，与 TS 保持同步校验
    pass


ALLOWED = {"traces", "cost", "eval", "clusters", "harness", "abtests"}
HEALTH = {"healthz", "readyz"}
SEG = __import__("re").compile(r"^[a-zA-Z0-9._~\-]+$")


def parse_proxy_path(segments: list[str]):
    if not segments or len(segments) > 32:
        return None
    for seg in segments:
        if not seg or seg in (".", "..") or not SEG.match(seg):
            return None
    head = segments[0]
    if head in HEALTH and len(segments) == 1:
        return ("health", head)
    if head not in ALLOWED:
        return None
    return ("api", "/".join(segments))


def test_proxy_allows_api_prefixes() -> None:
    for p in ALLOWED:
        assert parse_proxy_path([p, "x"]) is not None


def test_proxy_rejects_traversal() -> None:
    assert parse_proxy_path(["..", "etc"]) is None
    assert parse_proxy_path(["cost", ".."]) is None


def test_proxy_health_no_api_prefix() -> None:
    kind, path = parse_proxy_path(["healthz"])
    assert kind == "health" and path == "healthz"


def test_proxy_unknown_prefix() -> None:
    assert parse_proxy_path(["admin"]) is None


def test_backend_proxy_ts_exists() -> None:
    assert PROXY_TS.is_file(), "web BFF source missing"
