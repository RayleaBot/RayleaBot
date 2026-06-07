from __future__ import annotations

import json
import os
import shutil
import subprocess
import tempfile
import textwrap
import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]


class StartBatTests(unittest.TestCase):
    def _prepare_workspace(self, workspace: Path) -> None:
        shutil.copy2(REPO_ROOT / "start.bat", workspace / "start.bat")
        scripts_dir = workspace / "scripts"
        scripts_dir.mkdir()
        (scripts_dir / "start-dev.mjs").write_text("", encoding="utf-8")

    def _write_fake_node(self, workspace: Path) -> tuple[Path, Path]:
        bin_dir = workspace / "bin"
        bin_dir.mkdir()
        calls_path = workspace / "node-calls.log"
        fake_node = textwrap.dedent(
            f"""\
            @echo off
            setlocal
            >> "{calls_path}" echo CWD=%CD%
            >> "{calls_path}" echo ARGS=%*
            >> "{calls_path}" echo PROFILE=%RAYLEA_START_PROFILE%
            >> "{calls_path}" echo SKIP_LAUNCH=%RAYLEA_START_SKIP_LAUNCH%
            exit /b 0
            """
        )
        (bin_dir / "node.cmd").write_text(fake_node, encoding="ascii")
        return bin_dir, calls_path

    def test_launcher_package_json_allows_required_build_dependencies(self) -> None:
        package_json = json.loads((REPO_ROOT / "launcher" / "package.json").read_text(encoding="utf-8"))
        approved = set(package_json.get("pnpm", {}).get("onlyBuiltDependencies", []))
        self.assertEqual(approved, {"electron", "electron-winstaller"})

    @unittest.skipIf(os.name != "nt", "start.bat is a Windows entrypoint")
    def test_start_bat_invokes_node_orchestrator(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            workspace = Path(tmpdir)
            self._prepare_workspace(workspace)
            bin_dir, calls_path = self._write_fake_node(workspace)

            env = os.environ.copy()
            env["PATH"] = str(bin_dir) + os.pathsep + env["PATH"]
            env["RAYLEA_START_SKIP_LAUNCH"] = "1"

            result = subprocess.run(
                ["cmd", "/c", "start.bat", "--dry-run"],
                cwd=workspace,
                env=env,
                capture_output=True,
                text=True,
                timeout=30,
            )

            self.assertEqual(result.returncode, 0, msg=result.stdout + result.stderr)
            lines = [line for line in calls_path.read_text(encoding="utf-8").splitlines() if line.strip()]
            self.assertEqual(lines[0], f"CWD={workspace}")
            self.assertEqual(lines[1], "ARGS=scripts\\start-dev.mjs --dry-run")
            self.assertEqual(lines[3], "SKIP_LAUNCH=1")

    @unittest.skipIf(os.name != "nt", "start.bat is a Windows entrypoint")
    def test_start_bat_preserves_start_profile_env(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            workspace = Path(tmpdir)
            self._prepare_workspace(workspace)
            bin_dir, calls_path = self._write_fake_node(workspace)

            env = os.environ.copy()
            env["PATH"] = str(bin_dir) + os.pathsep + env["PATH"]
            env["RAYLEA_START_PROFILE"] = "build"

            result = subprocess.run(
                ["cmd", "/c", "start.bat"],
                cwd=workspace,
                env=env,
                capture_output=True,
                text=True,
                timeout=30,
            )

            self.assertEqual(result.returncode, 0, msg=result.stdout + result.stderr)
            lines = [line for line in calls_path.read_text(encoding="utf-8").splitlines() if line.strip()]
            self.assertEqual(lines[1], "ARGS=scripts\\start-dev.mjs")
            self.assertEqual(lines[2], "PROFILE=build")


if __name__ == "__main__":
    unittest.main()
