"""Shared package content rules; runtime documentation is copied explicitly."""
import fnmatch
from pathlib import Path

FORBIDDEN_TOP_LEVEL_PATHS = {
    ".github",
    "contracts",
    "docs",
    "examples",
    "fixtures",
    "launcher/src",
    "plugins",
    "scripts",
    "sdk",
    "server",
    "web/src",
}
FORBIDDEN_DIRECTORY_NAMES = {
    ".cache",
    ".git",
    ".pytest_cache",
    ".venv",
    "__pycache__",
    "node_modules",
    "test",
    "tests",
    "venv",
}
FORBIDDEN_FILE_PATTERNS = (
    "*.go",
    "*.map",
    "*.py",
    "*.pyc",
    "*.pyo",
    "*.spec.*",
    "*.test.*",
    "*.ts",
    "*.tsx",
    "*.vue",
    "*_test.*",
    "go.mod",
    "go.sum",
    "package.json",
    "pnpm-lock.yaml",
    "pnpm-workspace.yaml",
)


def normalize_relative(path: Path) -> str:
    return path.as_posix().strip("/")


def is_forbidden_file_name(name: str) -> bool:
    return any(fnmatch.fnmatchcase(name, pattern) for pattern in FORBIDDEN_FILE_PATTERNS)


def find_forbidden_paths(root: Path) -> list[str]:
    forbidden: list[str] = []
    for item in sorted(root.rglob("*")):
        relative = item.relative_to(root)
        normalized = normalize_relative(relative)
        parts = relative.parts
        if any(normalized == path or normalized.startswith(path + "/") for path in FORBIDDEN_TOP_LEVEL_PATHS):
            forbidden.append(normalized)
            continue
        if any(part in FORBIDDEN_DIRECTORY_NAMES for part in parts):
            forbidden.append(normalized)
            continue
        if item.is_file() and is_forbidden_file_name(item.name):
            forbidden.append(normalized)
    return forbidden


def should_skip_release_path(relative_path: Path) -> bool:
    return any(part in FORBIDDEN_DIRECTORY_NAMES for part in relative_path.parts) or is_forbidden_file_name(relative_path.name) or fnmatch.fnmatchcase(relative_path.name, "*.md")
