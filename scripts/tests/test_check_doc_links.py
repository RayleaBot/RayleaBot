import importlib.util
import sys
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("check_doc_links", ROOT / "scripts" / "check-doc-links.py")
assert SPEC is not None and SPEC.loader is not None
links = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = links
SPEC.loader.exec_module(links)


class DocumentationLinkTests(unittest.TestCase):
    def test_relative_anchor_and_percent_encoded_image_are_valid(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            docs = root / "docs"
            images = docs / "images"
            images.mkdir(parents=True)
            (docs / "target.md").write_text("# Target heading\n", encoding="utf-8")
            (images / "sample image.png").write_bytes(b"image")
            source = docs / "source.md"
            source.write_text(
                "[target](target.md#target-heading)\n![sample](images/sample%20image.png)\n",
                encoding="utf-8",
            )

            self.assertEqual(links.check_files(root, [source]), [])

    def test_missing_path_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            source = root / "source.md"
            source.write_text("[missing](missing.md)\n", encoding="utf-8")

            self.assertIn("missing local target", links.check_files(root, [source])[0])

    def test_missing_anchor_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            target = root / "target.md"
            target.write_text("# Existing\n", encoding="utf-8")
            source = root / "source.md"
            source.write_text("[missing](target.md#absent)\n", encoding="utf-8")

            self.assertIn("missing anchor #absent", links.check_files(root, [source])[0])

    def test_images_and_reference_links_are_checked(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            source = root / "source.md"
            source.write_text("![missing][asset]\n\n[asset]: missing.png\n", encoding="utf-8")

            self.assertIn("missing local target", links.check_files(root, [source])[0])

    def test_fenced_indented_and_inline_code_are_ignored(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            source = root / "source.md"
            source.write_text(
                "```md\n[missing](inside-fence.md)\n```\n"
                "    [missing](indented.md)\n"
                "`[missing](inline.md)`\n",
                encoding="utf-8",
            )

            self.assertEqual(links.check_files(root, [source]), [])

    def test_malformed_external_uri_is_rejected_without_network(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            source = root / "source.md"
            source.write_text("[valid](https://example.com/a) [invalid](https:///missing-host)\n", encoding="utf-8")

            errors = links.check_files(root, [source])
            self.assertEqual(len(errors), 1)
            self.assertIn("HTTP(S) URI has no host", errors[0])


if __name__ == "__main__":
    unittest.main()
