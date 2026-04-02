#!/usr/bin/env python3
from __future__ import annotations

import argparse
import contextlib
import io
import json
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
import zipfile
from pathlib import Path

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


SETUP_IDENTIFIER = "admin"
SETUP_SECRET = "fixture-only-secret"
DIAGNOSTICS_REQUIRED_ENTRIES = {"system-status.json", "readiness.json", "doctor.json"}
STARTUP_READY_STATUSES = {"ready", "degraded", "setup_required"}
MANAGED_READY_STATUSES = {"ready", "degraded"}
NON_BLOCKING_RECOVERY_STATUSES = {"compatible", "degraded"}


class SmokeError(RuntimeError):
    pass


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="RayleaBot long self-host smoke check")
    parser.add_argument("--artifact-id", required=True, choices=sorted(REQUIRED_PATHS.keys()))
    parser.add_argument("--archive", required=True)
    parser.add_argument("--window-seconds", type=int, default=600)
    parser.add_argument("--probe-interval-seconds", type=int, default=30)
    return parser


def ensure_monotonic_uptime(previous: int, current: int) -> int:
    if current < previous:
        raise SmokeError(f"uptime_seconds regressed from {previous} to {current}")
    return current


def validate_diagnostics_archive(payload: bytes) -> None:
    with zipfile.ZipFile(io.BytesIO(payload)) as zf:
        names = {name for name in zf.namelist() if not name.endswith("/")}
    missing = sorted(DIAGNOSTICS_REQUIRED_ENTRIES - names)
    if missing:
        raise SmokeError(f"diagnostics export missing required entries: {missing}")


def extract_backup_archive_path(task_body: dict[str, object]) -> str:
    task = task_body.get("task")
    if not isinstance(task, dict):
        raise SmokeError(f"task detail missing task payload: {task_body}")
    if str(task.get("task_type", "")) != "backup.create":
        raise SmokeError(f"unexpected task type for backup smoke: {task}")
    if str(task.get("status", "")) != "succeeded":
        raise SmokeError(f"backup task did not succeed: {task}")
    result = task.get("result")
    if not isinstance(result, dict):
        raise SmokeError(f"backup task missing result summary: {task}")
    details = result.get("details")
    if not isinstance(details, dict):
        raise SmokeError(f"backup task missing result details: {task}")
    archive_path = details.get("archive_path")
    if not isinstance(archive_path, str) or not archive_path.strip():
        raise SmokeError(f"backup task missing archive_path detail: {task}")
    return archive_path


def assert_recovery_summary_acceptable(summary: dict[str, object] | None) -> None:
    if summary is None:
        return
    status = str(summary.get("status", ""))
    if status not in NON_BLOCKING_RECOVERY_STATUSES:
        raise SmokeError(f"unexpected recovery summary status during self-host smoke: {summary}")


def request_json(
    url: str,
    *,
    method: str = "GET",
    body: dict[str, object] | None = None,
    headers: dict[str, str] | None = None,
    expected_status: int = 200,
    expected_statuses: set[int] | None = None,
    timeout: int = 5,
) -> dict[str, object]:
    payload = None
    request_headers = dict(headers or {})
    if body is not None:
        payload = json.dumps(body).encode("utf-8")
        request_headers.setdefault("Content-Type", "application/json")
    request = urllib.request.Request(url, data=payload, method=method, headers=request_headers)
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            status = response.status
            raw_body = response.read()
    except urllib.error.HTTPError as exc:
        status = exc.code
        raw_body = exc.read()
    allowed_statuses = expected_statuses or {expected_status}
    if status not in allowed_statuses:
        expected_label = ", ".join(str(item) for item in sorted(allowed_statuses))
        raise SmokeError(f"{method} {url} returned {status}, expected {expected_label}: {raw_body.decode('utf-8', errors='replace')}")
    if not raw_body:
        return {}
    return json.loads(raw_body.decode("utf-8"))


def request_bytes(
    url: str,
    *,
    headers: dict[str, str] | None = None,
    expected_status: int = 200,
    timeout: int = 15,
) -> bytes:
    request = urllib.request.Request(url, headers=headers or {}, method="GET")
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            status = response.status
            payload = response.read()
    except urllib.error.HTTPError as exc:
        status = exc.code
        payload = exc.read()
    if status != expected_status:
        raise SmokeError(f"GET {url} returned {status}, expected {expected_status}: {payload.decode('utf-8', errors='replace')}")
    return payload


def bearer_headers(session_token: str) -> dict[str, str]:
    return {"Authorization": f"Bearer {session_token}"}


def start_server(root: Path, server_bin: Path) -> subprocess.Popen[str]:
    return subprocess.Popen(
        server_base_command(server_bin),
        cwd=root,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )


def wait_for_management_state(
    root: Path,
    process: subprocess.Popen[str],
    base_url: str,
    *,
    allowed_ready_statuses: set[str],
    timeout_seconds: int = 60,
) -> None:
    deadline = time.time() + timeout_seconds
    last_error: Exception | None = None
    while time.time() < deadline:
        if process.poll() is not None:
            raise SmokeError(f"server exited before management probes stabilized\n{read_process_output(process)}")
        try:
            request_json(f"{base_url}healthz")
            ready_body = request_json(f"{base_url}readyz", expected_statuses={200, 503})
            status = str(ready_body.get("status", ""))
            if status in allowed_ready_statuses:
                return
            last_error = SmokeError(f"unexpected readyz status: {ready_body}")
        except Exception as exc:  # noqa: BLE001
            last_error = exc
        time.sleep(1)
    raise SmokeError(f"timed out waiting for management state: {last_error}")


def bootstrap_admin(base_url: str) -> str:
    body = request_json(
        f"{base_url}api/setup/admin",
        method="POST",
        body={"identifier": SETUP_IDENTIFIER, "secret": SETUP_SECRET},
    )
    session_token = body.get("session_token")
    if not isinstance(session_token, str) or not session_token:
        raise SmokeError(f"setup/admin did not return session_token: {body}")
    return session_token


def login(base_url: str) -> str:
    body = request_json(
        f"{base_url}api/session/login",
        method="POST",
        body={"identifier": SETUP_IDENTIFIER, "secret": SETUP_SECRET},
    )
    session_token = body.get("session_token")
    if not isinstance(session_token, str) or not session_token:
        raise SmokeError(f"session/login did not return session_token: {body}")
    return session_token


def validate_managed_status(base_url: str, session_token: str, previous_uptime: int | None, stalled_polls: int) -> tuple[int, int]:
    ready_body = request_json(f"{base_url}readyz")
    ready_status = str(ready_body.get("status", ""))
    if ready_status not in MANAGED_READY_STATUSES:
        raise SmokeError(f"readyz returned blocking status during self-host smoke: {ready_body}")

    status_body = request_json(f"{base_url}api/system/status", headers=bearer_headers(session_token))
    if str(status_body.get("status", "")) != "running":
        raise SmokeError(f"system status must remain running during self-host smoke: {status_body}")

    assert_recovery_summary_acceptable(status_body.get("recovery_summary") if isinstance(status_body, dict) else None)

    uptime_raw = status_body.get("uptime_seconds")
    if not isinstance(uptime_raw, int):
        raise SmokeError(f"system status missing integer uptime_seconds: {status_body}")
    if previous_uptime is None:
        return uptime_raw, 0

    current_uptime = ensure_monotonic_uptime(previous_uptime, uptime_raw)
    if current_uptime == previous_uptime:
        stalled_polls += 1
    else:
        stalled_polls = 0
    if stalled_polls >= 2:
        raise SmokeError(f"uptime_seconds stopped growing across multiple probes: previous={previous_uptime} current={current_uptime}")
    return current_uptime, stalled_polls


def run_diagnostics_export(base_url: str, session_token: str) -> None:
    payload = request_bytes(f"{base_url}api/system/diagnostics/export", headers=bearer_headers(session_token))
    validate_diagnostics_archive(payload)


def poll_backup_task(base_url: str, session_token: str, task_id: str, *, timeout_seconds: int = 120) -> dict[str, object]:
    deadline = time.time() + timeout_seconds
    seen_in_list = False
    while time.time() < deadline:
        tasks_list = request_json(f"{base_url}api/tasks?limit=50", headers=bearer_headers(session_token))
        items = tasks_list.get("items")
        if isinstance(items, list):
            seen_in_list = seen_in_list or any(isinstance(item, dict) and item.get("task_id") == task_id for item in items)

        task_detail = request_json(f"{base_url}api/tasks/{task_id}", headers=bearer_headers(session_token))
        task = task_detail.get("task")
        if not isinstance(task, dict):
            raise SmokeError(f"task detail missing task payload: {task_detail}")
        status = str(task.get("status", ""))
        if status == "succeeded":
            if not seen_in_list:
                raise SmokeError(f"backup task {task_id} never appeared in /api/tasks")
            return task_detail
        if status in {"failed", "cancelled", "interrupted"}:
            raise SmokeError(f"backup task {task_id} ended in blocking state: {task_detail}")
        time.sleep(1)
    raise SmokeError(f"timed out waiting for backup task {task_id}")


def graceful_shutdown(base_url: str, session_token: str, process: subprocess.Popen[str]) -> None:
    with contextlib.suppress(Exception):
        request_json(
            f"{base_url}api/system/shutdown",
            method="POST",
            headers=bearer_headers(session_token),
            expected_status=202,
        )
    deadline = time.time() + 20
    while process.poll() is None and time.time() < deadline:
        time.sleep(1)
    stop_process(process)


def run_backup_cycle(root: Path, base_url: str, session_token: str) -> None:
    accepted = request_json(
        f"{base_url}api/system/backup",
        method="POST",
        headers=bearer_headers(session_token),
        expected_status=202,
    )
    task_id = accepted.get("task_id")
    if not isinstance(task_id, str) or not task_id:
        raise SmokeError(f"system/backup did not return task_id: {accepted}")
    task_detail = poll_backup_task(base_url, session_token, task_id)
    archive_path = extract_backup_archive_path(task_detail)
    if not Path(archive_path).exists():
        raise SmokeError(f"backup task archive_path does not exist: {archive_path}")
    repo_relative = Path(archive_path)
    if not repo_relative.is_absolute():
        if not (root / repo_relative).exists():
            raise SmokeError(f"backup task archive_path is not resolvable from package root: {archive_path}")


def execute_self_host_smoke(artifact_id: str, archive_path: Path, *, window_seconds: int, probe_interval_seconds: int) -> None:
    with tempfile.TemporaryDirectory(prefix="rayleabot-self-host-") as tmp:
        temp_root = Path(tmp)
        release_root = unpack_archive(artifact_id, archive_path, temp_root)
        ensure_required_paths(release_root, artifact_id)
        ensure_runtime_bootstrap(release_root, artifact_id)

        port = choose_free_port()
        config_path = write_user_config(release_root, port=port)
        server_bin = relative_executable(release_root, artifact_id)
        if not server_bin.exists():
            raise SmokeError(f"server executable missing: {server_bin}")

        base_url = f"http://127.0.0.1:{port}/"
        process = start_server(release_root, server_bin)
        session_token = ""
        try:
            wait_for_management_state(release_root, process, base_url, allowed_ready_statuses=STARTUP_READY_STATUSES)
            bootstrap_admin(base_url)
            session_token = login(base_url)

            previous_uptime: int | None = None
            stalled_polls = 0
            diagnostics_done = False
            backup_done = False
            midpoint = time.time() + max(window_seconds / 2, 1)
            deadline = time.time() + max(window_seconds, 1)

            while time.time() < deadline:
                previous_uptime, stalled_polls = validate_managed_status(base_url, session_token, previous_uptime, stalled_polls)
                now = time.time()
                if now >= midpoint and not diagnostics_done:
                    run_diagnostics_export(base_url, session_token)
                    diagnostics_done = True
                if now >= midpoint and not backup_done:
                    run_backup_cycle(release_root, base_url, session_token)
                    backup_done = True

                sleep_seconds = min(probe_interval_seconds, max(int(deadline - time.time()), 0))
                if sleep_seconds <= 0:
                    break
                time.sleep(sleep_seconds)

            if not diagnostics_done:
                run_diagnostics_export(base_url, session_token)
            if not backup_done:
                run_backup_cycle(release_root, base_url, session_token)
        finally:
            if process.poll() is None:
                if session_token:
                    graceful_shutdown(base_url, session_token, process)
                else:
                    stop_process(process)

        restarted = start_server(release_root, server_bin)
        restart_session_token = ""
        try:
            wait_for_management_state(release_root, restarted, base_url, allowed_ready_statuses=MANAGED_READY_STATUSES)
            restart_session_token = login(base_url)
            validate_managed_status(base_url, restart_session_token, None, 0)
            run_diagnostics_export(base_url, restart_session_token)
        finally:
            if restarted.poll() is None:
                if restart_session_token:
                    graceful_shutdown(base_url, restart_session_token, restarted)
                else:
                    stop_process(restarted)


def main() -> int:
    args = build_parser().parse_args()
    try:
        execute_self_host_smoke(
            args.artifact_id,
            Path(args.archive),
            window_seconds=args.window_seconds,
            probe_interval_seconds=args.probe_interval_seconds,
        )
        print("self-host smoke passed")
        return 0
    except (SmokeError, RuntimeError) as exc:
        print(f"self-host smoke failed: {exc}")
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
