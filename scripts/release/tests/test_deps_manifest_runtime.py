import io
import json
import sys
import unittest
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory
from urllib.parse import urlparse
from unittest import mock

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "scripts" / "release"))

import package_runtime


class _FakeResponse(io.BytesIO):
    def __init__(self, payload: bytes, status: int = 200):
        super().__init__(payload)
        self.status = status

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc, tb):
        self.close()
        return False


class DepsManifestRuntimeTests(unittest.TestCase):
    def test_resource_metadata_requires_archive_and_entrypoints(self) -> None:
        resource = {
            "id": "python-windows-x64",
            "kind": "python-runtime",
            "version": "3.12.13",
            "platform": "windows-x64",
            "sources": [
                {
                    "url": "https://example.invalid/python.zip",
                    "kind": "upstream",
                }
            ],
            "sha256": "10b7a95b928e551fc78cac665999e1ae1f08fb738b255adb0a8d3b9c2824a9c0",
            "archive_format": "zip",
            "entrypoints": {
                "python": ["python/python.exe"],
            },
        }

        self.assertTrue(package_runtime.resource_has_complete_metadata(resource))
        resource["entrypoints"] = {}
        self.assertFalse(package_runtime.resource_has_complete_metadata(resource))

    def test_ensure_runtime_bootstrap_prepares_current_platform_resources(self) -> None:
        with TemporaryDirectory() as tmp:
            root = Path(tmp)
            deps_dir = root / ".deps"
            deps_dir.mkdir(parents=True, exist_ok=True)

            archives = {
                "https://example.invalid/chromium.zip": self._runtime_archive({
                    "chrome-win64/chrome.exe": b"chrome",
                }),
                "https://example.invalid/python.zip": self._runtime_archive({
                    "python/python.exe": b"python",
                }),
                "https://example.invalid/node.zip": self._runtime_archive({
                    "node-v24.14.0-win-x64/node.exe": b"node",
                    "node-v24.14.0-win-x64/npm.cmd": b"npm",
                }),
            }
            manifest = {
                "manifest_version": 3,
                "resources": [
                    {
                        "id": "chromium-windows-x64",
                        "kind": "chromium",
                        "version": "147.0.7727.24",
                        "platform": "windows-x64",
                        "sources": [
                            {"url": "https://example.invalid/chromium.zip", "kind": "upstream"}
                        ],
                        "sha256": package_runtime.hashlib.sha256(archives["https://example.invalid/chromium.zip"]).hexdigest(),
                        "archive_format": "zip",
                        "entrypoints": {"browser": ["chrome-win64/chrome.exe"]},
                    },
                    {
                        "id": "python-windows-x64",
                        "kind": "python-runtime",
                        "version": "3.12.13",
                        "platform": "windows-x64",
                        "sources": [
                            {"url": "https://example.invalid/python.zip", "kind": "upstream"}
                        ],
                        "sha256": package_runtime.hashlib.sha256(archives["https://example.invalid/python.zip"]).hexdigest(),
                        "archive_format": "zip",
                        "entrypoints": {"python": ["python/python.exe"]},
                    },
                    {
                        "id": "nodejs-windows-x64",
                        "kind": "nodejs-runtime",
                        "version": "24.14.0",
                        "platform": "windows-x64",
                        "sources": [
                            {"url": "https://example.invalid/node.zip", "kind": "upstream"}
                        ],
                        "sha256": package_runtime.hashlib.sha256(archives["https://example.invalid/node.zip"]).hexdigest(),
                        "archive_format": "zip",
                        "entrypoints": {
                            "node": ["node-v24.14.0-win-x64/node.exe"],
                            "npm": ["node-v24.14.0-win-x64/npm.cmd"],
                        },
                    },
                ],
            }
            (deps_dir / "manifest.json").write_text(json.dumps(manifest), encoding="utf-8")

            def fake_urlopen(request, timeout=60):  # noqa: ANN001
                url = request if isinstance(request, str) else request.full_url
                return _FakeResponse(archives[url])

            with mock.patch.object(package_runtime.urllib.request, "urlopen", side_effect=fake_urlopen):
                package_runtime.ensure_runtime_bootstrap(root, "windows-x64-full")

            self.assertTrue((root / ".deps" / "store" / "chromium-windows-x64" / "147.0.7727.24" / "chrome-win64" / "chrome.exe").exists())
            self.assertTrue((root / ".deps" / "store" / "python-windows-x64" / "3.12.13" / "python" / "python.exe").exists())
            self.assertTrue((root / ".deps" / "store" / "nodejs-windows-x64" / "24.14.0" / "node-v24.14.0-win-x64" / "npm.cmd").exists())

    def test_download_runtime_archive_falls_back_to_next_source(self) -> None:
        with TemporaryDirectory() as tmp:
            root = Path(tmp)
            archive = self._runtime_archive({"node/node.exe": b"node", "node/npm.cmd": b"npm"})
            resource = {
                "id": "nodejs-windows-x64",
                "kind": "nodejs-runtime",
                "version": "24.14.0",
                "platform": "windows-x64",
                "sources": [
                    {"url": "https://primary.example.invalid/node.zip", "kind": "upstream"},
                    {"url": "https://mirror.example.invalid/node.zip", "kind": "mirror"},
                ],
                "sha256": package_runtime.hashlib.sha256(archive).hexdigest(),
                "archive_format": "zip",
                "entrypoints": {
                    "node": ["node/node.exe"],
                    "npm": ["node/npm.cmd"],
                },
            }

            def fake_urlopen(request, timeout=60):  # noqa: ANN001
                url = request if isinstance(request, str) else request.full_url
                if "primary" in url:
                    raise OSError("offline")
                return _FakeResponse(archive)

            with mock.patch.object(package_runtime.urllib.request, "urlopen", side_effect=fake_urlopen):
                archive_path = package_runtime.download_runtime_archive(root, resource)

            self.assertTrue(archive_path.exists())

    def test_download_runtime_archive_uses_fastest_probed_source(self) -> None:
        with TemporaryDirectory() as tmp:
            root = Path(tmp)
            archive = self._runtime_archive({"node/node.exe": b"node", "node/npm.cmd": b"npm"})
            resource = {
                "id": "nodejs-windows-x64",
                "kind": "nodejs-runtime",
                "version": "24.14.0",
                "platform": "windows-x64",
                "sources": [
                    {"url": "https://nodejs.org/node.zip", "kind": "upstream"},
                    {"url": "https://mirror.example.invalid/node.zip", "kind": "mirror"},
                ],
                "sha256": package_runtime.hashlib.sha256(archive).hexdigest(),
                "archive_format": "zip",
                "entrypoints": {
                    "node": ["node/node.exe"],
                    "npm": ["node/npm.cmd"],
                },
            }
            requested: list[str] = []

            def fake_probe(source, index):  # noqa: ANN001
                parsed = urlparse(source["url"])
                return {
                    "source": source,
                    "index": index,
                    "ok": True,
                    "bytes_per_second": 10 if parsed.hostname == "nodejs.org" else 100,
                }

            def fake_urlopen(request, timeout=60):  # noqa: ANN001
                url = request if isinstance(request, str) else request.full_url
                requested.append(url)
                return _FakeResponse(archive)

            with mock.patch.object(package_runtime, "probe_runtime_download_source", side_effect=fake_probe):
                with mock.patch.object(package_runtime.urllib.request, "urlopen", side_effect=fake_urlopen):
                    archive_path = package_runtime.download_runtime_archive(root, resource)

            self.assertTrue(archive_path.exists())
            self.assertEqual(["https://mirror.example.invalid/node.zip"], requested)

    def test_download_runtime_archive_uses_manifest_order_when_probes_fail(self) -> None:
        with TemporaryDirectory() as tmp:
            root = Path(tmp)
            archive = self._runtime_archive({"node/node.exe": b"node", "node/npm.cmd": b"npm"})
            resource = {
                "id": "nodejs-windows-x64",
                "kind": "nodejs-runtime",
                "version": "24.14.0",
                "platform": "windows-x64",
                "sources": [
                    {"url": "https://primary.example.invalid/node.zip", "kind": "upstream"},
                    {"url": "https://mirror.example.invalid/node.zip", "kind": "mirror"},
                ],
                "sha256": package_runtime.hashlib.sha256(archive).hexdigest(),
                "archive_format": "zip",
                "entrypoints": {
                    "node": ["node/node.exe"],
                    "npm": ["node/npm.cmd"],
                },
            }
            requested: list[str] = []

            def fake_probe(source, index):  # noqa: ANN001
                return {"source": source, "index": index, "ok": False, "bytes_per_second": 0.0}

            def fake_urlopen(request, timeout=60):  # noqa: ANN001
                url = request if isinstance(request, str) else request.full_url
                requested.append(url)
                return _FakeResponse(archive)

            with mock.patch.object(package_runtime, "probe_runtime_download_source", side_effect=fake_probe):
                with mock.patch.object(package_runtime.urllib.request, "urlopen", side_effect=fake_urlopen):
                    archive_path = package_runtime.download_runtime_archive(root, resource)

            self.assertTrue(archive_path.exists())
            self.assertEqual(["https://primary.example.invalid/node.zip"], requested)

    def test_extract_runtime_archive_cleans_stale_temp_roots(self) -> None:
        with TemporaryDirectory() as tmp:
            root = Path(tmp)
            resource = {
                "id": "chromium-windows-x64",
                "kind": "chromium",
                "version": "147.0.7727.24",
                "platform": "windows-x64",
                "sources": [
                    {"url": "https://example.invalid/chromium.zip", "kind": "upstream"}
                ],
                "sha256": "unused",
                "archive_format": "zip",
                "entrypoints": {"browser": ["chrome-win64/chrome.exe"]},
            }
            store_parent = root / ".deps" / "store" / "chromium-windows-x64"
            hidden_stale_root = store_parent / ".chromium-windows-x64-147.0.7727.24-stale"
            plain_stale_root = store_parent / "chromium-windows-x64-147.0.7727.24-stale"
            (hidden_stale_root / "chrome-win64").mkdir(parents=True, exist_ok=True)
            (plain_stale_root / "chrome-win64").mkdir(parents=True, exist_ok=True)
            archive_path = root / "chromium.zip"
            archive_path.write_bytes(self._runtime_archive({
                "chrome-win64/chrome.exe": b"chrome",
            }))

            package_runtime.extract_runtime_archive(root, resource, archive_path)

            self.assertFalse(hidden_stale_root.exists())
            self.assertFalse(plain_stale_root.exists())
            self.assertTrue(
                (
                    root
                    / ".deps"
                    / "store"
                    / "chromium-windows-x64"
                    / "147.0.7727.24"
                    / "chrome-win64"
                    / "chrome.exe"
                ).exists()
            )

    @staticmethod
    def _runtime_archive(entries: dict[str, bytes]) -> bytes:
        buffer = io.BytesIO()
        with zipfile.ZipFile(buffer, "w", compression=zipfile.ZIP_DEFLATED) as zf:
            for name, payload in entries.items():
                zf.writestr(name, payload)
        return buffer.getvalue()


if __name__ == "__main__":
    unittest.main()
