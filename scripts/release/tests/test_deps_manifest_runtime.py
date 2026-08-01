import io
import json
import sys
import unittest
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory
from unittest import mock
from urllib.parse import urlparse

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
    def test_windows_release_root_is_compacted_before_runtime_extraction(self) -> None:
        with TemporaryDirectory() as tmp:
            destination = Path(tmp) / "compatible"
            root = destination / "RayleaBot-v0.1.0-local.20260802-windows-x64-full"
            root.mkdir(parents=True)

            compact = package_runtime.compact_release_root(root, destination, "nt")

            self.assertEqual(compact.parent, destination.parent)
            self.assertTrue(compact.name.startswith("r-"))
            self.assertTrue(compact.is_dir())
            self.assertFalse(root.exists())

    def test_resource_metadata_requires_browser_entrypoint(self) -> None:
        resource = self._resource(self._runtime_archive({"chrome-win64/chrome.exe": b"chrome"}))

        self.assertTrue(package_runtime.resource_has_complete_metadata(resource))
        resource["entrypoints"] = {}
        self.assertFalse(package_runtime.resource_has_complete_metadata(resource))

    def test_ensure_runtime_bootstrap_prepares_chromium(self) -> None:
        with TemporaryDirectory() as tmp:
            root = Path(tmp)
            archive = self._runtime_archive({"chrome-win64/chrome.exe": b"chrome"})
            resource = self._resource(archive)
            manifest = {"manifest_version": 4, "resources": [resource]}
            (root / ".deps").mkdir()
            (root / ".deps" / "manifest.json").write_text(json.dumps(manifest), encoding="utf-8")

            with mock.patch.object(
                package_runtime.urllib.request,
                "urlopen",
                return_value=_FakeResponse(archive),
            ):
                package_runtime.ensure_runtime_bootstrap(root, "windows-x64-full")

            self.assertTrue(
                (root / ".deps" / "store" / "chromium-windows-x64" / "147.0.7727.24" / "chrome-win64" / "chrome.exe").exists()
            )
            self.assertEqual(["chromium"], list(package_runtime.REQUIRED_ENTRYPOINTS))

    def test_download_runtime_archive_falls_back_to_next_source(self) -> None:
        with TemporaryDirectory() as tmp:
            archive = self._runtime_archive({"chrome-win64/chrome.exe": b"chrome"})
            resource = self._resource(archive)
            resource["sources"] = [
                {"url": "https://primary.example.invalid/chromium.zip", "kind": "upstream"},
                {"url": "https://mirror.example.invalid/chromium.zip", "kind": "mirror"},
            ]

            def fake_urlopen(request, timeout=60):  # noqa: ANN001
                url = request if isinstance(request, str) else request.full_url
                if "primary" in url:
                    raise OSError("offline")
                return _FakeResponse(archive)

            with mock.patch.object(package_runtime, "select_runtime_download_sources", side_effect=lambda sources: sources):
                with mock.patch.object(package_runtime.urllib.request, "urlopen", side_effect=fake_urlopen):
                    path = package_runtime.download_runtime_archive(Path(tmp), resource)

            self.assertTrue(path.exists())

    def test_download_runtime_archive_uses_fastest_probed_source(self) -> None:
        with TemporaryDirectory() as tmp:
            archive = self._runtime_archive({"chrome-win64/chrome.exe": b"chrome"})
            resource = self._resource(archive)
            resource["sources"] = [
                {"url": "https://storage.googleapis.com/chromium.zip", "kind": "upstream"},
                {"url": "https://mirror.example.invalid/chromium.zip", "kind": "mirror"},
            ]
            requested: list[str] = []

            def fake_probe(source, index):  # noqa: ANN001
                speed = 10 if urlparse(source["url"]).hostname == "storage.googleapis.com" else 100
                return {"source": source, "index": index, "ok": True, "bytes_per_second": speed}

            def fake_urlopen(request, timeout=60):  # noqa: ANN001
                url = request if isinstance(request, str) else request.full_url
                requested.append(url)
                return _FakeResponse(archive)

            with mock.patch.object(package_runtime, "probe_runtime_download_source", side_effect=fake_probe):
                with mock.patch.object(package_runtime.urllib.request, "urlopen", side_effect=fake_urlopen):
                    package_runtime.download_runtime_archive(Path(tmp), resource)

            self.assertEqual(["https://mirror.example.invalid/chromium.zip"], requested)

    def test_download_runtime_archive_uses_manifest_order_when_probes_fail(self) -> None:
        sources = [
            {"url": "https://primary.example.invalid/chromium.zip", "kind": "upstream"},
            {"url": "https://mirror.example.invalid/chromium.zip", "kind": "mirror"},
        ]

        def fake_probe(source, index):  # noqa: ANN001
            return {"source": source, "index": index, "ok": False, "bytes_per_second": 0.0}

        with mock.patch.object(package_runtime, "probe_runtime_download_source", side_effect=fake_probe):
            self.assertEqual(sources, package_runtime.select_runtime_download_sources(sources))

    def test_extract_runtime_archive_cleans_stale_temp_roots(self) -> None:
        with TemporaryDirectory() as tmp:
            root = Path(tmp)
            archive = self._runtime_archive({"chrome-win64/chrome.exe": b"chrome"})
            resource = self._resource(archive)
            store_parent = root / ".deps" / "store" / "chromium-windows-x64"
            stale = [
                store_parent / ".chromium-windows-x64-147.0.7727.24-stale",
                store_parent / "chromium-windows-x64-147.0.7727.24-stale",
            ]
            for path in stale:
                (path / "chrome-win64").mkdir(parents=True)
            archive_path = root / "chromium.zip"
            archive_path.write_bytes(archive)

            package_runtime.extract_runtime_archive(root, resource, archive_path)

            self.assertTrue(all(not path.exists() for path in stale))

    def test_replace_directory_retries_transient_windows_permission_errors(self) -> None:
        source = mock.Mock()
        source.replace.side_effect = [PermissionError("locked"), None]

        with mock.patch.object(package_runtime.time, "sleep"):
            package_runtime.replace_directory_with_retry(source, Path("target"), timeout_seconds=1)

        self.assertEqual(2, source.replace.call_count)

    @staticmethod
    def _resource(archive: bytes) -> dict[str, object]:
        return {
            "id": "chromium-windows-x64",
            "kind": "chromium",
            "version": "147.0.7727.24",
            "platform": "windows-x64",
            "sources": [{"url": "https://example.invalid/chromium.zip", "kind": "upstream"}],
            "sha256": package_runtime.hashlib.sha256(archive).hexdigest(),
            "archive_format": "zip",
            "entrypoints": {"browser": ["chrome-win64/chrome.exe"]},
        }

    @staticmethod
    def _runtime_archive(entries: dict[str, bytes]) -> bytes:
        buffer = io.BytesIO()
        with zipfile.ZipFile(buffer, "w", compression=zipfile.ZIP_DEFLATED) as zf:
            for name, payload in entries.items():
                zf.writestr(name, payload)
        return buffer.getvalue()


if __name__ == "__main__":
    unittest.main()
