#!/usr/bin/env python3
from __future__ import annotations

import contextlib
import os
import re
import signal
import socket
import subprocess
import tarfile
import zipfile
from pathlib import Path


SERVER_BINARIES = {
    "windows-x64-full": "raylea-server.exe",
    "linux-x64-full": "raylea-server",
    "macos-arm64-full": "raylea-server",
    "linux-x64-server": "raylea-server",
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
