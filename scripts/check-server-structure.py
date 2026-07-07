#!/usr/bin/env python3
from __future__ import annotations

import json
import re
import sys
from dataclasses import dataclass
from pathlib import Path

MODULE = "github.com/RayleaBot/RayleaBot/server"
INTERNAL_PREFIX = MODULE + "/internal/"

DISALLOWED_PACKAGE_DIR_NAMES = {"common", "utils", "helper", "helpers"}
ALLOWED_GENERIC_PACKAGE_DIRS: set[str] = set()

PACKAGE_DECL_RE = re.compile(r"^\s*package\s+([A-Za-z_][A-Za-z0-9_]*)\b", re.MULTILINE)
IMPORT_SINGLE_RE = re.compile(r'^\s*import\s+(?:[.\w]+\s+)?"([^"]+)"', re.MULTILINE)
IMPORT_BLOCK_RE = re.compile(r"^\s*import\s*\((.*?)^\s*\)", re.MULTILINE | re.DOTALL)
IMPORT_LINE_RE = re.compile(r'^\s*(?:[.\w]+\s+)?"([^"]+)"', re.MULTILINE)
PROCESS_EXIT_RE = re.compile(r"\b(?:os\.Exit|log\.Fatalf?|log\.Fatalln)\s*\(")
RAW_SQL_CALL_RE = re.compile(r"\.(?:Exec|ExecContext|QueryContext|QueryRowContext|QueryRow)\s*\(")
SMALL_PRODUCTION_PACKAGE_WARNING_LINES = 150
ALLOWED_SMALL_PRODUCTION_PACKAGE_DIRS = {
    "internal/command",
    "internal/health",
    "internal/logpath",
    "internal/runtimepaths",
}


@dataclass(frozen=True)
class GoFile:
    path: Path
    rel: str
    package_dir: str
    is_test: bool
    is_generated: bool
    package_name: str
    imports: tuple[str, ...]


def main() -> int:
    root = Path(__file__).resolve().parents[1]
    server_internal = root / "server" / "internal"
    files = collect_go_files(root, server_internal)

    errors: list[str] = []
    warnings: list[str] = []
    manual_sql_exceptions = load_manual_sql_exceptions(root, errors)

    check_plugin_boundaries(files, errors)
    check_disallowed_dirs(server_internal, root, errors)
    check_package_names(files, warnings)
    check_small_production_packages(files, warnings)
    check_process_exit_calls(files, errors)
    check_manual_sql_exceptions(files, root, manual_sql_exceptions, errors)

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
    if "sqlcgen" in path.parts:
        return True
    return "Code generated" in text[:512]


def check_plugin_boundaries(files: list[GoFile], errors: list[str]) -> None:
    for file in files:
        if file.is_test:
            continue
        imports = set(file.imports)
        if file.package_dir.startswith("internal/plugins/runtime"):
            for imported in imports:
                if imported.startswith(INTERNAL_PREFIX + "management"):
                    errors.append(f"{file.rel} imports management projection from plugin runtime")
        if file.package_dir == "internal/management":
            for imported in imports:
                if imported.startswith(INTERNAL_PREFIX + "plugins/runtime"):
                    errors.append(f"{file.rel} imports plugin runtime internals from management projection")


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


def check_small_production_packages(files: list[GoFile], warnings: list[str]) -> None:
    package_lines: dict[str, int] = {}
    package_files: dict[str, int] = {}
    for file in files:
        if file.is_test or file.is_generated:
            continue
        text = file.path.read_text(encoding="utf-8")
        line_count = text.count("\n")
        if text and not text.endswith("\n"):
            line_count += 1
        package_lines[file.package_dir] = package_lines.get(file.package_dir, 0) + line_count
        package_files[file.package_dir] = package_files.get(file.package_dir, 0) + 1

    for package_dir in sorted(package_lines):
        if package_dir in ALLOWED_SMALL_PRODUCTION_PACKAGE_DIRS:
            continue
        total = package_lines[package_dir]
        if total >= SMALL_PRODUCTION_PACKAGE_WARNING_LINES:
            continue
        warnings.append(
            f"{package_dir} has {total} production line(s) across {package_files[package_dir]} file(s); consider merging thin same-lifecycle code"
        )


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
    for rel, entry in allowed_files.items():
        if not isinstance(rel, str):
            errors.append(f"{registry_path.relative_to(root).as_posix()} has an invalid manual SQL entry")
            continue
        if not isinstance(entry, dict):
            errors.append(f"{registry_path.relative_to(root).as_posix()} entry {rel} must be an object")
            continue
        category = entry.get("category")
        reason = entry.get("reason")
        owner = entry.get("owner")
        target_action = entry.get("target_action")
        revisit_after = entry.get("revisit_after")
        if category not in {"A", "B", "C", "D"}:
            errors.append(f"{registry_path.relative_to(root).as_posix()} entry {rel} has invalid category")
        for field_name, value in {
            "reason": reason,
            "owner": owner,
            "target_action": target_action,
            "revisit_after": revisit_after,
        }.items():
            if not isinstance(value, str) or not value.strip():
                errors.append(f"{registry_path.relative_to(root).as_posix()} entry {rel} missing {field_name}")
        if not isinstance(reason, str) or not reason.strip():
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
        if "/management/" in rel:
            errors.append(f"{rel} uses handwritten SQL in management handler layer")
        if rel not in registry:
            errors.append(f"{rel} uses handwritten SQL but is not listed in docs/engineering/manual-sql-exceptions.json")

    for rel in sorted(registry):
        path = root / Path(rel)
        if not path.exists():
            errors.append(f"docs/engineering/manual-sql-exceptions.json references missing file {rel}")
        elif rel not in raw_sql_files:
            errors.append(f"docs/engineering/manual-sql-exceptions.json lists {rel}, but no handwritten SQL was found")


if __name__ == "__main__":
    raise SystemExit(main())
