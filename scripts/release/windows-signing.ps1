param(
    [string[]]$SignTargets = @('dist/server/raylea-server.exe', 'dist/server/raylea-updater.exe', 'launcher/dist/package/win-unpacked/RayleaLauncher.exe'),
    [string]$LauncherDirectory = 'launcher/dist/package/win-unpacked',
    [string]$CertificateSHA1 = $env:RAYLEA_WINDOWS_CERT_SHA1,
    [string]$TimestampURL = $env:RAYLEA_TIMESTAMP_URL,
    [string]$SignTool,
    [string]$OutputPath = $env:GITHUB_OUTPUT
)

$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $false
if ($SignTargets.Count -eq 0) { throw 'Signing requires artifact targets.' }
foreach ($target in $SignTargets) {
    if (-not (Test-Path -LiteralPath $target -PathType Leaf)) { throw "Signing target is missing: $target" }
}
if (-not (Test-Path -LiteralPath $LauncherDirectory -PathType Container)) { throw 'Launcher package directory is missing.' }
if ([string]::IsNullOrWhiteSpace($OutputPath)) { throw 'Signing output path is required.' }
'signer_sha256=' | Out-File -LiteralPath $OutputPath -Append -Encoding utf8

$configured = -not [string]::IsNullOrWhiteSpace($CertificateSHA1)
if (-not $configured) {
    $existing = @($SignTargets | ForEach-Object { Get-AuthenticodeSignature -LiteralPath $_ })
    if (@($existing | Where-Object { $_.Status -ne 'NotSigned' }).Count -eq 0) {
        Write-Host 'No signing identity or pre-signed artifacts; Windows update remains guided.'
        return
    }
}

if ([string]::IsNullOrWhiteSpace($SignTool)) {
    $SignTool = Get-ChildItem "${env:ProgramFiles(x86)}\Windows Kits\10\bin\*\x64\signtool.exe" -ErrorAction SilentlyContinue | Sort-Object FullName | Select-Object -Last 1 -ExpandProperty FullName
}
if ([string]::IsNullOrWhiteSpace($SignTool)) { throw 'signtool.exe is required for configured or pre-signed artifacts.' }
if ([string]::IsNullOrWhiteSpace($TimestampURL)) { $TimestampURL = 'http://timestamp.digicert.com' }

if ($configured) {
    foreach ($target in $SignTargets) {
        & $SignTool sign /sha1 $CertificateSHA1 /fd SHA256 /tr $TimestampURL /td SHA256 $target
        if ($LASTEXITCODE -ne 0) { throw "Production signing failed: $target" }
    }
}

$allPE = @($SignTargets) + @(Get-ChildItem -LiteralPath $LauncherDirectory -Recurse -File | Where-Object { $_.Extension -in '.exe', '.dll' } | ForEach-Object FullName)
foreach ($target in $allPE | Sort-Object -Unique) {
    & $SignTool verify /pa /all $target
    if ($LASTEXITCODE -ne 0) { throw "PE signature verification failed: $target" }
}

$digests = @()
foreach ($target in $SignTargets) {
    $signature = Get-AuthenticodeSignature -LiteralPath $target
    if ($signature.Status -ne 'Valid' -or $null -eq $signature.SignerCertificate) { throw "Required artifact has no valid signer: $target" }
    $digests += [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData($signature.SignerCertificate.RawData)).ToLowerInvariant()
}
if (@($digests | Sort-Object -Unique).Count -ne 1) { throw 'Required artifact signer identities differ.' }
"signer_sha256=$($digests[0])" | Out-File -LiteralPath $OutputPath -Append -Encoding utf8
