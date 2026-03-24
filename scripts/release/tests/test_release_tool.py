import json
import sys
import tarfile
import tempfile
import unittest
import zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "scripts" / "release"))

import release_tool


class ReleaseToolTests(unittest.TestCase):
    def test_package_metadata_and_verify_windows_bundle(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            temp = Path(tmp)
            server_bin = temp / "raylea-server.exe"
            launcher_bin = temp / "RayleaBot.Launcher.exe"
            web_dist = temp / "web-dist"
            builtin = temp / "builtin"
            deps = temp / ".deps"
            default_config = temp / "config" / "default.yaml"
            output = temp / "out"

            server_bin.write_text("server", encoding="utf-8")
            launcher_bin.write_text("launcher", encoding="utf-8")
            (web_dist / "index.html").parent.mkdir(parents=True, exist_ok=True)
            (web_dist / "index.html").write_text("<html></html>", encoding="utf-8")
            (builtin / "help" / "info.json").parent.mkdir(parents=True, exist_ok=True)
            (builtin / "help" / "info.json").write_text("{}", encoding="utf-8")
            (deps / "manifest.json").parent.mkdir(parents=True, exist_ok=True)
            (deps / "manifest.json").write_text('{"manifest_version":1,"resources":[]}', encoding="utf-8")
            default_config.parent.mkdir(parents=True, exist_ok=True)
            default_config.write_text("schema_version: \"2\"\n", encoding="utf-8")

            archive_path, sidecar = release_tool.stage_release_root(
                artifact_id="windows-x64-full",
                version="0.1.0",
                git_commit="abcdef1",
                built_at="2026-03-24T10:00:00Z",
                output_dir=output,
                server_bin=server_bin,
                web_dist=web_dist,
                builtin_dir=builtin,
                deps_dir=deps,
                default_config=default_config,
                launcher_bin=launcher_bin,
                systemd_file=None,
                release_notes_ref="https://example.invalid/releases/v0.1.0",
            )

            self.assertTrue(archive_path.exists())
            with zipfile.ZipFile(archive_path) as zf:
                names = set(zf.namelist())
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/build_info.json", names)
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/RayleaLauncher.exe", names)
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/config/default.yaml", names)

            manifest_path, checksums_path = release_tool.build_release_metadata(
                version="0.1.0",
                git_commit="abcdef1",
                built_at="2026-03-24T10:00:00Z",
                config_schema_version="2",
                db_schema_version="13",
                plugin_protocol_version="1",
                release_notes_ref="https://example.invalid/releases/v0.1.0",
                deps_manifest=deps / "manifest.json",
                sidecars=[sidecar],
                output_dir=output / "release",
            )

            manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
            self.assertEqual(manifest["artifacts"][0]["artifact_id"], "windows-x64-full")
            self.assertEqual(manifest["artifacts"][0]["smoke_profile"], "windows_full_smoke")
            self.assertIn("release_manifest.json", checksums_path.read_text(encoding="utf-8"))

            release_tool.verify_release_bundle(manifest_path, checksums_path, output)

    def test_package_linux_bundle_includes_systemd_file(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            temp = Path(tmp)
            server_bin = temp / "raylea-server"
            web_dist = temp / "web-dist"
            builtin = temp / "builtin"
            deps = temp / ".deps"
            default_config = temp / "config" / "default.yaml"
            systemd_file = temp / "rayleabot.service"
            output = temp / "out"

            server_bin.write_text("server", encoding="utf-8")
            (web_dist / "index.html").parent.mkdir(parents=True, exist_ok=True)
            (web_dist / "index.html").write_text("<html></html>", encoding="utf-8")
            (builtin / "help" / "info.json").parent.mkdir(parents=True, exist_ok=True)
            (builtin / "help" / "info.json").write_text("{}", encoding="utf-8")
            (deps / "manifest.json").parent.mkdir(parents=True, exist_ok=True)
            (deps / "manifest.json").write_text('{"manifest_version":1,"resources":[]}', encoding="utf-8")
            default_config.parent.mkdir(parents=True, exist_ok=True)
            default_config.write_text("schema_version: \"2\"\n", encoding="utf-8")
            systemd_file.write_text("[Service]\nExecStart=/opt/raylea/raylea-server\n", encoding="utf-8")

            archive_path, _ = release_tool.stage_release_root(
                artifact_id="linux-x64-server",
                version="0.1.0",
                git_commit="abcdef1",
                built_at="2026-03-24T10:00:00Z",
                output_dir=output,
                server_bin=server_bin,
                web_dist=web_dist,
                builtin_dir=builtin,
                deps_dir=deps,
                default_config=default_config,
                launcher_bin=None,
                systemd_file=systemd_file,
                release_notes_ref=None,
            )

            with tarfile.open(archive_path, "r:gz") as tf:
                names = set(tf.getnames())
            self.assertIn("RayleaBot-v0.1.0-linux-x64-server/systemd/rayleabot.service", names)


if __name__ == "__main__":
    unittest.main()
