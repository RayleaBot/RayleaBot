"""Current release artifact layouts shared by packaging and content checks."""
from pathlib import Path
import sys
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from artifact_ids_generated import ARTIFACT_WINDOWS_X64_FULL, ARTIFACT_LINUX_X64_FULL, ARTIFACT_MACOS_ARM64_FULL, ARTIFACT_LINUX_X64_SERVER

COMMON_PATHS = frozenset({
    "build_info.json", "LICENSE", "THIRD_PARTY_NOTICES.md", "web/dist/index.html",
    ".deps/manifest.json", "templates/help.menu/template.json", "templates/status.panel/template.json",
})

ARTIFACT_MATRIX = {
    ARTIFACT_WINDOWS_X64_FULL: {
        "platform": "windows-x64", "support_level": "first_class", "smoke_profile": "windows_full_smoke",
        "extension": ".zip", "archive_type": "zip", "launcher_required": True,
        "server_binary": "raylea-server.exe",
        "required_paths": COMMON_PATHS | {"raylea-server.exe", "raylea-updater.exe", "RayleaLauncher.exe", "WINDOWS-RUNTIME.md"},
    },
    ARTIFACT_LINUX_X64_FULL: {
        "platform": "linux-x64", "support_level": "first_class", "smoke_profile": "linux_full_smoke",
        "extension": ".tar.gz", "archive_type": "tar.gz", "launcher_required": True,
        "server_binary": "raylea-server",
        "required_paths": COMMON_PATHS | {"raylea-server", "RayleaLauncher", "LINUX-RUNTIME.md"},
    },
    ARTIFACT_MACOS_ARM64_FULL: {
        "platform": "macos-arm64", "support_level": "first_class", "smoke_profile": "macos_full_smoke",
        "extension": ".tar.gz", "archive_type": "tar.gz", "launcher_required": True,
        "server_binary": "raylea-server",
        "required_paths": COMMON_PATHS | {"raylea-server", "RayleaLauncher.app/Contents/MacOS/RayleaLauncher"},
    },
    ARTIFACT_LINUX_X64_SERVER: {
        "platform": "linux-x64", "support_level": "first_class", "smoke_profile": "linux_server_smoke",
        "extension": ".tar.gz", "archive_type": "tar.gz", "launcher_required": False,
        "server_binary": "raylea-server",
        "required_paths": COMMON_PATHS | {"raylea-server", "systemd/rayleabot.service"},
    },
}

REQUIRED_PATHS = {name: definition["required_paths"] for name, definition in ARTIFACT_MATRIX.items()}
SERVER_BINARIES = {name: definition["server_binary"] for name, definition in ARTIFACT_MATRIX.items()}
