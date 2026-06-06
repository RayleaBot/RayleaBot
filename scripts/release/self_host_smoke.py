#!/usr/bin/env python3
from __future__ import annotations

import argparse
import contextlib
import io
import json
import shutil
import subprocess
import tempfile
import time
import urllib.error
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
    stop_process,
    store_root,
    unpack_archive,
    write_user_config,
)


SETUP_IDENTIFIER = "admin"
SETUP_SECRET = "fixture-only-secret"
DIAGNOSTICS_REQUIRED_ENTRIES = {"system-status.json", "readiness.json", "doctor.json"}
STARTUP_READY_STATUSES = {"ready", "degraded", "setup_required"}
MANAGED_READY_STATUSES = {"ready", "degraded"}
NON_BLOCKING_RECOVERY_STATUSES = {"compatible", "degraded"}
EXPECTED_PROTOCOL_TRANSPORTS = {"reverse_ws", "forward_ws", "http_api", "webhook"}
EXPECTED_PROTOCOL_PROVIDERS = {"standard", "napcat", "luckylillia"}
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


def select_template_id(list_payload: dict[str, object]) -> str:
    items = list_payload.get("items")
    if not isinstance(items, list) or len(items) == 0:
        raise SmokeError(f"render template list must contain items: {list_payload}")

    selected = ""
    available: set[str] = set()
    for item in items:
        if not isinstance(item, dict):
            raise SmokeError(f"render template list item must be an object: {list_payload}")
        template_id = require_non_empty_string(item.get("id"), "render template id")
        available.add(template_id)
        require_non_empty_string(item.get("version"), f"render template {template_id} version")
        require_non_empty_string(item.get("current_revision_id"), f"render template {template_id} current_revision_id")
        updated_at = item.get("updated_at")
        if updated_at is not None:
            require_non_empty_string(updated_at, f"render template {template_id} updated_at")
        if not isinstance(item.get("width"), int) or not isinstance(item.get("height"), int):
            raise SmokeError(f"render template dimensions must be integers: {item}")
        if not isinstance(item.get("has_input_schema"), bool):
            raise SmokeError(f"render template has_input_schema must be boolean: {item}")
        if template_id == DEFAULT_TEMPLATE_ID:
            selected = template_id

    if not selected:
        raise SmokeError(f"render template list is missing required packaged template {DEFAULT_TEMPLATE_ID}: {sorted(available)}")
    return selected


def validate_render_template_source(payload: dict[str, object], template_id: str) -> str:
    if str(payload.get("template_id", "")) != template_id:
        raise SmokeError(f"unexpected render template source payload: {payload}")
    revision_id = require_non_empty_string(payload.get("revision_id"), f"{template_id} revision_id")
    source = payload.get("source")
    if not isinstance(source, dict):
        raise SmokeError(f"render template source must be an object: {payload}")
    manifest = source.get("manifest_json")
    if not isinstance(manifest, dict):
        raise SmokeError(f"render template manifest_json must be an object: {payload}")
    if str(manifest.get("id", "")) != template_id:
        raise SmokeError(f"render template manifest id must match path template_id: {payload}")
    require_non_empty_string(source.get("html"), f"{template_id} html")
    require_non_empty_string(source.get("stylesheet"), f"{template_id} stylesheet")
    return revision_id


def validate_render_template_validation(payload: dict[str, object], template_id: str) -> None:
    if payload.get("valid") is not True:
        raise SmokeError(f"render template validation must succeed in packaged smoke: {payload}")
    issues = payload.get("issues")
    if not isinstance(issues, list):
        raise SmokeError(f"render template validation issues must be a list: {payload}")
    normalized_manifest = payload.get("normalized_manifest")
    if not isinstance(normalized_manifest, dict):
        raise SmokeError(f"render template validation must return normalized_manifest: {payload}")
    if str(normalized_manifest.get("id", "")) != template_id:
        raise SmokeError(f"render template validation manifest id mismatch: {payload}")


def validate_render_template_detail(payload: dict[str, object], template_id: str, *, expected_kind: str | None = None) -> str:
    template = payload.get("template")
    if not isinstance(template, dict):
        raise SmokeError(f"render template detail must contain template object: {payload}")
    if str(template.get("id", "")) != template_id:
        raise SmokeError(f"render template detail template_id mismatch: {payload}")
    current_revision_id = require_non_empty_string(template.get("current_revision_id"), f"{template_id} current_revision_id")
    current_revision = template.get("current_revision")
    if not isinstance(current_revision, dict):
        raise SmokeError(f"render template detail must contain current_revision: {payload}")
    revision_id = require_non_empty_string(current_revision.get("revision_id"), f"{template_id} current_revision.revision_id")
    if revision_id != current_revision_id:
        raise SmokeError(f"render template detail current revision mismatch: {payload}")
    revision_kind = require_non_empty_string(current_revision.get("kind"), f"{template_id} current_revision.kind")
    if expected_kind is not None and revision_kind != expected_kind:
        raise SmokeError(f"render template detail revision kind mismatch: expected {expected_kind} got {revision_kind}")
    last_validation = template.get("last_validation")
    if not isinstance(last_validation, dict):
        raise SmokeError(f"render template detail must contain last_validation: {payload}")
    if "valid" not in last_validation or "issue_count" not in last_validation:
        raise SmokeError(f"render template detail last_validation missing required fields: {payload}")
    return current_revision_id


def validate_render_template_versions(
    payload: dict[str, object],
    *,
    expected_top_revision_id: str | None = None,
    expected_top_kind: str | None = None,
) -> list[str]:
    items = payload.get("items")
    if not isinstance(items, list) or len(items) == 0:
        raise SmokeError(f"render template versions must contain items: {payload}")

    revision_ids: list[str] = []
    for item in items:
        if not isinstance(item, dict):
            raise SmokeError(f"render template version item must be an object: {payload}")
        revision_id = require_non_empty_string(item.get("revision_id"), "render template version revision_id")
        revision_ids.append(revision_id)
        require_non_empty_string(item.get("template_version"), f"{revision_id} template_version")
        require_non_empty_string(item.get("saved_at"), f"{revision_id} saved_at")
        kind = require_non_empty_string(item.get("kind"), f"{revision_id} kind")
        if kind not in {"save", "rollback"}:
            raise SmokeError(f"render template version kind must stay within frozen values: {item}")
    if expected_top_revision_id is not None and revision_ids[0] != expected_top_revision_id:
        raise SmokeError(f"render template versions top revision mismatch: expected {expected_top_revision_id} got {revision_ids[0]}")
    if expected_top_kind is not None and str(items[0].get("kind", "")) != expected_top_kind:
        raise SmokeError(f"render template versions top kind mismatch: expected {expected_top_kind} got {items[0]}")
    return revision_ids


def exercise_packaged_protocol_and_template_workflows(base_url: str, session_token: str) -> tuple[str, str]:
    protocol_snapshot = request_json(f"{base_url}api/protocols/onebot11", headers=bearer_headers(session_token))
    validate_protocol_snapshot(protocol_snapshot)

    protocol_compatibility = request_json(
        f"{base_url}api/protocols/onebot11/compatibility",
        headers=bearer_headers(session_token),
    )
    validate_protocol_compatibility(protocol_compatibility)

    template_list = request_json(f"{base_url}api/system/render/templates", headers=bearer_headers(session_token))
    template_id = select_template_id(template_list)

    source_body = request_json(
        f"{base_url}api/system/render/templates/{template_id}/source",
        headers=bearer_headers(session_token),
    )
    base_revision_id = validate_render_template_source(source_body, template_id)
    source_bundle = source_body["source"]
    if not isinstance(source_bundle, dict):
        raise SmokeError(f"render template source bundle must be an object: {source_body}")

    validation_body = request_json(
        f"{base_url}api/system/render/templates/{template_id}/validate",
        method="POST",
        body={"source": source_bundle},
        headers=bearer_headers(session_token),
    )
    validate_render_template_validation(validation_body, template_id)

    updated_source = json.loads(json.dumps(source_bundle))
    if not isinstance(updated_source, dict):
        raise SmokeError("render template source bundle clone failed")
    html = require_non_empty_string(updated_source.get("html"), f"{template_id} html")
    updated_source["html"] = f"{html}\n<!-- self-host smoke save -->"

    save_body = request_json(
        f"{base_url}api/system/render/templates/{template_id}/source",
        method="PUT",
        body={
            "base_revision_id": base_revision_id,
            "message": "Self-host smoke save",
            "source": updated_source,
        },
        headers=bearer_headers(session_token),
    )
    save_revision_id = validate_render_template_detail(save_body, template_id, expected_kind="save")
    if save_revision_id == base_revision_id:
        raise SmokeError("render template save must create a new revision")

    versions_after_save = request_json(
        f"{base_url}api/system/render/templates/{template_id}/versions",
        headers=bearer_headers(session_token),
    )
    revision_ids_after_save = validate_render_template_versions(
        versions_after_save,
        expected_top_revision_id=save_revision_id,
        expected_top_kind="save",
    )
    if base_revision_id not in revision_ids_after_save:
        raise SmokeError(f"render template versions must retain base revision after save: {versions_after_save}")

    rollback_body = request_json(
        f"{base_url}api/system/render/templates/{template_id}/rollback",
        method="POST",
        body={
            "target_revision_id": base_revision_id,
            "base_revision_id": save_revision_id,
            "message": "Self-host smoke rollback",
        },
        headers=bearer_headers(session_token),
    )
    rollback_revision_id = validate_render_template_detail(rollback_body, template_id, expected_kind="rollback")
    if rollback_revision_id in {base_revision_id, save_revision_id}:
        raise SmokeError("render template rollback must create a distinct rollback revision")

    versions_after_rollback = request_json(
        f"{base_url}api/system/render/templates/{template_id}/versions",
        headers=bearer_headers(session_token),
    )
    revision_ids_after_rollback = validate_render_template_versions(
        versions_after_rollback,
        expected_top_revision_id=rollback_revision_id,
        expected_top_kind="rollback",
    )
    if save_revision_id not in revision_ids_after_rollback or base_revision_id not in revision_ids_after_rollback:
        raise SmokeError(f"render template versions must retain save and target revisions after rollback: {versions_after_rollback}")
    return template_id, rollback_revision_id


def verify_render_template_after_restart(base_url: str, session_token: str, template_id: str, expected_revision_id: str) -> None:
    detail_body = request_json(
        f"{base_url}api/system/render/templates/{template_id}",
        headers=bearer_headers(session_token),
    )
    current_revision_id = validate_render_template_detail(detail_body, template_id)
    if current_revision_id != expected_revision_id:
        raise SmokeError(
            f"render template current revision changed after restart: expected {expected_revision_id} got {current_revision_id}"
        )
    source_body = request_json(
        f"{base_url}api/system/render/templates/{template_id}/source",
        headers=bearer_headers(session_token),
    )
    source_revision_id = validate_render_template_source(source_body, template_id)
    if source_revision_id != expected_revision_id:
        raise SmokeError(
            f"render template source revision changed after restart: expected {expected_revision_id} got {source_revision_id}"
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


def poll_task(
    base_url: str,
    session_token: str,
    task_id: str,
    *,
    expected_task_type: str,
    timeout_seconds: int = 120,
) -> dict[str, object]:
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
        if str(task.get("task_type", "")) != expected_task_type:
            raise SmokeError(f"unexpected task type for {expected_task_type}: {task}")
        status = str(task.get("status", ""))
        if status == "succeeded":
            if not seen_in_list:
                raise SmokeError(f"task {task_id} never appeared in /api/tasks")
            return task_detail
        if status in {"failed", "cancelled", "interrupted"}:
            raise SmokeError(f"task {task_id} ended in blocking state: {task_detail}")
        time.sleep(1)
    raise SmokeError(f"timed out waiting for task {task_id}")


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
    resources = ["python-runtime", "nodejs-runtime"]
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
        if runtime_bootstrap_result_mode(result) is None:
            raise SmokeError(f"runtime bootstrap task did not report a valid acquisition mode for {kind}: {task_detail}")
        archive_path = result.get("archive_path")
        store_root_path = result.get("store_root")
        if not isinstance(archive_path, str) or not Path(archive_path).exists():
            raise SmokeError(f"runtime bootstrap task returned missing archive_path for {kind}: {task_detail}")
        if not isinstance(store_root_path, str) or not Path(store_root_path).exists():
            raise SmokeError(f"runtime bootstrap task returned missing store_root for {kind}: {task_detail}")


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
            run_runtime_bootstrap_cycle(release_root, artifact_id, base_url, session_token)
            template_id, expected_revision_id = exercise_packaged_protocol_and_template_workflows(
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
            verify_render_template_after_restart(base_url, restart_session_token, template_id, expected_revision_id)
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
