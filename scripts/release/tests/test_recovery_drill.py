import subprocess
import sys
import time
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "scripts" / "release"))

import recovery_drill


class RecoveryDrillTests(unittest.TestCase):
    def test_required_paths_include_contracts_and_web_dist(self) -> None:
        required = recovery_drill.REQUIRED_PATHS["windows-x64-full"]

        self.assertIn("RayleaLauncher.exe", required)
        self.assertIn("contracts/config.user.schema.json", required)
        self.assertIn("contracts/plugin-info.schema.json", required)
        self.assertIn("web/dist/index.html", required)

    def test_read_server_output_stops_running_process_before_collecting_logs(self) -> None:
        process = subprocess.Popen(
            [
                sys.executable,
                "-c",
                "import sys, time; print('ready', flush=True); sys.stderr.write('still-running\\n'); sys.stderr.flush(); time.sleep(60)",
            ],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )
        try:
            time.sleep(0.2)
            output = recovery_drill.read_server_output(process)
        finally:
            if process.poll() is None:
                process.kill()
                process.wait(timeout=5)

        self.assertIsNotNone(process.poll())
        self.assertIn("ready", output)
        self.assertIn("still-running", output)


if __name__ == "__main__":
    unittest.main()
