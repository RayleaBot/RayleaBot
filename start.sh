#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$SCRIPT_DIR"

NODE_VERSION=""
while read -r tool version _rest; do
  if [ "$tool" = "nodejs" ]; then
    NODE_VERSION="$version"
    break
  fi
done < "$SCRIPT_DIR/.tool-versions"

if [ -z "$NODE_VERSION" ]; then
  echo "[RayleaBot] Startup failed: .tool-versions does not declare Node.js." >&2
  exit 1
fi

NODE_BIN=${RAYLEA_NODE_EXECUTABLE:-}
if [ -n "$NODE_BIN" ]; then
  case "$NODE_BIN" in
    /*) ;;
    *) echo "[RayleaBot] RAYLEA_NODE_EXECUTABLE must be an absolute path." >&2; exit 1 ;;
  esac
  if [ ! -f "$NODE_BIN" ] || [ ! -x "$NODE_BIN" ]; then
    echo "[RayleaBot] RAYLEA_NODE_EXECUTABLE is not an executable file: $NODE_BIN" >&2
    exit 1
  fi
else
  NODE_BIN=$(command -v node 2>/dev/null || true)
fi
if [ -z "$NODE_BIN" ]; then
  echo "[RayleaBot] Startup failed: Node.js $NODE_VERSION was not found." >&2
  echo "[RayleaBot] Run python scripts/check-toolchain.py for installation guidance." >&2
  exit 1
fi

NODE_ACTUAL=$("$NODE_BIN" --version 2>/dev/null || true)
if [ "$NODE_ACTUAL" != "v$NODE_VERSION" ]; then
  echo "[RayleaBot] Startup failed: Node.js version mismatch." >&2
  echo "[RayleaBot] Current version: ${NODE_ACTUAL:-unavailable}" >&2
  echo "[RayleaBot] Required version: v$NODE_VERSION" >&2
  echo "[RayleaBot] Executable: $NODE_BIN" >&2
  exit 1
fi

echo "[RayleaBot] Using Node.js $NODE_ACTUAL."
exec "$NODE_BIN" scripts/start-dev.mjs "$@"
