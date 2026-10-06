"""Resolve release channel and GitHub publication settings from a validated tag."""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import sys

from check_release_notes import ROOT, validate_release_notes, validate_release_tag


def release_channel(version: str, requested: str | None = None) -> str:
    validate_release_tag("v" + version)
    channel = "beta" if "-" in version.split("+", 1)[0] else "stable"
    if requested is not None and requested != channel:
        raise ValueError(f"version {version} requires channel={channel}, not {requested}")
    return channel


def publication_settings(tag: str) -> dict[str, str]:
    version = validate_release_tag(tag)
    channel = release_channel(version)
    return {
        "value": version,
        "channel": channel,
        "prerelease": "true" if channel == "beta" else "false",
        "make_latest": "false" if channel == "beta" else "legacy",
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--tag", required=True)
    parser.add_argument("--channel", choices=["stable", "beta"])
    parser.add_argument("--notes-dir", type=Path, default=ROOT / "docs/release/notes")
    parser.add_argument("--github-output", type=Path)
    args = parser.parse_args()
    try:
        settings = publication_settings(args.tag)
        release_channel(settings["value"], args.channel)
        validate_release_notes(args.tag, args.notes_dir)
        output = args.github_output or (Path(os.environ["GITHUB_OUTPUT"]) if os.environ.get("GITHUB_OUTPUT") else None)
        if output:
            with output.open("a", encoding="utf-8") as stream:
                stream.writelines(f"{key}={value}\n" for key, value in settings.items())
        print(json.dumps(settings))
    except (OSError, UnicodeError, ValueError) as exc:
        print(f"release policy check failed: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
