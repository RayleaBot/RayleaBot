import sys
import argparse
from contextlib import redirect_stdout
import hashlib
import io
import json
import subprocess
import tempfile
from unittest import mock
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "scripts" / "release"))

import package_artifact


class PackageArtifactTests(unittest.TestCase):
    def test_archive_path_uses_platform_archive_suffix(self) -> None:
        output = Path("dist/release")

        self.assertEqual(
            output / "RayleaBot-v0.1.0-windows-x64-full.zip",
            package_artifact.archive_path(output, "0.1.0", "windows-x64-full"),
        )
        self.assertEqual(
            output / "RayleaBot-v0.1.0-linux-x64-server.tar.gz",
            package_artifact.archive_path(output, "0.1.0", "linux-x64-server"),
        )

    def test_recovery_drill_requires_external_plugin_fixture(self) -> None:
        with self.assertRaises(SystemExit):
            package_artifact.parse_args(
                [
                    "--artifact-id",
                    "linux-x64-server",
                    "--version",
                    "0.1.0",
                    "--git-commit",
                    "abcdef1",
                    "--release-notes-ref",
                    "https://example.invalid/releases/v0.1.0",
                    "--server-bin",
                    "dist/server/raylea-server",
                    "--run-recovery-drill",
                ]
            )


class ValidationEvidenceTests(unittest.TestCase):
    def args(self):
        return argparse.Namespace(artifact_id="linux-x64-server", version="0.1.0", git_commit="abcdef1",
                                  observation_window_seconds="300", window_seconds="600", probe_interval_seconds="30")

    def test_real_child_output_status_and_archive_digest_are_retained(self):
        with tempfile.TemporaryDirectory() as temporary, redirect_stdout(io.StringIO()):
            root = Path(temporary)
            archive = root / "release.tar.gz"
            archive.write_bytes(b"actual artifact bytes")
            with package_artifact.ValidationEvidence(root / "evidence", self.args()) as evidence:
                evidence.record_archive(archive)
                evidence.run("self-host-smoke", [sys.executable, "-c", "import sys; print('stdout marker'); print('stderr marker', file=sys.stderr)"])
            result = json.loads((root / "evidence/validation.json").read_text())
            self.assertEqual(result["status"], "passed")
            self.assertEqual(result["archive"]["sha256"], hashlib.sha256(archive.read_bytes()).hexdigest())
            self.assertEqual(result["checks"][0]["exit_code"], 0)
            self.assertGreaterEqual(result["checks"][0]["elapsed_seconds"], 0)
            log = (root / "evidence/self-host-smoke.log").read_text()
            self.assertIn("stdout marker", log)
            self.assertIn("stderr marker", log)

    def test_failed_child_is_not_reported_as_success_and_preserves_evidence(self):
        with tempfile.TemporaryDirectory() as temporary, redirect_stdout(io.StringIO()):
            root = Path(temporary)
            with self.assertRaises(subprocess.CalledProcessError) as caught:
                with package_artifact.ValidationEvidence(root / "evidence", self.args()) as evidence:
                    evidence.run("recovery-drill", [sys.executable, "-c", "import sys; print('failure marker'); sys.exit(7)"])
            self.assertEqual(caught.exception.returncode, 7)
            result = json.loads((root / "evidence/validation.json").read_text())
            self.assertEqual((result["status"], result["checks"][0]["status"], result["checks"][0]["exit_code"]), ("failed", "failed", 7))
            self.assertIn("failure marker", (root / "evidence/recovery-drill.log").read_text())
            with self.assertRaises(FileExistsError):
                package_artifact.ValidationEvidence(root / "evidence", self.args())
            self.assertEqual(json.loads((root / "evidence/validation.json").read_text()), result)

    def test_package_preserves_recovery_result_and_discards_synthetic_runtime(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            output = root / "packages"
            calls = []
            recovery_work = []
            def fake_run(evidence, name, arguments):
                calls.append((name, arguments))
                if name == "package":
                    output.mkdir()
                    package_artifact.archive_path(output, "0.1.0", "linux-x64-server").write_bytes(b"artifact")
                if name == "recovery-drill":
                    work = Path(arguments[arguments.index("--output-dir") + 1])
                    recovery_work.append(work)
                    work.mkdir(parents=True)
                    (work / "result.json").write_text('{"restored_login":true}')
                    (work / "synthetic-state.db").write_bytes(b"private runtime state")
            arguments = ["--artifact-id", "linux-x64-server", "--version", "0.1.0", "--git-commit", "abcdef1",
                         "--release-notes-ref", "https://example.invalid/notes/0.1.0", "--server-bin", "server",
                         "--output-dir", str(output), "--evidence-dir", str(root / "evidence"), "--run-smoke",
                         "--run-recovery-drill", "--recovery-plugin-fixture", "fixture.zip", "--run-self-host-smoke",
                         "--observation-window-seconds", "300", "--window-seconds", "600", "--probe-interval-seconds", "30"]
            with mock.patch.object(package_artifact.ValidationEvidence, "run", fake_run):
                self.assertEqual(package_artifact.main(arguments), 0)
            self.assertEqual([name for name, _ in calls], ["package", "archive-smoke", "recovery-drill", "self-host-smoke"])
            self.assertEqual(json.loads((root / "evidence/recovery-result.json").read_text()), {"restored_login": True})
            self.assertFalse(recovery_work[0].exists())
            self.assertFalse(any((root / "evidence").rglob("*.db")))
            self.assertEqual(calls[2][1][calls[2][1].index("--observation-window-seconds") + 1], "300")
            self.assertEqual(calls[3][1][calls[3][1].index("--window-seconds") + 1], "600")


if __name__ == "__main__":
    unittest.main()
