"""集成：K8s kustomize 产物含密钥外置与 Backend 连接 env。"""

from __future__ import annotations

import shutil
import subprocess
import tempfile
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[2]
K8S_BASE = ROOT / "deploy" / "k8s" / "base"
K8S_PROD = ROOT / "deploy" / "k8s" / "overlays" / "production"


@pytest.fixture(scope="module")
def kubectl() -> str:
    path = shutil.which("kubectl")
    if not path:
        pytest.skip("kubectl not installed")
    return path


def test_base_kustomize_has_backend_env(kubectl: str) -> None:
    r = subprocess.run(
        [kubectl, "kustomize", str(K8S_BASE)],
        capture_output=True,
        text=True,
    )
    # commonLabels 弃用会写 stderr，仍应成功
    assert r.returncode == 0, r.stderr
    out = r.stdout
    assert "AGENTPULSE_POSTGRES_PASSWORD" in out
    assert "AGENTPULSE_CLICKHOUSE_PASSWORD" in out
    assert "AGENTPULSE_CHROMA_API_KEY" in out
    assert "AGENTPULSE_JUDGE_API_KEY" in out
    # 明文 Secret 不应再出现在 base
    assert "changeme-replace-in-production" not in out
    assert "ap-dev-key-replace-me-01" not in out


def test_chroma_uses_pvc_and_auth(kubectl: str) -> None:
    r = subprocess.run(
        [kubectl, "kustomize", str(K8S_BASE)],
        capture_output=True,
        text=True,
    )
    assert r.returncode == 0, r.stderr
    assert "agentpulse-chroma-data" in r.stdout
    assert "CHROMA_SERVER_AUTHN_PROVIDER" in r.stdout
    assert "persistentVolumeClaim" in r.stdout


def test_production_secret_generator(kubectl: str) -> None:
    with tempfile.TemporaryDirectory() as td:
        env = Path(td) / "secrets.env"
        env.write_text(
            "\n".join(
                [
                    "postgres-password=strong-pg-password-01",
                    "clickhouse-password=strong-ch-password-01",
                    "chroma-token=strong-chroma-token-01",
                    "api-keys=ap-prod-test-key-0001",
                    "judge-api-key=sk-test-judge-key-0001",
                ]
            ),
            encoding="utf-8",
        )
        # 复制 overlay 到临时目录以免污染工作区
        # 使用 --load-restrictor 或 cwd 到 production 并临时 secrets.env
        prod_env = K8S_PROD / "secrets.env"
        prod_env.write_text(env.read_text(encoding="utf-8"), encoding="utf-8")
        try:
            r = subprocess.run(
                [kubectl, "kustomize", str(K8S_PROD)],
                capture_output=True,
                text=True,
            )
            assert r.returncode == 0, r.stderr
            assert "kind: Secret" in r.stdout
            assert "agentpulse-secrets" in r.stdout
            assert "AGENTPULSE_POSTGRES_PASSWORD" in r.stdout
        finally:
            if prod_env.exists():
                prod_env.unlink()


def test_secrets_example_exists() -> None:
    assert (K8S_BASE / "secrets.example.yaml").is_file()
    assert (K8S_PROD / "secrets.env.example").is_file()
