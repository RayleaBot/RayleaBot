from __future__ import annotations

import json
import os
import shutil
import subprocess
import tempfile
import textwrap
import time
import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]


class StartBatTests(unittest.TestCase):
    def _prepare_workspace(self, workspace: Path) -> tuple[Path, Path, Path]:
        shutil.copy2(REPO_ROOT / "start.bat", workspace / "start.bat")

        web_dir = workspace / "web"
        server_dir = workspace / "server"
        launcher_dir = workspace / "launcher"
        for directory in (web_dir, server_dir, launcher_dir):
            directory.mkdir()
            (directory / "pnpm-lock.yaml").write_text("lockfileVersion: '9.0'\n", encoding="utf-8")

        return web_dir, server_dir, launcher_dir

    def _write_fake_commands(self, workspace: Path) -> tuple[Path, Path, Path]:
        bin_dir = workspace / "bin"
        bin_dir.mkdir()
        pnpm_calls_path = workspace / "pnpm-calls.log"
        go_calls_path = workspace / "go-calls.log"
        fake_pnpm = textwrap.dedent(
            f"""\
            @echo off
            setlocal
            >> "{pnpm_calls_path}" echo CWD=%CD%
            >> "{pnpm_calls_path}" echo ARGS=%*
            exit /b 0
            """
        )
        fake_go = textwrap.dedent(
            f"""\
            @echo off
            setlocal
            >> "{go_calls_path}" echo CWD=%CD%
            >> "{go_calls_path}" echo ARGS=%*
            exit /b 0
            """
        )
        (bin_dir / "pnpm.cmd").write_text(fake_pnpm, encoding="ascii")
        (bin_dir / "go.cmd").write_text(fake_go, encoding="ascii")
        return bin_dir, pnpm_calls_path, go_calls_path

    def _read_log_lines(self, path: Path, expected_min_lines: int) -> list[str]:
        lines: list[str] = []
        for _ in range(50):
            if path.exists():
                lines = [line for line in path.read_text(encoding="utf-8").splitlines() if line.strip()]
                if len(lines) >= expected_min_lines:
                    return lines
            time.sleep(0.1)
        return lines

    def test_launcher_package_json_allows_required_build_dependencies(self) -> None:
        package_json = json.loads((REPO_ROOT / "launcher" / "package.json").read_text(encoding="utf-8"))
        approved = set(package_json.get("pnpm", {}).get("onlyBuiltDependencies", []))
        self.assertEqual(approved, {"electron", "electron-winstaller"})

    def test_start_bat_runs_web_dev_mode_by_default_and_can_skip_launch(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            workspace = Path(tmpdir)
            web_dir, server_dir, launcher_dir = self._prepare_workspace(workspace)
            bin_dir, pnpm_calls_path, go_calls_path = self._write_fake_commands(workspace)

            env = os.environ.copy()
            env["PATH"] = str(bin_dir) + os.pathsep + env["PATH"]
            env["RAYLEA_START_SKIP_LAUNCH"] = "1"

            result = subprocess.run(
                ["cmd", "/c", "start.bat"],
                cwd=workspace,
                env=env,
                capture_output=True,
                text=True,
                timeout=30,
            )

            self.assertEqual(result.returncode, 0, msg=result.stdout + result.stderr)

            self.assertIn("[RayleaBot] Web dev mode enabled.", result.stdout)
            self.assertIn("[RayleaBot] Starting web dev server...", result.stdout)

            lines = self._read_log_lines(pnpm_calls_path, 8)
            self.assertEqual(len(lines), 8)

            expected_web_dir = str(web_dir)
            expected_launcher_dir = str(launcher_dir)
            self.assertEqual(lines[6], f"CWD={web_dir}")
            self.assertEqual(
                [line.removeprefix("ARGS=") for line in lines[1::2]],
                [
                    f'--dir "{expected_web_dir}" install --frozen-lockfile',
                    f'--dir "{expected_launcher_dir}" install --frozen-lockfile',
                    f'--dir "{expected_launcher_dir}" run build:app',
                    "dev",
                ],
            )
            go_lines = [line for line in go_calls_path.read_text(encoding="utf-8").splitlines() if line.strip()]
            self.assertEqual(len(go_lines), 2)
            self.assertEqual(go_lines[0], f"CWD={server_dir}")
            self.assertEqual(
                go_lines[1].removeprefix("ARGS="),
                f'build -o "{workspace / "server" / "raylea-server.exe"}" ./cmd/raylea-server',
            )

    def test_start_bat_keeps_build_mode_for_packaged_web_checks(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            workspace = Path(tmpdir)
            web_dir, server_dir, launcher_dir = self._prepare_workspace(workspace)
            bin_dir, pnpm_calls_path, go_calls_path = self._write_fake_commands(workspace)

            env = os.environ.copy()
            env["PATH"] = str(bin_dir) + os.pathsep + env["PATH"]
            env["RAYLEA_START_SKIP_LAUNCH"] = "1"
            env["RAYLEA_START_WEB_MODE"] = "build"

            result = subprocess.run(
                ["cmd", "/c", "start.bat"],
                cwd=workspace,
                env=env,
                capture_output=True,
                text=True,
                timeout=30,
            )

            self.assertEqual(result.returncode, 0, msg=result.stdout + result.stderr)

            lines = self._read_log_lines(pnpm_calls_path, 8)
            self.assertEqual(len(lines), 8)

            expected_web_dir = str(web_dir)
            expected_launcher_dir = str(launcher_dir)
            self.assertEqual(
                [line.removeprefix("ARGS=") for line in lines[1::2]],
                [
                    f'--dir "{expected_web_dir}" install --frozen-lockfile',
                    f'--dir "{expected_web_dir}" run build',
                    f'--dir "{expected_launcher_dir}" install --frozen-lockfile',
                    f'--dir "{expected_launcher_dir}" run build:app',
                ],
            )
            go_lines = [line for line in go_calls_path.read_text(encoding="utf-8").splitlines() if line.strip()]
            self.assertEqual(len(go_lines), 2)
            self.assertEqual(go_lines[0], f"CWD={server_dir}")

    def test_start_bat_launches_electron_from_launcher_dir(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            workspace = Path(tmpdir)
            web_dir, server_dir, launcher_dir = self._prepare_workspace(workspace)
            bin_dir, pnpm_calls_path, go_calls_path = self._write_fake_commands(workspace)

            env = os.environ.copy()
            env["PATH"] = str(bin_dir) + os.pathsep + env["PATH"]
            env["RAYLEA_START_WEB_MODE"] = "build"

            result = subprocess.run(
                ["cmd", "/c", "start.bat"],
                cwd=workspace,
                env=env,
                capture_output=True,
                text=True,
                timeout=30,
            )

            self.assertEqual(result.returncode, 0, msg=result.stdout + result.stderr)

            lines = self._read_log_lines(pnpm_calls_path, 10)
            self.assertEqual(len(lines), 10)

            expected_web_dir = str(web_dir)
            expected_launcher_dir = str(launcher_dir)
            self.assertEqual(
                [line.removeprefix("ARGS=") for line in lines[1::2]],
                [
                    f'--dir "{expected_web_dir}" install --frozen-lockfile',
                    f'--dir "{expected_web_dir}" run build',
                    f'--dir "{expected_launcher_dir}" install --frozen-lockfile',
                    f'--dir "{expected_launcher_dir}" run build:app',
                    f'--dir "{expected_launcher_dir}" exec electron "."',
                ],
            )
            go_lines = [line for line in go_calls_path.read_text(encoding="utf-8").splitlines() if line.strip()]
            self.assertEqual(len(go_lines), 2)
            self.assertEqual(go_lines[0], f"CWD={server_dir}")


if __name__ == "__main__":
    unittest.main()
