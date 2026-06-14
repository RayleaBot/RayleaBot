#!/usr/bin/env python3
from __future__ import annotations

import ast
import re
import sys
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

PACKAGE_DECL_RE = re.compile(r"^\s*package\s+([A-Za-z_][A-Za-z0-9_]*)\b", re.MULTILINE)
IMPORT_SINGLE_RE = re.compile(r'^\s*import\s+(?:[.\w]+\s+)?"([^"]+)"', re.MULTILINE)
IMPORT_BLOCK_RE = re.compile(r"^\s*import\s*\((.*?)^\s*\)", re.MULTILINE | re.DOTALL)
IMPORT_LINE_RE = re.compile(r'^\s*(?:[.\w]+\s+)?"([^"]+)"', re.MULTILINE)


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

    check_package_sizes(files, warnings, errors)
    check_file_sizes(files, warnings, errors)
    check_import_boundaries(files, errors)
    check_fan_out(files, warnings)
    check_disallowed_dirs(server_internal, root, errors)
    check_package_names(files, warnings)

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
        if path.name in DISALLOWED_PACKAGE_DIR_NAMES:
            errors.append(f"{path.relative_to(root).as_posix()} uses a disallowed generic package name")


def check_package_names(files: list[GoFile], warnings: list[str]) -> None:
    seen: set[str] = set()
    for file in files:
        if file.package_dir in seen or not file.package_name:
            continue
        seen.add(file.package_dir)
        leaf = Path(file.package_dir).name
        if file.package_name != leaf:
            warnings.append(f"{file.package_dir} package name is {file.package_name}; directory leaf is {leaf}")


if __name__ == "__main__":
    raise SystemExit(main())
