import json
import jsonschema
import io
import shutil
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
    def test_windows_launcher_bundle_requires_runtime_guide(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            bundle = Path(tmp)
            (bundle / "RayleaLauncher.exe").write_text("wails", encoding="utf-8")

            with self.assertRaises(ValueError) as ctx:
                release_tool.assert_windows_launcher_bundle_layout(bundle)

        self.assertIn("WINDOWS-RUNTIME.md", str(ctx.exception))

    def test_package_windows_bundle_and_unsigned_metadata(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            temp = Path(tmp)
            server_bin = temp / "raylea-server.exe"
            launcher_bundle = temp / "win-unpacked"
            web_dist = temp / "web-dist"
            deps = temp / ".deps"
            templates = temp / "templates"
            license_file = temp / "LICENSE"
            notices_file = temp / "THIRD_PARTY_NOTICES.md"
            output = temp / "out"

            server_bin.write_text("server", encoding="utf-8")
            license_file.write_text("AGPL", encoding="utf-8")
            notices_file.write_text("notices", encoding="utf-8")
            (launcher_bundle / "RayleaLauncher.exe").parent.mkdir(parents=True, exist_ok=True)
            (launcher_bundle / "RayleaLauncher.exe").write_text("wails", encoding="utf-8")
            (launcher_bundle / "WINDOWS-RUNTIME.md").write_text(
                "Microsoft Edge WebView2 Runtime is required.\n",
                encoding="utf-8",
            )
            (web_dist / "index.html").parent.mkdir(parents=True, exist_ok=True)
            (web_dist / "index.html").write_text("<html></html>", encoding="utf-8")
            (web_dist / "app.js.map").write_text("source map", encoding="utf-8")
            (web_dist / "README.md").write_text("dev docs", encoding="utf-8")
            (deps / "manifest.json").parent.mkdir(parents=True, exist_ok=True)
            (deps / "manifest.json").write_text('{"manifest_version":5,"resources":[]}', encoding="utf-8")
            (deps / "store" / "python" / "3.12").mkdir(parents=True, exist_ok=True)
            (deps / "store" / "python" / "3.12" / "python.exe").write_text("runtime", encoding="utf-8")
            (deps / "cache" / "downloads").mkdir(parents=True, exist_ok=True)
            (deps / "cache" / "downloads" / "python.zip").write_text("download", encoding="utf-8")
            (templates / "help.menu" / "template.json").parent.mkdir(parents=True, exist_ok=True)
            (templates / "help.menu" / "template.json").write_text("{}", encoding="utf-8")
            (templates / "help.menu" / "template.test.mjs").write_text("test", encoding="utf-8")
            (templates / "status.panel" / "template.json").parent.mkdir(parents=True, exist_ok=True)
            (templates / "status.panel" / "template.json").write_text("{}", encoding="utf-8")

            archive_path, sidecar = release_tool.stage_release_root(
                artifact_id="windows-x64-full",
                version="0.1.0",
                git_commit="abcdef1",
                built_at="2026-03-24T10:00:00Z",
                output_dir=output,
                server_bin=server_bin,
                web_dist=web_dist,
                deps_dir=deps,
                templates_dir=templates,
                launcher_bundle=launcher_bundle,
                systemd_file=None,
                release_notes_ref="https://example.invalid/releases/v0.1.0",
                license_file=license_file,
                third_party_notices=notices_file,
            )

            self.assertTrue(archive_path.exists())
            with zipfile.ZipFile(archive_path) as zf:
                names = set(zf.namelist())
                build_info = json.loads(
                    zf.read("RayleaBot-v0.1.0-windows-x64-full/build_info.json").decode("utf-8")
                )
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/build_info.json", names)
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/RayleaLauncher.exe", names)
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/WINDOWS-RUNTIME.md", names)
            self.assertNotIn("RayleaBot-v0.1.0-windows-x64-full/raylea-updater.exe", names)
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/LICENSE", names)
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/THIRD_PARTY_NOTICES.md", names)
            self.assertFalse(any("app.asar" in name or "/launcher/" in name for name in names))
            self.assertNotIn("RayleaBot-v0.1.0-windows-x64-full/config/default.yaml", names)
            self.assertNotIn("RayleaBot-v0.1.0-windows-x64-full/contracts/config.user.schema.json", names)
            self.assertNotIn("RayleaBot-v0.1.0-windows-x64-full/contracts/plugin-info.schema.json", names)
            self.assertNotIn("RayleaBot-v0.1.0-windows-x64-full/web/dist/app.js.map", names)
            self.assertNotIn("RayleaBot-v0.1.0-windows-x64-full/web/dist/README.md", names)
            self.assertFalse(any(name.endswith((".go", ".py", ".ts", ".vue")) for name in names))
            self.assertNotIn("RayleaBot-v0.1.0-windows-x64-full/.deps/store/python/3.12/python.exe", names)
            self.assertNotIn("RayleaBot-v0.1.0-windows-x64-full/.deps/cache/downloads/python.zip", names)
            self.assertNotIn("RayleaBot-v0.1.0-windows-x64-full/templates/help.menu/template.test.mjs", names)
            self.assertFalse(any("/plugins/" in name for name in names))
            self.assertNotIn(
                "RayleaBot-v0.1.0-windows-x64-full/sdk/python/pyproject.toml",
                names,
            )
            self.assertNotIn(
                "RayleaBot-v0.1.0-windows-x64-full/sdk/nodejs/src/index.ts",
                names,
            )
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/templates/help.menu/template.json", names)
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/templates/status.panel/template.json", names)
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/web/dist/index.html", names)
            self.assertEqual("https://example.invalid/releases/v0.1.0", build_info["release_notes_ref"])
            self.assertEqual("3", build_info["plugin_manifest_version"])
            self.assertEqual("3", build_info["plugin_ui_bridge_version"])

            sidecar_path = archive_path.with_suffix(archive_path.suffix + ".artifact.json")
            relocated = temp / "downloaded"
            relocated.mkdir()
            shutil.copy2(archive_path, relocated / archive_path.name)
            shutil.copy2(sidecar_path, relocated / sidecar_path.name)
            loaded_sidecar = release_tool.load_sidecar(relocated / sidecar_path.name)
            self.assertEqual(relocated / archive_path.name, loaded_sidecar.archive_path)

            manifest_path = release_tool.build_release_metadata(
                version="0.1.0",
                git_commit="abcdef1",
                built_at="2026-03-24T10:00:00Z",
                config_schema_version="4",
                db_schema_version="000001",
                plugin_protocol_version="3",
                release_notes_ref="https://example.invalid/releases/v0.1.0",
                sidecars=[sidecar],
                output_dir=output / "release",
            )

            manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
            schema = json.loads((ROOT / "contracts/release-manifest.schema.json").read_text(encoding="utf-8"))
            jsonschema.Draft202012Validator(schema, format_checker=jsonschema.FormatChecker()).validate(manifest)
            jsonschema.Draft202012Validator(schema, format_checker=jsonschema.FormatChecker()).validate(build_info)
            self.assertEqual(manifest["artifacts"][0]["artifact_id"], "windows-x64-full")
            self.assertEqual(manifest["artifacts"][0]["smoke_profile"], "windows_full_smoke")
            self.assertEqual(2, manifest["manifest_version"])
            self.assertEqual("3", manifest["plugin_protocol_version"])
            self.assertEqual("3", manifest["plugin_manifest_version"])
            self.assertEqual("3", manifest["plugin_ui_bridge_version"])
            self.assertEqual("guided", manifest["artifacts"][0]["update_mode"])
            self.assertNotIn("sha256", manifest["artifacts"][0])
            self.assertFalse((manifest_path.parent / "release_manifest.v2.sig.json").exists())
            self.assertFalse((manifest_path.parent / "SHA256SUMS.txt").exists())


    def test_metadata_rejects_manifest_outside_schema(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            temp = Path(tmp)
            archive = temp / "RayleaBot-v0.1.0-windows-x64-full.zip"
            archive.write_bytes(b"archive")
            sidecar = release_tool.ArtifactSidecar(
                artifact_id="windows-x64-full",
                archive_path=archive,
                file_name=archive.name,
                platform="windows-x64",
                support_level="first_class",
                smoke_profile="windows_full_smoke",
                expanded_size_bytes=1,
                file_count=1,
                update_mode="guided",
            )

            with self.assertRaises(jsonschema.ValidationError):
                release_tool.build_release_metadata(
                    version="0.1.0",
                    git_commit="abcdef1",
                    built_at="2026-03-24T10:00:00Z",
                    config_schema_version="4",
                    db_schema_version="000001",
                    plugin_protocol_version="3",
                    release_notes_ref="http://example.invalid/releases/v0.1.0",
                    sidecars=[sidecar],
                    output_dir=temp / "release",
                )

            self.assertFalse((temp / "release" / "release_manifest.v2.json").exists())

    def test_launcher_bundle_rejects_development_sources(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            source_path = Path(tmp) / "src" / "main.go"
            source_path.parent.mkdir(parents=True)
            source_path.write_text("package main\n", encoding="utf-8")

            with self.assertRaises(ValueError) as ctx:
                release_tool.assert_launcher_bundle_clean(Path(tmp))

        self.assertIn("src/main.go", str(ctx.exception))

    def test_package_linux_desktop_bundle_places_launcher_at_release_root(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            temp = Path(tmp)
            server_bin = temp / "raylea-server"
            launcher_bundle = temp / "linux-unpacked"
            web_dist = temp / "web-dist"
            deps = temp / ".deps"
            templates = temp / "templates"
            license_file = temp / "LICENSE"
            notices_file = temp / "THIRD_PARTY_NOTICES.md"
            output = temp / "out"

            server_bin.write_text("server", encoding="utf-8")
            license_file.write_text("AGPL", encoding="utf-8")
            notices_file.write_text("notices", encoding="utf-8")
            (launcher_bundle / "RayleaLauncher").parent.mkdir(parents=True, exist_ok=True)
            (launcher_bundle / "RayleaLauncher").write_text("launcher", encoding="utf-8")
            (launcher_bundle / "LINUX-RUNTIME.md").write_text(
                "GTK 3 and WebKit2GTK 4.1 are required.\n",
                encoding="utf-8",
            )
            (web_dist / "index.html").parent.mkdir(parents=True, exist_ok=True)
            (web_dist / "index.html").write_text("<html></html>", encoding="utf-8")
            (deps / "manifest.json").parent.mkdir(parents=True, exist_ok=True)
            (deps / "manifest.json").write_text('{"manifest_version":5,"resources":[]}', encoding="utf-8")
            (templates / "help.menu" / "template.json").parent.mkdir(parents=True, exist_ok=True)
            (templates / "help.menu" / "template.json").write_text("{}", encoding="utf-8")
            (templates / "status.panel" / "template.json").parent.mkdir(parents=True, exist_ok=True)
            (templates / "status.panel" / "template.json").write_text("{}", encoding="utf-8")

            archive_path, _ = release_tool.stage_release_root(
                artifact_id="linux-x64-full",
                version="0.1.0",
                git_commit="abcdef1",
                built_at="2026-03-24T10:00:00Z",
                output_dir=output,
                server_bin=server_bin,
                web_dist=web_dist,
                deps_dir=deps,
                templates_dir=templates,
                launcher_bundle=launcher_bundle,
                systemd_file=None,
                release_notes_ref=None,
                license_file=license_file,
                third_party_notices=notices_file,
            )

            with tarfile.open(archive_path, "r:gz") as tf:
                names = set(tf.getnames())
            self.assertIn("RayleaBot-v0.1.0-linux-x64-full/RayleaLauncher", names)
            self.assertIn("RayleaBot-v0.1.0-linux-x64-full/LINUX-RUNTIME.md", names)
            self.assertIn("RayleaBot-v0.1.0-linux-x64-full/LICENSE", names)
            self.assertIn("RayleaBot-v0.1.0-linux-x64-full/THIRD_PARTY_NOTICES.md", names)
            self.assertNotIn("RayleaBot-v0.1.0-linux-x64-full/contracts/config.user.schema.json", names)
            self.assertIn("RayleaBot-v0.1.0-linux-x64-full/web/dist/index.html", names)
            self.assertIn("RayleaBot-v0.1.0-linux-x64-full/templates/help.menu/template.json", names)

    def test_package_macos_desktop_bundle_includes_app_bundle(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            temp = Path(tmp)
            server_bin = temp / "raylea-server"
            launcher_bundle = temp / "RayleaLauncher.app"
            web_dist = temp / "web-dist"
            deps = temp / ".deps"
            templates = temp / "templates"
            license_file = temp / "LICENSE"
            notices_file = temp / "THIRD_PARTY_NOTICES.md"
            output = temp / "out"

            server_bin.write_text("server", encoding="utf-8")
            license_file.write_text("AGPL", encoding="utf-8")
            notices_file.write_text("notices", encoding="utf-8")
            mac_binary = launcher_bundle / "Contents" / "MacOS" / "RayleaLauncher"
            mac_binary.parent.mkdir(parents=True, exist_ok=True)
            mac_binary.write_text("launcher", encoding="utf-8")
            plist = launcher_bundle / "Contents" / "Info.plist"
            plist.write_text("<plist/>", encoding="utf-8")
            (web_dist / "index.html").parent.mkdir(parents=True, exist_ok=True)
            (web_dist / "index.html").write_text("<html></html>", encoding="utf-8")
            (deps / "manifest.json").parent.mkdir(parents=True, exist_ok=True)
            (deps / "manifest.json").write_text('{"manifest_version":5,"resources":[]}', encoding="utf-8")
            (templates / "help.menu" / "template.json").parent.mkdir(parents=True, exist_ok=True)
            (templates / "help.menu" / "template.json").write_text("{}", encoding="utf-8")
            (templates / "status.panel" / "template.json").parent.mkdir(parents=True, exist_ok=True)
            (templates / "status.panel" / "template.json").write_text("{}", encoding="utf-8")

            archive_path, _ = release_tool.stage_release_root(
                artifact_id="macos-arm64-full",
                version="0.1.0",
                git_commit="abcdef1",
                built_at="2026-03-24T10:00:00Z",
                output_dir=output,
                server_bin=server_bin,
                web_dist=web_dist,
                deps_dir=deps,
                templates_dir=templates,
                launcher_bundle=launcher_bundle,
                systemd_file=None,
                release_notes_ref=None,
                license_file=license_file,
                third_party_notices=notices_file,
            )

            with tarfile.open(archive_path, "r:gz") as tf:
                names = set(tf.getnames())
            self.assertIn("RayleaBot-v0.1.0-macos-arm64-full/RayleaLauncher.app/Contents/MacOS/RayleaLauncher", names)
            self.assertIn("RayleaBot-v0.1.0-macos-arm64-full/RayleaLauncher.app/Contents/Info.plist", names)
            self.assertIn("RayleaBot-v0.1.0-macos-arm64-full/LICENSE", names)
            self.assertIn("RayleaBot-v0.1.0-macos-arm64-full/THIRD_PARTY_NOTICES.md", names)
            self.assertNotIn("RayleaBot-v0.1.0-macos-arm64-full/contracts/plugin-info.schema.json", names)
            self.assertIn("RayleaBot-v0.1.0-macos-arm64-full/web/dist/index.html", names)
            self.assertIn("RayleaBot-v0.1.0-macos-arm64-full/templates/status.panel/template.json", names)

    def test_package_linux_bundle_includes_systemd_file(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            temp = Path(tmp)
            server_bin = temp / "raylea-server"
            web_dist = temp / "web-dist"
            deps = temp / ".deps"
            templates = temp / "templates"
            systemd_file = temp / "rayleabot.service"
            license_file = temp / "LICENSE"
            notices_file = temp / "THIRD_PARTY_NOTICES.md"
            output = temp / "out"

            server_bin.write_text("server", encoding="utf-8")
            license_file.write_text("AGPL", encoding="utf-8")
            notices_file.write_text("notices", encoding="utf-8")
            (web_dist / "index.html").parent.mkdir(parents=True, exist_ok=True)
            (web_dist / "index.html").write_text("<html></html>", encoding="utf-8")
            (deps / "manifest.json").parent.mkdir(parents=True, exist_ok=True)
            (deps / "manifest.json").write_text('{"manifest_version":5,"resources":[]}', encoding="utf-8")
            (templates / "help.menu" / "template.json").parent.mkdir(parents=True, exist_ok=True)
            (templates / "help.menu" / "template.json").write_text("{}", encoding="utf-8")
            (templates / "status.panel" / "template.json").parent.mkdir(parents=True, exist_ok=True)
            (templates / "status.panel" / "template.json").write_text("{}", encoding="utf-8")
            systemd_file.write_text("[Service]\nExecStart=/opt/raylea/raylea-server\n", encoding="utf-8")

            archive_path, _ = release_tool.stage_release_root(
                artifact_id="linux-x64-server",
                version="0.1.0",
                git_commit="abcdef1",
                built_at="2026-03-24T10:00:00Z",
                output_dir=output,
                server_bin=server_bin,
                web_dist=web_dist,
                deps_dir=deps,
                templates_dir=templates,
                launcher_bundle=None,
                systemd_file=systemd_file,
                release_notes_ref=None,
                license_file=license_file,
                third_party_notices=notices_file,
            )

            with tarfile.open(archive_path, "r:gz") as tf:
                names = set(tf.getnames())
            self.assertIn("RayleaBot-v0.1.0-linux-x64-server/systemd/rayleabot.service", names)
            self.assertIn("RayleaBot-v0.1.0-linux-x64-server/LICENSE", names)
            self.assertIn("RayleaBot-v0.1.0-linux-x64-server/THIRD_PARTY_NOTICES.md", names)
            self.assertNotIn("RayleaBot-v0.1.0-linux-x64-server/contracts/config.user.schema.json", names)
            self.assertIn("RayleaBot-v0.1.0-linux-x64-server/web/dist/index.html", names)
            self.assertIn("RayleaBot-v0.1.0-linux-x64-server/templates/help.menu/template.json", names)


if __name__ == "__main__":
    unittest.main()
