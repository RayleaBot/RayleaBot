import json
import calendar
from datetime import date
import re
import unittest
from pathlib import Path
from urllib.parse import urlparse


ROOT = Path(__file__).resolve().parents[3]
MANIFEST_PATH = ROOT / ".deps" / "manifest.json"


def has_source_url(urls: list[str], host: str, path_prefix: str) -> bool:
    for url in urls:
        parsed = urlparse(url)
        if parsed.scheme == "https" and parsed.hostname == host and parsed.path.startswith(path_prefix):
            return True
    return False


class DepsManifestMetadataTests(unittest.TestCase):
    def test_ffmpeg_sources_pin_a_release_instead_of_latest(self) -> None:
        manifest = json.loads(MANIFEST_PATH.read_text(encoding="utf-8"))
        resources = [item for item in manifest.get("resources", []) if item.get("kind") == "ffmpeg"]
        for resource in resources:
            urls = [source.get("url", "") for source in resource.get("sources", [])]
            self.assertTrue(all("/latest/" not in url for url in urls), resource)

    def test_btbn_ffmpeg_pins_month_end_build_and_build_specific_cache_version(self) -> None:
        resources = json.loads(MANIFEST_PATH.read_text(encoding="utf-8"))["resources"]
        tags = set()
        for resource in resources:
            if resource["kind"] != "ffmpeg" or resource["platform"] == "macos-arm64":
                continue
            source = resource["sources"][0]["url"]
            match = re.search(r"/autobuild-(\d{4})-(\d{2})-(\d{2})-(\d{2})-(\d{2})/([^/]+)$", urlparse(source).path)
            self.assertIsNotNone(match, resource)
            year, month, day = map(int, match.groups()[:3])
            # Prefer a successful calendar month-end build and independently
            # verify its upstream retention and digest when updating the pin.
            self.assertEqual(day, calendar.monthrange(year, month)[1], resource)
            filename = match.group(6)
            suffix = "-" + date(year, month, day).strftime("%Y%m%d")
            # The shared variant is a different archive of the same build, so its cache version names the variant.
            if "-gpl-shared-" in filename:
                suffix += "-shared"
            self.assertTrue(resource["version"].endswith(suffix), resource)
            archive_version = resource["version"].removesuffix(suffix)
            self.assertRegex(archive_version, r"^n9[.]0[.]\d+-\d+-g[0-9a-f]+$", resource)
            self.assertTrue(filename.startswith("ffmpeg-" + archive_version + "-"), resource)
            archive_root = filename.removesuffix(".zip").removesuffix(".tar.xz")
            for candidates in resource["entrypoints"].values():
                self.assertTrue(all(path.startswith(archive_root + "/bin/") for path in candidates), resource)
            tags.add(match.groups()[:5])
        self.assertEqual(len(tags), 1)

    def test_chromium_resources_include_upstream_and_trusted_mirror(self) -> None:
        manifest = json.loads(MANIFEST_PATH.read_text(encoding="utf-8"))
        for resource in manifest.get("resources", []):
            if resource.get("kind") != "chromium":
                continue
            urls = [source.get("url", "") for source in resource.get("sources", []) if isinstance(source, dict)]
            self.assertTrue(has_source_url(urls, "storage.googleapis.com", "/chrome-for-testing-public/"), resource)
            self.assertTrue(has_source_url(urls, "npmmirror.com", "/mirrors/chrome-for-testing/"), resource)


if __name__ == "__main__":
    unittest.main()
