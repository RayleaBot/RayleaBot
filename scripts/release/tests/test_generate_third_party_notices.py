import json
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "scripts" / "release"))

import generate_third_party_notices as notices


class ThirdPartyNoticeTests(unittest.TestCase):
    def test_pnpm_license_inputs_cover_supported_libc_variants(self) -> None:
        for project in ("web", "launcher"):
            contents = (ROOT / project / "pnpm-workspace.yaml").read_text(encoding="utf-8")
            architecture_block = contents.split("supportedArchitectures:", 1)[1].split("\nallowBuilds:", 1)[0]

            with self.subTest(project=project):
                self.assertIn("    - glibc", architecture_block)
                self.assertIn("    - musl", architecture_block)

    def test_node_package_uses_declared_license_when_file_is_absent(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            package_dir = Path(tmp) / "package"
            package_dir.mkdir()
            (package_dir / "package.json").write_text(
                json.dumps({"name": "example", "version": "1.0.0", "license": "MIT"}),
                encoding="utf-8",
            )
            payload = {"MIT": [{"name": "example", "paths": [str(package_dir)]}]}

            components = notices.components_from_pnpm_payload(payload, Path(tmp), "npm:test")

            self.assertEqual(components[0].license_expression, "MIT")
            self.assertIn("declares this license expression", components[0].notice)

    def test_node_package_rejects_unknown_license(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            package_dir = Path(tmp) / "package"
            package_dir.mkdir()
            (package_dir / "package.json").write_text(
                json.dumps({"name": "example", "version": "1.0.0", "license": "UNKNOWN"}),
                encoding="utf-8",
            )
            (package_dir / "LICENSE").write_text("license", encoding="utf-8")

            with self.assertRaisesRegex(notices.NoticeGenerationError, "unknown or missing"):
                notices.components_from_pnpm_payload(
                    {"UNKNOWN": [{"name": "example", "paths": [str(package_dir)]}]},
                    Path(tmp),
                    "npm:test",
                )

    def test_unreviewed_license_fails_closed(self) -> None:
        with self.assertRaisesRegex(notices.NoticeGenerationError, "unreviewed license expression"):
            notices.normalize_license_expression("LicenseRef-Custom", "example@1.0.0")

    def test_license_text_removes_trailing_whitespace(self) -> None:
        self.assertEqual(
            notices.normalize_license_text("first  \r\n\r\n\tsecond\t\r\n"),
            "first\n\n    second",
        )

    def test_render_is_deterministic_and_groups_identical_notices(self) -> None:
        components = [
            notices.Component("npm:web", "bravo", "2.0.0", "MIT", "[LICENSE]\ntext"),
            notices.Component("npm:web", "alpha", "1.0.0", "MIT", "[LICENSE]\ntext"),
        ]

        rendered = notices.render_notices(list(reversed(components)))

        self.assertLess(rendered.index("alpha | 1.0.0"), rendered.index("bravo | 2.0.0"))
        self.assertEqual(rendered.count("    [LICENSE]"), 1)
        self.assertIn("Applies to: alpha@1.0.0, bravo@2.0.0", rendered)
        self.assertNotIn("    \n", rendered)


if __name__ == "__main__":
    unittest.main()
