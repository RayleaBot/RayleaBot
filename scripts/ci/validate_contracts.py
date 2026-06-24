#!/usr/bin/env python3
"""Validate RayleaBot contracts in PR or strict mode."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError as exc:  # pragma: no cover - exercised by CI environment setup.
    raise SystemExit("PyYAML is required: python -m pip install pyyaml") from exc


ROOT = Path(__file__).resolve().parents[2]
CONTRACTS = ROOT / "contracts"
FIXTURES = ROOT / "fixtures"
EXAMPLES = ROOT / "examples"

REQUIRED_CONTRACT_FILES = {
    "README.md",
    "backup-manifest.schema.json",
    "config.user.schema.json",
    "deps-manifest.schema.json",
    "error-codes.yaml",
    "web-api.openapi.yaml",
    "websocket-events.yaml",
    "plugin-info.schema.json",
    "plugin-management-ui.yaml",
    "plugin-management-ui-bridge.schema.json",
    "plugin-protocol.schema.json",
    "release-manifest.schema.json",
    "cli-commands.yaml",
}

STRICT_FIXTURE_DIRS = [
    FIXTURES / "config",
    FIXTURES / "backup-manifest",
    FIXTURES / "deps-manifest",
    FIXTURES / "web-api",
    FIXTURES / "websocket",
    FIXTURES / "errors",
    FIXTURES / "plugin-info",
    FIXTURES / "plugin-protocol",
    FIXTURES / "release-manifest",
    FIXTURES / "cli",
]

STRICT_OPENAPI_PATHS = {
    "/healthz",
    "/readyz",
    "/api/setup/admin",
    "/api/setup/status",
    "/api/session/login",
    "/api/session",
    "/api/launcher/status",
    "/api/launcher/shutdown",
    "/api/config",
    "/api/third-party/accounts",
    "/api/third-party/accounts/{platform}/login/qrcode",
    "/api/third-party/accounts/{platform}/login/qrcode/{login_id}",
    "/api/third-party/accounts/{platform}/{account_id}",
    "/api/third-party/users/resolve",
    "/api/third-party/media",
    "/api/third-party/monitors",
    "/api/bilibili/login/qrcode",
    "/api/bilibili/login/qrcode/{login_id}",
    "/api/bilibili/users/resolve",
    "/api/bilibili/source/status",
    "/api/bilibili/source/restart",
    "/api/governance/blacklist",
    "/api/governance/blacklist/entries",
    "/api/governance/blacklist/entries/{entry_type}/{target_id}",
    "/api/governance/command-policy",
    "/api/governance/whitelist",
    "/api/governance/whitelist/entries",
    "/api/governance/whitelist/entries/{entry_type}/{target_id}",
    "/api/governance/whitelist/state",
    "/api/system/status",
    "/api/system/shutdown",
    "/api/system/backup",
    "/api/system/metrics",
    "/api/system/recovery/recheck",
    "/api/system/recovery/confirm",
    "/api/system/render/templates",
    "/api/system/render/templates/{template_id}",
    "/api/system/render/templates/{template_id}/asset",
    "/api/system/render/templates/{template_id}/preview-html",
    "/api/system/runtime/bootstrap",
    "/api/system/diagnostics/export",
    "/api/system/scheduler/jobs",
    "/api/system/scheduler/jobs/{job_id}/trigger",
    "/api/logs",
    "/api/logs/{log_id}",
    "/api/protocols/onebot11",
    "/api/protocols/onebot11/compatibility",
    "/api/protocols/onebot11/identities/resolve",
    "/api/protocols/onebot11/reverse-ws",
    "/api/protocols/onebot11/targets",
    "/api/protocols/onebot11/webhook",
    "/api/tasks",
    "/api/tasks/{task_id}",
    "/api/tasks/{task_id}/cancel",
    "/api/plugins",
    "/api/plugins/install",
    "/api/plugins/{plugin_id}",
    "/api/plugins/{plugin_id}/enable",
    "/api/plugins/{plugin_id}/disable",
    "/api/plugins/{plugin_id}/recover",
    "/api/plugins/{plugin_id}/reload",
    "/api/plugins/{plugin_id}/settings",
    "/api/plugins/{plugin_id}/secrets",
    "/api/webhooks/{plugin_id}/{route}",
}


def fail(message: str) -> None:
    raise SystemExit(message)


def load_json(path: Path) -> Any:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except Exception as exc:
        fail(f"{path.relative_to(ROOT)}: invalid JSON: {exc}")


def load_yaml(path: Path) -> Any:
    try:
        return yaml.safe_load(path.read_text(encoding="utf-8"))
    except Exception as exc:
        fail(f"{path.relative_to(ROOT)}: invalid YAML: {exc}")


def load_any(path: Path) -> Any:
    if path.suffix == ".json":
        return load_json(path)
    if path.suffix in {".yaml", ".yml"}:
        return load_yaml(path)
    fail(f"{path.relative_to(ROOT)}: unsupported fixture extension")


def collect_refs(document: Any, refs: list[str]) -> None:
    if isinstance(document, dict):
        for key, value in document.items():
            if key in {"x-fixtures", "example_ref"}:
                if isinstance(value, list):
                    refs.extend(str(item) for item in value)
                else:
                    refs.append(str(value))
            collect_refs(value, refs)
    elif isinstance(document, list):
        for item in document:
            collect_refs(item, refs)


def require_object(value: Any, label: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        fail(f"{label} must be an object")
    return value


def require_fields(value: dict[str, Any], fields: list[str], label: str) -> None:
    missing = [field for field in fields if field not in value]
    if missing:
        fail(f"{label} missing fields: {missing}")


def validate_required_files() -> None:
    existing = {path.name for path in CONTRACTS.iterdir() if path.is_file()}
    missing = REQUIRED_CONTRACT_FILES - existing
    if missing:
        fail(f"missing contract files: {sorted(missing)}")


def validate_parseable_documents() -> list[Any]:
    documents: list[Any] = []
    for path in sorted(CONTRACTS.iterdir()):
        if path.suffix == ".json":
            documents.append(load_json(path))
        elif path.suffix in {".yaml", ".yml"}:
            documents.append(load_yaml(path))

    for path in sorted(FIXTURES.rglob("*")):
        if path.is_file() and path.suffix in {".json", ".yaml", ".yml"}:
            load_any(path)

    for path in sorted(EXAMPLES.glob("plugins/*/info.json")):
        load_json(path)

    return documents


def validate_fixture_refs(documents: list[Any]) -> None:
    refs: list[str] = []
    for document in documents:
        collect_refs(document, refs)
    if not refs:
        fail("contracts must declare fixture references")
    for ref in refs:
        ref_path = ROOT / ref
        if not ref_path.exists():
            fail(f"missing referenced fixture: {ref}")
        if ref_path.is_file() and ref_path.suffix in {".json", ".yaml", ".yml"}:
            load_any(ref_path)


def validate_openapi_basic(web_api: dict[str, Any]) -> None:
    if web_api.get("openapi") != "3.1.0":
        fail("contracts/web-api.openapi.yaml must use OpenAPI 3.1.0")
    paths = require_object(web_api.get("paths"), "web-api paths")
    if not paths:
        fail("web-api paths must not be empty")
    components = require_object(web_api.get("components"), "web-api components")
    require_object(components.get("schemas"), "web-api components.schemas")
    for path in ["/healthz", "/readyz", "/api/session/login", "/api/tasks"]:
        if path not in paths:
            fail(f"web-api missing required entry path: {path}")


def validate_errors_basic(error_codes: dict[str, Any]) -> None:
    codes = require_object(error_codes.get("codes"), "error-codes codes")
    if not codes:
        fail("error-codes.yaml must declare codes")
    required = ["code", "message_key", "message", "description", "http_status", "retryable", "applies_to"]
    for code, body in codes.items():
        require_fields(require_object(body, f"error code {code}"), required, f"error code {code}")


def validate_websocket_basic(events: dict[str, Any]) -> None:
    envelope = require_object(events.get("envelope"), "websocket envelope")
    for field in ["channel", "type", "timestamp", "data"]:
        if field not in envelope.get("required", []):
            fail(f"websocket envelope missing required field: {field}")
    channels = events.get("channels")
    if not isinstance(channels, list) or not channels:
        fail("websocket-events.yaml must declare channels")
    for channel in channels:
        channel_obj = require_object(channel, "websocket channel")
        require_fields(channel_obj, ["path", "events"], f"websocket channel {channel_obj.get('path')}")
        for event in channel_obj.get("events", []):
            event_obj = require_object(event, "websocket event")
            require_fields(event_obj, ["event", "payload_schema"], f"websocket event {event_obj.get('event')}")


def validate_config_basic(config_schema: dict[str, Any]) -> None:
    if config_schema.get("type") != "object":
        fail("config.user.schema.json must define an object schema")
    properties = require_object(config_schema.get("properties"), "config schema properties")
    for field in ["schema_version", "server", "onebot", "admin", "permission", "database"]:
        if field not in properties:
            fail(f"config.user.schema.json missing property: {field}")


def validate_release_basic(release_schema: dict[str, Any]) -> None:
    if "oneOf" not in release_schema:
        fail("release-manifest.schema.json must distinguish release_manifest and build_info via oneOf")
    artifact = require_object(release_schema.get("$defs", {}).get("artifact"), "release artifact")
    for field in ["artifact_id", "file_name", "platform", "sha256", "size"]:
        if field not in artifact.get("required", []):
            fail(f"release-manifest.schema.json artifact missing required field: {field}")


def validate_pr() -> dict[str, Any]:
    validate_required_files()
    documents = validate_parseable_documents()
    validate_fixture_refs(documents)

    web_api = require_object(load_yaml(CONTRACTS / "web-api.openapi.yaml"), "web-api")
    websocket_events = require_object(load_yaml(CONTRACTS / "websocket-events.yaml"), "websocket-events")
    error_codes = require_object(load_yaml(CONTRACTS / "error-codes.yaml"), "error-codes")
    config_schema = require_object(load_json(CONTRACTS / "config.user.schema.json"), "config schema")
    release_schema = require_object(load_json(CONTRACTS / "release-manifest.schema.json"), "release schema")

    validate_openapi_basic(web_api)
    validate_errors_basic(error_codes)
    validate_websocket_basic(websocket_events)
    validate_config_basic(config_schema)
    validate_release_basic(release_schema)

    return {
        "web_api": web_api,
        "websocket_events": websocket_events,
        "error_codes": error_codes,
        "release_schema": release_schema,
    }


def validate_fixture_matrix() -> None:
    for path in STRICT_FIXTURE_DIRS:
        if not path.is_dir():
            fail(f"missing fixture directory: {path.relative_to(ROOT)}")
        names = {item.name for item in path.iterdir() if item.is_file()}
        for prefix in ["ok.", "invalid.", "edge."]:
            if not any(name.startswith(prefix) for name in names):
                fail(f"{path.relative_to(ROOT)} must contain a {prefix} fixture")


def validate_baseline() -> None:
    baseline = (ROOT / "docs" / "engineering" / "baseline.md").read_text(encoding="utf-8")
    for snippet in ["Go `1.25.8`", "Node.js `24.14.0`", "`pnpm 10.32.1`", "Python `3.12.13`"]:
        if snippet not in baseline:
            fail(f"docs/engineering/baseline.md missing expected snippet: {snippet}")

    required_commands = [
        'mkdir -p dist && go build -o "dist/raylea-server$(go env GOEXE)" ./cmd/raylea-server',
        "pnpm install --frozen-lockfile",
        "pnpm test",
        "pnpm build",
    ]
    for command in required_commands:
        if command not in baseline:
            fail(f"docs/engineering/baseline.md must mention command: {command}")

    go_mod = (ROOT / "server" / "go.mod").read_text(encoding="utf-8")
    if "module github.com/RayleaBot/RayleaBot/server" not in go_mod:
        fail("server/go.mod must use module path github.com/RayleaBot/RayleaBot/server")
    if "go 1.25.8" not in go_mod:
        fail("server/go.mod must pin Go 1.25.8")

    for package_path in [ROOT / "web" / "package.json", ROOT / "launcher" / "package.json"]:
        package_json = load_json(package_path)
        if package_json.get("packageManager") != "pnpm@10.32.1":
            fail(f"{package_path.relative_to(ROOT)} packageManager must be pnpm@10.32.1")
        engines = package_json.get("engines", {})
        if engines.get("node") != "24.14.0":
            fail(f"{package_path.relative_to(ROOT)} engines.node must be 24.14.0")
        if engines.get("pnpm") != "10.32.1":
            fail(f"{package_path.relative_to(ROOT)} engines.pnpm must be 10.32.1")


def validate_strict_openapi(web_api: dict[str, Any]) -> None:
    actual_paths = set(web_api.get("paths", {}).keys())
    if actual_paths != STRICT_OPENAPI_PATHS:
        missing = sorted(STRICT_OPENAPI_PATHS - actual_paths)
        extra = sorted(actual_paths - STRICT_OPENAPI_PATHS)
        fail(f"web-api paths drift: missing={missing}; extra={extra}")


def validate_strict_websocket(events: dict[str, Any]) -> None:
    event_names = {
        event.get("event")
        for channel in events.get("channels", [])
        for event in channel.get("events", [])
        if isinstance(event, dict)
    }
    expected = {"tasks.updated", "logs.appended", "events.received", "plugins.console"}
    if event_names != expected:
        fail(f"websocket event names drift: expected={sorted(expected)} actual={sorted(event_names)}")


def validate_strict_release(release_schema: dict[str, Any]) -> None:
    artifact = release_schema["$defs"]["artifact"]
    expected = {"windows-x64-full", "linux-x64-full", "macos-arm64-full", "linux-x64-server"}
    actual = set(artifact["properties"]["artifact_id"].get("enum", []))
    if actual != expected:
        fail(f"release artifact matrix drift: expected={sorted(expected)} actual={sorted(actual)}")


def validate_no_legacy_contract_content() -> None:
    snapshot = json.dumps(
        {
            path.name: load_any(path)
            for path in sorted(CONTRACTS.iterdir())
            if path.suffix in {".json", ".yaml", ".yml"}
        },
        ensure_ascii=False,
    )
    for legacy in ["platform.config_error", '"task.updated"', '"authors"']:
        if legacy in snapshot:
            fail(f"out-of-scope content leaked into formal contracts: {legacy}")


def validate_strict_cli() -> None:
    cli_commands = require_object(load_yaml(CONTRACTS / "cli-commands.yaml"), "cli commands")
    expected = {"reset-admin", "backup", "restore", "doctor", "cleanup"}
    actual = set(cli_commands.get("commands", {}).keys())
    if actual != expected:
        fail(f"cli commands drift: expected={sorted(expected)} actual={sorted(actual)}")


def validate_strict() -> None:
    loaded = validate_pr()
    validate_fixture_matrix()
    validate_baseline()
    validate_strict_openapi(loaded["web_api"])
    validate_strict_websocket(loaded["websocket_events"])
    validate_strict_release(loaded["release_schema"])
    validate_strict_cli()
    validate_no_legacy_contract_content()


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--mode", choices=["pr", "strict"], default="pr")
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv or sys.argv[1:])
    if args.mode == "pr":
        validate_pr()
    else:
        validate_strict()
    print(f"contracts validation passed: mode={args.mode}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
