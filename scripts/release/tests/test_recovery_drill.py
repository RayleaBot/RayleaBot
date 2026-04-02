import subprocess
import sys
import tarfile
import time
import unittest
import urllib.error
import zipfile
from tempfile import TemporaryDirectory
from pathlib import Path
from unittest import mock

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

    def test_select_previous_release_requires_release_manifest_asset(self) -> None:
        releases = [
            {
                "tag_name": "v0.2.0",
                "draft": False,
                "prerelease": False,
                "assets": [{"name": "release_manifest.json"}],
            },
            {
                "tag_name": "v0.1.0",
                "draft": False,
                "prerelease": False,
                "assets": [{"name": "some-other-asset"}],
            },
        ]

        selected = recovery_drill.select_previous_release(releases, "0.3.0")

        self.assertIsNotNone(selected)
        self.assertEqual("v0.2.0", selected["tag_name"])

    def test_compare_versions_ignores_prerelease_suffixes(self) -> None:
        self.assertLess(recovery_drill.compare_versions("1.2.3", "2.0.0"), 0)
        self.assertEqual(0, recovery_drill.compare_versions("1.2.3-smoke", "1.2.3"))
        self.assertGreater(recovery_drill.compare_versions("9999.0.0-smoke", "1.2.3"), 0)

    def test_download_previous_archive_skips_when_release_api_is_inaccessible(self) -> None:
        with TemporaryDirectory() as tmp:
            download_dir = Path(tmp)
            error = urllib.error.HTTPError(
                recovery_drill.release_api_url("RayleaBot/RayleaBot"),
                404,
                "Not Found",
                hdrs=None,
                fp=None,
            )
            with mock.patch("recovery_drill.urllib.request.urlopen", side_effect=error):
                with self.assertRaises(recovery_drill.DrillBootstrapSkip) as ctx:
                    recovery_drill.download_previous_archive(
                        "RayleaBot/RayleaBot",
                        "0.3.0",
                        "windows-x64-full",
                        download_dir,
                    )

        self.assertIn("release api is not accessible", str(ctx.exception))

    def test_read_build_info_from_zip_archive(self) -> None:
        with TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            archive_path = tmp_path / "sample.zip"
            with zipfile.ZipFile(archive_path, "w") as zf:
                zf.writestr(
                    "RayleaBot-v1.2.3-windows-x64-full/build_info.json",
                    '{"version":"1.2.3","artifact_id":"windows-x64-full"}',
                )

            build_info = recovery_drill.read_build_info_from_archive("windows-x64-full", archive_path)

        self.assertEqual("1.2.3", build_info["version"])
        self.assertEqual("windows-x64-full", build_info["artifact_id"])

    def test_read_build_info_from_tar_archive(self) -> None:
        with TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            archive_path = tmp_path / "sample.tar.gz"
            payload_path = tmp_path / "build_info.json"
            payload_path.write_text('{"version":"1.2.3","artifact_id":"linux-x64-server"}', encoding="utf-8")
            with tarfile.open(archive_path, "w:gz") as tf:
                tf.add(payload_path, arcname="RayleaBot-v1.2.3-linux-x64-server/build_info.json")

            build_info = recovery_drill.read_build_info_from_archive("linux-x64-server", archive_path)

        self.assertEqual("1.2.3", build_info["version"])
        self.assertEqual("linux-x64-server", build_info["artifact_id"])

    def test_assert_recovery_summary_requires_guidance_for_degraded_summaries(self) -> None:
        with self.assertRaises(recovery_drill.DrillError):
            recovery_drill.assert_recovery_summary(
                {
                    "operation": "upgrade",
                    "phase": "post_startup",
                    "status": "degraded",
                    "requires_post_start_checks": False,
                    "issues": [],
                    "skipped_plugins": [{"plugin_id": recovery_drill.INCOMPATIBLE_PLUGIN_ID}],
                    "manual_actions": [],
                    "next_steps": [],
                },
                expected_operation="upgrade",
                expected_phase="post_startup",
                expected_statuses={"degraded"},
                requires_post_start_checks=False,
                require_skipped_plugin=True,
                require_guidance=True,
            )

    def test_assert_recovery_summary_rejects_guidance_for_compatible_summaries(self) -> None:
        with self.assertRaises(recovery_drill.DrillError):
            recovery_drill.assert_recovery_summary(
                {
                    "operation": "restore",
                    "phase": "post_startup",
                    "status": "compatible",
                    "requires_post_start_checks": False,
                    "issues": [],
                    "skipped_plugins": [],
                    "manual_actions": ["unexpected action"],
                    "next_steps": ["unexpected step"],
                },
                expected_operation="restore",
                expected_phase="post_startup",
                expected_statuses={"compatible"},
                requires_post_start_checks=False,
                require_guidance=False,
            )


if __name__ == "__main__":
    unittest.main()
