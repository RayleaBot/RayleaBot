import io
import lzma
import sys
import tarfile
import unittest
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "scripts"))
from archive_io import extract_archive


class ArchiveBoundaryTests(unittest.TestCase):
    def test_zip_traversal_duplicate_and_symlink_are_rejected(self):
        for names in (["../outside"], ["file", "./file"], ["C:/outside"], ["NUL.txt"]):
            with self.subTest(names=names), TemporaryDirectory() as tmp:
                root = Path(tmp)
                archive = root / "archive.zip"
                with zipfile.ZipFile(archive, "w") as writer:
                    for name in names:
                        writer.writestr(name, b"payload")
                with self.assertRaises(ValueError):
                    extract_archive(archive, root / "target")
                self.assertFalse((root / "outside").exists())

    def test_tar_links_never_escape_and_existing_files_survive(self):
        with TemporaryDirectory() as tmp:
            root = Path(tmp)
            archive = root / "archive.tar.gz"
            with tarfile.open(archive, "w:gz") as writer:
                link = tarfile.TarInfo("link")
                link.type = tarfile.SYMTYPE
                link.linkname = "../outside"
                writer.addfile(link)
            with self.assertRaises(ValueError):
                extract_archive(archive, root / "target", allow_links=True)
            archive = root / "archive.zip"
            with zipfile.ZipFile(archive, "w") as writer:
                writer.writestr("file", b"new")
            target = root / "target"
            target.mkdir(exist_ok=True)
            (target / "file").write_bytes(b"old")
            with self.assertRaises(FileExistsError):
                extract_archive(archive, target)
            self.assertEqual((target / "file").read_bytes(), b"old")

    def test_xz_dictionary_limit_matches_server_policy(self):
        fixture = ROOT / "server/internal/platform/deps/testdata/archives/dictionary-limit.tar.xz"
        self.assertTrue(fixture.is_file(), fixture)
        with TemporaryDirectory() as tmp, self.assertRaises(lzma.LZMAError):
            extract_archive(fixture, Path(tmp), allow_links=True)

    def test_gzip_crc_and_xz_checksums_are_verified(self):
        for mode in ("w:gz", "w:xz"):
            with self.subTest(mode=mode), TemporaryDirectory() as tmp:
                root = Path(tmp)
                archive = root / "archive"
                with tarfile.open(archive, mode) as writer:
                    entry = tarfile.TarInfo("file")
                    entry.size = 7
                    writer.addfile(entry, io.BytesIO(b"payload"))
                extract_archive(archive, root / "valid")
                payload = bytearray(archive.read_bytes())
                payload[-6] ^= 0xff
                archive.write_bytes(payload)
                with self.assertRaises(Exception):
                    extract_archive(archive, root / "invalid")


if __name__ == "__main__":
    unittest.main()
