import sys
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "plugins" / "runtime" / "python"))
sys.path.insert(0, str(ROOT / "sdk" / "python"))

import rayleabot
import rayleabot_runtime
from rayleabot.protocol import ActionError as CompatibilityActionError
from rayleabot_runtime.protocol import ActionError as RuntimeActionError


class CompatibilityFacadeTests(unittest.TestCase):
    def test_root_exports_delegate_to_runtime_client(self) -> None:
        self.assertIs(rayleabot.RayleaBotPlugin, rayleabot_runtime.RayleaBotPlugin)
        self.assertIs(CompatibilityActionError, RuntimeActionError)
        self.assertIs(rayleabot.protocol.ActionError, RuntimeActionError)


if __name__ == "__main__":
    unittest.main()
