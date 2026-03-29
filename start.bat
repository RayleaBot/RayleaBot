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

rem We run via 'electron .' during development to ensure we use the fresh dist/ build
echo [RayleaBot] Starting launcher...
start "" "%LAUNCHER_DIR%\node_modules\electron\dist\electron.exe" "%LAUNCHER_DIR%"
exit /b 0
