#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import platform
import re
import shutil
import sqlite3
import subprocess
import sys
import tempfile
from dataclasses import dataclass

REPO_ROOT = Path(__file__).resolve().parents[1]

sys.path.insert(0, str(Path(__file__).resolve().parent))
from tool_versions import read_tool_versions


TOOL_VERSIONS = read_tool_versions(REPO_ROOT)
REQUIRED_GO_VERSION = "go" + TOOL_VERSIONS["golang"]
REQUIRED_NODE_VERSION = "v" + TOOL_VERSIONS["nodejs"]
REQUIRED_NPM_VERSION = TOOL_VERSIONS["npm"]
REQUIRED_COREPACK_VERSION = TOOL_VERSIONS["corepack"]
REQUIRED_PNPM_VERSION = TOOL_VERSIONS["pnpm"]
REQUIRED_PYTHON_VERSION = TOOL_VERSIONS["python"]
REQUIRED_SQLC_VERSION = "v" + TOOL_VERSIONS["sqlc"]

GO_INSTALL_URL = "https://go.dev/dl/"
NODE_INSTALL_URL = f"https://nodejs.org/dist/{REQUIRED_NODE_VERSION}/"
PYTHON_INSTALL_URL = f"https://www.python.org/downloads/release/python-{REQUIRED_PYTHON_VERSION.replace('.', '')}/"
COREPACK_INSTALL = f"npm install --global corepack@{REQUIRED_COREPACK_VERSION}"
SQLC_INSTALL = f"go install github.com/sqlc-dev/sqlc/cmd/sqlc@{REQUIRED_SQLC_VERSION}"


@dataclass(frozen=True)
class CommandOutput:
    returncode: int
    stdout: str
    stderr: str


@dataclass(frozen=True)
class CheckResult:
    name: str
    status: str
    detail: str
    remediation: str = ""

    @property
    def failed(self) -> bool:
        return self.status == "error"


def run_command(args: list[str], cwd: Path | None = None) -> CommandOutput:
    resolved = shutil.which(args[0])
    if resolved:
        args = [resolved, *args[1:]]
    try:
        result = subprocess.run(args, capture_output=True, check=False, text=True, cwd=cwd)
    except OSError as exc:
        return CommandOutput(127, "", str(exc))
    return CommandOutput(result.returncode, result.stdout.strip(), result.stderr.strip())


def executable_exists(name: str) -> bool:
    return shutil.which(name) is not None


def first_line(value: str) -> str:
    return value.strip().splitlines()[0].strip() if value.strip() else ""


def normalize_sqlc_version(output: str) -> str:
    value = first_line(output)
    return value if value.startswith("v") else f"v{value}" if value else ""


def command_failure_detail(result: CommandOutput) -> str:
    return first_line(result.stderr or result.stdout) or f"exit code {result.returncode}"


def check_go() -> CheckResult:
    if not executable_exists("go"):
        return CheckResult(
            "Go",
            "error",
            f"Go is not on PATH; required {REQUIRED_GO_VERSION}.",
            "\n".join(
                [
                    f"Install Go {TOOL_VERSIONS['golang']} before running server tests.",
                    f"Download: {GO_INSTALL_URL}",
                    f"Windows: winget install GoLang.Go --version {TOOL_VERSIONS['golang']}",
                    f"Linux x64 online: curl -LO https://go.dev/dl/{REQUIRED_GO_VERSION}.linux-amd64.tar.gz && sudo tar -C /usr/local -xzf {REQUIRED_GO_VERSION}.linux-amd64.tar.gz",
                    f"Offline: copy the matching {REQUIRED_GO_VERSION} archive into the runner image and put its bin directory on PATH; set GOTOOLCHAIN=local for a local-only failure.",
                ]
            ),
        )

    result = run_command(["go", "env", "GOVERSION"], cwd=REPO_ROOT / "server")
    if result.returncode != 0:
        return CheckResult(
            "Go",
            "error",
            f"Unable to read the server Go toolchain version: {command_failure_detail(result)}.",
            "Fix PATH so `go env GOVERSION` runs from server/, then rerun `python scripts/check-toolchain.py`.",
        )
    actual = first_line(result.stdout)
    if actual != REQUIRED_GO_VERSION:
        return CheckResult(
            "Go",
            "error",
            f"Found {actual or first_line(result.stdout)}; required {REQUIRED_GO_VERSION}.",
            "\n".join(
                [
                    "Install the exact Go patch version used by server/go.mod.",
                    f"Download: {GO_INSTALL_URL}",
                    f"Windows: winget install GoLang.Go --version {TOOL_VERSIONS['golang']}",
                    f"Linux x64 online: curl -LO https://go.dev/dl/{REQUIRED_GO_VERSION}.linux-amd64.tar.gz && sudo tar -C /usr/local -xzf {REQUIRED_GO_VERSION}.linux-amd64.tar.gz",
                    f"Offline: preinstall {REQUIRED_GO_VERSION} in the image or workstation and set GOTOOLCHAIN=local before running tests.",
                ]
            ),
        )
    return CheckResult("Go", "ok", actual)


def check_node() -> CheckResult:
    if not executable_exists("node"):
        return CheckResult(
            "Node.js",
            "error",
            f"Node.js is not on PATH; required {REQUIRED_NODE_VERSION}.",
            f"Install Node.js {TOOL_VERSIONS['nodejs']} from {NODE_INSTALL_URL}; offline images must preinstall it before running Web or Launcher tests.",
        )

    result = run_command(["node", "--version"])
    if result.returncode != 0:
        return CheckResult(
            "Node.js",
            "error",
            f"Unable to read Node.js version: {command_failure_detail(result)}.",
            "Fix PATH so `node --version` runs.",
        )
    actual = first_line(result.stdout)
    if actual != REQUIRED_NODE_VERSION:
        return CheckResult(
            "Node.js",
            "error",
            f"Found {actual}; required {REQUIRED_NODE_VERSION}.",
            f"Install Node.js {TOOL_VERSIONS['nodejs']} from {NODE_INSTALL_URL}, then install Corepack with `{COREPACK_INSTALL}`.",
        )
    return CheckResult("Node.js", "ok", actual)


def check_npm() -> CheckResult:
    if not executable_exists("npm"):
        return CheckResult(
            "npm",
            "error",
            f"npm is not on PATH; required {REQUIRED_NPM_VERSION}.",
            f"Install Node.js {TOOL_VERSIONS['nodejs']} from {NODE_INSTALL_URL}; its distribution includes npm {REQUIRED_NPM_VERSION}.",
        )

    result = run_command(["npm", "--version"])
    if result.returncode != 0:
        return CheckResult(
            "npm",
            "error",
            f"Unable to read npm version: {command_failure_detail(result)}.",
            "Fix PATH so `npm --version` runs.",
        )
    actual = first_line(result.stdout)
    if actual != REQUIRED_NPM_VERSION:
        return CheckResult(
            "npm",
            "error",
            f"Found {actual}; required {REQUIRED_NPM_VERSION}.",
            f"Reinstall Node.js {TOOL_VERSIONS['nodejs']} from {NODE_INSTALL_URL}, or run `npm install --global npm@{REQUIRED_NPM_VERSION}`.",
        )
    return CheckResult("npm", "ok", actual)


def check_corepack() -> CheckResult:
    if not executable_exists("corepack"):
        return CheckResult(
            "Corepack",
            "error",
            f"Corepack is not on PATH; required {REQUIRED_COREPACK_VERSION}.",
            f"Node.js 26 no longer bundles Corepack; install it with `{COREPACK_INSTALL}`.",
        )

    result = run_command(["corepack", "--version"])
    if result.returncode != 0:
        return CheckResult(
            "Corepack",
            "error",
            f"Unable to read Corepack version: {command_failure_detail(result)}.",
            f"Reinstall it with `{COREPACK_INSTALL}`.",
        )
    actual = first_line(result.stdout)
    if actual != REQUIRED_COREPACK_VERSION:
        return CheckResult(
            "Corepack",
            "error",
            f"Found {actual}; required {REQUIRED_COREPACK_VERSION}.",
            f"Install the pinned version with `{COREPACK_INSTALL}`.",
        )
    return CheckResult("Corepack", "ok", actual)


def check_pnpm() -> CheckResult:
    pnpm_actual = ""
    if executable_exists("pnpm"):
        result = run_command(["pnpm", "--version"])
        if result.returncode == 0:
            pnpm_actual = first_line(result.stdout)
            if pnpm_actual == REQUIRED_PNPM_VERSION:
                return CheckResult("pnpm", "ok", pnpm_actual)

    corepack_actual = ""
    if executable_exists("corepack"):
        result = run_command(["corepack", "pnpm", "--version"])
        if result.returncode == 0:
            corepack_actual = first_line(result.stdout)
            if corepack_actual == REQUIRED_PNPM_VERSION:
                found = pnpm_actual or "not found"
                return CheckResult(
                    "pnpm",
                    "warning",
                    f"`pnpm --version` is {found}; `corepack pnpm --version` is {corepack_actual}.",
                    f"Run `corepack enable` and `corepack prepare pnpm@{REQUIRED_PNPM_VERSION} --activate`, or use `corepack pnpm` for project commands.",
                )

    found = pnpm_actual or corepack_actual or "not found"
    return CheckResult(
        "pnpm",
        "error",
        f"Found {found}; required {REQUIRED_PNPM_VERSION}.",
        f"Run `corepack enable` and `corepack prepare pnpm@{REQUIRED_PNPM_VERSION} --activate`; offline images must pre-seed Corepack's pnpm {REQUIRED_PNPM_VERSION} package.",
    )


def check_python() -> CheckResult:
    actual = platform.python_version()
    if actual != REQUIRED_PYTHON_VERSION:
        return CheckResult(
            "Python",
            "error",
            f"This script is running under Python {actual}; required {REQUIRED_PYTHON_VERSION}.",
            f"Install Python {REQUIRED_PYTHON_VERSION} from {PYTHON_INSTALL_URL}, then run this script with that interpreter.",
        )
    return CheckResult("Python", "ok", actual)


def check_sqlc() -> CheckResult:
    if not executable_exists("sqlc"):
        return CheckResult(
            "sqlc",
            "error",
            f"sqlc is not on PATH; required {REQUIRED_SQLC_VERSION}.",
            f"Install with `{SQLC_INSTALL}` and ensure GOPATH/bin is on PATH.",
        )

    result = run_command(["sqlc", "version"])
    if result.returncode != 0:
        return CheckResult(
            "sqlc",
            "error",
            f"Unable to read sqlc version: {command_failure_detail(result)}.",
            f"Reinstall with `{SQLC_INSTALL}`.",
        )
    actual = normalize_sqlc_version(result.stdout)
    if actual != REQUIRED_SQLC_VERSION:
        return CheckResult(
            "sqlc",
            "error",
            f"Found {actual}; required {REQUIRED_SQLC_VERSION}.",
            f"Install with `{SQLC_INSTALL}` and ensure GOPATH/bin is before older sqlc binaries on PATH.",
        )
    return CheckResult("sqlc", "ok", actual)


def current_resource_platform() -> str:
    system = platform.system().lower()
    machine = platform.machine().lower()
    arch = "arm64" if machine in {"arm64", "aarch64"} else "x64"
    if system == "windows":
        return f"windows-{arch}"
    if system == "darwin":
        return f"macos-{arch}"
    return f"linux-{arch}"


def managed_chromium_paths() -> list[Path]:
    manifest_path = REPO_ROOT / ".deps" / "manifest.json"
    if not manifest_path.exists():
        return []
    try:
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return []
    wanted_platform = current_resource_platform()
    paths: list[Path] = []
    for resource in manifest.get("resources", []):
        if resource.get("kind") != "chromium" or resource.get("platform") != wanted_platform:
            continue
        resource_id = str(resource.get("id", ""))
        version = str(resource.get("version", ""))
        for entrypoint in resource.get("entrypoints", {}).get("browser", []):
            paths.append(REPO_ROOT / ".deps" / "store" / resource_id / version / entrypoint)
    return paths


def check_chromium() -> CheckResult:
    system_candidates = ["chrome", "google-chrome", "chromium", "chromium-browser", "msedge"]
    for name in system_candidates:
        path = shutil.which(name)
        if path:
            return CheckResult("Chromium", "ok", f"system browser: {path}")

    if platform.system() == "Darwin":
        for applications in (Path("/Applications"), Path.home() / "Applications"):
            for bundle, executable in (("Google Chrome", "Google Chrome"), ("Microsoft Edge", "Microsoft Edge"), ("Chromium", "Chromium")):
                candidate = applications / f"{bundle}.app" / "Contents" / "MacOS" / executable
                if candidate.is_file() and os.access(candidate, os.X_OK):
                    return CheckResult("Chromium", "ok", f"system browser: {candidate}")

    for path in managed_chromium_paths():
        if path.exists():
            return CheckResult("Chromium", "ok", f"managed browser: {path}")

    candidates = [str(path) for path in managed_chromium_paths()]
    target = candidates[0] if candidates else ".deps/store/<chromium-id>/<version>/<entrypoint>"
    return CheckResult(
        "Chromium",
        "warning",
        "No system Chrome/Chromium/Edge or prepared managed Chromium was found.",
        "\n".join(
            [
                "Install Chrome, Chromium, or Edge, or prepare the managed runtime from .deps/manifest.json.",
                f"Expected managed entrypoint for this platform: {target}",
                "Offline: copy the matching Chromium archive into cache/downloads/runtime and let runtime bootstrap unpack it, or bake the prepared .deps/store entry into the image.",
            ]
        ),
    )


def check_database_permissions() -> CheckResult:
    data_dir = REPO_ROOT / "data"
    if not data_dir.exists():
        parent = data_dir.parent
        if not os.access(parent, os.W_OK):
            return CheckResult(
                "Database path",
                "error",
                f"{data_dir} does not exist and {parent} is not writable.",
                "Create a writable data directory before running the server: `mkdir -p data`.",
            )
        return CheckResult(
            "Database path",
            "warning",
            f"{data_dir} does not exist yet; parent directory is writable.",
            "Create it explicitly in locked-down or offline images: `mkdir -p data`.",
        )
    if not data_dir.is_dir():
        return CheckResult(
            "Database path",
            "error",
            f"{data_dir} exists but is not a directory.",
            "Replace it with a writable directory named data.",
        )

    db_path: Path | None = None
    try:
        with tempfile.NamedTemporaryFile(prefix=".doctor-", suffix=".db", dir=data_dir, delete=False) as tmp:
            db_path = Path(tmp.name)
        conn = sqlite3.connect(db_path)
        try:
            conn.execute("PRAGMA user_version")
            conn.execute("CREATE TABLE doctor_write_check(id INTEGER PRIMARY KEY)")
            conn.commit()
        finally:
            conn.close()
    except OSError as exc:
        return CheckResult(
            "Database path",
            "error",
            f"{data_dir} is not writable: {exc}.",
            "Grant write permission to the data directory used by SQLite state.",
        )
    except sqlite3.Error as exc:
        return CheckResult(
            "Database path",
            "error",
            f"SQLite cannot create a database in {data_dir}: {exc}.",
            "Grant write permission to the data directory and verify the filesystem supports SQLite writes.",
        )
    finally:
        if db_path is not None:
            try:
                db_path.unlink(missing_ok=True)
            except OSError:
                pass

    return CheckResult("Database path", "ok", f"{data_dir} is writable")


def check_runtime_paths() -> list[CheckResult]:
    return [check_chromium(), check_database_permissions()]


TASK_TOOLS = {
    "all": ("go", "node", "npm", "corepack", "pnpm", "python", "sqlc"),
    "server": ("go",),
    "web": ("node", "npm", "corepack", "pnpm"),
    "launcher": ("go", "node", "npm", "corepack", "pnpm"),
    "contracts": ("go", "node", "python"),
    "sql": ("sqlc",),
    "runtime": (),
}


def check_version_files(tool_names: tuple[str, ...], root: Path = REPO_ROOT) -> CheckResult:
    errors: list[str] = []
    if "go" in tool_names:
        modules = [root / name / "go.mod" for name in ("server", "launcher", "sdk/go")]
        modules.extend(sorted((root / "examples/plugins").glob("*/go.mod")))
        for path in modules:
            match = re.search(r"^go\s+(\S+)\s*$", path.read_text(encoding="utf-8"), re.MULTILINE)
            if not match or match[1] != TOOL_VERSIONS["golang"]:
                errors.append(f"{path.relative_to(root)}: expected go {TOOL_VERSIONS['golang']}")
    if "node" in tool_names or "pnpm" in tool_names:
        packages = [root / name / "package.json" for name in ("web", "launcher", "sdk/vue")]
        packages.extend(sorted((root / "examples/plugins").glob("*/web/package.json")))
        for path in packages:
            document = json.loads(path.read_text(encoding="utf-8"))
            engines = document.get("engines", {})
            if "node" in engines and engines["node"] != TOOL_VERSIONS["nodejs"]:
                errors.append(f"{path.relative_to(root)}: engines.node differs from .tool-versions")
            if "pnpm" in engines and engines["pnpm"] != REQUIRED_PNPM_VERSION:
                errors.append(f"{path.relative_to(root)}: engines.pnpm differs from .tool-versions")
            if "packageManager" in document and document["packageManager"] != f"pnpm@{REQUIRED_PNPM_VERSION}":
                errors.append(f"{path.relative_to(root)}: packageManager differs from .tool-versions")
    return CheckResult("Version declarations", "error" if errors else "ok", "; ".join(errors) if errors else "selected ecosystem files match .tool-versions")


def run_checks(include_runtime: bool, task: str = "all") -> list[CheckResult]:
    selected = TASK_TOOLS[task]
    checks = {"go": check_go, "node": check_node, "npm": check_npm, "corepack": check_corepack, "pnpm": check_pnpm, "python": check_python, "sqlc": check_sqlc}
    results = [check_version_files(selected), *(checks[name]() for name in selected)]
    if include_runtime:
        results.extend(check_runtime_paths())
    return results


def print_results(results: list[CheckResult]) -> None:
    for result in results:
        stream = sys.stderr if result.failed else sys.stdout
        print(f"[{result.status}] {result.name}: {result.detail}", file=stream)
        if result.remediation:
            for line in result.remediation.splitlines():
                print(f"  fix: {line}", file=stream)


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Check RayleaBot development toolchain.")
    parser.add_argument("--task", choices=TASK_TOOLS, default="all", help="Check tools needed for the selected task; default checks the complete frozen toolchain.")
    parser.add_argument(
        "--toolchain-only",
        action="store_true",
        help="Skip runtime resource and database permission checks.",
    )
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(sys.argv[1:] if argv is None else argv)
    try:
        results = run_checks(include_runtime=not args.toolchain_only and args.task in {"all", "server", "launcher", "runtime"}, task=args.task)
    except (OSError, ValueError) as exc:
        print(f"[error] Unable to read version declarations: {exc}", file=sys.stderr)
        return 1
    print_results(results)
    return 1 if any(result.failed for result in results) else 0


if __name__ == "__main__":
    raise SystemExit(main())
