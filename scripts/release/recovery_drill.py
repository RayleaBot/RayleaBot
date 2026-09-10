#!/usr/bin/env python3
"""Validate one current release archive through fresh setup and same-format recovery."""
from __future__ import annotations

import argparse
import json
from pathlib import Path
import time

from artifact_matrix import ARTIFACT_MATRIX
from package_runtime import ensure_required_paths, relative_executable, unpack_archive
from rehearse_current_recovery import rehearse


def run_recovery_drill(artifact_id: str, archive_path: Path, plugin_fixture: Path,
                       *, output_dir: Path, observation_window_seconds: float = 0) -> dict:
    if observation_window_seconds < 0:
        raise ValueError("observation window must not be negative")
    archive_path = archive_path.resolve(strict=True)
    plugin_fixture = plugin_fixture.resolve(strict=True)
    output_dir.mkdir(parents=True, exist_ok=False)
    release_root = unpack_archive(artifact_id, archive_path, output_dir / "package")
    ensure_required_paths(release_root, artifact_id)
    result = rehearse(relative_executable(release_root, artifact_id), output_dir / "rehearsal",
                      distribution_root=release_root, plugin_fixture=plugin_fixture,
                      observation_window_seconds=observation_window_seconds)
    result = {"artifact_id": artifact_id, "archive": str(archive_path), "recovery": result}
    (output_dir / "result.json").write_text(json.dumps(result, indent=2), encoding="utf-8")
    return result


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--artifact-id", required=True, choices=sorted(ARTIFACT_MATRIX))
    parser.add_argument("--archive", required=True, type=Path)
    parser.add_argument("--plugin-fixture", required=True, type=Path)
    parser.add_argument("--output-dir", type=Path)
    parser.add_argument("--observation-window-seconds", type=float, default=0)
    return parser


def main() -> int:
    args = build_parser().parse_args()
    output = args.output_dir or args.archive.parent / f"recovery-{args.artifact_id}-{time.time_ns()}"
    try:
        result = run_recovery_drill(args.artifact_id, args.archive, args.plugin_fixture,
                                    output_dir=output.resolve(), observation_window_seconds=args.observation_window_seconds)
    except (OSError, ValueError, RuntimeError, AssertionError) as exc:
        print(f"current-format recovery drill failed: {exc}")
        return 1
    print(json.dumps(result, indent=2))
    print(f"Recovery evidence: {output.resolve()}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
