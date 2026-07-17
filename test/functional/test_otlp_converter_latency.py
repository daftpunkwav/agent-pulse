"""功能：通过 go test 验证 OTLP 转换与 PartialSuccess（调度 backend 功能测试）。"""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BACKEND = ROOT / "backend"


def test_go_functional_otlp() -> None:
    r = subprocess.run(
        ["go", "test", "./test/functional/...", "-count=1", "-timeout", "60s"],
        cwd=BACKEND,
        capture_output=True,
        text=True,
    )
    sys.stdout.write(r.stdout)
    sys.stderr.write(r.stderr)
    assert r.returncode == 0, r.stderr or r.stdout
