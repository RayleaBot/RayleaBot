@echo off
setlocal EnableExtensions

chcp 65001 >nul 2>&1

cd /d "%~dp0"
if errorlevel 1 (
  echo [RayleaBot] 启动失败：无法进入项目目录 "%~dp0"。
  goto :failure
)

set "NODE_VERSION="
for /f "tokens=1,2" %%A in (.tool-versions) do (
  if /i "%%A"=="nodejs" set "NODE_VERSION=%%B"
)
if not defined NODE_VERSION (
  echo [RayleaBot] 启动失败：.tool-versions 中缺少 Node.js 版本。
  goto :failure
)

set "NODE_BIN="
if defined RAYLEA_START_NODE (
  if not exist "%RAYLEA_START_NODE%" (
    echo [RayleaBot] 启动失败：RAYLEA_START_NODE 指向的文件不存在：
    echo   %RAYLEA_START_NODE%
    goto :failure
  )
  set "NODE_BIN=%RAYLEA_START_NODE%"
)

if not defined NODE_BIN if exist "%~dp0.deps\store\nodejs-windows-x64\%NODE_VERSION%\node-v%NODE_VERSION%-win-x64\node.exe" (
  set "NODE_BIN=%~dp0.deps\store\nodejs-windows-x64\%NODE_VERSION%\node-v%NODE_VERSION%-win-x64\node.exe"
)

if not defined NODE_BIN if exist "%USERPROFILE%\.local\opt\node-v%NODE_VERSION%-win-x64\node.exe" (
  set "NODE_BIN=%USERPROFILE%\.local\opt\node-v%NODE_VERSION%-win-x64\node.exe"
)

if not defined NODE_BIN (
  for /f "delims=" %%I in ('where node.exe 2^>nul') do if not defined NODE_BIN set "NODE_BIN=%%I"
)

if not defined NODE_BIN (
  echo [RayleaBot] 启动失败：未找到 Node.js %NODE_VERSION%。
  echo [RayleaBot] 请运行 python scripts\check-toolchain.py 查看安装指引。
  goto :failure
)

set "NODE_ACTUAL="
set "NODE_VERSION_FILE=%TEMP%\rayleabot-node-version-%RANDOM%-%RANDOM%.txt"
set "NODE_IS_SCRIPT="
if /i "%NODE_BIN:~-4%"==".cmd" set "NODE_IS_SCRIPT=1"
if /i "%NODE_BIN:~-4%"==".bat" set "NODE_IS_SCRIPT=1"
if defined NODE_IS_SCRIPT (
  call "%NODE_BIN%" --version >"%NODE_VERSION_FILE%" 2>nul
) else (
  "%NODE_BIN%" --version >"%NODE_VERSION_FILE%" 2>nul
)
if exist "%NODE_VERSION_FILE%" set /p "NODE_ACTUAL="<"%NODE_VERSION_FILE%"
del /q "%NODE_VERSION_FILE%" >nul 2>&1
if not defined NODE_ACTUAL set "NODE_ACTUAL=无法读取"
if /i not "%NODE_ACTUAL%"=="v%NODE_VERSION%" (
  echo [RayleaBot] 启动失败：Node.js 版本不匹配。
  echo [RayleaBot] 当前版本：%NODE_ACTUAL%
  echo [RayleaBot] 需要版本：v%NODE_VERSION%
  echo [RayleaBot] 可执行文件：%NODE_BIN%
  goto :failure
)

echo [RayleaBot] 使用 Node.js %NODE_ACTUAL%。
if defined NODE_IS_SCRIPT (
  call "%NODE_BIN%" scripts\start-dev.mjs %*
) else (
  "%NODE_BIN%" scripts\start-dev.mjs %*
)
set "EXIT_CODE=%errorlevel%"
if "%EXIT_CODE%"=="0" exit /b 0

echo.
echo [RayleaBot] 启动失败，退出码 %EXIT_CODE%。
echo [RayleaBot] 请查看上方错误和 logs\dev\start\ 中的启动日志。
goto :pause_and_exit

:failure
set "EXIT_CODE=1"

:pause_and_exit
if "%RAYLEA_START_NO_PAUSE%"=="1" exit /b %EXIT_CODE%
echo.
echo [RayleaBot] 按任意键关闭窗口...
pause >nul
exit /b %EXIT_CODE%
