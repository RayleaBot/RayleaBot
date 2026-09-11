import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "scripts"))
from tool_versions import read_tool_versions


class ToolVersionSourcesTests(unittest.TestCase):
    def test_bootstrap_and_workspace_read_the_same_changed_source(self):
        bash = str(Path(os.environ.get("ProgramFiles", "C:/Program Files")) / "Git/bin/bash.exe") if os.name == "nt" else shutil.which("bash")
        if not bash or not Path(bash).is_file():
            self.skipTest("Bash unavailable")
        source = (ROOT / ".tool-versions").read_text(encoding="utf-8").replace("golang 1.26.6", "golang 9.8.7")
        node_script = "import { readToolVersions } from './scripts/tool-versions.mjs'; import { pathToFileURL } from 'node:url'; console.log(JSON.stringify(readToolVersions(pathToFileURL(process.argv[1]))));"
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            for suffix in ("\n", "\r\n"):
                path = root / ".tool-versions"
                path.write_bytes(("# fixture\n\n" + source).replace("\n", suffix).encode())
                expected = read_tool_versions(root)
                shell = subprocess.run([bash, str(ROOT / "scripts/read-tool-versions.sh"), str(path)], cwd=ROOT, capture_output=True, text=True, check=True)
                self.assertEqual(dict(line.split("=", 1) for line in shell.stdout.splitlines()), expected)
                node = subprocess.run([shutil.which("node"), "--input-type=module", "-e", node_script, str(path)], cwd=ROOT, capture_output=True, text=True, check=True)
                self.assertEqual(json.loads(node.stdout), expected)
            for broken in (source + "\ngolang 1.2.3\n", source.replace("nodejs ", "missing "), source.replace("9.8.7", "latest")):
                path.write_text(broken, encoding="utf-8")
                with self.assertRaises(ValueError):
                    read_tool_versions(root)
                for args in ([bash, str(ROOT / "scripts/read-tool-versions.sh"), str(path)], [shutil.which("node"), "--input-type=module", "-e", node_script, str(path)]):
                    result = subprocess.run(args, cwd=ROOT, capture_output=True, text=True)
                    self.assertNotEqual(result.returncode, 0)

    def test_workflow_setup_consumes_source_outputs_after_checkout(self):
        import yaml
        for path in (ROOT / ".github/workflows").glob("*.yml"):
            workflow = yaml.safe_load(path.read_text(encoding="utf-8"))
            for name, job in workflow.get("jobs", {}).items():
                available = False
                checked_out = False
                for step in job.get("steps", []):
                    uses = step.get("uses", "")
                    if uses.startswith("actions/checkout@"):
                        checked_out = True
                    if uses == "./.github/actions/tool-versions":
                        self.assertTrue(checked_out, f"{path.name}/{name}: versions before checkout")
                        available = True
                    for action, field, tool in (("actions/setup-node@", "node-version", "nodejs"), ("actions/setup-python@", "python-version", "python"), ("pnpm/action-setup@", "version", "pnpm")):
                        if uses.startswith(action) and field in step.get("with", {}):
                            self.assertTrue(available, f"{path.name}/{name}: missing source reader")
                            self.assertEqual(step["with"][field], "${{ steps.toolchain.outputs." + tool + " }}")
