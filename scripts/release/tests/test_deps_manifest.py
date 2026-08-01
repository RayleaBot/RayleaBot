import json
import re
import unittest
from pathlib import Path
from urllib.parse import urlparse


ROOT = Path(__file__).resolve().parents[3]
MANIFEST_PATH = ROOT / ".deps" / "manifest.json"
SHA256_PATTERN = re.compile(r"^[0-9a-f]{64}$")


def has_source_url(urls: list[str], host: str, path_prefix: str) -> bool:
    for url in urls:
        parsed = urlparse(url)
        if parsed.scheme == "https" and parsed.hostname == host and parsed.path.startswith(path_prefix):
            return True
    return False


class DepsManifestMetadataTests(unittest.TestCase):
    def test_manifest_shape_tracks_bootstrap_ready_contract(self) -> None:
        manifest = json.loads(MANIFEST_PATH.read_text(encoding="utf-8"))
        self.assertEqual(4, manifest.get("manifest_version"))
        resources = manifest.get("resources", [])
        self.assertEqual(3, len(resources))
        self.assertEqual({"windows-x64", "linux-x64", "macos-arm64"}, {item.get("platform") for item in resources})
        for resource in resources:
            self.assertEqual("chromium", resource.get("kind"), resource)
            self.assertIn(resource.get("archive_format"), {"zip", "tar.gz", "tar.xz"}, resource)
            sources = resource.get("sources")
            self.assertIsInstance(sources, list, resource)
            self.assertTrue(sources, resource)
            seen_urls: set[str] = set()
            for source in sources:
                self.assertIsInstance(source, dict, resource)
                url = source.get("url")
                kind = source.get("kind")
                self.assertIsInstance(url, str, resource)
                self.assertTrue(url.startswith("https://"), resource)
                self.assertNotIn(url, seen_urls, resource)
                seen_urls.add(url)
                self.assertIn(kind, {"upstream", "mirror"}, resource)
            entrypoints = resource.get("entrypoints")
            self.assertIsInstance(entrypoints, dict, resource)
            self.assertTrue(entrypoints, resource)
            for candidates in entrypoints.values():
                self.assertIsInstance(candidates, list, resource)
                self.assertTrue(candidates, resource)
                for candidate in candidates:
                    self.assertIsInstance(candidate, str, resource)
                    self.assertTrue(candidate, resource)
                    self.assertFalse(candidate.startswith("/"), resource)

    def test_chromium_resources_have_concrete_sources_and_sha256(self) -> None:
        manifest = json.loads(MANIFEST_PATH.read_text(encoding="utf-8"))
        resources = manifest.get("resources", [])
        for resource in resources:
            sources = resource.get("sources")
            sha256 = resource.get("sha256")
            self.assertIsInstance(sources, list, resource)
            self.assertGreaterEqual(len(sources), 1, resource)
            for source in sources:
                self.assertIsInstance(source.get("url"), str, resource)
                self.assertTrue(source["url"].startswith("https://"), resource)
                self.assertNotIn("TODO(", source["url"], resource)
                self.assertIn(source.get("kind"), {"upstream", "mirror"}, resource)
            self.assertIsInstance(sha256, str, resource)
            self.assertRegex(sha256, SHA256_PATTERN, resource)
            self.assertIn(resource.get("archive_format"), {"zip", "tar.gz", "tar.xz"}, resource)
            self.assertEqual(["browser"], list(resource.get("entrypoints", {})), resource)

    def test_chromium_resources_include_upstream_and_trusted_mirror(self) -> None:
        manifest = json.loads(MANIFEST_PATH.read_text(encoding="utf-8"))
        for resource in manifest.get("resources", []):
            urls = [source.get("url", "") for source in resource.get("sources", []) if isinstance(source, dict)]
            self.assertTrue(has_source_url(urls, "storage.googleapis.com", "/chrome-for-testing-public/"), resource)
            self.assertTrue(has_source_url(urls, "npmmirror.com", "/mirrors/chrome-for-testing/"), resource)


if __name__ == "__main__":
    unittest.main()
