#!/usr/bin/env pwsh
# NexVPN Production Build Script

$goPath = "C:\Program Files\Go\bin"
$nodePath = "C:\Program Files\nodejs"
$wailsPath = "$env:USERPROFILE\go\bin"

$env:PATH = "$goPath;$nodePath;$wailsPath;" + $env:PATH

Write-Host "NexVPN Production Build" -ForegroundColor Cyan
Write-Host "=======================" -ForegroundColor Cyan
Write-Host ""

Set-Location frontend
npm run build
Set-Location ..

# Generate Windows native multi-resolution icon & manifest resources
& "$wailsPath\go-winres.exe" make --in winres\winres.json

& "$wailsPath\wails.exe" build -nopackage -o NexVPN.exe

if ($LASTEXITCODE -eq 0) {
    Write-Host ""
    Write-Host "Build successful!" -ForegroundColor Green
    Write-Host "Binary: build\bin\NexVPN.exe" -ForegroundColor Green
} else {
    Write-Host "Build failed!" -ForegroundColor Red
    exit 1
}
