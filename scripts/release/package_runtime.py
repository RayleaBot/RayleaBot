#!/usr/bin/env python3
from __future__ import annotations

import contextlib
import hashlib
import json
import os
import re
import signal
import socket
import subprocess
import shutil
import tarfile
import tempfile
import time
import urllib.request
import zipfile
from pathlib import Path


SERVER_BINARIES = {
    "windows-x64-full": "raylea-server.exe",
    "linux-x64-full": "raylea-server",
    "macos-arm64-full": "raylea-server",
    "linux-x64-server": "raylea-server",
}

RESOURCE_KINDS = ("chromium", "python-runtime", "nodejs-runtime")
REQUIRED_ENTRYPOINTS = {
    "chromium": ("browser",),
    "python-runtime": ("python",),
    "nodejs-runtime": ("node", "npm"),
}
SOURCE_KINDS = {"upstream", "mirror"}
ARCHIVE_SUFFIXES = {
    "zip": ".zip",
    "tar.gz": ".tar.gz",
    "tar.xz": ".tar.xz",
}

REQUIRED_PATHS = {
    "windows-x64-full": {
        "raylea-server.exe",
        "RayleaLauncher.exe",
        "build_info.json",
        "config/default.yaml",
        "contracts/config.user.schema.json",
        "contracts/plugin-info.schema.json",
        "web/dist/index.html",
        ".deps/manifest.json",
        "templates/help.menu/template.json",
        "templates/status.panel/template.json",
    },
    "linux-x64-full": {
        "raylea-server",
        "RayleaLauncher",
        "build_info.json",
        "config/default.yaml",
        "contracts/config.user.schema.json",
        "contracts/plugin-info.schema.json",
        "web/dist/index.html",
        ".deps/manifest.json",
        "templates/help.menu/template.json",
        "templates/status.panel/template.json",
    },
    "macos-arm64-full": {
        "raylea-server",
        "RayleaLauncher.app/Contents/MacOS/RayleaLauncher",
        "build_info.json",
        "config/default.yaml",
        "contracts/config.user.schema.json",
        "contracts/plugin-info.schema.json",
        "web/dist/index.html",
        ".deps/manifest.json",
        "templates/help.menu/template.json",
        "templates/status.panel/template.json",
    },
    "linux-x64-server": {
        "raylea-server",
        "build_info.json",
        "config/default.yaml",
        "contracts/config.user.schema.json",
        "contracts/plugin-info.schema.json",
        "web/dist/index.html",
        ".deps/manifest.json",
        "systemd/rayleabot.service",
        "templates/help.menu/template.json",
        "templates/status.panel/template.json",
    },
}


def unpack_archive(artifact_id: str, archive_path: Path, destination: Path) -> Path:
    destination.mkdir(parents=True, exist_ok=True)
    if artifact_id == "windows-x64-full":
        with zipfile.ZipFile(archive_path) as zf:
            zf.extractall(destination)
            names = [name for name in zf.namelist() if name]
    else:
        with tarfile.open(archive_path, "r:gz") as tf:
            tf.extractall(destination)
            names = [member.name for member in tf.getmembers() if member.name]
    if not names:
        raise RuntimeError("archive is empty")
    root_name = Path(names[0]).parts[0]
    root = destination / root_name
    if not root.is_dir():
        raise RuntimeError(f"release root not found after extraction: {root}")
    return root


def ensure_required_paths(root: Path, artifact_id: str) -> None:
    missing = sorted(path for path in REQUIRED_PATHS[artifact_id] if not (root / path).exists())
    if missing:
        raise RuntimeError(f"missing required packaged paths: {missing}")


def choose_free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        return int(sock.getsockname()[1])


def write_user_config(root: Path, port: int) -> Path:
    default_path = root / "config" / "default.yaml"
    user_path = root / "config" / "user.yaml"
    text = default_path.read_text(encoding="utf-8")
    updated = re.sub(r"(?m)^(\s*port:\s*)8080$", rf"\g<1>{port}", text, count=1)
    if updated == text:
        raise RuntimeError("failed to rewrite server.port in default config")
    user_path.write_text(updated, encoding="utf-8")
    return user_path


def relative_executable(root: Path, artifact_id: str) -> Path:
    return root / SERVER_BINARIES[artifact_id]


def server_base_command(server_bin: Path) -> list[str]:
    return [
        str(server_bin),
        "-config",
        "config/user.yaml",
        "-config-schema",
        "contracts/config.user.schema.json",
    ]


def artifact_platform(artifact_id: str) -> str:
    parts = artifact_id.split("-")
    if len(parts) < 2:
        raise RuntimeError(f"invalid artifact id: {artifact_id}")
    return "-".join(parts[:2])


def load_deps_manifest(root: Path) -> dict[str, object]:
    manifest_path = root / ".deps" / "manifest.json"
    payload = json.loads(manifest_path.read_text(encoding="utf-8"))
    if payload.get("manifest_version") != 3:
        raise RuntimeError(f"unsupported deps manifest version: {payload.get('manifest_version')}")
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
    sources = resource.get("sources")
    sha256 = str(resource.get("sha256", "")).strip().lower()
    archive_format = str(resource.get("archive_format", "")).strip()
    if not _sources_are_complete(sources):
        return False
    if len(sha256) != 64 or any(ch not in "0123456789abcdef" for ch in sha256):
        return False
    if archive_format not in ARCHIVE_SUFFIXES:
        return False
    entrypoints = resource.get("entrypoints")
    if not isinstance(entrypoints, dict):
        return False
    for key in REQUIRED_ENTRYPOINTS.get(str(resource.get("kind", "")), ()):
        candidates = entrypoints.get(key)
        if not isinstance(candidates, list) or not any(_valid_entrypoint_candidate(item) for item in candidates):
            return False
    return True


def _sources_are_complete(value: object) -> bool:
    if not isinstance(value, list) or len(value) == 0:
        return False
    seen: set[str] = set()
    for item in value:
        if not isinstance(item, dict):
            return False
        url = str(item.get("url", "")).strip()
        kind = str(item.get("kind", "")).strip()
        if not url.startswith("https://") or "TODO(" in url.upper():
            return False
        if kind not in SOURCE_KINDS:
            return False
        if url in seen:
            return False
        seen.add(url)
    return True


def resource_sources(resource: dict[str, object]) -> list[dict[str, str]]:
    sources = resource.get("sources")
    if not isinstance(sources, list):
        raise RuntimeError(f"resource sources missing: {resource}")
    normalized: list[dict[str, str]] = []
    for item in sources:
        if not isinstance(item, dict):
            raise RuntimeError(f"resource source entry invalid: {resource}")
        normalized.append(
            {
                "url": str(item.get("url", "")).strip(),
                "kind": str(item.get("kind", "")).strip(),
                "label": str(item.get("label", "")).strip(),
            }
        )
    if not _sources_are_complete(normalized):
        raise RuntimeError(f"resource sources are not bootstrap-ready: {resource}")
    return normalized


def _valid_entrypoint_candidate(value: object) -> bool:
    if not isinstance(value, str):
        return False
    text = value.strip()
    return bool(text) and not text.startswith("..") and not Path(text).is_absolute()


def store_root(root: Path, resource: dict[str, object]) -> Path:
    return root / ".deps" / "store" / str(resource["id"]) / str(resource["version"])


def cache_root(root: Path) -> Path:
    return root / "cache" / "downloads" / "runtime"


def resolve_prepared_entrypoints(root: Path, resource: dict[str, object]) -> dict[str, Path]:
    prepared: dict[str, Path] = {}
    entrypoints = resource.get("entrypoints")
    if not isinstance(entrypoints, dict):
        raise RuntimeError(f"resource entrypoints missing for {resource}")
    for key in REQUIRED_ENTRYPOINTS.get(str(resource.get("kind", "")), ()):
        candidates = entrypoints.get(key)
        if not isinstance(candidates, list):
            raise RuntimeError(f"resource entrypoint list missing for {resource}")
        resolved = None
        for candidate in candidates:
            if not _valid_entrypoint_candidate(candidate):
                continue
            path = store_root(root, resource) / Path(str(candidate))
            if path.exists() and path.is_file():
                resolved = path
                break
        if resolved is None:
            raise RuntimeError(f"prepared runtime is missing entrypoint {key} for {resource['kind']}")
        prepared[key] = resolved
    return prepared


@contextlib.contextmanager
def runtime_lock(root: Path):
    lock_path = root / "cache" / "downloads" / "platform.lock"
    lock_path.parent.mkdir(parents=True, exist_ok=True)
    while True:
        try:
            fd = os.open(lock_path, os.O_CREAT | os.O_EXCL | os.O_WRONLY)
            os.write(fd, f"{os.getpid()}\n".encode("utf-8"))
            os.close(fd)
            break
        except FileExistsError:
            if lock_path.exists() and time.time() - lock_path.stat().st_mtime > 1800:
                lock_path.unlink(missing_ok=True)
                continue
            time.sleep(0.2)
    try:
        yield
    finally:
        lock_path.unlink(missing_ok=True)


def ensure_runtime_bootstrap(root: Path, artifact_id: str) -> None:
    manifest = load_deps_manifest(root)
    platform = artifact_platform(artifact_id)
    with runtime_lock(root):
        for kind in RESOURCE_KINDS:
            resource = find_platform_resource(manifest, platform, kind)
            if not resource_has_complete_metadata(resource):
                raise RuntimeError(f"deps manifest resource is not bootstrap-ready: {resource}")
            try:
                resolve_prepared_entrypoints(root, resource)
                continue
            except RuntimeError:
                pass
            archive_path = download_runtime_archive(root, resource)
            extract_runtime_archive(root, resource, archive_path)
            resolve_prepared_entrypoints(root, resource)


def download_runtime_archive(root: Path, resource: dict[str, object]) -> Path:
    cache = cache_root(root)
    cache.mkdir(parents=True, exist_ok=True)
    archive_format = str(resource["archive_format"])
    archive_path = cache / f"{resource['id']}-{resource['version']}{ARCHIVE_SUFFIXES[archive_format]}"
    if archive_path.exists() and sha256_file(archive_path) == str(resource["sha256"]).lower():
        return archive_path

    temp_path = archive_path.with_suffix(archive_path.suffix + ".download")
    attempted: list[str] = []
    final_error: Exception | None = None
    for source in resource_sources(resource):
        url = source["url"]
        attempted.append(url)
        temp_path.unlink(missing_ok=True)
        try:
            with urllib.request.urlopen(url, timeout=60) as response:
                temp_path.write_bytes(response.read())
        except Exception as exc:  # noqa: BLE001
            final_error = RuntimeError(f"download runtime archive failed from {url}: {exc}")
            continue
        if sha256_file(temp_path) != str(resource["sha256"]).lower():
            temp_path.unlink(missing_ok=True)
            final_error = RuntimeError(f"runtime archive sha256 mismatch from {url}: {resource['id']}")
            continue
        temp_path.replace(archive_path)
        return archive_path
    if final_error is None:
        raise RuntimeError(f"runtime archive download failed: {resource['id']}")
    raise RuntimeError(f"{final_error}; attempted_sources={attempted}")


def extract_runtime_archive(root: Path, resource: dict[str, object], archive_path: Path) -> None:
    target_root = store_root(root, resource)
    target_root.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix=f"{resource['id']}-", dir=target_root.parent) as tmp:
        temp_root = Path(tmp)
        archive_format = str(resource["archive_format"])
        if archive_format == "zip":
            with zipfile.ZipFile(archive_path) as zf:
                zf.extractall(temp_root)
        else:
            with tarfile.open(archive_path, "r:*") as tf:
                tf.extractall(temp_root)
        if target_root.exists():
            shutil.rmtree(target_root, ignore_errors=True)
        temp_root.replace(target_root)


def sha256_file(path: Path) -> str:
    hasher = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            hasher.update(chunk)
    return hasher.hexdigest()


def stop_process(process: subprocess.Popen[str], *, timeout_seconds: int = 10) -> None:
    if process.poll() is not None:
        return
    with contextlib.suppress(ProcessLookupError):
        if os.name == "nt":
            process.terminate()
        else:
            process.send_signal(signal.SIGTERM)
    try:
        process.wait(timeout=timeout_seconds)
    except subprocess.TimeoutExpired:
        process.kill()
        process.wait(timeout=timeout_seconds)


def read_process_output(process: subprocess.Popen[str]) -> str:
    stop_process(process)
    stdout, stderr = process.communicate(timeout=5)
    return f"stdout:\n{stdout}\nstderr:\n{stderr}"
