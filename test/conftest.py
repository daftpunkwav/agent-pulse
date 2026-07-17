"""AgentPulse 根测试 conftest。"""

from __future__ import annotations

import sys
from pathlib import Path

# 允许从仓库根引用 sdk-python/src
ROOT = Path(__file__).resolve().parents[1]
SDK_SRC = ROOT / "sdk-python" / "src"
if str(SDK_SRC) not in sys.path:
    sys.path.insert(0, str(SDK_SRC))
