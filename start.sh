#!/bin/sh
set -eu

# Development-only shortcut for building and starting the local Wails launcher.
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$SCRIPT_DIR"

LAUNCHER_DIR="$SCRIPT_DIR/launcher"

echo "[RayleaBot] Installing launcher dependencies..."
pnpm --dir "$LAUNCHER_DIR" install --frozen-lockfile

echo "[RayleaBot] Building launcher..."
pnpm --dir "$LAUNCHER_DIR" run build:app

if [ "${RAYLEA_START_SKIP_LAUNCH:-0}" = "1" ]; then
  exit 0
fi

echo "[RayleaBot] Starting launcher..."
(cd "$LAUNCHER_DIR" && GOWORK=off go run -tags gtk3 .)
