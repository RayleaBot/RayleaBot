import sqlite3
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "scripts" / "release"))

import breaking_baseline_prepare
import breaking_plugin_epoch_reset


class BreakingPluginEpochResetTests(unittest.TestCase):
    def test_reset_clears_only_plugin_epoch_state(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            root = base / "install"
            (root / "config").mkdir(parents=True)
            (root / "data" / "plugins" / "weather").mkdir(parents=True)
            (root / "plugins" / "installed" / "weather").mkdir(parents=True)
            (root / "config" / "user.yaml").write_text('schema_version: "3"\n', encoding="utf-8")
            (root / "data" / "plugins" / "weather" / "state.json").write_text("{}", encoding="utf-8")
            (root / "plugins" / "installed" / "weather" / "info.json").write_text("{}", encoding="utf-8")
            database = root / "data" / "rayleabot.db"
            self._create_database(database)
            backup = base / "backup.zip"
            breaking_baseline_prepare.write_backup(root, backup, created_at="2026-08-02T00:00:00Z")

            result = breaking_plugin_epoch_reset.reset_plugin_epoch(root, backup)

            self.assertEqual("2", result["plugin_epoch"])
            self.assertEqual(["plugins/installed", "data/plugins"], result["removed_paths"])
            self.assertFalse((root / "plugins" / "installed").exists())
            self.assertFalse((root / "data" / "plugins").exists())
            connection = sqlite3.connect(database)
            try:
                self.assertEqual(0, connection.execute("SELECT COUNT(*) FROM plugin_instances").fetchone()[0])
                self.assertEqual(0, connection.execute("SELECT COUNT(*) FROM third_party_accounts").fetchone()[0])
                self.assertEqual(0, connection.execute("SELECT COUNT(*) FROM render_template_states WHERE source_type = 'plugin'").fetchone()[0])
                self.assertEqual(1, connection.execute("SELECT COUNT(*) FROM admin_sessions").fetchone()[0])
                self.assertEqual(1, connection.execute("SELECT COUNT(*) FROM management_logs").fetchone()[0])
                self.assertEqual(1, connection.execute("SELECT COUNT(*) FROM system_configs").fetchone()[0])
                self.assertEqual(1, connection.execute("SELECT COUNT(*) FROM secret_store").fetchone()[0])
            finally:
                connection.close()

    @staticmethod
    def _create_database(path: Path) -> None:
        connection = sqlite3.connect(path)
        try:
            schema = (ROOT / "server" / "internal" / "storage" / "schema.sql").read_text(encoding="utf-8")
            connection.executescript(schema)
            connection.executescript(
                """
            INSERT INTO admin_sessions (session_id, subject, issued_at, expires_at)
              VALUES ('hash', 'admin', 'now', 'later');
            INSERT INTO plugin_instances VALUES ('weather', 'enabled', 'now');
            INSERT INTO plugin_packages VALUES ('weather', 'local_zip', 'weather.zip', '1.0.0', 'm', 'p', 'now');
            INSERT INTO plugin_kv VALUES ('weather', 'state', '{}', 2, 'now');
            INSERT INTO scheduler_jobs (job_id, plugin_id, cron_expr, next_run, created_at, updated_at)
              VALUES ('job', 'weather', '* * * * *', 'now', 'now', 'now');
            INSERT INTO secret_store VALUES ('plugin:weather:secret:token', x'00', 'now', 'now');
            INSERT INTO secret_store VALUES ('config:onebot:access_token', x'01', 'now', 'now');
            INSERT INTO system_configs VALUES ('plugin:weather:settings', 'city', '"beijing"', 'now');
            INSERT INTO system_configs VALUES ('governance', 'enabled', 'true', 'now');
            INSERT INTO third_party_accounts (platform, account_id, secret_key, updated_at)
              VALUES ('bilibili', 'primary', 'third_party:bilibili:primary:cookie', 'now');
            INSERT INTO render_template_revisions
              (revision_id, template_id, template_version, kind, saved_at, source_digest, manifest_json, html, stylesheet)
              VALUES ('revision', 'weather/card', '1', 'save', 'now', 'digest', '{}', '<p/>', '');
            INSERT INTO render_template_states
              (template_id, current_revision_id, updated_at, validation_valid, validation_checked_at, validation_issue_count, source_type, source_plugin_id)
              VALUES ('weather/card', 'revision', 'now', 1, 'now', 0, 'plugin', 'weather');
            INSERT INTO management_logs (log_id, ts, level, source, message)
              VALUES ('log', 'now', 'info', 'test', 'preserve');
            """
            )
            connection.commit()
        finally:
            connection.close()


if __name__ == "__main__":
    unittest.main()
