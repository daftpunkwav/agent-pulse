$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$Sdk = Join-Path $Root "sdk-python"
Write-Host "== sdk-python package tests =="
Push-Location $Sdk
try {
  python -m pytest tests/ -q --tb=short
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} finally {
  Pop-Location
}
Write-Host "== test/ unit+module (sdk related) =="
Push-Location (Join-Path $Root "test")
try {
  python -m pytest unit/test_sdk_init_env.py unit/test_set_input_any.py module/test_sdk_integrations_pii.py -q --tb=short
  exit $LASTEXITCODE
} finally {
  Pop-Location
}
