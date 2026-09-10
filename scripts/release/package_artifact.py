#!/usr/bin/env python3
"""Package a RayleaBot release artifact and run optional smoke checks."""

from __future__ import annotations

import argparse
from contextlib import nullcontext
from datetime import datetime, timezone
import hashlib
import json
import shutil
import tempfile
import time
import subprocess
import sys
from pathlib import Path


from artifact_matrix import ARTIFACT_MATRIX

ROOT = Path(__file__).resolve().parents[2]


def run(args: list[str]) -> None:
    print("+ " + " ".join(args))
    subprocess.run(args, cwd=ROOT, check=True)


class ValidationEvidence:
    """Keep check outcomes and output without retaining synthetic runtime data."""

    def __init__(self, directory: Path, args: argparse.Namespace):
        self.directory = directory
        self.directory.mkdir(parents=True, exist_ok=False)
        self.result = {
            "artifact_id": args.artifact_id, "version": args.version,
            "git_commit": args.git_commit, "status": "running", "checks": [],
            "observation_window_seconds": args.observation_window_seconds or "0",
            "window_seconds": args.window_seconds or "600",
            "probe_interval_seconds": args.probe_interval_seconds or "30",
        }
        self.save()

    def __enter__(self):
        return self

    def __exit__(self, exception_type, exception, traceback):
        self.result["status"] = "passed" if exception is None else "failed"
        self.result["finished_at"] = datetime.now(timezone.utc).isoformat()
        self.save()

    def save(self) -> None:
        (self.directory / "validation.json").write_text(json.dumps(self.result, indent=2) + "\n", encoding="utf-8")

    def record_archive(self, archive: Path) -> None:
        with archive.open("rb") as source:
            digest = hashlib.file_digest(source, "sha256").hexdigest()
        self.result["archive"] = {"file_name": archive.name, "size_bytes": archive.stat().st_size, "sha256": digest}
        self.save()

    def run(self, name: str, args: list[str]) -> None:
        check = {"name": name, "status": "running", "started_at": datetime.now(timezone.utc).isoformat(), "exit_code": None}
        self.result["checks"].append(check)
        self.save()
        started = time.monotonic()
        print("+ " + " ".join(args), flush=True)
        try:
            with (self.directory / (name + ".log")).open("w", encoding="utf-8") as log:
                with subprocess.Popen(args, cwd=ROOT, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                                      text=True, encoding="utf-8", errors="replace") as process:
                    for line in process.stdout:
                        print(line, end="", flush=True)
                        log.write(line)
                        log.flush()
                    check["exit_code"] = process.wait()
                    if process.returncode:
                        raise subprocess.CalledProcessError(process.returncode, args)
            check["status"] = "passed"
        except BaseException:
            check["status"] = "failed"
            raise
        finally:
            check["elapsed_seconds"] = round(time.monotonic() - started, 3)
            check["finished_at"] = datetime.now(timezone.utc).isoformat()
            self.save()


def archive_suffix(artifact_id: str) -> str:
    return ARTIFACT_MATRIX[artifact_id]["extension"]


def archive_path(output_dir: Path, version: str, artifact_id: str) -> Path:
    return output_dir / f"RayleaBot-v{version}-{artifact_id}{archive_suffix(artifact_id)}"


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--artifact-id", required=True, choices=sorted(ARTIFACT_MATRIX))
    parser.add_argument("--version", required=True)
    parser.add_argument("--git-commit", required=True)
    parser.add_argument("--release-notes-ref", required=True)
    parser.add_argument("--server-bin", required=True)
    parser.add_argument("--web-dist", default="web/dist")
    parser.add_argument("--deps-dir", default=".deps")
    parser.add_argument("--templates-dir", default="templates")
    parser.add_argument("--output-dir", default="dist/release")
    parser.add_argument("--evidence-dir", type=Path, help="new directory for persistent validation results and check logs")
    parser.add_argument("--launcher-bundle", default="")
    parser.add_argument("--updater-bin", default="")
    parser.add_argument("--systemd-file", default="")
    parser.add_argument("--license-file", default="LICENSE")
    parser.add_argument("--third-party-notices", default="THIRD_PARTY_NOTICES.md")
    parser.add_argument("--windows-signer-sha256", default="")
    parser.add_argument("--run-smoke", action="store_true")
    parser.add_argument("--run-recovery-drill", action="store_true")
    parser.add_argument("--recovery-plugin-fixture", default="")
    parser.add_argument("--run-self-host-smoke", action="store_true")
    parser.add_argument("--observation-window-seconds", default="")
    parser.add_argument("--window-seconds", default="")
    parser.add_argument("--probe-interval-seconds", default="")
    args = parser.parse_args(argv)
    if args.run_recovery_drill and not args.recovery_plugin_fixture:
        parser.error("--recovery-plugin-fixture is required with --run-recovery-drill")
    return args


def package(args: argparse.Namespace, evidence: ValidationEvidence | None) -> int:
    output_dir = Path(args.output_dir)
    execute = evidence.run if evidence is not None else lambda name, arguments: run(arguments)

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
        "--deps-dir",
        args.deps_dir,
        "--templates-dir",
        args.templates_dir,
        "--license-file",
        args.license_file,
        "--third-party-notices",
        args.third_party_notices,
        "--output-dir",
        args.output_dir,
    ]
    if args.launcher_bundle:
        package_args.extend(["--launcher-bundle", args.launcher_bundle])
    if args.updater_bin:
        package_args.extend(["--updater-bin", args.updater_bin])
    if args.systemd_file:
        package_args.extend(["--systemd-file", args.systemd_file])
    if args.windows_signer_sha256:
        package_args.extend(["--windows-signer-sha256", args.windows_signer_sha256])
    execute("package", package_args)

    archive = archive_path(output_dir, args.version, args.artifact_id)
    if evidence is not None:
        evidence.record_archive(archive)
    if args.run_smoke:
        execute(
            "archive-smoke",
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
            "--plugin-fixture",
            args.recovery_plugin_fixture,
        ]
        if args.observation_window_seconds:
            recovery_args.extend(["--observation-window-seconds", args.observation_window_seconds])
        if evidence is None:
            execute("recovery-drill", recovery_args)
        else:
            with tempfile.TemporaryDirectory(prefix="rayleabot-recovery-validation-") as temporary:
                work = Path(temporary) / "recovery"
                recovery_args.extend(["--output-dir", str(work)])
                execute("recovery-drill", recovery_args)
                shutil.copyfile(work / "result.json", evidence.directory / "recovery-result.json")
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
        execute("self-host-smoke", smoke_args)
    return 0


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv or sys.argv[1:])
    context = ValidationEvidence(args.evidence_dir, args) if args.evidence_dir is not None else nullcontext(None)
    with context as evidence:
        return package(args, evidence)


if __name__ == "__main__":
    raise SystemExit(main())
