#!/usr/bin/env python3
from __future__ import annotations

import re
from dataclasses import dataclass
from pathlib import Path

MODULE = "github.com/RayleaBot/RayleaBot/server"
INTERNAL_PREFIX = MODULE + "/internal/"

DISALLOWED_PACKAGE_DIR_NAMES = {"common", "utils", "helper", "helpers"}

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
    package_name: str
    imports: tuple[str, ...]


def main() -> int:
    root = Path(__file__).resolve().parents[1]
    server_internal = root / "server" / "internal"
    files = collect_go_files(root, server_internal)

    errors: list[str] = []
    warnings: list[str] = []

    check_plugin_boundaries(files, errors)
    check_adapter_boundaries(files, errors)
    check_model_boundaries(files, errors)
    check_disallowed_dirs(server_internal, root, errors)
    check_package_names(files, warnings)
    check_process_exit_calls(files, errors)
    check_management_sql(files, errors)

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
        if within_package(file.package_dir, "internal/plugins/runtime"):
            for imported in imports:
                if within_package(imported, INTERNAL_PREFIX + "management"):
                    errors.append(f"{file.rel} imports management projection from plugin runtime")
        if within_package(file.package_dir, "internal/management"):
            for imported in imports:
                if within_package(imported, INTERNAL_PREFIX + "plugins/runtime"):
                    errors.append(f"{file.rel} imports plugin runtime internals from management projection")


def within_package(path: str, root: str) -> bool:
    return path == root or path.startswith(root + "/")


def check_adapter_boundaries(files: list[GoFile], errors: list[str]) -> None:
    for file in files:
        if file.is_test or not within_package(file.package_dir, "internal/bot/adapters"):
            continue
        for imported in file.imports:
            if any(within_package(imported, INTERNAL_PREFIX + name) for name in ("management", "config/runtime", "operations/system", "app")):
                errors.append(f"{file.rel} imports {imported}; adapter domain must own its state and reload errors")


def check_model_boundaries(files: list[GoFile], errors: list[str]) -> None:
    for file in files:
        if file.is_test:
            continue
        forbidden: tuple[str, ...] = ()
        if file.package_dir == "internal/plugins":
            forbidden = ("storage", "sqlcgen", "plugins/catalog", "plugins/lifecycle", "plugins/runtime", "plugins/actions", "management")
        elif within_package(file.package_dir, "internal/platform/health"):
            forbidden = ("",)
        elif within_package(file.package_dir, "internal/platform/runtimepaths"):
            forbidden = ("plugins", "operations/recovery", "operations/system", "management", "app")
        elif within_package(file.package_dir, "internal/bot/menu") or within_package(file.package_dir, "internal/render"):
            forbidden = ("plugins/actions", "plugins/lifecycle", "plugins/runtime")
        for imported in file.imports:
            if any(imported.startswith(INTERNAL_PREFIX) if name == "" else within_package(imported, INTERNAL_PREFIX + name) for name in forbidden):
                errors.append(f"{file.rel} imports {imported}; model or helper package depends on an implementation owner")


def check_disallowed_dirs(server_internal: Path, root: Path, errors: list[str]) -> None:
    for path in sorted(server_internal.rglob("*")):
        if not path.is_dir():
            continue
        rel = path.relative_to(root).as_posix()
        if path.name in DISALLOWED_PACKAGE_DIR_NAMES:
            errors.append(f"{rel} uses a disallowed generic package name")


def check_package_names(files: list[GoFile], warnings: list[str]) -> None:
    seen: set[str] = set()
    for file in files:
        if file.is_test or file.package_dir in seen or not file.package_name:
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


def check_management_sql(files: list[GoFile], errors: list[str]) -> None:
    for file in files:
        if file.is_test or file.is_generated or not within_package(file.package_dir, "internal/management"):
            continue
        if RAW_SQL_CALL_RE.search(file.path.read_text(encoding="utf-8")):
            errors.append(f"{file.rel} uses handwritten SQL in management handler layer")


if __name__ == "__main__":
    raise SystemExit(main())
