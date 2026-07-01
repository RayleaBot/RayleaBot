#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import platform
import shutil
import sqlite3
import subprocess
import sys
import tempfile
from dataclasses import dataclass

REPO_ROOT = Path(__file__).resolve().parents[1]

REQUIRED_GO_VERSION = "go1.25.11"
REQUIRED_NODE_VERSION = "v24.14.0"
REQUIRED_PNPM_VERSION = "11.9.0"
REQUIRED_SQLC_VERSION = "v1.29.0"

GO_INSTALL_URL = "https://go.dev/dl/"
NODE_INSTALL_URL = "https://nodejs.org/dist/v24.14.0/"
SQLC_INSTALL = "go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0"


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


def run_command(args: list[str]) -> CommandOutput:
    resolved = shutil.which(args[0])
    if resolved:
        args = [resolved, *args[1:]]
    try:
        result = subprocess.run(args, capture_output=True, check=False, text=True)
    except OSError as exc:
        return CommandOutput(127, "", str(exc))
    return CommandOutput(result.returncode, result.stdout.strip(), result.stderr.strip())


def executable_exists(name: str) -> bool:
    return shutil.which(name) is not None


def first_line(value: str) -> str:
    return value.strip().splitlines()[0].strip() if value.strip() else ""


def version_from_go_output(output: str) -> str:
    parts = output.split()
    return parts[2] if len(parts) >= 3 else ""


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
                    "Install Go 1.25.11 before running server tests.",
                    f"Download: {GO_INSTALL_URL}",
                    "Windows: winget install GoLang.Go --version 1.25.11",
                    "Linux x64 online: curl -LO https://go.dev/dl/go1.25.11.linux-amd64.tar.gz && sudo tar -C /usr/local -xzf go1.25.11.linux-amd64.tar.gz",
                    "Offline: copy the matching go1.25.11 archive into the runner image and put its bin directory on PATH; set GOTOOLCHAIN=local for a local-only failure.",
                ]
            ),
        )

    result = run_command(["go", "version"])
    if result.returncode != 0:
        return CheckResult(
            "Go",
            "error",
            f"Unable to read Go version: {command_failure_detail(result)}.",
            "Fix PATH so `go version` runs, then rerun `python scripts/check-toolchain.py`.",
        )
    actual = version_from_go_output(result.stdout)
    if actual != REQUIRED_GO_VERSION:
        return CheckResult(
            "Go",
            "error",
            f"Found {actual or first_line(result.stdout)}; required {REQUIRED_GO_VERSION}.",
            "\n".join(
                [
                    "Install the exact Go patch version used by server/go.mod.",
                    f"Download: {GO_INSTALL_URL}",
                    "Windows: winget install GoLang.Go --version 1.25.11",
                    "Linux x64 online: curl -LO https://go.dev/dl/go1.25.11.linux-amd64.tar.gz && sudo tar -C /usr/local -xzf go1.25.11.linux-amd64.tar.gz",
                    "Offline: preinstall go1.25.11 in the image or workstation and set GOTOOLCHAIN=local before running tests.",
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
            f"Install Node.js 24.14.0 from {NODE_INSTALL_URL}; offline images must preinstall it before running Web or Launcher tests.",
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
            f"Install Node.js 24.14.0 from {NODE_INSTALL_URL}, then run `corepack enable`.",
        )
    return CheckResult("Node.js", "ok", actual)


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
                    "Run `corepack enable` and `corepack prepare pnpm@11.9.0 --activate`, or use `corepack pnpm` for project commands.",
                )

    found = pnpm_actual or corepack_actual or "not found"
    return CheckResult(
        "pnpm",
        "error",
        f"Found {found}; required {REQUIRED_PNPM_VERSION}.",
        "Run `corepack enable` and `corepack prepare pnpm@11.9.0 --activate`; offline images must pre-seed Corepack's pnpm 11.9.0 package.",
    )


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


def run_checks(include_runtime: bool) -> list[CheckResult]:
    results = [check_go(), check_node(), check_pnpm(), check_sqlc()]
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
    parser.add_argument(
        "--toolchain-only",
        action="store_true",
        help="Skip runtime resource and database permission checks.",
    )
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(sys.argv[1:] if argv is None else argv)
    results = run_checks(include_runtime=not args.toolchain_only)
    print_results(results)
    return 1 if any(result.failed for result in results) else 0


if __name__ == "__main__":
    raise SystemExit(main())
