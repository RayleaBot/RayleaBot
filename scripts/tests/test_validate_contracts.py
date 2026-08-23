import importlib.util
import unittest
from pathlib import Path

_SCRIPT = (
    Path(__file__).resolve().parents[1]
    / "ci"
    / "validate_contracts.py"
)
_spec = importlib.util.spec_from_file_location("validate_contracts_under_test", _SCRIPT)
_validate_contracts = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_validate_contracts)


class CliFixtureSemanticsSelfTest(unittest.TestCase):
    def test_cli_fixture_semantics_self_test(self) -> None:
        self.assertEqual(_validate_contracts.run_cli_semantics_self_test(), [])


if __name__ == "__main__":
    unittest.main()
