#!/usr/bin/env python3
from __future__ import annotations

import argparse
import contextlib
import http.client
import json
import os
import re
import shutil
import signal
import socket
import sqlite3
import subprocess
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

SAMPLE_PLUGIN_ID = "recovery-sample"
SAMPLE_PLUGIN_INFO = {
    "id": SAMPLE_PLUGIN_ID,
    "name": "Recovery Sample",
    "version": "1.0.0",
    "manifest_version": "1",
    "plugin_protocol_version": "1",
    "type": "managed_runtime",
    "runtime": "python",
    "entry": "plugin.py",
    "license": "MIT",
    "description": "Sample plugin manifest used by packaged recovery drill.",
    "author": "raylea",
    "role": "user",
    "capabilities": ["event.subscribe", "logger.write"],
    "permissions": {
        "required": [],
        "optional": [],
    },
}


class DrillError(RuntimeError):
    pass


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
        raise DrillError("archive is empty")
    root_name = Path(names[0]).parts[0]
    root = destination / root_name
    if not root.is_dir():
        raise DrillError(f"release root not found after extraction: {root}")
    return root


def ensure_required_paths(root: Path, artifact_id: str) -> None:
    missing = sorted(path for path in REQUIRED_PATHS[artifact_id] if not (root / path).exists())
    if missing:
        raise DrillError(f"missing required packaged paths: {missing}")


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
        raise DrillError("failed to rewrite server.port in default config")
    user_path.write_text(updated, encoding="utf-8")
    return user_path


def seed_database(root: Path) -> Path:
    database_path = root / "data" / "rayleabot.db"
    database_path.parent.mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(database_path)
    try:
        connection.execute("CREATE TABLE IF NOT EXISTS recovery_seed (value TEXT NOT NULL)")
        connection.execute("DELETE FROM recovery_seed")
        connection.execute("INSERT INTO recovery_seed(value) VALUES (?)", ("seeded",))
        connection.commit()
    finally:
        connection.close()
    return database_path


def seed_installed_plugin(root: Path) -> Path:
    plugin_dir = root / "plugins" / "installed" / SAMPLE_PLUGIN_ID
    plugin_dir.mkdir(parents=True, exist_ok=True)
    info_path = plugin_dir / "info.json"
    info_path.write_text(json.dumps(SAMPLE_PLUGIN_INFO, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    (plugin_dir / "plugin.py").write_text("print('recovery sample')\n", encoding="utf-8")
    return info_path


def seed_runtime_workspace(root: Path) -> tuple[Path, Path, Path]:
    port = choose_free_port()
    config_path = write_user_config(root, port)
    database_path = seed_database(root)
    plugin_info_path = seed_installed_plugin(root)
    return config_path, database_path, plugin_info_path


def relative_executable(root: Path, artifact_id: str) -> Path:
    return root / SERVER_BINARIES[artifact_id]


def run_command(root: Path, command: list[str], *, timeout: int = 120) -> subprocess.CompletedProcess[str]:
    result = subprocess.run(
        command,
        cwd=root,
        text=True,
        capture_output=True,
        timeout=timeout,
        check=False,
    )
    if result.returncode != 0:
        raise DrillError(
            f"command failed ({result.returncode}): {' '.join(command)}\nstdout:\n{result.stdout}\nstderr:\n{result.stderr}"
        )
    return result


def run_doctor(root: Path, server_bin: Path) -> None:
    run_command(
        root,
        [
            str(server_bin),
            "-config",
            "config/user.yaml",
            "-config-schema",
            "contracts/config.user.schema.json",
            "doctor",
        ],
    )


def run_backup(root: Path, server_bin: Path) -> Path:
    run_command(
        root,
        [
            str(server_bin),
            "-config",
            "config/user.yaml",
            "-config-schema",
            "contracts/config.user.schema.json",
            "backup",
        ],
    )
    backups = sorted((root / "backups").glob("backup-*.zip"))
    if not backups:
        raise DrillError("backup command did not create a backup archive")
    backup_path = backups[-1]
    with zipfile.ZipFile(backup_path) as zf:
        names = set(zf.namelist())
    if "backup-manifest.json" not in names:
        raise DrillError("backup archive missing backup-manifest.json")
    return backup_path


def overwrite_runtime_state(config_path: Path, database_path: Path, plugin_info_path: Path) -> None:
    config_path.write_text("schema_version: \"2\"\nserver:\n  host: 127.0.0.1\n  port: 1\n", encoding="utf-8")
    database_path.write_bytes(b"corrupted")
    plugin_info_path.unlink()


def run_restore(root: Path, server_bin: Path, backup_path: Path) -> None:
    run_command(
        root,
        [
            str(server_bin),
            "-config",
            "config/user.yaml",
            "-config-schema",
            "contracts/config.user.schema.json",
            "restore",
            str(backup_path),
        ],
    )


def assert_restored(config_path: Path, database_path: Path, plugin_info_path: Path, expected: tuple[bytes, bytes, bytes]) -> None:
    actual = (
        config_path.read_bytes(),
        database_path.read_bytes(),
        plugin_info_path.read_bytes(),
    )
    if actual != expected:
        raise DrillError("restored runtime state does not match backup snapshot")


def wait_for_http(url: str, *, timeout_seconds: int = 20, expect_substring: str | None = None) -> None:
    deadline = time.time() + timeout_seconds
    last_error: Exception | None = None
    while time.time() < deadline:
        try:
            with urllib.request.urlopen(url, timeout=2) as response:
                body = response.read().decode("utf-8", errors="replace")
            if expect_substring is not None and expect_substring not in body:
                raise DrillError(f"response from {url} did not contain expected substring {expect_substring!r}")
            return
        except Exception as exc:  # noqa: BLE001
            last_error = exc
            time.sleep(0.5)
    raise DrillError(f"timed out waiting for {url}: {last_error}")


@contextlib.contextmanager
def running_server(root: Path, server_bin: Path):
    command = [
        str(server_bin),
        "-config",
        "config/user.yaml",
        "-config-schema",
        "contracts/config.user.schema.json",
    ]
    process = subprocess.Popen(
        command,
        cwd=root,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    try:
        yield process
    finally:
        stop_process(process)


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


def read_server_output(process: subprocess.Popen[str]) -> str:
    stop_process(process)
    stdout, stderr = process.communicate(timeout=5)
    return f"stdout:\n{stdout}\nstderr:\n{stderr}"


def probe_server(root: Path, server_bin: Path, port: int) -> None:
    with running_server(root, server_bin) as process:
        health_url = f"http://127.0.0.1:{port}/healthz"
        index_url = f"http://127.0.0.1:{port}/"
        deadline = time.time() + 20
        while time.time() < deadline:
            if process.poll() is not None:
                raise DrillError(f"server exited before health probe succeeded\n{read_server_output(process)}")
            try:
                wait_for_http(health_url, timeout_seconds=1)
                wait_for_http(index_url, timeout_seconds=1, expect_substring="<html")
                return
            except Exception:  # noqa: BLE001
                time.sleep(0.5)
        raise DrillError(f"timed out waiting for packaged server probes\n{read_server_output(process)}")


def extract_configured_port(config_path: Path) -> int:
    match = re.search(r"(?m)^\s*port:\s*(\d+)\s*$", config_path.read_text(encoding="utf-8"))
    if match is None:
        raise DrillError("failed to read configured server.port from config/user.yaml")
    return int(match.group(1))


def run_recovery_drill(artifact_id: str, archive_path: Path) -> None:
    with tempfile.TemporaryDirectory(prefix="rayleabot-recovery-") as tmp:
        temp_root = Path(tmp)
        release_root = unpack_archive(artifact_id, archive_path, temp_root)
        ensure_required_paths(release_root, artifact_id)
        config_path, database_path, plugin_info_path = seed_runtime_workspace(release_root)
        expected_snapshot = (
            config_path.read_bytes(),
            database_path.read_bytes(),
            plugin_info_path.read_bytes(),
        )
        server_bin = relative_executable(release_root, artifact_id)
        if not server_bin.exists():
            raise DrillError(f"server executable missing: {server_bin}")

        run_doctor(release_root, server_bin)
        backup_path = run_backup(release_root, server_bin)
        overwrite_runtime_state(config_path, database_path, plugin_info_path)
        run_restore(release_root, server_bin, backup_path)
        assert_restored(config_path, database_path, plugin_info_path, expected_snapshot)
        run_doctor(release_root, server_bin)
        probe_server(release_root, server_bin, extract_configured_port(config_path))


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="RayleaBot packaged recovery drill")
    parser.add_argument("--artifact-id", required=True, choices=sorted(REQUIRED_PATHS.keys()))
    parser.add_argument("--archive", required=True)
    return parser


def main() -> int:
    args = build_parser().parse_args()
    run_recovery_drill(args.artifact_id, Path(args.archive))
    print("recovery drill passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
