#!/usr/bin/env python3
from __future__ import annotations

import argparse
import hashlib
import json
import shutil
import tarfile
import tempfile
import zipfile
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path


ARTIFACT_MATRIX = {
    "windows-x64-full": {
        "platform": "windows-x64",
        "support_level": "first_class",
        "smoke_profile": "windows_full_smoke",
        "extension": ".zip",
        "launcher_required": True,
    },
    "linux-x64-full": {
        "platform": "linux-x64",
        "support_level": "first_class",
        "smoke_profile": "linux_full_smoke",
        "extension": ".tar.gz",
        "launcher_required": True,
    },
    "macos-arm64-full": {
        "platform": "macos-arm64",
        "support_level": "first_class",
        "smoke_profile": "macos_full_smoke",
        "extension": ".tar.gz",
        "launcher_required": True,
    },
    "linux-x64-server": {
        "platform": "linux-x64",
        "support_level": "first_class",
        "smoke_profile": "linux_server_smoke",
        "extension": ".tar.gz",
        "launcher_required": False,
    },
}


@dataclass(frozen=True)
class ArtifactSidecar:
    artifact_id: str
    archive_path: Path
    file_name: str
    platform: str
    support_level: str
    smoke_profile: str


def utc_now_iso() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def ensure_clean_dir(path: Path) -> None:
    if path.exists():
        shutil.rmtree(path)
    path.mkdir(parents=True, exist_ok=True)


def copy_tree(src: Path, dst: Path) -> None:
    if dst.exists():
        shutil.rmtree(dst)
    shutil.copytree(src, dst)


def copy_file(src: Path, dst: Path) -> None:
    dst.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(src, dst)


def copy_launcher_bundle(src: Path, dst: Path) -> None:
    if src.is_dir():
        if src.suffix == ".app":
            copy_tree(src, dst / src.name)
            return
        copy_tree(src, dst)
        return
    if src.is_file():
        copy_file(src, dst / src.name)
        return
    raise ValueError(f"launcher bundle path does not exist: {src}")


def stage_release_root(
    artifact_id: str,
    version: str,
    git_commit: str,
    built_at: str,
    output_dir: Path,
    server_bin: Path,
    web_dist: Path,
    builtin_dir: Path,
    contracts_dir: Path,
    deps_dir: Path,
    templates_dir: Path,
    default_config: Path,
    launcher_bundle: Path | None,
    systemd_file: Path | None,
    release_notes_ref: str | None,
) -> tuple[Path, ArtifactSidecar]:
    if artifact_id not in ARTIFACT_MATRIX:
        raise ValueError(f"unsupported artifact_id: {artifact_id}")

    matrix = ARTIFACT_MATRIX[artifact_id]
    if matrix["launcher_required"] and launcher_bundle is None:
        raise ValueError(f"{artifact_id} requires --launcher-bundle")
    if artifact_id == "linux-x64-server" and systemd_file is None:
        raise ValueError("linux-x64-server requires --systemd-file")

    root_name = f"RayleaBot-v{version}-{artifact_id}"
    stage_root = output_dir / "staging" / root_name
    ensure_clean_dir(stage_root)

    copy_file(server_bin, stage_root / server_bin.name)
    if matrix["launcher_required"] and launcher_bundle is not None:
        copy_launcher_bundle(launcher_bundle, stage_root / "launcher")
    if artifact_id == "linux-x64-server" and systemd_file is not None:
        copy_file(systemd_file, stage_root / "systemd" / "rayleabot.service")

    copy_tree(web_dist, stage_root / "web" / "dist")
    copy_tree(builtin_dir, stage_root / "plugins" / "builtin")
    copy_tree(contracts_dir, stage_root / "contracts")
    copy_tree(deps_dir, stage_root / ".deps")
    copy_tree(templates_dir, stage_root / "templates")
    copy_file(default_config, stage_root / "config" / "default.yaml")

    build_info = {
        "version": version,
        "git_commit": git_commit,
        "artifact_id": artifact_id,
        "built_at": built_at,
    }
    if release_notes_ref:
        build_info["release_notes_ref"] = release_notes_ref
    (stage_root / "build_info.json").write_text(json.dumps(build_info, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

    archive_name = f"{root_name}{matrix['extension']}"
    archive_path = output_dir / archive_name
    archive_path.parent.mkdir(parents=True, exist_ok=True)
    if archive_path.exists():
        archive_path.unlink()

    if artifact_id == "windows-x64-full":
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
    return ArtifactSidecar(
        artifact_id=payload["artifact_id"],
        archive_path=Path(payload["archive_path"]),
        file_name=payload["file_name"],
        platform=payload["platform"],
        support_level=payload["support_level"],
        smoke_profile=payload["smoke_profile"],
    )


def build_release_metadata(
    version: str,
    git_commit: str,
    built_at: str,
    config_schema_version: str,
    db_schema_version: str,
    plugin_protocol_version: str,
    release_notes_ref: str,
    deps_manifest: Path,
    sidecars: list[ArtifactSidecar],
    output_dir: Path,
) -> tuple[Path, Path]:
    output_dir.mkdir(parents=True, exist_ok=True)
    deps_manifest_sha256 = sha256_file(deps_manifest)
    artifacts = []
    checksum_lines = []
    for sidecar in sorted(sidecars, key=lambda item: item.artifact_id):
        archive = sidecar.archive_path
        artifact_sha = sha256_file(archive)
        artifacts.append(
            {
                "artifact_id": sidecar.artifact_id,
                "file_name": sidecar.file_name,
                "platform": sidecar.platform,
                "sha256": artifact_sha,
                "size": archive.stat().st_size,
                "support_level": sidecar.support_level,
                "deps_manifest_sha256": deps_manifest_sha256,
                "smoke_profile": sidecar.smoke_profile,
            }
        )
        checksum_lines.append(f"{artifact_sha}  {sidecar.file_name}")

    release_manifest = {
        "version": version,
        "git_commit": git_commit,
        "built_at": built_at,
        "config_schema_version": config_schema_version,
        "db_schema_version": db_schema_version,
        "plugin_protocol_version": plugin_protocol_version,
        "artifacts": artifacts,
        "release_notes_ref": release_notes_ref,
    }
    manifest_path = output_dir / "release_manifest.json"
    manifest_path.write_text(json.dumps(release_manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    checksum_lines.append(f"{sha256_file(manifest_path)}  release_manifest.json")
    checksums_path = output_dir / "SHA256SUMS.txt"
    checksums_path.write_text("\n".join(checksum_lines) + "\n", encoding="utf-8")
    return manifest_path, checksums_path


def parse_checksums(path: Path) -> dict[str, str]:
    result: dict[str, str] = {}
    for line in path.read_text(encoding="utf-8").splitlines():
        if not line.strip():
            continue
        digest, file_name = line.split("  ", 1)
        result[file_name] = digest
    return result


def verify_release_bundle(manifest_path: Path, checksums_path: Path, artifact_dir: Path) -> None:
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    checksums = parse_checksums(checksums_path)
    manifest_digest = sha256_file(manifest_path)
    if checksums.get("release_manifest.json") != manifest_digest:
        raise SystemExit("SHA256SUMS.txt does not match release_manifest.json")

    for artifact in manifest.get("artifacts", []):
        file_name = artifact["file_name"]
        path = artifact_dir / file_name
        if not path.exists():
            raise SystemExit(f"missing artifact listed in manifest: {file_name}")
        digest = sha256_file(path)
        if digest != artifact["sha256"]:
            raise SystemExit(f"artifact sha256 mismatch: {file_name}")
        if checksums.get(file_name) != digest:
            raise SystemExit(f"SHA256SUMS.txt mismatch: {file_name}")
        if path.stat().st_size != artifact["size"]:
            raise SystemExit(f"artifact size mismatch: {file_name}")


def cmd_package(args: argparse.Namespace) -> int:
    archive_path, _ = stage_release_root(
        artifact_id=args.artifact_id,
        version=args.version,
        git_commit=args.git_commit,
        built_at=args.built_at or utc_now_iso(),
        output_dir=Path(args.output_dir),
        server_bin=Path(args.server_bin),
        web_dist=Path(args.web_dist),
        builtin_dir=Path(args.builtin_dir),
        contracts_dir=Path(args.contracts_dir),
        deps_dir=Path(args.deps_dir),
        templates_dir=Path(args.templates_dir),
        default_config=Path(args.default_config),
        launcher_bundle=Path(args.launcher_bundle) if args.launcher_bundle else None,
        systemd_file=Path(args.systemd_file) if args.systemd_file else None,
        release_notes_ref=args.release_notes_ref,
    )
    print(archive_path)
    return 0


def cmd_metadata(args: argparse.Namespace) -> int:
    sidecars = [load_sidecar(Path(path)) for path in args.sidecar]
    manifest_path, checksums_path = build_release_metadata(
        version=args.version,
        git_commit=args.git_commit,
        built_at=args.built_at or utc_now_iso(),
        config_schema_version=args.config_schema_version,
        db_schema_version=args.db_schema_version,
        plugin_protocol_version=args.plugin_protocol_version,
        release_notes_ref=args.release_notes_ref,
        deps_manifest=Path(args.deps_manifest),
        sidecars=sidecars,
        output_dir=Path(args.output_dir),
    )
    print(manifest_path)
    print(checksums_path)
    return 0


def cmd_verify(args: argparse.Namespace) -> int:
    verify_release_bundle(Path(args.manifest), Path(args.checksums), Path(args.artifact_dir))
    print("release bundle verified")
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
    package.add_argument("--builtin-dir", required=True)
    package.add_argument("--contracts-dir", required=True)
    package.add_argument("--deps-dir", required=True)
    package.add_argument("--templates-dir", required=True)
    package.add_argument("--default-config", required=True)
    package.add_argument("--launcher-bundle")
    package.add_argument("--systemd-file")
    package.add_argument("--release-notes-ref")
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
    metadata.add_argument("--deps-manifest", required=True)
    metadata.add_argument("--sidecar", action="append", required=True)
    metadata.add_argument("--output-dir", required=True)
    metadata.set_defaults(func=cmd_metadata)

    verify = sub.add_parser("verify")
    verify.add_argument("--manifest", required=True)
    verify.add_argument("--checksums", required=True)
    verify.add_argument("--artifact-dir", required=True)
    verify.set_defaults(func=cmd_verify)

    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()
    return args.func(args)


if __name__ == "__main__":
    raise SystemExit(main())
