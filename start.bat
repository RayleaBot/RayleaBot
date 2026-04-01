@echo off
setlocal

cd /d "%~dp0"

set "WEB_DIR=%~dp0web"
set "SERVER_OUTPUT=%~dp0server\raylea-server.exe"
set "LAUNCHER_DIR=%~dp0launcher"

echo [RayleaBot] Installing web dependencies...
call pnpm --dir "%WEB_DIR%" install --frozen-lockfile
if errorlevel 1 (
    echo [RayleaBot] Web dependency install failed.
    exit /b 1
)

echo [RayleaBot] Building web...
call pnpm --dir "%WEB_DIR%" run build
if errorlevel 1 (
    echo [RayleaBot] Web build failed.
    exit /b 1
)

echo [RayleaBot] Building server...
pushd "%~dp0server"
if errorlevel 1 (
    echo [RayleaBot] Server source directory is unavailable.
    exit /b 1
)

call go build -o "%SERVER_OUTPUT%" ./cmd/raylea-server
set "SERVER_BUILD_EXIT=%errorlevel%"
popd

if not "%SERVER_BUILD_EXIT%"=="0" (
    echo [RayleaBot] Server build failed.
    exit /b 1
)

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

echo [RayleaBot] Starting launcher...
start "" "%LAUNCHER_DIR%\node_modules\electron\dist\electron.exe" "%LAUNCHER_DIR%"
exit /b 0
