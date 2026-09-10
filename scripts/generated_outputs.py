"""Deterministic generated-output checking with explicit ownership markers."""
from pathlib import Path


def sync_outputs(root: Path, outputs: dict[Path, bytes], marker: bytes, verify: bool) -> list[str]:
    """Only remove stale files carrying this generator's marker in owned parents."""
    stale = []
    for directory in {path.parent for path in outputs}:
        if not directory.exists():
            continue
        for path in directory.iterdir():
            if path.is_file() and path not in outputs and marker in path.read_bytes()[:256]:
                stale.append(path)
    failures = [path.relative_to(root).as_posix() for path in stale]
    if not verify:
        for path in stale:
            path.unlink()
        failures.clear()
    for path, payload in outputs.items():
        if verify:
            if not path.is_file() or path.read_bytes().replace(b"\r\n", b"\n") != payload:
                failures.append(path.relative_to(root).as_posix())
        else:
            path.parent.mkdir(parents=True, exist_ok=True)
            if not path.is_file() or path.read_bytes() != payload:
                path.write_bytes(payload)
    return sorted(failures)
