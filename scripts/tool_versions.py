"""Read the repository's fixed toolchain versions without importing a doctor."""
from pathlib import Path
import re

TOOLS = ("golang", "nodejs", "python", "pnpm", "npm", "corepack", "sqlc")

def read_tool_versions(root: Path) -> dict[str, str]:
    versions: dict[str, str] = {}
    for line in (root / ".tool-versions").read_text(encoding="utf-8").splitlines():
        fields = line.split("#", 1)[0].split()
        if not fields:
            continue
        if len(fields) != 2 or fields[0] in versions or not re.fullmatch(r"[a-z]+", fields[0]) or not re.fullmatch(r"\d+\.\d+\.\d+", fields[1]):
            raise ValueError(".tool-versions requires one fixed version per tool")
        versions[fields[0]] = fields[1]
    for name in TOOLS:
        if not re.fullmatch(r"\d+\.\d+\.\d+", versions.get(name, "")):
            raise ValueError(f".tool-versions must pin {name} to an exact version")
    return versions
