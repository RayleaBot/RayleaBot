import tomllib
import unittest
from pathlib import Path


SDK_ROOT = Path(__file__).resolve().parents[1]


class SDKPackageTests(unittest.TestCase):
    def test_sdk_installs_the_matching_runtime_client(self) -> None:
        config = tomllib.loads((SDK_ROOT / "pyproject.toml").read_text(encoding="utf-8"))

        self.assertEqual(config["project"]["name"], "rayleabot-sdk")
        self.assertEqual(
            config["project"]["dependencies"],
            [f"rayleabot-plugin-runtime=={config['project']['version']}"],
        )

    def test_sdk_has_no_legacy_import_package(self) -> None:
        self.assertFalse((SDK_ROOT / "rayleabot").exists())


if __name__ == "__main__":
    unittest.main()
