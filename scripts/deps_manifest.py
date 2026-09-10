"""Managed resource validation shared by contract checks and release packaging."""
from __future__ import annotations

import json
from functools import lru_cache
from pathlib import Path
from urllib.parse import urlsplit

from jsonschema import Draft202012Validator, FormatChecker


@lru_cache(maxsize=1)
def validator() -> Draft202012Validator:
    schema = json.loads((Path(__file__).resolve().parent.parent / "contracts/deps-manifest.schema.json").read_text(encoding="utf-8"))
    return Draft202012Validator(schema, format_checker=FormatChecker())


def semantic_errors(manifest: object) -> list[str]:
    if not isinstance(manifest, dict) or not isinstance(manifest.get("resources"), list):
        return []
    errors: list[str] = []
    identifiers: set[str] = set()
    kinds: set[tuple[str, str]] = set()
    for resource in manifest["resources"]:
        if not isinstance(resource, dict):
            continue
        identifier = resource.get("id")
        key = (resource.get("platform"), resource.get("kind"))
        if isinstance(identifier, str) and all(isinstance(value, str) for value in key):
            if identifier in identifiers or key in kinds:
                errors.append("resource IDs and platform/kind pairs must be unique")
            identifiers.add(identifier)
            kinds.add(key)
        sources = resource.get("sources")
        if not isinstance(sources, list):
            continue
        seen: set[str] = set()
        for source in sources:
            if not isinstance(source, dict) or not isinstance(source.get("url"), str):
                continue
            url = source["url"]
            try:
                parsed = urlsplit(url)
                valid = parsed.scheme == "https" and bool(parsed.hostname) and parsed.username is None and not parsed.fragment
            except ValueError:
                valid = False
            if not valid:
                errors.append("source requires HTTPS, hostname, and no credentials or fragment")
            if url in seen:
                errors.append("source URLs must be unique")
            seen.add(url)
    return errors


def validate_manifest(manifest: object) -> None:
    validator().validate(manifest)
    errors = semantic_errors(manifest)
    if errors:
        raise ValueError("; ".join(errors))
