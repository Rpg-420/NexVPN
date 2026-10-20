@echo off
rem ---------------------------------------------------------------------------
rem  NexVPN-Report launcher.
rem
rem  Double-click this file when NexVPN will not start, or starts and then dies.
rem  It needs nothing from the application — only Windows PowerShell, which ships
rem  with Windows. Keep this file ASCII-only: cmd.exe reads .cmd files using the
rem  console code page, and non-ASCII text here would render as garbage. The
rem  Persian output comes from the PowerShell script, not from this file.
rem ---------------------------------------------------------------------------

rem Switch the console to UTF-8 so the Persian verdict is readable.
chcp 65001 >nul 2>&1

setlocal

powershell.exe -NoProfile -NoLogo -ExecutionPolicy Bypass -File "%~dp0NexVPN-Report.ps1"
set "REPORT_EXIT=%ERRORLEVEL%"

echo.
if not "%REPORT_EXIT%"=="0" (
    echo Report tool exited with code %REPORT_EXIT%.
    echo.
)

echo Press any key to close this window...
pause >nul

endlocal
