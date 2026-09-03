import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]
SCRIPT = ROOT / "scripts/release/check_release_notes.py"


class ReleaseNotesTests(unittest.TestCase):
    def check(self, directory: Path, tag: str = "v1.2.3") -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [sys.executable, "-X", "utf8", str(SCRIPT), "--tag", tag, "--notes-dir", str(directory)],
            capture_output=True,
            text=True,
            encoding="utf-8",
            cwd=directory,
        )

    def test_accepts_complete_chinese_notes_for_stable_and_prerelease_tags(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            for tag in ("v1.2.3", "v1.2.3-beta.1+build.7"):
                with self.subTest(tag=tag):
                    path = directory / f"{tag}.md"
                    body = "修复更新检查失败时无法查看原因的问题。\n\n## 升级说明\n\n请参阅升级指南。\n"
                    path.write_text(body, encoding="utf-8-sig")
                    original = path.read_bytes()
                    result = self.check(directory, tag)
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertEqual(path.read_bytes(), original)

    def test_requires_the_exact_tag_file(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            (directory / "v1.2.2.md").write_text("上一版说明。", encoding="utf-8")
            self.assertNotEqual(self.check(directory).returncode, 0)

    def test_rejects_empty_or_editor_only_notes(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            for body in ("", " \n\t", "<!-- 编辑提示 -->", "# v1.2.3\n\n## 升级说明\n---\n<!-- 编辑提示 -->"):
                with self.subTest(body=body):
                    (directory / "v1.2.3.md").write_text(body, encoding="utf-8")
                    self.assertNotEqual(self.check(directory).returncode, 0)

    def test_rejects_placeholders_in_text_links_and_comments(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            for remaining in (
                "修复{{触发条件}}下的问题。",
                "[下载](https://example.invalid/{{TAG}}/app.zip)",
                "<!-- {{CONTRIBUTORS}} -->",
                "{{\n尚未填写\n}}",
            ):
                with self.subTest(remaining=remaining):
                    (directory / "v1.2.3.md").write_text("本次修复更新诊断。\n" + remaining, encoding="utf-8")
                    self.assertNotEqual(self.check(directory).returncode, 0)

    def test_rejects_invalid_utf8(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            (directory / "v1.2.3.md").write_bytes(b"\xff\xfe\x00")
            self.assertNotEqual(self.check(directory).returncode, 0)

    def test_rejects_nonrelease_tags_and_path_traversal(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            for tag in ("1.2.3", "v1.2", "v01.2.3", "v../outside", "v1.2.3/../../outside", "v1.2.3\\outside"):
                with self.subTest(tag=tag):
                    self.assertNotEqual(self.check(directory, tag).returncode, 0)


if __name__ == "__main__":
    unittest.main()
