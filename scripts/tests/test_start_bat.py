from __future__ import annotations

import os
import shutil
import subprocess
import tempfile
import textwrap
import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]


class StartBatTests(unittest.TestCase):
    def test_start_bat_runs_pnpm_against_launcher_dir_and_can_skip_launch(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            workspace = Path(tmpdir)
            shutil.copy2(REPO_ROOT / "start.bat", workspace / "start.bat")

            launcher_dir = workspace / "launcher"
            launcher_dir.mkdir()
            (launcher_dir / "pnpm-lock.yaml").write_text("lockfileVersion: '9.0'\n", encoding="utf-8")

            bin_dir = workspace / "bin"
            bin_dir.mkdir()
            calls_path = workspace / "pnpm-calls.log"
            fake_pnpm = textwrap.dedent(
                f"""\
                @echo off
                setlocal
                >> "{calls_path}" echo CWD=%CD%
                >> "{calls_path}" echo ARGS=%*
                exit /b 0
                """
            )
            (bin_dir / "pnpm.cmd").write_text(fake_pnpm, encoding="ascii")

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

            lines = [line for line in calls_path.read_text(encoding="utf-8").splitlines() if line.strip()]
            self.assertEqual(len(lines), 4)

            expected_launcher_dir = str(launcher_dir)
            self.assertEqual(
                [line.removeprefix("ARGS=") for line in lines[1::2]],
                [
                    f'--dir "{expected_launcher_dir}" install --frozen-lockfile',
                    f'--dir "{expected_launcher_dir}" build',
                ],
            )


if __name__ == "__main__":
    unittest.main()
