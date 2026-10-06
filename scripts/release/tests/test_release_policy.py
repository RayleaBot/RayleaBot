import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "scripts/release"))
from release_policy import publication_settings, release_channel
import release_tool


class ReleasePolicyTests(unittest.TestCase):
    def test_channel_and_latest_follow_semver_prerelease_not_build_metadata(self):
        for tag in ("v0.7.0", "v0.7.0+build-1"):
            self.assertEqual(publication_settings(tag)["channel"], "stable")
            self.assertEqual(publication_settings(tag)["prerelease"], "false")
            self.assertEqual(publication_settings(tag)["make_latest"], "legacy")
        for tag in ("v0.7.0-beta.1", "v0.7.0-rc.2+build.3"):
            self.assertEqual(publication_settings(tag)["channel"], "beta")
            self.assertEqual(publication_settings(tag)["prerelease"], "true")
            self.assertEqual(publication_settings(tag)["make_latest"], "false")
        for tag in ("0.7.0", "v../outside", "v0.7", "v0.7.0\nchannel=stable"):
            with self.assertRaises(ValueError):
                publication_settings(tag)

    def test_explicit_channel_cannot_disagree_with_version(self):
        for version, channel in (("0.7.0-beta.1", "stable"), ("0.7.0", "beta"), ("0.7.0", "unknown")):
            with self.assertRaises(ValueError):
                release_channel(version, channel)

    def test_cli_emits_outputs_only_after_exact_release_notes_pass(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            output = root / "output"
            args = [sys.executable, str(ROOT / "scripts/release/release_policy.py"), "--tag", "v0.7.0-beta.1", "--notes-dir", str(root), "--github-output", str(output)]
            failed = subprocess.run(args, capture_output=True)
            self.assertNotEqual(failed.returncode, 0)
            self.assertFalse(output.exists())
            (root / "v0.7.0-beta.1.md").write_text("预发布版本与升级说明。", encoding="utf-8")
            passed = subprocess.run(args, capture_output=True)
            self.assertEqual(passed.returncode, 0, passed.stderr)
            settings = dict(line.split("=", 1) for line in output.read_text(encoding="utf-8").splitlines())
            self.assertEqual(settings, publication_settings("v0.7.0-beta.1"))

    def test_release_manifest_preserves_beta_channel_and_experimental_macos(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            archive = root / "RayleaBot-v0.7.0-beta.1-macos-arm64-full.tar.gz"
            archive.write_bytes(b"fixture")
            matrix = release_tool.ARTIFACT_MATRIX["macos-arm64-full"]
            sidecar = release_tool.ArtifactSidecar("macos-arm64-full", archive, archive.name, "macos-arm64", matrix["support_level"], matrix["smoke_profile"], 7, 1, "guided")
            path = release_tool.build_release_metadata(
                version="0.7.0-beta.1", channel="beta", git_commit="a" * 40,
                built_at="2026-10-06T00:00:00Z", config_schema_version="4", db_schema_version="000006",
                plugin_protocol_version="4", release_notes_ref="https://example.invalid/releases/v0.7.0-beta.1",
                download_base_url="https://example.invalid/releases/download/v0.7.0-beta.1", sidecars=[sidecar], output_dir=root / "out")
            manifest = json.loads(path.read_text(encoding="utf-8"))
            self.assertEqual(manifest["channel"], "beta")
            self.assertEqual(manifest["artifacts"][0]["support_level"], "experimental")


if __name__ == "__main__":
    unittest.main()
