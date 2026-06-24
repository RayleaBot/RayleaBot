#!/usr/bin/env python3
from __future__ import annotations

import ast
import json
import re
import sys
from collections import Counter
from dataclasses import dataclass
from pathlib import Path

MODULE = "github.com/RayleaBot/RayleaBot/server"
INTERNAL_PREFIX = MODULE + "/internal/"

PACKAGE_PROD_WARN = 20
PACKAGE_PROD_FAIL = 30
PACKAGE_TOTAL_WARN = 30
PACKAGE_TOTAL_FAIL = 50
FILE_WARN_LINES = 500
FILE_FAIL_LINES = 1000
FAN_OUT_WARN = 12

PACKAGE_BASELINES = {
    "internal/management/http": {"prod": 80, "total": 94},
    "internal/app": {"prod": 30, "total": 43},
    "internal/integrations/bilibili/source": {"prod": 39, "total": 41},
    "internal/render/service": {"prod": 27, "total": 35},
    "internal/bot/adapter/onebot11/shell": {"prod": 29, "total": 34},
    "internal/logging": {"prod": 30, "total": 34},
    "internal/plugins/lifecycle": {"prod": 23, "total": 29},
    "internal/plugins/runtime/manager": {"prod": 22, "total": 28},
    "internal/plugins/actions": {"prod": 25, "total": 27},
    "internal/system": {"prod": 26, "total": 26},
}

ALLOWED_MANAGEMENT_HTTP_IMPORTERS = (
    "internal/app",
    "internal/management",
)

ALLOWED_FAN_OUT_PREFIXES = (
    "internal/app",
    "internal/management/router",
)

DISALLOWED_PACKAGE_DIR_NAMES = {"common", "utils", "helper", "helpers"}
ALLOWED_GENERIC_PACKAGE_DIRS = {
    "server/internal/integrations/common",
}

PACKAGE_DECL_RE = re.compile(r"^\s*package\s+([A-Za-z_][A-Za-z0-9_]*)\b", re.MULTILINE)
IMPORT_SINGLE_RE = re.compile(r'^\s*import\s+(?:[.\w]+\s+)?"([^"]+)"', re.MULTILINE)
IMPORT_BLOCK_RE = re.compile(r"^\s*import\s*\((.*?)^\s*\)", re.MULTILINE | re.DOTALL)
IMPORT_LINE_RE = re.compile(r'^\s*(?:[.\w]+\s+)?"([^"]+)"', re.MULTILINE)
PROCESS_EXIT_RE = re.compile(r"\b(?:os\.Exit|log\.Fatalf?|log\.Fatalln)\s*\(")
RAW_SQL_CALL_RE = re.compile(r"\.(?:Exec|ExecContext|QueryContext|QueryRowContext|QueryRow)\s*\(")


@dataclass(frozen=True)
class GoFile:
    path: Path
    rel: str
    package_dir: str
    is_test: bool
    is_generated: bool
    line_count: int
    package_name: str
    imports: tuple[str, ...]


def main() -> int:
    root = Path(__file__).resolve().parents[1]
    server_internal = root / "server" / "internal"
    files = collect_go_files(root, server_internal)

    errors: list[str] = []
    warnings: list[str] = []
    manual_sql_exceptions = load_manual_sql_exceptions(root, errors)
    metric_lines = check_architecture_budget(files, root, errors)

    check_package_sizes(files, warnings, errors)
    check_file_sizes(files, warnings, errors)
    check_import_boundaries(files, errors)
    check_fan_out(files, warnings)
    check_disallowed_dirs(server_internal, root, errors)
    check_package_names(files, warnings)
    check_process_exit_calls(files, errors)
    check_manual_sql_exceptions(files, root, manual_sql_exceptions, errors)

    for message in metric_lines:
        print(message)
    for message in warnings:
        print(f"WARN {message}")
    for message in errors:
        print(f"ERROR {message}")

    if errors:
        print(f"server structure check failed: {len(errors)} error(s), {len(warnings)} warning(s)")
        return 1

    print(f"server structure check passed: {len(warnings)} warning(s)")
    return 0


def collect_go_files(root: Path, server_internal: Path) -> list[GoFile]:
    files: list[GoFile] = []
    for path in sorted(server_internal.rglob("*.go")):
        rel = path.relative_to(root).as_posix()
        text = path.read_text(encoding="utf-8")
        package_dir = path.parent.relative_to(root / "server").as_posix()
        is_test = path.name.endswith("_test.go")
        is_generated = is_generated_go_file(path, text)
        package_match = PACKAGE_DECL_RE.search(text)
        package_name = package_match.group(1) if package_match else ""
        files.append(
            GoFile(
                path=path,
                rel=rel,
                package_dir=package_dir,
                is_test=is_test,
                is_generated=is_generated,
                line_count=text.count("\n") + (0 if text.endswith("\n") else 1),
                package_name=package_name,
                imports=tuple(parse_imports(text)),
            )
        )
    return files


def parse_imports(text: str) -> list[str]:
    imports = IMPORT_SINGLE_RE.findall(text)
    for block in IMPORT_BLOCK_RE.findall(text):
        imports.extend(IMPORT_LINE_RE.findall(block))
    return imports


def is_generated_go_file(path: Path, text: str) -> bool:
    name = path.name
    if name.endswith("_gen.go") or name.endswith(".pb.go"):
        return True
    if "sqlcgen" in path.parts or "schemaassets" in path.parts:
        return True
    return "Code generated" in text[:512]


def check_package_sizes(files: list[GoFile], warnings: list[str], errors: list[str]) -> None:
    packages: dict[str, list[GoFile]] = {}
    for file in files:
        packages.setdefault(file.package_dir, []).append(file)

    for package_dir, package_files in sorted(packages.items()):
        prod_count = sum(1 for file in package_files if not file.is_test)
        total_count = len(package_files)
        baseline = PACKAGE_BASELINES.get(package_dir)

        if baseline is not None:
            if prod_count > baseline["prod"]:
                errors.append(f"{package_dir} production files {prod_count} exceed baseline {baseline['prod']}")
            if total_count > baseline["total"]:
                errors.append(f"{package_dir} total files {total_count} exceed baseline {baseline['total']}")
        else:
            if prod_count > PACKAGE_PROD_FAIL:
                errors.append(f"{package_dir} production files {prod_count} exceed limit {PACKAGE_PROD_FAIL}")
            if total_count > PACKAGE_TOTAL_FAIL:
                errors.append(f"{package_dir} total files {total_count} exceed limit {PACKAGE_TOTAL_FAIL}")

        if prod_count > PACKAGE_PROD_WARN:
            warnings.append(f"{package_dir} production files {prod_count} exceed warning {PACKAGE_PROD_WARN}")
        if total_count > PACKAGE_TOTAL_WARN:
            warnings.append(f"{package_dir} total files {total_count} exceed warning {PACKAGE_TOTAL_WARN}")


def check_file_sizes(files: list[GoFile], warnings: list[str], errors: list[str]) -> None:
    for file in files:
        if file.is_test or file.is_generated:
            continue
        if file.line_count > FILE_FAIL_LINES:
            errors.append(f"{file.rel} has {file.line_count} lines; limit is {FILE_FAIL_LINES}")
        elif file.line_count > FILE_WARN_LINES:
            warnings.append(f"{file.rel} has {file.line_count} lines; warning is {FILE_WARN_LINES}")


def check_import_boundaries(files: list[GoFile], errors: list[str]) -> None:
    management_http_import = INTERNAL_PREFIX + "management/http"
    for file in files:
        if file.is_test:
            continue
        if management_http_import not in file.imports:
            continue
        if file.package_dir.startswith(ALLOWED_MANAGEMENT_HTTP_IMPORTERS):
            continue
        errors.append(f"{file.rel} imports internal/management/http outside management or apphost")

    for file in files:
        if file.is_test:
            continue
        if file.package_dir not in {"internal/system", "internal/configruntime"}:
            continue
        if "managementhttp." in file.path.read_text(encoding="utf-8"):
            errors.append(f"{file.rel} references management HTTP DTO symbols")


def check_fan_out(files: list[GoFile], warnings: list[str]) -> None:
    fan_out: dict[str, set[str]] = {}
    for file in files:
        if file.is_test:
            continue
        imports = fan_out.setdefault(file.package_dir, set())
        for imported in file.imports:
            if imported.startswith(INTERNAL_PREFIX):
                imports.add(imported.removeprefix(MODULE + "/"))

    for package_dir, imports in sorted(fan_out.items()):
        if package_dir.startswith(ALLOWED_FAN_OUT_PREFIXES):
            continue
        if len(imports) > FAN_OUT_WARN:
            warnings.append(f"{package_dir} imports {len(imports)} internal packages; warning is {FAN_OUT_WARN}")


def check_disallowed_dirs(server_internal: Path, root: Path, errors: list[str]) -> None:
    for path in sorted(server_internal.rglob("*")):
        if not path.is_dir():
            continue
        rel = path.relative_to(root).as_posix()
        if rel in ALLOWED_GENERIC_PACKAGE_DIRS:
            continue
        if path.name in DISALLOWED_PACKAGE_DIR_NAMES:
            errors.append(f"{rel} uses a disallowed generic package name")


def check_package_names(files: list[GoFile], warnings: list[str]) -> None:
    seen: set[str] = set()
    for file in files:
        if file.package_dir in seen or not file.package_name:
            continue
        seen.add(file.package_dir)
        leaf = Path(file.package_dir).name
        if file.package_name != leaf:
            warnings.append(f"{file.package_dir} package name is {file.package_name}; directory leaf is {leaf}")


def check_process_exit_calls(files: list[GoFile], errors: list[str]) -> None:
    for file in files:
        if file.is_test or file.is_generated:
            continue
        text = file.path.read_text(encoding="utf-8")
        if PROCESS_EXIT_RE.search(text):
            errors.append(f"{file.rel} calls os.Exit or log.Fatal outside cmd")


def load_manual_sql_exceptions(root: Path, errors: list[str]) -> dict[str, str]:
    registry_path = root / "docs" / "engineering" / "manual-sql-exceptions.json"
    try:
        raw = registry_path.read_text(encoding="utf-8")
    except FileNotFoundError:
        errors.append(f"{registry_path.relative_to(root).as_posix()} is missing")
        return {}

    try:
        parsed = json.loads(raw)
    except json.JSONDecodeError as exc:
        errors.append(f"{registry_path.relative_to(root).as_posix()} is invalid JSON: {exc}")
        return {}

    allowed_files = parsed.get("allowed_files")
    if not isinstance(allowed_files, dict):
        errors.append(f"{registry_path.relative_to(root).as_posix()} must contain an allowed_files object")
        return {}

    registry: dict[str, str] = {}
    for rel, reason in allowed_files.items():
        if not isinstance(rel, str) or not isinstance(reason, str) or not reason.strip():
            errors.append(f"{registry_path.relative_to(root).as_posix()} has an invalid manual SQL entry")
            continue
        registry[rel.replace("\\", "/")] = reason.strip()
    return registry


def check_manual_sql_exceptions(files: list[GoFile], root: Path, registry: dict[str, str], errors: list[str]) -> None:
    raw_sql_files: set[str] = set()
    for file in files:
        if file.is_test or file.is_generated or file.package_dir.startswith("internal/sqlcgen"):
            continue
        text = file.path.read_text(encoding="utf-8")
        if RAW_SQL_CALL_RE.search(text):
            raw_sql_files.add(file.rel)

    for rel in sorted(raw_sql_files):
        if rel not in registry:
            errors.append(f"{rel} uses handwritten SQL but is not listed in docs/engineering/manual-sql-exceptions.json")

    for rel in sorted(registry):
        path = root / Path(rel)
        if not path.exists():
            errors.append(f"docs/engineering/manual-sql-exceptions.json references missing file {rel}")
        elif rel not in raw_sql_files:
            errors.append(f"docs/engineering/manual-sql-exceptions.json lists {rel}, but no handwritten SQL was found")


def check_architecture_budget(files: list[GoFile], root: Path, errors: list[str]) -> list[str]:
    budget_path = root / "docs" / "engineering" / "server-architecture-budget.json"
    try:
        budget = json.loads(budget_path.read_text(encoding="utf-8"))
    except FileNotFoundError:
        errors.append(f"{budget_path.relative_to(root).as_posix()} is missing")
        return []
    except json.JSONDecodeError as exc:
        errors.append(f"{budget_path.relative_to(root).as_posix()} is invalid JSON: {exc}")
        return []

    production_files = [file for file in files if not file.is_test and not file.is_generated]
    package_files: dict[str, list[GoFile]] = {}
    package_imports: dict[str, set[str]] = {}
    app_external_imports: set[str] = set()
    for file in production_files:
        package_files.setdefault(file.package_dir, []).append(file)
        imports = package_imports.setdefault(file.package_dir, set())
        for imported in file.imports:
            if not imported.startswith(INTERNAL_PREFIX):
                continue
            rel_import = imported.removeprefix(MODULE + "/")
            imports.add(rel_import)
            if file.package_dir == "internal/app" or file.package_dir.startswith("internal/app/"):
                if not rel_import.startswith("internal/app"):
                    app_external_imports.add(rel_import)

    single_file_packages = {package_dir: package_files for package_dir, package_files in package_files.items() if len(package_files) == 1}
    server_root = root / "server"
    server_directory_count = sum(
        1
        for path in server_root.rglob("*")
        if path.is_dir() and not {".git", "dist", ".gocache"}.intersection(path.parts)
    )
    metrics = {
        "production_package_count": len(package_files),
        "single_file_production_package_count": len(single_file_packages),
        "two_file_production_package_count": sum(1 for package_files in package_files.values() if len(package_files) == 2),
        "internal_app_external_internal_import_union": len(app_external_imports),
        "module_go_single_file_package_count": sum(1 for package_files in single_file_packages.values() if package_files[0].path.name == "module.go"),
        "server_directory_count": server_directory_count,
    }

    metric_lines = [f"METRIC {name}={value}" for name, value in sorted(metrics.items())]
    generic_counts = Counter(file.path.name for file in production_files if file.path.name in {"routes.go", "repository.go", "http.go", "registry.go", "module.go", "config.go", "errors.go"})
    for name, value in sorted(generic_counts.items()):
        metric_lines.append(f"METRIC generic_filename.{name}={value}")

    budget_metrics = budget.get("metrics", {})
    if not isinstance(budget_metrics, dict):
        errors.append(f"{budget_path.relative_to(root).as_posix()} metrics must be an object")
        budget_metrics = {}
    for name, value in metrics.items():
        maximum = budget_metric_max(budget_metrics, name, errors, budget_path, root)
        if maximum is not None and value > maximum:
            errors.append(f"{name} {value} exceeds architecture budget {maximum}")

    fan_out_budget = budget.get("package_internal_fan_out", {})
    if not isinstance(fan_out_budget, dict):
        errors.append(f"{budget_path.relative_to(root).as_posix()} package_internal_fan_out must be an object")
        fan_out_budget = {}
    for package_dir, entry in sorted(fan_out_budget.items()):
        maximum = budget_entry_max(entry, errors, budget_path, root, f"package_internal_fan_out.{package_dir}")
        value = len(package_imports.get(package_dir, set()))
        metric_lines.append(f"METRIC package_internal_fan_out.{package_dir}={value}")
        if maximum is not None and value > maximum:
            errors.append(f"{package_dir} internal fan-out {value} exceeds architecture budget {maximum}")

    allowlist = budget.get("single_file_production_package_allowlist", {})
    if not isinstance(allowlist, dict):
        errors.append(f"{budget_path.relative_to(root).as_posix()} single_file_production_package_allowlist must be an object")
        allowlist = {}
    for package_dir in sorted(single_file_packages):
        reason = allowlist.get(package_dir)
        if not isinstance(reason, str) or not reason.strip():
            errors.append(f"{package_dir} is a single-file production package without an allowlist reason")
    for package_dir in sorted(allowlist):
        if package_dir not in single_file_packages:
            errors.append(f"single-file package allowlist references {package_dir}, but it is not currently a single-file production package")

    return metric_lines


def budget_metric_max(metrics: dict[str, object], name: str, errors: list[str], budget_path: Path, root: Path) -> int | None:
    return budget_entry_max(metrics.get(name), errors, budget_path, root, f"metrics.{name}")


def budget_entry_max(entry: object, errors: list[str], budget_path: Path, root: Path, name: str) -> int | None:
    if not isinstance(entry, dict):
        errors.append(f"{budget_path.relative_to(root).as_posix()} missing {name}")
        return None
    maximum = entry.get("max")
    if not isinstance(maximum, int):
        errors.append(f"{budget_path.relative_to(root).as_posix()} {name}.max must be an integer")
        return None
    return maximum


if __name__ == "__main__":
    raise SystemExit(main())
