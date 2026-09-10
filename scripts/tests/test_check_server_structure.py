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


class PluginBoundaryTests(unittest.TestCase):
    def test_nested_packages_cannot_bypass_boundaries(self) -> None:
        cases = [
            ("management", "plugins/runtime", True),
            ("management/plugins", "plugins/runtime/session", True),
            ("plugins/runtime", "management/events", True),
            ("plugins/runtime/session", "management", True),
            ("management/plugins", "plugins/catalog", False),
            ("plugins/runtime/session", "plugins", False),
            ("management", "plugins/runtimeview", False),
            ("plugins/runtime", "managementview", False),
        ]
        for package, dependency, invalid in cases:
            with self.subTest(package=package, dependency=dependency):
                with tempfile.TemporaryDirectory() as directory:
                    root = Path(directory)
                    source = root / "server/internal" / package / "boundary.go"
                    source.parent.mkdir(parents=True)
                    source.write_text(
                        f'package {source.parent.name}\nimport "{structure.INTERNAL_PREFIX}{dependency}"\n',
                        encoding="utf-8",
                    )
                    files = structure.collect_go_files(root, root / "server/internal")
                    errors: list[str] = []
                    structure.check_plugin_boundaries(files, errors)
                    self.assertEqual(len(errors), int(invalid), errors)


class AdapterOwnershipTests(unittest.TestCase):
    def test_adapter_boundaries_check_owner_and_nested_dependencies(self) -> None:
        cases = [
            ("bot/adapters", "management/events", True),
            ("bot/adapters/nested", "configruntime", True),
            ("bot/adapters", "config", False),
            ("bot/adapters", "systemview", False),
        ]
        for package, dependency, invalid in cases:
            with self.subTest(package=package, dependency=dependency), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                source = root / "server/internal" / package / "boundary.go"
                source.parent.mkdir(parents=True)
                source.write_text(f'package {source.parent.name}\nimport "{structure.INTERNAL_PREFIX}{dependency}"\n', encoding="utf-8")
                files = structure.collect_go_files(root, root / "server/internal")
                errors: list[str] = []
                structure.check_adapter_boundaries(files, errors)
                self.assertEqual(len(errors), int(invalid), errors)



if __name__ == "__main__":
    unittest.main()
