from __future__ import annotations

import importlib.util
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).resolve().parents[1] / "ci/detect_changes.py"
SPEC = importlib.util.spec_from_file_location("detect_changes", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
changes = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(changes)


class ChangeClassificationTests(unittest.TestCase):
    def test_build_inputs_trigger_their_consumers(self) -> None:
        cases = [
            (["go.work.sum"], {"server", "sdk", "release", "ci"}),
            (["docs/engineering/manual-sql-exceptions.json"], {"server", "ci", "docs"}),
            (["docs/notes.md", "docs/engineering/manual-sql-exceptions.json"], {"server", "ci", "docs"}),
            ([".\\docs\\engineering\\manual-sql-exceptions.json"], {"server", "ci", "docs"}),
            (["design-qa.md"], {"docs", "docs_only"}),
        ]
        for paths, expected in cases:
            with self.subTest(paths=paths):
                result = changes.classify(paths)
                self.assertEqual({key for key, enabled in result.items() if enabled}, expected)

    def test_unknown_path_fails_even_when_mixed_with_known_documents(self) -> None:
        with self.assertRaisesRegex(ValueError, "unclassified.future-file"):
            changes.classify(["docs/notes.md", "unclassified.future-file"])

    def test_rename_checks_source_and_destination_and_deletion_keeps_scope(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)

            def git(*args: str) -> str:
                return subprocess.run(
                    ["git", "-c", "user.name=CI fixture", "-c", "user.email=ci@example.invalid",
                     "-c", "commit.gpgsign=false", *args],
                    cwd=root, check=True, capture_output=True, text=True, encoding="utf-8",
                ).stdout.strip()

            def classify_diff(base: str, head: str) -> set[str]:
                result = subprocess.run(
                    [sys.executable, str(SCRIPT), "--base", base, "--head", head],
                    cwd=root, check=True, capture_output=True, text=True, encoding="utf-8",
                )
                return {line.split("=", 1)[0] for line in result.stdout.splitlines() if line.endswith("=true")}

            git("init", "--quiet")
            source = root / "server/example.go"
            source.parent.mkdir()
            source.write_text("package example\n", encoding="utf-8")
            registry = root / "docs/engineering/manual-sql-exceptions.json"
            registry.parent.mkdir(parents=True)
            registry.write_text("{}\n", encoding="utf-8")
            git("add", ".")
            git("commit", "--quiet", "-m", "base")
            base = git("rev-parse", "HEAD")

            git("mv", "server/example.go", "docs/example.md")
            git("commit", "--quiet", "-m", "move between areas")
            renamed = git("rev-parse", "HEAD")
            self.assertEqual(classify_diff(base, renamed), {"server", "docs"})

            git("rm", "docs/engineering/manual-sql-exceptions.json")
            git("commit", "--quiet", "-m", "remove checker input")
            self.assertEqual(classify_diff(renamed, git("rev-parse", "HEAD")), {"server", "ci", "docs"})


if __name__ == "__main__":
    unittest.main()
