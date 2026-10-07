"""Publish complete GitHub releases to an S3-compatible update mirror."""
from __future__ import annotations

import argparse
import copy
import json
from pathlib import Path
import re
import subprocess
import tempfile
from urllib.parse import urlparse

from release_policy import release_channel
from release_tool import validate_release_metadata


def version_key(version: str) -> tuple:
    release_channel(version)
    core, _, prerelease = version.split("+", 1)[0].partition("-")
    parts = tuple((0, int(part)) if part.isdigit() else (1, part) for part in prerelease.split("."))
    return (*map(int, core.split(".")), not bool(prerelease), parts)


def rewrite_manifest(manifest: dict, base: str) -> dict:
    validate_release_metadata(manifest)
    parsed = urlparse(base)
    if parsed.scheme != "https" or not parsed.hostname or parsed.username or parsed.query or parsed.fragment:
        raise ValueError("mirror requires an HTTPS public base URL")
    result = copy.deepcopy(manifest)
    for artifact in result["artifacts"]:
        artifact["download_url"] = f"{base.rstrip('/')}/v{result['version']}/{artifact['file_name']}"
    validate_release_metadata(result)
    return result


def channel_documents(manifests: list[dict]) -> dict:
    ordered = sorted(manifests, key=lambda entry: version_key(entry["version"]), reverse=True)[:30]
    if not ordered:
        raise ValueError("no compatible published release to mirror")
    result = {"releases.json": [{"version": entry["version"], "channel": entry["channel"], "release_notes_ref": entry["release_notes_ref"]} for entry in ordered], "beta.json": ordered[0]}
    stable = next((entry for entry in ordered if entry["channel"] == "stable"), None)
    if stable:
        result["stable.json"] = stable
    return result


def run(*args: str) -> str:
    return subprocess.check_output(args, text=True, encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repository", required=True)
    parser.add_argument("--base-url", required=True)
    parser.add_argument("--bucket", required=True)
    parser.add_argument("--endpoint", required=True)
    args = parser.parse_args()
    if urlparse(args.endpoint).scheme != "https" or not urlparse(args.endpoint).hostname:
        raise ValueError("object storage endpoint must use HTTPS")
    if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", args.repository):
        raise ValueError("invalid repository")
    if not re.fullmatch(r"[a-z0-9][a-z0-9.-]+", args.bucket):
        raise ValueError("invalid bucket")
    releases = json.loads(run("gh", "api", f"repos/{args.repository}/releases?per_page=100"))
    candidates = []
    for release in releases:
        if release["draft"] or not release["tag_name"].startswith("v"):
            continue
        try:
            key = version_key(release["tag_name"][1:])
        except ValueError:
            continue
        if any(asset["name"] == "release_manifest.v2.json" for asset in release["assets"]):
            candidates.append((key, release))
    candidates.sort(key=lambda entry: entry[0], reverse=True)
    manifests = []
    with tempfile.TemporaryDirectory(prefix="raylea-mirror-") as temporary:
        root = Path(temporary)

        def upload(path: Path, key: str, mutable: bool = False) -> None:
            run("aws", "s3", "cp", str(path), f"s3://{args.bucket}/{key}", "--endpoint-url", args.endpoint,
                "--cache-control", "public,max-age=300" if mutable else "public,max-age=31536000,immutable",
                "--content-type", "application/json" if path.suffix == ".json" else "application/octet-stream",
                "--only-show-errors")

        for _, release in candidates[:30]:
            tag = release["tag_name"]
            directory = root / tag
            directory.mkdir()
            run("gh", "release", "download", tag, "--repo", args.repository, "--pattern", "release_manifest.v2.json", "--dir", str(directory))
            source = json.loads((directory / "release_manifest.v2.json").read_text(encoding="utf-8"))
            # Pre-0.4 metadata has a different download contract and is never
            # advertised by this mirror. Current-format invalid metadata fails.
            if version_key(source["version"]) < version_key("0.4.0-0"):
                continue
            if tag != "v" + source["version"]:
                raise ValueError("release tag and manifest version disagree")
            mirrored = rewrite_manifest(source, args.base_url)
            inventory = json.loads(run("aws", "s3api", "list-objects-v2", "--bucket", args.bucket,
                                       "--prefix", tag + "/", "--endpoint-url", args.endpoint, "--output", "json"))
            existing = {item["Key"]: item["Size"] for item in inventory.get("Contents", [])}
            for artifact in source["artifacts"]:
                name = artifact["file_name"]
                # Versioned assets are immutable; a complete earlier upload is
                # reusable. A partial upload or absent asset is downloaded again.
                if existing.get(f"{tag}/{name}") == artifact["archive_size_bytes"]:
                    continue
                run("gh", "release", "download", tag, "--repo", args.repository, "--pattern", name, "--dir", str(directory))
                path = directory / name
                if path.stat().st_size != artifact["archive_size_bytes"]:
                    raise ValueError(f"archive size mismatch: {tag}/{name}")
                upload(path, f"{tag}/{name}")
                path.unlink()
            path = directory / "release_manifest.v2.json"
            path.write_text(json.dumps(mirrored, ensure_ascii=False), encoding="utf-8")
            upload(path, f"{tag}/release_manifest.v2.json")
            manifests.append(mirrored)
        # A release published or promoted during synchronization must not be
        # hidden by pointers based on the earlier snapshot.
        refreshed = json.loads(run("gh", "api", f"repos/{args.repository}/releases?per_page=100"))
        def release_state(entries: list[dict]) -> set:
            return {(entry["tag_name"], entry["draft"], entry["prerelease"]) for entry in entries}
        if release_state(refreshed) != release_state(releases):
            raise RuntimeError("release list changed during sync; rerun before publishing channel metadata")
        # Root pointers are published only after all indexed assets exist.
        for name, document in channel_documents(manifests).items():
            path = root / name
            path.write_text(json.dumps(document, ensure_ascii=False), encoding="utf-8")
            upload(path, name, mutable=True)
    print(f"Published {len(manifests)} complete releases to {args.base_url}")


if __name__ == "__main__":
    main()
