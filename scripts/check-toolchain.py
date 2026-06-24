#!/usr/bin/env python3
from __future__ import annotations

import shutil
import subprocess
import sys

REQUIRED_GO_VERSION = "go1.25.8"
GO_INSTALL_URL = "https://go.dev/dl/"


def main() -> int:
    go_bin = shutil.which("go")
    if go_bin is None:
        print(
            f"Go toolchain not found. Required: {REQUIRED_GO_VERSION}. Install Go from {GO_INSTALL_URL}.",
            file=sys.stderr,
        )
        return 1

    try:
        result = subprocess.run(
            [go_bin, "version"],
            check=True,
            capture_output=True,
            text=True,
        )
    except subprocess.CalledProcessError as exc:
        message = (exc.stderr or exc.stdout or str(exc)).strip()
        print(f"Unable to read Go toolchain version: {message}", file=sys.stderr)
        return 1

    version_output = result.stdout.strip()
    parts = version_output.split()
    actual = parts[2] if len(parts) >= 3 else ""
    if actual != REQUIRED_GO_VERSION:
        print(
            f"Go toolchain mismatch. Found: {actual or version_output}; required: {REQUIRED_GO_VERSION}. "
            f"Install the required version from {GO_INSTALL_URL}.",
            file=sys.stderr,
        )
        return 1

    print(f"toolchain check passed: {actual}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
