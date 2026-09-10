#!/usr/bin/env python3
from __future__ import annotations

import argparse
import tarfile
import tempfile
import zipfile
from pathlib import Path

from package_runtime import (
    RESOURCE_KINDS,
    REQUIRED_PATHS,
    artifact_platform,
    archive_root_name,
    ensure_no_forbidden_paths,
    find_platform_resource,
    load_deps_manifest,
    resource_has_complete_metadata,
    unpack_archive,
)


from artifact_matrix import ARTIFACT_MATRIX

EXPECTED = {
    artifact_id: {"archive_type": definition["archive_type"], "entries": REQUIRED_PATHS[artifact_id]}
    for artifact_id, definition in ARTIFACT_MATRIX.items()
}


def list_entries(artifact_id: str, archive_path: Path) -> set[str]:
    expected = EXPECTED[artifact_id]
    if expected["archive_type"] == "zip":
        with zipfile.ZipFile(archive_path) as zf:
            names = [name for name in zf.namelist() if not name.endswith("/")]
    else:
        with tarfile.open(archive_path, "r:gz") as tf:
            names = [member.name for member in tf.getmembers() if member.isfile()]

    root_prefix = archive_root_name(names)
    result: set[str] = set()
    for name in names:
        relative = Path(name).relative_to(root_prefix)
        result.add(relative.as_posix())
    return result


def validate_runtime_bootstrap_prerequisites(artifact_id: str, archive_path: Path) -> None:
    with tempfile.TemporaryDirectory(prefix="rayleabot-release-smoke-") as tmp:
        root = unpack_archive(artifact_id, archive_path, Path(tmp))
        ensure_no_forbidden_paths(root)
        manifest = load_deps_manifest(root)
        platform = artifact_platform(artifact_id)
        for kind in RESOURCE_KINDS:
            resource = find_platform_resource(manifest, platform, kind)
            if not resource_has_complete_metadata(resource):
                raise RuntimeError(f"packaged deps manifest resource is not bootstrap-ready: {resource}")


def main() -> int:
    parser = argparse.ArgumentParser(description="RayleaBot packaged artifact smoke check")
    parser.add_argument("--artifact-id", required=True, choices=sorted(EXPECTED.keys()))
    parser.add_argument("--archive", required=True)
    args = parser.parse_args()

    entries = list_entries(args.artifact_id, Path(args.archive))
    missing = sorted(EXPECTED[args.artifact_id]["entries"] - entries)
    if missing:
        raise SystemExit(f"missing packaged entries: {missing}")
    validate_runtime_bootstrap_prerequisites(args.artifact_id, Path(args.archive))
    print("release smoke passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
