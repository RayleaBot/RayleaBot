from __future__ import annotations

import importlib.util
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


class ManagementSQLTests(unittest.TestCase):
    def test_handwritten_sql_is_rejected_only_in_management_layer(self) -> None:
        for package, invalid in [("management", True), ("management/events", True), ("storage", False)]:
            with self.subTest(package=package), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                source = root / "server/internal" / package / "query.go"
                source.parent.mkdir(parents=True)
                source.write_text(f"package {source.parent.name}\nfunc run() {{ db.QueryContext(ctx, q) }}\n", encoding="utf-8")
                errors: list[str] = []
                structure.check_management_sql(structure.collect_go_files(root, root / "server/internal"), errors)
                self.assertEqual(len(errors), int(invalid), errors)


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


class DomainOwnershipTests(unittest.TestCase):
    def test_model_and_adapter_boundaries_check_owner_and_nested_dependencies(self) -> None:
        cases = [
            ("plugins", "storage", True),
            ("plugins/catalog", "storage", False),
            ("plugins", "plugins/runtime/session", True),
            ("plugins", "bot/chatevent", False),
            ("platform/health", "operations/recovery", True),
            ("platform/runtimepaths", "plugins/catalog", True),
            ("platform/runtimepaths", "config", False),
            ("bot/menu", "plugins/actions", True),
            ("bot/menu", "render", False),
            ("render", "plugins/lifecycle", True),
            ("bot/adapters", "management/events", True),
            ("bot/adapters/nested", "config/runtime", True),
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
                structure.check_model_boundaries(files, errors)
                structure.check_adapter_boundaries(files, errors)
                self.assertEqual(len(errors), int(invalid), errors)


class PackageNameTests(unittest.TestCase):
    def test_external_test_package_does_not_hide_production_package_name(self) -> None:
        for production_name, warning_count in [("example", 0), ("incorrect", 1)]:
            with self.subTest(production_name=production_name), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                package = root / "server/internal/example"
                package.mkdir(parents=True)
                (package / "a_test.go").write_text("package example_test\n", encoding="utf-8")
                (package / "z.go").write_text(f"package {production_name}\n", encoding="utf-8")
                warnings: list[str] = []
                structure.check_package_names(structure.collect_go_files(root, root / "server/internal"), warnings)
                self.assertEqual(len(warnings), warning_count, warnings)


if __name__ == "__main__":
    unittest.main()
