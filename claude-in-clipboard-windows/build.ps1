# Builds build\ClaudeClip.exe (tray only, no console window) and runs the checks.
# Needs Go 1.22 or newer: https://go.dev/dl/  (winget install GoLang.Go)
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

# A running copy would keep the old .exe locked.
Get-Process ClaudeClip -ErrorAction SilentlyContinue | Stop-Process -Force

go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'go vet failed' }
go test ./...
if ($LASTEXITCODE -ne 0) { throw 'tests failed' }

New-Item -ItemType Directory -Force build | Out-Null
go build -trimpath -ldflags '-H=windowsgui -s -w' -o build\ClaudeClip.exe .
if ($LASTEXITCODE -ne 0) { throw 'build failed' }
Write-Host 'Built build\ClaudeClip.exe'
