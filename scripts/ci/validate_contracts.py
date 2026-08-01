#!/usr/bin/env python3
"""Validate RayleaBot contracts in PR or strict mode."""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path
from typing import Any
from urllib.parse import parse_qs, unquote, urlsplit

try:
    import yaml
except ImportError as exc:  # pragma: no cover - exercised by CI environment setup.
    raise SystemExit("PyYAML is required: python -m pip install pyyaml") from exc

try:
    from jsonschema import Draft202012Validator, FormatChecker
    from referencing import Registry, Resource
    from referencing.jsonschema import DRAFT202012
except ImportError as exc:  # pragma: no cover - exercised by CI environment setup.
    raise SystemExit("jsonschema is required: python -m pip install jsonschema") from exc


class JSONSafeLoader(yaml.SafeLoader):
    """Safe YAML loader that keeps date-like scalars as JSON strings."""


JSONSafeLoader.yaml_implicit_resolvers = {
    key: [
        (tag, regexp)
        for tag, regexp in resolvers
        if tag != "tag:yaml.org,2002:timestamp"
    ]
    for key, resolvers in yaml.SafeLoader.yaml_implicit_resolvers.items()
}


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
    "plugin-artifact.schema.json",
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
    FIXTURES / "plugin-artifact",
    FIXTURES / "plugin-protocol",
    FIXTURES / "release-manifest",
    FIXTURES / "cli",
]

JSON_SCHEMA_FIXTURE_AREAS = {
    "config": "config.user.schema.json",
    "backup-manifest": "backup-manifest.schema.json",
    "deps-manifest": "deps-manifest.schema.json",
    "plugin-info": "plugin-info.schema.json",
    "plugin-artifact": "plugin-artifact.schema.json",
    "release-manifest": "release-manifest.schema.json",
}

FIXTURE_SECRET_PATTERNS = [
    ("OpenAI API key", re.compile(r"\bsk-[A-Za-z0-9_-]{20,}\b")),
    ("GitHub token", re.compile(r"\b(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9_]{20,}\b")),
    ("GitHub fine-grained token", re.compile(r"\bgithub_pat_[A-Za-z0-9_]{20,}\b")),
    ("AWS access key", re.compile(r"\bAKIA[0-9A-Z]{16}\b")),
    ("Google API key", re.compile(r"\bAIza[0-9A-Za-z_-]{35}\b")),
    ("Slack token", re.compile(r"\bxox[baprs]-[0-9A-Za-z-]{20,}\b")),
    ("JWT", re.compile(r"\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b")),
    ("Bilibili SESSDATA", re.compile(r"\bSESSDATA=(?!fixture\b|backup\b|example\b|test\b)[^;\s]{12,}", re.IGNORECASE)),
    ("Bilibili csrf", re.compile(r"\bbili_jct=(?!fixture\b|backup\b|example\b|test\b)[0-9a-f]{16,}", re.IGNORECASE)),
    ("Weibo SUB cookie", re.compile(r"\bSUB=(?!fixture\b|example\b|test\b)[^;\s]{12,}", re.IGNORECASE)),
    ("Douyin sessionid", re.compile(r"\bsessionid=(?!fixture\b|example\b|test\b)[0-9a-f]{16,}", re.IGNORECASE)),
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
    "/api/system/diagnostics",
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
    "/api/plugins",
    "/api/plugins/install",
    "/api/plugins/install/inspect",
    "/api/plugins/{plugin_id}",
    "/api/plugins/{plugin_id}/enable",
    "/api/plugins/{plugin_id}/disable",
    "/api/plugins/{plugin_id}/recover",
    "/api/plugins/{plugin_id}/reload",
    "/api/plugins/{plugin_id}/management/actions",
    "/api/plugins/{plugin_id}/settings",
    "/api/plugins/{plugin_id}/secrets",
    "/api/update/status",
    "/api/update/check",
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
        return yaml.load(path.read_text(encoding="utf-8"), Loader=JSONSafeLoader)
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


def validate_fixture_secret_scan() -> None:
    for path in sorted(FIXTURES.rglob("*")):
        if not path.is_file() or path.suffix not in {".json", ".yaml", ".yml"}:
            continue
        text = path.read_text(encoding="utf-8")
        for label, pattern in FIXTURE_SECRET_PATTERNS:
            match = pattern.search(text)
            if match:
                fail(f"{path.relative_to(ROOT)} contains possible real {label}: {match.group(0)}")


def iter_refs(value: Any, location: str = "$") -> list[tuple[str, str]]:
    refs: list[tuple[str, str]] = []
    if isinstance(value, dict):
        ref = value.get("$ref")
        if isinstance(ref, str):
            refs.append((location, ref))
        for key, child in value.items():
            refs.extend(iter_refs(child, f"{location}/{key}"))
    elif isinstance(value, list):
        for index, child in enumerate(value):
            refs.extend(iter_refs(child, f"{location}/{index}"))
    return refs


def validate_no_network_refs(documents: dict[Path, Any]) -> None:
    for path, document in documents.items():
        for location, ref in iter_refs(document):
            parsed = urlsplit(ref)
            if parsed.scheme in {"http", "https"} or parsed.netloc:
                fail(f"{path.relative_to(ROOT)} {location}: network $ref is forbidden: {ref}")


def format_schema_error(error: Any) -> str:
    path = "/".join(str(item) for item in error.absolute_path)
    return f"{path or '<root>'}: {error.message}"


def fixture_expected_valid(path: Path, document: dict[str, Any]) -> bool:
    expect = document.get("expect")
    if isinstance(expect, dict) and isinstance(expect.get("valid"), bool):
        return expect["valid"]
    return not path.name.startswith("invalid.")


def require_fixture_outcome(path: Path, expected: bool, errors: list[str]) -> None:
    actual = not errors
    if actual == expected:
        return
    if expected:
        fail(f"{path.relative_to(ROOT)}: expected valid fixture; validation errors={errors[:3]}")
    fail(f"{path.relative_to(ROOT)}: invalid fixture did not fail validation")


def plugin_info_package_errors(document: dict[str, Any], manifest: Any) -> list[str]:
    package_files = document.get("package_files")
    if package_files is None:
        return []
    if not isinstance(package_files, dict):
        return ["package_files must be an object"]
    if not isinstance(manifest, dict):
        return []

    errors: list[str] = []
    for template in manifest.get("render_templates", []):
        if not isinstance(template, dict) or not isinstance(template.get("path"), str):
            continue
        manifest_path = template["path"].rstrip("/") + "/template.json"
        if manifest_path not in package_files:
            errors.append(f"missing package file: {manifest_path}")
            continue
        content = package_files[manifest_path]
        if not isinstance(content, dict):
            errors.append(f"invalid template manifest: {manifest_path}")
    return errors


def dependency_manifest_errors(manifest: Any) -> list[str]:
    if not isinstance(manifest, dict):
        return []
    errors: list[str] = []
    for index, resource in enumerate(manifest.get("resources", [])):
        if not isinstance(resource, dict):
            continue
        urls = [source.get("url") for source in resource.get("sources", []) if isinstance(source, dict)]
        if len(urls) != len(set(urls)):
            errors.append(f"resources/{index}/sources: source URLs must be unique")
    return errors


def validate_json_schema_fixtures() -> None:
    for area, schema_name in JSON_SCHEMA_FIXTURE_AREAS.items():
        schema_path = CONTRACTS / schema_name
        schema = require_object(load_json(schema_path), f"{schema_name} schema")
        try:
            Draft202012Validator.check_schema(schema)
        except Exception as exc:
            fail(f"{schema_path.relative_to(ROOT)} is not valid Draft 2020-12 schema: {exc}")
        validator = Draft202012Validator(schema, format_checker=FormatChecker())

        for path in sorted((FIXTURES / area).iterdir()):
            if not path.is_file() or path.suffix not in {".json", ".yaml", ".yml"}:
                continue
            document = require_object(load_any(path), str(path.relative_to(ROOT)))
            instance = document.get("input") if "input" in document else document
            errors = [format_schema_error(error) for error in validator.iter_errors(instance)]
            if area == "plugin-info":
                errors.extend(plugin_info_package_errors(document, instance))
            elif area == "deps-manifest":
                errors.extend(dependency_manifest_errors(instance))
            require_fixture_outcome(path, fixture_expected_valid(path, document), errors)

        if area == "deps-manifest":
            runtime_manifest_path = ROOT / ".deps" / "manifest.json"
            runtime_manifest = require_object(
                load_json(runtime_manifest_path),
                str(runtime_manifest_path.relative_to(ROOT)),
            )
            runtime_errors = [format_schema_error(error) for error in validator.iter_errors(runtime_manifest)]
            runtime_errors.extend(dependency_manifest_errors(runtime_manifest))
            if runtime_errors:
                fail(
                    f"{runtime_manifest_path.relative_to(ROOT)} drifted from {schema_name}: "
                    + "; ".join(runtime_errors)
                )

def validate_plugin_protocol_fixtures() -> None:
    schema_path = CONTRACTS / "plugin-protocol.schema.json"
    schema = require_object(load_json(schema_path), "plugin protocol schema")
    try:
        Draft202012Validator.check_schema(schema)
    except Exception as exc:
        fail(f"{schema_path.relative_to(ROOT)} is not valid Draft 2020-12 schema: {exc}")
    validator = Draft202012Validator(schema, format_checker=FormatChecker())

    for path in sorted((FIXTURES / "plugin-protocol").iterdir()):
        if not path.is_file() or path.suffix not in {".json", ".yaml", ".yml"}:
            continue
        document = require_object(load_any(path), str(path.relative_to(ROOT)))
        frames = document.get("frames")
        if not isinstance(frames, list) or not frames:
            fail(f"{path.relative_to(ROOT)}: frames must be a non-empty array")
        errors: list[str] = []
        for index, frame in enumerate(frames):
            errors.extend(
                f"frames/{index}/{format_schema_error(error)}"
                for error in validator.iter_errors(frame)
            )

        manifest = document.get("manifest")
        concurrency = manifest.get("concurrency", 1) if isinstance(manifest, dict) else 1
        if isinstance(concurrency, int) and concurrency > 1:
            for index, frame in enumerate(frames):
                if isinstance(frame, dict) and frame.get("type") == "action" and not frame.get("parent_request_id"):
                    errors.append(f"frames/{index}: concurrent plugin action requires parent_request_id")
        require_fixture_outcome(path, fixture_expected_valid(path, document), errors)


def pointer_escape(value: str) -> str:
    return value.replace("~", "~0").replace("/", "~1")


def pointer_get(document: Any, pointer: str) -> Any:
    current = document
    for token in pointer.removeprefix("/").split("/") if pointer else []:
        token = token.replace("~1", "/").replace("~0", "~")
        if isinstance(current, list):
            current = current[int(token)]
        else:
            current = current[token]
    return current


def resolve_local_object(document: dict[str, Any], value: Any, pointer: str) -> tuple[Any, str]:
    seen: set[str] = set()
    while isinstance(value, dict) and isinstance(value.get("$ref"), str):
        ref = value["$ref"]
        if not ref.startswith("#/"):
            return value, pointer
        pointer = ref[1:]
        if pointer in seen:
            fail(f"contracts/web-api.openapi.yaml: cyclic object $ref at {pointer}")
        seen.add(pointer)
        value = pointer_get(document, pointer)
    return value, pointer


def build_contract_registry(documents: dict[Path, Any]) -> Registry:
    registry = Registry()
    for path, document in documents.items():
        registry = registry.with_resource(
            path.resolve().as_uri(),
            Resource.from_contents(document, default_specification=DRAFT202012),
        )
    return registry


def schema_errors_at_pointer(
    contract_path: Path,
    registry: Registry,
    pointer: str,
    instance: Any,
) -> list[str]:
    validator = Draft202012Validator(
        {"$ref": f"{contract_path.resolve().as_uri()}#{pointer}"},
        registry=registry,
        format_checker=FormatChecker(),
    )
    try:
        return [format_schema_error(error) for error in validator.iter_errors(instance)]
    except Exception as exc:
        fail(f"{contract_path.relative_to(ROOT)}{pointer}: schema resolution failed: {exc}")


def matching_openapi_path(paths: dict[str, Any], request_path: str) -> str | None:
    request_path = urlsplit(request_path).path
    if request_path in paths:
        return request_path
    for candidate in paths:
        candidate_parts = candidate.strip("/").split("/")
        request_parts = request_path.strip("/").split("/")
        if len(candidate_parts) != len(request_parts):
            continue
        if all(
            (part.startswith("{") and part.endswith("}") and bool(actual)) or part == actual
            for part, actual in zip(candidate_parts, request_parts, strict=True)
        ):
            return candidate
    return None


def request_path_parameters(contract_path: str, request_target: str) -> dict[str, str]:
    actual_parts = urlsplit(request_target).path.strip("/").split("/")
    contract_parts = contract_path.strip("/").split("/")
    return {
        contract.removeprefix("{").removesuffix("}"): unquote(actual)
        for contract, actual in zip(contract_parts, actual_parts, strict=True)
        if contract.startswith("{") and contract.endswith("}")
    }


def header_value(headers: Any, name: str) -> str | None:
    if not isinstance(headers, dict):
        return None
    for key, value in headers.items():
        if str(key).lower() == name.lower() and isinstance(value, str):
            return value.split(";", 1)[0].strip().lower()
    return None


def parameter_schema_type(document: dict[str, Any], schema: Any, pointer: str) -> Any:
    resolved, _ = resolve_local_object(document, schema, pointer)
    if not isinstance(resolved, dict):
        return None
    return resolved.get("type")


def coerce_parameter_scalar(value: str, schema_type: Any) -> Any:
    allowed_types = set(schema_type) if isinstance(schema_type, list) else {schema_type}
    if "integer" in allowed_types:
        if not re.fullmatch(r"-?(?:0|[1-9][0-9]*)", value):
            raise ValueError("must be an integer")
        return int(value)
    if "number" in allowed_types:
        try:
            return float(value)
        except ValueError as exc:
            raise ValueError("must be a number") from exc
    if "boolean" in allowed_types:
        if value == "true":
            return True
        if value == "false":
            return False
        raise ValueError("must be true or false")
    return value


def parameter_instance(
    document: dict[str, Any],
    parameter: dict[str, Any],
    schema: Any,
    schema_pointer: str,
    values: list[str],
) -> Any:
    resolved_schema, resolved_pointer = resolve_local_object(document, schema, schema_pointer)
    if not isinstance(resolved_schema, dict):
        raise ValueError("schema must be an object")
    schema_type = resolved_schema.get("type")
    if schema_type == "array":
        items = resolved_schema.get("items")
        item_type = parameter_schema_type(document, items, resolved_pointer + "/items")
        serialized_values = values
        if parameter.get("explode") is False:
            serialized_values = [part for value in values for part in value.split(",")]
        return [coerce_parameter_scalar(value, item_type) for value in serialized_values]
    if len(values) != 1:
        raise ValueError("must not be repeated")
    return coerce_parameter_scalar(values[0], schema_type)


def validate_openapi_request_parameters(
    web_api: dict[str, Any],
    registry: Registry,
    contract_path: Path,
    contract_route: str,
    operation: dict[str, Any],
    operation_pointer: str,
    request: dict[str, Any],
) -> list[str]:
    request_target = str(request.get("path", ""))
    query = parse_qs(urlsplit(request_target).query, keep_blank_values=True)
    path_values = request_path_parameters(contract_route, request_target)
    headers = request.get("headers")
    route_object = require_object(web_api["paths"][contract_route], f"OpenAPI route {contract_route}")
    parameter_sources = [
        (route_object.get("parameters", []), f"/paths/{pointer_escape(contract_route)}/parameters"),
        (operation.get("parameters", []), operation_pointer + "/parameters"),
    ]
    errors: list[str] = []

    for parameters, parameters_pointer in parameter_sources:
        if parameters is None:
            continue
        if not isinstance(parameters, list):
            fail(f"contracts/web-api.openapi.yaml{parameters_pointer}: parameters must be an array")
        for index, parameter_value in enumerate(parameters):
            parameter_pointer = f"{parameters_pointer}/{index}"
            parameter, parameter_pointer = resolve_local_object(
                web_api,
                parameter_value,
                parameter_pointer,
            )
            if not isinstance(parameter, dict):
                fail(f"contracts/web-api.openapi.yaml{parameter_pointer}: parameter must be an object")
            name = parameter.get("name")
            location = parameter.get("in")
            schema = parameter.get("schema")
            if not isinstance(name, str) or not isinstance(location, str) or not isinstance(schema, dict):
                fail(f"contracts/web-api.openapi.yaml{parameter_pointer}: parameter requires name, in, and schema")

            values: list[str] | None
            if location == "query":
                values = query.get(name)
            elif location == "path":
                value = path_values.get(name)
                values = [value] if value is not None else None
            elif location == "header":
                value = header_value(headers, name)
                values = [value] if value is not None else None
            else:
                continue

            if values is None:
                if parameter.get("required") is True:
                    errors.append(f"parameters/{location}/{name}: required parameter is missing")
                continue

            schema_pointer = parameter_pointer + "/schema"
            try:
                instance = parameter_instance(web_api, parameter, schema, schema_pointer, values)
            except ValueError as exc:
                errors.append(f"parameters/{location}/{name}: {exc}")
                continue
            errors.extend(
                f"parameters/{location}/{name}/{error}"
                for error in schema_errors_at_pointer(contract_path, registry, schema_pointer, instance)
            )
    return errors


def select_media_type(content: Any, preferred: str | None, body: Any) -> str | None:
    if not isinstance(content, dict) or not content:
        return None
    if preferred and preferred in content:
        return preferred
    if isinstance(body, (dict, list)) and "application/json" in content:
        return "application/json"
    if "application/json" in content:
        return "application/json"
    if len(content) == 1:
        return next(iter(content))
    return None


def response_entry(responses: dict[Any, Any], status: int) -> tuple[Any, str] | None:
    for key, value in responses.items():
        if str(key) == str(status):
            return value, str(key)
    if "default" in responses:
        return responses["default"], "default"
    return None


def validate_openapi_fixtures(web_api: dict[str, Any], registry: Registry) -> None:
    contract_path = CONTRACTS / "web-api.openapi.yaml"
    paths = require_object(web_api.get("paths"), "web-api paths")
    for name, schema in require_object(web_api.get("components", {}).get("schemas"), "web-api schemas").items():
        try:
            Draft202012Validator.check_schema(schema)
        except Exception as exc:
            fail(f"contracts/web-api.openapi.yaml components.schemas.{name}: invalid Draft 2020-12 schema: {exc}")

    for path in sorted((FIXTURES / "web-api").iterdir()):
        if not path.is_file() or path.suffix not in {".json", ".yaml", ".yml"}:
            continue
        document = require_object(load_any(path), str(path.relative_to(ROOT)))
        request = require_object(document.get("request"), f"{path.relative_to(ROOT)} request")
        response = require_object(document.get("response"), f"{path.relative_to(ROOT)} response")
        method = str(request.get("method", "")).lower()
        contract_route = matching_openapi_path(paths, str(request.get("path", "")))
        if contract_route is None:
            fail(f"{path.relative_to(ROOT)}: request path is not declared in OpenAPI")
        operation = paths[contract_route].get(method)
        if not isinstance(operation, dict):
            fail(f"{path.relative_to(ROOT)}: method {method.upper()} is not declared for {contract_route}")
        operation_pointer = f"/paths/{pointer_escape(contract_route)}/{method}"

        request_errors = validate_openapi_request_parameters(
            web_api,
            registry,
            contract_path,
            contract_route,
            operation,
            operation_pointer,
            request,
        )
        if "body" in request:
            request_body, request_pointer = resolve_local_object(
                web_api,
                operation.get("requestBody"),
                operation_pointer + "/requestBody",
            )
            if not isinstance(request_body, dict):
                request_errors.append("request body is not declared")
            else:
                content = request_body.get("content")
                media_type = select_media_type(
                    content,
                    header_value(request.get("headers"), "content-type"),
                    request["body"],
                )
                if media_type is None or not isinstance(content.get(media_type), dict) or "schema" not in content[media_type]:
                    request_errors.append("request body media type has no schema")
                else:
                    schema_pointer = f"{request_pointer}/content/{pointer_escape(media_type)}/schema"
                    request_errors.extend(schema_errors_at_pointer(contract_path, registry, schema_pointer, request["body"]))

        status = response.get("status")
        if not isinstance(status, int):
            fail(f"{path.relative_to(ROOT)}: response.status must be an integer")
        responses = require_object(operation.get("responses"), f"OpenAPI responses for {contract_route}")
        entry = response_entry(responses, status)
        if entry is None:
            fail(f"{path.relative_to(ROOT)}: response status {status} is not declared")
        response_object, response_key = entry
        response_pointer = f"{operation_pointer}/responses/{pointer_escape(response_key)}"
        response_object, response_pointer = resolve_local_object(web_api, response_object, response_pointer)

        response_errors: list[str] = []
        if "body" in response:
            content = response_object.get("content") if isinstance(response_object, dict) else None
            media_type = select_media_type(
                content,
                response.get("content_type") or header_value(response.get("headers"), "content-type"),
                response["body"],
            )
            if media_type is None or not isinstance(content.get(media_type), dict) or "schema" not in content[media_type]:
                response_errors.append("response body media type has no schema")
            else:
                schema_pointer = f"{response_pointer}/content/{pointer_escape(media_type)}/schema"
                response_errors.extend(schema_errors_at_pointer(contract_path, registry, schema_pointer, response["body"]))

        expected = fixture_expected_valid(path, document)
        if expected:
            require_fixture_outcome(path, True, response_errors)
            if document.get("case") != "invalid" and request_errors:
                fail(f"{path.relative_to(ROOT)}: request body validation failed: {request_errors[:3]}")
        elif not request_errors and not response_errors:
            fail(f"{path.relative_to(ROOT)}: invalid fixture did not fail request or response validation")


def websocket_event_schema(events: dict[str, Any], frame: dict[str, Any]) -> Any:
    frame_type = frame.get("type")
    for event in events.get("session_events", []):
        if isinstance(event, dict) and event.get("event") == frame_type:
            return event.get("payload_schema")
    for channel in events.get("channels", []):
        if not isinstance(channel, dict) or channel.get("channel") != frame.get("channel"):
            continue
        for event in channel.get("events", []):
            if isinstance(event, dict) and event.get("event") == frame_type:
                return event.get("payload_schema")
    return None


def validate_websocket_fixtures(events: dict[str, Any]) -> None:
    envelope = require_object(events.get("envelope"), "websocket envelope")
    envelope_schema = {
        "$schema": "https://json-schema.org/draft/2020-12/schema",
        "type": "object",
        "additionalProperties": False,
        "required": envelope.get("required", []),
        "properties": envelope.get("properties", {}),
    }
    try:
        Draft202012Validator.check_schema(envelope_schema)
    except Exception as exc:
        fail(f"contracts/websocket-events.yaml envelope is not valid Draft 2020-12 schema: {exc}")
    envelope_validator = Draft202012Validator(envelope_schema, format_checker=FormatChecker())

    for path in sorted((FIXTURES / "websocket").iterdir()):
        if not path.is_file() or path.suffix not in {".json", ".yaml", ".yml"}:
            continue
        document = require_object(load_any(path), str(path.relative_to(ROOT)))
        frame = require_object(document.get("frame"), f"{path.relative_to(ROOT)} frame")
        errors = [format_schema_error(error) for error in envelope_validator.iter_errors(frame)]
        payload_schema = websocket_event_schema(events, frame)
        if not isinstance(payload_schema, dict):
            errors.append(f"unknown websocket event {frame.get('channel')}/{frame.get('type')}")
        else:
            try:
                Draft202012Validator.check_schema(payload_schema)
            except Exception as exc:
                fail(f"contracts/websocket-events.yaml {frame.get('type')}: invalid payload schema: {exc}")
            payload_validator = Draft202012Validator(payload_schema, format_checker=FormatChecker())
            errors.extend(f"data/{format_schema_error(error)}" for error in payload_validator.iter_errors(frame.get("data")))
        require_fixture_outcome(path, fixture_expected_valid(path, document), errors)


def validate_contract_instances(web_api: dict[str, Any], websocket_events: dict[str, Any]) -> None:
    contract_documents = {
        path.resolve(): load_any(path)
        for path in sorted(CONTRACTS.iterdir())
        if path.suffix in {".json", ".yaml", ".yml"}
    }
    validate_no_network_refs(contract_documents)
    registry = build_contract_registry(contract_documents)
    validate_json_schema_fixtures()
    validate_plugin_protocol_fixtures()
    validate_openapi_fixtures(web_api, registry)
    validate_websocket_fixtures(websocket_events)


def validate_openapi_basic(web_api: dict[str, Any]) -> None:
    if web_api.get("openapi") != "3.1.0":
        fail("contracts/web-api.openapi.yaml must use OpenAPI 3.1.0")
    paths = require_object(web_api.get("paths"), "web-api paths")
    if not paths:
        fail("web-api paths must not be empty")
    components = require_object(web_api.get("components"), "web-api components")
    require_object(components.get("schemas"), "web-api components.schemas")
    for path in ["/healthz", "/readyz", "/api/session/login", "/api/logs"]:
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
    validate_config_field_metadata(config_schema)


def iter_config_schema_leaves(
    config_schema: dict[str, Any],
    node: dict[str, Any],
    prefix: str,
) -> list[tuple[str, dict[str, Any]]]:
    ref = node.get("$ref")
    if isinstance(ref, str):
        ref_prefix = "#/$defs/"
        if not ref.startswith(ref_prefix):
            fail(f"config.user.schema.json unsupported $ref: {ref}")
        defs = require_object(config_schema.get("$defs"), "config schema $defs")
        target = require_object(defs.get(ref.removeprefix(ref_prefix)), f"config schema ref {ref}")
        return iter_config_schema_leaves(config_schema, target, prefix)

    properties = node.get("properties")
    if isinstance(properties, dict) and properties:
        leaves: list[tuple[str, dict[str, Any]]] = []
        for key, child in properties.items():
            leaves.extend(
                iter_config_schema_leaves(
                    config_schema,
                    require_object(child, f"config schema property {prefix}.{key}"),
                    f"{prefix}.{key}" if prefix else key,
                )
            )
        return leaves

    if not prefix:
        return []
    return [(prefix, node)]


def validate_config_field_metadata(config_schema: dict[str, Any]) -> None:
    allowed_apply_policies = {"hot_reload", "adapter_reload", "restart_required", "secret_only", "read_only"}
    leaves = iter_config_schema_leaves(config_schema, config_schema, "")
    missing_apply_policy: list[str] = []
    invalid_apply_policy: list[str] = []
    missing_redaction: list[str] = []
    for path, node in leaves:
        apply_policy = node.get("x-apply-policy")
        if not isinstance(apply_policy, str) or not apply_policy:
            missing_apply_policy.append(path)
        elif apply_policy not in allowed_apply_policies:
            invalid_apply_policy.append(f"{path}={apply_policy}")
        if node.get("x-secret") is True and not node.get("x-redaction"):
            missing_redaction.append(path)

    if missing_apply_policy:
        fail(f"config.user.schema.json fields missing x-apply-policy: {missing_apply_policy}")
    if invalid_apply_policy:
        fail(f"config.user.schema.json fields have invalid x-apply-policy: {invalid_apply_policy}")
    if missing_redaction:
        fail(f"config.user.schema.json secret fields missing x-redaction: {missing_redaction}")


def validate_release_basic(release_schema: dict[str, Any]) -> None:
    if "oneOf" not in release_schema:
        fail("release-manifest.schema.json must distinguish manifest, signature envelope, and build info via oneOf")
    artifact = require_object(release_schema.get("$defs", {}).get("artifact"), "release artifact")
    for field in [
        "artifact_id",
        "file_name",
        "platform",
        "sha256",
        "archive_size_bytes",
        "expanded_size_bytes",
        "file_count",
        "update_mode",
        "min_updater_protocol_version",
    ]:
        if field not in artifact.get("required", []):
            fail(f"release-manifest.schema.json artifact missing required field: {field}")


def validate_pr() -> dict[str, Any]:
    validate_required_files()
    documents = validate_parseable_documents()
    validate_fixture_refs(documents)
    validate_fixture_secret_scan()

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
    validate_contract_instances(web_api, websocket_events)

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
    for snippet in ["Go `1.25.12`", "Node.js `24.18.0`", "`pnpm 11.11.0`", "Python `3.12.13`"]:
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
    if "go 1.25.12" not in go_mod:
        fail("server/go.mod must pin Go 1.25.12")

    expected_pnpm_workspaces = {
        ROOT / "web" / "package.json": {
            "allowBuilds": {
                "@parcel/watcher": True,
                "core-js": False,
                "esbuild": True,
            },
            "overrides": {
                "esbuild": "0.28.1",
                "glob": "10.5.0",
                "immutable": "5.1.8",
                "js-cookie": "3.0.7",
                "js-yaml": "4.2.0",
                "picomatch": "4.0.4",
                "postcss": "8.5.18",
            },
        },
        ROOT / "launcher" / "package.json": {
            "allowBuilds": {
                "electron": True,
                "electron-winstaller": True,
            },
            "overrides": {
                "@fluentui/react-motion": "9.16.1",
                "@xmldom/xmldom": "0.8.13",
                "axios": "1.16.0",
                "follow-redirects": "1.16.0",
                "form-data": "4.0.6",
                "glob": "10.5.0",
                "ip-address": "10.1.1",
                "js-yaml": "4.2.0",
                "lodash": "4.18.0",
                "tar": "7.5.16",
                "tmp": "0.2.7",
                "undici": "7.28.0",
            },
        },
    }

    for package_path, expected_workspace in expected_pnpm_workspaces.items():
        package_json = load_json(package_path)
        if package_json.get("packageManager") != "pnpm@11.11.0":
            fail(f"{package_path.relative_to(ROOT)} packageManager must be pnpm@11.11.0")
        engines = package_json.get("engines", {})
        if engines.get("node") != "24.18.0":
            fail(f"{package_path.relative_to(ROOT)} engines.node must be 24.18.0")
        if engines.get("pnpm") != "11.11.0":
            fail(f"{package_path.relative_to(ROOT)} engines.pnpm must be 11.11.0")
        if "pnpm" in package_json:
            fail(f"{package_path.relative_to(ROOT)} must keep pnpm settings in pnpm-workspace.yaml")

        workspace_path = package_path.with_name("pnpm-workspace.yaml")
        workspace_config = require_object(load_yaml(workspace_path), f"{workspace_path.relative_to(ROOT)}")
        if workspace_config.get("packages") != ["."]:
            fail(f"{workspace_path.relative_to(ROOT)} packages must include only the project root")
        if workspace_config.get("supportedArchitectures") != {
            "os": ["win32", "linux", "darwin"],
            "cpu": ["x64", "arm64"],
            "libc": ["glibc", "musl"],
        }:
            fail(f"{workspace_path.relative_to(ROOT)} supportedArchitectures drifted")
        if workspace_config.get("allowBuilds") != expected_workspace["allowBuilds"]:
            fail(f"{workspace_path.relative_to(ROOT)} allowBuilds drifted")
        if workspace_config.get("overrides") != expected_workspace["overrides"]:
            fail(f"{workspace_path.relative_to(ROOT)} overrides drifted")


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
    expected = {"logs.appended", "events.received", "plugins.console"}
    if event_names != expected:
        fail(f"websocket event names drift: expected={sorted(expected)} actual={sorted(event_names)}")


def validate_strict_release(release_schema: dict[str, Any]) -> None:
    expected = {"windows-x64-full", "linux-x64-full", "macos-arm64-full", "linux-x64-server"}
    actual = set(release_schema["$defs"]["artifactId"].get("enum", []))
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
    expected = {"version", "update", "reset-admin", "backup", "restore", "doctor", "cleanup"}
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
