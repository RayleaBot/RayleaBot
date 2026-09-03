#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def validate_release_notes(tag: str, notes_dir: Path) -> Path:
    schema = json.loads((ROOT / "contracts/release-manifest.schema.json").read_text(encoding="utf-8"))
    version_pattern = schema["$defs"]["semver"]["pattern"]
    if not tag.startswith("v") or not re.fullmatch(version_pattern, tag[1:]):
        raise ValueError("release tag must be v followed by a version accepted by the release contract")

    path = notes_dir / f"{tag}.md"
    body = path.read_text(encoding="utf-8-sig")
    placeholder = re.search(r"\{\{.*?\}\}", body, flags=re.DOTALL)
    if placeholder:
        line = body.count("\n", 0, placeholder.start()) + 1
        raise ValueError(f"{path}:{line}: unresolved release notes placeholder")

    visible = re.sub(r"<!--.*?(?:-->|$)", "", body, flags=re.DOTALL)
    content_lines = [
        line for line in visible.splitlines()
        if line.strip() and not re.fullmatch(r"\s*(?:#{1,6}(?:\s.*)?|[-*_]{3,})\s*", line)
    ]
    if not content_lines:
        raise ValueError(f"{path}: release notes have no body content")
    return path


def main() -> int:
    parser = argparse.ArgumentParser(description="Check the release body before building release artifacts.")
    parser.add_argument("--tag", required=True)
    parser.add_argument("--notes-dir", type=Path, default=ROOT / "docs/release/notes")
    args = parser.parse_args()
    try:
        path = validate_release_notes(args.tag, args.notes_dir)
    except (OSError, UnicodeError, ValueError) as exc:
        print(f"release notes check failed: {exc}", file=sys.stderr)
        return 1
    print(f"release notes checked: {path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
