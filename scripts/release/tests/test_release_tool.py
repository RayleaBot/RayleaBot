import json
import hashlib
import base64
import shutil
import subprocess
import sys
import tarfile
import tempfile
import unittest
import zipfile
import struct
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "scripts" / "release"))

import release_tool


def write_test_asar(path: Path, entries: list[str]) -> None:
    root: dict[str, object] = {"files": {}}
    for entry in entries:
        node = root
        for part in Path(entry).parts:
            files = node.setdefault("files", {})
            assert isinstance(files, dict)
            child = files.setdefault(part, {"files": {}})
            assert isinstance(child, dict)
            node = child
    raw_header = json.dumps(root, separators=(",", ":")).encode("utf-8")
    padded_size = (len(raw_header) + 3) & ~3
    padded_header = raw_header + (b"\0" * (padded_size - len(raw_header)))
    prefix = struct.pack("<IIII", 4, padded_size + 8, padded_size + 4, padded_size)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(prefix + padded_header)


class ReleaseToolTests(unittest.TestCase):
    @unittest.skipUnless(shutil.which("openssl"), "OpenSSL is required")
    def test_sign_release_manifest_emits_exact_dual_signed_ed25519_envelope(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            private_key = root / "release.pem"
            next_private_key = root / "release-next.pem"
            manifest = root / "release_manifest.v2.json"
            signature = root / "release_manifest.v2.sig.json"
            manifest.write_bytes(b'{"manifest_version":2}\n')
            subprocess.run(
                ["openssl", "genpkey", "-algorithm", "ED25519", "-out", str(private_key)],
                check=True,
                capture_output=True,
            )
            subprocess.run(
                ["openssl", "genpkey", "-algorithm", "ED25519", "-out", str(next_private_key)],
                check=True,
                capture_output=True,
            )

            release_tool.sign_release_manifest(
                manifest,
                signature,
                [("release-2026", private_key), ("release-2027", next_private_key)],
            )

            envelope = json.loads(signature.read_text(encoding="utf-8"))
            self.assertEqual("ed25519", envelope["algorithm"])
            self.assertEqual("release-2026", envelope["key_id"])
            self.assertEqual(hashlib.sha256(manifest.read_bytes()).hexdigest(), envelope["manifest_sha256"])
            self.assertEqual(["release-2026", "release-2027"], [item["key_id"] for item in envelope["signatures"]])
            for item in envelope["signatures"]:
                self.assertEqual(64, len(base64.urlsafe_b64decode(item["signature"])))

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
            updater_bin = temp / "raylea-updater.exe"
            license_file = temp / "LICENSE"
            notices_file = temp / "THIRD_PARTY_NOTICES.md"
            output = temp / "out"

            server_bin.write_text("server", encoding="utf-8")
            updater_bin.write_text("updater", encoding="utf-8")
            license_file.write_text("AGPL", encoding="utf-8")
            notices_file.write_text("notices", encoding="utf-8")
            (launcher_bundle / "RayleaLauncher.exe").parent.mkdir(parents=True, exist_ok=True)
            (launcher_bundle / "RayleaLauncher.exe").write_text("entry", encoding="utf-8")
            (launcher_bundle / "launcher" / "RayleaLauncher.exe").parent.mkdir(parents=True, exist_ok=True)
            (launcher_bundle / "launcher" / "RayleaLauncher.exe").write_text("electron", encoding="utf-8")
            write_test_asar(
                launcher_bundle / "launcher" / "resources" / "app.asar",
                ["dist/main/main/index.js", "node_modules/yaml/package.json", "package.json"],
            )
            (launcher_bundle / "launcher" / "locales" / "zh-CN.pak").parent.mkdir(parents=True, exist_ok=True)
            (launcher_bundle / "launcher" / "locales" / "zh-CN.pak").write_text("locale", encoding="utf-8")
            (launcher_bundle / "launcher" / "libEGL.dll").write_text("dll", encoding="utf-8")
            (web_dist / "index.html").parent.mkdir(parents=True, exist_ok=True)
            (web_dist / "index.html").write_text("<html></html>", encoding="utf-8")
            (web_dist / "app.js.map").write_text("source map", encoding="utf-8")
            (web_dist / "README.md").write_text("dev docs", encoding="utf-8")
            (builtin / "fortune").mkdir(parents=True, exist_ok=True)
            (builtin / "fortune" / "info.json").write_text("{}", encoding="utf-8")
            (builtin / "fortune" / "main.py").write_text("print('fortune')\n", encoding="utf-8")
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
                updater_bin=updater_bin,
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
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/raylea-updater.exe", names)
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/LICENSE", names)
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/THIRD_PARTY_NOTICES.md", names)
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/launcher/RayleaLauncher.exe", names)
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/launcher/resources/app.asar", names)
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/launcher/locales/zh-CN.pak", names)
            self.assertIn("RayleaBot-v0.1.0-windows-x64-full/launcher/libEGL.dll", names)
            self.assertNotIn("RayleaBot-v0.1.0-windows-x64-full/resources/app.asar", names)
            self.assertNotIn("RayleaBot-v0.1.0-windows-x64-full/locales/zh-CN.pak", names)
            self.assertNotIn("RayleaBot-v0.1.0-windows-x64-full/libEGL.dll", names)
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
            self.assertFalse(any("/plugins/runtime/" in name for name in names))
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
            self.assertEqual(2, build_info["update_protocol_version"])

            sidecar_path = archive_path.with_suffix(archive_path.suffix + ".artifact.json")
            relocated = temp / "downloaded"
            relocated.mkdir()
            shutil.copy2(archive_path, relocated / archive_path.name)
            shutil.copy2(sidecar_path, relocated / sidecar_path.name)
            loaded_sidecar = release_tool.load_sidecar(relocated / sidecar_path.name)
            self.assertEqual(relocated / archive_path.name, loaded_sidecar.archive_path)

            manifest_path, checksums_path = release_tool.build_release_metadata(
                version="0.1.0",
                git_commit="abcdef1",
                built_at="2026-03-24T10:00:00Z",
                config_schema_version="2",
                db_schema_version="000004",
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
            self.assertEqual(2, manifest["manifest_version"])
            self.assertEqual("guided", manifest["artifacts"][0]["update_mode"])
            self.assertIn("release_manifest.v2.json", checksums_path.read_text(encoding="utf-8"))
            self.assertEqual(release_tool.sha256_file(manifest_path), checksums["release_manifest.v2.json"])

            release_tool.verify_release_bundle(manifest_path, checksums_path, output)

    def test_launcher_asar_rejects_unbundled_renderer_dependencies(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            asar_path = Path(tmp) / "resources" / "app.asar"
            write_test_asar(
                asar_path,
                ["dist/main/main/index.js", "node_modules/react/index.js", "package.json"],
            )

            with self.assertRaises(ValueError) as ctx:
                release_tool.assert_launcher_bundle_clean(Path(tmp))

        self.assertIn("node_modules/react", str(ctx.exception))

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
            license_file = temp / "LICENSE"
            notices_file = temp / "THIRD_PARTY_NOTICES.md"
            output = temp / "out"

            server_bin.write_text("server", encoding="utf-8")
            license_file.write_text("AGPL", encoding="utf-8")
            notices_file.write_text("notices", encoding="utf-8")
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
                updater_bin=None,
                license_file=license_file,
                third_party_notices=notices_file,
            )

            with tarfile.open(archive_path, "r:gz") as tf:
                names = set(tf.getnames())
            self.assertIn("RayleaBot-v0.1.0-linux-x64-full/RayleaLauncher", names)
            self.assertIn("RayleaBot-v0.1.0-linux-x64-full/locales/en-US.pak", names)
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
            builtin = temp / "builtin"
            deps = temp / ".deps"
            templates = temp / "templates"
            default_config = temp / "config" / "default.yaml"
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
                updater_bin=None,
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
            builtin = temp / "builtin"
            deps = temp / ".deps"
            templates = temp / "templates"
            default_config = temp / "config" / "default.yaml"
            systemd_file = temp / "rayleabot.service"
            license_file = temp / "LICENSE"
            notices_file = temp / "THIRD_PARTY_NOTICES.md"
            output = temp / "out"

            server_bin.write_text("server", encoding="utf-8")
            license_file.write_text("AGPL", encoding="utf-8")
            notices_file.write_text("notices", encoding="utf-8")
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
                updater_bin=None,
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
