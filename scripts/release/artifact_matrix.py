"""Current release artifact layouts shared by packaging and content checks."""

COMMON_PATHS = frozenset({
    "build_info.json", "LICENSE", "THIRD_PARTY_NOTICES.md", "web/dist/index.html",
    ".deps/manifest.json", "templates/help.menu/template.json", "templates/status.panel/template.json",
})

ARTIFACT_MATRIX = {
    "windows-x64-full": {
        "platform": "windows-x64", "support_level": "first_class", "smoke_profile": "windows_full_smoke",
        "extension": ".zip", "archive_type": "zip", "launcher_required": True,
        "server_binary": "raylea-server.exe",
        "required_paths": COMMON_PATHS | {"raylea-server.exe", "raylea-updater.exe", "RayleaLauncher.exe", "WINDOWS-RUNTIME.md"},
    },
    "linux-x64-full": {
        "platform": "linux-x64", "support_level": "first_class", "smoke_profile": "linux_full_smoke",
        "extension": ".tar.gz", "archive_type": "tar.gz", "launcher_required": True,
        "server_binary": "raylea-server",
        "required_paths": COMMON_PATHS | {"raylea-server", "RayleaLauncher", "LINUX-RUNTIME.md"},
    },
    "macos-arm64-full": {
        "platform": "macos-arm64", "support_level": "first_class", "smoke_profile": "macos_full_smoke",
        "extension": ".tar.gz", "archive_type": "tar.gz", "launcher_required": True,
        "server_binary": "raylea-server",
        "required_paths": COMMON_PATHS | {"raylea-server", "RayleaLauncher.app/Contents/MacOS/RayleaLauncher"},
    },
    "linux-x64-server": {
        "platform": "linux-x64", "support_level": "first_class", "smoke_profile": "linux_server_smoke",
        "extension": ".tar.gz", "archive_type": "tar.gz", "launcher_required": False,
        "server_binary": "raylea-server",
        "required_paths": COMMON_PATHS | {"raylea-server", "systemd/rayleabot.service"},
    },
}

REQUIRED_PATHS = {name: definition["required_paths"] for name, definition in ARTIFACT_MATRIX.items()}
SERVER_BINARIES = {name: definition["server_binary"] for name, definition in ARTIFACT_MATRIX.items()}
