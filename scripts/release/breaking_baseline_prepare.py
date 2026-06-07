#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import zipfile
from datetime import datetime, timezone
from pathlib import Path


BACKUP_DIRS = (
    "config",
    "data",
    "plugins/installed",
)
OPTIONAL_FILES = (
    "logs/recovery-summary.json",
    "build_info.json",
)


def utc_now_iso() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def normalize_relative(path: Path) -> str:
    return path.as_posix().strip("/")


def should_skip(path: Path) -> bool:
    return any(part in {"__pycache__", ".pytest_cache"} for part in path.parts) or path.suffix in {".pyc", ".pyo"}


def collect_backup_entries(root: Path) -> list[Path]:
    entries: list[Path] = []
    for relative_dir in BACKUP_DIRS:
        directory = root / relative_dir
        if not directory.exists():
            continue
        for item in sorted(directory.rglob("*")):
            if item.is_file():
                relative = item.relative_to(root)
                if not should_skip(relative):
                    entries.append(relative)
    for relative_file in OPTIONAL_FILES:
        file_path = root / relative_file
        if file_path.is_file():
            entries.append(Path(relative_file))
    return sorted(set(entries), key=lambda item: item.as_posix())


def write_backup(root: Path, output: Path, *, created_at: str | None = None) -> Path:
    root = root.resolve()
    if not root.is_dir():
        raise ValueError(f"install root does not exist: {root}")
    created_at = created_at or utc_now_iso()
    output.parent.mkdir(parents=True, exist_ok=True)

    entries = collect_backup_entries(root)
    manifest = {
        "kind": "breaking-baseline-backup",
        "created_at": created_at,
        "source_root": str(root),
        "included_paths": [normalize_relative(entry) for entry in entries],
        "rollback": {
            "stop_service": True,
            "restore_directories": list(BACKUP_DIRS),
            "restore_optional_files": list(OPTIONAL_FILES),
            "start_original_package": True,
        },
    }

    with zipfile.ZipFile(output, "w", compression=zipfile.ZIP_DEFLATED) as archive:
        archive.writestr("breaking-baseline-backup.json", json.dumps(manifest, ensure_ascii=False, indent=2) + "\n")
        for relative in entries:
            archive.write(root / relative, normalize_relative(relative))
    return output


def main() -> int:
    parser = argparse.ArgumentParser(description="Prepare a backup before installing a breaking RayleaBot baseline.")
    parser.add_argument("--root", required=True, type=Path, help="RayleaBot install root to back up.")
    parser.add_argument("--output", required=True, type=Path, help="Backup zip path to create.")
    args = parser.parse_args()

    backup_path = write_backup(args.root, args.output)
    print(backup_path)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
