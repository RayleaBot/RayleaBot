import sys
import argparse
from contextlib import redirect_stdout
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


class ValidationEvidenceTests(unittest.TestCase):
    def args(self):
        return argparse.Namespace(artifact_id="linux-x64-server", version="0.1.0", git_commit="abcdef1")

    def test_real_child_output_status_and_archive_identity_are_retained(self):
        with tempfile.TemporaryDirectory() as temporary, redirect_stdout(io.StringIO()):
            root = Path(temporary)
            archive = root / "release.tar.gz"
            archive.write_bytes(b"actual artifact bytes")
            with package_artifact.ValidationEvidence(root / "evidence", self.args()) as evidence:
                evidence.record_archive(archive)
                evidence.run("archive-smoke", [sys.executable, "-c", "import sys; print('stdout marker'); print('stderr marker', file=sys.stderr)"])
            result = json.loads((root / "evidence/validation.json").read_text())
            self.assertEqual(result["status"], "passed")
            self.assertEqual(result["archive"], {"file_name": archive.name, "size_bytes": archive.stat().st_size})
            self.assertEqual(result["checks"][0]["exit_code"], 0)
            self.assertGreaterEqual(result["checks"][0]["elapsed_seconds"], 0)
            log = (root / "evidence/archive-smoke.log").read_text()
            self.assertIn("stdout marker", log)
            self.assertIn("stderr marker", log)

    def test_failed_child_is_not_reported_as_success_and_preserves_evidence(self):
        with tempfile.TemporaryDirectory() as temporary, redirect_stdout(io.StringIO()):
            root = Path(temporary)
            with self.assertRaises(subprocess.CalledProcessError) as caught:
                with package_artifact.ValidationEvidence(root / "evidence", self.args()) as evidence:
                    evidence.run("archive-smoke", [sys.executable, "-c", "import sys; print('failure marker'); sys.exit(7)"])
            self.assertEqual(caught.exception.returncode, 7)
            result = json.loads((root / "evidence/validation.json").read_text())
            self.assertEqual((result["status"], result["checks"][0]["status"], result["checks"][0]["exit_code"]), ("failed", "failed", 7))
            self.assertIn("failure marker", (root / "evidence/archive-smoke.log").read_text())
            with self.assertRaises(FileExistsError):
                package_artifact.ValidationEvidence(root / "evidence", self.args())
            self.assertEqual(json.loads((root / "evidence/validation.json").read_text()), result)

    def test_package_runs_the_archive_smoke_on_the_packaged_archive(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            output = root / "packages"
            calls = []
            def fake_run(evidence, name, arguments):
                calls.append((name, arguments))
                if name == "package":
                    output.mkdir()
                    package_artifact.archive_path(output, "0.1.0", "linux-x64-server").write_bytes(b"artifact")
            arguments = ["--artifact-id", "linux-x64-server", "--version", "0.1.0", "--git-commit", "abcdef1",
                         "--release-notes-ref", "https://example.invalid/notes/0.1.0", "--server-bin", "server",
                         "--output-dir", str(output), "--evidence-dir", str(root / "evidence"), "--run-smoke"]
            with mock.patch.object(package_artifact.ValidationEvidence, "run", fake_run):
                self.assertEqual(package_artifact.main(arguments), 0)
            self.assertEqual([name for name, _ in calls], ["package", "archive-smoke"])
            smoke = calls[1][1]
            self.assertEqual(smoke[smoke.index("--archive") + 1], str(package_artifact.archive_path(output, "0.1.0", "linux-x64-server")))


if __name__ == "__main__":
    unittest.main()
