# Storage Migrations

RayleaBot uses a current-schema snapshot plus numbered legacy compatibility migrations.

## Sources

| Purpose | Source |
| --- | --- |
| Current SQLite schema snapshot | `server/internal/storage/schema.sql` |
| SQL query generation schema | `server/sqlc.yaml` |
| Legacy compatibility migrations | `server/internal/storage/migrations/*.sql` |
| Runtime migration list | `server/internal/storage/store_schema.go` |

`schema.sql` is the complete schema for new databases and the input for sqlc. `migrations/` contains compatibility steps for databases created before the current schema snapshot.

## Runtime Rules

- New databases are initialized from `schema.sql`.
- New databases record every known migration version in `schema_migrations` after the snapshot is applied.
- Existing databases are upgraded by applying missing numbered migrations.
- `000001_base.sql` is the legacy compatibility base. It is not the current schema snapshot.
- `schema_migrations` stores `version`, `name`, and `applied_at`; startup backfills missing names for older rows.
- Compatibility migrations may use explicit state inspection to skip a step when the target schema already exists.
- Duplicate-column errors are not normal control flow and are not ignored by matching database error strings.

## Schema Change Rules

- New tables, indexes, columns, constraints, and seed rows must update `server/internal/storage/schema.sql`.
- If existing databases need an upgrade path, add a numbered migration under `server/internal/storage/migrations/`.
- If SQL query generation is affected, run `sqlc generate` and verify `sqlc diff`.
- Storage tests must cover fresh initialization and legacy migration convergence for the changed schema.
- Application queries must name columns explicitly; schema equivalence tests intentionally ignore SQLite column order because `ALTER TABLE ... ADD COLUMN` appends fields.

## Runner Limits

The built-in runner supports ordinary semicolon-separated SQLite statements. Migration files must not rely on procedural bodies, triggers, string literals containing semicolon-delimited SQL, or parser-sensitive SQL blocks.

Use a parser-backed migration tool only when migrations need rollback metadata, multi-step online migration orchestration, complex SQL bodies, or stronger drift planning than the built-in runner can provide.

## Tool Evaluation

`goose`, `golang-migrate`, and `Atlas` are candidates for a future migration tool boundary. They are not part of the frozen stack. Introducing one requires updating `docs/engineering/baseline.md`, CI, release notes, and the storage tests in the same change.
