import json
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "scripts"))
from deps_manifest import validate_manifest


class SharedManagedManifestTests(unittest.TestCase):
    def test_shared_contract_fixtures(self):
        fixtures = sorted((ROOT / "fixtures/deps-manifest").glob("*.json"))
        self.assertGreater(len(fixtures), 0)
        for path in fixtures:
            with self.subTest(path=path.name):
                document = json.loads(path.read_text(encoding="utf-8"))
                if path.name.startswith("invalid."):
                    with self.assertRaises(Exception):
                        validate_manifest(document)
                else:
                    validate_manifest(document)


if __name__ == "__main__":
    unittest.main()
