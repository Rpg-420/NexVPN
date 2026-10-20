#!/usr/bin/env pwsh
# NexVPN Production Build Script
#
# Produces build\bin\NexVPN.exe.
#
# Exit code is 0 ONLY when the binary was actually written. Every step is checked
# explicitly, because the previous version relied on $LASTEXITCODE alone: when
# `& "$wailsPath\wails.exe" ...` failed with CommandNotFoundException, $LASTEXITCODE
# still held the 0 from the earlier successful `npm run build`, so the script
# printed "Build successful!" even though no toolchain was installed and the
# binary on disk was a month old.

$root = if ($PSScriptRoot) { $PSScriptRoot } else { (Get-Location).Path }

# --- Toolchain discovery ---------------------------------------------------
# Go is not installed system-wide on this machine; a portable toolchain lives
# under the WorkBuddy binaries directory. The standard installer locations are
# kept as fallbacks so the script also works on an ordinary dev box.
$goBin = @(
    "$env:USERPROFILE\.workbuddy-ai\binaries\go\bin",
    "$env:ProgramFiles\Go\bin",
    "C:\Go\bin"
) | Where-Object { Test-Path (Join-Path $_ "go.exe") } | Select-Object -First 1

$nodeBin  = "C:\Program Files\nodejs"
$toolsBin = "$env:USERPROFILE\go\bin"   # `go install` output directory

$prefix = (@($goBin, $nodeBin, $toolsBin) | Where-Object { $_ }) -join ";"
if ($prefix) { $env:PATH = "$prefix;$env:PATH" }

Write-Host "NexVPN Production Build" -ForegroundColor Cyan
Write-Host "=======================" -ForegroundColor Cyan
Write-Host ""

# --- Preflight: fail loudly BEFORE doing any work --------------------------
$goExe     = if ($goBin) { Join-Path $goBin "go.exe" } else { $null }
$wailsExe  = Join-Path $toolsBin "wails.exe"
$winresExe = Join-Path $toolsBin "go-winres.exe"

$missing = @()
if (-not $goExe -or -not (Test-Path $goExe)) {
    $missing += "Go toolchain    (looked in: $env:USERPROFILE\.workbuddy-ai\binaries\go\bin, $env:ProgramFiles\Go\bin, C:\Go\bin)"
}
if (-not (Test-Path $wailsExe))  { $missing += "wails CLI       -> $wailsExe" }
if (-not (Test-Path $winresExe)) { $missing += "go-winres       -> $winresExe" }

if ($missing.Count -gt 0) {
    Write-Host "Preflight failed - missing build tools:" -ForegroundColor Red
    $missing | ForEach-Object { Write-Host "  $_" -ForegroundColor Red }
    Write-Host ""
    Write-Host "Install them with:" -ForegroundColor Yellow
    Write-Host "  go install github.com/wailsapp/wails/v2/cmd/wails@v2.9.3"
    Write-Host "  go install github.com/tc-hib/go-winres@latest"
    Write-Host ""
    Write-Host "Nothing was built." -ForegroundColor Red
    exit 1
}

Write-Host "Go:      $goExe" -ForegroundColor DarkGray
Write-Host "wails:   $wailsExe" -ForegroundColor DarkGray
Write-Host "winres:  $winresExe" -ForegroundColor DarkGray
Write-Host ""

# --- Helpers ---------------------------------------------------------------
function Invoke-Step {
    param([string]$Label, [scriptblock]$Action)
    Write-Host ">> $Label" -ForegroundColor Cyan
    & $Action
    if ($LASTEXITCODE -ne 0) {
        Write-Host ""
        Write-Host "$Label FAILED (exit $LASTEXITCODE)." -ForegroundColor Red
        exit 1
    }
}

# --- Build -----------------------------------------------------------------
Push-Location (Join-Path $root "frontend")
try {
    Invoke-Step "Frontend build (npm run build)" { npm run build }
} finally {
    Pop-Location
}

Push-Location $root
try {
    Invoke-Step "Windows resources (go-winres)" {
        # This writes rsrc_windows_386.syso / rsrc_windows_amd64.syso into the
        # repository root. They are gitignored build artifacts, but they are what
        # embeds the application manifest — including the
        # "require administrator" execution level that TUN mode needs.
        #
        # Side effect worth knowing: any *.syso in a package directory is linked
        # into that package's test binary too, so while these files exist
        # `go test ./...` builds a root-package test exe that Windows refuses to
        # launch from a non-elevated shell ("The requested operation requires
        # elevation"). That is unavoidable — Go has no per-target resource
        # filtering — and the manifest is not optional, so the trade is taken
        # deliberately. Run the root package's tests from an elevated shell, or
        # delete the .syso files first (the next build regenerates them).
        & $winresExe make --in winres\winres.json
    }
    Invoke-Step "Wails application build" {
        # -s / --skipfrontend: the frontend was already compiled by the step
        # above, so wails must NOT run `npm install` + `npm run build` a second
        # time. Doing so doubled the build time and re-emptied frontend/dist,
        # which is exactly the directory the previous step had just produced.
        #
        # Binding generation is attempted first and is allowed to fail in exactly
        # one way. Wails generates bindings by compiling a helper
        # (wailsbindings.exe) and then *running* it — and that helper inherits
        # this app's manifest, which requests Administrator. A non-elevated shell
        # therefore cannot launch it:
        #
        #   fork/exec ...\wailsbindings.exe: The requested operation requires
        #   elevation.
        #
        # The frontend-facing API does not change in an ordinary build, so the
        # committed bindings stay valid and -skipbindings is the correct recovery.
        # Regenerate them deliberately (from an elevated shell) after adding or
        # changing a method on App.
        $wailsLog = & $wailsExe build -nopackage -s -o NexVPN.exe 2>&1
        $wailsLog | Write-Host
        if ($LASTEXITCODE -ne 0 -and ($wailsLog -match 'requires elevation')) {
            Write-Host ""
            Write-Host "Binding generation needs elevation (the app manifest requests Administrator)." -ForegroundColor Yellow
            Write-Host "Retrying with -skipbindings; frontend/wailsjs is left unchanged." -ForegroundColor Yellow
            Write-Host ""
            & $wailsExe build -nopackage -s -skipbindings -o NexVPN.exe
        }
    }
} finally {
    Pop-Location
}

# --- Verify the artifact actually exists -----------------------------------
$exe = Join-Path $root "build\bin\NexVPN.exe"
if (-not (Test-Path $exe)) {
    Write-Host ""
    Write-Host "Build reported success but $exe does not exist." -ForegroundColor Red
    exit 1
}

# --- Ship the standalone diagnostic next to the executable ------------------
# NexVPN-Report has to be reachable without the application running, so it is
# copied beside NexVPN.exe rather than embedded in the binary. If it is missing,
# a user whose app will not start has no way to produce a usable report.
$binDir = Join-Path $root "build\bin"
foreach ($tool in @("NexVPN-Report.ps1", "NexVPN-Report.cmd")) {
    $src = Join-Path $root "tools\$tool"
    if (-not (Test-Path $src)) {
        Write-Host ""
        Write-Host "Missing diagnostic tool: $src" -ForegroundColor Red
        exit 1
    }
    Copy-Item -LiteralPath $src -Destination (Join-Path $binDir $tool) -Force
}

# The PowerShell script MUST keep its UTF-8 BOM: PowerShell 5.1 reads a .ps1
# without one as ANSI and every Persian string becomes mojibake.
$reportPs1 = Join-Path $binDir "NexVPN-Report.ps1"
$firstBytes = [System.IO.File]::ReadAllBytes($reportPs1)[0..2]
if (-not ($firstBytes[0] -eq 0xEF -and $firstBytes[1] -eq 0xBB -and $firstBytes[2] -eq 0xBF)) {
    Write-Host ""
    Write-Host "NexVPN-Report.ps1 is missing its UTF-8 BOM; Persian output would be garbled." -ForegroundColor Red
    exit 1
}

# --- Verify the manifest is actually embedded -------------------------------
# TUN mode is the default mode, and it can only configure a Wintun adapter from
# an elevated process. Without "require administrator" in the manifest every TUN
# core dies with "configure tun interface: Access is denied", which the engine
# then misreads as a dead server. A build that silently loses the manifest is
# therefore worse than a failed build.
$exeBytes = [System.IO.File]::ReadAllBytes($exe)
$exeText = [System.Text.Encoding]::ASCII.GetString($exeBytes)
if ($exeText -notmatch 'requireAdministrator') {
    Write-Host ""
    Write-Host "The built executable has no 'require administrator' manifest." -ForegroundColor Red
    Write-Host "TUN mode cannot work without it. Did go-winres run?" -ForegroundColor Red
    exit 1
}

$info = Get-Item $exe
Write-Host ""
Write-Host "Build successful!" -ForegroundColor Green
Write-Host ("Binary: {0}" -f $exe) -ForegroundColor Green
Write-Host ("Size:   {0:N1} MB" -f ($info.Length / 1MB)) -ForegroundColor Green
Write-Host ("Built:  {0}" -f $info.LastWriteTime) -ForegroundColor Green
Write-Host ("Report: {0}" -f $reportPs1) -ForegroundColor Green
exit 0
