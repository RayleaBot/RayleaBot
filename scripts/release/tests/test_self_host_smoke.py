import io
import json
import struct
import subprocess
import tempfile
from contextlib import redirect_stdout

import yaml
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
                "--plugin-fixture",
                "fixture.zip",
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
                                "kind": "chromium",
                                "used_cached_archive": True,
                                "store_root": "/tmp/chromium",
                            }
                        ]
                    },
                },
            }
        }

        resources = self_host_smoke.extract_runtime_bootstrap_results(task_body)

        self.assertEqual("chromium", resources[0]["kind"])
        self.assertTrue(resources[0]["used_cached_archive"])

    def test_runtime_bootstrap_result_mode_accepts_prepared_store(self) -> None:
        mode = self_host_smoke.runtime_bootstrap_result_mode(
            {
                "kind": "chromium",
                "used_prepared_store": True,
                "used_cached_archive": False,
            }
        )

        self.assertEqual("prepared_store", mode)

    def test_runtime_bootstrap_result_mode_accepts_system_browser(self) -> None:
        mode = self_host_smoke.runtime_bootstrap_result_mode(
            {
                "kind": "chromium",
                "used_system_browser": True,
                "used_prepared_store": False,
                "used_cached_archive": False,
            }
        )

        self.assertEqual("system_browser", mode)

    def test_runtime_bootstrap_cycle_accepts_system_browser_without_managed_paths(self) -> None:
        task_body = {
            "task": {
                "task_type": "runtime.bootstrap",
                "status": "succeeded",
                "result": {
                    "details": {
                        "resources": [
                            {
                                "kind": "chromium",
                                "used_system_browser": True,
                            }
                        ]
                    }
                },
            }
        }
        with (
            mock.patch.object(self_host_smoke, "remove_prepared_runtime_stores"),
            mock.patch.object(
                self_host_smoke,
                "create_runtime_bootstrap_task",
                return_value="task_runtime_bootstrap_0001",
            ),
            mock.patch.object(self_host_smoke, "poll_task", return_value=task_body),
        ):
            self_host_smoke.run_runtime_bootstrap_cycle(
                Path("unused"),
                "linux-x64-server",
                "http://127.0.0.1:8088",
                "session-token",
            )

    def test_runtime_bootstrap_result_mode_accepts_downloaded_archive(self) -> None:
        mode = self_host_smoke.runtime_bootstrap_result_mode(
            {
                "kind": "chromium",
                "used_prepared_store": False,
                "used_cached_archive": False,
                "selected_source": "https://storage.googleapis.com/chrome-for-testing-public/chromium.zip",
                "attempted_sources": [
                    "https://storage.googleapis.com/chrome-for-testing-public/chromium.zip",
                ],
            }
        )

        self.assertEqual("downloaded", mode)

    def test_runtime_bootstrap_result_mode_rejects_missing_acquisition_path(self) -> None:
        mode = self_host_smoke.runtime_bootstrap_result_mode(
            {
                "kind": "chromium",
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

    def test_bootstrap_admin_supplies_origin_and_one_time_setup_token(self) -> None:
        with mock.patch.object(
            self_host_smoke,
            "request_json",
            return_value={"session_token": "session-token"},
        ) as request:
            token = self_host_smoke.bootstrap_admin("http://127.0.0.1:8080/")

        self.assertEqual("session-token", token)
        self.assertEqual("http://127.0.0.1:8080", request.call_args.kwargs["headers"]["Origin"])
        self.assertEqual(self_host_smoke.SETUP_TOKEN, request.call_args.kwargs["headers"]["X-Raylea-Setup-Token"])

    def test_start_server_injects_deterministic_setup_token(self) -> None:
        with mock.patch.object(self_host_smoke, "start_captured_process") as start:
            self_host_smoke.start_server(Path("root"), Path("raylea-server"))

        self.assertEqual(self_host_smoke.SETUP_TOKEN, start.call_args.kwargs["env"]["RAYLEA_SETUP_TOKEN"])

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

        payload["provider"] = "unknown"
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
                    "updated_at": "2026-04-18T10:30:00Z",
                    "source": {"type": "system", "plugin_id": None, "local_id": None},
                }
            ]
        }

        self.assertEqual("help.menu", self_host_smoke.select_template_id(payload))

        payload["items"][0]["id"] = "status.panel"
        with self.assertRaises(self_host_smoke.SmokeError):
            self_host_smoke.select_template_id(payload)

    def test_validate_render_template_detail_and_preview_html_match_current_contract(self) -> None:
        detail = {
            "template": {
                "id": "help.menu",
                "version": "1",
                "width": 960,
                "height": 640,
                "has_input_schema": True,
                "updated_at": "2026-04-18T10:30:00Z",
                "source": {"type": "system", "plugin_id": None, "local_id": None},
                "input_schema_json": {"type": "object"},
                "preview_data_json": {"title": "RayleaBot"},
            }
        }
        preview = yaml.safe_load((ROOT / "fixtures/web-api/ok.system-render-template-preview-html.yaml").read_text(encoding="utf-8"))["response"]["body"]

        preview_data = self_host_smoke.validate_render_template_detail(detail, "help.menu")
        source_digest = self_host_smoke.validate_render_template_preview_html(preview, "help.menu")

        self.assertEqual({"title": "RayleaBot"}, preview_data)
        self.assertEqual(preview["source_digest"], source_digest)

        preview["html"] = ""
        with self.assertRaises(self_host_smoke.SmokeError):
            self_host_smoke.validate_render_template_preview_html(preview, "help.menu")


    def test_template_preview_rejects_retired_revision_and_unknown_fields(self):
        valid = yaml.safe_load((ROOT / "fixtures/web-api/ok.system-render-template-preview-html.yaml").read_text(encoding="utf-8"))["response"]["body"]
        for payload in [
            {**valid, "revision_id": "retired"},
            {**{key: value for key, value in valid.items() if key != "source_digest"}, "revision_id": "retired"},
            {**valid, "width": True},
        ]:
            with self.subTest(payload=payload), self.assertRaises(self_host_smoke.SmokeError):
                self_host_smoke.validate_render_template_preview_html(payload, "help.menu")

    def test_png_probe_reads_owned_file_signature_and_dimensions(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "templates/help.menu").mkdir(parents=True)
            (root / "templates/help.menu/template.json").write_text('{"width":960}')
            (root / "data/render").mkdir(parents=True)
            artifact = "artifact_" + "a" * 24
            image = root / "data/render" / (artifact + ".png")
            fields = {"artifact_id": artifact, "mime": "image/png"}
            image.write_bytes(b"\x89PNG\r\n\x1a\n" + struct.pack(">I", 13) + b"IHDR" + struct.pack(">II", 960, 640))
            self.assertEqual(self_host_smoke.verify_probe_png(root, fields), (960, 640))
            image.write_bytes(b"text fallback")
            with self.assertRaises(self_host_smoke.SmokeError):
                self_host_smoke.verify_probe_png(root, fields)
            with self.assertRaises(self_host_smoke.SmokeError):
                self_host_smoke.verify_probe_png(root, {"artifact_id": "../other", "mime": "image/png"})

    def test_real_owned_child_exit_is_observed_without_killing_it(self):
        with subprocess.Popen([sys.executable, "-c", "import sys;sys.stdin.read()"], stdin=subprocess.PIPE) as process:
            witness = self_host_smoke.ProcessWitness(process.pid)
            try:
                self.assertFalse(witness.exited())
                process.communicate(input=b"", timeout=10)
                witness.wait_exit(timeout=2)
            finally:
                witness.close()

    def test_browser_ownership_rejects_unrelated_browser_even_with_similar_profile(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            profile = root / "rayleabot-chromium-fixture"
            profile.mkdir()
            with mock.patch.object(self_host_smoke, "descendant_commands", return_value=[]):
                with self.assertRaises(self_host_smoke.SmokeError):
                    self_host_smoke.BrowserOwnership(123, root)
            self.assertTrue(profile.exists())

    def test_plugin_task_uses_the_formal_status_and_checks_its_terminal_log(self):
        body = {"task_id": "task_fixture", "status": "succeeded"}
        with mock.patch.object(self_host_smoke, "request_json", return_value=body), mock.patch.object(self_host_smoke, "poll_task") as log:
            self_host_smoke.wait_plugin_task("http://127.0.0.1/", "fixture-token", {"task_id": "task_fixture"}, "plugin.install")
            self.assertEqual(log.call_args.kwargs["expected_task_type"], "plugin.install")

    def test_smoke_workspace_preserves_primary_failure_and_evidence(self):
        with tempfile.TemporaryDirectory() as parent:
            root = Path(parent) / "owned"
            root.mkdir()
            original = self_host_smoke.SmokeError("original render failure")
            with mock.patch.object(self_host_smoke.tempfile, "mkdtemp", return_value=str(root)), redirect_stdout(io.StringIO()):
                with self.assertRaises(self_host_smoke.SmokeError) as caught:
                    with self_host_smoke.smoke_workspace() as workspace:
                        (workspace / "server-output.log").write_text("captured evidence")
                        raise original
            self.assertIs(caught.exception, original)
            self.assertEqual((root / "server-output.log").read_text(), "captured evidence")

    def test_smoke_workspace_removes_its_successful_run_only(self):
        with tempfile.TemporaryDirectory() as parent:
            root = Path(parent) / "owned"
            root.mkdir()
            other = Path(parent) / "other"
            other.mkdir()
            with mock.patch.object(self_host_smoke.tempfile, "mkdtemp", return_value=str(root)):
                with self_host_smoke.smoke_workspace():
                    pass
            self.assertFalse(root.exists())
            self.assertTrue(other.exists())


if __name__ == "__main__":
    unittest.main()
