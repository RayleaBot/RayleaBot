from __future__ import annotations

import importlib.util
from pathlib import Path
import sys
import json
import tempfile
import unittest
from unittest import mock


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
    def test_go_reads_effective_server_toolchain(self) -> None:
        module = load_module()

        calls: list[tuple[list[str], Path | None]] = []

        def fake_run(args: list[str], cwd: Path | None = None):
            calls.append((args, cwd))
            return module.CommandOutput(0, "go1.26.6\n", "")

        original_exists = module.executable_exists
        original_run = module.run_command
        try:
            module.executable_exists = lambda name: name == "go"
            module.run_command = fake_run
            result = module.check_go()
        finally:
            module.executable_exists = original_exists
            module.run_command = original_run

        self.assertEqual(result.status, "ok")
        self.assertEqual(calls, [(["go", "env", "GOVERSION"], module.REPO_ROOT / "server")])

    def test_pnpm_uses_corepack_when_global_shim_is_old(self) -> None:
        module = load_module()

        def fake_exists(name: str) -> bool:
            return name in {"pnpm", "corepack"}

        def fake_run(args: list[str]):
            if args == ["pnpm", "--version"]:
                return module.CommandOutput(0, "11.21.0\n", "")
            if args == ["corepack", "pnpm", "--version"]:
                return module.CommandOutput(0, "11.22.0\n", "")
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
        self.assertIn("corepack prepare pnpm@11.22.0 --activate", result.remediation)

    def test_python_checks_running_interpreter(self) -> None:
        module = load_module()

        original_version = module.platform.python_version
        try:
            module.platform.python_version = lambda: "3.14.7"
            result = module.check_python()
        finally:
            module.platform.python_version = original_version

        self.assertEqual(result.status, "ok")
        self.assertEqual(result.detail, "3.14.7")


    def test_selected_server_task_does_not_require_frontend_tools(self) -> None:
        module = load_module()
        good = module.CheckResult("Go", "ok", module.REQUIRED_GO_VERSION)
        with mock.patch.object(module, "check_go", return_value=good) as go, mock.patch.object(module, "check_node", side_effect=AssertionError("frontend tool was required")):
            results = module.run_checks(include_runtime=False, task="server")
        go.assert_called_once()
        self.assertFalse(any(result.failed for result in results))

    def test_tool_versions_reject_unpinned_or_duplicate_tools(self) -> None:
        module = load_module()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            valid = (REPO_ROOT / ".tool-versions").read_text(encoding="utf-8")
            for content in (valid + "\ngolang 1.26.6\n", valid.replace("nodejs 26.7.0", "nodejs latest")):
                (root / ".tool-versions").write_text(content, encoding="utf-8")
                with self.assertRaises(ValueError):
                    module.read_tool_versions(root)

    def test_ecosystem_version_drift_is_an_error(self) -> None:
        module = load_module()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for name in ("server", "launcher", "sdk/go"):
                target = root / name / "go.mod"
                target.parent.mkdir(parents=True)
                target.write_text(f"module fixture\n\ngo {module.TOOL_VERSIONS['golang']}\n", encoding="utf-8")
            (root / "server/go.mod").write_text("module fixture\n\ngo 1.0.0\n", encoding="utf-8")
            result = module.check_version_files(("go",), root)
            self.assertTrue(result.failed)
            self.assertIn("server", result.detail)
            for name in ("web", "launcher", "sdk/vue"):
                target = root / name / "package.json"
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_text(json.dumps({"engines": {"node": module.TOOL_VERSIONS["nodejs"]}, "packageManager": "pnpm@1.0.0"}), encoding="utf-8")
            self.assertTrue(module.check_version_files(("node", "pnpm"), root).failed)

    def test_doctor_finds_macos_app_bundle_without_path_entry(self) -> None:
        module = load_module()
        with mock.patch.object(module.platform, "system", return_value="Darwin"), mock.patch.object(module.shutil, "which", return_value=None), mock.patch.object(module.Path, "is_file", return_value=True), mock.patch.object(module.os, "access", return_value=True):
            result = module.check_chromium()
        self.assertEqual(result.status, "ok")
        self.assertIn("Google Chrome.app", result.detail)


if __name__ == "__main__":
    unittest.main()
