#!/usr/bin/env python3
"""Detect changed repository areas for GitHub Actions."""

from __future__ import annotations

import argparse
import os
import subprocess
import sys
from pathlib import Path


OUTPUT_KEYS = (
    "server",
    "web",
    "launcher",
    "sdk",
    "contracts",
    "release",
    "docs",
    "docs_only",
    "ci",
)

DOC_ROOT_FILES = {"AGENTS.md", "CLAUDE.md", "README.md", "PRODUCT.md", ".impeccable.md"}
TOOLCHAIN_ROOT_FILES = {".gitignore", ".tool-versions", "Makefile", "start.bat", "start.sh"}


def normalize_path(path: str) -> str:
    normalized = path.replace("\\", "/").strip()
    while normalized.startswith("./"):
        normalized = normalized[2:]
    return normalized


def run_git(args: list[str]) -> list[str]:
    completed = subprocess.run(
        ["git", "-c", "core.quotepath=false", *args],
        check=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    return [normalize_path(line) for line in completed.stdout.splitlines() if line.strip()]


def diff_files(base: str | None, head: str | None) -> list[str]:
    if base and set(base) == {"0"}:
        base = None

    candidates: list[list[str]] = []
    if base and head:
        candidates.append(["diff", "--name-only", f"{base}...{head}"])
        candidates.append(["diff", "--name-only", base, head])
    if head:
        candidates.append(["diff-tree", "--no-commit-id", "--name-only", "-r", head])
    candidates.append(["diff", "--name-only", "HEAD~1", "HEAD"])

    for args in candidates:
        try:
            files = run_git(args)
        except (subprocess.CalledProcessError, FileNotFoundError):
            continue
        if files:
            return files
    return []


def is_docs_path(path: str) -> bool:
    if path in DOC_ROOT_FILES:
        return True
    if path.startswith("docs/"):
        return True
    if path.startswith(".agents/"):
        return True
    if path.endswith("/AGENTS.md") or path.endswith("/CLAUDE.md") or path.endswith("/README.md"):
        return True
    return False


def classify(files: list[str]) -> dict[str, bool]:
    paths = [normalize_path(path) for path in files if normalize_path(path)]
    result = {key: False for key in OUTPUT_KEYS}
    unclassified: list[str] = []

    for path in paths:
        matched = False

        if path.startswith("server/"):
            result["server"] = True
            matched = True
        if path.startswith("config/") or path.startswith("templates/"):
            result["server"] = True
            result["release"] = True
            matched = True
        if path.startswith("plugins/"):
            result["server"] = True
            result["release"] = True
            matched = True
        if path.startswith("web/"):
            result["web"] = True
            matched = True
        if path.startswith("launcher/"):
            result["launcher"] = True
            matched = True
        if path.startswith("sdk/"):
            result["sdk"] = True
            matched = True
        if path.startswith("contracts/") or path.startswith("fixtures/") or path.startswith("examples/"):
            result["contracts"] = True
            matched = True
        if path.startswith("scripts/release/") or path.startswith("packaging/") or path.startswith(".deps/"):
            result["release"] = True
            matched = True
        if path.startswith(".deps/"):
            result["server"] = True
        if path in {".github/workflows/release.yml", ".github/workflows/self-host-smoke.yml"}:
            result["release"] = True
            matched = True
        if path.startswith(".github/workflows/") or path.startswith("scripts/ci/"):
            result["ci"] = True
            matched = True
        if path.startswith(".github/"):
            result["ci"] = True
            matched = True
        if path == "scripts/generate-runtime-schemas.mjs":
            result["server"] = True
            result["contracts"] = True
            result["ci"] = True
            matched = True
        if path.startswith("scripts/") and not path.startswith("scripts/release/"):
            result["ci"] = True
            matched = True
        if path in {
            "scripts/check-toolchain.py",
            "scripts/gbash.cmd",
            "scripts/gbash.ps1",
            "scripts/start-dev.mjs",
            "scripts/start-dev-support.mjs",
        } or path.startswith("scripts/tests/"):
            result["server"] = True
            result["web"] = True
            result["launcher"] = True
        if path == "scripts/check-server-structure.py":
            result["server"] = True
        if path.startswith(".devcontainer/") or path in TOOLCHAIN_ROOT_FILES:
            result["server"] = True
            result["web"] = True
            result["launcher"] = True
            result["ci"] = True
            matched = True
        if path == "LICENSE":
            result["sdk"] = True
            result["release"] = True
            matched = True
        if path == "THIRD_PARTY_NOTICES.md":
            result["release"] = True
            matched = True
        if is_docs_path(path):
            result["docs"] = True
            matched = True
        if (
            path.startswith(".agents/")
            or path.endswith("/AGENTS.md")
            or path.endswith("/CLAUDE.md")
            or path in {"AGENTS.md", "CLAUDE.md"}
        ):
            result["ci"] = True

        if not matched:
            unclassified.append(path)

    if unclassified:
        joined = ", ".join(sorted(unclassified))
        raise ValueError(f"unclassified tracked path(s): {joined}")

    result["docs_only"] = bool(paths) and all(is_docs_path(path) for path in paths)
    return result


def write_outputs(result: dict[str, bool], output_file: str | None) -> None:
    lines = [f"{key}={str(result[key]).lower()}" for key in OUTPUT_KEYS]
    for line in lines:
        print(line)
    if output_file:
        with Path(output_file).open("a", encoding="utf-8") as handle:
            for line in lines:
                handle.write(f"{line}\n")


def self_test() -> None:
    cases = [
        (["docs/test.md"], {"docs": True, "docs_only": True}),
        (["server/internal/app/app.go"], {"server": True, "docs_only": False}),
        (["contracts/web-api.openapi.yaml"], {"contracts": True}),
        (["scripts/release/release_tool.py"], {"release": True}),
        ([".github/workflows/ci.yml"], {"ci": True, "docs_only": False}),
        (["AGENTS.md"], {"docs": True, "docs_only": True}),
        (["templates/help.menu/template.json"], {"server": True, "release": True}),
        (["plugins/builtin/fortune/info.json"], {"server": True, "release": True}),
        ([".deps/manifest.json"], {"server": True, "release": True}),
        (["scripts/check-toolchain.py"], {"server": True, "web": True, "launcher": True, "ci": True}),
        (["server/AGENTS.md", "server/internal/app/app.go"], {"server": True, "ci": True}),
    ]
    for files, expected in cases:
        result = classify(files)
        for key, value in expected.items():
            if result[key] != value:
                raise AssertionError(f"{files}: expected {key}={value}, got {result[key]}")

    try:
        classify(["unclassified.future-file"])
    except ValueError:
        pass
    else:
        raise AssertionError("an unclassified path must fail closed")

    classify(run_git(["ls-files"]))
    print("detect_changes self-test passed")


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", default=os.environ.get("GITHUB_BASE_SHA"))
    parser.add_argument("--head", default=os.environ.get("GITHUB_SHA"))
    parser.add_argument("--files", nargs="*", help="Classify an explicit file list.")
    parser.add_argument("--github-output", default=os.environ.get("GITHUB_OUTPUT"))
    parser.add_argument("--self-test", action="store_true")
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv or sys.argv[1:])
    if args.self_test:
        self_test()
        return 0

    files = [normalize_path(path) for path in args.files] if args.files is not None else diff_files(args.base, args.head)
    print("changed files:")
    if files:
        for path in files:
            print(f"- {path}")
    else:
        print("- <none detected>")
    try:
        result = classify(files)
    except ValueError as exc:
        print(f"change classification failed: {exc}", file=sys.stderr)
        return 2
    write_outputs(result, args.github_output)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
