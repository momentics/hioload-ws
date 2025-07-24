@echo off
REM ================================================================
REM create_aliases_windows.bat
REM 
REM Creates N loopback IP aliases on Windows.
REM Usage: create_aliases_windows.bat <num_aliases>
REM Example: create_aliases_windows.bat 128
REM 
REM This script uses netsh to add IP addresses 127.0.0.2..127.0.0.<N+1> to loopback.
REM Requires administrative privileges.
REM ================================================================

REM Check for admin privileges
>nul 2>&1 "%SYSTEMROOT%\system32\cacls.exe" "%SYSTEMROOT%\system32\config\system"
if '%errorlevel%' NEQ '0' (
    echo This script requires administrative privileges.
    pause
    exit /b 1
)

REM Parse argument
if "%~1"=="" (
    echo Usage: %~nx0 ^<num_aliases^>
    exit /b 1
)
setlocal ENABLEDELAYEDEXPANSION
set NUM=%~1

REM Base IP
set LOOPBACK_BASE=127.0.0.

REM Loop to add aliases
for /L %%i in (2,1,%NUM%) do (
    set IP_ADDR=!LOOPBACK_BASE!%%i
    echo Adding alias !IP_ADDR!
    netsh interface ip add address "Loopback Pseudo-Interface 1" !IP_ADDR! 255.0.0.0
)
echo Done adding %NUM% aliases.
endlocal
