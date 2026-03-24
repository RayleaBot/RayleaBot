@echo off
setlocal

rem Development-only shortcut for building and starting the launcher.
cd /d "%~dp0"

set "PROJECT=launcher\src\RayleaBot.Launcher\RayleaBot.Launcher.csproj"
set "CONFIG=Debug"
set "TFM=net10.0"
set "OUTPUT_DIR=%~dp0launcher\src\RayleaBot.Launcher\bin\%CONFIG%\%TFM%"
set "LAUNCHER_EXE=%OUTPUT_DIR%\RayleaBot.Launcher.exe"

echo [RayleaBot] Building launcher...
dotnet build "%PROJECT%" -c %CONFIG% --nologo
if errorlevel 1 (
    echo [RayleaBot] Launcher build failed.
    exit /b 1
)

if not exist "%LAUNCHER_EXE%" (
    echo [RayleaBot] Launcher executable not found: "%LAUNCHER_EXE%"
    exit /b 1
)

echo [RayleaBot] Starting launcher...
start "RayleaBot Launcher" /D "%OUTPUT_DIR%" "%LAUNCHER_EXE%"

exit /b 0
