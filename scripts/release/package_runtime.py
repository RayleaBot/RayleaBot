#!/usr/bin/env python3
from __future__ import annotations

import contextlib
from collections import deque
import hashlib
import json
import os
import signal
import sys
import socket
import subprocess
import shutil
import tarfile
import tempfile
import threading
import time
import urllib.request
from urllib.parse import urlsplit
import zipfile
from pathlib import Path, PurePosixPath
from typing import TextIO


sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from deps_manifest import validate_manifest
from archive_io import extract_archive, copy_bounded, MAX_ARCHIVE_BYTES
from jsonschema import ValidationError

from artifact_matrix import ARTIFACT_MATRIX, REQUIRED_PATHS, SERVER_BINARIES
from release_content import find_forbidden_paths


RESOURCE_KINDS = ("chromium", "ffmpeg")
REQUIRED_ENTRYPOINTS = {
    "chromium": ("browser",),
    "ffmpeg": ("ffmpeg", "ffprobe"),
}
SOURCE_KINDS = {"upstream", "mirror"}
ARCHIVE_SUFFIXES = {
    "zip": ".zip",
    "tar.gz": ".tar.gz",
    "tar.xz": ".tar.xz",
}
SOURCE_PROBE_BYTES = 1024 * 1024
SOURCE_PROBE_TIMEOUT_SECONDS = 8
SOURCE_PROBE_CLOSE_RATIO = 0.10

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


def ensure_required_paths(root: Path, artifact_id: str) -> None:
    missing = sorted(path for path in REQUIRED_PATHS[artifact_id] if not (root / path).exists())
    if missing:
        raise RuntimeError(f"missing required packaged paths: {missing}")
    ensure_no_forbidden_paths(root)


def ensure_no_forbidden_paths(root: Path) -> None:
    forbidden = find_forbidden_paths(root)
    if forbidden:
        raise RuntimeError(f"packaged archive contains development files: {forbidden}")


def choose_free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        return int(sock.getsockname()[1])


def write_user_config(root: Path, port: int) -> Path:
    user_path = root / "config" / "user.yaml"
    user_path.parent.mkdir(parents=True, exist_ok=True)
    user_path.write_text(f"server:\n  host: 127.0.0.1\n  port: {port}\n", encoding="utf-8")
    server_bin = root / ("raylea-server.exe" if (root / "raylea-server.exe").is_file() else "raylea-server")
    subprocess.run(
        server_base_command(server_bin) + ["config", "init"],
        cwd=root, check=True, capture_output=True, text=True, encoding="utf-8", timeout=120,
    )
    return user_path


def relative_executable(root: Path, artifact_id: str) -> Path:
    return root / SERVER_BINARIES[artifact_id]


def server_base_command(server_bin: Path) -> list[str]:
    return [
        str(server_bin),
        "-config",
        "config/user.yaml",
    ]


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
        try:
            parsed = urlsplit(url)
            if not parsed.hostname or parsed.username is not None or parsed.fragment:
                return False
        except ValueError:
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
    for source in select_runtime_download_sources(resource_sources(resource)):
        url = source["url"]
        attempted.append(url)
        temp_path.unlink(missing_ok=True)
        try:
            with urllib.request.urlopen(url, timeout=60) as response, temp_path.open("xb") as output:
                if hasattr(response, "geturl") and urlsplit(response.geturl()).scheme != "https":
                    raise RuntimeError("runtime download redirected outside HTTPS")
                expected = int(response.headers.get("Content-Length", -1)) if hasattr(response, "headers") else -1
                if expected > MAX_ARCHIVE_BYTES:
                    raise RuntimeError("runtime archive exceeds download size limit")
                copied = copy_bounded(response, output, MAX_ARCHIVE_BYTES)
                if expected >= 0 and copied != expected:
                    raise RuntimeError("runtime download size mismatch")
                output.flush()
                os.fsync(output.fileno())
        except Exception as exc:  # noqa: BLE001
            temp_path.unlink(missing_ok=True)
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


def select_runtime_download_sources(sources: list[dict[str, str]]) -> list[dict[str, str]]:
    if len(sources) <= 1:
        return sources
    results: list[dict[str, object] | None] = [None] * len(sources)
    threads: list[threading.Thread] = []
    for index, source in enumerate(sources):
        thread = threading.Thread(target=_probe_runtime_source_worker, args=(source, index, results), daemon=True)
        threads.append(thread)
        thread.start()
    deadline = time.monotonic() + SOURCE_PROBE_TIMEOUT_SECONDS + 4
    for thread in threads:
        remaining = max(0.0, deadline - time.monotonic())
        thread.join(remaining)
    if not any(item and item.get("ok") for item in results):
        return sources

    ranked = [
        item
        if item is not None
        else {"source": sources[index], "index": index, "ok": False, "bytes_per_second": 0.0}
        for index, item in enumerate(results)
    ]

    def compare_key(item: dict[str, object]) -> tuple[int, float, int]:
        ok = 1 if item.get("ok") else 0
        return (-ok, -float(item.get("bytes_per_second", 0.0)), int(item["index"]))

    ranked.sort(key=compare_key)
    ranked = _restore_close_probe_order(ranked)
    return [item["source"] for item in ranked if isinstance(item.get("source"), dict)]


def _probe_runtime_source_worker(
    source: dict[str, str],
    index: int,
    results: list[dict[str, object] | None],
) -> None:
    results[index] = probe_runtime_download_source(source, index)


def probe_runtime_download_source(source: dict[str, str], index: int) -> dict[str, object]:
    result: dict[str, object] = {
        "source": source,
        "index": index,
        "ok": False,
        "bytes_per_second": 0.0,
    }
    try:
        request = urllib.request.Request(source["url"], headers={"Range": f"bytes=0-{SOURCE_PROBE_BYTES - 1}"})
        started = time.monotonic()
        with urllib.request.urlopen(request, timeout=SOURCE_PROBE_TIMEOUT_SECONDS) as response:
            status = getattr(response, "status", 200)
            if status not in (200, 206):
                return result
            payload = response.read(SOURCE_PROBE_BYTES)
        elapsed = max(time.monotonic() - started, 0.001)
        if not payload:
            return result
        result["ok"] = True
        result["bytes_per_second"] = len(payload) / elapsed
        return result
    except Exception:  # noqa: BLE001
        return result


def _restore_close_probe_order(ranked: list[dict[str, object]]) -> list[dict[str, object]]:
    ordered: list[dict[str, object]] = []
    group: list[dict[str, object]] = []
    group_best = 0.0
    for item in ranked:
        if not item.get("ok"):
            if group:
                ordered.extend(sorted(group, key=lambda entry: int(entry["index"])))
                group = []
            ordered.append(item)
            continue
        speed = float(item.get("bytes_per_second", 0.0))
        if not group:
            group = [item]
            group_best = speed
            continue
        if group_best > 0 and abs(group_best - speed) / group_best <= SOURCE_PROBE_CLOSE_RATIO:
            group.append(item)
            continue
        ordered.extend(sorted(group, key=lambda entry: int(entry["index"])))
        group = [item]
        group_best = speed
    if group:
        ordered.extend(sorted(group, key=lambda entry: int(entry["index"])))
    return ordered


def extract_runtime_archive(root: Path, resource: dict[str, object], archive_path: Path) -> None:
    target_root = store_root(root, resource)
    target_root.parent.mkdir(parents=True, exist_ok=True)
    cleanup_stale_runtime_temp_roots(target_root.parent, resource)
    with tempfile.TemporaryDirectory(prefix=f"{resource['id']}-", dir=target_root.parent) as tmp:
        temp_root = Path(tmp)
        extract_archive(archive_path, temp_root, allow_links=True)
        for key in REQUIRED_ENTRYPOINTS[str(resource["kind"])]:
            if not any((temp_root / value).is_file() for value in resource["entrypoints"][key]):
                raise RuntimeError(f"runtime archive is missing required entrypoint: {key}")
        previous = None
        if target_root.exists():
            backup_root = Path(tempfile.mkdtemp(prefix=f".{resource['id']}-previous-", dir=target_root.parent))
            previous = backup_root / "store"
            replace_directory_with_retry(target_root, previous)
        try:
            replace_directory_with_retry(temp_root, target_root)
        except Exception:
            if previous is not None:
                replace_directory_with_retry(previous, target_root)
                previous.parent.rmdir()
            raise
        if previous is not None:
            shutil.rmtree(previous.parent)


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


def cleanup_stale_runtime_temp_roots(parent: Path, resource: dict[str, object]) -> None:
    prefixes = (
        f"{resource['id']}-{resource['version']}-",
        f".{resource['id']}-{resource['version']}-",
    )
    if not parent.exists():
        return
    for item in parent.iterdir():
        if item.is_dir() and any(item.name.startswith(prefix) for prefix in prefixes):
            shutil.rmtree(item, ignore_errors=True)


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


class _ProcessOutputCapture:
    def __init__(self, stdout: TextIO, stderr: TextIO, *, limit: int = 2 * 1024 * 1024) -> None:
        self._streams = {"stdout": stdout, "stderr": stderr}
        self._chunks = {"stdout": deque[str](), "stderr": deque[str]()}
        self._sizes = {"stdout": 0, "stderr": 0}
        self._limit = limit
        self._lock = threading.Lock()
        self._threads = [
            threading.Thread(target=self._drain, args=(name,), daemon=True)
            for name in ("stdout", "stderr")
        ]
        for thread in self._threads:
            thread.start()

    def _drain(self, name: str) -> None:
        stream = self._streams[name]
        try:
            for chunk in iter(stream.readline, ""):
                with self._lock:
                    chunks = self._chunks[name]
                    chunks.append(chunk)
                    self._sizes[name] += len(chunk)
                    while self._sizes[name] > self._limit and len(chunks) > 1:
                        self._sizes[name] -= len(chunks.popleft())
        finally:
            stream.close()

    def collect(self) -> tuple[str, str]:
        for thread in self._threads:
            thread.join(timeout=5)
        with self._lock:
            return ("".join(self._chunks["stdout"]), "".join(self._chunks["stderr"]))


def start_captured_process(
    command: list[str],
    *,
    cwd: Path,
    env: dict[str, str] | None = None,
) -> subprocess.Popen[str]:
    process = subprocess.Popen(
        command,
        cwd=cwd,
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        encoding="utf-8",
        errors="replace",
    )
    if process.stdout is None or process.stderr is None:
        process.kill()
        raise RuntimeError("captured process did not expose stdout and stderr")
    process._raylea_output_capture = _ProcessOutputCapture(process.stdout, process.stderr)  # type: ignore[attr-defined]
    return process


def read_process_output(process: subprocess.Popen[str]) -> str:
    stop_process(process)
    capture = getattr(process, "_raylea_output_capture", None)
    if isinstance(capture, _ProcessOutputCapture):
        stdout, stderr = capture.collect()
    else:
        stdout, stderr = process.communicate(timeout=5)
    return f"stdout:\n{stdout}\nstderr:\n{stderr}"
