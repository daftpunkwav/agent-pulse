$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
if (-not $Root) { $Root = (Resolve-Path "$PSScriptRoot\..").Path }
$Backend = Join-Path $Root "backend"
Write-Host "== backend layered tests =="
Push-Location $Backend
try {
  go test ./test/unit/... ./test/module/... ./test/functional/... ./test/integration/... -count=1 -timeout 180s
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
  Write-Host "== backend internal packages =="
  go test ./internal/config/ ./internal/api/ ./internal/service/ ./internal/collector/ ./internal/domain/ ./internal/pii/ -count=1 -timeout 180s
  exit $LASTEXITCODE
} finally {
  Pop-Location
}
