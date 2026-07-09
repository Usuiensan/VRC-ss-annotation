@echo off
REM VRChat photo folder watcher launcher.
REM This batch runs the current source code with the watch subcommand.

cd /d "%~dp0"

set "WATCH_ROOT=D:\VRChat_pic"

echo ========================================
echo Start VRChat photo folder watcher
echo Watch root: %WATCH_ROOT%
echo Press Ctrl+C to stop
echo ========================================
echo.

go run . watch --root "%WATCH_ROOT%"

echo.
echo Watcher stopped.
pause
