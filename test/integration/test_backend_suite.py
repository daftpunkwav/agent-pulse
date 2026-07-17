"""集成：调度 backend/test 全部分层 + internal 关键包测试。"""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path

BACKEND = Path(__file__).resolve().parents[2] / "backend"


def _go_test(*packages: str) -> None:
    r = subprocess.run(
        ["go", "test", *packages, "-count=1", "-timeout", "180s"],
        cwd=BACKEND,
        capture_output=True,
        text=True,
    )
    sys.stdout.write(r.stdout)
    sys.stderr.write(r.stderr)
    assert r.returncode == 0, f"go test failed: {r.stderr or r.stdout}"


def test_backend_layered_tests() -> None:
    _go_test("./test/unit/...", "./test/module/...", "./test/functional/...", "./test/integration/...")


def test_backend_internal_packages() -> None:
    _go_test(
        "./internal/config/",
        "./internal/api/",
        "./internal/service/",
        "./internal/collector/",
        "./internal/domain/",
        "./internal/pii/",
    )
