#!/usr/bin/env python3
"""Generate deterministic third-party notices from installed locked dependencies."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import shutil
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Any


REPO_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_OUTPUT = REPO_ROOT / "THIRD_PARTY_NOTICES.md"
UNKNOWN_LICENSE_MARKERS = {"", "unknown", "unlicensed", "none", "n/a"}
REVIEWED_LICENSE_EXPRESSIONS = {
    "0BSD",
    "Apache-2.0",
    "BSD-2-Clause",
    "BSD-3-Clause",
    "ISC",
    "MIT",
    "MPL-2.0",
}
LICENSE_FILE_PREFIXES = ("license", "copying", "copyright", "notice", "patents")


class NoticeGenerationError(RuntimeError):
    pass


@dataclass(frozen=True)
class Component:
    ecosystem: str
    name: str
    version: str
    license_expression: str
    notice: str

    @property
    def key(self) -> tuple[str, str, str]:
        return self.ecosystem, self.name, self.version


def run_command(command: list[str], cwd: Path, env: dict[str, str] | None = None) -> str:
    completed = subprocess.run(
        command,
        cwd=cwd,
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        encoding="utf-8",
        env=env,
    )
    if completed.returncode != 0:
        detail = completed.stderr.strip() or completed.stdout.strip() or f"exit code {completed.returncode}"
        raise NoticeGenerationError(f"{' '.join(command)} failed in {cwd}: {detail}")
    return completed.stdout


def pnpm_command() -> list[str]:
    pnpm = shutil.which("pnpm")
    if pnpm:
        return command_for_executable(pnpm)
    corepack = shutil.which("corepack")
    if corepack:
        return [*command_for_executable(corepack), "pnpm"]
    raise NoticeGenerationError("pnpm or corepack is required to inspect production licenses")


def command_for_executable(executable: str) -> list[str]:
    if Path(executable).suffix.lower() in {".cmd", ".bat"}:
        return [os.environ.get("COMSPEC", "cmd.exe"), "/d", "/c", executable]
    return [executable]


def normalize_license_expression(value: Any, component: str) -> str:
    if isinstance(value, dict):
        value = value.get("type")
    expression = str(value or "").strip()
    if expression.lower() in UNKNOWN_LICENSE_MARKERS or "unknown" in expression.lower():
        raise NoticeGenerationError(f"{component} has an unknown or missing license expression")
    if expression not in REVIEWED_LICENSE_EXPRESSIONS:
        raise NoticeGenerationError(f"{component} has an unreviewed license expression: {expression}")
    return expression


def normalize_license_text(value: str) -> str:
    normalized = value.replace("\r\n", "\n").replace("\r", "\n")
    return "\n".join(line.expandtabs(4).rstrip() for line in normalized.split("\n")).strip()


def license_documents(package_dir: Path, component: str, declared_expression: str | None = None) -> str:
    if not package_dir.is_dir():
        raise NoticeGenerationError(f"{component} package directory is missing: {package_dir}")
    files = [
        path
        for path in package_dir.iterdir()
        if path.is_file() and path.name.lower().startswith(LICENSE_FILE_PREFIXES)
    ]
    primary = [path for path in files if path.name.lower().startswith(("license", "copying"))]
    if not primary:
        if declared_expression is None:
            raise NoticeGenerationError(f"{component} has no LICENSE or COPYING file")
        return (
            "[Declared license]\n"
            f"{declared_expression}\n\n"
            "The installed package declares this license expression in package.json "
            "but does not include a standalone LICENSE or COPYING file."
        )

    sections: list[str] = []
    for path in sorted(files, key=lambda item: item.name.lower()):
        try:
            content = normalize_license_text(path.read_text(encoding="utf-8"))
        except UnicodeDecodeError:
            content = normalize_license_text(path.read_text(encoding="latin-1"))
        if not content:
            raise NoticeGenerationError(f"{component} has an empty license file: {path.name}")
        sections.append(f"[{path.name}]\n{content}")
    return "\n\n".join(sections)


def components_from_pnpm_payload(payload: dict[str, Any], project_dir: Path, ecosystem: str) -> list[Component]:
    components: dict[tuple[str, str, str], Component] = {}
    for reported_expression, entries in payload.items():
        expression = normalize_license_expression(reported_expression, f"{ecosystem} dependency group")
        if not isinstance(entries, list):
            raise NoticeGenerationError(f"{ecosystem} license report group {reported_expression!r} is not a list")
        for entry in entries:
            if not isinstance(entry, dict):
                raise NoticeGenerationError(f"{ecosystem} license report contains a non-object entry")
            paths = entry.get("paths")
            if not isinstance(paths, list) or not paths:
                raise NoticeGenerationError(f"{ecosystem} dependency {entry.get('name')} has no installed path")
            for raw_path in paths:
                package_dir = Path(str(raw_path))
                package_json_path = package_dir / "package.json"
                try:
                    manifest = json.loads(package_json_path.read_text(encoding="utf-8"))
                except (OSError, json.JSONDecodeError) as exc:
                    raise NoticeGenerationError(f"unable to read {package_json_path}: {exc}") from exc
                name = str(manifest.get("name") or entry.get("name") or "").strip()
                version = str(manifest.get("version") or "").strip()
                if not name or not version:
                    raise NoticeGenerationError(f"{package_json_path} is missing name or version")
                component_name = f"{name}@{version}"
                manifest_expression = normalize_license_expression(manifest.get("license"), component_name)
                if manifest_expression != expression:
                    raise NoticeGenerationError(
                        f"{component_name} license mismatch: package.json={manifest_expression!r}, pnpm={expression!r}"
                    )
                component = Component(
                    ecosystem=ecosystem,
                    name=name,
                    version=version,
                    license_expression=manifest_expression,
                    notice=license_documents(package_dir, component_name, manifest_expression),
                )
                existing = components.get(component.key)
                if existing is not None and existing != component:
                    raise NoticeGenerationError(f"conflicting license documents for {component_name}")
                components[component.key] = component
    return sorted(components.values(), key=lambda item: item.key)


def collect_node_components(project_name: str) -> list[Component]:
    project_dir = REPO_ROOT / project_name
    output = run_command([*pnpm_command(), "licenses", "list", "--prod", "--json"], project_dir)
    if output.strip() == "No licenses in packages found":
        return []
    try:
        payload = json.loads(output)
    except json.JSONDecodeError as exc:
        raise NoticeGenerationError(f"pnpm returned invalid license JSON for {project_name}: {exc}") from exc
    if not isinstance(payload, dict):
        raise NoticeGenerationError(f"pnpm returned a non-object license report for {project_name}")
    return components_from_pnpm_payload(payload, project_dir, f"npm:{project_name}")


def decode_json_stream(raw: str) -> list[dict[str, Any]]:
    decoder = json.JSONDecoder()
    offset = 0
    values: list[dict[str, Any]] = []
    while offset < len(raw):
        while offset < len(raw) and raw[offset].isspace():
            offset += 1
        if offset >= len(raw):
            break
        value, offset = decoder.raw_decode(raw, offset)
        if not isinstance(value, dict):
            raise NoticeGenerationError("go list returned a non-object module record")
        values.append(value)
    return values


def detect_go_license(text: str, component: str) -> str:
    normalized = text.lower()
    if "apache license" in normalized and "version 2.0" in normalized:
        return "Apache-2.0"
    if "mozilla public license" in normalized and "version 2.0" in normalized:
        return "MPL-2.0"
    if "permission is hereby granted, free of charge" in normalized:
        return "MIT"
    if (
        "permission to use, copy, modify, and/or distribute this software" in normalized
        or "permission to use, copy, modify, and distribute this software" in normalized
    ):
        return "ISC"
    if "redistribution and use in source and binary forms" in normalized:
        return "BSD-3-Clause" if "neither the name" in normalized else "BSD-2-Clause"
    if "this software is provided 'as-is'" in normalized and "altered source versions must be plainly marked" in normalized:
        return "Zlib"
    raise NoticeGenerationError(f"{component} has an unrecognized Go module license")


def collect_go_components() -> list[Component]:
    projects = [
        (REPO_ROOT / "server", "./cmd/raylea-server"),
        *((REPO_ROOT / path, ".") for path in (
            "plugins/builtin/echo",
            "plugins/builtin/fortune",
            "plugins/builtin/game_guide",
            "plugins/builtin/subscription_hub",
        )),
    ]
    modules_by_key: dict[tuple[str, str], dict[str, Any]] = {}
    for project_dir, package_pattern in projects:
        for goos, goarch in (("windows", "amd64"), ("linux", "amd64"), ("darwin", "arm64")):
            target_env = dict(os.environ)
            target_env.update({"GOOS": goos, "GOARCH": goarch, "CGO_ENABLED": "0", "GOWORK": "off"})
            packages = decode_json_stream(
                run_command(["go", "list", "-deps", "-json", package_pattern], project_dir, target_env)
            )
            for package in packages:
                module = package.get("Module")
                if not isinstance(module, dict) or module.get("Main"):
                    continue
                name = str(module.get("Path") or "").strip()
                version = str(module.get("Version") or "").strip()
                if name.startswith("github.com/RayleaBot/RayleaBot/"):
                    continue
                if name and version:
                    modules_by_key[(name, version)] = module

    components: list[Component] = []
    for module in modules_by_key.values():
        effective = module.get("Replace") if isinstance(module.get("Replace"), dict) else module
        name = str(module.get("Path") or "").strip()
        version = str(effective.get("Version") or module.get("Version") or "").strip()
        raw_dir = str(effective.get("Dir") or "").strip()
        module_dir = Path(raw_dir) if raw_dir else None
        if not name or not version or module_dir is None or not module_dir.is_dir():
            raise NoticeGenerationError(f"Go module {name or '<unknown>'} is missing version or module directory")
        component_name = f"{name}@{version}"
        notice = license_documents(module_dir, component_name)
        components.append(
            Component(
                ecosystem="go",
                name=name,
                version=version,
                license_expression=detect_go_license(notice, component_name),
                notice=notice,
            )
        )
    return sorted(components, key=lambda item: item.key)


def collect_electron_runtime_component() -> Component:
    package_dir = REPO_ROOT / "launcher" / "node_modules" / "electron"
    manifest_path = package_dir / "package.json"
    try:
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise NoticeGenerationError(f"unable to read Electron package metadata: {exc}") from exc
    name = str(manifest.get("name") or "electron")
    version = str(manifest.get("version") or "").strip()
    expression = normalize_license_expression(manifest.get("license"), f"{name}@{version}")
    chromium_notices = package_dir / "dist" / "LICENSES.chromium.html"
    if not chromium_notices.is_file() or chromium_notices.stat().st_size == 0:
        raise NoticeGenerationError("Electron runtime is missing LICENSES.chromium.html")
    notice = license_documents(package_dir, f"{name}@{version}")
    notice += (
        "\n\n[Runtime notices]\n"
        "The packaged Electron runtime ships LICENSES.chromium.html beside the executable. "
        "That file contains the Chromium and bundled runtime notices."
    )
    return Component("electron-runtime", name, version, expression, notice)


def merge_components(groups: list[list[Component]]) -> list[Component]:
    merged: dict[tuple[str, str, str], Component] = {}
    for group in groups:
        for component in group:
            existing = merged.get(component.key)
            if existing is not None and existing != component:
                raise NoticeGenerationError(f"conflicting notice records for {component.name}@{component.version}")
            merged[component.key] = component
    return sorted(merged.values(), key=lambda item: item.key)


def render_notices(components: list[Component]) -> str:
    if not components:
        raise NoticeGenerationError("no third-party components were discovered")
    components = sorted(components, key=lambda item: item.key)
    lines = [
        "# Third-Party Notices",
        "",
        "This file is generated from the locked production dependency graphs by",
        "`python scripts/release/generate_third_party_notices.py`.",
        "",
        "RayleaBot is licensed under AGPL-3.0-only. The components below retain their own licenses.",
        "",
        "## Component index",
        "",
        "| Ecosystem | Component | Version | License |",
        "| --- | --- | --- | --- |",
    ]
    for component in components:
        name = component.name.replace("|", "\\|")
        expression = component.license_expression.replace("|", "\\|")
        lines.append(f"| {component.ecosystem} | {name} | {component.version} | {expression} |")

    notice_groups: dict[tuple[str, str], list[Component]] = {}
    for component in components:
        digest = hashlib.sha256(component.notice.encode("utf-8")).hexdigest()
        notice_groups.setdefault((component.license_expression, digest), []).append(component)

    lines.extend(["", "## License texts and notices", ""])
    for (expression, digest), group in sorted(notice_groups.items(), key=lambda item: (item[0][0], item[0][1])):
        names = ", ".join(f"{component.name}@{component.version}" for component in group)
        lines.extend([f"### {expression} ({digest[:12]})", "", f"Applies to: {names}", ""])
        for raw_line in group[0].notice.splitlines():
            lines.append(f"    {raw_line}" if raw_line else "")
        lines.append("")
    return "\n".join(lines).rstrip() + "\n"


def generate() -> str:
    components = merge_components(
        [
            collect_go_components(),
            collect_node_components("web"),
            collect_node_components("launcher"),
            collect_node_components("sdk/vue"),
            collect_node_components("plugins/builtin/fortune/ui"),
            collect_node_components("plugins/builtin/subscription_hub/ui"),
            [collect_electron_runtime_component()],
        ]
    )
    return render_notices(components)


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, default=DEFAULT_OUTPUT)
    parser.add_argument("--check", action="store_true", help="Fail when the tracked notice is missing or stale.")
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(sys.argv[1:] if argv is None else argv)
    try:
        generated = generate()
        output = args.output.resolve()
        if args.check:
            current = output.read_text(encoding="utf-8")
            if current.replace("\r\n", "\n") != generated:
                raise NoticeGenerationError(
                    f"{output} is stale; run python scripts/release/generate_third_party_notices.py"
                )
        else:
            output.parent.mkdir(parents=True, exist_ok=True)
            output.write_text(generated, encoding="utf-8", newline="\n")
    except (NoticeGenerationError, OSError) as exc:
        print(f"third-party notice generation failed: {exc}", file=sys.stderr)
        return 1
    print(f"third-party notices {'verified' if args.check else 'written'}: {args.output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
