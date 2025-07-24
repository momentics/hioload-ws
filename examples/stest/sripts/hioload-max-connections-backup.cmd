@echo off
REM =========================================================================
REM hioload-ws: Registry Key Backup Script
REM Author: momentics <momentics@gmail.com>
REM License: Apache-2.0
REM
REM This script exports the Tcpip Parameters registry key to a backup file.
REM Always run as Administrator.
REM =========================================================================

REM --- Backup the TCP/IP Parameters key (contains MaxUserPort, etc) ---
reg export "HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters" max-connections.backup /y

REM --- Optional: Also backup the Session Manager SubSystems key (handle quotas) ---
reg export "HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\Session Manager\SubSystems" max-connections-subsys.backup /y

echo.
echo TCP/IP Parameters branch backup complete: max-connections.backup
echo Session Manager SubSystems backup complete: max-connections-subsys.backup
echo.

REM =========================================================================
REM NOTES:
REM - The file hioload-max-connections.reg.bkp contains a full export of all TCP/IP tuning values, including:
REM     MaxUserPort, TcpTimedWaitDelay, TCPNumConnections, MaxFreeTcbs, etc.
REM - If you need to restore: double-click the backup .reg file or use 'reg import ...'
REM - You must run this script with Administrator privileges.
REM - For maximum safety, use a different backup file for each registry branch.
REM =========================================================================
