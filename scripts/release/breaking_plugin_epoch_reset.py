#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import shutil
import sqlite3
import tempfile
import zipfile
from datetime import datetime, timezone
from pathlib import Path


RESET_TABLES = (
    "plugin_instances",
    "plugin_packages",
    "plugin_kv",
    "scheduler_jobs",
    "third_party_accounts",
    "bilibili_source_config",
    "bilibili_source_rooms",
    "bilibili_source_seen",
    "bilibili_source_dynamics",
    "bilibili_source_state",
)


def timestamp_slug() -> str:
    return datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")


def verify_breaking_backup(root: Path, backup: Path) -> None:
    root = root.resolve()
    backup = backup.resolve()
    if not backup.is_file() or backup.is_relative_to(root):
        raise ValueError("the verified breaking-baseline backup must be a file outside the install root")
    with zipfile.ZipFile(backup) as archive:
        if archive.testzip() is not None:
            raise ValueError("breaking-baseline backup failed ZIP integrity validation")
        try:
            manifest = json.loads(archive.read("breaking-baseline-backup.json"))
        except (KeyError, json.JSONDecodeError) as exc:
            raise ValueError("breaking-baseline backup manifest is missing or invalid") from exc
    if manifest.get("kind") != "breaking-baseline-backup":
        raise ValueError("backup is not a breaking-baseline backup")
    if Path(str(manifest.get("source_root", ""))).resolve() != root:
        raise ValueError("backup source root does not match the reset target")
    included = set(manifest.get("included_paths", []))
    if "data/rayleabot.db" not in included:
        raise ValueError("backup does not contain data/rayleabot.db")


def existing_tables(connection: sqlite3.Connection) -> set[str]:
    return {
        row[0]
        for row in connection.execute("SELECT name FROM sqlite_master WHERE type = 'table'")
    }


def count_rows(connection: sqlite3.Connection, table: str) -> int:
    return int(connection.execute(f'SELECT COUNT(*) FROM "{table}"').fetchone()[0])


def reset_database(database: Path) -> dict[str, int]:
    if not database.is_file():
        raise ValueError(f"database does not exist: {database}")
    connection = sqlite3.connect(database, timeout=10)
    try:
        connection.execute("PRAGMA foreign_keys = ON")
        tables = existing_tables(connection)
        missing = sorted(set(RESET_TABLES) - tables)
        if missing:
            raise ValueError(f"database is missing reset tables: {', '.join(missing)}")

        counts = {table: count_rows(connection, table) for table in RESET_TABLES}
        counts["plugin_settings"] = int(
            connection.execute(
                "SELECT COUNT(*) FROM system_configs WHERE namespace LIKE 'plugin:%:settings'"
            ).fetchone()[0]
        )
        counts["plugin_and_third_party_secrets"] = int(
            connection.execute(
                "SELECT COUNT(*) FROM secret_store WHERE key LIKE 'plugin:%' OR key LIKE 'third_party:%'"
            ).fetchone()[0]
        )
        counts["plugin_template_states"] = int(
            connection.execute(
                "SELECT COUNT(*) FROM render_template_states WHERE source_type = 'plugin'"
            ).fetchone()[0]
        )

        connection.execute("BEGIN IMMEDIATE")
        connection.execute(
            "CREATE TEMP TABLE reset_plugin_templates AS "
            "SELECT template_id FROM render_template_states WHERE source_type = 'plugin'"
        )
        connection.execute("DELETE FROM render_template_states WHERE source_type = 'plugin'")
        connection.execute(
            "DELETE FROM render_template_revisions "
            "WHERE template_id IN (SELECT template_id FROM reset_plugin_templates)"
        )
        connection.execute("DROP TABLE reset_plugin_templates")
        connection.execute("DELETE FROM system_configs WHERE namespace LIKE 'plugin:%:settings'")
        connection.execute(
            "DELETE FROM secret_store WHERE key LIKE 'plugin:%' OR key LIKE 'third_party:%'"
        )
        for table in RESET_TABLES:
            connection.execute(f'DELETE FROM "{table}"')
        connection.commit()
        connection.execute("PRAGMA wal_checkpoint(TRUNCATE)")
        return counts
    except Exception:
        connection.rollback()
        raise
    finally:
        connection.close()


def retire_plugin_trees(root: Path) -> list[str]:
    targets = (root / "plugins" / "installed", root / "data" / "plugins")
    removed: list[str] = []
    with tempfile.TemporaryDirectory(prefix=f"rayleabot-plugin-v1-{timestamp_slug()}-", dir=root.parent) as holding:
        holding_root = Path(holding)
        for target in targets:
            if not target.exists():
                continue
            resolved = target.resolve()
            if not resolved.is_relative_to(root.resolve()):
                raise ValueError(f"refusing to retire path outside install root: {resolved}")
            destination = holding_root / target.relative_to(root)
            destination.parent.mkdir(parents=True, exist_ok=True)
            target.replace(destination)
            removed.append(target.relative_to(root).as_posix())
        # TemporaryDirectory removes the retired trees after they leave discovery roots.
    return removed


def reset_plugin_epoch(root: Path, backup: Path, database: Path | None = None) -> dict[str, object]:
    root = root.resolve()
    if not root.is_dir():
        raise ValueError(f"install root does not exist: {root}")
    verify_breaking_backup(root, backup)
    database = (database or root / "data" / "rayleabot.db").resolve()
    if not database.is_relative_to(root):
        raise ValueError("database must be inside the install root")
    counts = reset_database(database)
    removed = retire_plugin_trees(root)
    return {
        "plugin_epoch": "2",
        "database": str(database),
        "cleared_rows": counts,
        "removed_paths": removed,
        "backup": str(backup.resolve()),
    }


def main() -> int:
    parser = argparse.ArgumentParser(description="Reset all pre-Go plugin state after a verified external backup.")
    parser.add_argument("--root", required=True, type=Path)
    parser.add_argument("--backup", required=True, type=Path)
    parser.add_argument("--database", type=Path)
    parser.add_argument("--apply", action="store_true", help="Required confirmation for the destructive reset.")
    args = parser.parse_args()
    if not args.apply:
        parser.error("--apply is required")
    result = reset_plugin_epoch(args.root, args.backup, args.database)
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
