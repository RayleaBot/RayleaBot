import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "scripts" / "release"))

from package_runtime import ensure_no_forbidden_paths


class SmokeReleaseTests(unittest.TestCase):
    def test_packaged_plugin_entries_are_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "plugins" / "installed" / "fortune" / "bin").mkdir(parents=True)
            (root / "plugins" / "installed" / "fortune" / "bin" / "fortune").write_bytes(
                b"compiled-plugin",
            )

            with self.assertRaises(RuntimeError) as ctx:
                ensure_no_forbidden_paths(root)

        self.assertIn("plugins/installed/fortune", str(ctx.exception).replace("\\", "/"))


if __name__ == "__main__":
    unittest.main()
