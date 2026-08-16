#!/usr/bin/env pwsh
# NexVPN Development Server Script

$goPath = "C:\Program Files\Go\bin"
$nodePath = "C:\Program Files\nodejs"
$wailsPath = "$env:GOPATH\bin"

if (-not $env:GOPATH) {
    $env:GOPATH = "$env:USERPROFILE\go"
    $wailsPath = "$env:GOPATH\bin"
}

$env:PATH = "$goPath;$nodePath;$wailsPath;" + $env:PATH

Write-Host "NexVPN Development Server" -ForegroundColor Cyan
Write-Host "=========================" -ForegroundColor Cyan
Write-Host ""

& "$wailsPath\wails.exe" dev
