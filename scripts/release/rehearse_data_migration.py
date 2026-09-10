"""Exercise legacy config, password and SQLite recovery using a real Server binary."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import socket
import sqlite3
import subprocess
import time
import urllib.error
import urllib.request
import zipfile

import yaml


def run(binary: Path, root: Path, *args: str) -> None:
    with (root / "cli.log").open("a", encoding="utf-8") as log:
        subprocess.run([str(binary), "-config", str(root / "config/user.yaml"), *args], cwd=root,
                       stdout=log, stderr=subprocess.STDOUT, check=True, timeout=60)


def request(origin: str, path: str, *, data: dict | None = None, token: str = "", control: bool = False) -> dict:
    headers = {"Origin": origin, "Content-Type": "application/json", "X-Raylea-Session-Transport": "bearer"}
    if token:
        headers["Authorization"] = "Bearer " + token
    if control:
        headers["X-Raylea-Launcher-Control"] = "B" * 43
    body = json.dumps(data).encode() if data is not None else None
    with urllib.request.urlopen(urllib.request.Request(origin + path, data=body, headers=headers), timeout=5) as response:
        raw = response.read()
        return json.loads(raw) if raw else {}


def rehearse(repo: Path, binary: Path, output: Path) -> dict:
    legacy = output / "legacy"
    restored = output / "restored"
    legacy.mkdir(parents=True)
    restored.mkdir()
    run(binary, legacy, "config", "init")
    with socket.socket() as probe:
        probe.bind(("127.0.0.1", 0))
        port = probe.getsockname()[1]
    config_path = legacy / "config/user.yaml"
    config = yaml.safe_load(config_path.read_text(encoding="utf-8"))
    config["schema_version"] = "3"
    config.pop("adapters", None)
    config["onebot"] = {name: {"enabled": False, "url": "", "access_token": ""}
                        for name in ("forward_ws", "reverse_ws", "http_api", "webhook")}
    config["server"].update(host="127.0.0.1", port=port)
    config_path.write_text(yaml.safe_dump(config, allow_unicode=True), encoding="utf-8")
    database = legacy / "data/rayleabot.db"
    database.parent.mkdir(exist_ok=True)
    with sqlite3.connect(database) as connection:
        for migration in sorted((repo / "server/internal/storage/migrations").glob("*.sql")):
            version = int(migration.name.split("_", 1)[0])
            if version > 7:
                break
            connection.executescript(migration.read_text(encoding="utf-8"))
            connection.execute("INSERT INTO schema_migrations VALUES (?, ?, ?)",
                               (version, migration.stem, "2026-09-09T00:00:00Z"))
        connection.execute("INSERT INTO auth_bootstrap_state VALUES (1, ?, ?, ?, ?)",
                           ("admin", hashlib.sha256(b"fixture-only-secret").digest(), os.urandom(32), "2026-09-09T00:00:00Z"))
        for table in ("blacklist_entries", "whitelist_entries"):
            connection.execute(f"INSERT INTO {table} (entry_type,target_id,reason,created_at) VALUES ('user','1001','migration fixture','2026-01-01T00:00:00Z')")
        connection.execute("UPDATE whitelist_state SET enabled=1")

    run(binary, legacy, "backup")
    archive = next((legacy / "backups").glob("*.zip"))
    with zipfile.ZipFile(archive) as package:
        manifest = json.loads(package.read("backup-manifest.json"))
    assert manifest["config_schema_version"] == "3", manifest
    assert manifest["db_schema_version"] == "000007", manifest

    run(binary, restored, "config", "init")
    run(binary, restored, "restore", str(archive))
    origin = f"http://127.0.0.1:{port}"
    environment = {**os.environ, "RAYLEA_SETUP_TOKEN": "A" * 43, "RAYLEA_LAUNCHER_CONTROL_TOKEN": "B" * 43}
    with (restored / "server.log").open("w", encoding="utf-8") as log:
        server = subprocess.Popen([str(binary), "-config", str(restored / "config/user.yaml")], cwd=restored,
                                  env=environment, stdout=log, stderr=subprocess.STDOUT,
                                  creationflags=subprocess.CREATE_NO_WINDOW if os.name == "nt" else 0)
        try:
            deadline = time.monotonic() + 30
            while True:
                if server.poll() is not None:
                    raise RuntimeError(f"Server exited: inspect {restored / 'server.log'}")
                try:
                    request(origin, "/healthz")
                    break
                except (OSError, urllib.error.URLError):
                    if time.monotonic() >= deadline:
                        raise TimeoutError("restored Server did not become healthy")
                    time.sleep(0.1)
            session = request(origin, "/api/session/login", data={"identifier": "admin", "secret": "fixture-only-secret"})
            rules = request(origin, "/api/governance/blacklist", token=session["session_token"])
            assert rules["user_entries"][0]["scope"] == {
                "kind": "global", "source_protocol": "onebot11", "source_adapter": "", "bot_id": ""}
        finally:
            if server.poll() is None:
                try:
                    request(origin, "/api/launcher/shutdown", data={}, control=True)
                    server.wait(timeout=10)
                except (OSError, subprocess.TimeoutExpired):
                    server.kill()
                    server.wait(timeout=5)
    assert server.returncode == 0, server.returncode
    with sqlite3.connect(restored / "data/rayleabot.db") as connection:
        version = connection.execute("SELECT MAX(version) FROM schema_migrations").fetchone()[0]
        digest = connection.execute("SELECT secret_digest FROM auth_bootstrap_state").fetchone()[0]
        entries = connection.execute("SELECT COUNT(*) FROM access_list_entries WHERE target_id='1001'").fetchone()[0]
        enabled = connection.execute("SELECT enabled FROM whitelist_state").fetchone()[0]
    assert version >= 8 and entries == 2 and enabled == 1
    assert (digest.startswith(b"raylea-pwd:") and b":argon2id:" in digest) and digest != hashlib.sha256(b"fixture-only-secret").digest()
    restored_config = yaml.safe_load((restored / "config/user.yaml").read_text(encoding="utf-8"))
    assert restored_config["schema_version"] == "4" and "onebot" not in restored_config
    result = {"archive": str(archive), "source_database_version": 7, "restored_database_version": version,
              "config_migrated": True, "password_upgraded": True, "access_lists_preserved": True,
              "server_exit_code": server.returncode}
    (output / "result.json").write_text(json.dumps(result, indent=2), encoding="utf-8")
    return result


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--server", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True, help="new directory for synthetic fixture artifacts")
    arguments = parser.parse_args()
    print(json.dumps(rehearse(Path(__file__).resolve().parents[2], arguments.server.resolve(), arguments.output.resolve()), indent=2))
