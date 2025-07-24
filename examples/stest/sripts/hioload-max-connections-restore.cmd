@echo off
REM =========================================================================
REM hioload-ws: FULL RESTORE of Connection and Handle Registry Keys from Backup
REM Author: momentics <momentics@gmail.com>
REM Usage: Run as Administrator. STOP any services/applications using these settings.
REM This script deletes old keys and imports backup files for a clean overwrite.
REM =========================================================================

REM 1. Restore TCP/IP Parameters (MaxUserPort, TcpTimedWaitDelay, etc.)
echo.
echo Deleting HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters ...
reg delete "HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters" /f

echo Importing backup: max-connections.backup ...
reg import max-connections.backup

REM 2. Restore Session Manager SubSystems (handle/user object quota, etc.)
echo.
echo Deleting HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\Session Manager\SubSystems ...
reg delete "HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\Session Manager\SubSystems" /f

echo Importing backup: max-connections-subsys.backup ...
reg import max-connections-subsys.backup

echo.
echo === Restore complete. Please reboot Windows to apply all settings. ===
echo.

REM =========================================================================
REM This procedure ensures all values are reset exactly as in your backup.
REM If either .reg.bkp file is missing/corrupt, examine each section manually.
REM =========================================================================
