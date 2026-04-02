#!/usr/bin/env python3
from __future__ import annotations

import argparse
import contextlib
import io
import json
import os
import re
import shutil
import sqlite3
import subprocess
import tarfile
import tempfile
import time
import urllib.error
import urllib.request
import zipfile
from pathlib import Path

import self_host_smoke
from package_runtime import (
    REQUIRED_PATHS,
    choose_free_port,
    ensure_required_paths,
    ensure_runtime_bootstrap,
    read_process_output,
    relative_executable,
    server_base_command,
    stop_process,
    unpack_archive,
    write_user_config,
)

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
INCOMPATIBLE_PLUGIN_ID = "recovery-incompatible"
INCOMPATIBLE_PLUGIN_INFO = {
    "id": INCOMPATIBLE_PLUGIN_ID,
    "name": "Recovery Incompatible Sample",
    "version": "1.0.0",
    "manifest_version": "1",
    "plugin_protocol_version": "1",
    "type": "managed_runtime",
    "runtime": "python",
    "entry": "plugin.py",
    "license": "MIT",
    "description": "Sample incompatible plugin manifest used by packaged recovery drill.",
    "author": "raylea",
    "role": "user",
    "capabilities": ["event.subscribe"],
    "permissions": {
        "required": [],
        "optional": [],
    },
    "platforms": ["unsupported-platform"],
}


class DrillError(RuntimeError):
    pass


class DrillBootstrapSkip(RuntimeError):
    pass


DEFAULT_OBSERVATION_WINDOW_SECONDS = 300


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


def seed_installed_plugins(root: Path, *, include_incompatible: bool = True) -> list[Path]:
    manifests = []
    inventory = [SAMPLE_PLUGIN_INFO]
    if include_incompatible:
        inventory.append(INCOMPATIBLE_PLUGIN_INFO)
    for manifest in inventory:
        plugin_dir = root / "plugins" / "installed" / str(manifest["id"])
        plugin_dir.mkdir(parents=True, exist_ok=True)
        info_path = plugin_dir / "info.json"
        info_path.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        (plugin_dir / "plugin.py").write_text("print('recovery sample')\n", encoding="utf-8")
        manifests.append(info_path)
    return manifests


def seed_runtime_workspace(root: Path, *, include_incompatible: bool = True) -> tuple[Path, Path, list[Path]]:
    port = choose_free_port()
    config_path = write_user_config(root, port)
    database_path = seed_database(root)
    plugin_info_paths = seed_installed_plugins(root, include_incompatible=include_incompatible)
    return config_path, database_path, plugin_info_paths


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


def run_command_allow_failure(root: Path, command: list[str], *, timeout: int = 120) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        command,
        cwd=root,
        text=True,
        capture_output=True,
        timeout=timeout,
        check=False,
    )


def run_doctor(root: Path, server_bin: Path) -> None:
    run_command(
        root,
        [
            *server_base_command(server_bin),
            "doctor",
        ],
    )


def run_backup(root: Path, server_bin: Path) -> Path:
    run_command(
        root,
        [
            *server_base_command(server_bin),
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


def overwrite_runtime_state(config_path: Path, database_path: Path, plugin_info_paths: list[Path]) -> None:
    config_path.write_text("schema_version: \"2\"\nserver:\n  host: 127.0.0.1\n  port: 1\n", encoding="utf-8")
    database_path.write_bytes(b"corrupted")
    for plugin_info_path in plugin_info_paths:
        if plugin_info_path.exists():
            plugin_info_path.unlink()


def run_restore(root: Path, server_bin: Path, backup_path: Path) -> None:
    result = run_restore_capture(root, server_bin, backup_path)
    if result.returncode != 0:
        raise DrillError(
            f"restore failed ({result.returncode})\nstdout:\n{result.stdout}\nstderr:\n{result.stderr}"
        )


def run_restore_capture(root: Path, server_bin: Path, backup_path: Path) -> subprocess.CompletedProcess[str]:
    return run_command_allow_failure(
        root,
        [
            *server_base_command(server_bin),
            "restore",
            str(backup_path),
        ],
    )


def snapshot_runtime_state(root: Path) -> dict[str, object]:
    plugin_snapshots: dict[str, bytes] = {}
    plugins_root = root / "plugins" / "installed"
    if plugins_root.exists():
        for info_path in sorted(plugins_root.glob("*/info.json")):
            plugin_snapshots[info_path.relative_to(root).as_posix()] = info_path.read_bytes()
    return {
        "config/user.yaml": (root / "config" / "user.yaml").read_bytes(),
        "data/rayleabot.db": (root / "data" / "rayleabot.db").read_bytes(),
        "plugins": plugin_snapshots,
    }


def assert_restored(root: Path, expected: dict[str, object]) -> None:
    actual = snapshot_runtime_state(root)
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
        *server_base_command(server_bin),
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


def probe_server(root: Path, server_bin: Path, port: int) -> None:
    with running_server(root, server_bin) as process:
        health_url = f"http://127.0.0.1:{port}/healthz"
        index_url = f"http://127.0.0.1:{port}/"
        deadline = time.time() + 20
        while time.time() < deadline:
            if process.poll() is not None:
                raise DrillError(f"server exited before health probe succeeded\n{read_process_output(process)}")
            try:
                wait_for_http(health_url, timeout_seconds=1)
                wait_for_http(index_url, timeout_seconds=1, expect_substring="<html")
                return
            except Exception:  # noqa: BLE001
                time.sleep(0.5)
        raise DrillError(f"timed out waiting for packaged server probes\n{read_process_output(process)}")


def read_server_output(process: subprocess.Popen[str]) -> str:
    return read_process_output(process)


def extract_configured_port(config_path: Path) -> int:
    match = re.search(r"(?m)^\s*port:\s*(\d+)\s*$", config_path.read_text(encoding="utf-8"))
    if match is None:
        raise DrillError("failed to read configured server.port from config/user.yaml")
    return int(match.group(1))


def read_json_file(path: Path) -> dict[str, object]:
    return json.loads(path.read_text(encoding="utf-8"))


def read_recovery_summary(root: Path) -> dict[str, object]:
    summary_path = root / "logs" / "recovery-summary.json"
    if not summary_path.exists():
        raise DrillError(f"recovery summary not found: {summary_path}")
    return read_json_file(summary_path)


def archive_members(artifact_id: str, archive_path: Path) -> list[str]:
    if artifact_id == "windows-x64-full":
        with zipfile.ZipFile(archive_path) as zf:
            return [name for name in zf.namelist() if name]
    with tarfile.open(archive_path, "r:gz") as tf:
        return [member.name for member in tf.getmembers() if member.name]


def archive_root_name(artifact_id: str, archive_path: Path) -> str:
    names = archive_members(artifact_id, archive_path)
    if not names:
        raise DrillError(f"archive is empty: {archive_path}")
    return Path(names[0]).parts[0]


def read_archive_json(artifact_id: str, archive_path: Path, relative_path: str) -> dict[str, object]:
    member_name = f"{archive_root_name(artifact_id, archive_path)}/{relative_path}"
    if artifact_id == "windows-x64-full":
        with zipfile.ZipFile(archive_path) as zf:
            with zf.open(member_name) as handle:
                return json.loads(handle.read().decode("utf-8"))
    with tarfile.open(archive_path, "r:gz") as tf:
        member = tf.getmember(member_name)
        handle = tf.extractfile(member)
        if handle is None:
            raise DrillError(f"archive member not found: {member_name}")
        with handle:
            return json.loads(handle.read().decode("utf-8"))


def read_build_info_from_archive(artifact_id: str, archive_path: Path) -> dict[str, object]:
    build_info = read_archive_json(artifact_id, archive_path, "build_info.json")
    if str(build_info.get("artifact_id", "")) != artifact_id:
        raise DrillError(f"archive build_info artifact_id mismatch: {archive_path}")
    return build_info


def semver_parts(version: str) -> tuple[int, int, int]:
    cleaned = version.strip()
    if not cleaned:
        return (0, 0, 0)
    for marker in ("-", "+"):
        cleaned = cleaned.split(marker, 1)[0]
    parts = []
    for item in cleaned.split(".")[:3]:
        try:
            parts.append(int(item.strip()))
        except ValueError:
            parts.append(0)
    while len(parts) < 3:
        parts.append(0)
    return tuple(parts[:3])


def compare_versions(left: str, right: str) -> int:
    lp = semver_parts(left)
    rp = semver_parts(right)
    if lp < rp:
        return -1
    if lp > rp:
        return 1
    return 0


def release_api_url(repository: str) -> str:
    return f"https://api.github.com/repos/{repository}/releases"


def github_api_headers() -> dict[str, str]:
    headers = {
        "Accept": "application/vnd.github+json",
        "User-Agent": "rayleabot-recovery-drill",
    }
    token = os.environ.get("GITHUB_TOKEN", "").strip() or os.environ.get("GH_TOKEN", "").strip()
    if token:
        headers["Authorization"] = f"Bearer {token}"
    return headers


def find_release_asset(release: dict[str, object], asset_name: str) -> dict[str, object] | None:
    for asset in release.get("assets", []):
        if not isinstance(asset, dict):
            continue
        if str(asset.get("name", "")) == asset_name:
            return asset
    return None


def select_previous_release(releases: list[dict[str, object]], current_version: str) -> dict[str, object] | None:
    current_tag = f"v{current_version}"
    for release in releases:
        if release.get("draft") or release.get("prerelease"):
            continue
        tag_name = str(release.get("tag_name", ""))
        if tag_name == current_tag:
            continue
        if find_release_asset(release, "release_manifest.json") is not None:
            return release
    return None


def download_asset(asset: dict[str, object], target: Path) -> Path:
    target.parent.mkdir(parents=True, exist_ok=True)
    with urllib.request.urlopen(str(asset["browser_download_url"]), timeout=120) as response, target.open("wb") as handle:
        shutil.copyfileobj(response, handle)
    return target


def download_previous_archive(repository: str, current_version: str, artifact_id: str, download_dir: Path) -> Path:
    request = urllib.request.Request(release_api_url(repository), headers=github_api_headers())
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            releases = json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        if exc.code in {403, 404}:
            raise DrillBootstrapSkip(
                f"release api is not accessible for {artifact_id} (HTTP {exc.code})"
            ) from exc
        raise
    release = select_previous_release(releases, current_version)
    if release is None:
        raise DrillBootstrapSkip(f"no previous published release found for {artifact_id}")
    download_dir.mkdir(parents=True, exist_ok=True)
    release_tag = str(release.get("tag_name", "")).strip() or "previous"
    manifest_asset = find_release_asset(release, "release_manifest.json")
    if manifest_asset is None:
        raise DrillBootstrapSkip(f"previous release {release_tag} does not include release_manifest.json")
    manifest_path = download_asset(manifest_asset, download_dir / release_tag / "release_manifest.json")
    manifest = read_json_file(manifest_path)
    artifact_record = next(
        (
            item
            for item in manifest.get("artifacts", [])
            if isinstance(item, dict) and str(item.get("artifact_id", "")) == artifact_id
        ),
        None,
    )
    if artifact_record is None:
        raise DrillBootstrapSkip(f"previous release {release_tag} does not include artifact {artifact_id}")
    asset = find_release_asset(release, str(artifact_record.get("file_name", "")))
    if asset is None:
        raise DrillBootstrapSkip(f"previous release {release_tag} is missing archive asset for {artifact_id}")
    return download_asset(asset, download_dir / release_tag / str(asset["name"]))


def assert_recovery_summary(
    summary: dict[str, object],
    *,
    expected_operation: str | set[str],
    expected_phase: str,
    expected_statuses: set[str],
    requires_post_start_checks: bool,
    require_skipped_plugin: bool = False,
    require_guidance: bool | None = None,
) -> None:
    expected_operations = (
        {expected_operation} if isinstance(expected_operation, str) else set(expected_operation)
    )
    if str(summary.get("operation", "")) not in expected_operations:
        raise DrillError(f"unexpected recovery summary operation: {summary}")
    if str(summary.get("phase", "")) != expected_phase:
        raise DrillError(f"unexpected recovery summary phase: {summary}")
    if str(summary.get("status", "")) not in expected_statuses:
        raise DrillError(f"unexpected recovery summary status: {summary}")
    if bool(summary.get("requires_post_start_checks")) != requires_post_start_checks:
        raise DrillError(f"unexpected recovery summary post-start flag: {summary}")
    if require_skipped_plugin:
        skipped_plugins = summary.get("skipped_plugins", [])
        plugin_ids = {str(item.get("plugin_id", "")) for item in skipped_plugins if isinstance(item, dict)}
        if INCOMPATIBLE_PLUGIN_ID not in plugin_ids:
            raise DrillError(f"expected skipped plugin {INCOMPATIBLE_PLUGIN_ID} in recovery summary: {summary}")
    if require_guidance is True:
        manual_actions = summary.get("manual_actions", [])
        next_steps = summary.get("next_steps", [])
        if not isinstance(manual_actions, list) or len(manual_actions) == 0:
            raise DrillError(f"expected manual_actions in degraded recovery summary: {summary}")
        if not isinstance(next_steps, list) or len(next_steps) == 0:
            raise DrillError(f"expected next_steps in degraded recovery summary: {summary}")
    if require_guidance is False:
        if summary.get("manual_actions") or summary.get("next_steps") or summary.get("skipped_plugins"):
            raise DrillError(f"compatible recovery summary must not retain manual guidance: {summary}")


def canonical_summary(summary: dict[str, object]) -> dict[str, object]:
    return {
        "status": summary.get("status"),
        "phase": summary.get("phase"),
        "operation": summary.get("operation"),
        "issues": summary.get("issues", []),
        "skipped_plugins": summary.get("skipped_plugins", []),
        "manual_actions": summary.get("manual_actions", []),
        "next_steps": summary.get("next_steps", []),
    }


def extract_diagnostics_recovery_summary(payload: bytes) -> dict[str, object]:
    with zipfile.ZipFile(io.BytesIO(payload)) as zf:
        try:
            return json.loads(zf.read("recovery-summary.json").decode("utf-8"))
        except KeyError as exc:
            raise DrillError("diagnostics export missing recovery-summary.json") from exc


def observe_recovery_window(
    root: Path,
    process: subprocess.Popen[str],
    port: int,
    *,
    expected_operation: str | set[str],
    expected_statuses: set[str],
    require_skipped_plugin: bool,
    require_guidance: bool | None,
    observation_window_seconds: int,
    allow_bootstrap: bool,
) -> None:
    base_url = f"http://127.0.0.1:{port}/"
    self_host_smoke.wait_for_management_state(
        root,
        process,
        base_url,
        allowed_ready_statuses=self_host_smoke.STARTUP_READY_STATUSES if allow_bootstrap else self_host_smoke.MANAGED_READY_STATUSES,
    )
    if allow_bootstrap:
        self_host_smoke.bootstrap_admin(base_url)
    session_token = self_host_smoke.login(base_url)

    baseline: dict[str, object] | None = None
    midpoint = time.time() + max(observation_window_seconds / 2, 1)
    deadline = time.time() + max(observation_window_seconds, 1)
    diagnostics_checked = False
    while time.time() < deadline:
        ready_body = self_host_smoke.request_json(f"{base_url}readyz")
        if str(ready_body.get("status", "")) not in self_host_smoke.MANAGED_READY_STATUSES:
            raise DrillError(f"readyz returned blocking status during recovery observation: {ready_body}")

        status_body = self_host_smoke.request_json(
            f"{base_url}api/system/status",
            headers=self_host_smoke.bearer_headers(session_token),
        )
        api_summary = status_body.get("recovery_summary")
        if not isinstance(api_summary, dict):
            raise DrillError(f"system status missing recovery_summary during observation: {status_body}")
        file_summary = read_recovery_summary(root)
        assert_recovery_summary(
            api_summary,
            expected_operation=expected_operation,
            expected_phase="post_startup",
            expected_statuses=expected_statuses,
            requires_post_start_checks=False,
            require_skipped_plugin=require_skipped_plugin,
            require_guidance=require_guidance,
        )
        current = canonical_summary(file_summary)
        if baseline is None:
            baseline = current
        if canonical_summary(api_summary) != baseline or current != baseline:
            raise DrillError("recovery summary drifted between API and local file during observation")

        if time.time() >= midpoint and not diagnostics_checked:
            payload = self_host_smoke.request_bytes(
                f"{base_url}api/system/diagnostics/export",
                headers=self_host_smoke.bearer_headers(session_token),
            )
            self_host_smoke.validate_diagnostics_archive(payload)
            diagnostics_summary = extract_diagnostics_recovery_summary(payload)
            if canonical_summary(diagnostics_summary) != baseline:
                raise DrillError("diagnostics recovery summary drifted from API/file state")
            diagnostics_checked = True
        time.sleep(min(5, max(deadline - time.time(), 0)))

    if baseline is None:
        raise DrillError("recovery observation window completed without a summary baseline")

    self_host_smoke.graceful_shutdown(base_url, session_token, process)


def run_server_observation(
    root: Path,
    server_bin: Path,
    port: int,
    *,
    expected_operation: str | set[str],
    expected_statuses: set[str],
    require_skipped_plugin: bool,
    require_guidance: bool | None,
    observation_window_seconds: int,
) -> None:
    process = subprocess.Popen(
        [*server_base_command(server_bin)],
        cwd=root,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    try:
        observe_recovery_window(
            root,
            process,
            port,
            expected_operation=expected_operation,
            expected_statuses=expected_statuses,
            require_skipped_plugin=require_skipped_plugin,
            require_guidance=require_guidance,
            observation_window_seconds=observation_window_seconds,
            allow_bootstrap=True,
        )
    finally:
        if process.poll() is None:
            stop_process(process)

    restarted = subprocess.Popen(
        [*server_base_command(server_bin)],
        cwd=root,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    try:
        observe_recovery_window(
            root,
            restarted,
            port,
            expected_operation=expected_operation,
            expected_statuses=expected_statuses,
            require_skipped_plugin=require_skipped_plugin,
            require_guidance=require_guidance,
            observation_window_seconds=max(30, observation_window_seconds // 2),
            allow_bootstrap=False,
        )
    finally:
        if restarted.poll() is None:
            stop_process(restarted)


def run_recovery_drill(artifact_id: str, archive_path: Path, *, observation_window_seconds: int) -> None:
    with tempfile.TemporaryDirectory(prefix="rayleabot-recovery-") as tmp:
        temp_root = Path(tmp)
        for scenario_name, include_incompatible, expected_status, require_guidance in [
            ("compatible", False, "compatible", False),
            ("degraded", True, "degraded", True),
        ]:
            release_root = unpack_archive(artifact_id, archive_path, temp_root / scenario_name)
            ensure_required_paths(release_root, artifact_id)
            ensure_runtime_bootstrap(release_root, artifact_id)
            config_path, database_path, plugin_info_paths = seed_runtime_workspace(
                release_root,
                include_incompatible=include_incompatible,
            )
            expected_snapshot = snapshot_runtime_state(release_root)
            server_bin = relative_executable(release_root, artifact_id)
            if not server_bin.exists():
                raise DrillError(f"server executable missing: {server_bin}")

            run_doctor(release_root, server_bin)
            backup_path = run_backup(release_root, server_bin)
            overwrite_runtime_state(config_path, database_path, plugin_info_paths)
            run_restore(release_root, server_bin, backup_path)
            assert_restored(release_root, expected_snapshot)
            assert_recovery_summary(
                read_recovery_summary(release_root),
                expected_operation="restore",
                expected_phase="pre_restore",
                expected_statuses={"pending"},
                requires_post_start_checks=True,
            )
            run_server_observation(
                release_root,
                server_bin,
                extract_configured_port(config_path),
                expected_operation="restore",
                expected_statuses={expected_status},
                require_skipped_plugin=include_incompatible,
                require_guidance=require_guidance,
                observation_window_seconds=observation_window_seconds,
            )
            assert_recovery_summary(
                read_recovery_summary(release_root),
                expected_operation="restore",
                expected_phase="post_startup",
                expected_statuses={expected_status},
                requires_post_start_checks=False,
                require_skipped_plugin=include_incompatible,
                require_guidance=require_guidance,
            )
            run_doctor(release_root, server_bin)


def run_cross_version_recovery_drill(
    artifact_id: str,
    previous_archive: Path,
    current_archive: Path,
    *,
    observation_window_seconds: int,
) -> None:
    previous_build = read_build_info_from_archive(artifact_id, previous_archive)
    current_build = read_build_info_from_archive(artifact_id, current_archive)
    previous_version = str(previous_build.get("version", "")).strip()
    current_version = str(current_build.get("version", "")).strip()
    if compare_versions(previous_version, current_version) >= 0:
        raise DrillError(
            f"previous archive must be older than current archive for cross-version drill: {previous_version} !< {current_version}"
        )

    with tempfile.TemporaryDirectory(prefix="rayleabot-recovery-pair-") as tmp:
        temp_root = Path(tmp)
        for scenario_name, include_incompatible, expected_status, require_guidance in [
            ("compatible", False, "compatible", False),
            ("degraded", True, "degraded", True),
        ]:
            previous_root = unpack_archive(artifact_id, previous_archive, temp_root / f"{scenario_name}-previous")
            current_root = unpack_archive(artifact_id, current_archive, temp_root / f"{scenario_name}-current")
            rollback_root = unpack_archive(artifact_id, previous_archive, temp_root / f"{scenario_name}-rollback")
            ensure_required_paths(previous_root, artifact_id)
            ensure_required_paths(current_root, artifact_id)
            ensure_required_paths(rollback_root, artifact_id)
            ensure_runtime_bootstrap(previous_root, artifact_id)
            ensure_runtime_bootstrap(current_root, artifact_id)
            ensure_runtime_bootstrap(rollback_root, artifact_id)

            previous_server = relative_executable(previous_root, artifact_id)
            current_server = relative_executable(current_root, artifact_id)
            rollback_server = relative_executable(rollback_root, artifact_id)

            # Upgrade flow: restore a previous backup into the current build.
            _, _, _ = seed_runtime_workspace(previous_root, include_incompatible=include_incompatible)
            previous_backup = run_backup(previous_root, previous_server)
            run_restore(current_root, current_server, previous_backup)
            assert_recovery_summary(
                read_recovery_summary(current_root),
                expected_operation="upgrade",
                expected_phase="pre_restore",
                expected_statuses={"pending"},
                requires_post_start_checks=True,
            )
            run_server_observation(
                current_root,
                current_server,
                extract_configured_port(current_root / "config" / "user.yaml"),
                expected_operation="upgrade",
                expected_statuses={expected_status},
                require_skipped_plugin=include_incompatible,
                require_guidance=require_guidance,
                observation_window_seconds=observation_window_seconds,
            )
            assert_recovery_summary(
                read_recovery_summary(current_root),
                expected_operation="upgrade",
                expected_phase="post_startup",
                expected_statuses={expected_status},
                requires_post_start_checks=False,
                require_skipped_plugin=include_incompatible,
                require_guidance=require_guidance,
            )
            run_doctor(current_root, current_server)

            # Rollback-style flow: restore the pre-upgrade backup with the older packaged build.
            rollback_config, rollback_db, rollback_plugin_paths = seed_runtime_workspace(
                rollback_root,
                include_incompatible=include_incompatible,
            )
            expected_snapshot = snapshot_runtime_state(previous_root)
            overwrite_runtime_state(rollback_config, rollback_db, rollback_plugin_paths)
            run_restore(rollback_root, rollback_server, previous_backup)
            assert_restored(rollback_root, expected_snapshot)
            assert_recovery_summary(
                read_recovery_summary(rollback_root),
                expected_operation={"restore", "rollback"},
                expected_phase="pre_restore",
                expected_statuses={"pending"},
                requires_post_start_checks=True,
            )
            run_server_observation(
                rollback_root,
                rollback_server,
                extract_configured_port(rollback_root / "config" / "user.yaml"),
                expected_operation={"restore", "rollback"},
                expected_statuses={expected_status},
                require_skipped_plugin=include_incompatible,
                require_guidance=require_guidance,
                observation_window_seconds=observation_window_seconds,
            )
            assert_recovery_summary(
                read_recovery_summary(rollback_root),
                expected_operation={"restore", "rollback"},
                expected_phase="post_startup",
                expected_statuses={expected_status},
                requires_post_start_checks=False,
                require_skipped_plugin=include_incompatible,
                require_guidance=require_guidance,
            )
            run_doctor(rollback_root, rollback_server)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="RayleaBot packaged recovery drill")
    parser.add_argument("--artifact-id", required=True, choices=sorted(REQUIRED_PATHS.keys()))
    parser.add_argument("--archive", required=True)
    parser.add_argument("--previous-archive")
    parser.add_argument("--repository")
    parser.add_argument("--current-version")
    parser.add_argument("--download-dir")
    parser.add_argument("--observation-window-seconds", type=int, default=DEFAULT_OBSERVATION_WINDOW_SECONDS)
    return parser


def main() -> int:
    args = build_parser().parse_args()
    try:
        if args.previous_archive:
            run_cross_version_recovery_drill(
                args.artifact_id,
                Path(args.previous_archive),
                Path(args.archive),
                observation_window_seconds=args.observation_window_seconds,
            )
            print("cross-version recovery drill passed")
            return 0
        if args.repository and args.current_version and args.download_dir:
            previous_archive = download_previous_archive(
                args.repository,
                args.current_version,
                args.artifact_id,
                Path(args.download_dir),
            )
            run_cross_version_recovery_drill(
                args.artifact_id,
                previous_archive,
                Path(args.archive),
                observation_window_seconds=args.observation_window_seconds,
            )
            print("cross-version recovery drill passed")
            return 0
        run_recovery_drill(
            args.artifact_id,
            Path(args.archive),
            observation_window_seconds=args.observation_window_seconds,
        )
        print("recovery drill passed")
    except DrillBootstrapSkip as exc:
        print(f"recovery drill bootstrap skip: {exc}")
        return 0
    except (DrillError, RuntimeError) as exc:
        print(f"recovery drill failed: {exc}")
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
