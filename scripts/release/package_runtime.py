#!/usr/bin/env python3
from __future__ import annotations

import hashlib
import json
import os
import sys
import shutil
import tarfile
import time
import zipfile
from pathlib import Path, PurePosixPath


sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from deps_manifest import validate_manifest
from archive_io import extract_archive
from jsonschema import ValidationError

# REQUIRED_PATHS is re-exported for smoke_release.
from artifact_matrix import ARTIFACT_MATRIX, REQUIRED_PATHS
from release_content import find_forbidden_paths


RESOURCE_KINDS = ("chromium", "ffmpeg")


def archive_root_name(names: list[str]) -> str:
    roots: set[str] = set()
    seen: set[str] = set()
    for name in names:
        normalized = name.replace("\\", "/").rstrip("/")
        path = PurePosixPath(normalized)
        if not normalized or path.is_absolute() or any(part in {"", ".", ".."} or ":" in part for part in normalized.split("/")):
            raise RuntimeError(f"unsafe release archive entry: {name}")
        if normalized in seen:
            raise RuntimeError(f"duplicate release archive entry: {name}")
        seen.add(normalized)
        roots.add(path.parts[0])
    if len(roots) != 1:
        raise RuntimeError("release archive must contain exactly one root directory")
    return next(iter(roots))


def unpack_archive(artifact_id: str, archive_path: Path, destination: Path) -> Path:
    destination.mkdir(parents=True, exist_ok=True)
    if ARTIFACT_MATRIX[artifact_id]["archive_type"] == "zip":
        with zipfile.ZipFile(archive_path) as zf:
            names = [name for name in zf.namelist() if name]
            root_name = archive_root_name(names)

    else:
        with tarfile.open(archive_path, "r:gz") as tf:
            names = [member.name for member in tf.getmembers() if member.name]
            root_name = archive_root_name(names)

    extract_archive(archive_path, destination, allow_links=True)
    root = destination / root_name
    if not root.is_dir():
        raise RuntimeError(f"release root not found after extraction: {root}")
    return compact_release_root(root, destination)


def compact_release_root(root: Path, destination: Path, platform_name: str | None = None) -> Path:
    if (platform_name or os.name) != "nt":
        return root
    digest = hashlib.sha256(str(destination.resolve()).encode("utf-8")).hexdigest()[:8]
    compact = destination.parent / f"r-{digest}"
    if compact.exists():
        shutil.rmtree(compact)
    replace_directory_with_retry(root, compact)
    return compact


def ensure_no_forbidden_paths(root: Path) -> None:
    forbidden = find_forbidden_paths(root)
    if forbidden:
        raise RuntimeError(f"packaged archive contains development files: {forbidden}")


def artifact_platform(artifact_id: str) -> str:
    parts = artifact_id.split("-")
    if len(parts) < 2:
        raise RuntimeError(f"invalid artifact id: {artifact_id}")
    return "-".join(parts[:2])


def load_deps_manifest(root: Path) -> dict[str, object]:
    manifest_path = root / ".deps" / "manifest.json"
    payload = json.loads(manifest_path.read_text(encoding="utf-8"))
    validate_manifest(payload)
    return payload


def find_platform_resource(manifest: dict[str, object], platform: str, kind: str) -> dict[str, object]:
    resources = manifest.get("resources")
    if not isinstance(resources, list):
        raise RuntimeError("deps manifest resources must be a list")
    for item in resources:
        if isinstance(item, dict) and item.get("platform") == platform and item.get("kind") == kind:
            return item
    raise RuntimeError(f"deps manifest missing {kind} for {platform}")


def resource_has_complete_metadata(resource: dict[str, object]) -> bool:
    try:
        validate_manifest({"manifest_version": 5, "resources": [resource]})
    except (ValueError, TypeError, ValidationError):
        return False
    return True


def replace_directory_with_retry(source: Path, target: Path, *, timeout_seconds: float = 5) -> None:
    deadline = time.monotonic() + max(timeout_seconds, 0)
    while True:
        try:
            source.replace(target)
            return
        except PermissionError:
            if time.monotonic() >= deadline:
                raise
            time.sleep(0.2)
