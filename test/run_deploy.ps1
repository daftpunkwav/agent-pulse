$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
Write-Host "== deploy/k8s integration =="
Push-Location (Join-Path $Root "test")
try {
  python -m pytest integration/test_k8s_manifests.py -q --tb=short
  exit $LASTEXITCODE
} finally {
  Pop-Location
}
