$ErrorActionPreference = "Stop"
$Here = $PSScriptRoot
Write-Host "======== AgentPulse full test suite ========"
& "$Here\run_backend.ps1"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
& "$Here\run_sdk.ps1"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
& "$Here\run_web.ps1"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
& "$Here\run_deploy.ps1"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "======== functional + integration (python) ========"
Push-Location $Here
try {
  python -m pytest functional/ integration/ -q --tb=short
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} finally {
  Pop-Location
}
Write-Host "======== ALL PASSED ========"
