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
            launcher_bundle = temp / "win-unpacked"
            web_dist = temp / "web-dist"
            builtin = temp / "builtin"
            deps = temp / ".deps"
            templates = temp / "templates"
            default_config = temp / "config" / "default.yaml"
            output = temp / "out"

            server_bin.write_text("server", encoding="utf-8")
            (launcher_bundle / "RayleaLauncher.exe").parent.mkdir(parents=True, exist_ok=True)
            (launcher_bundle / "RayleaLauncher.exe").write_text("launcher", encoding="utf-8")
            (launcher_bundle / "resources" / "app.asar").parent.mkdir(parents=True, exist_ok=True)
            (launcher_bundle / "resources" / "app.asar").write_text("asar", encoding="utf-8")
            (web_dist / "index.html").parent.mkdir(parents=True, exist_ok=True)
            (web_dist / "index.html").write_text("<html></html>", encoding="utf-8")
            (web_dist / "app.js.map").write_text("source map", encoding="utf-8")
            (web_dist / "README.md").write_text("dev docs", encoding="utf-8")
            (builtin / "fortune" / "web").mkdir(parents=True, exist_ok=True)
            (builtin / "fortune" / "info.json").write_text("{}", encoding="utf-8")
            (builtin / "fortune" / "main.py").write_text("print('fortune')\n", encoding="utf-8")
            (builtin / "fortune" / "web" / "index.html").write_text("<html></html>", encoding="utf-8")
            (builtin / "fortune" / "tests").mkdir(parents=True, exist_ok=True)
            (builtin / "fortune" / "tests" / "test_fortune.py").write_text("def test_fortune(): pass\n", encoding="utf-8")
            (builtin / "fortune" / "__pycache__").mkdir(parents=True, exist_ok=True)
            (builtin / "fortune" / "__pycache__" / "main.pyc").write_bytes(b"cache")
            (deps / "manifest.json").parent.mkdir(parents=True, exist_ok=True)
            (deps / "manifest.json").write_text('{"manifest_version":1,"resources":[]}', encoding="utf-8")
            (deps / "store" / "python" / "3.12").mkdir(parents=True, exist_ok=True)
            (deps / "store" / "python" / "3.12" / "python.exe").write_text("runtime", encoding="utf-8")
            (deps / "cache" / "downloads").mkdir(parents=True, exist_ok=True)
            (deps / "cache" / "downloads" / "python.zip").write_text("download", encoding="utf-8")
            (templates / "help.menu" / "template.json").parent.mkdir(parents=True, exist_ok=True)
            (templates / "help.menu" / "template.json").write_text("{}", encoding="utf-8")
            (templates / "help.menu" / "template.test.mjs").write_text("test", encoding="utf-8")
            (templates / "status.panel" / "template.json").parent.mkdir(parents=True, exist_ok=True)
            (templates / "status.panel" / "template.json").write_text("{}", encoding="utf-8")
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
                templates_dir=templates,
                default_config=default_config,
                launcher_bundle=launcher_bundle,
                systemd_file=None,
                release_notes_ref="https://example.invalid/releases/v0.1.0",
            )

            self.assertTrue(archive_path.exists())
            with zipfile.ZipFile(archive_path) as zf:
                names = set(zf.namelist())
                build_info = json.loads(
                    zf.read("RayleaBot-v0.1.0-windows-x64-full/build_info.json").decode("utf-8")
                )
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/build_info.json", names)
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/RayleaLauncher.exe", names)
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/resources/app.asar", names)
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/config/default.yaml", names)
            self.assertNotIn("RayleaBot-v0.1.0-windows-x64-full/contracts/config.user.schema.json", names)
            self.assertNotIn("RayleaBot-v0.1.0-windows-x64-full/contracts/plugin-info.schema.json", names)
            self.assertNotIn("RayleaBot-v0.1.0-windows-x64-full/web/dist/app.js.map", names)
            self.assertNotIn("RayleaBot-v0.1.0-windows-x64-full/web/dist/README.md", names)
            self.assertNotIn("RayleaBot-v0.1.0-windows-x64-full/plugins/builtin/fortune/tests/test_fortune.py", names)
            self.assertNotIn("RayleaBot-v0.1.0-windows-x64-full/plugins/builtin/fortune/__pycache__/main.pyc", names)
            self.assertNotIn("RayleaBot-v0.1.0-windows-x64-full/.deps/store/python/3.12/python.exe", names)
            self.assertNotIn("RayleaBot-v0.1.0-windows-x64-full/.deps/cache/downloads/python.zip", names)
            self.assertNotIn("RayleaBot-v0.1.0-windows-x64-full/templates/help.menu/template.test.mjs", names)
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/plugins/builtin/fortune/info.json", names)
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/plugins/builtin/fortune/main.py", names)
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/plugins/builtin/fortune/web/index.html", names)
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/templates/help.menu/template.json", names)
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/templates/status.panel/template.json", names)
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/web/dist/index.html", names)
            self.assertEqual("https://example.invalid/releases/v0.1.0", build_info["release_notes_ref"])

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
            checksums = release_tool.parse_checksums(checksums_path)
            self.assertEqual(manifest["artifacts"][0]["artifact_id"], "windows-x64-full")
            self.assertEqual(manifest["artifacts"][0]["smoke_profile"], "windows_full_smoke")
            self.assertIn("release_manifest.json", checksums_path.read_text(encoding="utf-8"))
            self.assertEqual(release_tool.sha256_file(manifest_path), checksums["release_manifest.json"])

            release_tool.verify_release_bundle(manifest_path, checksums_path, output)

    def test_package_linux_desktop_bundle_places_launcher_at_release_root(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            temp = Path(tmp)
            server_bin = temp / "raylea-server"
            launcher_bundle = temp / "linux-unpacked"
            web_dist = temp / "web-dist"
            builtin = temp / "builtin"
            deps = temp / ".deps"
            templates = temp / "templates"
            default_config = temp / "config" / "default.yaml"
            output = temp / "out"

            server_bin.write_text("server", encoding="utf-8")
            (launcher_bundle / "RayleaLauncher").parent.mkdir(parents=True, exist_ok=True)
            (launcher_bundle / "RayleaLauncher").write_text("launcher", encoding="utf-8")
            (launcher_bundle / "locales" / "en-US.pak").parent.mkdir(parents=True, exist_ok=True)
            (launcher_bundle / "locales" / "en-US.pak").write_text("locale", encoding="utf-8")
            (web_dist / "index.html").parent.mkdir(parents=True, exist_ok=True)
            (web_dist / "index.html").write_text("<html></html>", encoding="utf-8")
            (builtin / "help" / "info.json").parent.mkdir(parents=True, exist_ok=True)
            (builtin / "help" / "info.json").write_text("{}", encoding="utf-8")
            (deps / "manifest.json").parent.mkdir(parents=True, exist_ok=True)
            (deps / "manifest.json").write_text('{"manifest_version":1,"resources":[]}', encoding="utf-8")
            (templates / "help.menu" / "template.json").parent.mkdir(parents=True, exist_ok=True)
            (templates / "help.menu" / "template.json").write_text("{}", encoding="utf-8")
            (templates / "status.panel" / "template.json").parent.mkdir(parents=True, exist_ok=True)
            (templates / "status.panel" / "template.json").write_text("{}", encoding="utf-8")
            default_config.parent.mkdir(parents=True, exist_ok=True)
            default_config.write_text("schema_version: \"2\"\n", encoding="utf-8")

            archive_path, _ = release_tool.stage_release_root(
                artifact_id="linux-x64-full",
                version="0.1.0",
                git_commit="abcdef1",
                built_at="2026-03-24T10:00:00Z",
                output_dir=output,
                server_bin=server_bin,
                web_dist=web_dist,
                builtin_dir=builtin,
                deps_dir=deps,
                templates_dir=templates,
                default_config=default_config,
                launcher_bundle=launcher_bundle,
                systemd_file=None,
                release_notes_ref=None,
            )

            with tarfile.open(archive_path, "r:gz") as tf:
                names = set(tf.getnames())
            self.assertIn("RayleaBot-v0.1.0-linux-x64-full/RayleaLauncher", names)
            self.assertIn("RayleaBot-v0.1.0-linux-x64-full/locales/en-US.pak", names)
            self.assertNotIn("RayleaBot-v0.1.0-linux-x64-full/contracts/config.user.schema.json", names)
            self.assertIn("RayleaBot-v0.1.0-linux-x64-full/web/dist/index.html", names)
            self.assertIn("RayleaBot-v0.1.0-linux-x64-full/templates/help.menu/template.json", names)

    def test_package_macos_desktop_bundle_includes_app_bundle(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            temp = Path(tmp)
            server_bin = temp / "raylea-server"
            launcher_bundle = temp / "RayleaLauncher.app"
            web_dist = temp / "web-dist"
            builtin = temp / "builtin"
            deps = temp / ".deps"
            templates = temp / "templates"
            default_config = temp / "config" / "default.yaml"
            output = temp / "out"

            server_bin.write_text("server", encoding="utf-8")
            mac_binary = launcher_bundle / "Contents" / "MacOS" / "RayleaLauncher"
            mac_binary.parent.mkdir(parents=True, exist_ok=True)
            mac_binary.write_text("launcher", encoding="utf-8")
            plist = launcher_bundle / "Contents" / "Info.plist"
            plist.write_text("<plist/>", encoding="utf-8")
            (web_dist / "index.html").parent.mkdir(parents=True, exist_ok=True)
            (web_dist / "index.html").write_text("<html></html>", encoding="utf-8")
            (builtin / "help" / "info.json").parent.mkdir(parents=True, exist_ok=True)
            (builtin / "help" / "info.json").write_text("{}", encoding="utf-8")
            (deps / "manifest.json").parent.mkdir(parents=True, exist_ok=True)
            (deps / "manifest.json").write_text('{"manifest_version":1,"resources":[]}', encoding="utf-8")
            (templates / "help.menu" / "template.json").parent.mkdir(parents=True, exist_ok=True)
            (templates / "help.menu" / "template.json").write_text("{}", encoding="utf-8")
            (templates / "status.panel" / "template.json").parent.mkdir(parents=True, exist_ok=True)
            (templates / "status.panel" / "template.json").write_text("{}", encoding="utf-8")
            default_config.parent.mkdir(parents=True, exist_ok=True)
            default_config.write_text("schema_version: \"2\"\n", encoding="utf-8")

            archive_path, _ = release_tool.stage_release_root(
                artifact_id="macos-arm64-full",
                version="0.1.0",
                git_commit="abcdef1",
                built_at="2026-03-24T10:00:00Z",
                output_dir=output,
                server_bin=server_bin,
                web_dist=web_dist,
                builtin_dir=builtin,
                deps_dir=deps,
                templates_dir=templates,
                default_config=default_config,
                launcher_bundle=launcher_bundle,
                systemd_file=None,
                release_notes_ref=None,
            )

            with tarfile.open(archive_path, "r:gz") as tf:
                names = set(tf.getnames())
            self.assertIn("RayleaBot-v0.1.0-macos-arm64-full/RayleaLauncher.app/Contents/MacOS/RayleaLauncher", names)
            self.assertIn("RayleaBot-v0.1.0-macos-arm64-full/RayleaLauncher.app/Contents/Info.plist", names)
            self.assertNotIn("RayleaBot-v0.1.0-macos-arm64-full/contracts/plugin-info.schema.json", names)
            self.assertIn("RayleaBot-v0.1.0-macos-arm64-full/web/dist/index.html", names)
            self.assertIn("RayleaBot-v0.1.0-macos-arm64-full/templates/status.panel/template.json", names)

    def test_package_linux_bundle_includes_systemd_file(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            temp = Path(tmp)
            server_bin = temp / "raylea-server"
            web_dist = temp / "web-dist"
            builtin = temp / "builtin"
            deps = temp / ".deps"
            templates = temp / "templates"
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
            (templates / "help.menu" / "template.json").parent.mkdir(parents=True, exist_ok=True)
            (templates / "help.menu" / "template.json").write_text("{}", encoding="utf-8")
            (templates / "status.panel" / "template.json").parent.mkdir(parents=True, exist_ok=True)
            (templates / "status.panel" / "template.json").write_text("{}", encoding="utf-8")
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
                templates_dir=templates,
                default_config=default_config,
                launcher_bundle=None,
                systemd_file=systemd_file,
                release_notes_ref=None,
            )

            with tarfile.open(archive_path, "r:gz") as tf:
                names = set(tf.getnames())
            self.assertIn("RayleaBot-v0.1.0-linux-x64-server/systemd/rayleabot.service", names)
            self.assertNotIn("RayleaBot-v0.1.0-linux-x64-server/contracts/config.user.schema.json", names)
            self.assertIn("RayleaBot-v0.1.0-linux-x64-server/web/dist/index.html", names)
            self.assertIn("RayleaBot-v0.1.0-linux-x64-server/templates/help.menu/template.json", names)


if __name__ == "__main__":
    unittest.main()
