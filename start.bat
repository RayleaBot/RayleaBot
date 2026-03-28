@echo off
setlocal

rem Development-only shortcut for building and starting the packaged Electron launcher.
cd /d "%~dp0"

set "LAUNCHER_DIR=%~dp0launcher"
set "LAUNCHER_PACKAGE_DIR=%LAUNCHER_DIR%\dist\package\win-unpacked"
set "LAUNCHER_EXE=%LAUNCHER_PACKAGE_DIR%\RayleaLauncher.exe"

echo [RayleaBot] Installing launcher dependencies...
call pnpm --dir "%LAUNCHER_DIR%" install --frozen-lockfile
if errorlevel 1 (
    echo [RayleaBot] Launcher dependency install failed.
    exit /b 1
)

echo [RayleaBot] Building launcher...
call pnpm --dir "%LAUNCHER_DIR%" build
if errorlevel 1 (
    echo [RayleaBot] Launcher build failed.
    exit /b 1
)

if /I "%RAYLEA_START_SKIP_LAUNCH%"=="1" (
    exit /b 0
)

if not exist "%LAUNCHER_EXE%" (
    echo [RayleaBot] Launcher executable not found: "%LAUNCHER_EXE%"
    exit /b 1
)

echo [RayleaBot] Starting launcher...
start "RayleaBot Launcher" /D "%LAUNCHER_PACKAGE_DIR%" "%LAUNCHER_EXE%"

exit /b 0
