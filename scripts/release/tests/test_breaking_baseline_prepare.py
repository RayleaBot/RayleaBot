import json
import sys
import tempfile
import unittest
import zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "scripts" / "release"))

import breaking_baseline_prepare


class BreakingBaselinePrepareTests(unittest.TestCase):
    def test_write_backup_includes_runtime_state_and_manifest(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / "install"
            (root / "config").mkdir(parents=True)
            (root / "data").mkdir()
            (root / "plugins" / "installed" / "weather").mkdir(parents=True)
            (root / "logs").mkdir()
            (root / "config" / "user.yaml").write_text("schema_version: \"2\"\n", encoding="utf-8")
            (root / "data" / "rayleabot.db").write_bytes(b"db")
            (root / "plugins" / "installed" / "weather" / "info.json").write_text("{}", encoding="utf-8")
            (root / "plugins" / "installed" / "weather" / "__pycache__").mkdir()
            (root / "plugins" / "installed" / "weather" / "__pycache__" / "plugin.pyc").write_bytes(b"cache")
            (root / "logs" / "recovery-summary.json").write_text("{}", encoding="utf-8")
            output = Path(tmp) / "baseline-backup.zip"

            backup_path = breaking_baseline_prepare.write_backup(root, output, created_at="2026-06-07T00:00:00Z")

            self.assertEqual(output, backup_path)
            with zipfile.ZipFile(backup_path) as archive:
                names = set(archive.namelist())
                manifest = json.loads(archive.read("breaking-baseline-backup.json").decode("utf-8"))

        self.assertIn("config/user.yaml", names)
        self.assertIn("data/rayleabot.db", names)
        self.assertIn("plugins/installed/weather/info.json", names)
        self.assertIn("logs/recovery-summary.json", names)
        self.assertNotIn("plugins/installed/weather/__pycache__/plugin.pyc", names)
        self.assertEqual("breaking-baseline-backup", manifest["kind"])
        self.assertEqual("2026-06-07T00:00:00Z", manifest["created_at"])
        self.assertEqual(["config", "data", "plugins/installed"], manifest["rollback"]["restore_directories"])

    def test_collect_backup_entries_ignores_missing_optional_paths(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "config").mkdir()
            (root / "config" / "user.yaml").write_text("schema_version: \"2\"\n", encoding="utf-8")

            entries = breaking_baseline_prepare.collect_backup_entries(root)

        self.assertEqual([Path("config/user.yaml")], entries)


if __name__ == "__main__":
    unittest.main()
