import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "scripts" / "release"))

import package_artifact


class PackageArtifactTests(unittest.TestCase):
    def test_archive_path_uses_platform_archive_suffix(self) -> None:
        output = Path("dist/release")

        self.assertEqual(
            output / "RayleaBot-v0.1.0-windows-x64-full.zip",
            package_artifact.archive_path(output, "0.1.0", "windows-x64-full"),
        )
        self.assertEqual(
            output / "RayleaBot-v0.1.0-linux-x64-server.tar.gz",
            package_artifact.archive_path(output, "0.1.0", "linux-x64-server"),
        )

    def test_download_dir_alias_maps_to_recovery_download_dir(self) -> None:
        args = package_artifact.parse_args(
            [
                "--artifact-id",
                "linux-x64-server",
                "--version",
                "0.1.0",
                "--git-commit",
                "abcdef1",
                "--release-notes-ref",
                "https://example.invalid/releases/v0.1.0",
                "--server-bin",
                "dist/server/raylea-server",
                "--download-dir",
                "dist/release/recovery-bootstrap",
            ]
        )

        self.assertEqual("dist/release/recovery-bootstrap", args.recovery_download_dir)


if __name__ == "__main__":
    unittest.main()
