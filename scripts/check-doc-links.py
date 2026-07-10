#!/usr/bin/env python3
"""Check tracked Markdown links without making network requests."""

from __future__ import annotations

import argparse
import html
import re
import subprocess
import sys
import unicodedata
from dataclasses import dataclass
from functools import lru_cache
from pathlib import Path
from urllib.parse import unquote, urlsplit


REPO_ROOT = Path(__file__).resolve().parents[1]
FENCE_RE = re.compile(r"^\s{0,3}(`{3,}|~{3,})")
REFERENCE_RE = re.compile(r"^\s{0,3}\[[^]]+\]:\s*(?:<([^>]+)>|(\S+))")
HTML_TARGET_RE = re.compile(r"\b(?:href|src)\s*=\s*(['\"])(.*?)\1", re.IGNORECASE)
EXPLICIT_ANCHOR_RE = re.compile(
    r"<(?:a|span|h[1-6])\b[^>]*\b(?:id|name)\s*=\s*(['\"])(.*?)\1",
    re.IGNORECASE,
)
ATX_HEADING_RE = re.compile(r"^\s{0,3}#{1,6}\s+(.+?)\s*#*\s*$")
SETEXT_RE = re.compile(r"^\s{0,3}(?:=+|-+)\s*$")
SCHEME_RE = re.compile(r"^[A-Za-z][A-Za-z0-9+.-]*:")


@dataclass(frozen=True)
class LinkReference:
    source: Path
    line: int
    target: str


def tracked_markdown_files(root: Path) -> list[Path]:
    completed = subprocess.run(
        [
            "git",
            "-c",
            "core.quotepath=false",
            "ls-files",
            "--cached",
            "--others",
            "--exclude-standard",
            "-z",
            "--",
            "*.md",
        ],
        cwd=root,
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if completed.returncode != 0:
        detail = completed.stderr.decode("utf-8", errors="replace").strip()
        raise RuntimeError(f"git ls-files failed: {detail or completed.returncode}")
    paths = [Path(raw.decode("utf-8")) for raw in completed.stdout.split(b"\0") if raw]
    return sorted(resolved for path in paths if (resolved := (root / path).resolve()).is_file())


def remove_inline_code(line: str) -> str:
    return re.sub(r"(`+)(.*?)\1", "", line)


def inline_link_targets(line: str) -> list[str]:
    targets: list[str] = []
    cursor = 0
    while True:
        marker = line.find("](", cursor)
        if marker < 0:
            return targets
        position = marker + 2
        while position < len(line) and line[position].isspace():
            position += 1
        if position < len(line) and line[position] == "<":
            end = line.find(">", position + 1)
            if end >= 0:
                targets.append(line[position + 1 : end])
                cursor = end + 1
                continue

        start = position
        depth = 0
        escaped = False
        while position < len(line):
            char = line[position]
            if escaped:
                escaped = False
            elif char == "\\":
                escaped = True
            elif char == "(":
                depth += 1
            elif char == ")":
                if depth == 0:
                    break
                depth -= 1
            elif char.isspace() and depth == 0:
                break
            position += 1
        targets.append(line[start:position])
        cursor = max(position + 1, marker + 2)


def markdown_links(path: Path) -> list[LinkReference]:
    links: list[LinkReference] = []
    fence_char = ""
    fence_length = 0
    for line_number, raw_line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        fence = FENCE_RE.match(raw_line)
        if fence:
            marker = fence.group(1)
            if not fence_char:
                fence_char, fence_length = marker[0], len(marker)
            elif marker[0] == fence_char and len(marker) >= fence_length:
                fence_char, fence_length = "", 0
            continue
        if fence_char or raw_line.startswith(("    ", "\t")):
            continue

        line = remove_inline_code(raw_line)
        reference = REFERENCE_RE.match(line)
        if reference:
            links.append(LinkReference(path, line_number, reference.group(1) or reference.group(2) or ""))
        links.extend(LinkReference(path, line_number, target) for target in inline_link_targets(line))
        links.extend(LinkReference(path, line_number, match.group(2)) for match in HTML_TARGET_RE.finditer(line))
    return links


def normalize_heading_text(value: str) -> str:
    value = html.unescape(value)
    value = re.sub(r"!\[([^]]*)\]\([^)]*\)", r"\1", value)
    value = re.sub(r"\[([^]]+)\]\([^)]*\)", r"\1", value)
    value = re.sub(r"<[^>]+>", "", value)
    value = value.replace("`", "").replace("*", "").replace("~", "")
    value = value.strip().lower()
    normalized: list[str] = []
    pending_hyphen = False
    for char in value:
        if char.isspace():
            pending_hyphen = bool(normalized)
            continue
        if unicodedata.category(char).startswith("P") and char not in {"-", "_"}:
            continue
        if pending_hyphen and normalized[-1] != "-":
            normalized.append("-")
        normalized.append(char)
        pending_hyphen = False
    return "".join(normalized).strip("-")


@lru_cache(maxsize=None)
def markdown_anchors(path: Path) -> frozenset[str]:
    lines = path.read_text(encoding="utf-8").splitlines()
    anchors: set[str] = set()
    counts: dict[str, int] = {}
    previous = ""
    for line in lines:
        anchors.update(match.group(2) for match in EXPLICIT_ANCHOR_RE.finditer(line))
        heading = ATX_HEADING_RE.match(line)
        heading_text = heading.group(1) if heading else previous.strip() if previous and SETEXT_RE.match(line) else ""
        if heading_text:
            base = normalize_heading_text(heading_text)
            if base:
                duplicate = counts.get(base, 0)
                anchors.add(base if duplicate == 0 else f"{base}-{duplicate}")
                counts[base] = duplicate + 1
        previous = "" if not line.strip() else line
    return frozenset(anchors)


def exact_case_exists(path: Path, root: Path) -> bool:
    try:
        relative = path.relative_to(root)
    except ValueError:
        return False
    current = root
    for part in relative.parts:
        if not current.is_dir() or part not in {entry.name for entry in current.iterdir()}:
            return False
        current /= part
    return current.exists()


def external_uri_error(target: str) -> str | None:
    if any(char.isspace() or ord(char) < 32 for char in target):
        return "external URI contains whitespace or control characters"
    try:
        parsed = urlsplit(target)
        if parsed.scheme.lower() in {"http", "https"} and (not parsed.netloc or not parsed.hostname):
            return "HTTP(S) URI has no host"
        if parsed.scheme.lower() == "mailto" and not parsed.path:
            return "mailto URI has no recipient"
        if target.startswith("//") and (not parsed.netloc or not parsed.hostname):
            return "protocol-relative URI has no host"
        if SCHEME_RE.match(target) and not (parsed.netloc or parsed.path):
            return "URI has no scheme-specific target"
    except ValueError as exc:
        return f"invalid URI: {exc}"
    return None


def local_target(reference: LinkReference, root: Path) -> tuple[Path, str] | str:
    target = reference.target.replace("\\(", "(").replace("\\)", ")")
    path_part, separator, fragment = target.partition("#")
    path_part = path_part.split("?", 1)[0]
    decoded_path = unquote(path_part)
    decoded_fragment = unquote(fragment) if separator else ""
    if "\0" in decoded_path:
        return "local path contains a NUL byte"
    if decoded_path.startswith("/"):
        resolved = (root / decoded_path.lstrip("/")).resolve()
    elif decoded_path:
        resolved = (reference.source.parent / decoded_path).resolve()
    else:
        resolved = reference.source.resolve()
    try:
        resolved.relative_to(root)
    except ValueError:
        return f"local path escapes repository: {decoded_path}"
    return resolved, decoded_fragment


def validate_link(reference: LinkReference, root: Path) -> str | None:
    target = reference.target.strip()
    if SCHEME_RE.match(target) or target.startswith("//"):
        return external_uri_error(target)
    parsed = local_target(LinkReference(reference.source, reference.line, target), root)
    if isinstance(parsed, str):
        return parsed
    path, fragment = parsed
    if not path.exists():
        return f"missing local target: {target or '.'}"
    if not exact_case_exists(path, root):
        return f"local target has incorrect path casing: {target or '.'}"
    if not fragment:
        return None
    anchor_path = path / "README.md" if path.is_dir() else path
    if anchor_path.suffix.lower() != ".md":
        return None
    if not anchor_path.is_file():
        return f"anchor target is not a Markdown file: {target}"
    if fragment not in markdown_anchors(anchor_path):
        return f"missing anchor #{fragment} in {anchor_path.relative_to(root).as_posix()}"
    return None


def check_files(root: Path, files: list[Path]) -> list[str]:
    errors: list[str] = []
    for path in files:
        for reference in markdown_links(path):
            error = validate_link(reference, root)
            if error:
                errors.append(f"{path.relative_to(root).as_posix()}:{reference.line}: {error}")
    return errors


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=REPO_ROOT)
    parser.add_argument("files", nargs="*")
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(sys.argv[1:] if argv is None else argv)
    root = args.root.resolve()
    try:
        files = [((root / item).resolve() if not Path(item).is_absolute() else Path(item).resolve()) for item in args.files]
        files = files or tracked_markdown_files(root)
        errors = check_files(root, files)
    except (OSError, RuntimeError, UnicodeError) as exc:
        print(f"documentation link check failed: {exc}", file=sys.stderr)
        return 1
    if errors:
        print("documentation link check failed:", file=sys.stderr)
        for error in errors:
            print(f"- {error}", file=sys.stderr)
        return 1
    print(f"documentation link check passed ({len(files)} Markdown files)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
