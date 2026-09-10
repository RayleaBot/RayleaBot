import json
import sys
import unittest
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "scripts" / "release"))

from artifact_matrix import ARTIFACT_MATRIX
from package_runtime import unpack_archive


class ArtifactMatrixTests(unittest.TestCase):
    def test_matrix_covers_exactly_the_formal_artifacts(self):
        schema = json.loads((ROOT / "contracts/release-manifest.schema.json").read_text(encoding="utf-8"))
        self.assertEqual(set(ARTIFACT_MATRIX), set(schema["$defs"]["artifactId"]["enum"]))
        for name, item in ARTIFACT_MATRIX.items():
            self.assertIn(item["server_binary"], item["required_paths"], name)
            self.assertIn("LICENSE", item["required_paths"], name)
            self.assertIn("THIRD_PARTY_NOTICES.md", item["required_paths"], name)
        self.assertIn("raylea-updater.exe", ARTIFACT_MATRIX["windows-x64-full"]["required_paths"])
        self.assertIn("systemd/rayleabot.service", ARTIFACT_MATRIX["linux-x64-server"]["required_paths"])

    def test_invalid_archive_roots_are_rejected_before_writing(self):
        for names in [[], ["../escape"], ["root/../escape"], ["root/a", "other/b"], ["C:/escape"], ["root/a", "root/a"]]:
            with self.subTest(names=names), TemporaryDirectory() as tmp:
                root = Path(tmp)
                archive = root / "bundle.zip"
                with zipfile.ZipFile(archive, "w") as bundle:
                    for name in names:
                        bundle.writestr(name, b"fixture")
                with self.assertRaises(RuntimeError):
                    unpack_archive("windows-x64-full", archive, root / "unpacked")
                self.assertEqual(list((root / "unpacked").iterdir()), [])
                self.assertFalse((root / "escape").exists())


if __name__ == "__main__":
    unittest.main()
