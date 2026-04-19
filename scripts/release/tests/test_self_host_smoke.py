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

    def test_extract_task_id_requires_task_identifier(self) -> None:
        self.assertEqual(
            "task_runtime_bootstrap_0001",
            self_host_smoke.extract_task_id({"task_id": "task_runtime_bootstrap_0001"}, "system/runtime/bootstrap"),
        )
        with self.assertRaises(self_host_smoke.SmokeError):
            self_host_smoke.extract_task_id({}, "system/runtime/bootstrap")

    def test_extract_runtime_bootstrap_results_reads_resource_details(self) -> None:
        task_body = {
            "task": {
                "task_id": "task_runtime_bootstrap_0001",
                "task_type": "runtime.bootstrap",
                "status": "succeeded",
                "result": {
                    "summary": "runtime bootstrap completed",
                    "details": {
                        "resources": [
                            {
                                "kind": "python-runtime",
                                "used_cached_archive": True,
                                "store_root": "/tmp/python",
                            }
                        ]
                    },
                },
            }
        }

        resources = self_host_smoke.extract_runtime_bootstrap_results(task_body)

        self.assertEqual("python-runtime", resources[0]["kind"])
        self.assertTrue(resources[0]["used_cached_archive"])

    def test_runtime_bootstrap_result_mode_accepts_prepared_store(self) -> None:
        mode = self_host_smoke.runtime_bootstrap_result_mode(
            {
                "kind": "python-runtime",
                "used_prepared_store": True,
                "used_cached_archive": False,
            }
        )

        self.assertEqual("prepared_store", mode)

    def test_runtime_bootstrap_result_mode_accepts_downloaded_archive(self) -> None:
        mode = self_host_smoke.runtime_bootstrap_result_mode(
            {
                "kind": "nodejs-runtime",
                "used_prepared_store": False,
                "used_cached_archive": False,
                "selected_source": "https://nodejs.org/download/release/v24.14.0/node-v24.14.0-win-x64.zip",
                "attempted_sources": [
                    "https://nodejs.org/download/release/v24.14.0/node-v24.14.0-win-x64.zip",
                ],
            }
        )

        self.assertEqual("downloaded", mode)

    def test_runtime_bootstrap_result_mode_rejects_missing_acquisition_path(self) -> None:
        mode = self_host_smoke.runtime_bootstrap_result_mode(
            {
                "kind": "nodejs-runtime",
                "used_prepared_store": False,
                "used_cached_archive": False,
                "selected_source": "",
                "attempted_sources": [],
            }
        )

        self.assertIsNone(mode)

    def test_recovery_summary_accepts_absent_compatible_and_degraded(self) -> None:
        self_host_smoke.assert_recovery_summary_acceptable(None)
        self_host_smoke.assert_recovery_summary_acceptable({"status": "compatible", "manual_actions": [], "next_steps": [], "skipped_plugins": []})
        self_host_smoke.assert_recovery_summary_acceptable({
            "status": "degraded",
            "manual_actions": ["处理被跳过插件的兼容性问题后，再在管理面中手动重新启用。"],
            "next_steps": ["查看恢复摘要中的跳过插件列表并完成兼容性处理。"],
        })

    def test_recovery_summary_rejects_pending_and_blocked(self) -> None:
        with self.assertRaises(self_host_smoke.SmokeError):
            self_host_smoke.assert_recovery_summary_acceptable({"status": "pending"})
        with self.assertRaises(self_host_smoke.SmokeError):
            self_host_smoke.assert_recovery_summary_acceptable({"status": "blocked"})

    def test_recovery_summary_rejects_guidance_mismatch(self) -> None:
        with self.assertRaises(self_host_smoke.SmokeError):
            self_host_smoke.assert_recovery_summary_acceptable({"status": "compatible", "manual_actions": ["unexpected"]})
        with self.assertRaises(self_host_smoke.SmokeError):
            self_host_smoke.assert_recovery_summary_acceptable({"status": "degraded", "manual_actions": [], "next_steps": []})

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

    def test_validate_protocol_snapshot_requires_frozen_transport_matrix(self) -> None:
        payload = {
            "protocol": "onebot11",
            "provider": "standard",
            "configured_transports": ["forward_ws"],
            "active_transports": [],
            "transport_status": [
                {"transport": "reverse_ws", "enabled": False, "configured": False, "endpoint": "", "state": "idle", "summary": "未启用"},
                {"transport": "forward_ws", "enabled": True, "configured": True, "endpoint": "ws://127.0.0.1:8089", "state": "connecting", "summary": "正在主动连接"},
                {"transport": "http_api", "enabled": False, "configured": False, "endpoint": "", "state": "idle", "summary": "未启用"},
                {"transport": "webhook", "enabled": False, "configured": False, "endpoint": "", "state": "idle", "summary": "未启用"},
            ],
            "readiness_status": "setup_required",
            "summary": "OneBot11 尚未配置连接",
            "recent_transport_issues": [],
        }

        self_host_smoke.validate_protocol_snapshot(payload)

        payload["transport_status"] = payload["transport_status"][:-1]
        with self.assertRaises(self_host_smoke.SmokeError):
            self_host_smoke.validate_protocol_snapshot(payload)

    def test_validate_protocol_compatibility_requires_categories_and_representative_items(self) -> None:
        payload = {
            "protocol": "onebot11",
            "categories": [
                {
                    "key": "events",
                    "title": "核心事件",
                    "items": [
                        {
                            "key": "notice.flash_file",
                            "label": "闪传文件事件",
                            "summary": "ok",
                            "support": {"standard": "supported", "napcat": "supported", "luckylillia": "supported"},
                        }
                    ],
                },
                {
                    "key": "message_segments",
                    "title": "消息段",
                    "items": [
                        {
                            "key": "flash_file",
                            "label": "闪传文件",
                            "summary": "ok",
                            "support": {"standard": "supported", "napcat": "supported", "luckylillia": "supported"},
                        }
                    ],
                },
                {
                    "key": "read_capabilities",
                    "title": "读取能力",
                    "items": [
                        {
                            "key": "message.history.get",
                            "label": "读取历史消息",
                            "summary": "ok",
                            "support": {"standard": "supported", "napcat": "supported", "luckylillia": "supported"},
                        }
                    ],
                },
                {
                    "key": "provider_extensions",
                    "title": "Provider 扩展",
                    "items": [
                        {
                            "key": "provider.napcat.group.sign.set",
                            "label": "NapCat 群签到",
                            "summary": "ok",
                            "support": {"standard": "unsupported", "napcat": "supported", "luckylillia": "unsupported"},
                        },
                        {
                            "key": "provider.luckylillia.friend_groups.get",
                            "label": "LuckyLillia 好友分组",
                            "summary": "ok",
                            "support": {"standard": "unsupported", "napcat": "unsupported", "luckylillia": "supported"},
                        },
                    ],
                },
            ],
        }

        self_host_smoke.validate_protocol_compatibility(payload)

        payload["categories"][3]["items"] = payload["categories"][3]["items"][:-1]
        with self.assertRaises(self_host_smoke.SmokeError):
            self_host_smoke.validate_protocol_compatibility(payload)

    def test_select_template_id_requires_packaged_help_menu_template(self) -> None:
        payload = {
            "items": [
                {
                    "id": "help.menu",
                    "version": "1",
                    "width": 960,
                    "height": 640,
                    "has_input_schema": True,
                    "current_revision_id": "rev_help_menu_0001",
                    "updated_at": "2026-04-18T10:30:00Z",
                }
            ]
        }

        self.assertEqual("help.menu", self_host_smoke.select_template_id(payload))

        payload["items"][0]["id"] = "status.panel"
        with self.assertRaises(self_host_smoke.SmokeError):
            self_host_smoke.select_template_id(payload)

    def test_validate_render_preview_details_requires_artifact_fields(self) -> None:
        details = {
            "artifact_id": "render_preview_0001.png",
            "image_url": "/api/system/render/artifacts/render_preview_0001.png",
            "mime": "image/png",
            "cache_key": "help.menu:v1:default:abc12345",
            "template": "help.menu",
            "theme": "default",
            "from_cache": False,
        }

        artifact_id, image_url = self_host_smoke.validate_render_preview_details(details, "help.menu")

        self.assertEqual("render_preview_0001.png", artifact_id)
        self.assertEqual("/api/system/render/artifacts/render_preview_0001.png", image_url)

        details["artifact_id"] = ""
        with self.assertRaises(self_host_smoke.SmokeError):
            self_host_smoke.validate_render_preview_details(details, "help.menu")

    def test_validate_render_template_versions_checks_save_and_rollback_order(self) -> None:
        payload = {
            "items": [
                {
                    "revision_id": "rev_help_menu_0003",
                    "template_version": "1",
                    "saved_at": "2026-04-18T11:20:00Z",
                    "kind": "rollback",
                    "message": "rollback",
                },
                {
                    "revision_id": "rev_help_menu_0002",
                    "template_version": "1",
                    "saved_at": "2026-04-18T11:05:00Z",
                    "kind": "save",
                    "message": "save",
                },
            ]
        }

        revision_ids = self_host_smoke.validate_render_template_versions(
            payload,
            expected_top_revision_id="rev_help_menu_0003",
            expected_top_kind="rollback",
        )

        self.assertEqual(["rev_help_menu_0003", "rev_help_menu_0002"], revision_ids)

        payload["items"][0]["kind"] = "draft"
        with self.assertRaises(self_host_smoke.SmokeError):
            self_host_smoke.validate_render_template_versions(payload)


if __name__ == "__main__":
    unittest.main()
