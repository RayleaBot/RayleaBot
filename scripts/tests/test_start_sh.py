from __future__ import annotations

import os
import shutil
import stat
import subprocess
import tempfile
import textwrap
import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]


@unittest.skipIf(os.name == "nt", "start.sh tests require a POSIX shell")
class StartShTests(unittest.TestCase):
    def _prepare_workspace(self, workspace: Path) -> None:
        shutil.copy2(REPO_ROOT / "start.sh", workspace / "start.sh")
        shutil.copy2(REPO_ROOT / ".tool-versions", workspace / ".tool-versions")
        scripts_dir = workspace / "scripts"
        scripts_dir.mkdir()
        (scripts_dir / "start-dev.mjs").write_text("", encoding="utf-8")

    def _write_fake_node(self, workspace: Path, version: str = "v26.7.0") -> tuple[Path, Path]:
        bin_dir = workspace / "bin"
        bin_dir.mkdir()
        calls_path = workspace / "node-calls.log"
        fake_node = textwrap.dedent(
            f"""\
            #!/bin/sh
            if [ "${{1:-}}" = "--version" ]; then
              printf '%s\\n' "{version}"
              exit 0
            fi
            printf 'CWD=%s\\n' "$PWD" >> "{calls_path}"
            printf 'ARGS=%s\\n' "$*" >> "{calls_path}"
            printf 'PROFILE=%s\\n' "${{RAYLEA_START_PROFILE:-}}" >> "{calls_path}"
            printf 'SKIP_LAUNCH=%s\\n' "${{RAYLEA_START_SKIP_LAUNCH:-}}" >> "{calls_path}"
            exit 0
            """
        )
        fake_node_path = bin_dir / "node"
        fake_node_path.write_text(fake_node, encoding="utf-8")
        fake_node_path.chmod(fake_node_path.stat().st_mode | stat.S_IEXEC)
        return bin_dir, calls_path

    def test_start_sh_invokes_node_orchestrator(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            workspace = Path(tmpdir)
            self._prepare_workspace(workspace)
            bin_dir, calls_path = self._write_fake_node(workspace)

            env = os.environ.copy()
            env["PATH"] = str(bin_dir) + os.pathsep + env["PATH"]
            env["RAYLEA_START_SKIP_LAUNCH"] = "1"

            result = subprocess.run(
                ["sh", "start.sh", "--dry-run"],
                cwd=workspace,
                env=env,
                capture_output=True,
                text=True,
                timeout=30,
            )

            self.assertEqual(result.returncode, 0, msg=result.stdout + result.stderr)
            lines = [line for line in calls_path.read_text(encoding="utf-8").splitlines() if line.strip()]
            self.assertEqual(lines[0], f"CWD={workspace}")
            self.assertEqual(lines[1], "ARGS=scripts/start-dev.mjs --dry-run")
            self.assertEqual(lines[3], "SKIP_LAUNCH=1")

    def test_start_sh_preserves_start_profile_env(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            workspace = Path(tmpdir)
            self._prepare_workspace(workspace)
            bin_dir, calls_path = self._write_fake_node(workspace)

            env = os.environ.copy()
            env["PATH"] = str(bin_dir) + os.pathsep + env["PATH"]
            env["RAYLEA_START_PROFILE"] = "build"

            result = subprocess.run(
                ["sh", "start.sh"],
                cwd=workspace,
                env=env,
                capture_output=True,
                text=True,
                timeout=30,
            )

            self.assertEqual(result.returncode, 0, msg=result.stdout + result.stderr)
            lines = [line for line in calls_path.read_text(encoding="utf-8").splitlines() if line.strip()]
            self.assertEqual(lines[1], "ARGS=scripts/start-dev.mjs")
            self.assertEqual(lines[2], "PROFILE=build")

    def test_start_sh_rejects_wrong_node_version(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            workspace = Path(tmpdir)
            self._prepare_workspace(workspace)
            bin_dir, calls_path = self._write_fake_node(workspace, version="v25.0.0")

            env = os.environ.copy()
            env["PATH"] = str(bin_dir) + os.pathsep + env["PATH"]

            result = subprocess.run(
                ["sh", "start.sh"],
                cwd=workspace,
                env=env,
                capture_output=True,
                text=True,
                timeout=30,
            )

            self.assertEqual(result.returncode, 1, msg=result.stdout + result.stderr)
            self.assertIn("Node.js version mismatch", result.stderr)
            self.assertFalse(calls_path.exists())


if __name__ == "__main__":
    unittest.main()
