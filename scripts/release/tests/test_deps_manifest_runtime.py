import io
import sys
import unittest
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory
from unittest import mock

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "scripts" / "release"))

import package_runtime


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

    def test_compacting_release_root_retries_and_preserves_extracted_content(self) -> None:
        with TemporaryDirectory() as tmp:
            destination = Path(tmp) / "validation"
            root = destination / "RayleaBot-v0.4.0-windows-x64-full"
            root.mkdir(parents=True)
            (root / "build_info.json").write_bytes(b"verified extracted content")
            original = Path.replace
            attempts = 0

            def replace(source, target):
                nonlocal attempts
                attempts += 1
                if attempts == 1:
                    raise PermissionError("temporary scanner handle")
                return original(source, target)

            with mock.patch.object(Path, "replace", autospec=True, side_effect=replace), mock.patch.object(package_runtime.time, "sleep"):
                compact = package_runtime.compact_release_root(root, destination, "nt")
            self.assertEqual(attempts, 2)
            self.assertEqual((compact / "build_info.json").read_bytes(), b"verified extracted content")
            self.assertFalse(root.exists())

    def test_compacting_release_root_preserves_source_after_retry_budget(self) -> None:
        with TemporaryDirectory() as tmp:
            destination = Path(tmp) / "validation"
            root = destination / "RayleaBot-v0.4.0-windows-x64-full"
            root.mkdir(parents=True)
            (root / "build_info.json").write_bytes(b"retained failure evidence")
            with mock.patch.object(Path, "replace", side_effect=PermissionError("still locked")), mock.patch.object(package_runtime.time, "monotonic", side_effect=[0.0, 6.0]):
                with self.assertRaises(PermissionError):
                    package_runtime.compact_release_root(root, destination, "nt")
            self.assertEqual((root / "build_info.json").read_bytes(), b"retained failure evidence")
            self.assertEqual(list(Path(tmp).glob("r-*")), [])

    def test_resource_metadata_requires_browser_entrypoint(self) -> None:
        resource = self._resource(self._runtime_archive({"chrome-win64/chrome.exe": b"chrome"}))

        self.assertTrue(package_runtime.resource_has_complete_metadata(resource))
        resource["entrypoints"] = {}
        self.assertFalse(package_runtime.resource_has_complete_metadata(resource))

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
            "version": "152.0.7977.42",
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
