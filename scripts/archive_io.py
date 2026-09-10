"""Bounded archive I/O for private release/runtime staging directories."""
from __future__ import annotations

import contextlib
import gzip
import io
import lzma
import os
import posixpath
import stat
import tarfile
import zipfile
from pathlib import Path, PurePosixPath

MAX_ARCHIVE_BYTES = 2 << 30
MAX_FILE_BYTES = 2 << 30
MAX_EXPANDED_BYTES = 8 << 30
MAX_ENTRIES = 100_000


def relative_name(name: str) -> str:
    if not name or any(char in name for char in "\\:\x00") or name.startswith("/"):
        raise ValueError("archive requires a relative slash path")
    clean = posixpath.normpath(name)
    if clean in {".", ".."} or clean.startswith("../"):
        raise ValueError("archive entry escapes root")
    for part in clean.split("/"):
        base = part.split(".", 1)[0].upper()
        if (part.endswith((" ", ".")) or any(ord(char) < 32 or char in '<>"|?*' for char in part)
                or base in {"CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$"}
                or len(base) == 4 and base[:3] in {"COM", "LPT"} and base[3] in "123456789¹²³"):
            raise ValueError("archive contains an unsafe portable path")
    return clean


def copy_bounded(source, target, limit: int) -> int:
    written = 0
    while True:
        chunk = source.read(min(1024 * 1024, limit - written + 1))
        if not chunk:
            return written
        if written + len(chunk) > limit:
            raise ValueError("stream exceeds size limit")
        target.write(chunk)
        written += len(chunk)


class _XZReader(io.RawIOBase):
    """XZ decoding with a real dictionary limit and bounded output buffers."""
    def __init__(self, source):
        self.source = source
        self.decoder = lzma.LZMADecompressor(memlimit=66 << 20)
        self.pending = b""
        self.complete = False

    def readable(self):
        return True

    def readinto(self, buffer):
        if self.complete:
            return 0
        while True:
            data = b""
            if self.decoder.needs_input:
                data, self.pending = self.pending, b""
                if not data:
                    data = self.source.read(65536)
                if not data:
                    raise EOFError("truncated XZ stream")
            output = self.decoder.decompress(data, max_length=len(buffer))
            if self.decoder.eof:
                self.pending = self.decoder.unused_data
                # Read through legal stream padding and permit concatenated XZ.
                padding = 0
                while True:
                    if not self.pending:
                        self.pending = self.source.read(65536)
                        if not self.pending:
                            self.complete = True
                            break
                    trimmed = self.pending.lstrip(b"\x00")
                    padding += len(self.pending) - len(trimmed)
                    self.pending = trimmed
                    if trimmed:
                        break
                if padding % 4:
                    raise ValueError("invalid XZ stream padding")
                if not self.complete:
                    self.decoder = lzma.LZMADecompressor(memlimit=66 << 20)
            if output or self.complete:
                buffer[:len(output)] = output
                return len(output)


def extract_archive(archive: Path, destination: Path, *, allow_links: bool = False) -> list[str]:
    if archive.stat().st_size > MAX_ARCHIVE_BYTES:
        raise ValueError("archive exceeds size limit")
    destination.mkdir(parents=True, exist_ok=True)
    root = destination.resolve(strict=True)
    seen: set[str] = set()
    names: list[str] = []
    links: list[tuple[str, str, bool]] = []
    expanded = 0

    def target_for(name: str) -> Path:
        target = root.joinpath(*PurePosixPath(name).parts)
        if not target.resolve().is_relative_to(root):
            raise ValueError("archive path escapes destination")
        for parent in (target, *target.parents):
            if parent == root:
                break
            if parent.is_symlink():
                raise ValueError("archive path contains a symlink")
        return target

    def entry(name: str, size: int, mode: int, source=None, link: str | None = None, hard=False, directory=False):
        nonlocal expanded
        clean = relative_name(name)
        key = clean.casefold()
        if key in seen or len(seen) >= MAX_ENTRIES:
            raise ValueError("duplicate path or archive entry limit")
        seen.add(key)
        names.append(clean)
        if size < 0 or size > MAX_FILE_BYTES or expanded + size > MAX_EXPANDED_BYTES:
            raise ValueError("archive expanded size limit")
        expanded += size
        target = target_for(clean)
        if link is not None:
            if not allow_links or not link or any(char in link for char in "\\:\x00") or link.startswith("/"):
                raise ValueError("unsafe archive link")
            resolved = relative_name(link if hard else posixpath.join(posixpath.dirname(clean), link))
            target_for(resolved)
            links.append((clean, link, hard))
        elif directory:
            target.mkdir(parents=True, exist_ok=True)
        else:
            target.parent.mkdir(parents=True, exist_ok=True)
            with target.open("xb") as output:
                if copy_bounded(source, output, size) != size:
                    raise ValueError("archive file size mismatch")
            target.chmod(mode & 0o777)

    if zipfile.is_zipfile(archive):
        with zipfile.ZipFile(archive) as reader:
            for member in reader.infolist():
                mode = member.external_attr >> 16
                if member.is_dir():
                    entry(member.filename, 0, mode, directory=True)
                elif stat.S_ISLNK(mode):
                    if member.file_size > 4096:
                        raise ValueError("archive link too long")
                    entry(member.filename, member.file_size, mode, link=reader.read(member).decode("utf-8"))
                elif stat.S_IFMT(mode) not in {0, stat.S_IFREG}:
                    raise ValueError("unsupported archive file type")
                else:
                    with reader.open(member) as source:
                        entry(member.filename, member.file_size, mode or 0o644, source)
    else:
        with contextlib.ExitStack() as stack:
            file = stack.enter_context(archive.open("rb"))
            compressed = False
            if file.peek(6).startswith(b"\xfd7zXZ\x00"):
                file = stack.enter_context(io.BufferedReader(_XZReader(file)))
                compressed = True
            elif file.peek(2).startswith(b"\x1f\x8b"):
                file = stack.enter_context(gzip.GzipFile(fileobj=file))
                compressed = True
            reader = stack.enter_context(tarfile.open(fileobj=file, mode="r|"))
            for member in reader:
                if member.isdir() and member.name in {".", "./"}:
                    continue
                if member.isdir():
                    entry(member.name, 0, member.mode, directory=True)
                elif member.issym() or member.islnk():
                    entry(member.name, 0, member.mode, link=member.linkname, hard=member.islnk())
                elif member.isfile():
                    with reader.extractfile(member) as source:
                        entry(member.name, member.size, member.mode, source)
                else:
                    raise ValueError("unsupported archive file type")
            if compressed:
                copy_bounded(file, io.BytesIO(), 1 << 20)
    # Regular files never traverse archive links; links are installed last.
    for name, link, hard in links:
        target = target_for(name)
        target.parent.mkdir(parents=True, exist_ok=True)
        if hard:
            os.link(target_for(relative_name(link)), target)
        else:
            target.symlink_to(link, target_is_directory=(target.parent / link).is_dir())
    for name, _, _ in links:
        if not root.joinpath(name).resolve(strict=True).is_relative_to(root):
            raise ValueError("archive link resolves outside destination")
    return names
