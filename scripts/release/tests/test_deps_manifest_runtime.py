import io
import json
import sys
import unittest
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory
from unittest import mock

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "scripts" / "release"))

import package_runtime


class _FakeResponse(io.BytesIO):
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
            "source": "https://example.invalid/python.zip",
            "sha256": "10b9fd9ba9441f246f2cb279c2c6e6b2f98e60ef7960c313fd2bbc7f0c1e6f5e",
            "archive_format": "zip",
            "entrypoints": {
                "python": ["python/install/python.exe"],
                "pip": ["python/install/Scripts/pip.exe"],
            },
        }

        self.assertTrue(package_runtime.resource_has_complete_metadata(resource))
        resource["entrypoints"] = {"python": ["python/install/python.exe"]}
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
                    "python/install/python.exe": b"python",
                    "python/install/Scripts/pip.exe": b"pip",
                }),
                "https://example.invalid/node.zip": self._runtime_archive({
                    "node-v24.14.0-win-x64/node.exe": b"node",
                    "node-v24.14.0-win-x64/npm.cmd": b"npm",
                }),
            }
            manifest = {
                "manifest_version": 2,
                "resources": [
                    {
                        "id": "chromium-windows-x64",
                        "kind": "chromium",
                        "version": "147.0.7727.24",
                        "platform": "windows-x64",
                        "source": "https://example.invalid/chromium.zip",
                        "sha256": package_runtime.hashlib.sha256(archives["https://example.invalid/chromium.zip"]).hexdigest(),
                        "archive_format": "zip",
                        "entrypoints": {"browser": ["chrome-win64/chrome.exe"]},
                    },
                    {
                        "id": "python-windows-x64",
                        "kind": "python-runtime",
                        "version": "3.12.13",
                        "platform": "windows-x64",
                        "source": "https://example.invalid/python.zip",
                        "sha256": package_runtime.hashlib.sha256(archives["https://example.invalid/python.zip"]).hexdigest(),
                        "archive_format": "zip",
                        "entrypoints": {
                            "python": ["python/install/python.exe"],
                            "pip": ["python/install/Scripts/pip.exe"],
                        },
                    },
                    {
                        "id": "nodejs-windows-x64",
                        "kind": "nodejs-runtime",
                        "version": "24.14.0",
                        "platform": "windows-x64",
                        "source": "https://example.invalid/node.zip",
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
            self.assertTrue((root / ".deps" / "store" / "python-windows-x64" / "3.12.13" / "python" / "install" / "python.exe").exists())
            self.assertTrue((root / ".deps" / "store" / "nodejs-windows-x64" / "24.14.0" / "node-v24.14.0-win-x64" / "npm.cmd").exists())

    @staticmethod
    def _runtime_archive(entries: dict[str, bytes]) -> bytes:
        buffer = io.BytesIO()
        with zipfile.ZipFile(buffer, "w", compression=zipfile.ZIP_DEFLATED) as zf:
            for name, payload in entries.items():
                zf.writestr(name, payload)
        return buffer.getvalue()


if __name__ == "__main__":
    unittest.main()
