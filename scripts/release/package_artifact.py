#!/usr/bin/env python3
"""Package a RayleaBot release artifact and run optional smoke checks."""

from __future__ import annotations

import argparse
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def run(args: list[str]) -> None:
    print("+ " + " ".join(args))
    subprocess.run(args, cwd=ROOT, check=True)


def archive_suffix(artifact_id: str) -> str:
    return ".zip" if artifact_id == "windows-x64-full" else ".tar.gz"


def archive_path(output_dir: Path, version: str, artifact_id: str) -> Path:
    return output_dir / f"RayleaBot-v{version}-{artifact_id}{archive_suffix(artifact_id)}"


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--artifact-id", required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--git-commit", required=True)
    parser.add_argument("--release-notes-ref", required=True)
    parser.add_argument("--server-bin", required=True)
    parser.add_argument("--web-dist", default="web/dist")
    parser.add_argument("--builtin-dir", default="plugins/builtin")
    parser.add_argument("--deps-dir", default=".deps")
    parser.add_argument("--templates-dir", default="templates")
    parser.add_argument("--default-config", default="config/default.yaml")
    parser.add_argument("--output-dir", default="dist/release")
    parser.add_argument("--launcher-bundle", default="")
    parser.add_argument("--systemd-file", default="")
    parser.add_argument("--repository", default="")
    parser.add_argument("--run-smoke", action="store_true")
    parser.add_argument("--run-recovery-drill", action="store_true")
    parser.add_argument("--run-self-host-smoke", action="store_true")
    parser.add_argument("--recovery-download-dir", "--download-dir", dest="recovery_download_dir", default="")
    parser.add_argument("--observation-window-seconds", default="")
    parser.add_argument("--window-seconds", default="")
    parser.add_argument("--probe-interval-seconds", default="")
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv or sys.argv[1:])
    output_dir = Path(args.output_dir)

    package_args = [
        sys.executable,
        "scripts/release/release_tool.py",
        "package",
        "--artifact-id",
        args.artifact_id,
        "--version",
        args.version,
        "--git-commit",
        args.git_commit,
        "--release-notes-ref",
        args.release_notes_ref,
        "--server-bin",
        args.server_bin,
        "--web-dist",
        args.web_dist,
        "--builtin-dir",
        args.builtin_dir,
        "--deps-dir",
        args.deps_dir,
        "--templates-dir",
        args.templates_dir,
        "--default-config",
        args.default_config,
        "--output-dir",
        args.output_dir,
    ]
    if args.launcher_bundle:
        package_args.extend(["--launcher-bundle", args.launcher_bundle])
    if args.systemd_file:
        package_args.extend(["--systemd-file", args.systemd_file])
    run(package_args)

    archive = archive_path(output_dir, args.version, args.artifact_id)
    if args.run_smoke:
        run(
            [
                sys.executable,
                "scripts/release/smoke_release.py",
                "--artifact-id",
                args.artifact_id,
                "--archive",
                str(archive),
            ]
        )
    if args.run_recovery_drill:
        recovery_args = [
            sys.executable,
            "scripts/release/recovery_drill.py",
            "--artifact-id",
            args.artifact_id,
            "--archive",
            str(archive),
        ]
        if args.repository:
            recovery_args.extend(["--repository", args.repository])
        if args.version:
            recovery_args.extend(["--current-version", args.version])
        if args.recovery_download_dir:
            recovery_args.extend(["--download-dir", args.recovery_download_dir])
        if args.observation_window_seconds:
            recovery_args.extend(["--observation-window-seconds", args.observation_window_seconds])
        run(recovery_args)
    if args.run_self_host_smoke:
        smoke_args = [
            sys.executable,
            "scripts/release/self_host_smoke.py",
            "--artifact-id",
            args.artifact_id,
            "--archive",
            str(archive),
        ]
        if args.window_seconds:
            smoke_args.extend(["--window-seconds", args.window_seconds])
        if args.probe_interval_seconds:
            smoke_args.extend(["--probe-interval-seconds", args.probe_interval_seconds])
        run(smoke_args)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
