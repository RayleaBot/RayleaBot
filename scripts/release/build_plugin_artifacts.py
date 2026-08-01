#!/usr/bin/env python3
"""Build and verify the production Go plugin artifact tree."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import shutil
import stat
import subprocess
import sys
import zipfile
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]
BUILTIN_PLUGINS = (
    Path("plugins/builtin/echo"),
    Path("plugins/builtin/fortune"),
    Path("plugins/builtin/game_guide"),
    Path("plugins/builtin/subscription_hub"),
)
EXAMPLE_PLUGINS = (
    Path("examples/plugins/echo-go"),
    Path("examples/plugins/hello-go"),
    Path("examples/plugins/example-capability-parameters"),
    Path("examples/plugins/example-config-panel"),
    Path("examples/plugins/example-governance-control"),
    Path("examples/plugins/example-plugin-list"),
    Path("examples/plugins/example-render-card"),
    Path("examples/plugins/example-scheduler"),
    Path("examples/plugins/example-webhook"),
    Path("examples/plugins/notice-logger"),
)
TARGETS = ("windows-x64", "linux-x64", "macos-arm64")
FORBIDDEN_SUFFIXES = {".go", ".py", ".ts", ".tsx", ".vue", ".map"}
FORBIDDEN_NAMES = {"go.mod", "go.sum", "package.json", "pnpm-lock.yaml", "pnpm-workspace.yaml"}
FORBIDDEN_PARTS = {"node_modules", "test", "tests", "src", "tools"}


class ArtifactError(RuntimeError):
    pass


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def expected_backend_path(manifest: dict[str, object], target: str) -> str:
    logical = str(manifest.get("entry", ""))
    if not logical.startswith("bin/") or Path(logical).suffix:
        raise ArtifactError("plugin entry must be an extensionless path below bin/")
    return logical + (".exe" if target == "windows-x64" else "")


def verify_binary(path: Path, target: str) -> None:
    prefix = path.read_bytes()[:4]
    expected = {
        "windows-x64": b"MZ",
        "linux-x64": b"\x7fELF",
        "macos-arm64": b"\xcf\xfa\xed\xfe",
    }[target]
    if not prefix.startswith(expected):
        raise ArtifactError(f"backend binary format does not match {target}: {path}")
    if target != "windows-x64" and os.name != "nt" and not path.stat().st_mode & stat.S_IXUSR:
        raise ArtifactError(f"backend binary is not executable: {path}")


def verify_archive_mode(archive_path: Path, plugin_id: str, backend: str, target: str) -> None:
    if not archive_path.is_file():
        raise ArtifactError(f"plugin artifact ZIP does not exist: {archive_path}")
    expected = f"{plugin_id}/{backend}"
    with zipfile.ZipFile(archive_path) as archive:
        names = archive.namelist()
        if any(not name.startswith(f"{plugin_id}/") for name in names):
            raise ArtifactError("plugin artifact ZIP must contain one plugin root directory")
        try:
            backend_info = archive.getinfo(expected)
        except KeyError as exc:
            raise ArtifactError(f"plugin artifact ZIP is missing backend entry: {expected}") from exc
        if target != "windows-x64" and stat.S_IMODE(backend_info.external_attr >> 16) != 0o755:
            raise ArtifactError(f"plugin artifact ZIP backend mode is not 0755: {expected}")


def verify_artifact(root: Path, expected_target: str, archive_path: Path | None = None) -> dict[str, object]:
    if expected_target not in TARGETS:
        raise ArtifactError(f"unsupported target platform: {expected_target}")
    if not root.is_dir():
        raise ArtifactError(f"plugin artifact root does not exist: {root}")

    info_path = root / "info.json"
    artifact_path = root / "artifact.json"
    try:
        manifest = json.loads(info_path.read_text(encoding="utf-8"))
        artifact = json.loads(artifact_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise ArtifactError(f"read plugin artifact metadata: {exc}") from exc

    if manifest.get("manifest_version") != "2" or manifest.get("runtime") != "go":
        raise ArtifactError("production plugin manifest must use manifest v2 and the Go runtime")
    if manifest.get("plugin_protocol_version") != "1":
        raise ArtifactError("production plugin protocol version must be 1")
    platforms = manifest.get("platforms")
    if not isinstance(platforms, list) or expected_target not in platforms:
        raise ArtifactError(f"plugin manifest does not declare {expected_target}")
    if artifact.get("artifact_version") != "1":
        raise ArtifactError("plugin artifact version must be 1")
    if artifact.get("plugin_id") != manifest.get("id") or artifact.get("plugin_version") != manifest.get("version"):
        raise ArtifactError("plugin artifact identity does not match info.json")
    if artifact.get("target_platform") != expected_target:
        raise ArtifactError("plugin artifact target does not match the release target")
    if artifact.get("manifest_sha256") != sha256_file(info_path):
        raise ArtifactError("plugin manifest digest mismatch")

    raw_files = artifact.get("files")
    if not isinstance(raw_files, list) or not raw_files:
        raise ArtifactError("plugin artifact file inventory is empty")
    listed: dict[str, dict[str, object]] = {}
    backend_files: list[str] = []
    ui_files: set[str] = set()
    for item in raw_files:
        if not isinstance(item, dict):
            raise ArtifactError("plugin artifact inventory entry must be an object")
        relative = str(item.get("path", ""))
        path = Path(relative)
        if not relative or path.is_absolute() or ".." in path.parts or relative in listed:
            raise ArtifactError(f"invalid plugin artifact inventory path: {relative!r}")
        listed[relative] = item
        role = item.get("role")
        if role == "backend":
            backend_files.append(relative)
        if role == "ui":
            ui_files.add(relative)

    actual = {
        path.relative_to(root).as_posix()
        for path in root.rglob("*")
        if path.is_file() and path != artifact_path
    }
    if set(listed) != actual:
        raise ArtifactError(
            f"plugin artifact inventory mismatch: missing={sorted(actual - set(listed))}, extra={sorted(set(listed) - actual)}"
        )
    for relative, item in listed.items():
        path = root / relative
        if path.stat().st_size != item.get("size") or sha256_file(path) != item.get("sha256"):
            raise ArtifactError(f"plugin artifact digest or size mismatch: {relative}")
        if path.suffix.lower() in FORBIDDEN_SUFFIXES or path.name in FORBIDDEN_NAMES or any(part in FORBIDDEN_PARTS for part in path.parts):
            raise ArtifactError(f"plugin artifact contains development source: {relative}")

    backend = expected_backend_path(manifest, expected_target)
    if backend_files != [backend]:
        raise ArtifactError(f"plugin artifact must contain exactly one backend entry {backend}: {backend_files}")
    verify_binary(root / backend, expected_target)
    if archive_path is not None:
        verify_archive_mode(archive_path, str(manifest["id"]), backend, expected_target)

    management = manifest.get("management_ui")
    if isinstance(management, dict):
        pages = management.get("pages")
        if not isinstance(pages, list) or not pages:
            raise ArtifactError("management_ui.pages must not be empty")
        for page in pages:
            entry = str(page.get("entry", "")) if isinstance(page, dict) else ""
            if entry not in ui_files:
                raise ArtifactError(f"management UI entry is not listed as a UI file: {entry}")
    elif ui_files:
        raise ArtifactError("UI files require management_ui pages in info.json")
    return artifact


def build_plugins(target: str, output: Path, plugins: tuple[Path, ...] = BUILTIN_PLUGINS) -> Path:
    if target not in TARGETS:
        raise ArtifactError(f"unsupported target platform: {target}")
    output = output.resolve()
    target_root = output / target
    if target_root.exists():
        shutil.rmtree(target_root)
    target_root.mkdir(parents=True, exist_ok=True)

    for relative in plugins:
        plugin_dir = (REPO_ROOT / relative).resolve()
        subprocess.run(
            ["go", "run", "./tools/build", "-target", target, "-out", str(output)],
            cwd=plugin_dir,
            check=True,
        )

    artifacts = sorted(path for path in target_root.iterdir() if path.is_dir())
    if len(artifacts) != len(plugins):
        raise ArtifactError(f"expected {len(plugins)} expanded plugin artifacts, found {len(artifacts)}")
    identities: set[str] = set()
    for artifact_root in artifacts:
        manifest = json.loads((artifact_root / "info.json").read_text(encoding="utf-8"))
        archive_path = output / f"{manifest['id']}-{manifest['version']}-{target}.zip"
        document = verify_artifact(artifact_root, target, archive_path)
        plugin_id = str(document["plugin_id"])
        if plugin_id in identities:
            raise ArtifactError(f"duplicate built-in plugin ID: {plugin_id}")
        identities.add(plugin_id)
    return target_root


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--target", choices=TARGETS, required=True)
    parser.add_argument("--output", type=Path, default=REPO_ROOT / "dist" / "plugin-artifacts")
    parser.add_argument("--verify-only", type=Path)
    parser.add_argument("--include-examples", action="store_true", help="also build every Go example plugin")
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv or sys.argv[1:])
    if args.verify_only is not None:
        verify_artifact(args.verify_only.resolve(), args.target)
        print(args.verify_only.resolve())
        return 0
    plugins = BUILTIN_PLUGINS + EXAMPLE_PLUGINS if args.include_examples else BUILTIN_PLUGINS
    print(build_plugins(args.target, args.output, plugins))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
