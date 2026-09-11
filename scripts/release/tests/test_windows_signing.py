import hashlib
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[3]


class WindowsSigningGateTests(unittest.TestCase):
    def test_signing_failures_cannot_fall_back_to_guided(self):
        pwsh = shutil.which("pwsh")
        if not pwsh:
            self.skipTest("PowerShell unavailable")
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            package = root / "package"
            package.mkdir()
            for name in ("server.exe", "updater.exe", "launcher.exe", "vendor.dll"):
                (package / name).write_bytes(b"fixture")
            harness = root / "harness.ps1"
            harness.write_text(r'''
param([string]$Scenario, [string]$Gate, [string]$Package, [string]$OutputPath)
$ErrorActionPreference = 'Stop'
$global:SigningScenario = $Scenario
function global:Get-AuthenticodeSignature {
    param([string]$LiteralPath)
    $status = if ($global:SigningScenario -eq 'unsigned') { 'NotSigned' } else { 'Valid' }
    if ($global:SigningScenario -eq 'partial' -and $LiteralPath.EndsWith('server.exe')) { $status = 'NotSigned' }
    if ($global:SigningScenario -eq 'invalid-signer') { $status = 'HashMismatch' }
    [byte[]]$data = @(1, 2, 3)
    if ($global:SigningScenario -eq 'different-signers' -and $LiteralPath.EndsWith('server.exe')) { $data = @(4, 5, 6) }
    return [pscustomobject]@{ Status = $status; SignerCertificate = [pscustomobject]@{ RawData = $data } }
}
function global:FixtureSignTool {
    $global:LASTEXITCODE = 0
    if ($global:SigningScenario -eq 'sign-failure' -and $args[0] -eq 'sign') { $global:LASTEXITCODE = 1 }
    if ($global:SigningScenario -eq 'verify-failure' -and $args[0] -eq 'verify') { $global:LASTEXITCODE = 1 }
    if ($global:SigningScenario -eq 'unsigned') { throw 'Unconfigured unsigned release invoked signing' }
}
$certificate = if ($Scenario -in @('unsigned', 'pre-signed', 'partial')) { '' } else { 'fixture-thumbprint' }
$tool = if ($Scenario -eq 'missing-tool') { 'NonexistentFixtureSignTool' } else { 'FixtureSignTool' }
$targets = @('server.exe', 'updater.exe', 'launcher.exe') | ForEach-Object { Join-Path $Package $_ }
& $Gate -SignTargets $targets -LauncherDirectory $Package -CertificateSHA1 $certificate -SignTool $tool -OutputPath $OutputPath
''', encoding="utf-8")
            for scenario in ("unsigned", "pre-signed", "signed", "sign-failure", "verify-failure", "partial", "invalid-signer", "different-signers", "missing-tool"):
                with self.subTest(scenario=scenario):
                    output = root / (scenario + ".out")
                    completed = subprocess.run([pwsh, "-NoProfile", "-File", str(harness), scenario, str(ROOT / "scripts/release/windows-signing.ps1"), str(package), str(output)], cwd=ROOT, env={**os.environ, "RAYLEA_WINDOWS_CERT_SHA1": ""}, text=True, capture_output=True, timeout=30)
                    entries = output.read_text(encoding="utf-8-sig").splitlines()
                    success = scenario in {"unsigned", "pre-signed", "signed"}
                    self.assertEqual(completed.returncode == 0, success, completed.stderr)
                    expected = "" if scenario not in {"pre-signed", "signed"} else hashlib.sha256(bytes([1, 2, 3])).hexdigest()
                    self.assertEqual(entries[-1], "signer_sha256=" + expected)
