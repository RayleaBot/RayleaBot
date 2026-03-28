#!/usr/bin/env python3
from __future__ import annotations

import argparse
import tarfile
import zipfile
from pathlib import Path


EXPECTED = {
    "windows-x64-full": {
        "archive_type": "zip",
        "entries": {
            "raylea-server.exe",
            "launcher/RayleaLauncher.exe",
            "build_info.json",
            "config/default.yaml",
            "web/index.html",
            ".deps/manifest.json",
        },
    },
    "linux-x64-full": {
        "archive_type": "tar.gz",
        "entries": {
            "raylea-server",
            "launcher/RayleaLauncher",
            "build_info.json",
            "config/default.yaml",
            "web/index.html",
            ".deps/manifest.json",
        },
    },
    "macos-arm64-full": {
        "archive_type": "tar.gz",
        "entries": {
            "raylea-server",
            "launcher/RayleaLauncher.app/Contents/MacOS/RayleaLauncher",
            "build_info.json",
            "config/default.yaml",
            "web/index.html",
            ".deps/manifest.json",
        },
    },
    "linux-x64-server": {
        "archive_type": "tar.gz",
        "entries": {
            "raylea-server",
            "build_info.json",
            "config/default.yaml",
            "systemd/rayleabot.service",
            "web/index.html",
            ".deps/manifest.json",
        },
    },
}


def list_entries(artifact_id: str, archive_path: Path) -> set[str]:
    expected = EXPECTED[artifact_id]
    if expected["archive_type"] == "zip":
        with zipfile.ZipFile(archive_path) as zf:
            names = [name for name in zf.namelist() if not name.endswith("/")]
    else:
        with tarfile.open(archive_path, "r:gz") as tf:
            names = [member.name for member in tf.getmembers() if member.isfile()]

    root_prefix = Path(names[0]).parts[0]
    result: set[str] = set()
    for name in names:
        relative = Path(name).relative_to(root_prefix)
        result.add(relative.as_posix())
    return result


def main() -> int:
    parser = argparse.ArgumentParser(description="RayleaBot packaged artifact smoke check")
    parser.add_argument("--artifact-id", required=True, choices=sorted(EXPECTED.keys()))
    parser.add_argument("--archive", required=True)
    args = parser.parse_args()

    entries = list_entries(args.artifact_id, Path(args.archive))
    missing = sorted(EXPECTED[args.artifact_id]["entries"] - entries)
    if missing:
        raise SystemExit(f"missing packaged entries: {missing}")
    print("release smoke passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
