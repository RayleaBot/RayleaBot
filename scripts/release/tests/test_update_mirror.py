import copy
import json
from pathlib import Path
import sys
import unittest
from unittest.mock import patch
import subprocess

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "scripts/release"))
from sync_update_mirror import channel_documents, rewrite_manifest, version_key, main


class UpdateMirrorTests(unittest.TestCase):
    def fixture(self):
        return json.loads((ROOT / "fixtures/release-manifest/ok.release-manifest-minimal.json").read_text(encoding="utf-8"))["input"]

    def test_rewrites_every_archive_to_independent_host(self):
        original = self.fixture()
        snapshot = copy.deepcopy(original)
        result = rewrite_manifest(original, "https://downloads.example.com/raylea/")
        self.assertEqual(original, snapshot)
        for asset in result["artifacts"]:
            self.assertEqual(asset["download_url"], f"https://downloads.example.com/raylea/v{result['version']}/{asset['file_name']}")

    def test_channels_use_semver_and_never_promote_beta_to_stable(self):
        manifests = [{"version": version, "channel": channel, "release_notes_ref": "https://example.com/release"} for version, channel in [("1.9.0", "stable"), ("1.10.0-beta.2", "beta"), ("1.10.0-beta.10", "beta")]]
        docs = channel_documents(manifests)
        self.assertEqual(docs["stable.json"]["version"], "1.9.0")
        self.assertEqual(docs["beta.json"]["version"], "1.10.0-beta.10")
        manifests.append({"version": "1.10.0", "channel": "stable", "release_notes_ref": "https://example.com/release"})
        self.assertEqual(channel_documents(manifests)["beta.json"]["version"], "1.10.0")
        self.assertEqual(version_key("1.0.0+build-1"), version_key("1.0.0"))

    def test_rejects_invalid_public_urls_before_publication(self):
        for base in ["http://example.com", "https://user:password@example.com", "https://example.com/?token=fixture"]:
            argv = ["sync", "--repository", "RayleaBot/RayleaBot", "--base-url", base, "--bucket", "release-fixture", "--endpoint", "https://storage.example.com"]
            with patch.object(sys, "argv", argv), patch("sync_update_mirror.run") as invoke:
                with self.assertRaises(ValueError):
                    main()
                invoke.assert_not_called()
        with self.assertRaises(ValueError):
            channel_documents([])

    def test_upload_failure_keeps_channel_pointers_and_retry_reuses_assets(self):
        manifest = self.fixture()
        manifest["version"] = "0.4.0"
        manifest["channel"] = "stable"
        for artifact in manifest["artifacts"]:
            artifact["archive_size_bytes"] = 1
        releases = [{"tag_name": "v0.4.0", "draft": False, "prerelease": False, "assets": [{"name": "release_manifest.v2.json"}]}]
        argv = ["sync", "--repository", "RayleaBot/RayleaBot", "--base-url", "https://downloads.example.com", "--bucket", "release-fixture", "--endpoint", "https://storage.example.com"]
        for fail in [True, False]:
            uploads = []
            archive_downloads = []
            def invoke(*args):
                if args[:2] == ("gh", "api"):
                    return json.dumps(releases)
                if args[:3] == ("gh", "release", "download"):
                    name = args[args.index("--pattern") + 1]
                    target = Path(args[args.index("--dir") + 1]) / name
                    if name == "release_manifest.v2.json":
                        target.write_text(json.dumps(manifest), encoding="utf-8")
                    else:
                        archive_downloads.append(name)
                        target.write_bytes(b"x")
                    return ""
                if args[:3] == ("aws", "s3api", "list-objects-v2"):
                    return json.dumps({"Contents": [] if fail else [{"Key": f"v0.4.0/{asset['file_name']}", "Size": 1} for asset in manifest["artifacts"]]})
                if args[:3] == ("aws", "s3", "cp"):
                    uploads.append(args[4])
                    if fail:
                        raise subprocess.CalledProcessError(1, args)
                    return ""
                raise AssertionError(args)
            with patch.object(sys, "argv", argv), patch("sync_update_mirror.run", side_effect=invoke):
                if fail:
                    with self.assertRaises(subprocess.CalledProcessError):
                        main()
                    self.assertFalse(any(key.endswith("/stable.json") or key.endswith("/beta.json") or key.endswith("/releases.json") for key in uploads))
                else:
                    main()
                    self.assertFalse(archive_downloads)
                    pointers = {"s3://release-fixture/releases.json", "s3://release-fixture/beta.json", "s3://release-fixture/stable.json"}
                    self.assertTrue(pointers.issubset(uploads))
                    first_pointer = min(uploads.index(key) for key in pointers)
                    self.assertTrue(all(index < first_pointer for index, key in enumerate(uploads) if "/v0.4.0/" in key))


if __name__ == "__main__":
    unittest.main()
