import json
import sys
import unittest
import zipfile
from pathlib import Path
from tempfile import TemporaryDirectory
from unittest import mock

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "scripts" / "release"))

import recovery_drill
from artifact_matrix import REQUIRED_PATHS


class RecoveryDrillTests(unittest.TestCase):
    def write_archive(self, root, *, omit=None):
        archive = root / "current.zip"
        with zipfile.ZipFile(archive, "w") as bundle:
            for path in REQUIRED_PATHS["windows-x64-full"]:
                if path != omit:
                    bundle.writestr("distribution/" + path, "fixture")
        return archive

    def test_current_archive_reuses_fresh_recovery_and_keeps_evidence(self):
        with TemporaryDirectory() as tmp:
            root = Path(tmp)
            archive = self.write_archive(root)
            fixture = root / "plugin.zip"
            fixture.write_bytes(b"plugin fixture")
            output = root / "evidence"
            with mock.patch("recovery_drill.rehearse", return_value={"restored_login": True}) as run:
                result = recovery_drill.run_recovery_drill("windows-x64-full", archive, fixture,
                                                          output_dir=output, observation_window_seconds=3)
            binary, destination = run.call_args.args
            self.assertTrue(binary.is_file())
            self.assertEqual(destination, output / "rehearsal")
            self.assertEqual(run.call_args.kwargs["plugin_fixture"], fixture.resolve())
            self.assertEqual(run.call_args.kwargs["observation_window_seconds"], 3)
            self.assertEqual(json.loads((output / "result.json").read_text()), result)
            self.assertTrue(result["recovery"]["restored_login"])

    def test_missing_packaged_binary_prevents_rehearsal(self):
        with TemporaryDirectory() as tmp:
            root = Path(tmp)
            archive = self.write_archive(root, omit="raylea-server.exe")
            fixture = root / "plugin.zip"
            fixture.write_bytes(b"fixture")
            with mock.patch("recovery_drill.rehearse") as run, self.assertRaises(RuntimeError):
                recovery_drill.run_recovery_drill("windows-x64-full", archive, fixture, output_dir=root / "evidence")
            run.assert_not_called()

    def test_existing_evidence_directory_is_not_overwritten(self):
        with TemporaryDirectory() as tmp:
            root = Path(tmp)
            archive = self.write_archive(root)
            fixture = root / "plugin.zip"
            fixture.write_bytes(b"fixture")
            with self.assertRaises(FileExistsError):
                recovery_drill.run_recovery_drill("windows-x64-full", archive, fixture, output_dir=root)


if __name__ == "__main__":
    unittest.main()
