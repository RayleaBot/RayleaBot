from __future__ import annotations

import hashlib
import json
import stat
import tempfile
import unittest
import zipfile
from pathlib import Path


import sys

SCRIPT_DIR = Path(__file__).resolve().parents[1]
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

import build_plugin_artifacts


class BuildPluginArtifactsTest(unittest.TestCase):
    def test_verify_artifact_accepts_complete_windows_package(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp) / "example.plugin"
            self._write_artifact(root)

            document = build_plugin_artifacts.verify_artifact(root, "windows-x64")

            self.assertEqual("example.plugin", document["plugin_id"])

    def test_verify_artifact_rejects_unlisted_source(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp) / "example.plugin"
            self._write_artifact(root)
            (root / "main.go").write_text("package main\n", encoding="utf-8")

            with self.assertRaisesRegex(build_plugin_artifacts.ArtifactError, "inventory mismatch"):
                build_plugin_artifacts.verify_artifact(root, "windows-x64")

    def test_verify_archive_mode_requires_executable_unix_backend(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            archive_path = Path(temp) / "example.zip"
            with zipfile.ZipFile(archive_path, "w") as archive:
                entry = zipfile.ZipInfo("example.plugin/bin/example")
                entry.create_system = 3
                entry.external_attr = (stat.S_IFREG | 0o644) << 16
                archive.writestr(entry, b"\x7fELFfixture")

            with self.assertRaisesRegex(build_plugin_artifacts.ArtifactError, "mode is not 0755"):
                build_plugin_artifacts.verify_archive_mode(
                    archive_path,
                    "example.plugin",
                    "bin/example",
                    "linux-x64",
                )

    @staticmethod
    def _write_artifact(root: Path) -> None:
        (root / "bin").mkdir(parents=True)
        (root / "ui").mkdir()
        manifest = {
            "id": "example.plugin",
            "name": "Example",
            "version": "0.2.0",
            "manifest_version": "2",
            "plugin_protocol_version": "1",
            "runtime": "go",
            "entry": "bin/example",
            "platforms": ["windows-x64", "linux-x64", "macos-arm64"],
            "management_ui": {"pages": [{"id": "main", "label": "Main", "entry": "ui/index.html"}]},
        }
        info = json.dumps(manifest, ensure_ascii=False, separators=(",", ":")).encode()
        (root / "info.json").write_bytes(info)
        (root / "bin" / "example.exe").write_bytes(b"MZ\x00\x00fixture")
        (root / "ui" / "index.html").write_text("<!doctype html>", encoding="utf-8")

        files = []
        roles = {
            "bin/example.exe": "backend",
            "info.json": "manifest",
            "ui/index.html": "ui",
        }
        for relative, role in roles.items():
            payload = (root / relative).read_bytes()
            files.append(
                {
                    "path": relative,
                    "role": role,
                    "size": len(payload),
                    "sha256": hashlib.sha256(payload).hexdigest(),
                }
            )
        artifact = {
            "artifact_version": "1",
            "plugin_id": "example.plugin",
            "plugin_version": "0.2.0",
            "target_platform": "windows-x64",
            "manifest_sha256": hashlib.sha256(info).hexdigest(),
            "files": files,
        }
        (root / "artifact.json").write_text(json.dumps(artifact), encoding="utf-8")


if __name__ == "__main__":
    unittest.main()
