$ErrorActionPreference = 'Stop'

$gitBash = 'C:\Program Files\Git\usr\bin\bash.exe'
if (-not (Test-Path -LiteralPath $gitBash)) {
  Write-Error "gbash: Git Bash not found at $gitBash"
  exit 127
}

$env:PATH = "C:\Program Files\Git\usr\bin;C:\Program Files\Git\bin;C:\Program Files\Git\cmd;$env:PATH"
& $gitBash --noprofile --norc @args
exit $LASTEXITCODE
