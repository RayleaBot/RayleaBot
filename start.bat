@echo off
setlocal

cd /d "%~dp0"

set "WEB_DIR=%~dp0web"
set "SERVER_OUTPUT=%~dp0server\raylea-server.exe"
set "LAUNCHER_DIR=%~dp0launcher"
if not defined RAYLEA_START_WEB_MODE set "RAYLEA_START_WEB_MODE=dev"

echo [RayleaBot] Installing web dependencies...
call pnpm --dir "%WEB_DIR%" install --frozen-lockfile
if errorlevel 1 (
    echo [RayleaBot] Web dependency install failed.
    exit /b 1
)

if /I "%RAYLEA_START_WEB_MODE%"=="build" (
    set "RAYLEA_WEB_UI_BASE_URL="
    echo [RayleaBot] Building web...
    call pnpm --dir "%WEB_DIR%" run build
    if errorlevel 1 (
        echo [RayleaBot] Web build failed.
        exit /b 1
    )
) else if /I "%RAYLEA_START_WEB_MODE%"=="dev" (
    echo [RayleaBot] Web dev mode enabled.
) else (
    echo [RayleaBot] Unsupported web mode: %RAYLEA_START_WEB_MODE%
    echo [RayleaBot] Use RAYLEA_START_WEB_MODE=dev or RAYLEA_START_WEB_MODE=build.
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

if /I "%RAYLEA_START_WEB_MODE%"=="dev" (
    if not defined VITE_BACKEND_TARGET set "VITE_BACKEND_TARGET=http://127.0.0.1:8080"
    if not defined VITE_WS_BASE_URL set "VITE_WS_BASE_URL=%VITE_BACKEND_TARGET%"
    if not defined RAYLEA_WEB_UI_BASE_URL set "RAYLEA_WEB_UI_BASE_URL=http://127.0.0.1:4173/"
    echo [RayleaBot] Starting web dev server...
    call :START_WEB_DEV_SERVER
    if errorlevel 1 (
        echo [RayleaBot] Web dev server failed to start.
        exit /b 1
    )
)

if /I "%RAYLEA_START_SKIP_LAUNCH%"=="1" (
    exit /b 0
)

if /I "%RAYLEA_START_WEB_MODE%"=="dev" (
    echo [RayleaBot] Waiting for web dev server at %RAYLEA_WEB_UI_BASE_URL%...
    call :WAIT_FOR_WEB_DEV_SERVER
    if errorlevel 1 (
        echo [RayleaBot] Web dev server did not become ready.
        exit /b 1
    )
)

echo [RayleaBot] Starting launcher...
call pnpm --dir "%LAUNCHER_DIR%" exec electron "."
exit /b %errorlevel%

:START_WEB_DEV_SERVER
start "RayleaBot Web Dev" /D "%WEB_DIR%" cmd /d /c "pnpm dev"
exit /b %errorlevel%

:WAIT_FOR_WEB_DEV_SERVER
powershell -NoProfile -ExecutionPolicy Bypass -Command "$deadline = (Get-Date).AddSeconds(30); while ((Get-Date) -lt $deadline) { try { $response = Invoke-WebRequest -UseBasicParsing -Uri $env:RAYLEA_WEB_UI_BASE_URL -TimeoutSec 2; if ($response.StatusCode -ge 200 -and $response.StatusCode -lt 500) { exit 0 } } catch {}; Start-Sleep -Milliseconds 500 }; exit 1"
exit /b %errorlevel%
