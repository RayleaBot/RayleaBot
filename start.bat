@echo off
setlocal

rem Development-only shortcut for building and starting the local Electron launcher.
cd /d "%~dp0"

set "LAUNCHER_DIR=%~dp0launcher"
set "LAUNCHER_ENTRY=."

echo [RayleaBot] Installing launcher dependencies...
call pnpm --dir "%LAUNCHER_DIR%" install --frozen-lockfile
if errorlevel 1 (
    echo [RayleaBot] Launcher dependency install failed.
    exit /b 1
)

echo [RayleaBot] Building launcher...
call pnpm --dir "%LAUNCHER_DIR%" run build:app
if errorlevel 1 (
    echo [RayleaBot] Launcher build failed.
    exit /b 1
)

if /I "%RAYLEA_START_SKIP_LAUNCH%"=="1" (
    exit /b 0
)

if not exist "%LAUNCHER_DIR%\dist\main\main\index.js" (
    echo [RayleaBot] Launcher main bundle not found: "%LAUNCHER_DIR%\dist\main\main\index.js"
    exit /b 1
)

echo [RayleaBot] Starting launcher...
call pnpm --dir "%LAUNCHER_DIR%" exec electron "%LAUNCHER_ENTRY%"

exit /b 0
