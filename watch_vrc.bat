@echo off
REM VRChat photo folder watcher launcher.
REM This batch runs the compiled binary with the watch subcommand.

cd /d "%~dp0"

set "WATCH_ROOT=C:\FURUKAWA\VRChat_pic"
set "EXE=%~dp0VRCSSAnnotationTool.exe"

echo Updating main branch...
git fetch origin main
if errorlevel 1 (
    echo Failed to fetch origin/main.
    pause
    exit /b 1
)

git merge --ff-only origin/main
if errorlevel 1 (
    echo Failed to fast-forward main. Commit or stash local changes, then try again.
    pause
    exit /b 1
)

echo Building VRCSSAnnotationTool.exe...
go build -ldflags "-s -w" -o "%EXE%" .
if errorlevel 1 (
    echo Failed to build VRCSSAnnotationTool.exe.
    pause
    exit /b 1
)

if not exist "%EXE%" (
    echo VRCSSAnnotationTool.exe was not found.
    echo Build it first:
    echo go build -ldflags "-s -w" -o VRCSSAnnotationTool.exe .
    pause
    exit /b 1
)

echo ========================================
echo Start VRChat photo folder watcher
echo Watch root: %WATCH_ROOT%
echo Press Ctrl+C to stop
echo ========================================
echo.

"%EXE%" watch --root "%WATCH_ROOT%"

echo.
echo Watcher stopped.
pause
