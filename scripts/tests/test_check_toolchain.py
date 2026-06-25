from __future__ import annotations

import importlib.util
from pathlib import Path
import sys
import unittest


REPO_ROOT = Path(__file__).resolve().parents[2]
SCRIPT_PATH = REPO_ROOT / "scripts" / "check-toolchain.py"


def load_module():
    spec = importlib.util.spec_from_file_location("check_toolchain", SCRIPT_PATH)
    if spec is None or spec.loader is None:
        raise RuntimeError("unable to load check-toolchain.py")
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


class CheckToolchainTests(unittest.TestCase):
    def test_version_from_go_output(self) -> None:
        module = load_module()
        self.assertEqual(
            module.version_from_go_output("go version go1.25.8 windows/amd64"),
            "go1.25.8",
        )

    def test_pnpm_uses_corepack_when_global_shim_is_old(self) -> None:
        module = load_module()

        def fake_exists(name: str) -> bool:
            return name in {"pnpm", "corepack"}

        def fake_run(args: list[str]):
            if args == ["pnpm", "--version"]:
                return module.CommandOutput(0, "10.31.0\n", "")
            if args == ["corepack", "pnpm", "--version"]:
                return module.CommandOutput(0, "10.32.1\n", "")
            return module.CommandOutput(127, "", "unexpected command")

        original_exists = module.executable_exists
        original_run = module.run_command
        try:
            module.executable_exists = fake_exists
            module.run_command = fake_run
            result = module.check_pnpm()
        finally:
            module.executable_exists = original_exists
            module.run_command = original_run

        self.assertEqual(result.status, "warning")
        self.assertIn("corepack pnpm --version", result.detail)
        self.assertIn("corepack prepare pnpm@10.32.1 --activate", result.remediation)


if __name__ == "__main__":
    unittest.main()
