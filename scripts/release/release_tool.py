#!/usr/bin/env python3
from __future__ import annotations

import argparse
import base64
import hashlib
import json
import fnmatch
import re
import shutil
import subprocess
import tarfile
import tempfile
import zipfile
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
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

FORBIDDEN_TOP_LEVEL_PATHS = {
    ".github",
    "contracts",
    "docs",
    "examples",
    "fixtures",
    "launcher/src",
    "scripts",
    "server",
    "web/src",
}
FORBIDDEN_DIRECTORY_NAMES = {
    ".cache",
    ".git",
    ".pytest_cache",
    ".venv",
    "__pycache__",
    "node_modules",
    "test",
    "tests",
    "venv",
}
FORBIDDEN_FILE_PATTERNS = (
    "*.map",
    "*.pyc",
    "*.pyo",
    "*.spec.*",
    "*.test.*",
    "*_test.*",
)
RELEASE_FILTERED_FILE_PATTERNS = FORBIDDEN_FILE_PATTERNS + ("*.md",)
RELEASE_FILTERED_DIRECTORY_NAMES = FORBIDDEN_DIRECTORY_NAMES


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
    windows_signer_sha256: str | None


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


def normalize_relative(path: Path) -> str:
    return path.as_posix().strip("/")


def is_forbidden_file_name(name: str) -> bool:
    return any(fnmatch.fnmatchcase(name, pattern) for pattern in FORBIDDEN_FILE_PATTERNS)


def should_skip_release_path(relative_path: Path) -> bool:
    parts = relative_path.parts
    if any(part in RELEASE_FILTERED_DIRECTORY_NAMES for part in parts):
        return True
    if parts and any(fnmatch.fnmatchcase(parts[-1], pattern) for pattern in RELEASE_FILTERED_FILE_PATTERNS):
        return True
    return False


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


def copy_launcher_bundle(src: Path, dst_root: Path) -> None:
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


def find_forbidden_release_entries(root: Path) -> list[str]:
    forbidden: list[str] = []
    for item in sorted(root.rglob("*")):
        relative = item.relative_to(root)
        normalized = normalize_relative(relative)
        parts = relative.parts
        if normalized in FORBIDDEN_TOP_LEVEL_PATHS:
            forbidden.append(normalized)
            continue
        if any(normalized == path or normalized.startswith(path + "/") for path in FORBIDDEN_TOP_LEVEL_PATHS):
            forbidden.append(normalized)
            continue
        if any(part in FORBIDDEN_DIRECTORY_NAMES for part in parts):
            forbidden.append(normalized)
            continue
        if item.is_file() and is_forbidden_file_name(item.name):
            forbidden.append(normalized)
    return forbidden


def assert_release_tree_clean(root: Path) -> None:
    forbidden = find_forbidden_release_entries(root)
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
    builtin_dir: Path,
    deps_dir: Path,
    templates_dir: Path,
    default_config: Path,
    launcher_bundle: Path | None,
    systemd_file: Path | None,
    release_notes_ref: str | None,
    updater_bin: Path | None,
    license_file: Path,
    third_party_notices: Path,
    windows_signer_sha256: str | None = None,
) -> tuple[Path, ArtifactSidecar]:
    if artifact_id not in ARTIFACT_MATRIX:
        raise ValueError(f"unsupported artifact_id: {artifact_id}")

    matrix = ARTIFACT_MATRIX[artifact_id]
    if matrix["launcher_required"] and launcher_bundle is None:
        raise ValueError(f"{artifact_id} requires --launcher-bundle")
    if artifact_id == "linux-x64-server" and systemd_file is None:
        raise ValueError("linux-x64-server requires --systemd-file")
    if artifact_id == "windows-x64-full" and updater_bin is None:
        raise ValueError("windows-x64-full requires --updater-bin")
    for required_file, label in ((license_file, "LICENSE"), (third_party_notices, "THIRD_PARTY_NOTICES.md")):
        if not required_file.is_file() or required_file.stat().st_size == 0:
            raise ValueError(f"release package requires non-empty {label}")
    signer_digest = (windows_signer_sha256 or "").strip().lower()
    if signer_digest and not re.fullmatch(r"[0-9a-f]{64}", signer_digest):
        raise ValueError("windows signer SHA256 must be 64 lowercase hexadecimal characters")
    if artifact_id != "windows-x64-full" and signer_digest:
        raise ValueError("windows signer SHA256 is only valid for windows-x64-full")

    root_name = f"RayleaBot-v{version}-{artifact_id}"
    stage_root = output_dir / "staging" / root_name
    ensure_clean_dir(stage_root)

    copy_file(server_bin, stage_root / server_bin.name)
    if artifact_id == "windows-x64-full" and updater_bin is not None:
        copy_file(updater_bin, stage_root / "raylea-updater.exe")
    if matrix["launcher_required"] and launcher_bundle is not None:
        copy_launcher_bundle(launcher_bundle, stage_root)
    if artifact_id == "linux-x64-server" and systemd_file is not None:
        copy_file(systemd_file, stage_root / "systemd" / "rayleabot.service")

    copy_release_tree(web_dist, stage_root / "web" / "dist")
    copy_release_tree(builtin_dir, stage_root / "plugins" / "builtin")
    copy_deps_manifest(deps_dir, stage_root / ".deps")
    copy_release_tree(templates_dir, stage_root / "templates")
    copy_file(default_config, stage_root / "config" / "default.yaml")
    copy_file(license_file, stage_root / "LICENSE")
    copy_file(third_party_notices, stage_root / "THIRD_PARTY_NOTICES.md")

    build_info = {
        "version": version,
        "git_commit": git_commit,
        "artifact_id": artifact_id,
        "built_at": built_at,
        "update_protocol_version": 2,
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
        expanded_size_bytes=expanded_size_bytes,
        file_count=file_count,
        update_mode="automatic" if artifact_id == "windows-x64-full" and signer_digest else "guided",
        windows_signer_sha256=signer_digest or None,
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
                "windows_signer_sha256": sidecar.windows_signer_sha256,
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
        windows_signer_sha256=payload.get("windows_signer_sha256"),
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
    channel: str = "stable",
    published_at: str | None = None,
    expires_at: str | None = None,
) -> tuple[Path, Path]:
    output_dir.mkdir(parents=True, exist_ok=True)
    if channel not in {"stable", "beta"}:
        raise ValueError("release channel must be stable or beta")
    publication = parse_release_time(published_at or built_at)
    expiration = parse_release_time(expires_at) if expires_at else publication + timedelta(days=7)
    if expiration <= publication:
        raise ValueError("release manifest expiration must be later than publication")
    deps_manifest_sha256 = sha256_file(deps_manifest)
    artifacts = []
    checksum_lines = []
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
        if sidecar.update_mode not in {"automatic", "guided", "manual"}:
            raise ValueError(f"invalid update mode for {sidecar.artifact_id}")
        if sidecar.update_mode == "automatic" and (
            sidecar.artifact_id != "windows-x64-full"
            or not sidecar.windows_signer_sha256
            or not re.fullmatch(r"[0-9a-f]{64}", sidecar.windows_signer_sha256)
        ):
            raise ValueError("automatic updates require a verified windows-x64-full signer")
        artifact_sha = sha256_file(archive)
        artifacts.append(
            {
                "artifact_id": sidecar.artifact_id,
                "file_name": sidecar.file_name,
                "platform": sidecar.platform,
                "sha256": artifact_sha,
                "archive_size_bytes": archive.stat().st_size,
                "expanded_size_bytes": sidecar.expanded_size_bytes,
                "file_count": sidecar.file_count,
                "update_mode": sidecar.update_mode,
                "min_updater_protocol_version": 2,
                "support_level": sidecar.support_level,
                "deps_manifest_sha256": deps_manifest_sha256,
                "smoke_profile": sidecar.smoke_profile,
            }
        )
        if sidecar.windows_signer_sha256:
            artifacts[-1]["windows_signer_sha256"] = sidecar.windows_signer_sha256
        checksum_lines.append(f"{artifact_sha}  {sidecar.file_name}")

    release_manifest = {
        "manifest_version": 2,
        "version": version,
        "git_commit": git_commit,
        "built_at": built_at,
        "channel": channel,
        "published_at": iso_release_time(publication),
        "expires_at": iso_release_time(expiration),
        "update_protocol_version": 2,
        "config_schema_version": config_schema_version,
        "db_schema_version": db_schema_version,
        "plugin_protocol_version": plugin_protocol_version,
        "artifacts": artifacts,
        "release_notes_ref": release_notes_ref,
    }
    manifest_path = output_dir / "release_manifest.v2.json"
    manifest_path.write_text(json.dumps(release_manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    checksum_lines.append(f"{sha256_file(manifest_path)}  release_manifest.v2.json")
    checksums_path = output_dir / "SHA256SUMS.txt"
    checksums_path.write_text("\n".join(checksum_lines) + "\n", encoding="utf-8")
    return manifest_path, checksums_path


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
    if checksums.get(manifest_path.name) != manifest_digest:
        raise SystemExit(f"SHA256SUMS.txt does not match {manifest_path.name}")

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
        if path.stat().st_size != artifact["archive_size_bytes"]:
            raise SystemExit(f"artifact size mismatch: {file_name}")


def sign_release_manifest(
    manifest_path: Path,
    output_path: Path,
    keys: list[tuple[str, Path]],
    openssl: str = "openssl",
) -> Path:
    if not manifest_path.is_file():
        raise ValueError(f"release manifest does not exist: {manifest_path}")
    if not 1 <= len(keys) <= 2:
        raise ValueError("one or two Ed25519 signing keys are required")
    seen: set[str] = set()
    manifest_bytes = manifest_path.read_bytes()
    signatures: list[dict[str, str]] = []
    with tempfile.TemporaryDirectory(prefix="raylea-release-sign-") as temp_dir:
        for key_id, private_key in keys:
            if not re.fullmatch(r"[a-z0-9][a-z0-9._-]{2,63}", key_id) or key_id in seen:
                raise ValueError(f"invalid or duplicate release key id: {key_id}")
            if not private_key.is_file():
                raise ValueError(f"release private key does not exist: {private_key}")
            seen.add(key_id)
            signature_path = Path(temp_dir) / f"{key_id}.sig"
            result = subprocess.run(
                [openssl, "pkeyutl", "-sign", "-rawin", "-inkey", str(private_key), "-in", str(manifest_path), "-out", str(signature_path)],
                check=False,
                capture_output=True,
                text=True,
            )
            if result.returncode != 0:
                raise RuntimeError(f"OpenSSL Ed25519 signing failed for {key_id}")
            signature = signature_path.read_bytes()
            if len(signature) != 64:
                raise RuntimeError(f"OpenSSL returned an invalid Ed25519 signature for {key_id}")
            signatures.append({
                "key_id": key_id,
                "signature": base64.urlsafe_b64encode(signature).decode("ascii"),
            })
    envelope = {
        "signature_version": 1,
        "algorithm": "ed25519",
        "manifest_sha256": hashlib.sha256(manifest_bytes).hexdigest(),
        "key_id": keys[0][0],
        "signatures": signatures,
    }
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(envelope, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    return output_path


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
        deps_dir=Path(args.deps_dir),
        templates_dir=Path(args.templates_dir),
        default_config=Path(args.default_config),
        launcher_bundle=Path(args.launcher_bundle) if args.launcher_bundle else None,
        systemd_file=Path(args.systemd_file) if args.systemd_file else None,
        release_notes_ref=args.release_notes_ref,
        updater_bin=Path(args.updater_bin) if args.updater_bin else None,
        license_file=Path(args.license_file),
        third_party_notices=Path(args.third_party_notices),
        windows_signer_sha256=args.windows_signer_sha256,
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
        channel=args.channel,
        published_at=args.published_at,
        expires_at=args.expires_at,
    )
    print(manifest_path)
    print(checksums_path)
    return 0


def cmd_verify(args: argparse.Namespace) -> int:
    verify_release_bundle(Path(args.manifest), Path(args.checksums), Path(args.artifact_dir))
    print("release bundle verified")
    return 0


def cmd_sign(args: argparse.Namespace) -> int:
    keys: list[tuple[str, Path]] = []
    for value in args.key:
        key_id, separator, key_path = value.partition("=")
        if not separator:
            raise ValueError("--key must use key_id=private_key_path")
        keys.append((key_id, Path(key_path)))
    output = sign_release_manifest(Path(args.manifest), Path(args.output), keys, args.openssl)
    print(output)
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
    package.add_argument("--deps-dir", required=True)
    package.add_argument("--templates-dir", required=True)
    package.add_argument("--default-config", required=True)
    package.add_argument("--launcher-bundle")
    package.add_argument("--updater-bin")
    package.add_argument("--systemd-file")
    package.add_argument("--release-notes-ref")
    package.add_argument("--license-file", default="LICENSE")
    package.add_argument("--third-party-notices", default="THIRD_PARTY_NOTICES.md")
    package.add_argument("--windows-signer-sha256")
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
    metadata.add_argument("--channel", default="stable", choices=["stable", "beta"])
    metadata.add_argument("--published-at")
    metadata.add_argument("--expires-at")
    metadata.add_argument("--deps-manifest", required=True)
    metadata.add_argument("--sidecar", action="append", required=True)
    metadata.add_argument("--output-dir", required=True)
    metadata.set_defaults(func=cmd_metadata)

    verify = sub.add_parser("verify")
    verify.add_argument("--manifest", required=True)
    verify.add_argument("--checksums", required=True)
    verify.add_argument("--artifact-dir", required=True)
    verify.set_defaults(func=cmd_verify)

    sign = sub.add_parser("sign")
    sign.add_argument("--manifest", required=True)
    sign.add_argument("--output", required=True)
    sign.add_argument("--key", action="append", required=True, help="key_id=PEM_private_key_path; repeat once for dual-sign rotation")
    sign.add_argument("--openssl", default="openssl")
    sign.set_defaults(func=cmd_sign)

    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()
    return args.func(args)


if __name__ == "__main__":
    raise SystemExit(main())
