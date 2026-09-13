#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import shutil
import sys
import tarfile
import zipfile
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from artifact_ids_generated import ARTIFACT_WINDOWS_X64_FULL, ARTIFACT_LINUX_X64_SERVER
from artifact_matrix import ARTIFACT_MATRIX
from release_content import FORBIDDEN_DIRECTORY_NAMES, find_forbidden_paths, is_forbidden_file_name, should_skip_release_path
from contract_versions_generated import PLUGIN_MANIFEST_VERSION, PLUGIN_UI_BRIDGE_VERSION


RELEASE_METADATA_SCHEMA = Path(__file__).resolve().parents[2] / "contracts" / "release-manifest.schema.json"


@dataclass(frozen=True)
class ArtifactSidecar:
    artifact_id: str
    archive_path: Path
    file_name: str
    platform: str
    support_level: str
    smoke_profile: str
    expanded_size_bytes: int
    file_count: int
    update_mode: str


def utc_now_iso() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")



def ensure_clean_dir(path: Path) -> None:
    if path.exists():
        shutil.rmtree(path)
    path.mkdir(parents=True, exist_ok=True)


def copy_tree(src: Path, dst: Path) -> None:
    if dst.exists():
        shutil.rmtree(dst)
    shutil.copytree(src, dst)


def copy_release_tree(src: Path, dst: Path) -> None:
    if dst.exists():
        shutil.rmtree(dst)
    dst.mkdir(parents=True, exist_ok=True)

    for item in sorted(src.rglob("*")):
        relative = item.relative_to(src)
        if should_skip_release_path(relative):
            continue
        target = dst / relative
        if item.is_dir():
            target.mkdir(parents=True, exist_ok=True)
        else:
            copy_file(item, target)


def copy_deps_manifest(src: Path, dst: Path) -> None:
    if dst.exists():
        shutil.rmtree(dst)
    copy_file(src / "manifest.json", dst / "manifest.json")


def copy_file(src: Path, dst: Path) -> None:
    dst.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(src, dst)


def assert_launcher_bundle_clean(src: Path) -> None:
    if not src.exists():
        raise ValueError(f"launcher bundle path does not exist: {src}")
    candidates = [src] if src.is_file() else sorted(src.rglob("*"))
    for candidate in candidates:
        if candidate.is_symlink():
            raise ValueError(f"launcher bundle contains a symbolic link: {candidate}")
        if candidate.is_dir():
            continue
        relative = Path(candidate.name) if src.is_file() else candidate.relative_to(src)
        if any(part in FORBIDDEN_DIRECTORY_NAMES for part in relative.parts) or is_forbidden_file_name(candidate.name):
            raise ValueError(f"launcher bundle contains development files: {relative.as_posix()}")


def assert_windows_launcher_bundle_layout(src: Path) -> None:
    executable = src / "RayleaLauncher.exe"
    if not executable.is_file() or executable.stat().st_size == 0:
        raise ValueError("Windows launcher bundle is missing RayleaLauncher.exe")
    runtime_guide = src / "WINDOWS-RUNTIME.md"
    if not runtime_guide.is_file() or runtime_guide.stat().st_size == 0:
        raise ValueError("Windows launcher bundle is missing WINDOWS-RUNTIME.md")
    expected = {"RayleaLauncher.exe", "WINDOWS-RUNTIME.md"}
    unexpected = sorted(item.name for item in src.iterdir() if item.name not in expected)
    if unexpected:
        raise ValueError(f"Windows Wails launcher bundle has unexpected entries: {unexpected}")


def copy_launcher_bundle(src: Path, dst_root: Path) -> None:
    assert_launcher_bundle_clean(src)
    if src.is_dir():
        if src.suffix == ".app":
            copy_tree(src, dst_root / src.name)
            return
        for child in src.iterdir():
            target = dst_root / child.name
            if child.is_dir():
                copy_tree(child, target)
            else:
                copy_file(child, target)
        return
    if src.is_file():
        copy_file(src, dst_root / src.name)
        return
    raise ValueError(f"launcher bundle path does not exist: {src}")


def assert_release_tree_clean(root: Path) -> None:
    forbidden = find_forbidden_paths(root)
    if forbidden:
        raise ValueError(f"release package contains development files: {forbidden}")


def stage_release_root(
    artifact_id: str,
    version: str,
    git_commit: str,
    built_at: str,
    output_dir: Path,
    server_bin: Path,
    web_dist: Path,
    deps_dir: Path,
    templates_dir: Path,
    launcher_bundle: Path | None,
    systemd_file: Path | None,
    release_notes_ref: str | None,
    license_file: Path,
    third_party_notices: Path,
) -> tuple[Path, ArtifactSidecar]:
    if artifact_id not in ARTIFACT_MATRIX:
        raise ValueError(f"unsupported artifact_id: {artifact_id}")

    matrix = ARTIFACT_MATRIX[artifact_id]
    if matrix["launcher_required"] and launcher_bundle is None:
        raise ValueError(f"{artifact_id} requires --launcher-bundle")
    if artifact_id == ARTIFACT_LINUX_X64_SERVER and systemd_file is None:
        raise ValueError("linux-x64-server requires --systemd-file")
    if artifact_id == ARTIFACT_WINDOWS_X64_FULL and launcher_bundle is not None:
        assert_windows_launcher_bundle_layout(launcher_bundle)
    for required_file, label in ((license_file, "LICENSE"), (third_party_notices, "THIRD_PARTY_NOTICES.md")):
        if not required_file.is_file() or required_file.stat().st_size == 0:
            raise ValueError(f"release package requires non-empty {label}")
    root_name = f"RayleaBot-v{version}-{artifact_id}"
    stage_root = output_dir / "staging" / root_name
    ensure_clean_dir(stage_root)

    copy_file(server_bin, stage_root / server_bin.name)
    if matrix["launcher_required"] and launcher_bundle is not None:
        copy_launcher_bundle(launcher_bundle, stage_root)
    if artifact_id == ARTIFACT_LINUX_X64_SERVER and systemd_file is not None:
        copy_file(systemd_file, stage_root / "systemd" / "rayleabot.service")

    copy_release_tree(web_dist, stage_root / "web" / "dist")
    copy_deps_manifest(deps_dir, stage_root / ".deps")
    copy_release_tree(templates_dir, stage_root / "templates")
    copy_file(license_file, stage_root / "LICENSE")
    copy_file(third_party_notices, stage_root / "THIRD_PARTY_NOTICES.md")

    build_info = {
        "version": version,
        "git_commit": git_commit,
        "artifact_id": artifact_id,
        "built_at": built_at,
        "plugin_manifest_version": PLUGIN_MANIFEST_VERSION,
        "plugin_ui_bridge_version": PLUGIN_UI_BRIDGE_VERSION,
    }
    if release_notes_ref:
        build_info["release_notes_ref"] = release_notes_ref
    (stage_root / "build_info.json").write_text(json.dumps(build_info, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    assert_release_tree_clean(stage_root)

    packaged_files = sorted(path for path in stage_root.rglob("*") if path.is_file())
    expanded_size_bytes = sum(path.stat().st_size for path in packaged_files)
    file_count = len(packaged_files)

    archive_name = f"{root_name}{matrix['extension']}"
    archive_path = output_dir / archive_name
    archive_path.parent.mkdir(parents=True, exist_ok=True)
    if archive_path.exists():
        archive_path.unlink()

    if artifact_id == ARTIFACT_WINDOWS_X64_FULL:
        with zipfile.ZipFile(archive_path, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=9) as zf:
            for file_path in sorted(stage_root.rglob("*")):
                if file_path.is_dir():
                    continue
                zf.write(file_path, arcname=str(Path(root_name) / file_path.relative_to(stage_root)))
    else:
        with tarfile.open(archive_path, "w:gz") as tf:
            tf.add(stage_root, arcname=root_name)

    sidecar = ArtifactSidecar(
        artifact_id=artifact_id,
        archive_path=archive_path,
        file_name=archive_name,
        platform=matrix["platform"],
        support_level=matrix["support_level"],
        smoke_profile=matrix["smoke_profile"],
        expanded_size_bytes=expanded_size_bytes,
        file_count=file_count,
        update_mode="guided",
    )
    sidecar_path = archive_path.with_suffix(archive_path.suffix + ".artifact.json")
    if archive_path.suffix == ".gz":
        sidecar_path = archive_path.with_name(archive_path.name + ".artifact.json")
    sidecar_path.write_text(
        json.dumps(
            {
                "artifact_id": sidecar.artifact_id,
                "archive_path": str(sidecar.archive_path.resolve()),
                "file_name": sidecar.file_name,
                "platform": sidecar.platform,
                "support_level": sidecar.support_level,
                "smoke_profile": sidecar.smoke_profile,
                "expanded_size_bytes": sidecar.expanded_size_bytes,
                "file_count": sidecar.file_count,
                "update_mode": sidecar.update_mode,
            },
            ensure_ascii=False,
            indent=2,
        )
        + "\n",
        encoding="utf-8",
    )
    return archive_path, sidecar


def load_sidecar(path: Path) -> ArtifactSidecar:
    payload = json.loads(path.read_text(encoding="utf-8"))
    file_name = str(payload["file_name"])
    if Path(file_name).name != file_name:
        raise ValueError("artifact sidecar file_name must be a basename")
    archive_path = path.parent / file_name
    if not archive_path.is_file():
        raise ValueError(f"artifact archive is not adjacent to its sidecar: {archive_path}")
    return ArtifactSidecar(
        artifact_id=payload["artifact_id"],
        archive_path=archive_path,
        file_name=file_name,
        platform=payload["platform"],
        support_level=payload["support_level"],
        smoke_profile=payload["smoke_profile"],
        expanded_size_bytes=int(payload["expanded_size_bytes"]),
        file_count=int(payload["file_count"]),
        update_mode=payload["update_mode"],
    )


def validate_release_metadata(document: dict) -> None:
    # Only metadata generation needs jsonschema; packaging runs without it.
    import jsonschema

    schema = json.loads(RELEASE_METADATA_SCHEMA.read_text(encoding="utf-8"))
    jsonschema.Draft202012Validator(schema, format_checker=jsonschema.FormatChecker()).validate(document)


def build_release_metadata(
    version: str,
    git_commit: str,
    built_at: str,
    config_schema_version: str,
    db_schema_version: str,
    plugin_protocol_version: str,
    release_notes_ref: str,
    download_base_url: str,
    sidecars: list[ArtifactSidecar],
    output_dir: Path,
    channel: str = "stable",
    published_at: str | None = None,
) -> Path:
    output_dir.mkdir(parents=True, exist_ok=True)
    if channel not in {"stable", "beta"}:
        raise ValueError("release channel must be stable or beta")
    publication = parse_release_time(published_at or built_at)
    artifacts = []
    for sidecar in sorted(sidecars, key=lambda item: item.artifact_id):
        archive = sidecar.archive_path
        if not archive.is_file() or archive.name != sidecar.file_name or Path(sidecar.file_name).name != sidecar.file_name:
            raise ValueError(f"invalid release artifact path for {sidecar.artifact_id}")
        if not 1 <= sidecar.file_count <= 100_000:
            raise ValueError(f"invalid release file count for {sidecar.artifact_id}")
        if not 1 <= sidecar.expanded_size_bytes <= 8 * 1024 * 1024 * 1024:
            raise ValueError(f"invalid expanded size for {sidecar.artifact_id}")
        if not 1 <= archive.stat().st_size <= 2 * 1024 * 1024 * 1024:
            raise ValueError(f"invalid archive size for {sidecar.artifact_id}")
        if sidecar.update_mode not in {"guided", "manual"}:
            raise ValueError(f"invalid update mode for {sidecar.artifact_id}")
        artifacts.append(
            {
                "artifact_id": sidecar.artifact_id,
                "file_name": sidecar.file_name,
                "download_url": f"{download_base_url.rstrip('/')}/{sidecar.file_name}",
                "platform": sidecar.platform,
                "archive_size_bytes": archive.stat().st_size,
                "expanded_size_bytes": sidecar.expanded_size_bytes,
                "file_count": sidecar.file_count,
                "update_mode": sidecar.update_mode,
                "support_level": sidecar.support_level,
                "smoke_profile": sidecar.smoke_profile,
            }
        )

    release_manifest = {
        "manifest_version": 2,
        "version": version,
        "git_commit": git_commit,
        "built_at": built_at,
        "channel": channel,
        "published_at": iso_release_time(publication),
        "config_schema_version": config_schema_version,
        "db_schema_version": db_schema_version,
        "plugin_protocol_version": plugin_protocol_version,
        "plugin_manifest_version": PLUGIN_MANIFEST_VERSION,
        "plugin_ui_bridge_version": PLUGIN_UI_BRIDGE_VERSION,
        "artifacts": artifacts,
        "release_notes_ref": release_notes_ref,
    }
    validate_release_metadata(release_manifest)
    manifest_path = output_dir / "release_manifest.v2.json"
    manifest_path.write_text(json.dumps(release_manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    return manifest_path


def parse_release_time(value: str | None) -> datetime:
    if not value:
        raise ValueError("release timestamp is required")
    normalized = value.strip().replace("Z", "+00:00")
    parsed = datetime.fromisoformat(normalized)
    if parsed.tzinfo is None:
        raise ValueError("release timestamp must include a timezone")
    return parsed.astimezone(timezone.utc).replace(microsecond=0)


def iso_release_time(value: datetime) -> str:
    return value.astimezone(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def cmd_package(args: argparse.Namespace) -> int:
    archive_path, _ = stage_release_root(
        artifact_id=args.artifact_id,
        version=args.version,
        git_commit=args.git_commit,
        built_at=args.built_at or utc_now_iso(),
        output_dir=Path(args.output_dir),
        server_bin=Path(args.server_bin),
        web_dist=Path(args.web_dist),
        deps_dir=Path(args.deps_dir),
        templates_dir=Path(args.templates_dir),
        launcher_bundle=Path(args.launcher_bundle) if args.launcher_bundle else None,
        systemd_file=Path(args.systemd_file) if args.systemd_file else None,
        release_notes_ref=args.release_notes_ref,
        license_file=Path(args.license_file),
        third_party_notices=Path(args.third_party_notices),
    )
    print(archive_path)
    return 0


def cmd_metadata(args: argparse.Namespace) -> int:
    sidecars = [load_sidecar(Path(path)) for path in args.sidecar]
    manifest_path = build_release_metadata(
        version=args.version,
        git_commit=args.git_commit,
        built_at=args.built_at or utc_now_iso(),
        config_schema_version=args.config_schema_version,
        db_schema_version=args.db_schema_version,
        plugin_protocol_version=args.plugin_protocol_version,
        release_notes_ref=args.release_notes_ref,
        download_base_url=args.download_base_url,
        sidecars=sidecars,
        output_dir=Path(args.output_dir),
        channel=args.channel,
        published_at=args.published_at,
    )
    print(manifest_path)
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="RayleaBot release packaging helper")
    sub = parser.add_subparsers(dest="command", required=True)

    package = sub.add_parser("package")
    package.add_argument("--artifact-id", required=True, choices=sorted(ARTIFACT_MATRIX.keys()))
    package.add_argument("--version", required=True)
    package.add_argument("--git-commit", required=True)
    package.add_argument("--built-at")
    package.add_argument("--server-bin", required=True)
    package.add_argument("--web-dist", required=True)
    package.add_argument("--deps-dir", required=True)
    package.add_argument("--templates-dir", required=True)
    package.add_argument("--launcher-bundle")
    package.add_argument("--systemd-file")
    package.add_argument("--release-notes-ref")
    package.add_argument("--license-file", default="LICENSE")
    package.add_argument("--third-party-notices", default="THIRD_PARTY_NOTICES.md")
    package.add_argument("--output-dir", required=True)
    package.set_defaults(func=cmd_package)

    metadata = sub.add_parser("metadata")
    metadata.add_argument("--version", required=True)
    metadata.add_argument("--git-commit", required=True)
    metadata.add_argument("--built-at")
    metadata.add_argument("--config-schema-version", required=True)
    metadata.add_argument("--db-schema-version", required=True)
    metadata.add_argument("--plugin-protocol-version", required=True)
    metadata.add_argument("--release-notes-ref", required=True)
    metadata.add_argument("--download-base-url", required=True)
    metadata.add_argument("--channel", default="stable", choices=["stable", "beta"])
    metadata.add_argument("--published-at")
    metadata.add_argument("--sidecar", action="append", required=True)
    metadata.add_argument("--output-dir", required=True)
    metadata.set_defaults(func=cmd_metadata)

    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()
    return args.func(args)


if __name__ == "__main__":
    raise SystemExit(main())
