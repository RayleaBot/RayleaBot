import json
import re
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]
MANIFEST_PATH = ROOT / ".deps" / "manifest.json"
SHA256_PATTERN = re.compile(r"^[0-9a-f]{64}$")


class DepsManifestMetadataTests(unittest.TestCase):
    def test_runtime_resources_have_concrete_source_and_sha256(self) -> None:
        manifest = json.loads(MANIFEST_PATH.read_text(encoding="utf-8"))
        resources = manifest.get("resources", [])

        runtime_resources = [
            item
            for item in resources
            if isinstance(item, dict) and item.get("kind") in {"python-runtime", "nodejs-runtime"}
        ]

        self.assertEqual(6, len(runtime_resources))
        for resource in runtime_resources:
            source = resource.get("source")
            sha256 = resource.get("sha256")
            self.assertIsInstance(source, str, resource)
            self.assertTrue(source.startswith("https://"), resource)
            self.assertNotIn("TODO(", source, resource)
            self.assertIsInstance(sha256, str, resource)
            self.assertRegex(sha256, SHA256_PATTERN, resource)


if __name__ == "__main__":
    unittest.main()
