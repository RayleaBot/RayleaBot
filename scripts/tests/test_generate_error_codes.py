import importlib.util
from pathlib import Path
import sys
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / "generate-error-codes.py"
sys.path.insert(0, str(SCRIPT.parent))
SPEC = importlib.util.spec_from_file_location("generate_error_codes", SCRIPT)
generator = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(generator)


class ErrorCodeGeneratorTests(unittest.TestCase):
    def test_inconsistent_http_scope_is_rejected_in_both_directions(self):
        for status, surfaces in [(403, ["logs"]), (None, ["http"])]:
            with self.subTest(status=status, surfaces=surfaces):
                with self.assertRaisesRegex(ValueError, "HTTP applicability and status"):
                    generator.generate({"test.failure": {
                        "code": "test.failure", "message_key": "errors.test.failure",
                        "http_status": status, "applies_to": surfaces,
                    }}, {})
