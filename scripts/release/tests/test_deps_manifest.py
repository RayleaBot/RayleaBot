import json
import re
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]
MANIFEST_PATH = ROOT / ".deps" / "manifest.json"
SHA256_PATTERN = re.compile(r"^[0-9a-f]{64}$")


class DepsManifestMetadataTests(unittest.TestCase):
    def test_manifest_shape_tracks_bootstrap_ready_contract(self) -> None:
        manifest = json.loads(MANIFEST_PATH.read_text(encoding="utf-8"))
        self.assertEqual(3, manifest.get("manifest_version"))
        resources = manifest.get("resources", [])
        self.assertEqual(9, len(resources))
        for resource in resources:
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

    def test_runtime_resources_have_concrete_sources_and_sha256(self) -> None:
        manifest = json.loads(MANIFEST_PATH.read_text(encoding="utf-8"))
        resources = manifest.get("resources", [])

        runtime_resources = [
            item
            for item in resources
            if isinstance(item, dict) and item.get("kind") in {"python-runtime", "nodejs-runtime"}
        ]

        self.assertEqual(6, len(runtime_resources))
        for resource in runtime_resources:
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
            entrypoints = resource.get("entrypoints", {})
            if resource.get("kind") == "python-runtime":
                self.assertIn("python", entrypoints, resource)
            if resource.get("kind") == "nodejs-runtime":
                self.assertIn("node", entrypoints, resource)
                self.assertIn("npm", entrypoints, resource)

    def test_runtime_resources_include_trusted_mirrors(self) -> None:
        manifest = json.loads(MANIFEST_PATH.read_text(encoding="utf-8"))
        for resource in manifest.get("resources", []):
            if not isinstance(resource, dict):
                continue
            urls = [source.get("url", "") for source in resource.get("sources", []) if isinstance(source, dict)]
            if resource.get("kind") == "nodejs-runtime":
                self.assertTrue(any("nodejs.org/" in url for url in urls), resource)
                self.assertTrue(any("npmmirror.com/mirrors/node/" in url for url in urls), resource)
                self.assertTrue(any("mirrors.ustc.edu.cn/node/" in url for url in urls), resource)
                self.assertTrue(any("mirrors.nju.edu.cn/nodejs-release/" in url for url in urls), resource)
            if resource.get("kind") == "python-runtime":
                self.assertTrue(any("github.com/astral-sh/python-build-standalone/" in url for url in urls), resource)
                self.assertTrue(any("mirrors.nju.edu.cn/github-release/astral-sh/python-build-standalone/" in url for url in urls), resource)


if __name__ == "__main__":
    unittest.main()
