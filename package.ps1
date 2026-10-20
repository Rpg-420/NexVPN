#!/usr/bin/env pwsh
# NexVPN Production Release Packaging Script v1.4.0

$ErrorActionPreference = "Stop"

$goPath = "C:\Program Files\Go\bin"
$nodePath = "C:\Program Files\nodejs"
$wailsPath = "$env:USERPROFILE\go\bin"

$env:PATH = "$goPath;$nodePath;$wailsPath;" + $env:PATH

Write-Host "========================================" -ForegroundColor Cyan
Write-Host " NexVPN Production Release Packaging" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 1. Build Frontend
Write-Host "[1/4] Building Frontend Assets..." -ForegroundColor Yellow
Set-Location "$PSScriptRoot\frontend"
npm run build
if ($LASTEXITCODE -ne 0) {
    Write-Host "Frontend build failed!" -ForegroundColor Red
    exit 1
}
Set-Location $PSScriptRoot

# 2. Build Wails Binary
Write-Host "[2/4] Compiling NexVPN Binary via Wails..." -ForegroundColor Yellow
& "$wailsPath\wails.exe" build -nopackage -o NexVPN.exe
if ($LASTEXITCODE -ne 0) {
    Write-Host "Wails build failed!" -ForegroundColor Red
    exit 1
}

# 3. Create Distribution Folder
Write-Host "[3/4] Assembling Release Bundle..." -ForegroundColor Yellow
$distDir = "$PSScriptRoot\dist\NexVPN-v1.4.0-Windows-x64"
if (Test-Path $distDir) {
    Remove-Item -Recurse -Force $distDir -ErrorAction SilentlyContinue
}
New-Item -ItemType Directory -Path "$distDir\bin" -Force | Out-Null

Copy-Item "$PSScriptRoot\build\bin\NexVPN.exe" "$distDir\NexVPN.exe" -Force

# Copy core binaries & routing databases & DLLs
$binFiles = @(
    "xray.exe", 
    "sing-box.exe", 
    "openvpn.exe", 
    "wintun.dll", 
    "libcrypto-1_1-x64.dll", 
    "libssl-1_1-x64.dll", 
    "libpkcs11-helper-1.dll", 
    "geoip.dat", 
    "geosite.dat"
)

foreach ($f in $binFiles) {
    if (Test-Path "$PSScriptRoot\bin\$f") {
        Copy-Item "$PSScriptRoot\bin\$f" "$distDir\bin\$f" -Force
        Copy-Item "$PSScriptRoot\bin\$f" "$distDir\$f" -Force
    }
}

if (Test-Path "$PSScriptRoot\README.md") {
    Copy-Item "$PSScriptRoot\README.md" "$distDir\" -Force
}
if (Test-Path "$PSScriptRoot\LICENSE") {
    Copy-Item "$PSScriptRoot\LICENSE" "$distDir\" -Force
}

# 4. Create ZIP Archive
Write-Host "[4/4] Creating Portable ZIP Package..." -ForegroundColor Yellow
$zipPath = "$PSScriptRoot\dist\NexVPN-v1.4.0-Portable-x64.zip"
if (Test-Path $zipPath) {
    Remove-Item -Force $zipPath -ErrorAction SilentlyContinue
}
Compress-Archive -Path "$distDir\*" -DestinationPath $zipPath -CompressionLevel Optimal

Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host " Build & Release Packaging Complete! 🎉" -ForegroundColor Green
Write-Host " Portable Folder: $distDir" -ForegroundColor Green
Write-Host " Portable ZIP:    $zipPath" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
