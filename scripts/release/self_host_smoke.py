#!/usr/bin/env python3
from __future__ import annotations

import argparse
import contextlib
import io
import json
import os
import re
import struct
import uuid
import shutil
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
import zipfile
from pathlib import Path

from package_runtime import (
    REQUIRED_PATHS,
    artifact_platform,
    choose_free_port,
    ensure_required_paths,
    ensure_runtime_bootstrap,
    find_platform_resource,
    load_deps_manifest,
    read_process_output,
    relative_executable,
    server_base_command,
    start_captured_process,
    stop_process,
    store_root,
    unpack_archive,
    write_user_config,
)


SETUP_IDENTIFIER = "admin"
SETUP_SECRET = "fixture-only-secret"
SETUP_TOKEN = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
DIAGNOSTICS_REQUIRED_ENTRIES = {"system-status.json", "readiness.json", "doctor.json"}
STARTUP_READY_STATUSES = {"ready", "degraded", "setup_required"}
MANAGED_READY_STATUSES = {"ready", "degraded"}
NON_BLOCKING_RECOVERY_STATUSES = {"compatible", "degraded"}
EXPECTED_PROTOCOL_TRANSPORTS = {"reverse_ws", "forward_ws", "http_api", "webhook"}
EXPECTED_PROTOCOL_PROVIDERS = {"unknown", "standard", "napcat", "luckylillia"}
EXPECTED_PROTOCOL_READINESS_STATUSES = {"setup_required", "ready", "degraded", "failed"}
EXPECTED_COMPATIBILITY_CATEGORIES = {"events", "message_segments", "read_capabilities", "provider_extensions"}
EXPECTED_COMPATIBILITY_ITEMS = {
    "notice.flash_file",
    "flash_file",
    "message.history.get",
    "provider.napcat.group.sign.set",
    "provider.luckylillia.friend_groups.get",
}
EXPECTED_COMPATIBILITY_SUPPORT_VALUES = {"supported", "unsupported"}
DEFAULT_TEMPLATE_ID = "help.menu"


class SmokeError(RuntimeError):
    pass


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="RayleaBot long self-host smoke check")
    parser.add_argument("--artifact-id", required=True, choices=sorted(REQUIRED_PATHS.keys()))
    parser.add_argument("--archive", required=True)
    parser.add_argument("--plugin-fixture", required=True, type=Path)
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


def extract_task_id(payload: dict[str, object], endpoint: str) -> str:
    task_id = payload.get("task_id")
    if not isinstance(task_id, str) or not task_id:
        raise SmokeError(f"{endpoint} did not return task_id: {payload}")
    return task_id


def extract_task_details(task_body: dict[str, object], expected_task_type: str) -> dict[str, object]:
    task = task_body.get("task")
    if not isinstance(task, dict):
        raise SmokeError(f"task detail missing task payload: {task_body}")
    if str(task.get("task_type", "")) != expected_task_type:
        raise SmokeError(f"unexpected task type for {expected_task_type}: {task}")
    result = task.get("result")
    if not isinstance(result, dict):
        raise SmokeError(f"task missing result summary: {task}")
    details = result.get("details")
    if not isinstance(details, dict):
        raise SmokeError(f"task missing result details: {task}")
    return details


def task_body_from_log_detail(log_detail: dict[str, object], expected_task_id: str, expected_task_type: str) -> dict[str, object] | None:
    details = log_detail.get("details")
    if not isinstance(details, dict):
        return None
    if details.get("task_id") != expected_task_id:
        return None
    if str(details.get("task_type", "")) != expected_task_type:
        raise SmokeError(f"unexpected task type for {expected_task_type}: {details}")

    task: dict[str, object] = {
        "task_id": expected_task_id,
        "task_type": expected_task_type,
        "status": str(details.get("task_status", "")),
        "summary": str(details.get("task_summary", "")),
    }
    progress = details.get("task_progress")
    if isinstance(progress, int):
        task["progress"] = progress
    if isinstance(details.get("result_summary"), str):
        result_details = details.get("result_details")
        task["result"] = {
            "summary": details["result_summary"],
            "details": result_details if isinstance(result_details, dict) else {},
        }
    if isinstance(details.get("error_code"), str):
        error_details = details.get("error_details")
        task["error"] = {
            "code": details["error_code"],
            "message": str(details.get("error_message", "")),
            "details": error_details if isinstance(error_details, dict) else {},
        }
    return {"task": task}


def extract_runtime_bootstrap_results(task_body: dict[str, object]) -> list[dict[str, object]]:
    details = extract_task_details(task_body, "runtime.bootstrap")
    resources = details.get("resources")
    if not isinstance(resources, list):
        raise SmokeError(f"runtime bootstrap task missing resources detail: {task_body}")
    results: list[dict[str, object]] = []
    for item in resources:
        if not isinstance(item, dict):
            raise SmokeError(f"runtime bootstrap task returned invalid resource detail: {task_body}")
        results.append(item)
    return results


def runtime_bootstrap_result_mode(result: dict[str, object]) -> str | None:
    if result.get("used_system_browser") is True:
        return "system_browser"
    if result.get("used_prepared_store") is True:
        return "prepared_store"
    if result.get("used_cached_archive") is True:
        return "cached_archive"
    selected_source = result.get("selected_source")
    attempted_sources = result.get("attempted_sources")
    if isinstance(selected_source, str) and selected_source.strip():
        if isinstance(attempted_sources, list) and len(attempted_sources) > 0:
            return "downloaded"
    return None


def require_non_empty_string(value: object, label: str) -> str:
    if not isinstance(value, str) or not value.strip():
        raise SmokeError(f"{label} must be a non-empty string")
    return value.strip()


def validate_protocol_snapshot(snapshot: dict[str, object]) -> None:
    if str(snapshot.get("protocol", "")) != "onebot11":
        raise SmokeError(f"unexpected protocol snapshot payload: {snapshot}")
    provider = require_non_empty_string(snapshot.get("provider"), "protocol snapshot provider")
    if provider not in EXPECTED_PROTOCOL_PROVIDERS:
        raise SmokeError(f"unexpected protocol snapshot provider: {snapshot}")
    readiness_status = require_non_empty_string(snapshot.get("readiness_status"), "protocol snapshot readiness_status")
    if readiness_status not in EXPECTED_PROTOCOL_READINESS_STATUSES:
        raise SmokeError(f"unexpected protocol snapshot readiness_status: {snapshot}")
    require_non_empty_string(snapshot.get("summary"), "protocol snapshot summary")

    transport_status = snapshot.get("transport_status")
    if not isinstance(transport_status, list):
        raise SmokeError(f"protocol snapshot transport_status must be a list: {snapshot}")

    observed: set[str] = set()
    for item in transport_status:
        if not isinstance(item, dict):
            raise SmokeError(f"protocol snapshot transport_status item must be an object: {snapshot}")
        transport = require_non_empty_string(item.get("transport"), "protocol snapshot transport")
        observed.add(transport)
        require_non_empty_string(item.get("state"), f"{transport} state")
        require_non_empty_string(item.get("summary"), f"{transport} summary")
        if not isinstance(item.get("enabled"), bool) or not isinstance(item.get("configured"), bool):
            raise SmokeError(f"protocol snapshot transport flags must be boolean: {item}")
    if observed != EXPECTED_PROTOCOL_TRANSPORTS:
        raise SmokeError(f"protocol snapshot transport set mismatch: expected {sorted(EXPECTED_PROTOCOL_TRANSPORTS)} got {sorted(observed)}")

    for key in ("configured_transports", "active_transports"):
        transports = snapshot.get(key)
        if not isinstance(transports, list):
            raise SmokeError(f"protocol snapshot {key} must be a list: {snapshot}")
        unknown = {str(item) for item in transports} - EXPECTED_PROTOCOL_TRANSPORTS
        if unknown:
            raise SmokeError(f"protocol snapshot {key} contains unknown transports: {sorted(unknown)}")


def validate_protocol_compatibility(payload: dict[str, object]) -> None:
    if str(payload.get("protocol", "")) != "onebot11":
        raise SmokeError(f"unexpected protocol compatibility payload: {payload}")

    categories = payload.get("categories")
    if not isinstance(categories, list):
        raise SmokeError(f"protocol compatibility categories must be a list: {payload}")

    observed_categories: set[str] = set()
    observed_items: set[str] = set()
    for category in categories:
        if not isinstance(category, dict):
            raise SmokeError(f"protocol compatibility category must be an object: {payload}")
        key = require_non_empty_string(category.get("key"), "protocol compatibility category key")
        observed_categories.add(key)
        require_non_empty_string(category.get("title"), f"protocol compatibility category {key} title")
        items = category.get("items")
        if not isinstance(items, list):
            raise SmokeError(f"protocol compatibility category items must be a list: {category}")
        for item in items:
            if not isinstance(item, dict):
                raise SmokeError(f"protocol compatibility item must be an object: {category}")
            item_key = require_non_empty_string(item.get("key"), "protocol compatibility item key")
            observed_items.add(item_key)
            require_non_empty_string(item.get("label"), f"protocol compatibility item {item_key} label")
            require_non_empty_string(item.get("summary"), f"protocol compatibility item {item_key} summary")
            support = item.get("support")
            if not isinstance(support, dict):
                raise SmokeError(f"protocol compatibility support must be an object: {item}")
            for provider_key in ("standard", "napcat", "luckylillia"):
                value = require_non_empty_string(support.get(provider_key), f"{item_key} support {provider_key}")
                if value not in EXPECTED_COMPATIBILITY_SUPPORT_VALUES:
                    raise SmokeError(f"protocol compatibility support must stay within frozen values: {item}")

    if observed_categories != EXPECTED_COMPATIBILITY_CATEGORIES:
        raise SmokeError(
            f"protocol compatibility category set mismatch: expected {sorted(EXPECTED_COMPATIBILITY_CATEGORIES)} got {sorted(observed_categories)}"
        )
    missing_items = sorted(EXPECTED_COMPATIBILITY_ITEMS - observed_items)
    if missing_items:
        raise SmokeError(f"protocol compatibility missing representative items: {missing_items}")


def validate_render_template_source_info(source: object, template_id: str) -> None:
    if not isinstance(source, dict):
        raise SmokeError(f"render template {template_id} source must be an object")
    source_type = require_non_empty_string(source.get("type"), f"render template {template_id} source type")
    if source_type == "system":
        if source.get("plugin_id") is not None or source.get("local_id") is not None:
            raise SmokeError(f"system render template {template_id} must not expose plugin identity: {source}")
        return
    if source_type != "plugin":
        raise SmokeError(f"render template {template_id} source type is invalid: {source}")
    require_non_empty_string(source.get("plugin_id"), f"render template {template_id} source plugin_id")
    require_non_empty_string(source.get("local_id"), f"render template {template_id} source local_id")


def validate_render_template_metadata(template: dict[str, object]) -> str:
    template_id = require_non_empty_string(template.get("id"), "render template id")
    require_non_empty_string(template.get("version"), f"render template {template_id} version")
    require_non_empty_string(template.get("updated_at"), f"render template {template_id} updated_at")
    width = template.get("width")
    height = template.get("height")
    if type(width) is not int or width <= 0 or type(height) is not int or height <= 0:
        raise SmokeError(f"render template dimensions must be positive integers: {template}")
    if not isinstance(template.get("has_input_schema"), bool):
        raise SmokeError(f"render template has_input_schema must be boolean: {template}")
    validate_render_template_source_info(template.get("source"), template_id)
    return template_id


def select_template_id(list_payload: dict[str, object]) -> str:
    items = list_payload.get("items")
    if not isinstance(items, list) or len(items) == 0:
        raise SmokeError(f"render template list must contain items: {list_payload}")

    selected = ""
    available: set[str] = set()
    for item in items:
        if not isinstance(item, dict):
            raise SmokeError(f"render template list item must be an object: {list_payload}")
        template_id = validate_render_template_metadata(item)
        available.add(template_id)
        if template_id == DEFAULT_TEMPLATE_ID:
            selected = template_id

    if not selected:
        raise SmokeError(f"render template list is missing required packaged template {DEFAULT_TEMPLATE_ID}: {sorted(available)}")
    return selected


def validate_render_template_detail(payload: dict[str, object], template_id: str) -> dict[str, object]:
    template = payload.get("template")
    if not isinstance(template, dict):
        raise SmokeError(f"render template detail must contain template object: {payload}")
    if validate_render_template_metadata(template) != template_id:
        raise SmokeError(f"render template detail template_id mismatch: {payload}")
    input_schema = template.get("input_schema_json")
    if input_schema is not None and not isinstance(input_schema, dict):
        raise SmokeError(f"render template input_schema_json must be an object or null: {payload}")
    preview_data = template.get("preview_data_json")
    if preview_data is not None and not isinstance(preview_data, dict):
        raise SmokeError(f"render template preview_data_json must be an object or null: {payload}")
    return dict(preview_data) if isinstance(preview_data, dict) else {}


def validate_render_template_preview_html(payload: dict[str, object], template_id: str) -> str:
    if set(payload) != {"template_id", "source_digest", "width", "height", "html"}:
        raise SmokeError("template preview fields differ from the formal contract")
    if str(payload.get("template_id", "")) != template_id:
        raise SmokeError(f"unexpected render template preview payload: {payload}")
    source_digest = require_non_empty_string(payload.get("source_digest"), f"{template_id} source_digest")
    width = payload.get("width")
    height = payload.get("height")
    if type(width) is not int or width <= 0 or type(height) is not int or height <= 0:
        raise SmokeError(f"render template preview dimensions must be positive integers: {payload}")
    require_non_empty_string(payload.get("html"), f"{template_id} preview html")
    return source_digest


def exercise_packaged_protocol_and_template_workflows(base_url: str, session_token: str) -> tuple[str, str]:
    adapters_snapshot = request_json(f"{base_url}api/adapters", headers=bearer_headers(session_token))
    adapters = adapters_snapshot.get("adapters")
    if not isinstance(adapters, list):
        raise SmokeError("adapters snapshot must contain an array")
    for adapter in adapters:
        if not isinstance(adapter, dict):
            raise SmokeError("adapter must be an object")
        require_non_empty_string(adapter.get("id"), "adapter id")
        if adapter.get("protocol") == "onebot11":
            validate_protocol_snapshot(adapter.get("onebot11", {}))

    protocol_compatibility = request_json(
        f"{base_url}api/protocols/onebot11/compatibility",
        headers=bearer_headers(session_token),
    )
    validate_protocol_compatibility(protocol_compatibility)

    template_list = request_json(f"{base_url}api/system/render/templates", headers=bearer_headers(session_token))
    template_id = select_template_id(template_list)

    detail_body = request_json(
        f"{base_url}api/system/render/templates/{template_id}",
        headers=bearer_headers(session_token),
    )
    preview_data = validate_render_template_detail(detail_body, template_id)
    preview_body = request_json(
        f"{base_url}api/system/render/templates/{template_id}/preview-html",
        method="POST",
        body={"theme": "default", "data": preview_data},
        headers=bearer_headers(session_token),
    )
    return template_id, validate_render_template_preview_html(preview_body, template_id)


def verify_render_template_after_restart(base_url: str, session_token: str, template_id: str, expected_source_digest: str) -> None:
    detail_body = request_json(
        f"{base_url}api/system/render/templates/{template_id}",
        headers=bearer_headers(session_token),
    )
    preview_data = validate_render_template_detail(detail_body, template_id)
    preview_body = request_json(
        f"{base_url}api/system/render/templates/{template_id}/preview-html",
        method="POST",
        body={"theme": "default", "data": preview_data},
        headers=bearer_headers(session_token),
    )
    current_source_digest = validate_render_template_preview_html(preview_body, template_id)
    if current_source_digest != expected_source_digest:
        raise SmokeError(
            f"render template source digest changed after restart: expected {expected_source_digest} got {current_source_digest}"
        )


def create_runtime_bootstrap_task(base_url: str, session_token: str, resources: list[str] | None = None) -> str:
    body = {"resources": resources} if resources is not None else None
    accepted = request_json(
        f"{base_url}api/system/runtime/bootstrap",
        method="POST",
        body=body,
        headers=bearer_headers(session_token),
        expected_status=202,
    )
    return extract_task_id(accepted, "system/runtime/bootstrap")


def create_recovery_recheck_task(base_url: str, session_token: str) -> str:
    accepted = request_json(
        f"{base_url}api/system/recovery/recheck",
        method="POST",
        headers=bearer_headers(session_token),
        expected_status=202,
    )
    return extract_task_id(accepted, "system/recovery/recheck")


def assert_recovery_summary_acceptable(summary: dict[str, object] | None) -> None:
    if summary is None:
        return
    status = str(summary.get("status", ""))
    if status not in NON_BLOCKING_RECOVERY_STATUSES:
        raise SmokeError(f"unexpected recovery summary status during self-host smoke: {summary}")
    if status == "compatible":
        if summary.get("manual_actions") or summary.get("next_steps") or summary.get("skipped_plugins"):
            raise SmokeError(f"compatible recovery summary must not retain manual guidance: {summary}")
    if status == "degraded":
        manual_actions = summary.get("manual_actions", [])
        next_steps = summary.get("next_steps", [])
        if not isinstance(manual_actions, list) or len(manual_actions) == 0:
            raise SmokeError(f"degraded recovery summary must include manual_actions: {summary}")
        if not isinstance(next_steps, list) or len(next_steps) == 0:
            raise SmokeError(f"degraded recovery summary must include next_steps: {summary}")


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
        try:
            raw_body = exc.read()
        finally:
            exc.close()
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
        try:
            payload = exc.read()
        finally:
            exc.close()
    if status != expected_status:
        raise SmokeError(f"GET {url} returned {status}, expected {expected_status}: {payload.decode('utf-8', errors='replace')}")
    return payload


def bearer_headers(session_token: str) -> dict[str, str]:
    return {"Authorization": f"Bearer {session_token}"}


def start_server(root: Path, server_bin: Path, temporary_root: Path | None = None) -> subprocess.Popen[str]:
    environment = os.environ.copy()
    environment["RAYLEA_SETUP_TOKEN"] = SETUP_TOKEN
    if temporary_root is not None:
        temporary_root.mkdir(parents=True, exist_ok=False)
        environment.update({key: str(temporary_root.resolve()) for key in ("TMP", "TEMP", "TMPDIR")})
    return start_captured_process(server_base_command(server_bin), cwd=root, env=environment)


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
    if process.poll() is None:
        stop_process(process)
    raise SmokeError(f"timed out waiting for management state: {last_error}\n{read_process_output(process)}")


def bootstrap_admin(base_url: str) -> str:
    body = request_json(
        f"{base_url}api/setup/admin",
        method="POST",
        body={"identifier": SETUP_IDENTIFIER, "secret": SETUP_SECRET},
        headers={
            "Origin": base_url.rstrip("/"),
            "X-Raylea-Setup-Token": SETUP_TOKEN,
        },
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


def poll_task(
    base_url: str,
    session_token: str,
    task_id: str,
    *,
    expected_task_type: str,
    timeout_seconds: int = 120,
) -> dict[str, object]:
    deadline = time.time() + timeout_seconds
    seen_task = False
    while time.time() < deadline:
        logs_list = request_json(
            f"{base_url}api/logs?scope=current_session&source=tasks&limit=50",
            headers=bearer_headers(session_token),
        )
        items = logs_list.get("items")
        if isinstance(items, list):
            for item in items:
                if not isinstance(item, dict):
                    continue
                log_id = item.get("log_id")
                if not isinstance(log_id, str) or not log_id:
                    continue
                log_detail = request_json(f"{base_url}api/logs/{log_id}", headers=bearer_headers(session_token))
                task_detail = task_body_from_log_detail(log_detail, task_id, expected_task_type)
                if task_detail is None:
                    continue
                seen_task = True
                task = task_detail["task"]
                status = str(task.get("status", "")) if isinstance(task, dict) else ""
                if status == "succeeded":
                    return task_detail
                if status in {"failed", "cancelled", "interrupted"}:
                    raise SmokeError(f"task {task_id} ended in blocking state: {task_detail}")
        time.sleep(1)
    suffix = " after task log appeared" if seen_task else " without task log"
    raise SmokeError(f"timed out waiting for task {task_id}{suffix}")


def poll_backup_task(base_url: str, session_token: str, task_id: str, *, timeout_seconds: int = 120) -> dict[str, object]:
    return poll_task(
        base_url,
        session_token,
        task_id,
        expected_task_type="backup.create",
        timeout_seconds=timeout_seconds,
    )


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


def remove_prepared_runtime_stores(root: Path, artifact_id: str, resources: list[str]) -> None:
    manifest = load_deps_manifest(root)
    platform = artifact_platform(artifact_id)
    for kind in resources:
        resource = find_platform_resource(manifest, platform, kind)
        shutil.rmtree(store_root(root, resource), ignore_errors=True)


def run_runtime_bootstrap_cycle(root: Path, artifact_id: str, base_url: str, session_token: str) -> None:
    resources = ["chromium"]
    remove_prepared_runtime_stores(root, artifact_id, resources)
    task_id = create_runtime_bootstrap_task(base_url, session_token, resources)
    task_detail = poll_task(
        base_url,
        session_token,
        task_id,
        expected_task_type="runtime.bootstrap",
    )
    bootstrap_results = extract_runtime_bootstrap_results(task_detail)
    by_kind = {
        str(item.get("kind", "")): item
        for item in bootstrap_results
        if isinstance(item, dict) and isinstance(item.get("kind"), str)
    }
    for kind in resources:
        result = by_kind.get(kind)
        if result is None:
            raise SmokeError(f"runtime bootstrap task missing {kind} result: {task_detail}")
        mode = runtime_bootstrap_result_mode(result)
        if mode is None:
            raise SmokeError(f"runtime bootstrap task did not report a valid acquisition mode for {kind}: {task_detail}")
        if mode == "system_browser":
            continue
        store_root_path = result.get("store_root")
        if not isinstance(store_root_path, str) or not Path(store_root_path).exists():
            raise SmokeError(f"runtime bootstrap task returned missing store_root for {kind}: {task_detail}")
        if mode != "prepared_store":
            archive_path = result.get("archive_path")
            if not isinstance(archive_path, str) or not Path(archive_path).exists():
                raise SmokeError(f"runtime bootstrap task returned missing archive_path for {kind}: {task_detail}")



class ProcessWitness:
    """Track a specific owned process without sending signals to unrelated PIDs."""

    def __init__(self, pid: int):
        if type(pid) is not int or pid <= 0:
            raise SmokeError("fixture did not report a valid process ID")
        self.pid = pid
        self.handle = None
        if os.name == "nt":
            import ctypes
            from ctypes import wintypes
            self.kernel = ctypes.WinDLL("kernel32", use_last_error=True)
            self.kernel.OpenProcess.argtypes = [wintypes.DWORD, wintypes.BOOL, wintypes.DWORD]
            self.kernel.OpenProcess.restype = wintypes.HANDLE
            self.kernel.WaitForSingleObject.argtypes = [wintypes.HANDLE, wintypes.DWORD]
            self.kernel.WaitForSingleObject.restype = wintypes.DWORD
            self.kernel.CloseHandle.argtypes = [wintypes.HANDLE]
            self.handle = self.kernel.OpenProcess(0x00100000, False, pid)
            if not self.handle:
                raise SmokeError(f"cannot observe owned process {pid}: {ctypes.get_last_error()}")
        else:
            self.identity = self._unix_identity()
            if not self.identity:
                raise SmokeError(f"owned process {pid} exited before it was observed")

    def _unix_identity(self) -> str:
        return subprocess.run(["ps", "-p", str(self.pid), "-o", "lstart="],
                              capture_output=True, text=True, check=False).stdout.strip()

    def exited(self) -> bool:
        if self.handle is not None:
            status = self.kernel.WaitForSingleObject(self.handle, 0)
            if status == 0xFFFFFFFF:
                raise SmokeError(f"cannot query owned process {self.pid}")
            return status == 0
        return self._unix_identity() != self.identity

    def wait_exit(self, timeout: float = 20) -> None:
        deadline = time.monotonic() + timeout
        while not self.exited():
            if time.monotonic() >= deadline:
                raise SmokeError(f"owned process {self.pid} remained alive after shutdown")
            time.sleep(0.1)

    def close(self) -> None:
        if self.handle is not None:
            self.kernel.CloseHandle(self.handle)
            self.handle = None


def descendant_commands(parent_pid: int) -> list[tuple[int, str]]:
    if os.name == "nt":
        script = (
            "$queue=[Collections.Generic.Queue[int]]::new();"
            f"$queue.Enqueue({int(parent_pid)});"
            "$seen=[Collections.Generic.HashSet[int]]::new();"
            "$result=[Collections.Generic.List[object]]::new();"
            "while($queue.Count){$owner=$queue.Dequeue();"
            "foreach($child in @(Get-CimInstance Win32_Process -Filter ('ParentProcessId = '+$owner))){"
            "if($seen.Add([int]$child.ProcessId)){"
            "$queue.Enqueue([int]$child.ProcessId);"
            "$result.Add(@{pid=[int]$child.ProcessId;command=[string]$child.CommandLine})}}};"
            "ConvertTo-Json -InputObject @($result.ToArray()) -Compress"
        )
        result = subprocess.run(["powershell", "-NoProfile", "-NonInteractive", "-Command", script],
                                capture_output=True, text=True, check=True,
                                creationflags=subprocess.CREATE_NO_WINDOW)
        return [(int(item["pid"]), item["command"]) for item in json.loads(result.stdout)]
    result = subprocess.run(["ps", "-A", "-o", "pid=,ppid="], capture_output=True, text=True, check=True)
    children: dict[int, list[int]] = {}
    for line in result.stdout.splitlines():
        pid, parent = map(int, line.split())
        children.setdefault(parent, []).append(pid)
    pending, selected = [parent_pid], []
    while pending:
        for pid in children.get(pending.pop(), []):
            selected.append(pid)
            pending.append(pid)
    return [(pid, subprocess.run(["ps", "-p", str(pid), "-o", "command="],
                                 capture_output=True, text=True, check=False).stdout.strip())
            for pid in selected]


class BrowserOwnership:
    def __init__(self, server_pid: int, temporary_root: Path):
        root = temporary_root.resolve(strict=True)
        self.profiles = [path for path in root.iterdir()
                         if path.name.startswith("rayleabot-chromium-") and path.is_dir()
                         and not path.is_symlink() and path.resolve().parent == root]
        if not self.profiles:
            raise SmokeError("rendering did not create a Server-owned Chromium profile")
        self.processes: list[ProcessWitness] = []
        commands = descendant_commands(server_pid)
        try:
            for profile in self.profiles:
                marker = str(profile).replace("\\", "/").lower()
                owners = [(pid, command) for pid, command in commands
                          if marker in command.replace("\\", "/").lower() and " --type=" not in command]
                if not owners:
                    raise SmokeError("Chromium profile has no browser process descended from this Server")
                self.processes.extend(ProcessWitness(pid) for pid, _ in owners)
        except BaseException:
            self.close()
            raise

    def assert_released(self) -> None:
        for process in self.processes:
            process.wait_exit()
        remaining = [str(path) for path in self.profiles if path.exists()]
        if remaining:
            raise SmokeError(f"Server-owned Chromium profiles survived shutdown: {remaining}")

    def close(self) -> None:
        for process in self.processes:
            process.close()


def wait_plugin_state(base_url: str, token: str, plugin_id: str, expected: str) -> dict[str, object]:
    deadline = time.monotonic() + 60
    while time.monotonic() < deadline:
        plugin = request_json(f"{base_url}api/plugins/{plugin_id}", headers=bearer_headers(token))["plugin"]
        if plugin.get("state") == expected:
            return plugin
        if plugin.get("state") in {"failed", "invalid"}:
            raise SmokeError(f"acceptance plugin entered {plugin.get('state')}")
        time.sleep(0.1)
    raise SmokeError(f"acceptance plugin did not become {expected}")


def wait_plugin_task(base_url: str, token: str, accepted: dict[str, object], task_type: str) -> None:
    task_id = extract_task_id(accepted, task_type)
    deadline = time.monotonic() + 120
    while time.monotonic() < deadline:
        task = request_json(f"{base_url}api/system/tasks/{task_id}", headers=bearer_headers(token))
        if task.get("task_id") != task_id:
            raise SmokeError("plugin task identity changed")
        if task.get("status") == "succeeded":
            poll_task(base_url, token, task_id, expected_task_type=task_type, timeout_seconds=20)
            return
        if task.get("status") not in {"pending", "running"}:
            raise SmokeError(f"{task_type} ended with {task.get('status')}: {task.get('error_code')}")
        time.sleep(0.1)
    raise SmokeError(f"{task_type} did not finish")


def wait_acceptance_probe(base_url: str, token: str, plugin_id: str, probe: str) -> dict[str, object]:
    deadline = time.monotonic() + 90
    seen: set[str] = set()
    while time.monotonic() < deadline:
        query = urllib.parse.urlencode({"scope": "current_session", "source": "plugin",
                                       "plugin_id": plugin_id, "limit": 100})
        logs = request_json(f"{base_url}api/logs?{query}", headers=bearer_headers(token))
        for item in logs.get("items", []):
            log_id = item.get("log_id")
            if not isinstance(log_id, str) or log_id in seen:
                continue
            seen.add(log_id)
            detail = request_json(f"{base_url}api/logs/{log_id}", headers=bearer_headers(token))
            fields = detail.get("details", {})
            if fields.get("acceptance_probe") == probe:
                return fields
        time.sleep(0.2)
    raise SmokeError("native plugin did not log its completed render probe")


def verify_probe_png(root: Path, fields: dict[str, object]) -> tuple[int, int]:
    artifact_id = fields.get("artifact_id")
    if not isinstance(artifact_id, str) or not re.fullmatch(r"artifact_[0-9a-f]{24}", artifact_id):
        raise SmokeError("render probe did not return a valid artifact ID")
    if fields.get("mime") != "image/png":
        raise SmokeError("render probe did not return PNG")
    path = (root / "data/render" / f"{artifact_id}.png").resolve(strict=True)
    if not path.is_relative_to(root.resolve()):
        raise SmokeError("render probe artifact escaped the smoke installation")
    header = path.read_bytes()[:24]
    if len(header) != 24 or header[:8] != b"\x89PNG\r\n\x1a\n" or header[12:16] != b"IHDR":
        raise SmokeError("render probe artifact is not a PNG image")
    width, height = struct.unpack(">II", header[16:24])
    expected_width = json.loads((root / "templates/help.menu/template.json").read_text(encoding="utf-8"))["width"]
    if width != expected_width or not 0 < height <= 32768:
        raise SmokeError(f"render probe dimensions are invalid: {width}x{height}")
    return width, height


def request_plugin_state_change(base_url: str, token: str, plugin_id: str, action: str) -> None:
    detail = request_json(f"{base_url}api/plugins/{plugin_id}/{action}", method="POST", body={},
                          headers=bearer_headers(token), expected_status=200)
    plugin = detail.get("plugin")
    if not isinstance(plugin, dict) or plugin.get("id") != plugin_id:
        raise SmokeError(f"plugin {action} did not return its current detail")


def exercise_plugin_acceptance(root: Path, base_url: str, token: str, plugin_fixture: Path,
                               server_pid: int, temporary_root: Path,
                               browser_owners: list[BrowserOwnership]) -> dict[str, object]:
    headers = bearer_headers(token)
    inspected = request_json(f"{base_url}api/plugins/install/inspect", method="POST",
                             body={"source_type": "local_zip", "source": str(plugin_fixture.resolve(strict=True))},
                             headers=headers)
    plugin_id = inspected["plugin"]["id"]
    if plugin_id != "raylea.echo":
        raise SmokeError("self-host acceptance requires the external native echo fixture")
    accepted = request_json(f"{base_url}api/plugins/install", method="POST", expected_status=202,
                            body={"inspection_id": inspected["inspection_id"],
                                  "package_sha256": inspected["package_sha256"], "trusted_code_confirmed": True},
                            headers=headers)
    wait_plugin_task(base_url, token, accepted, "plugin.install")
    installed = root / "plugins/installed" / plugin_id
    if not (installed / "info.json").is_file():
        raise SmokeError("successful install did not publish the plugin directory")
    request_plugin_state_change(base_url, token, plugin_id, "enable")
    wait_plugin_state(base_url, token, plugin_id, "running")
    processes: list[ProcessWitness] = []
    probes = []
    try:
        for phase in ["initial", "reloaded"]:
            probe = f"{phase}-{uuid.uuid4().hex}"
            settings = request_json(f"{base_url}api/plugins/{plugin_id}/settings", method="PUT",
                                    body={"values": {"fixture_acceptance": True, "acceptance_probe": probe}},
                                    headers=headers)
            if settings.get("values", {}).get("acceptance_probe") != probe:
                raise SmokeError("HTTP settings did not persist the probe")
            fields = wait_acceptance_probe(base_url, token, plugin_id, probe)
            width, height = verify_probe_png(root, fields)
            process = ProcessWitness(fields.get("fixture_pid"))
            if process.exited():
                raise SmokeError("native plugin exited before lifecycle verification")
            processes.append(process)
            probes.append({"phase": phase, "artifact_id": fields["artifact_id"], "width": width,
                           "height": height, "plugin_pid": process.pid})
            if phase == "initial":
                browser_owners.append(BrowserOwnership(server_pid, temporary_root))
                request_plugin_state_change(base_url, token, plugin_id, "reload")
                process.wait_exit()
                wait_plugin_state(base_url, token, plugin_id, "running")
                current = request_json(f"{base_url}api/plugins/{plugin_id}/settings", headers=headers)
                if current.get("values", {}).get("acceptance_probe") != probe:
                    raise SmokeError("reload discarded saved plugin settings")
        request_plugin_state_change(base_url, token, plugin_id, "disable")
        wait_plugin_state(base_url, token, plugin_id, "disabled")
        for process in processes:
            process.wait_exit()
        accepted = request_json(f"{base_url}api/plugins/{plugin_id}", method="DELETE",
                                headers=headers, expected_status=202)
        wait_plugin_task(base_url, token, accepted, "plugin.uninstall")
        request_json(f"{base_url}api/plugins/{plugin_id}", headers=headers, expected_status=404)
        if installed.exists():
            raise SmokeError("successful uninstall left the installed plugin directory")
        result = {"plugin": plugin_id, "installed": True, "settings_persisted": True, "reloaded": True,
                  "disabled": True, "uninstalled": True, "native_processes_reaped": True, "png_probes": probes}
        print("plugin acceptance: " + json.dumps(result), flush=True)
        return result
    finally:
        for process in processes:
            process.close()



@contextlib.contextmanager
def smoke_workspace():
    root = Path(tempfile.mkdtemp(prefix="rayleabot-self-host-"))
    try:
        yield root
    except BaseException:
        print(f"self-host failure evidence retained at {root}", flush=True)
        raise
    else:
        shutil.rmtree(root)


def execute_self_host_smoke(artifact_id: str, archive_path: Path, *, plugin_fixture: Path, window_seconds: int, probe_interval_seconds: int) -> None:
    with smoke_workspace() as temp_root:
        release_root = unpack_archive(artifact_id, archive_path, temp_root)
        print(f"self-host installation: {release_root}", flush=True)
        ensure_required_paths(release_root, artifact_id)
        ensure_runtime_bootstrap(release_root, artifact_id)

        port = choose_free_port()
        config_path = write_user_config(release_root, port=port)
        server_bin = relative_executable(release_root, artifact_id)
        if not server_bin.exists():
            raise SmokeError(f"server executable missing: {server_bin}")

        base_url = f"http://127.0.0.1:{port}/"
        process_temp = temp_root / "owned-server-temporary-files"
        browser_owners: list[BrowserOwnership] = []
        process = start_server(release_root, server_bin, process_temp)
        session_token = ""
        try:
            wait_for_management_state(release_root, process, base_url, allowed_ready_statuses=STARTUP_READY_STATUSES)
            bootstrap_admin(base_url)
            session_token = login(base_url)
            run_runtime_bootstrap_cycle(release_root, artifact_id, base_url, session_token)
            exercise_plugin_acceptance(release_root, base_url, session_token, plugin_fixture,
                                       process.pid, process_temp, browser_owners)
            template_id, expected_source_digest = exercise_packaged_protocol_and_template_workflows(
                base_url,
                session_token,
            )

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
            try:
                if process.poll() is None:
                    if session_token:
                        graceful_shutdown(base_url, session_token, process)
                    else:
                        stop_process(process)
            finally:
                try:
                    for owner in browser_owners:
                        owner.assert_released()
                    if browser_owners:
                        print("Server-owned browser processes and profiles released", flush=True)
                finally:
                    for owner in browser_owners:
                        owner.close()
                    output = read_process_output(process)
                    (temp_root / "server-output.log").write_text(output, encoding="utf-8")
                    print(output, flush=True)

        restarted = start_server(release_root, server_bin)
        restart_session_token = ""
        try:
            wait_for_management_state(release_root, restarted, base_url, allowed_ready_statuses=MANAGED_READY_STATUSES)
            restart_session_token = login(base_url)
            validate_managed_status(base_url, restart_session_token, None, 0)
            verify_render_template_after_restart(base_url, restart_session_token, template_id, expected_source_digest)
            run_diagnostics_export(base_url, restart_session_token)
        finally:
            if restarted.poll() is None:
                if restart_session_token:
                    graceful_shutdown(base_url, restart_session_token, restarted)
                else:
                    stop_process(restarted)
            output = read_process_output(restarted)
            (temp_root / "server-restart-output.log").write_text(output, encoding="utf-8")
            print(output, flush=True)

        # Windows extraction uses an owned sibling with a shorter path.
        if not release_root.is_relative_to(temp_root):
            shutil.rmtree(release_root)


def main() -> int:
    args = build_parser().parse_args()
    try:
        execute_self_host_smoke(
            args.artifact_id,
            Path(args.archive),
            plugin_fixture=args.plugin_fixture,
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
