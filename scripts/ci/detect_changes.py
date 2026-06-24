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
    "docs_only",
    "ci",
)

DOC_ROOT_FILES = {"AGENTS.md", "CLAUDE.md", "README.md", "PRODUCT.md", "LICENSE"}


def normalize_path(path: str) -> str:
    normalized = path.replace("\\", "/").strip()
    while normalized.startswith("./"):
        normalized = normalized[2:]
    return normalized


def run_git(args: list[str]) -> list[str]:
    completed = subprocess.run(
        ["git", *args],
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
    if path.endswith("/AGENTS.md") or path.endswith("/CLAUDE.md") or path.endswith("/README.md"):
        return True
    return False


def classify(files: list[str]) -> dict[str, bool]:
    paths = [normalize_path(path) for path in files if normalize_path(path)]
    result = {key: False for key in OUTPUT_KEYS}

    for path in paths:
        if path.startswith("server/") or path.startswith("config/") or path.startswith("templates/"):
            result["server"] = True
        if path.startswith("plugins/builtin/") or path == "scripts/generate-runtime-schemas.mjs":
            result["server"] = True
        if path.startswith("web/"):
            result["web"] = True
        if path.startswith("launcher/"):
            result["launcher"] = True
        if path.startswith("sdk/"):
            result["sdk"] = True
        if path.startswith("contracts/") or path.startswith("fixtures/") or path.startswith("examples/"):
            result["contracts"] = True
        if path.startswith("scripts/release/") or path.startswith("packaging/") or path.startswith(".deps/"):
            result["release"] = True
        if path in {".github/workflows/release.yml", ".github/workflows/self-host-smoke.yml"}:
            result["release"] = True
        if path.startswith(".github/workflows/") or path.startswith("scripts/ci/"):
            result["ci"] = True
        if path == "scripts/check-agent-docs.mjs":
            result["ci"] = True

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
        (["docs/test.md"], {"docs_only": True}),
        (["server/internal/app/app.go"], {"server": True, "docs_only": False}),
        (["contracts/web-api.openapi.yaml"], {"contracts": True}),
        (["scripts/release/release_tool.py"], {"release": True}),
        ([".github/workflows/ci.yml"], {"ci": True, "docs_only": False}),
        (["AGENTS.md"], {"docs_only": True}),
    ]
    for files, expected in cases:
        result = classify(files)
        for key, value in expected.items():
            if result[key] != value:
                raise AssertionError(f"{files}: expected {key}={value}, got {result[key]}")
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
    write_outputs(classify(files), args.github_output)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
