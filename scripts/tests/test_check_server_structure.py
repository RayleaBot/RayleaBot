from __future__ import annotations

import importlib.util
import json
import sys
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).resolve().parents[1] / "check-server-structure.py"
SPEC = importlib.util.spec_from_file_location("check_server_structure", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
structure = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = structure
SPEC.loader.exec_module(structure)


class ManualSQLReviewTests(unittest.TestCase):
    def test_review_deadline_warns_but_invalid_metadata_fails(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            registry = root / "docs/engineering/manual-sql-exceptions.json"
            registry.parent.mkdir(parents=True)
            entry = {
                "category": "A", "reason": "dynamic table selection",
                "owner": "storage", "target_action": "review query builder",
                "revisit_after": "2000-01-01",
            }
            for deadline, valid in [("2000-01-01", True), ("invalid-date", False)]:
                with self.subTest(deadline=deadline):
                    entry["revisit_after"] = deadline
                    registry.write_text(json.dumps({"allowed_files": {"server/internal/store.go": entry}}), encoding="utf-8")
                    errors: list[str] = []
                    warnings: list[str] = []
                    allowed = structure.load_manual_sql_exceptions(root, errors, warnings)
                    self.assertEqual(bool(errors), not valid)
                    if valid:
                        self.assertIn("server/internal/store.go", allowed)
                        self.assertEqual(len(warnings), 1)


if __name__ == "__main__":
    unittest.main()
