import importlib
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(__file__)))

from rayleabot import protocol as protocol_module


class ProtocolRedactionTests(unittest.TestCase):
    def setUp(self):
        self.protocol = importlib.reload(protocol_module)

    def test_redact_sensitive_text_masks_error_credentials(self):
        message = "request failed: Cookie=SESSDATA=abc123; access_token: token-value password=secret"

        redacted = self.protocol.redact_sensitive_text(message)

        self.assertNotIn("abc123", redacted)
        self.assertNotIn("token-value", redacted)
        self.assertNotIn("secret", redacted)
        self.assertIn("[REDACTED]", redacted)


if __name__ == "__main__":
    unittest.main()
