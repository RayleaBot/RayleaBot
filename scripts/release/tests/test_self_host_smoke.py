import io
import sys
import unittest
import urllib.error
import zipfile
from pathlib import Path
from unittest import mock

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "scripts" / "release"))

import self_host_smoke


class SelfHostSmokeTests(unittest.TestCase):
    def test_parser_defaults_match_long_smoke_plan(self) -> None:
        args = self_host_smoke.build_parser().parse_args(
            [
                "--artifact-id",
                "linux-x64-server",
                "--archive",
                "bundle.tar.gz",
            ]
        )

        self.assertEqual(600, args.window_seconds)
        self.assertEqual(30, args.probe_interval_seconds)

    def test_ensure_monotonic_uptime_accepts_growth(self) -> None:
        self.assertEqual(12, self_host_smoke.ensure_monotonic_uptime(10, 12))

    def test_ensure_monotonic_uptime_rejects_regression(self) -> None:
        with self.assertRaises(self_host_smoke.SmokeError):
            self_host_smoke.ensure_monotonic_uptime(10, 9)

    def test_validate_diagnostics_archive_requires_core_entries(self) -> None:
        payload = io.BytesIO()
        with zipfile.ZipFile(payload, "w", compression=zipfile.ZIP_DEFLATED) as zf:
            zf.writestr("system-status.json", "{}")
            zf.writestr("readiness.json", "{}")

        with self.assertRaises(self_host_smoke.SmokeError):
            self_host_smoke.validate_diagnostics_archive(payload.getvalue())

    def test_validate_diagnostics_archive_accepts_required_entries(self) -> None:
        payload = io.BytesIO()
        with zipfile.ZipFile(payload, "w", compression=zipfile.ZIP_DEFLATED) as zf:
            zf.writestr("system-status.json", "{}")
            zf.writestr("readiness.json", "{}")
            zf.writestr("doctor.json", "{}")

        self_host_smoke.validate_diagnostics_archive(payload.getvalue())

    def test_extract_backup_archive_path_requires_succeeded_backup_task(self) -> None:
        task_body = {
            "task": {
                "task_id": "task_backup_create_0001",
                "task_type": "backup.create",
                "status": "succeeded",
                "summary": "backup",
                "result": {
                    "summary": "backup completed",
                    "details": {"archive_path": "/tmp/backup.zip"},
                },
            }
        }

        self.assertEqual("/tmp/backup.zip", self_host_smoke.extract_backup_archive_path(task_body))

    def test_extract_backup_archive_path_rejects_failed_task(self) -> None:
        task_body = {
            "task": {
                "task_id": "task_backup_create_0001",
                "task_type": "backup.create",
                "status": "failed",
                "summary": "backup",
            }
        }

        with self.assertRaises(self_host_smoke.SmokeError):
            self_host_smoke.extract_backup_archive_path(task_body)

    def test_recovery_summary_accepts_absent_compatible_and_degraded(self) -> None:
        self_host_smoke.assert_recovery_summary_acceptable(None)
        self_host_smoke.assert_recovery_summary_acceptable({"status": "compatible"})
        self_host_smoke.assert_recovery_summary_acceptable({"status": "degraded"})

    def test_recovery_summary_rejects_pending_and_blocked(self) -> None:
        with self.assertRaises(self_host_smoke.SmokeError):
            self_host_smoke.assert_recovery_summary_acceptable({"status": "pending"})
        with self.assertRaises(self_host_smoke.SmokeError):
            self_host_smoke.assert_recovery_summary_acceptable({"status": "blocked"})

    def test_request_json_accepts_allowed_http_error_status(self) -> None:
        payload = b'{"status":"setup_required"}'
        error = urllib.error.HTTPError(
            "http://127.0.0.1/readyz",
            503,
            "Service Unavailable",
            hdrs=None,
            fp=io.BytesIO(payload),
        )

        with mock.patch.object(self_host_smoke.urllib.request, "urlopen", side_effect=error):
            body = self_host_smoke.request_json(
                "http://127.0.0.1/readyz",
                expected_statuses={200, 503},
            )

        self.assertEqual("setup_required", body["status"])


if __name__ == "__main__":
    unittest.main()
