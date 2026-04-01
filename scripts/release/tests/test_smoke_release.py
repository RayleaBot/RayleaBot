import sys
import tarfile
import tempfile
import unittest
import zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "scripts" / "release"))

import smoke_release


class SmokeReleaseTests(unittest.TestCase):
    def test_windows_expected_entries_include_contracts_and_web_dist(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            temp = Path(tmp)
            archive_path = temp / "bundle.zip"
            root = Path("RayleaBot-v0.1.0-windows-x64-full")
            with zipfile.ZipFile(archive_path, "w", compression=zipfile.ZIP_DEFLATED) as zf:
                for name in sorted(smoke_release.EXPECTED["windows-x64-full"]["entries"]):
                    zf.writestr(str(root / name), "ok")

            entries = smoke_release.list_entries("windows-x64-full", archive_path)

            self.assertIn("contracts/config.user.schema.json", entries)
            self.assertIn("contracts/plugin-info.schema.json", entries)
            self.assertIn("web/dist/index.html", entries)

    def test_linux_server_expected_entries_include_contracts_and_web_dist(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            temp = Path(tmp)
            archive_path = temp / "bundle.tar.gz"
            root = Path("RayleaBot-v0.1.0-linux-x64-server")
            with tarfile.open(archive_path, "w:gz") as tf:
                for name in sorted(smoke_release.EXPECTED["linux-x64-server"]["entries"]):
                    file_path = temp / name.replace("/", "_")
                    file_path.parent.mkdir(parents=True, exist_ok=True)
                    file_path.write_text("ok", encoding="utf-8")
                    tf.add(file_path, arcname=str(root / name))

            entries = smoke_release.list_entries("linux-x64-server", archive_path)

            self.assertIn("contracts/config.user.schema.json", entries)
            self.assertIn("web/dist/index.html", entries)


if __name__ == "__main__":
    unittest.main()
