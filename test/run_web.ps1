$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$Web = Join-Path $Root "web"
Write-Host "== web typecheck =="
Push-Location $Web
try {
  npx tsc --noEmit
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
  Write-Host "== web backend-proxy unit =="
  npx --yes tsx src/lib/backend-proxy.test.ts
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} finally {
  Pop-Location
}
Write-Host "== test/module proxy rules =="
Push-Location (Join-Path $Root "test")
try {
  python -m pytest module/test_backend_proxy.py -q --tb=short
  exit $LASTEXITCODE
} finally {
  Pop-Location
}
