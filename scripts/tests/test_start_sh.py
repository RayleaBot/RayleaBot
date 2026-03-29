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
    def test_start_sh_runs_pnpm_against_launcher_dir_and_can_skip_launch(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            workspace = Path(tmpdir)
            shutil.copy2(REPO_ROOT / "start.sh", workspace / "start.sh")

            launcher_dir = workspace / "launcher"
            launcher_dir.mkdir()
            (launcher_dir / "pnpm-lock.yaml").write_text("lockfileVersion: '9.0'\n", encoding="utf-8")

            bin_dir = workspace / "bin"
            bin_dir.mkdir()
            calls_path = workspace / "pnpm-calls.log"
            fake_pnpm = textwrap.dedent(
                f"""\
                #!/bin/sh
                printf 'CWD=%s\n' "$PWD" >> "{calls_path}"
                printf 'ARGS=%s\n' "$*" >> "{calls_path}"
                exit 0
                """
            )
            fake_pnpm_path = bin_dir / "pnpm"
            fake_pnpm_path.write_text(fake_pnpm, encoding="utf-8")
            fake_pnpm_path.chmod(fake_pnpm_path.stat().st_mode | stat.S_IEXEC)

            env = os.environ.copy()
            env["PATH"] = str(bin_dir) + os.pathsep + env["PATH"]
            env["RAYLEA_START_SKIP_LAUNCH"] = "1"

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
            self.assertEqual(
                [line.removeprefix("ARGS=") for line in lines[1::2]],
                [
                    f"--dir {launcher_dir} install --frozen-lockfile",
                    f"--dir {launcher_dir} run build:app",
                ],
            )

    def test_start_sh_launches_electron_from_launcher_dir(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            workspace = Path(tmpdir)
            shutil.copy2(REPO_ROOT / "start.sh", workspace / "start.sh")

            launcher_dir = workspace / "launcher"
            (launcher_dir / "dist" / "main" / "main").mkdir(parents=True)
            (launcher_dir / "dist" / "main" / "main" / "index.js").write_text("// main bundle\n", encoding="utf-8")
            (launcher_dir / "pnpm-lock.yaml").write_text("lockfileVersion: '9.0'\n", encoding="utf-8")

            bin_dir = workspace / "bin"
            bin_dir.mkdir()
            calls_path = workspace / "pnpm-calls.log"
            fake_pnpm = textwrap.dedent(
                f"""\
                #!/bin/sh
                printf 'CWD=%s\n' "$PWD" >> "{calls_path}"
                printf 'ARGS=%s\n' "$*" >> "{calls_path}"
                exit 0
                """
            )
            fake_pnpm_path = bin_dir / "pnpm"
            fake_pnpm_path.write_text(fake_pnpm, encoding="utf-8")
            fake_pnpm_path.chmod(fake_pnpm_path.stat().st_mode | stat.S_IEXEC)

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

            self.assertEqual(result.returncode, 0, msg=result.stdout + result.stderr)

            lines = [line for line in calls_path.read_text(encoding="utf-8").splitlines() if line.strip()]
            self.assertEqual(
                [line.removeprefix("ARGS=") for line in lines[1::2]],
                [
                    f"--dir {launcher_dir} install --frozen-lockfile",
                    f"--dir {launcher_dir} run build:app",
                    f"--dir {launcher_dir} exec electron .",
                ],
            )


if __name__ == "__main__":
    unittest.main()
