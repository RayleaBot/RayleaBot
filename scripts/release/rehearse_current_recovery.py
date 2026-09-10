"""Exercise fresh initialization and current-format backup/restore with a real Server."""
from __future__ import annotations

import argparse
import contextlib
import hashlib
import json
import os
from pathlib import Path
import socket
import shutil
import sqlite3
import subprocess
import time
import urllib.error
import urllib.request
import zipfile

import yaml

SETUP_TOKEN = "A" * 43
CONTROL_TOKEN = "B" * 43
FIXTURE_SECRET = "fixture-only-secret"


def run(binary: Path, root: Path, *args: str) -> None:
    with (root / "cli.log").open("a", encoding="utf-8") as log:
        subprocess.run(
            [str(binary), "-config", str(root / "config/user.yaml"), *args], cwd=root,
            stdout=log, stderr=subprocess.STDOUT, check=True, timeout=60,
            creationflags=subprocess.CREATE_NO_WINDOW if os.name == "nt" else 0,
        )


def request(origin: str, path: str, *, data: dict | None = None,
            token: str = "", setup: bool = False, control: bool = False) -> dict:
    headers = {"Origin": origin, "Content-Type": "application/json", "X-Raylea-Session-Transport": "bearer"}
    if token:
        headers["Authorization"] = "Bearer " + token
    if setup:
        headers["X-Raylea-Setup-Token"] = SETUP_TOKEN
    if control:
        headers["X-Raylea-Launcher-Control"] = CONTROL_TOKEN
    body = json.dumps(data).encode() if data is not None else None
    try:
        with urllib.request.urlopen(urllib.request.Request(origin + path, data=body, headers=headers), timeout=10) as response:
            raw = response.read()
            return json.loads(raw) if raw else {}
    except urllib.error.HTTPError as exc:
        try:
            code = json.loads(exc.read()).get("error", {}).get("code", "unknown")
        except (ValueError, AttributeError):
            code = "invalid_error_response"
        raise RuntimeError(f"{path} returned HTTP {exc.code} ({code})") from exc


def choose_port() -> int:
    with socket.socket() as probe:
        probe.bind(("127.0.0.1", 0))
        return probe.getsockname()[1]


@contextlib.contextmanager
def running_server(binary: Path, root: Path, port: int):
    origin = f"http://127.0.0.1:{port}"
    environment = {**os.environ, "RAYLEA_SETUP_TOKEN": SETUP_TOKEN, "RAYLEA_LAUNCHER_CONTROL_TOKEN": CONTROL_TOKEN}
    with (root / "server.log").open("a", encoding="utf-8") as log:
        server = subprocess.Popen(
            [str(binary), "-config", str(root / "config/user.yaml")], cwd=root,
            env=environment, stdout=log, stderr=subprocess.STDOUT,
            creationflags=subprocess.CREATE_NO_WINDOW if os.name == "nt" else 0,
        )
        errors: list[BaseException] = []
        try:
            deadline = time.monotonic() + 30
            while True:
                if server.poll() is not None:
                    raise RuntimeError(f"Server exited: inspect {root / 'server.log'}")
                try:
                    request(origin, "/healthz")
                    break
                except (OSError, urllib.error.URLError):
                    if time.monotonic() >= deadline:
                        raise TimeoutError(f"Server did not become healthy: {root}")
                    time.sleep(0.1)
            yield origin
        except BaseException as exc:
            errors.append(exc)
        finally:
            if server.poll() is None:
                try:
                    request(origin, "/api/launcher/shutdown", data={}, control=True)
                    server.wait(timeout=15)
                except BaseException as exc:
                    errors.append(exc)
                finally:
                    if server.poll() is None:
                        try:
                            server.kill()
                        except BaseException as exc:
                            errors.append(exc)
                        # Reap even when kill fails or the process exits concurrently.
                        try:
                            server.wait(timeout=5)
                        except BaseException as exc:
                            errors.append(exc)
            if server.returncode is not None and server.returncode != 0:
                errors.append(RuntimeError(
                    f"Server did not exit cleanly: {server.returncode}; inspect {root / 'server.log'}"))
    if len(errors) == 1:
        raise errors[0]
    if errors:
        raise BaseExceptionGroup("Recovery rehearsal or Server shutdown failed", errors)


def database_facts(path: Path) -> dict:
    with sqlite3.connect(path) as connection:
        version, initialized_at = connection.execute(
            "SELECT version, initialized_at FROM schema_metadata WHERE singleton_id = 1"
        ).fetchone()
        digest = connection.execute("SELECT secret_digest FROM auth_bootstrap_state").fetchone()[0]
        kv = connection.execute("SELECT value_json FROM plugin_kv WHERE plugin_id = 'recovery.fixture' AND key = 'cursor'").fetchone()
        tables = sorted(row[0] for row in connection.execute("SELECT name FROM sqlite_master WHERE type='table'"))
        assert connection.execute("PRAGMA quick_check").fetchone()[0] == "ok"
    assert digest.startswith(b"raylea-pwd:") and b":argon2id:" in digest
    assert not any(name.startswith("bilibili_source_") for name in tables)
    return {"schema_version": version, "initialized_at": initialized_at,
            "plugin_cursor": json.loads(kv[0]) if kv else None, "tables": tables}


def wait_for_plugin(origin: str, token: str, plugin_id: str) -> None:
    deadline = time.monotonic() + 30
    while time.monotonic() < deadline:
        detail = request(origin, f"/api/plugins/{plugin_id}", token=token)["plugin"]
        if detail["state"] == "running":
            return
        if detail["state"] in {"failed", "invalid"}:
            raise RuntimeError(f"recovery fixture did not start: {detail['state']}")
        time.sleep(0.1)
    raise TimeoutError("recovery fixture did not reach running")


def install_fixture(origin: str, token: str, fixture: Path) -> str:
    inspection = request(origin, "/api/plugins/install/inspect", token=token,
                         data={"source_type": "local_zip", "source": str(fixture.resolve(strict=True))})
    task = request(origin, "/api/plugins/install", token=token,
                   data={"inspection_id": inspection["inspection_id"],
                         "package_sha256": inspection["package_sha256"], "trusted_code_confirmed": True})
    deadline = time.monotonic() + 30
    while True:
        status = request(origin, f"/api/system/tasks/{task['task_id']}", token=token)["status"]
        if status == "succeeded":
            break
        if status not in {"pending", "running"}:
            raise RuntimeError(f"recovery fixture install task ended with {status}")
        if time.monotonic() >= deadline:
            raise TimeoutError("recovery fixture install did not complete")
        time.sleep(0.1)
    plugin_id = inspection["plugin"]["id"]
    request(origin, f"/api/plugins/{plugin_id}/enable", token=token, data={})
    wait_for_plugin(origin, token, plugin_id)
    return plugin_id


def package_hashes(root: Path) -> dict[str, str]:
    return {path.relative_to(root).as_posix(): hashlib.sha256(path.read_bytes()).hexdigest()
            for path in sorted(root.rglob("*")) if path.is_file()}


def rehearse(binary: Path, output: Path, *, distribution_root: Path | None = None,
             plugin_fixture: Path | None = None, observation_window_seconds: float = 0,
             database_layout: str = "default") -> dict:
    """Use one current binary; output must be new and contains every synthetic artifact."""
    binary = binary.resolve(strict=True)
    if database_layout not in {"default", "custom", "absolute"}:
        raise ValueError("unsupported rehearsal database layout")
    output.mkdir(parents=True, exist_ok=False)
    source, restored = output / "source", output / "restored"
    source.mkdir()
    restored.mkdir()
    if distribution_root is not None:
        for root in (source, restored):
            shutil.copy2(distribution_root / "build_info.json", root / "build_info.json")
    run(binary, source, "config", "init")
    port = choose_port()
    config_path = source / "config/user.yaml"
    config = yaml.safe_load(config_path.read_text(encoding="utf-8"))
    config["server"].update(host="127.0.0.1", port=port)
    config["adapters"] = []
    config["command"]["prefixes"] = ["!"]
    if database_layout == "custom":
        config["database"]["path"] = "custom/state.db"
    elif database_layout == "absolute":
        config["database"]["path"] = str(source / "absolute-source" / "state.db")
    config_path.write_text(yaml.safe_dump(config, allow_unicode=True), encoding="utf-8")
    plugin_id = None
    with running_server(binary, source, port) as origin:
        session = request(origin, "/api/setup/admin", data={"identifier": "admin", "secret": FIXTURE_SECRET}, setup=True)
        assert session.get("session_token"), "fresh setup did not create a session"
        assert request(origin, "/api/setup/status")["initialized"] is True
        if plugin_fixture is not None:
            plugin_id = install_fixture(origin, session["session_token"], plugin_fixture)

    # Plugin KV and files represent the persisted business state restored together.
    configured_database = Path(config["database"]["path"])
    database = configured_database if configured_database.is_absolute() else source / configured_database
    with sqlite3.connect(database) as connection:
        connection.execute(
            "INSERT INTO plugin_kv (plugin_id, key, value_json, size_bytes, updated_at) VALUES (?, ?, ?, ?, ?)",
            ("recovery.fixture", "cursor", "42", 2, "2026-09-10T00:00:00Z"),
        )
    state_file = source / "data/plugins/recovery.fixture/state.json"
    state_file.parent.mkdir(parents=True)
    state_file.write_text('{"cursor":42}\n', encoding="utf-8")
    before = database_facts(database)
    run(binary, source, "backup")
    archives = list((source / "backups").glob("*.zip"))
    assert len(archives) == 1, archives
    archive = archives[0]
    source_database_digest = hashlib.sha256(database.read_bytes()).hexdigest()
    with zipfile.ZipFile(archive) as package:
        manifest = json.loads(package.read("backup-manifest.json"))
        assert manifest["config_schema_version"] == config["schema_version"], manifest
        assert manifest["db_schema_version"] == before["schema_version"], manifest
        assert "data/plugins/recovery.fixture/state.json" in package.namelist()

    # The target has no configuration or database before the restore command.
    run(binary, restored, "restore", str(archive))
    restored_config = yaml.safe_load((restored / "config/user.yaml").read_text(encoding="utf-8"))
    expected_config = json.loads(json.dumps(config))
    if configured_database.is_absolute():
        expected_config["database"]["path"] = "data/rayleabot.db"
    assert restored_config == expected_config
    assert (restored / "data/plugins/recovery.fixture/state.json").read_bytes() == state_file.read_bytes()
    restored_database = restored / expected_config["database"]["path"]
    after = database_facts(restored_database)
    assert before == after, (before, after)
    installed_hashes = package_hashes(source / "plugins/installed")
    assert package_hashes(restored / "plugins/installed") == installed_hashes
    for _ in range(2):
        with running_server(binary, restored, port) as origin:
            session = request(origin, "/api/session/login", data={"identifier": "admin", "secret": FIXTURE_SECRET})
            assert session.get("session_token"), "restored credentials could not log in"
            diagnostics = request(origin, "/api/system/diagnostics", token=session["session_token"])
            assert diagnostics["database"]["schema_version"] == before["schema_version"]
            assert diagnostics["database"]["initialized_at"] == before["initialized_at"]
            if plugin_id:
                wait_for_plugin(origin, session["session_token"], plugin_id)
            deadline = time.monotonic() + observation_window_seconds
            while time.monotonic() < deadline:
                request(origin, "/healthz")
                if plugin_id:
                    wait_for_plugin(origin, session["session_token"], plugin_id)
                time.sleep(min(1, max(0, deadline - time.monotonic())))
    assert database_facts(restored_database) == before
    assert hashlib.sha256(database.read_bytes()).hexdigest() == source_database_digest
    result = {"archive": str(archive), "schema_version": before["schema_version"],
              "initialized_at": before["initialized_at"], "fresh_setup": True,
              "configuration_preserved": True, "plugin_data_preserved": True,
              "restored_login": True, "repeated_start_idempotent": True,
              "installed_plugin": plugin_id, "installed_package_files": len(installed_hashes),
              "database_layout": database_layout, "restored_database_path": expected_config["database"]["path"]}
    (output / "result.json").write_text(json.dumps(result, indent=2), encoding="utf-8")
    return result


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--server", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True, help="new directory for synthetic artifacts")
    parser.add_argument("--database-layout", choices=("default", "custom", "absolute"), default="default")
    arguments = parser.parse_args()
    print(json.dumps(rehearse(arguments.server, arguments.output.resolve(), database_layout=arguments.database_layout), indent=2))


if __name__ == "__main__":
    main()
