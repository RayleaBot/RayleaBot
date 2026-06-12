@echo off
setlocal

set "GIT_BASH=C:\Program Files\Git\usr\bin\bash.exe"
if not exist "%GIT_BASH%" (
  echo gbash: Git Bash not found at %GIT_BASH% 1>&2
  exit /b 127
)

set "PATH=C:\Program Files\Git\usr\bin;C:\Program Files\Git\bin;C:\Program Files\Git\cmd;%PATH%"
"%GIT_BASH%" --noprofile --norc %*
exit /b %ERRORLEVEL%
